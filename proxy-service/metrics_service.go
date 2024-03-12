package proxyservice

import (
	"context"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

type MetricsService struct {
}

func (m MetricsService) Authenticate(ctx context.Context, req domain.AuthRequest) (domain.AuthResponse, error) {
	return domain.AuthResponse{}, ErrNotImplemented
}

func (m MetricsService) FeatureConfig(ctx context.Context, req domain.FeatureConfigRequest) ([]domain.FeatureConfig, error) {
	return []domain.FeatureConfig{}, ErrNotImplemented
}

func (m MetricsService) FeatureConfigByIdentifier(ctx context.Context, req domain.FeatureConfigByIdentifierRequest) (domain.FeatureConfig, error) {
	return domain.FeatureConfig{}, ErrNotImplemented
}

func (m MetricsService) TargetSegments(ctx context.Context, req domain.TargetSegmentsRequest) ([]domain.Segment, error) {
	return []domain.Segment{}, ErrNotImplemented
}

func (m MetricsService) TargetSegmentsByIdentifier(ctx context.Context, req domain.TargetSegmentsByIdentifierRequest) (domain.Segment, error) {
	return domain.Segment{}, ErrNotImplemented
}

func (m MetricsService) Evaluations(ctx context.Context, req domain.EvaluationsRequest) ([]clientgen.Evaluation, error) {
	return []clientgen.Evaluation{}, ErrNotImplemented
}

func (m MetricsService) EvaluationsByFeature(ctx context.Context, req domain.EvaluationsByFeatureRequest) (clientgen.Evaluation, error) {
	return clientgen.Evaluation{}, ErrNotImplemented
}

func (m MetricsService) Stream(ctx context.Context, req domain.StreamRequest) (domain.StreamResponse, error) {
	return domain.StreamResponse{}, nil
}

func (m MetricsService) Metrics(ctx context.Context, req domain.MetricsRequest) error {
	return ErrNotImplemented
}

func (m MetricsService) Health(ctx context.Context) (domain.HealthResponse, error) {
	return domain.HealthResponse{
		ConfigStatus: domain.ConfigStatus{},
		StreamStatus: domain.StreamStatus{},
		CacheStatus:  "",
	}, nil
}
