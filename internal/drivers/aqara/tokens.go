package aqara

import (
	"context"
	"time"
)

// AqaraTokens represents the Aqara Cloud API tokens
type AqaraTokens struct {
	RefreshToken         string
	AccessToken          string
	AccessTokenExpiresAt *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AqaraTokenStorage defines the interface for Aqara token persistence
// This interface is implemented by the storage layer to avoid tight coupling
type AqaraTokenStorage interface {
	GetAqaraTokens(ctx context.Context) (*AqaraTokens, error)
	SaveAqaraTokens(ctx context.Context, tokens *AqaraTokens) error
}
