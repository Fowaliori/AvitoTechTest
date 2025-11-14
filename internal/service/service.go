package service

import (
	"errors"
	"fmt"
	"pr-reviewer/internal/db"
	"pr-reviewer/internal/models"
	"time"
)

// ServiceError представляет ошибку бизнес-логики
type ServiceError struct {
	Code    models.ErrorResponseErrorCode
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}

var (
	ErrTeamExists          = &ServiceError{Code: models.TEAMEXISTS, Message: "команда с таким именем уже существует"}
	ErrTeamNotFound        = &ServiceError{Code: models.NOTFOUND, Message: "команда не найдена"}
	ErrUserNotFound        = &ServiceError{Code: models.NOTFOUND, Message: "пользователь не найден"}
	ErrPRExists            = &ServiceError{Code: models.PREXISTS, Message: "PR с таким идентификатором уже существует"}
	ErrPRNotFound          = &ServiceError{Code: models.NOTFOUND, Message: "PR не найден"}
	ErrPRMerged            = &ServiceError{Code: models.PRMERGED, Message: "нельзя переназначить ревьювера для объединённого PR"}
	ErrReviewerNotAssigned = &ServiceError{Code: models.NOTASSIGNED, Message: "ревьювер не назначен на этот PR"}
	ErrNoCandidate         = &ServiceError{Code: models.NOCANDIDATE, Message: "нет активных кандидатов для замены в команде"}
)

// Service содержит бизнес-логику
type Service struct {
	storage *db.Storage
}

// NewService создает новый сервис
func NewService(storage *db.Storage) *Service {
	return &Service{storage: storage}
}

// CreateTeam создает команду с участниками
func (s *Service) CreateTeam(team *models.Team) error {
	exists, err := s.storage.TeamExists(team.TeamName)

	if err != nil {
		return fmt.Errorf("ошибка при проверке: %w", err)
	}

	if exists {
		return ErrTeamExists
	}
	// Создаем/обновляем пользователей
	for _, member := range team.Members {
		err := s.storage.SaveUser(&models.User{
			UserId:   member.UserId,
			Username: member.Username,
			TeamName: team.TeamName,
			IsActive: member.IsActive,
		})
		if err != nil {
			return fmt.Errorf("ошибка при сохранении пользователя: %w", err)
		}
	}

	s.storage.SaveTeam(team)
	return nil
}

// GetTeam получает команду
func (s *Service) GetTeam(name string) (*models.Team, error) {
	team, err := s.storage.GetTeam(name)
	if err != nil {
		return nil, ErrTeamNotFound
	}
	return team, nil
}

// SetUserActive устанавливает флаг активности пользователя
func (s *Service) SetUserActive(userId string, isActive bool) (*models.User, error) {
	user, err := s.storage.GetUser(userId)
	if err != nil {
		return nil, ErrUserNotFound
	}

	user.IsActive = isActive
	err = s.storage.SaveUser(user)
	if err != nil {
		return nil, fmt.Errorf("ошибка при сохранении пользователя: %w", err)
	}

	return user, nil
}

// CreatePullRequest создает PR и автоматически назначает до 2 ревьюверов
func (s *Service) CreatePullRequest(prId, prName, authorId string) (*models.PullRequest, error) {
	if _, err := s.storage.PullRequestExists(prId); err != nil {
		return nil, ErrPRExists
	}

	author, err := s.storage.GetUser(authorId)
	if err != nil {
		return nil, ErrUserNotFound
	}

	team, err := s.storage.GetTeam(author.TeamName)
	if err != nil {
		return nil, ErrTeamNotFound
	}

	reviewers := s.findActiveReviewers(team, authorId, 2)

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestId:     prId,
		PullRequestName:   prName,
		AuthorId:          authorId,
		Status:            models.PullRequestStatusOPEN,
		AssignedReviewers: reviewers,
		CreatedAt:         &now,
	}

	s.storage.SavePullRequest(pr)
	return pr, nil
}

// MergePullRequest помечает PR как MERGED
func (s *Service) MergePullRequest(prId string) (*models.PullRequest, error) {
	pr, exists := s.storage.GetPullRequest(prId)
	if !exists {
		return nil, ErrPRNotFound
	}

	// Идемпотентная операция
	if pr.Status != models.PullRequestStatusMERGED {
		now := time.Now()
		pr.Status = models.PullRequestStatusMERGED
		pr.MergedAt = &now
		s.storage.SavePullRequest(pr)
	}

	return pr, nil
}

// ReassignReviewer переназначает ревьювера
func (s *Service) ReassignReviewer(prId, oldReviewerId, newReviewerId string) (*models.PullRequest, error) {
	pr, exists := s.storage.GetPullRequest(prId)
	if !exists {
		return nil, ErrPRNotFound
	}

	if pr.Status == models.PullRequestStatusMERGED {
		return nil, ErrPRMerged
	}

	// Ищем и заменяем ревьювера
	found := false
	for i, reviewerId := range pr.AssignedReviewers {
		if reviewerId == oldReviewerId {
			pr.AssignedReviewers[i] = newReviewerId
			found = true
			break
		}
	}

	if !found {
		return nil, ErrReviewerNotAssigned
	}

	s.storage.SavePullRequest(pr)
	return pr, nil
}

// GetUserPullRequests получает PR'ы, где пользователь назначен ревьювером
func (s *Service) GetUserPullRequests(userId string) []models.PullRequestShort {
	var result []models.PullRequestShort

	for _, pr := range s.storage.GetAllPullRequests() {
		for _, reviewerId := range pr.AssignedReviewers {
			if reviewerId == userId {
				result = append(result, models.PullRequestShort{
					PullRequestId:   pr.PullRequestId,
					PullRequestName: pr.PullRequestName,
					AuthorId:        pr.AuthorId,
					Status:          models.PullRequestShortStatus(pr.Status),
				})
				break
			}
		}
	}

	return result
}

// findActiveReviewers находит активных ревьюверов из команды (исключая автора)
func (s *Service) findActiveReviewers(team *models.Team, excludeUserId string, maxCount int) []string {
	var reviewers []string

	for _, member := range team.Members {
		if member.UserId != excludeUserId {
			user, err := s.storage.GetUser(member.UserId)
			if err != nil && user.IsActive {
				reviewers = append(reviewers, member.UserId)
				if len(reviewers) >= maxCount {
					break
				}
			}
		}
	}

	return reviewers
}

// IsServiceError проверяет, является ли ошибка ServiceError
func IsServiceError(err error) (*ServiceError, bool) {
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr, true
	}
	return nil, false
}
