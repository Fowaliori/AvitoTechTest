package service

import (
	"errors"
	"fmt"
	"math/rand"
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

	err = s.storage.SaveTeam(team)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении команды: %w", err)
	}
	return nil
}

// GetTeam получает команду
func (s *Service) GetTeam(name string) (*models.Team, error) {
	team, found, err := s.storage.GetTeam(name)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении команды: %w", err)
	}
	if !found {
		return nil, ErrTeamNotFound
	}
	return team, nil
}

// SetUserActive устанавливает флаг активности пользователя
func (s *Service) SetUserActive(userId string, isActive bool) (*models.User, error) {
	user, found, err := s.storage.GetUser(userId)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении пользователя: %w", err)
	}
	if !found {
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
	exists, err := s.storage.PullRequestExists(prId)
	if err != nil {
		return nil, fmt.Errorf("ошибка при проверке существования PR: %w", err)
	}
	if exists {
		return nil, ErrPRExists
	}

	team, found, err := s.storage.GetTeamByUserId(authorId)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении команды: %w", err)
	}
	if !found {
		return nil, ErrUserNotFound
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

	if err := s.storage.SavePullRequest(pr); err != nil {
		return nil, fmt.Errorf("ошибка при сохранении PR: %w", err)
	}
	return pr, nil
}

// MergePullRequest помечает PR как MERGED
func (s *Service) MergePullRequest(prId string) (*models.PullRequest, error) {
	pr, found, err := s.storage.GetPullRequest(prId)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении PR: %w", err)
	}
	if !found {
		return nil, ErrPRNotFound
	}

	if pr.Status == models.PullRequestStatusMERGED {
		return pr, nil
	}

	now := time.Now()
	pr.Status = models.PullRequestStatusMERGED
	pr.MergedAt = &now
	if err := s.storage.SavePullRequest(pr); err != nil {
		return nil, fmt.Errorf("ошибка при сохранении PR: %w", err)
	}

	return pr, nil
}

// ReassignReviewer переназначает ревьювера на случайного активного участника из команды заменяемого ревьювера
func (s *Service) ReassignReviewer(prId, oldReviewerId string) (*models.PullRequest, string, error) {
	pr, found, err := s.storage.GetPullRequest(prId)
	if err != nil {
		return nil, "", fmt.Errorf("ошибка при получении PR: %w", err)
	}
	if !found {
		return nil, "", ErrPRNotFound
	}

	if pr.Status == models.PullRequestStatusMERGED {
		return nil, "", ErrPRMerged
	}

	reviewerFound := false
	var foundIndex int
	for i, reviewerId := range pr.AssignedReviewers {
		if reviewerId == oldReviewerId {
			reviewerFound = true
			foundIndex = i
			break
		}
	}

	if !reviewerFound {
		return nil, "", ErrReviewerNotAssigned
	}

	team, teamFound, err := s.storage.GetTeamByUserId(oldReviewerId)
	if err != nil {
		return nil, "", fmt.Errorf("ошибка при получении команды ревьювера: %w", err)
	}
	if !teamFound {
		return nil, "", ErrUserNotFound
	}

	newReviewerId := s.findRandomActiveReviewer(team, append([]string{pr.AuthorId}, pr.AssignedReviewers...))
	if newReviewerId == "" {
		return nil, "", ErrNoCandidate
	}

	// Заменяем ревьювера
	pr.AssignedReviewers[foundIndex] = newReviewerId

	if err := s.storage.SavePullRequest(pr); err != nil {
		return nil, "", fmt.Errorf("ошибка при сохранении PR: %w", err)
	}

	return pr, newReviewerId, nil
}

// GetUserPullRequests получает PR'ы, где пользователь назначен ревьювером
func (s *Service) GetUserPullRequests(userId string) ([]models.PullRequestShort, error) {
	var result []models.PullRequestShort

	prs, err := s.storage.GetPullRequestsByReviewer(userId)
	if err != nil {
		return nil, err
	}

	for _, pr := range prs {
		result = append(result, models.PullRequestShort{
			PullRequestId:   pr.PullRequestId,
			PullRequestName: pr.PullRequestName,
			AuthorId:        pr.AuthorId,
			Status:          models.PullRequestShortStatus(pr.Status),
		})
	}

	return result, nil
}

// GetReviewTimeStats возвращает статистику среднего времени ревью по командам
func (s *Service) GetReviewTimeStats() ([]models.TeamReviewTimeStat, error) {
	return s.storage.GetAvgReviewTimeByTeam()
}

// findActiveReviewers выбирает до maxCount случайных активных ревьюверов из команды (исключая автора)
func (s *Service) findActiveReviewers(team *models.Team, excludeUserId string, maxCount int) []string {
	var candidates []string
	for _, member := range team.Members {
		if member.UserId != excludeUserId && member.IsActive {
			candidates = append(candidates, member.UserId)
		}
	}
	if len(candidates) <= maxCount {
		return candidates
	}

	indexMap := make(map[int]struct{})
	for len(indexMap) < maxCount {
		idx := rand.Intn(len(candidates)) //nolint:gosec
		indexMap[idx] = struct{}{}
	}
	var result []string
	for idx := range indexMap {
		result = append(result, candidates[idx])
	}
	return result
}

// findRandomActiveReviewer находит случайного активного участника из команды (исключая уже назначенных ревьюверов)
func (s *Service) findRandomActiveReviewer(team *models.Team, excludeReviewers []string) string {
	var candidates []string
	for _, member := range team.Members {
		if !member.IsActive {
			continue
		}
		excluded := false
		for _, reviewerId := range excludeReviewers {
			if member.UserId == reviewerId {
				excluded = true
				break
			}
		}
		if !excluded {
			candidates = append(candidates, member.UserId)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	return candidates[rand.Intn(len(candidates))] //nolint:gosec
}

// IsServiceError проверяет, является ли ошибка ServiceError
func IsServiceError(err error) (*ServiceError, bool) {
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr, true
	}
	return nil, false
}
