package auth

import "time"

// StaticKeyAuthenticator validates API keys against a static map.
// Suitable for development and testing; swap for a DB-backed
// implementation in production with zero handler changes.
type StaticKeyAuthenticator struct {
	keys map[string]DeviceIdentity
}

// NewStaticKeyAuthenticator creates an authenticator with the given key-to-identity mappings.
func NewStaticKeyAuthenticator(keys map[string]DeviceIdentity) *StaticKeyAuthenticator {
	return &StaticKeyAuthenticator{keys: keys}
}

// Authenticate validates the API key and returns the associated device identity.
func (a *StaticKeyAuthenticator) Authenticate(apiKey string) (DeviceIdentity, error) {
	if apiKey == "" {
		return DeviceIdentity{}, ErrMissingAPIKey
	}

	identity, ok := a.keys[apiKey]
	if !ok {
		return DeviceIdentity{}, ErrInvalidAPIKey
	}

	if !identity.ExpiresAt.IsZero() && time.Now().After(identity.ExpiresAt) {
		return DeviceIdentity{}, ErrExpiredAPIKey
	}

	return identity, nil
}
