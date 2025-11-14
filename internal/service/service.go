package service

import (
	"errors"
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
	if s.storage.TeamExists(team.TeamName) {
		return ErrTeamExists
	}

	// Создаем/обновляем пользователей
	for _, member := range team.Members {
		s.storage.SaveUser(&models.User{
			UserId:   member.UserId,
			Username: member.Username,
			TeamName: team.TeamName,
			IsActive: member.IsActive,
		})
	}

	s.storage.SaveTeam(team)
	return nil
}

// GetTeam получает команду
func (s *Service) GetTeam(name string) (*models.Team, error) {
	team, exists := s.storage.GetTeam(name)
	if !exists {
		return nil, ErrTeamNotFound
	}
	return team, nil
}

// SetUserActive устанавливает флаг активности пользователя
func (s *Service) SetUserActive(userId string, isActive bool) (*models.User, error) {
	user, exists := s.storage.GetUser(userId)
	if !exists {
		return nil, ErrUserNotFound
	}

	user.IsActive = isActive
	s.storage.SaveUser(user)
	return user, nil
}

// CreatePullRequest создает PR и автоматически назначает до 2 ревьюверов
func (s *Service) CreatePullRequest(prId, prName, authorId string) (*models.PullRequest, error) {
	if s.storage.PullRequestExists(prId) {
		return nil, ErrPRExists
	}

	author, exists := s.storage.GetUser(authorId)
	if !exists {
		return nil, ErrUserNotFound
	}

	team, exists := s.storage.GetTeam(author.TeamName)
	if !exists {
		return nil, ErrTeamNotFound
	}

	// Находим активных ревьюверов из команды (исключая автора)
	reviewers := s.findActiveReviewers(team, authorId, 2)
	if len(reviewers) == 0 {
		return nil, ErrNoCandidate
	}

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
			user, ok := s.storage.GetUser(member.UserId)
			if ok && user.IsActive {
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
