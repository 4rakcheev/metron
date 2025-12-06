package api

import (
	"context"
	"encoding/json"
	"log"
	"metron/internal/core"
	"net/http"
	"strings"
)

// SessionManager interface for API operations
type SessionManager interface {
	StartSession(ctx context.Context, deviceType, deviceID string, childIDs []string, durationMinutes int) (*core.Session, error)
	ExtendSession(ctx context.Context, sessionID string, additionalMinutes int) (*core.Session, error)
	StopSession(ctx context.Context, sessionID string) error
	GetSession(ctx context.Context, sessionID string) (*core.Session, error)
	ListActiveSessions(ctx context.Context) ([]*core.Session, error)
	GetChildStatus(ctx context.Context, childID string) (*core.ChildStatus, error)
}

// Storage interface for API operations
type Storage interface {
	ListChildren(ctx context.Context) ([]*core.Child, error)
	GetChild(ctx context.Context, id string) (*core.Child, error)
}

// API handles HTTP requests
type API struct {
	manager SessionManager
	storage Storage
	apiKey  string
	logger  *log.Logger
}

// NewAPI creates a new API instance
func NewAPI(manager SessionManager, storage Storage, apiKey string, logger *log.Logger) *API {
	if logger == nil {
		logger = log.Default()
	}
	return &API{
		manager: manager,
		storage: storage,
		apiKey:  apiKey,
		logger:  logger,
	}
}

// RegisterRoutes registers all API routes
func (a *API) RegisterRoutes(mux *http.ServeMux) {
	// Wrap all handlers with authentication
	mux.Handle("/sessions/tv/start", a.authenticate(http.HandlerFunc(a.handleStartTVSession)))
	mux.Handle("/sessions/", a.authenticate(http.HandlerFunc(a.handleSessionRoutes)))
	mux.Handle("/status", a.authenticate(http.HandlerFunc(a.handleStatus)))
	mux.Handle("/children", a.authenticate(http.HandlerFunc(a.handleChildren)))
	mux.Handle("/children/", a.authenticate(http.HandlerFunc(a.handleChildRoutes)))
}

// authenticate wraps a handler with API key authentication
func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-Metron-Key")
		if apiKey != a.apiKey {
			a.errorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Request/Response types

type StartTVSessionRequest struct {
	ChildIDs []string `json:"child_ids"`
	Minutes  int      `json:"minutes"`
}

type ExtendSessionRequest struct {
	AdditionalMinutes int `json:"additional_minutes"`
}

type SessionResponse struct {
	ID               string   `json:"id"`
	DeviceType       string   `json:"device_type"`
	DeviceID         string   `json:"device_id"`
	ChildIDs         []string `json:"child_ids"`
	StartTime        string   `json:"start_time"`
	ExpectedDuration int      `json:"expected_duration"`
	RemainingMinutes int      `json:"remaining_minutes"`
	Status           string   `json:"status"`
}

type ChildStatusResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	TodayUsed      int    `json:"today_used"`
	TodayRemaining int    `json:"today_remaining"`
	TodayLimit     int    `json:"today_limit"`
	SessionsToday  int    `json:"sessions_today"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Handlers

func (a *API) handleStartTVSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartTVSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.errorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.ChildIDs) == 0 {
		a.errorResponse(w, "At least one child ID is required", http.StatusBadRequest)
		return
	}

	if req.Minutes <= 0 {
		a.errorResponse(w, "Minutes must be positive", http.StatusBadRequest)
		return
	}

	session, err := a.manager.StartSession(r.Context(), "tv", "tv1", req.ChildIDs, req.Minutes)
	if err != nil {
		a.logger.Printf("Error starting session: %v", err)
		a.errorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.jsonResponse(w, toSessionResponse(session), http.StatusCreated)
}

func (a *API) handleSessionRoutes(w http.ResponseWriter, r *http.Request) {
	// Parse URL path: /sessions/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/sessions/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		a.errorResponse(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]

	if len(parts) == 1 {
		// GET /sessions/{id}
		if r.Method == http.MethodGet {
			a.handleGetSession(w, r, sessionID)
			return
		}
		a.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	action := parts[1]

	switch action {
	case "extend":
		if r.Method == http.MethodPost {
			a.handleExtendSession(w, r, sessionID)
			return
		}
	case "stop":
		if r.Method == http.MethodPost {
			a.handleStopSession(w, r, sessionID)
			return
		}
	}

	a.errorResponse(w, "Not found", http.StatusNotFound)
}

func (a *API) handleGetSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, err := a.manager.GetSession(r.Context(), sessionID)
	if err != nil {
		a.logger.Printf("Error getting session: %v", err)
		a.errorResponse(w, "Session not found", http.StatusNotFound)
		return
	}

	a.jsonResponse(w, toSessionResponse(session), http.StatusOK)
}

func (a *API) handleExtendSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req ExtendSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.errorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.AdditionalMinutes <= 0 {
		a.errorResponse(w, "Additional minutes must be positive", http.StatusBadRequest)
		return
	}

	session, err := a.manager.ExtendSession(r.Context(), sessionID, req.AdditionalMinutes)
	if err != nil {
		a.logger.Printf("Error extending session: %v", err)
		a.errorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.jsonResponse(w, toSessionResponse(session), http.StatusOK)
}

func (a *API) handleStopSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if err := a.manager.StopSession(r.Context(), sessionID); err != nil {
		a.logger.Printf("Error stopping session: %v", err)
		a.errorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessions, err := a.manager.ListActiveSessions(r.Context())
	if err != nil {
		a.logger.Printf("Error listing active sessions: %v", err)
		a.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := make([]SessionResponse, 0, len(sessions))
	for _, session := range sessions {
		response = append(response, toSessionResponse(session))
	}

	a.jsonResponse(w, response, http.StatusOK)
}

func (a *API) handleChildren(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	children, err := a.storage.ListChildren(r.Context())
	if err != nil {
		a.logger.Printf("Error listing children: %v", err)
		a.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := make([]map[string]interface{}, 0, len(children))
	for _, child := range children {
		response = append(response, map[string]interface{}{
			"id":            child.ID,
			"name":          child.Name,
			"weekday_limit": child.WeekdayLimit,
			"weekend_limit": child.WeekendLimit,
		})
	}

	a.jsonResponse(w, response, http.StatusOK)
}

func (a *API) handleChildRoutes(w http.ResponseWriter, r *http.Request) {
	// Parse URL path: /children/{id}/status
	path := strings.TrimPrefix(r.URL.Path, "/children/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 || parts[0] == "" {
		a.errorResponse(w, "Invalid path", http.StatusBadRequest)
		return
	}

	childID := parts[0]
	action := parts[1]

	if action == "status" && r.Method == http.MethodGet {
		a.handleChildStatus(w, r, childID)
		return
	}

	a.errorResponse(w, "Not found", http.StatusNotFound)
}

func (a *API) handleChildStatus(w http.ResponseWriter, r *http.Request, childID string) {
	status, err := a.manager.GetChildStatus(r.Context(), childID)
	if err != nil {
		a.logger.Printf("Error getting child status: %v", err)
		a.errorResponse(w, "Child not found", http.StatusNotFound)
		return
	}

	response := ChildStatusResponse{
		ID:             status.Child.ID,
		Name:           status.Child.Name,
		TodayUsed:      status.TodayUsed,
		TodayRemaining: status.TodayRemaining,
		TodayLimit:     status.TodayLimit,
		SessionsToday:  status.SessionsToday,
	}

	a.jsonResponse(w, response, http.StatusOK)
}

// Helper functions

func (a *API) jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (a *API) errorResponse(w http.ResponseWriter, message string, status int) {
	a.jsonResponse(w, ErrorResponse{Error: message}, status)
}

func toSessionResponse(session *core.Session) SessionResponse {
	return SessionResponse{
		ID:               session.ID,
		DeviceType:       session.DeviceType,
		DeviceID:         session.DeviceID,
		ChildIDs:         session.ChildIDs,
		StartTime:        session.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		ExpectedDuration: session.ExpectedDuration,
		RemainingMinutes: session.RemainingMinutes,
		Status:           string(session.Status),
	}
}
