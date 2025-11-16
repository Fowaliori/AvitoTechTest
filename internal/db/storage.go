package db

import (
	"database/sql"
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

	// Вставляем новых участников команды
	for _, m := range team.Members {
		if _, err = tx.Exec(`
			INSERT INTO users (user_id, username, team_name, is_active)
			VALUES ($1, $2, $3, $4)`,
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

func (s *Storage) GetTeam(name string) (*models.Team, bool, error) {
	rows, err := s.db.Query(`SELECT user_id, username, is_active FROM users WHERE team_name=$1`, name)
	if err != nil {
		return nil, false, fmt.Errorf("ошибка при запросе команды: %w", err)
	}
	defer rows.Close()

	var members []models.TeamMember
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&m.UserId, &m.Username, &m.IsActive); err != nil {
			return nil, false, fmt.Errorf("ошибка при сканировании участника: %w", err)
		}
		members = append(members, m)
	}

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("ошибка при чтении строк: %w", err)
	}

	if len(members) == 0 {
		return nil, false, nil
	}

	return &models.Team{
		TeamName: name,
		Members:  members,
	}, true, nil
}

func (s *Storage) GetTeamByUserId(userId string) (*models.Team, bool, error) {
	rows, err := s.db.Query(`
		SELECT u2.team_name, u2.user_id, u2.username, u2.is_active
		FROM users u1
		INNER JOIN users u2 ON u1.team_name = u2.team_name
		WHERE u1.user_id = $1`, userId)
	if err != nil {
		return nil, false, fmt.Errorf("ошибка при получении команды по user_id: %w", err)
	}
	defer rows.Close()

	var members []models.TeamMember
	var teamName string
	userFound := false

	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&teamName, &m.UserId, &m.Username, &m.IsActive); err != nil {
			return nil, false, fmt.Errorf("ошибка при сканировании участника: %w", err)
		}
		members = append(members, m)
		userFound = true
	}

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("ошибка при чтении строк: %w", err)
	}

	if !userFound {
		return nil, false, nil
	}

	return &models.Team{
		TeamName: teamName,
		Members:  members,
	}, true, nil
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

func (s *Storage) GetUser(id string) (*models.User, bool, error) {
	row := s.db.QueryRow(
		`SELECT user_id, username, team_name, is_active FROM users WHERE user_id=$1`,
		id,
	)

	var u models.User
	err := row.Scan(&u.UserId, &u.Username, &u.TeamName, &u.IsActive)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("ошибка при получении пользователя: %w", err)
	}

	return &u, true, nil
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

	// Сохраняем или обновляем PR
	if _, err = tx.Exec(`
		INSERT INTO pull_requests (
			pull_request_id, pull_request_name, author_id,
			status, created_at, merged_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (pull_request_id) DO UPDATE SET
			pull_request_name=EXCLUDED.pull_request_name,
			status=EXCLUDED.status,
			merged_at=EXCLUDED.merged_at`,
		pr.PullRequestId, pr.PullRequestName, pr.AuthorId,
		pr.Status, pr.CreatedAt, pr.MergedAt,
	); err != nil {
		return fmt.Errorf("ошибка создания PR: %w", err)
	}

	// Удаляем старых ревьюверов
	if _, err = tx.Exec(`DELETE FROM pull_request_reviewers WHERE pull_request_id=$1`, pr.PullRequestId); err != nil {
		return fmt.Errorf("ошибка удаления старых ревьюверов: %w", err)
	}

	// Добавляем ревьюверов
	for _, reviewerId := range pr.AssignedReviewers {
		if _, err = tx.Exec(`
			INSERT INTO pull_request_reviewers (pull_request_id, user_id)
			VALUES ($1, $2)`,
			pr.PullRequestId, reviewerId,
		); err != nil {
			return fmt.Errorf("ошибка добавления ревьювера %s: %w", reviewerId, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	return nil
}

func (s *Storage) GetPullRequest(id string) (*models.PullRequest, bool, error) {
	// Получаем основную информацию о PR
	row := s.db.QueryRow(`
		SELECT pull_request_id, pull_request_name, author_id,
		       status, created_at, merged_at
		FROM pull_requests WHERE pull_request_id=$1`, id)

	var pr models.PullRequest
	if err := row.Scan(
		&pr.PullRequestId, &pr.PullRequestName, &pr.AuthorId,
		&pr.Status, &pr.CreatedAt, &pr.MergedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("ошибка при получении PR: %w", err)
	}

	// Получаем ревьюверов из отдельной таблицы
	reviewerRows, err := s.db.Query(`
		SELECT user_id FROM pull_request_reviewers 
		WHERE pull_request_id=$1 
		ORDER BY user_id`, id)
	if err != nil {
		return nil, false, fmt.Errorf("ошибка при получении ревьюверов: %w", err)
	}
	defer reviewerRows.Close()

	var reviewers []string
	for reviewerRows.Next() {
		var reviewerId string
		if err := reviewerRows.Scan(&reviewerId); err != nil {
			return nil, false, fmt.Errorf("ошибка при сканировании ревьювера: %w", err)
		}
		reviewers = append(reviewers, reviewerId)
	}

	if err := reviewerRows.Err(); err != nil {
		return nil, false, fmt.Errorf("ошибка при чтении ревьюверов: %w", err)
	}

	pr.AssignedReviewers = reviewers
	return &pr, true, nil
}

func (s *Storage) GetPullRequestsByReviewer(userId string) ([]models.PullRequest, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT pr.pull_request_id
		FROM pull_requests pr
		INNER JOIN pull_request_reviewers r ON pr.pull_request_id = r.pull_request_id
		WHERE r.user_id = $1`, userId)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении PR ревьювера: %w", err)
	}
	defer rows.Close()

	var prIds []string
	for rows.Next() {
		var prId string
		if err := rows.Scan(&prId); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании: %w", err)
		}
		prIds = append(prIds, prId)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении строк: %w", err)
	}

	var pullRequests []models.PullRequest
	for _, prId := range prIds {
		pr, found, err := s.GetPullRequest(prId)
		if err != nil {
			return nil, err
		}
		if found {
			pullRequests = append(pullRequests, *pr)
		}
	}

	return pullRequests, nil
}

// GetAvgReviewTimeByTeam возвращает среднее время ревью по командам авторов PR
func (s *Storage) GetAvgReviewTimeByTeam() ([]models.TeamReviewTimeStat, error) {
	rows, err := s.db.Query(`
		SELECT
			u.team_name,
			(AVG(COALESCE(pr.merged_at, NOW()) - pr.created_at))::text AS avg_review_time,
			COUNT(*) AS total_prs
		FROM pull_requests pr
		JOIN users u ON u.user_id = pr.author_id
		WHERE pr.created_at IS NOT NULL
		GROUP BY u.team_name
		ORDER BY AVG(COALESCE(pr.merged_at, NOW()) - pr.created_at)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.TeamReviewTimeStat
	for rows.Next() {
		var row models.TeamReviewTimeStat
		if err := rows.Scan(&row.TeamName, &row.AvgReviewTime, &row.TotalPrs); err != nil {
			return nil, err
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
