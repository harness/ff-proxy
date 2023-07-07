package domain

import (
	"fmt"

	"github.com/harness/ff-golang-server-sdk/rest"
	jsoniter "github.com/json-iterator/go"

	"github.com/golang-jwt/jwt/v4"
	admingen "github.com/harness/ff-proxy/gen/admin"
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

// FeatureFlagKey is the key that maps to a FeatureConfig
type FeatureFlagKey string

// NewFeatureConfigKey creates a FeatureFlagKey from an environment and identifier
func NewFeatureConfigKey(envID string, identifier string) FeatureFlagKey {
	return FeatureFlagKey(fmt.Sprintf("env-%s-feature-config-%s", envID, identifier))
}

// NewFeatureConfigsKey creates a FeatureFlagKey from and environmet
func NewFeatureConfigsKey(envID string) FeatureFlagKey {
	return FeatureFlagKey(fmt.Sprintf("env-%s-feature-configs", envID))
}

// FeatureFlag stores feature flag data
type FeatureFlag rest.FeatureConfig

// FeatureConfig is the type containing FeatureConfig information and is what
// we return from /GET client/env/<env>/feature-configs
type FeatureConfig struct {
	FeatureFlag
}

// MarshalBinary marshals a FeatureFlag to bytes. Currently it just uses json
// marshaling but if we want to optimise storage space we could use something
// more efficient
func (f *FeatureFlag) MarshalBinary() ([]byte, error) {
	return jsoniter.Marshal(f)
}

// UnmarshalBinary unmarshals bytes to a FeatureFlag
func (f *FeatureFlag) UnmarshalBinary(b []byte) error {
	return jsoniter.Unmarshal(b, f)
}

// TargetKey is the key that maps to a Target
type TargetKey string

// NewTargetsKey creates a TargetKey from an environment
func NewTargetsKey(envID string) TargetKey {
	return TargetKey(fmt.Sprintf("env-%s-target-configs", envID))
}

// NewTargetKey creates a TargetKey from an environment and identifier
func NewTargetKey(envID string, identifier string) TargetKey {
	return TargetKey(fmt.Sprintf("env-%s-target-config-%s", envID, identifier))
}

// Target is a admingen.Target that we can declare methods on
type Target struct {
	admingen.Target
}

// MarshalBinary marshals a Target to bytes. Currently it uses json marshaling
// but if we want to optimise storage space we could use something more efficient
func (t *Target) MarshalBinary() ([]byte, error) {
	return jsoniter.Marshal(t)
}

// UnmarshalBinary unmarshals bytes to a Target
func (t *Target) UnmarshalBinary(b []byte) error {
	return jsoniter.Unmarshal(b, t)
}

// SegmentKey is the key that maps to a Segment
type SegmentKey string

// NewSegmentKey creates a SegmentKey from an environment and identifier
func NewSegmentKey(envID string, identifier string) SegmentKey {
	return SegmentKey(fmt.Sprintf("env-%s-segment-%s", envID, identifier))
}

// NewSegmentsKey creates a SegmentKey from an environment
func NewSegmentsKey(envID string) SegmentKey {
	return SegmentKey(fmt.Sprintf("env-%s-segments", envID))
}

// Segment is a rest.Segment that we can declare methods on
type Segment rest.Segment

// MarshalBinary marshals a Segment to bytes. Currently it uses json marshaling
// but if we want to optimise storage space we could use something more efficient
func (s *Segment) MarshalBinary() ([]byte, error) {
	return jsoniter.Marshal(s)
}

// UnmarshalBinary unmarshals bytes to a Segment
func (s *Segment) UnmarshalBinary(b []byte) error {
	return jsoniter.Unmarshal(b, s)
}

// AuthAPIKey is the APIKey type used for authentication lookups
type AuthAPIKey string

// NewAuthAPIKey creates an AuthAPIKey from a key
func NewAuthAPIKey(key string) AuthAPIKey {
	return AuthAPIKey(fmt.Sprintf("auth-key-%s", key))
}

// Token is a type that contains a generated token string and the claims
type Token struct {
	token  string
	claims Claims
}

// NewToken creates a new token
func NewToken(tokenString string, claims Claims) Token {
	return Token{token: tokenString, claims: claims}
}

// TokenString returns the auth token string
func (t Token) TokenString() string {
	return t.token
}

// Claims returns the tokens claims
func (t Token) Claims() Claims {
	return t.claims
}

// Claims are custom jwt claims used by the proxy for generating a jwt token
type Claims struct {
	Environment       string `json:"environment"`
	ClusterIdentifier string `json:"clusterIdentifier"`
	jwt.StandardClaims
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
