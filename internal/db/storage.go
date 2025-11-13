package db

import (
	"database/sql"
	"draft/internal/models"
	"encoding/json"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Storage реализует слой доступа к данным (PostgreSQL)
type Storage struct {
	db *sql.DB
}

// NewStorage открывает соединение с PostgreSQL и создаёт структуру Storage
func NewStorage(connString string) (*Storage, error) {
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("БД недоступна: %w", err)
	}

	s := &Storage{db: db}
	return s, nil
}

// ---------- Team ----------

func (s *Storage) TeamExists(name string) bool {
	var exists bool
	_ = s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM teams WHERE team_name=$1)`, name).Scan(&exists)
	return exists
}

func (s *Storage) SaveTeam(team *models.Team) {
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Commit()

	_, _ = tx.Exec(`INSERT INTO teams (team_name) VALUES ($1) ON CONFLICT (team_name) DO NOTHING`, team.TeamName)

	// Удалим старых участников, чтобы пересоздать
	_, _ = tx.Exec(`DELETE FROM users WHERE team_name=$1`, team.TeamName)

	for _, m := range team.Members {
		_, _ = tx.Exec(`
			INSERT INTO users (user_id, username, team_name, is_active)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id) DO UPDATE SET
				username=EXCLUDED.username,
				team_name=EXCLUDED.team_name,
				is_active=EXCLUDED.is_active`,
			m.UserId, m.Username, team.TeamName, m.IsActive,
		)
	}
}

func (s *Storage) GetTeam(name string) (*models.Team, bool) {
	rows, err := s.db.Query(`SELECT user_id, username, is_active FROM users WHERE team_name=$1`, name)
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	var members []models.TeamMember
	for rows.Next() {
		var m models.TeamMember
		_ = rows.Scan(&m.UserId, &m.Username, &m.IsActive)
		members = append(members, m)
	}

	if len(members) == 0 {
		return nil, false
	}

	return &models.Team{
		TeamName: name,
		Members:  members,
	}, true
}

// ---------- Users ----------

func (s *Storage) SaveUser(user *models.User) {
	_, _ = s.db.Exec(`
		INSERT INTO users (user_id, username, team_name, is_active)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			username=EXCLUDED.username,
			team_name=EXCLUDED.team_name,
			is_active=EXCLUDED.is_active`,
		user.UserId, user.Username, user.TeamName, user.IsActive,
	)
}

func (s *Storage) GetUser(id string) (*models.User, bool) {
	row := s.db.QueryRow(`SELECT user_id, username, team_name, is_active FROM users WHERE user_id=$1`, id)
	var u models.User
	if err := row.Scan(&u.UserId, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		return nil, false
	}
	return &u, true
}

// ---------- Pull Requests ----------

func (s *Storage) PullRequestExists(id string) bool {
	var exists bool
	_ = s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id=$1)`, id).Scan(&exists)
	return exists
}

func (s *Storage) SavePullRequest(pr *models.PullRequest) {
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Commit()

	// сериализуем массив ревьюверов в JSONB
	reviewersJSON, _ := json.Marshal(pr.AssignedReviewers)

	_, _ = tx.Exec(`
		INSERT INTO pull_requests (
			pull_request_id, pull_request_name, author_id,
			assigned_reviewers, status, created_at, merged_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (pull_request_id) DO UPDATE SET
			pull_request_name=EXCLUDED.pull_request_name,
			assigned_reviewers=EXCLUDED.assigned_reviewers,
			status=EXCLUDED.status,
			merged_at=EXCLUDED.merged_at`,
		pr.PullRequestId, pr.PullRequestName, pr.AuthorId,
		reviewersJSON, pr.Status, pr.CreatedAt, pr.MergedAt,
	)
}

func (s *Storage) GetPullRequest(id string) (*models.PullRequest, bool) {
	row := s.db.QueryRow(`
		SELECT pull_request_id, pull_request_name, author_id,
		       assigned_reviewers, status, created_at, merged_at
		FROM pull_requests WHERE pull_request_id=$1`, id)

	var pr models.PullRequest
	var reviewersJSON []byte

	if err := row.Scan(
		&pr.PullRequestId, &pr.PullRequestName, &pr.AuthorId,
		&reviewersJSON, &pr.Status, &pr.CreatedAt, &pr.MergedAt,
	); err != nil {
		return nil, false
	}

	_ = json.Unmarshal(reviewersJSON, &pr.AssignedReviewers)

	return &pr, true
}

func (s *Storage) GetAllPullRequests() []models.PullRequest {
	rows, err := s.db.Query(`SELECT pull_request_id FROM pull_requests`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []models.PullRequest
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		pr, ok := s.GetPullRequest(id)
		if ok {
			result = append(result, *pr)
		}
	}
	return result
}
