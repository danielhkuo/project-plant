package auth

import (
	"errors"
	"time"
)

// DeviceIdentity represents an authenticated device.
type DeviceIdentity struct {
	DeviceID  string
	ExpiresAt time.Time // zero value means no expiry
}

// Authenticator validates API keys and returns device identities.
type Authenticator interface {
	Authenticate(apiKey string) (DeviceIdentity, error)
}

var (
	ErrMissingAPIKey = errors.New("missing API key")
	ErrInvalidAPIKey = errors.New("invalid API key")
	ErrExpiredAPIKey = errors.New("API key expired")
)
