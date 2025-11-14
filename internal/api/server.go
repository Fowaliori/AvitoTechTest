package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"pr-reviewer/internal/models"
	"pr-reviewer/internal/service"
)

// Server реализует сгенерированный ServerInterface
type Server struct {
	service *service.Service
}

// NewServer создает сервер с внедрённым сервисом.
func NewServer(svc *service.Service) *Server {
	return &Server{service: svc}
}

// PostTeamAdd создает команду с участниками
func (s *Server) PostTeamAdd(w http.ResponseWriter, r *http.Request) {
	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		writeError(w, http.StatusBadRequest, models.NOTFOUND, "неверное тело запроса")
		return
	}

	if err := s.service.CreateTeam(&team); err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]*models.Team{"team": &team})
}

// GetTeamGet получает команду с участниками
func (s *Server) GetTeamGet(w http.ResponseWriter, r *http.Request, params GetTeamGetParams) {
	team, err := s.service.GetTeam(params.TeamName)
	if err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, team)
}

// PostUsersSetIsActive устанавливает флаг активности пользователя
func (s *Server) PostUsersSetIsActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserId   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, models.NOTFOUND, "неверное тело запроса")
		return
	}

	user, err := s.service.SetUserActive(req.UserId, req.IsActive)
	if err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]*models.User{"user": user})
}

// PostPullRequestCreate создает PR и автоматически назначает до 2 ревьюверов
func (s *Server) PostPullRequestCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestId   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorId        string `json:"author_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, models.NOTFOUND, "неверное тело запроса")
		return
	}

	pr, err := s.service.CreatePullRequest(req.PullRequestId, req.PullRequestName, req.AuthorId)
	if err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]*models.PullRequest{"pull_request": pr})
}

// PostPullRequestMerge помечает PR как MERGED
func (s *Server) PostPullRequestMerge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestId string `json:"pull_request_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, models.NOTFOUND, "неверное тело запроса")
		return
	}

	pr, err := s.service.MergePullRequest(req.PullRequestId)
	if err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]*models.PullRequest{"pull_request": pr})
}

// PostPullRequestReassign переназначает ревьювера
func (s *Server) PostPullRequestReassign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestId string `json:"pull_request_id"`
		OldReviewerId string `json:"old_reviewer_id"`
		NewReviewerId string `json:"new_reviewer_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, models.NOTFOUND, "неверное тело запроса")
		return
	}

	pr, err := s.service.ReassignReviewer(req.PullRequestId, req.OldReviewerId, req.NewReviewerId)
	if err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]*models.PullRequest{"pull_request": pr})
}

// GetUsersGetReview получает PR'ы, где пользователь назначен ревьювером
func (s *Server) GetUsersGetReview(w http.ResponseWriter, r *http.Request, params GetUsersGetReviewParams) {
	prs := s.service.GetUserPullRequests(params.UserId)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       params.UserId,
		"pull_requests": prs,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code models.ErrorResponseErrorCode, message string) {
	writeJSON(w, status, models.ErrorResponse{
		Error: struct {
			Code    models.ErrorResponseErrorCode `json:"code"`
			Message string                        `json:"message"`
		}{
			Code:    code,
			Message: message,
		},
	})
}

func handleError(w http.ResponseWriter, err error) {
	var serviceErr *service.ServiceError
	if errors.As(err, &serviceErr) {
		status := http.StatusBadRequest
		if serviceErr.Code == models.NOTFOUND {
			status = http.StatusNotFound
		}
		writeError(w, status, serviceErr.Code, serviceErr.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, models.NOTFOUND, "внутренняя ошибка сервера")
}
