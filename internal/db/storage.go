package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"pr-reviewer/internal/models"

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

func (s *Storage) TeamExists(name string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM teams WHERE team_name=$1)`, name).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("ошибка при проверке существования команды: %w", err)
	}
	return exists, nil
}

func (s *Storage) SaveTeam(team *models.Team) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	// Откатываем, если err != nil к моменту выхода из функции (или коммит не удался)
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`INSERT INTO teams (team_name) VALUES ($1) ON CONFLICT (team_name) DO NOTHING`, team.TeamName); err != nil {
		return fmt.Errorf("ошибка вставки команды: %w", err)
	}

	// Удалим старых участников, чтобы пересоздать
	if _, err = tx.Exec(`DELETE FROM users WHERE team_name=$1`, team.TeamName); err != nil {
		return fmt.Errorf("ошибка удаления предыдущих участников: %w", err)
	}

	for _, m := range team.Members {
		if _, err = tx.Exec(`
			INSERT INTO users (user_id, username, team_name, is_active)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id) DO UPDATE SET
				username=EXCLUDED.username,
				team_name=EXCLUDED.team_name,
				is_active=EXCLUDED.is_active`,
			m.UserId, m.Username, team.TeamName, m.IsActive,
		); err != nil {
			return fmt.Errorf("ошибка вставки участника (user_id=%s): %w", m.UserId, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	return nil
}

func (s *Storage) GetTeam(name string) (*models.Team, error) {
	rows, err := s.db.Query(`SELECT user_id, username, is_active FROM users WHERE team_name=$1`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var members []models.TeamMember
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&m.UserId, &m.Username, &m.IsActive); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании участника %w", err)
		}
		members = append(members, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении строк: %w", err)
	}

	return &models.Team{
		TeamName: name,
		Members:  members,
	}, nil
}

// ---------- Users ----------

func (s *Storage) SaveUser(user *models.User) error {
	_, err := s.db.Exec(`
		INSERT INTO users (user_id, username, team_name, is_active)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			username=EXCLUDED.username,
			team_name=EXCLUDED.team_name,
			is_active=EXCLUDED.is_active`,
		user.UserId, user.Username, user.TeamName, user.IsActive,
	)

	if err != nil {
		return fmt.Errorf("ошибка при сохранении пользователя: %w", err)
	}
	return nil
}

func (s *Storage) GetUser(id string) (*models.User, error) {
	row := s.db.QueryRow(
		`SELECT user_id, username, team_name, is_active FROM users WHERE user_id=$1`,
		id,
	)

	var u models.User
	err := row.Scan(&u.UserId, &u.Username, &u.TeamName, &u.IsActive)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("пользователь %s не найден", id)
		}
		return nil, fmt.Errorf("ошибка при получении пользователя: %w", err)
	}

	return &u, nil
}

// ---------- Pull Requests ----------

func (s *Storage) PullRequestExists(id string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id=$1)`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке существования pull request: %w", err)
	}
	return exists, nil
}

// TODO: делать rollback если err != nil
func (s *Storage) SavePullRequest(pr *models.PullRequest) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// сериализуем массив ревьюверов в JSONB
	reviewersJSON, _ := json.Marshal(pr.AssignedReviewers)

	if _, err = tx.Exec(`
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
	); err != nil {
		return fmt.Errorf("ошибка создания PR: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	return nil
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

	err := json.Unmarshal(reviewersJSON, &pr.AssignedReviewers)
	if err != nil {
		return nil, false
	}
	return &pr, true
}

func (s *Storage) GetPullRequestsByReviewer(userId string) []models.PullRequest {
	var pullRequests []models.PullRequest

	rows, err := s.db.Query(`
        SELECT pull_request_id, pull_request_name, author_id,
               assigned_reviewers, status
        FROM pull_requests 
        WHERE jsonb_exists(assigned_reviewers::jsonb, $1)`, userId)
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var pr models.PullRequest
		var reviewersJSON []byte

		if err := rows.Scan(
			&pr.PullRequestId, &pr.PullRequestName, &pr.AuthorId,
			&reviewersJSON, &pr.Status,
		); err != nil {
			continue
		}

		if err := json.Unmarshal(reviewersJSON, &pr.AssignedReviewers); err != nil {
			continue
		}

		pullRequests = append(pullRequests, pr)
	}

	return pullRequests
}

