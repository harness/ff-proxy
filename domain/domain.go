package domain

import (
	"fmt"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
)

// StreamState is the connection state for a stream
type StreamState string

// String makes StreamState implement the Stringer interface
func (s StreamState) String() string {
	return string(s)
}

type ConfigState string

const (
	// ConfigStateSynced is the status for when proxy has synced successfully
	ConfigStateSynced ConfigState = "SYNCED"
	// ConfigStateFailedToSync is the status for when proxy has failed to perform sync. Indicative of misconfigured key
	ConfigStateFailedToSync ConfigState = "FAILED_TO_SYNC"
	// ConfigStateReadReplica is the status for read replica
	ConfigStateReadReplica ConfigState = "READ_REPLICA"
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

// StreamStatus contains a streams state
type StreamStatus struct {
	State StreamState `json:"state"`
	Since int64       `json:"since"`
}

// NewStreamStatus creates a StreamStatus
func NewStreamStatus() StreamStatus {
	return StreamStatus{
		State: StreamStateInitializing,
		Since: time.Now().UnixMilli(),
	}
}

// ConfigStatus contains a config state
type ConfigStatus struct {
	State ConfigState `json:"state"`
	Since int64       `json:"since"`
}

// NewConfigStatus creates a ConfigStatus for proxy
func NewConfigStatus(status ConfigState) ConfigStatus {
	return ConfigStatus{
		State: status,
		Since: time.Now().UnixMilli(),
	}
}

// MarshalBinary makes StreamStatus implement the BinaryMarshaler interface
func (s *StreamStatus) MarshalBinary() ([]byte, error) {
	return jsoniter.Marshal(s)
}

// UnmarshalBinary makes StreamStatus implement the BinaryUnmarshaler interface
func (s *StreamStatus) UnmarshalBinary(b []byte) error {
	return jsoniter.Unmarshal(b, s)
}

// ToPtr is a helper func for converting any type to a pointer
func ToPtr[T any](t T) *T {
	return &t
}

// SafePtrDereference is a helper for getting the value stored at a pointer
func SafePtrDereference[T any](t *T) T {
	if t == nil {
		var d T
		return d
	}
	return *t
}

// SafeMap is a map of environmentIDs to sdks
type SafeMap struct {
	*sync.RWMutex
	m map[string]interface{}
}

// NewSafeMap creates an SafeMap
func NewSafeMap() *SafeMap {
	return &SafeMap{
		RWMutex: &sync.RWMutex{},
		m:       make(map[string]interface{}),
	}
}

// Set sets a key and value in the map
func (s *SafeMap) Set(key string, value interface{}) {
	s.Lock()
	defer s.Unlock()
	s.m[key] = value
}

// Get returns a copy of the map
func (s *SafeMap) Get() map[string]interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.m
}
