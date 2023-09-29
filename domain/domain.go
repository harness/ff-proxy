package domain

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
)

// StreamState is the connection state for a stream
type StreamState string

const (
	// StreamStateConnected is the status for when a stream is connected
	StreamStateConnected StreamState = "CONNECTED"
	// StreamStateDisconnected is the status for when a stream is disconnected
	StreamStateDisconnected StreamState = "DISCONNECTED"
	// StreamStateInitializing is the status for when the stream is initialising
	StreamStateInitializing StreamState = "INITIALIZING"
)

// AuthAPIKey is the APIKey type used for authentication lookups
type AuthAPIKey string

// NewAuthAPIKey creates an AuthAPIKey from a key
func NewAuthAPIKey(key string) AuthAPIKey {
	return AuthAPIKey(fmt.Sprintf("auth-key-%s", key))
}

// EnvironmentID is the environment value we store in the cache
type EnvironmentID string

// AuthConfig contains a hashed APIKey and the EnvironmentID it belongs to
type AuthConfig struct {
	APIKey        AuthAPIKey
	EnvironmentID EnvironmentID
}

// MarshalBinary marshals an EnvironmentID to bytes. Currently it uses json marshaling
// but if we want to optimise storage space we could use something more efficient
func (a *EnvironmentID) MarshalBinary() ([]byte, error) {
	return jsoniter.Marshal(a)
}

// UnmarshalBinary unmarshals bytes to an EnvironmentID
func (a *EnvironmentID) UnmarshalBinary(b []byte) error {
	return jsoniter.Unmarshal(b, a)
}

// EnvironmentHealth contains the health info for an environment
type EnvironmentHealth struct {
	ID           string       `json:"id"`
	StreamStatus StreamStatus `json:"streamStatus"`
}

// StreamStatus contains a streams state
type StreamStatus struct {
	State StreamState `json:"state"`
	Since int64       `json:"since"`
}

// ToPtr is a helper func for converting any type to a pointer
func ToPtr[T any](t T) *T {
	return &t
}

func SafePtrDereference[T any](t *T) T {
	if t == nil {
		var d T
		return d
	}
	return *t
}
