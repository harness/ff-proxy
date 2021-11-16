package proxyservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/repository"
)

//ProxyService is the interface for the ProxyService
type ProxyService interface {
	// Authenticate performs authentication
	Authenticate(ctx context.Context, req domain.AuthRequest) (domain.AuthResponse, error)

	// FeatureConfig gets all FeatureConfig for an environment
	FeatureConfig(ctx context.Context, req domain.FeatureConfigRequest) ([]domain.FeatureConfig, error)

	// FeatureConfigByIdentifier gets the feature config for a feature
	FeatureConfigByIdentifier(ctx context.Context, req domain.FeatureConfigByIdentifierRequest) (domain.FeatureConfig, error)

	// TargetSegments gets all of the TargetSegments in an environment
	TargetSegments(ctx context.Context, req domain.TargetSegmentsRequest) ([]domain.Segment, error)

	// TargetSegmentsByIdentifier get a TargetSegments from an environment by its identifier
	TargetSegmentsByIdentifier(ctx context.Context, req domain.TargetSegmentsByIdentifierRequest) (domain.Segment, error)

	// Evaluations gets all of the evaluations in an environment for a target
	Evaluations(ctx context.Context, req domain.EvaluationsRequest) ([]clientgen.Evaluation, error)

	// Evaluations gets all of the evaluations in an environment for a target for a particular feature
	EvaluationsByFeature(ctx context.Context, req domain.EvaluationsByFeatureRequest) (clientgen.Evaluation, error)

	// Stream streams flag updates out to the client
	Stream(ctx context.Context, req domain.StreamRequest, stream domain.Stream) error

	// Metrics forwards metrics to the analytics service
	Metrics(ctx context.Context, req domain.MetricsRequest) error
}

var (
	// ErrNotImplemented is the error returned when a method hasn't been implemented
	ErrNotImplemented = errors.New("endpoint not implemented")

	// ErrNotFound is the error returned when the service can't find the requested
	// resource
	ErrNotFound = errors.New("not found")

	// ErrInternal is the error that the proxy service returns when it encounters
	// an unexpected error
	ErrInternal = errors.New("internal error")

	// ErrUnauthorised is the error that the proxy service returns when the
	ErrUnauthorised = errors.New("unauthorised")
)

// evaluator is a type that can perform evaluations
type evaluator interface {
	// Evaluate evaluates featureConfig(s) against a target and returns an evaluation
	Evaluate(target domain.Target, featureConfigs ...domain.FeatureConfig) ([]clientgen.Evaluation, error)
}

// authTokenFn is a function that can generate an auth token
type authTokenFn func(key string) (string, error)

// Service is the proxy service implementation
type Service struct {
	logger      log.Logger
	featureRepo repository.FeatureConfigRepo
	targetRepo  repository.TargetRepo
	segmentRepo repository.SegmentRepo
	authFn      authTokenFn
	evaluator   evaluator
}

// NewService creates and returns a ProxyService
func NewService(fr repository.FeatureConfigRepo, tr repository.TargetRepo, sr repository.SegmentRepo, authFn authTokenFn, e evaluator, l log.Logger) Service {
	l = log.With(l, "component", "ProxyService")
	return Service{
		logger:      l,
		featureRepo: fr,
		targetRepo:  tr,
		segmentRepo: sr,
		authFn:      authFn,
		evaluator:   e,
	}
}

// Authenticate performs authentication
func (s Service) Authenticate(ctx context.Context, req domain.AuthRequest) (domain.AuthResponse, error) {
	token, err := s.authFn(req.APIKey)
	if err != nil {
		s.logger.Error("msg", "failed to generate auth token", "err", err)
		return domain.AuthResponse{}, ErrUnauthorised
	}

	return domain.AuthResponse{AuthToken: token}, nil
}

// FeatureConfig gets all FeatureConfig for an environment
func (s Service) FeatureConfig(ctx context.Context, req domain.FeatureConfigRequest) ([]domain.FeatureConfig, error) {
	key := domain.NewFeatureConfigKey(req.EnvironmentID)

	configs, err := s.featureRepo.Get(ctx, key)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return []domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return []domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return configs, nil
}

// FeatureConfigByIdentifier gets the feature config for a feature
func (s Service) FeatureConfigByIdentifier(ctx context.Context, req domain.FeatureConfigByIdentifierRequest) (domain.FeatureConfig, error) {
	key := domain.NewFeatureConfigKey(req.EnvironmentID)

	config, err := s.featureRepo.GetByIdentifier(ctx, key, req.Identifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return config, nil
}

// TargetSegments gets all of the TargetSegments in an environment
func (s Service) TargetSegments(ctx context.Context, req domain.TargetSegmentsRequest) ([]domain.Segment, error) {
	key := domain.NewSegmentKey(req.EnvironmentID)

	segments, err := s.segmentRepo.Get(ctx, key)
	if err != nil {
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return []domain.Segment{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		s.logger.Debug("Segments not found: Continue with empty segments")
		return []domain.Segment{}, nil

	}

	return segments, nil
}

// TargetSegmentsByIdentifier get a TargetSegments from an environment by its identifier
func (s Service) TargetSegmentsByIdentifier(ctx context.Context, req domain.TargetSegmentsByIdentifierRequest) (domain.Segment, error) {
	key := domain.NewSegmentKey(req.EnvironmentID)

	segment, err := s.segmentRepo.GetByIdentifier(ctx, key, req.Identifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return domain.Segment{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return domain.Segment{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return segment, nil
}

// Evaluations gets all of the evaluations in an environment for a target
func (s Service) Evaluations(ctx context.Context, req domain.EvaluationsRequest) ([]clientgen.Evaluation, error) {
	featureConfigKey := domain.NewFeatureConfigKey(req.EnvironmentID)
	targetKey := domain.NewTargetKey(req.EnvironmentID)

	configs, err := s.featureRepo.Get(ctx, featureConfigKey)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	target, err := s.targetRepo.GetByIdentifier(ctx, targetKey, req.TargetIdentifier)
	if err != nil {
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrInternal, err)
		}
		s.logger.Debug("Target not found: Continue with empty target")
	}

	evaluations, err := s.evaluator.Evaluate(target, configs...)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return evaluations, nil
}

// EvaluationsByFeature gets all of the evaluations in an environment for a target for a particular feature
func (s Service) EvaluationsByFeature(ctx context.Context, req domain.EvaluationsByFeatureRequest) (clientgen.Evaluation, error) {
	featureKey := domain.NewFeatureConfigKey(req.EnvironmentID)
	targetKey := domain.NewTargetKey(req.EnvironmentID)

	config, err := s.featureRepo.GetByIdentifier(ctx, featureKey, req.FeatureIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return clientgen.Evaluation{}, ErrNotFound
		}
		return clientgen.Evaluation{}, ErrInternal
	}

	target, err := s.targetRepo.GetByIdentifier(ctx, targetKey, req.TargetIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return clientgen.Evaluation{}, ErrNotFound
		}
		return clientgen.Evaluation{}, ErrInternal
	}

	evaluations, err := s.evaluator.Evaluate(target, config)
	if err != nil {
		return clientgen.Evaluation{}, ErrInternal
	}

	// This shouldn't happen
	if len(evaluations) != 1 {
		s.logger.Error("msg", "evaluations should only have a length of one")
		return clientgen.Evaluation{}, ErrInternal
	}

	return evaluations[0], nil
}

// Stream streams flag updates out to the client
func (s Service) Stream(ctx context.Context, req domain.StreamRequest, stream domain.Stream) error {
	return ErrNotImplemented
}

// Metrics forwards metrics to the analytics service
func (s Service) Metrics(ctx context.Context, req domain.MetricsRequest) error {
	s.logger.Debug("msg", "got metrics request", "metrics", fmt.Sprintf("%+v", req))
	return ErrNotImplemented
}
