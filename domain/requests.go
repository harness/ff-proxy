package domain

// AuthRequest contains the fields sent in an authentication request
type AuthRequest struct {
	APIKey string
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
