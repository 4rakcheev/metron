package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const ChildIDKey = "child_id"

// ChildSession represents an authenticated child session
type ChildSession struct {
	SessionID string
	ChildID   string
	ExpiresAt time.Time
}

// SessionManager manages child authentication sessions
type SessionManager struct {
	sessions map[string]*ChildSession
	mu       sync.RWMutex
	duration time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*ChildSession),
		duration: 24 * time.Hour, // 24 hour sessions
	}

	// Start background goroutine to clean up expired sessions
	go sm.cleanupExpiredSessions()

	return sm
}

// CreateSession creates a new session for a child
func (sm *SessionManager) CreateSession(childID string) string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate random session ID
	sessionID := generateSessionID()

	session := &ChildSession{
		SessionID: sessionID,
		ChildID:   childID,
		ExpiresAt: time.Now().Add(sm.duration),
	}

	sm.sessions[sessionID] = session

	return sessionID
}

// ValidateSession checks if a session is valid and returns the child ID
func (sm *SessionManager) ValidateSession(sessionID string) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return "", false
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		return "", false
	}

	return session.ChildID, true
}

// DeleteSession removes a session (logout)
func (sm *SessionManager) DeleteSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, sessionID)
}

// cleanupExpiredSessions periodically removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for sessionID, session := range sm.sessions {
			if now.After(session.ExpiresAt) {
				delete(sm.sessions, sessionID)
			}
		}
		sm.mu.Unlock()
	}
}

// generateSessionID generates a secure random session ID
func generateSessionID() string {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID (should never happen)
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(bytes)
}

// ChildAuth is middleware that validates child authentication
func ChildAuth(sm *SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get session ID from cookie first
		sessionID, err := c.Cookie("child_session")

		// If not in cookie, try Authorization header
		if err != nil || sessionID == "" {
			authHeader := c.GetHeader("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				sessionID = authHeader[7:]
			}
		}

		// Validate session ID
		if sessionID == "" {
			c.JSON(401, gin.H{
				"error": "Authentication required",
				"code":  "MISSING_SESSION",
			})
			c.Abort()
			return
		}

		childID, valid := sm.ValidateSession(sessionID)
		if !valid {
			c.JSON(401, gin.H{
				"error": "Invalid or expired session",
				"code":  "INVALID_SESSION",
			})
			c.Abort()
			return
		}

		// Store child ID in context
		c.Set(ChildIDKey, childID)
		c.Next()
	}
}

// GetChildID retrieves the authenticated child ID from context
func GetChildID(c *gin.Context) (string, bool) {
	childID, exists := c.Get(ChildIDKey)
	if !exists {
		return "", false
	}
	return childID.(string), true
}
