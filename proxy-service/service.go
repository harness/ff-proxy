package proxyservice

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/harness/ff-golang-server-sdk/evaluation"
	"github.com/harness/ff-golang-server-sdk/logger"
	"github.com/harness/ff-golang-server-sdk/rest"
	jsoniter "github.com/json-iterator/go"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/hash"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/harness/ff-proxy/v2/repository"
)

// ProxyService is the interface for the ProxyService
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

	// Stream returns the name of the GripChannel that the client should subscribe to
	Stream(ctx context.Context, req domain.StreamRequest) (domain.StreamResponse, error)

	// Metrics forwards metrics to the analytics service
	Metrics(ctx context.Context, req domain.MetricsRequest) error

	// Health checks the health of the system
	Health(ctx context.Context) (domain.HealthResponse, error)
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

	// ErrStreamDisconnected is the error that the proxy service returns when
	// its internal sdk has disconnected from the SaaS stream
	ErrStreamDisconnected = errors.New("SaaS stream disconnected")
)

// authTokenFn is a function that can generate an auth token
type authTokenFn func(key string) (domain.Token, error)

// CacheHealthFn is a function that checks the cache health
type CacheHealthFn func(ctx context.Context) error

// EnvHealthFn is a function that checks the health of all connected environments
type EnvHealthFn func() []domain.EnvironmentHealth

// clientService is the interface for interacting with the feature flag client service
type clientService interface {
	Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error)
}

// metricService is the interface for interacting with the feature flag metric service
type metricService interface {
	StoreMetrics(ctx context.Context, metrics domain.MetricsRequest) error
}

// SDKClients is an interface that can be used to find out if internal sdks are connected to the SaaS FF stream
type SDKClients interface {
	StreamConnected(key string) bool
}

// Config is the config for a Service
type Config struct {
	Logger           log.ContextualLogger
	FeatureRepo      repository.FeatureFlagRepo
	TargetRepo       repository.TargetRepo
	SegmentRepo      repository.SegmentRepo
	AuthRepo         repository.AuthRepo
	CacheHealthFn    CacheHealthFn
	AuthFn           authTokenFn
	ClientService    clientService
	MetricService    metricService
	Offline          bool
	Hasher           hash.Hasher
	StreamingEnabled bool
}

// Service is the proxy service implementation
type Service struct {
	logger           log.ContextualLogger
	featureRepo      repository.FeatureFlagRepo
	targetRepo       repository.TargetRepo
	segmentRepo      repository.SegmentRepo
	authRepo         repository.AuthRepo
	cacheHealthFn    CacheHealthFn
	authFn           authTokenFn
	clientService    clientService
	metricService    metricService
	offline          bool
	hasher           hash.Hasher
	streamingEnabled bool
	sdkClients       SDKClients
}

// NewService creates and returns a ProxyService
func NewService(c Config) Service {
	l := c.Logger.With("component", "ProxyService")
	return Service{
		logger:           l,
		featureRepo:      c.FeatureRepo,
		targetRepo:       c.TargetRepo,
		segmentRepo:      c.SegmentRepo,
		authRepo:         c.AuthRepo,
		cacheHealthFn:    c.CacheHealthFn,
		authFn:           c.AuthFn,
		clientService:    c.ClientService,
		metricService:    c.MetricService,
		offline:          c.Offline,
		hasher:           c.Hasher,
		streamingEnabled: c.StreamingEnabled,
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
	if err := s.targetRepo.DeltaAdd(ctx, envID, req.Target); err != nil {
		s.logger.Info(ctx, "failed to save target during auth", "err", err)
	}

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

	// fetch flags
	flags, err := s.featureRepo.Get(ctx, req.EnvironmentID)
	if err != nil {
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return []domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
		}
		// we don't return not found because we can't currently tell the difference between no features existing
		// and the environment itself not existing
		s.logger.Debug(ctx, "flags not found in cache: ", "err", err.Error())
	}

	configs := make([]domain.FeatureConfig, 0, len(flags))

	// build FeatureConfig
	emptyVariationMap := []clientgen.VariationMap{}
	for _, flag := range flags {

		// some sdks e.g. .NET don't cope well with being returned a null VariationToTargetMap so we send back an empty struct here for now
		// to match ff-server behaviour
		if flag.VariationToTargetMap == nil {
			flag.VariationToTargetMap = &emptyVariationMap
		}

		configs = append(configs, domain.FeatureConfig{
			FeatureFlag: flag,
		})
	}

	return configs, nil
}

// FeatureConfigByIdentifier gets the feature config for a feature
func (s Service) FeatureConfigByIdentifier(ctx context.Context, req domain.FeatureConfigByIdentifierRequest) (domain.FeatureConfig, error) {
	s.logger = s.logger.With("method", "FeatureConfigByIdentifier")

	// fetch flag
	flag, err := s.featureRepo.GetByIdentifier(ctx, req.EnvironmentID, req.Identifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return domain.FeatureConfig{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	// build FeatureConfig
	return domain.FeatureConfig{
		FeatureFlag: flag,
	}, nil
}

// TargetSegments gets all of the TargetSegments in an environment
func (s Service) TargetSegments(ctx context.Context, req domain.TargetSegmentsRequest) ([]domain.Segment, error) {
	s.logger = s.logger.With("method", "TargetSegments")

	segments, err := s.segmentRepo.Get(ctx, req.EnvironmentID)
	if err != nil {
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return []domain.Segment{}, fmt.Errorf("%w: %s", ErrInternal, err)
		}
		// we don't return not found because we can't currently tell the difference between no segments existing
		// and the environment itself not existing
		s.logger.Debug(ctx, "target segments not found in cache: ", "err", err.Error())
	}

	return segments, nil
}

// TargetSegmentsByIdentifier get a TargetSegments from an environment by its identifier
func (s Service) TargetSegmentsByIdentifier(ctx context.Context, req domain.TargetSegmentsByIdentifierRequest) (domain.Segment, error) {
	s.logger = s.logger.With("method", "TargetSegmentsByIdentifier")

	segment, err := s.segmentRepo.GetByIdentifier(ctx, req.EnvironmentID, req.Identifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			return domain.Segment{}, fmt.Errorf("%w: %s", ErrNotFound, err)
		}
		return domain.Segment{}, fmt.Errorf("%w: %s", ErrInternal, err)
	}

	return segment, nil
}

// Evaluations gets all the evaluations in an environment for a target
func (s Service) Evaluations(ctx context.Context, req domain.EvaluationsRequest) ([]clientgen.Evaluation, error) {
	s.logger = s.logger.With("method", "Evaluations")

	evaluations := []clientgen.Evaluation{}

	// fetch target
	t, err := s.targetRepo.GetByIdentifier(ctx, req.EnvironmentID, req.TargetIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			s.logger.Warn(ctx, "target not found in cache, serving request using only identifier attribute: ", "err", err.Error())
			t = domain.Target{Target: clientgen.Target{Identifier: req.TargetIdentifier}}
		} else {
			s.logger.Error(ctx, "error fetching target: ", "err", err.Error())
			return []clientgen.Evaluation{}, fmt.Errorf("%w: %s", ErrInternal, err)
		}

	}
	target := domain.ConvertTarget(t)
	query := s.GenerateQueryStore(ctx, req.EnvironmentID)
	sdkEvaluator, _ := evaluation.NewEvaluator(query, nil, logger.NewNoOpLogger())

	flagVariations, err := sdkEvaluator.EvaluateAll(&target)
	if err != nil {
		s.logger.Error(ctx, "ClientAPI.GetEvaluationByIdentifier() failed to perform evaluation", "environment", req.EnvironmentID, "target", target.Identifier, "err", err)
		return nil, err
	}
	//package all into the evaluations
	for i, fv := range flagVariations {

		kind := string(fv.Kind)
		eval := clientgen.Evaluation{
			Flag:       fv.FlagIdentifier,
			Value:      toString(fv.Variation, kind),
			Kind:       kind,
			Identifier: &flagVariations[i].Variation.Identifier,
		}
		evaluations = append(evaluations, eval)
	}

	return evaluations, nil
}

// EvaluationsByFeature gets all the evaluations in an environment for a target for a particular feature
func (s Service) EvaluationsByFeature(ctx context.Context, req domain.EvaluationsByFeatureRequest) (clientgen.Evaluation, error) {
	s.logger = s.logger.With("method", "EvaluationsByFeature")

	// fetch target
	t, err := s.targetRepo.GetByIdentifier(ctx, req.EnvironmentID, req.TargetIdentifier)
	if err != nil {
		if errors.Is(err, domain.ErrCacheNotFound) {
			s.logger.Warn(ctx, "target not found in cache, serving request using only identifier attribute: ", "err", err.Error())
			t = domain.Target{Target: clientgen.Target{Identifier: req.TargetIdentifier}}
		} else {
			s.logger.Error(ctx, "error fetching target: ", "err", err.Error())
			return clientgen.Evaluation{}, fmt.Errorf("%w: %s", ErrInternal, err)
		}
	}
	target := domain.ConvertTarget(t)

	query := s.GenerateQueryStore(ctx, req.EnvironmentID)

	sdkEvaluator, _ := evaluation.NewEvaluator(query, nil, logger.NewNoOpLogger())
	flagVariation, err := sdkEvaluator.Evaluate(req.FeatureIdentifier, &target)
	if err != nil {
		s.logger.Error(ctx, "ClientAPI.GetEvaluationByIdentifier() failed to perform evaluation", "environment", req.EnvironmentID, "feature", req.FeatureIdentifier, "target", target.Identifier, "err", err)
		return clientgen.Evaluation{}, err
	}

	return clientgen.Evaluation{
		Flag:       flagVariation.FlagIdentifier,
		Value:      toString(flagVariation.Variation, string(flagVariation.Kind)),
		Kind:       string(flagVariation.Kind),
		Identifier: &flagVariation.Variation.Identifier,
	}, nil
}

// Stream does a lookup for the environmentID for the APIKey in the StreamRequest
// and returns it as the GripChannel.
func (s Service) Stream(ctx context.Context, req domain.StreamRequest) (domain.StreamResponse, error) {
	s.logger = s.logger.With("method", "Stream")

	if !s.streamingEnabled {
		return domain.StreamResponse{}, fmt.Errorf("%w: streaming endpoint disabled", ErrNotImplemented)
	}

	hashedAPIKey := s.hasher.Hash(req.APIKey)

	repoKey, ok := s.authRepo.Get(ctx, domain.NewAuthAPIKey(hashedAPIKey))
	if !ok {
		return domain.StreamResponse{}, fmt.Errorf("%w: no environment found for apiKey %q", ErrNotFound, req.APIKey)
	}

	// If the internal sdk isn't connected to the SAAS stream for an environment then the Proxy shouldn't
	// accept streaming connections for that environment. We need to do this to avoid SDKs connected to the
	// Proxy from missing out on flag changes.
	if !s.sdkClients.StreamConnected(repoKey) {
		s.logger.Info(ctx, "rejecting stream connection because SaaS stream is disconnected", "environment", repoKey)
		return domain.StreamResponse{}, ErrStreamDisconnected
	}

	return domain.StreamResponse{GripChannel: repoKey}, nil
}

// Metrics forwards metrics to the analytics service
func (s Service) Metrics(ctx context.Context, req domain.MetricsRequest) error {
	s.logger = s.logger.With("method", "Metrics")

	s.logger.Debug(ctx, "got metrics request", "metrics", fmt.Sprintf("%+v", req))
	return s.metricService.StoreMetrics(ctx, req)
}

// Health checks the health of the system
func (s Service) Health(ctx context.Context) (domain.HealthResponse, error) {
	s.logger = s.logger.With("method", "Health")
	s.logger.Debug(ctx, "got health request")

	// check health functions
	err := s.cacheHealthFn(ctx)
	if err != nil {
		s.logger.Error(ctx, fmt.Sprintf("cache healthcheck error: %s", err.Error()))
	}

	systemHealth := domain.HealthResponse{
		CacheStatus: boolToHealthString(false, err == nil),
	}

	return systemHealth, nil
}

// TODO: Provide a more detailed health response for environments - FFM-8237
func boolToHealthString(envHealth bool, healthy bool) string {
	if envHealth {
		if !healthy {
			return "polling"
		}
	}

	if !healthy {
		return "unhealthy"
	}

	return "healthy"
}

func toString(variation rest.Variation, kind string) string {
	value := fmt.Sprintf("%v", variation.Value)
	if kind == "json" {
		data, err := jsoniter.Marshal(variation.Value)
		if err != nil {
			value = "{}"
		} else {
			value = string(data)
		}
	}
	return value
}
