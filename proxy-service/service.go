package proxyservice

import (
	"context"
	"errors"

	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/gen"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/repository"
)

var (
	// ErrNotImplemented is the error returned when a method hasn't been implemented
	ErrNotImplemented = errors.New("endpoint not implemented")
)

// ProxyService is the proxy service implementation
type ProxyService struct {
	logger      log.Logger
	featureRepo repository.FeatureConfigRepo
	targetRepo  repository.TargetRepo
}

// NewProxyService creates and returns a ProxyService
func NewProxyService(fr repository.FeatureConfigRepo, tr repository.TargetRepo, l log.Logger) ProxyService {
	l = log.With(l, "component", "ProxyService")
	return ProxyService{logger: l, featureRepo: fr, targetRepo: tr}
}

// Authenticate performs authentication
func (p ProxyService) Authenticate(ctx context.Context, req domain.AuthRequest) (domain.AuthResponse, error) {
	return domain.AuthResponse{}, ErrNotImplemented
}

// FeatureConfig gets all FeatureConfig for an environment
func (p ProxyService) FeatureConfig(ctx context.Context, req domain.FeatureConfigRequest) ([]domain.FeatureConfig, error) {
	return []domain.FeatureConfig{}, ErrNotImplemented
}

// FeatureConfigByIdentifier gets the feature config for a feature
func (p ProxyService) FeatureConfigByIdentifier(ctx context.Context, req domain.FeatureConfigByIdentifierRequest) (domain.FeatureConfig, error) {
	return domain.FeatureConfig{}, ErrNotImplemented
}

// TargetSegments gets all of the TargetSegments in an environment
func (p ProxyService) TargetSegments(ctx context.Context, req domain.TargetSegmentsRequest) ([]gen.Segment, error) {
	return []gen.Segment{}, ErrNotImplemented
}

// TargetSegmentsByIdentifier get a TargetSegments from an environment by its identifier
func (p ProxyService) TargetSegmentsByIdentifier(ctx context.Context, req domain.TargetSegmentsByIdentifierRequest) (gen.Segment, error) {
	return gen.Segment{}, ErrNotImplemented
}

// Evaluations gets all of the evaluations in an environment for a target
func (p ProxyService) Evaluations(ctx context.Context, req domain.EvaluationsRequest) ([]gen.Evaluation, error) {
	return []gen.Evaluation{}, ErrNotImplemented
}

// EvaluationsByFeature gets all of the evaluations in an environment for a target for a particular feature
func (p ProxyService) EvaluationsByFeature(ctx context.Context, req domain.EvaluationsByFeatureRequest) (gen.Evaluation, error) {
	return gen.Evaluation{}, ErrNotImplemented
}

// Stream streams flag updates out to the client
func (p ProxyService) Stream(ctx context.Context, req domain.StreamRequest, stream domain.Stream) error {
	return ErrNotImplemented
}
