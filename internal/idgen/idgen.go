package idgen

import (
	"github.com/google/uuid"
)

// ID prefixes for different models
const (
	PrefixChild   = "kid_"
	PrefixSession = "sess_"
	PrefixBypass  = "byp_"
)

// NewChild generates a new child ID with kid_ prefix
func NewChild() string {
	return PrefixChild + uuid.New().String()
}

// NewSession generates a new session ID with sess_ prefix
func NewSession() string {
	return PrefixSession + uuid.New().String()
}

// NewBypass generates a new bypass ID with byp_ prefix
func NewBypass() string {
	return PrefixBypass + uuid.New().String()
}

// New generates a generic UUID without prefix (for internal use only)
func New() string {
	return uuid.New().String()
}
