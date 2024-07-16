package domain

const (
	AuthRoute                     = "/client/auth"
	HealthRoute                   = "/health"
	FeatureConfigsRoute           = "/client/env/:environment_uuid/feature-configs"
	FeatureConfigsIdentifierRoute = "/client/env/:environment_uuid/feature-configs/:identifier"
	SegmentsRoute                 = "/client/env/:environment_uuid/target-segments"
	SegmentsIdentifierRoute       = "/client/env/:environment_uuid/target-segments/:identifier"
	EvaluationsRoute              = "/client/env/:environment_uuid/target/:target/evaluations"
	EvaluationsFlagRoute          = "/client/env/:environment_uuid/target/:target/evaluations/:feature"
	StreamRoute                   = "/stream"
	MetricsRoute                  = "/metrics/:environment_uuid"
)
