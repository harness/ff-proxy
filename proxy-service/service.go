package proxyservice

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

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
type authTokenFn func(key string) (domain.Token, error)

// clientService is the interface for interacting with the feature flag client service
type clientService interface {
	Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error)
}

// Service is the proxy service implementation
type Service struct {
	logger        log.ContextualLogger
	featureRepo   repository.FeatureFlagRepo
	targetRepo    repository.TargetRepo
	segmentRepo   repository.SegmentRepo
	authFn        authTokenFn
	evaluator     evaluator
	clientService clientService
	offline       bool
}

// NewService creates and returns a ProxyService
func NewService(fr repository.FeatureFlagRepo, tr repository.TargetRepo, sr repository.SegmentRepo, authFn authTokenFn, e evaluator, c clientService, l log.ContextualLogger, offline bool) Service {
	l = l.With("component", "ProxyService")
	return Service{
		logger:        l,
		featureRepo:   fr,
		targetRepo:    tr,
		segmentRepo:   sr,
		authFn:        authFn,
		evaluator:     e,
		clientService: c,
		offline:       offline,
	}
}

// Authenticate performs authentication
func (s Service) Authenticate(ctx context.Context, req domain.AuthRequest) (domain.AuthResponse, error) {
	s.logger = s.logger.With("method", "Authenticate")

	token, err := s.authFn(req.APIKey)
	if err != nil {
		s.logger.Error(ctx, "failed to generate auth token", "err", err)
		return domain.AuthResponse{}, ErrUnauthorised
	}

	// We don't need to bother saving the Target if it's empty
	if reflect.DeepEqual(req.Target, domain.Target{}) {
		return domain.AuthResponse{AuthToken: token.TokenString()}, nil
	}

	envID := token.Claims().Environment
	s.targetRepo.Add(ctx, domain.NewTargetKey(envID), req.Target)

	// if the proxy is running in offline mode we're done, we don't need to bother
	// forwarding the request to FeatureFlags
	if s.offline {
		return domain.AuthResponse{AuthToken: token.TokenString()}, nil
	}

	// We forward the auth request to the client service so that the Target is
	// updated/added in FeatureFlags. Potentially a bold assumption but since the
	// Target is added to the cache above I don't think this is in the critical
	// path so we can call it in a goroutine and just log out if it fails since
	// the proxy will continue to work for the SDKs with this Target. That being
	// said I'm not sure what the long term solution is here if it fails for
	// having Target parity between Feature Flags and the Proxy
	go func() {
		newCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if _, err := s.clientService.Authenticate(newCtx, req.APIKey, req.Target); err != nil {
			s.logger.Error(ctx, "failed to forward Target registration via auth request to client service", "err", err)
		}
		s.logger.Debug(ctx, "successfully registered target with feature flags", "target_identifier", req.Target.Target.Identifier)
	}()

	return domain.AuthResponse{AuthToken: token.TokenString()}, nil
}

// FeatureConfig gets all FeatureConfig for an environment
func (s Service) FeatureConfig(ctx context.Context, req domain.FeatureConfigRequest) ([]domain.FeatureConfig, error) {
	s.logger = s.logger.With("method", "FeatureConfig")

	configs := []domain.FeatureConfig{}
	flagKey := domain.NewFeatureConfigKey(req.EnvironmentID)
	segmentKey := domain.NewSegmentKey(req.EnvironmentID)

	// fetch flags
	flags, err := s.featureRepo.Get(ctx, flagKey)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return []domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return []domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// fetch segments
	segments, err := s.segmentRepo.Get(ctx, segmentKey)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return []domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return []domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// build FeatureConfig
	for _, flag := range flags {
		configs = append(configs, domain.FeatureConfig{
			FeatureFlag: flag,
			Segments:    segmentArrayToMap(segments),
		})
	}

	return configs, nil
}

// FeatureConfigByIdentifier gets the feature config for a feature
func (s Service) FeatureConfigByIdentifier(ctx context.Context, req domain.FeatureConfigByIdentifierRequest) (domain.FeatureConfig, error) {
	s.logger = s.logger.With("method", "FeatureConfigByIdentifier")

	flagKey := domain.NewFeatureConfigKey(req.EnvironmentID)
	segmentKey := domain.NewSegmentKey(req.EnvironmentID)

	// fetch flag
	flag, err := s.featureRepo.GetByIdentifier(ctx, flagKey, req.Identifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// fetch segments
	segments, err := s.segmentRepo.Get(ctx, segmentKey)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// build FeatureConfig
	return domain.FeatureConfig{
		FeatureFlag: flag,
		Segments:    segmentArrayToMap(segments),
	}, nil
}

// TargetSegments gets all of the TargetSegments in an environment
func (s Service) TargetSegments(ctx context.Context, req domain.TargetSegmentsRequest) ([]domain.Segment, error) {
	s.logger = s.logger.With("method", "TargetSegments")

	key := domain.NewSegmentKey(req.EnvironmentID)

	segments, err := s.segmentRepo.Get(ctx, key)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return []domain.Segment{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return []domain.Segment{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return segments, nil
}

// TargetSegmentsByIdentifier get a TargetSegments from an environment by its identifier
func (s Service) TargetSegmentsByIdentifier(ctx context.Context, req domain.TargetSegmentsByIdentifierRequest) (domain.Segment, error) {
	s.logger = s.logger.With("method", "TargetSegmentsByIdentifier")

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
	s.logger = s.logger.With("method", "Evaluations")

	configs := []domain.FeatureConfig{}
	flagKey := domain.NewFeatureConfigKey(req.EnvironmentID)
	targetKey := domain.NewTargetKey(req.EnvironmentID)
	segmentKey := domain.NewSegmentKey(req.EnvironmentID)

	// fetch flags
	flags, err := s.featureRepo.Get(ctx, flagKey)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// fetch segments
	segments, err := s.segmentRepo.Get(ctx, segmentKey)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return nil, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// build FeatureConfig
	for _, flag := range flags {
		configs = append(configs, domain.FeatureConfig{
			FeatureFlag: flag,
			Segments:    segmentArrayToMap(segments),
		})
	}

	// fetch targets
	target, err := s.targetRepo.GetByIdentifier(ctx, targetKey, req.TargetIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return []clientgen.Evaluation{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	evaluations, err := s.evaluator.Evaluate(target, configs...)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return evaluations, nil
}

// EvaluationsByFeature gets all of the evaluations in an environment for a target for a particular feature
func (s Service) EvaluationsByFeature(ctx context.Context, req domain.EvaluationsByFeatureRequest) (clientgen.Evaluation, error) {
	s.logger = s.logger.With("method", "EvaluationsByFeature")

	featureKey := domain.NewFeatureConfigKey(req.EnvironmentID)
	segmentKey := domain.NewSegmentKey(req.EnvironmentID)
	targetKey := domain.NewTargetKey(req.EnvironmentID)

	// fetch feature
	flag, err := s.featureRepo.GetByIdentifier(ctx, featureKey, req.FeatureIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return clientgen.Evaluation{}, ErrNotFound
		}
		return clientgen.Evaluation{}, ErrInternal
	}

	// fetch segment
	segments, err := s.segmentRepo.Get(ctx, segmentKey)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return clientgen.Evaluation{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return clientgen.Evaluation{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// fetch target
	target, err := s.targetRepo.GetByIdentifier(ctx, targetKey, req.TargetIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return clientgen.Evaluation{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return clientgen.Evaluation{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// build FeatureConfig
	config := domain.FeatureConfig{
		FeatureFlag: flag,
		Segments:    segmentArrayToMap(segments),
	}

	evaluations, err := s.evaluator.Evaluate(target, config)
	if err != nil {
		return clientgen.Evaluation{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// This shouldn't happen
	if len(evaluations) != 1 {
		s.logger.Error(ctx, "evaluations should only have a length of one")
		return clientgen.Evaluation{}, ErrInternal
	}

	return evaluations[0], nil
}

// Stream streams flag updates out to the client
func (s Service) Stream(ctx context.Context, req domain.StreamRequest, stream domain.Stream) error {
	s.logger = s.logger.With("method", "Stream")
	return ErrNotImplemented
}

// Metrics forwards metrics to the analytics service
func (s Service) Metrics(ctx context.Context, req domain.MetricsRequest) error {
	s.logger = s.logger.With("method", "Metrics")

	s.logger.Debug(ctx, "got metrics request", "metrics", fmt.Sprintf("%+v", req))
	return ErrNotImplemented
}

func segmentArrayToMap(segments []domain.Segment) map[string]domain.Segment {
	segmentMap := map[string]domain.Segment{}
	for _, segment := range segments {
		segmentMap[segment.Identifier] = segment
	}
	return segmentMap
}
