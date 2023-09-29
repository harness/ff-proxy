package domain

import (
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

// AuthRequest contains the fields sent in an authentication request
type AuthRequest struct {
	APIKey string
	Target Target
}

// AuthResponse contains the fields returned in an authentication response
type AuthResponse struct {
	AuthToken string `json:"authToken"`
}

// FeatureConfigRequest contains the fields sent in a GET /client/env/{environmentUUID}/feature-configs
type FeatureConfigRequest struct {
	EnvironmentID string
}

// FeatureConfigByIdentifierRequest contains the fields sent in a GET /client/env/{environmentUUID}/feature-configs/{identifier}
type FeatureConfigByIdentifierRequest struct {
	EnvironmentID string
	Identifier    string
}

// TargetSegmentsRequest contains the fields sent in a GET /client/env/{environmentUUID}/target-segments
type TargetSegmentsRequest struct {
	EnvironmentID string
}

// TargetSegmentsByIdentifierRequest contains the fields sent in a GET /client/env/{environmentUUID}/target-segments/{identifier}
type TargetSegmentsByIdentifierRequest struct {
	EnvironmentID string
	Identifier    string
}

// EvaluationsRequest contains the fields sent in a GET /client/env/{environmentUUID}/target/{target}/evaluations
type EvaluationsRequest struct {
	EnvironmentID    string
	TargetIdentifier string
}

// EvaluationsByFeatureRequest contains the fields sent in a GET /client/env/{environmentUUID}/target/{target}/evaluations/{feature} request
type EvaluationsByFeatureRequest struct {
	EnvironmentID     string
	TargetIdentifier  string
	FeatureIdentifier string
}

// StreamRequest contains the fields sent in a GET /stream request
type StreamRequest struct {
	APIKey string `json:"api_key"`
}

// StreamResponse contains the fields returned by a Stream request
type StreamResponse struct {
	GripChannel string
}

// MetricsRequest contains the fields sent in a POST /metrics request
type MetricsRequest struct {
	EnvironmentID string `json:"environment_id"`
	clientgen.Metrics
}

// HealthResponse contains the fields returned in a healthcheck response
type HealthResponse struct {
	Environments []EnvironmentHealth `json:"environments"`
	CacheStatus  string              `json:"cacheStatus"`
}

type GetProxyConfigInput struct {
	Key               string
	EnvID             string
	AuthToken         string
	ClusterIdentifier string
	PageNumber        int
	PageSize          int
}

// AuthenticateProxyKeyResponse is the type returned by AuthenticateProxyKey
type AuthenticateProxyKeyResponse struct {
	Token             string
	ClusterIdentifier string
}
