package proxyservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/gen"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/repository"
)

var (
	// ErrNotImplemented is the error returned when a method hasn't been implemented
	ErrNotImplemented = errors.New("endpoint not implemented")

	// ErrNotFound is the error returned when the service can't find the requested
	// resource
	ErrNotFound = errors.New("not found")

	// ErrInternal is the error that the proxy service returns when it encounters
	// an unexpected error
	ErrInternal = errors.New("internal error")
)

// evaluator is a type that can perform evaluations
type evaluator interface {
	// Evaluate evaluates featureConfig(s) against a target and returns an evaluation
	Evaluate(target domain.Target, featureConfigs ...domain.FeatureConfig) ([]gen.Evaluation, error)
}

// ProxyService is the proxy service implementation
type ProxyService struct {
	logger      log.Logger
	featureRepo repository.FeatureConfigRepo
	targetRepo  repository.TargetRepo
	segmentRepo repository.SegmentRepo
	evaluator   evaluator
}

// NewProxyService creates and returns a ProxyService
func NewProxyService(fr repository.FeatureConfigRepo, tr repository.TargetRepo, sr repository.SegmentRepo, e evaluator, l log.Logger) ProxyService {
	l = log.With(l, "component", "ProxyService")
	return ProxyService{
		logger:      l,
		featureRepo: fr,
		targetRepo:  tr,
		segmentRepo: sr,
		evaluator:   e,
	}
}

// Authenticate performs authentication
func (p ProxyService) Authenticate(ctx context.Context, req domain.AuthRequest) (domain.AuthResponse, error) {
	// For now just return a hardcoded token
	return domain.AuthResponse{AuthToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbnZpcm9ubWVudCI6Ijk0ZWY3MzYxLTFmMmQtNDBhZi05YjJjLWMxMTQ1ZDUzN2U1YSIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImZlYXR1cmVmbGFnc3FhIiwicHJvamVjdCI6IjAyZjM3ZDQ4LTgzOWUtNGM1Yi04YzJhLTU4NjljMzFlNDAwNSIsInByb2plY3RJZGVudGlmaWVyIjoiRmVhdHVyZUZsYWdzUUFEZW1vIiwiYWNjb3VudElEIjoiekVhYWstRkxTNDI1SUVPN09Mek1VZyIsIm9yZ2FuaXphdGlvbiI6ImZiNGUzZDc2LTRlZDEtNDAxOS1hNjc4LWU0YWJkN2EyNGViOSIsIm9yZ2FuaXphdGlvbklkZW50aWZpZXIiOiJmZWF0dXJlZmxhZ29yZyIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMSIsImtleV90eXBlIjoiU2VydmVyIn0.snL7AesKMCM99fjaZIRu3JzLzTBDddU-UuYOzyq_8Qo"}, nil
}

// FeatureConfig gets all FeatureConfig for an environment
func (p ProxyService) FeatureConfig(ctx context.Context, req domain.FeatureConfigRequest) ([]domain.FeatureConfig, error) {
	key := domain.NewFeatureConfigKey(req.EnvironmentID)

	configs, err := p.featureRepo.Get(ctx, key)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return []domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return []domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return configs, nil
}

// FeatureConfigByIdentifier gets the feature config for a feature
func (p ProxyService) FeatureConfigByIdentifier(ctx context.Context, req domain.FeatureConfigByIdentifierRequest) (domain.FeatureConfig, error) {
	key := domain.NewFeatureConfigKey(req.EnvironmentID)

	config, err := p.featureRepo.GetByIdentifier(ctx, key, req.Identifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return config, nil
}

// TargetSegments gets all of the TargetSegments in an environment
func (p ProxyService) TargetSegments(ctx context.Context, req domain.TargetSegmentsRequest) ([]domain.Segment, error) {
	key := domain.NewSegmentKey(req.EnvironmentID)

	segments, err := p.segmentRepo.Get(ctx, key)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return []domain.Segment{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return []domain.Segment{}, fmt.Errorf("%w: %s", ErrInternal, err)

	}

	return segments, nil
}

// TargetSegmentsByIdentifier get a TargetSegments from an environment by its identifier
func (p ProxyService) TargetSegmentsByIdentifier(ctx context.Context, req domain.TargetSegmentsByIdentifierRequest) (domain.Segment, error) {
	key := domain.NewSegmentKey(req.EnvironmentID)

	segment, err := p.segmentRepo.GetByIdentifier(ctx, key, req.Identifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return domain.Segment{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return domain.Segment{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return segment, nil
}

// Evaluations gets all of the evaluations in an environment for a target
func (p ProxyService) Evaluations(ctx context.Context, req domain.EvaluationsRequest) ([]gen.Evaluation, error) {
	featureConfigKey := domain.NewFeatureConfigKey(req.EnvironmentID)
	targetKey := domain.NewTargetKey(req.EnvironmentID)

	configs, err := p.featureRepo.Get(ctx, featureConfigKey)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	target, err := p.targetRepo.GetByIdentifier(ctx, targetKey, req.TargetIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	evaluations, err := p.evaluator.Evaluate(target, configs...)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return evaluations, nil
}

// EvaluationsByFeature gets all of the evaluations in an environment for a target for a particular feature
func (p ProxyService) EvaluationsByFeature(ctx context.Context, req domain.EvaluationsByFeatureRequest) (gen.Evaluation, error) {
	featureKey := domain.NewFeatureConfigKey(req.EnvironmentID)
	targetKey := domain.NewTargetKey(req.EnvironmentID)

	config, err := p.featureRepo.GetByIdentifier(ctx, featureKey, req.FeatureIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return gen.Evaluation{}, ErrNotFound
		}
		return gen.Evaluation{}, ErrInternal
	}

	target, err := p.targetRepo.GetByIdentifier(ctx, targetKey, req.TargetIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return gen.Evaluation{}, ErrNotFound
		}
		return gen.Evaluation{}, ErrInternal
	}

	evaluations, err := p.evaluator.Evaluate(target, config)
	if err != nil {
		return gen.Evaluation{}, ErrInternal
	}

	// This shouldn't happen
	if len(evaluations) != 1 {
		p.logger.Error("msg", "evaluations should only have a length of one")
		return gen.Evaluation{}, ErrInternal
	}

	return evaluations[0], nil
}

// Stream streams flag updates out to the client
func (p ProxyService) Stream(ctx context.Context, req domain.StreamRequest, stream domain.Stream) error {
	return ErrNotImplemented
}

// Metrics forwards metrics to the analytics service
func (p ProxyService) Metrics(ctx context.Context, req domain.MetricsRequest) error {
	p.logger.Debug("msg", "got metrics request", "metrics", fmt.Sprintf("%+v", req))
	return ErrNotImplemented
}
