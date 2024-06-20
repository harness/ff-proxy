package domain

import (
	"context"
	"fmt"
	"net/http"

	"github.com/harness/ff-proxy/v2/build"
	jsoniter "github.com/json-iterator/go"

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
	Rules         string
}

// TargetSegmentsByIdentifierRequest contains the fields sent in a GET /client/env/{environmentUUID}/target-segments/{identifier}
type TargetSegmentsByIdentifierRequest struct {
	EnvironmentID string
	Identifier    string
	Rules         string
}

// EvaluationsRequest contains the fields sent in a GET /client/env/{environmentUUID}/target/{target}/evaluations
type EvaluationsRequest struct {
	EnvironmentID    string
	TargetIdentifier string

	// Target is an optional field that will be populated if the client provides a 'Harness-Target' header in the request
	Target *Target
}

// EvaluationsByFeatureRequest contains the fields sent in a GET /client/env/{environmentUUID}/target/{target}/evaluations/{feature} request
type EvaluationsByFeatureRequest struct {
	EnvironmentID     string
	TargetIdentifier  string
	FeatureIdentifier string

	// Target is an optional field that will be populated if the client provides a 'Harness-Target' header in the request
	Target *Target
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
	// Size is only used internally by the Proxy so we don't want to include it in any JSON requests/responses
	Size          int    `json:"-"`
	EnvironmentID string `json:"environment_id"`
	clientgen.Metrics
}

// MarshalBinary makes MetricsRequest implement the encoding.BinaryMarshaler func
func (m *MetricsRequest) MarshalBinary() (data []byte, err error) {
	return jsoniter.Marshal(m)
}

// HealthResponse contains the fields returned in a healthcheck response
type HealthResponse struct {
	ConfigStatus ConfigStatus `json:"configStatus"`
	StreamStatus StreamStatus `json:"streamStatus"`
	CacheStatus  string       `json:"cacheStatus"`
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

func AddHarnessXHeaders(envID string) func(ctx context.Context, req *http.Request) error {
	return func(ctx context.Context, req *http.Request) error {
		accountID := ctx.Value(ContextKeyAccountID).(string)

		req.Header.Set("Harness-Accountid", accountID)
		req.Header.Set("Harness-Environmentid", envID)
		req.Header.Set("Harness-Sdk-Info", fmt.Sprintf("Proxy %s", build.Version))
		return nil
	}
}
