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

// clientService is the interface for interacting with the feature flag client service
type clientService interface {
	Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error)
}

// MetricStore is the interface for storing metrics
type MetricStore interface {
	StoreMetrics(ctx context.Context, metrics domain.MetricsRequest) error
}

// SDKClients is an interface that can be used to find out if internal sdks are connected to the SaaS FF stream
type SDKClients interface {
	StreamConnected(key string) bool
}

// Config is the config for a Service
type Config struct {
	Logger        log.ContextualLogger
	FeatureRepo   repository.FeatureFlagRepo
	TargetRepo    repository.TargetRepo
	SegmentRepo   repository.SegmentRepo
	AuthRepo      repository.AuthRepo
	AuthFn        authTokenFn
	ClientService clientService
	MetricStore   MetricStore
	Offline       bool
	Hasher        hash.Hasher

	HealthySaasStream func() bool

	// SDKStreamConnected is a callback that we call whenee
	SDKStreamConnected func(envID string)

	Health func(ctx context.Context) domain.HealthResponse

	ForwardTargets bool
}

type segmentRepo interface {
	Get(ctx context.Context, environmentID string) ([]domain.Segment, error)
	GetByIdentifier(ctx context.Context, environmentID string, identifier string) (domain.Segment, error)
}

// Service is the proxy service implementation
type Service struct {
	logger             log.ContextualLogger
	featureRepo        repository.FeatureFlagRepo
	targetRepo         repository.TargetRepo
	segmentRepo        segmentRepo
	authRepo           repository.AuthRepo
	authFn             authTokenFn
	clientService      clientService
	metricService      MetricStore
	offline            bool
	hasher             hash.Hasher
	healthySassStream  func() bool
	sdkStreamConnected func(envID string)

	health func(ctx context.Context) domain.HealthResponse

	forwardTargets bool
}

// NewService creates and returns a ProxyService
func NewService(c Config) Service {
	l := c.Logger.With("component", "ProxyService")
	return Service{
		logger:             l,
		featureRepo:        c.FeatureRepo,
		targetRepo:         c.TargetRepo,
		segmentRepo:        c.SegmentRepo,
		authRepo:           c.AuthRepo,
		authFn:             c.AuthFn,
		clientService:      c.ClientService,
		metricService:      c.MetricStore,
		offline:            c.Offline,
		hasher:             c.Hasher,
		healthySassStream:  c.HealthySaasStream,
		sdkStreamConnected: c.SDKStreamConnected,
		health:             c.Health,
		forwardTargets:     c.ForwardTargets,
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

	// If we aren't forwarding targets to Saas we're done
	if s.forwardTargets {
		// Otherwise forward targets in a goroutine so we don't block the auth request
		go func() {
			newCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if _, err := s.clientService.Authenticate(newCtx, req.APIKey, req.Target); err != nil {
				s.logger.Error(ctx, "failed to forward Target registration via auth request to client service", "err", err)
			}
			s.logger.Debug(ctx, "successfully registered target with feature flags", "target_identifier", req.Target.Target.Identifier)
		}()
	}

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

	// We fetch all the segments ahead of time and build up a map that we can
	// pass to the QueryStore. This might be overkill for users that don't have
	// a lot of rules but for users that do have lots of rules it saves the query
	// store from making a ton of individual segment calls to the cache.
	segmentMap := s.makeSegmentMap(ctx, req.EnvironmentID)

	query := s.GenerateQueryStore(ctx, req.EnvironmentID, segmentMap)
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

	query := s.GenerateQueryStore(ctx, req.EnvironmentID, nil)

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
	// We only want to allow streaming connections from SDKs if the Proxy has a healthy stream with SaaS.
	// This is because when the Saas -> Proxy stream is down the Proxy Polls for changes. Changes fetched
	// during a polling operation don't have an SSE event associated with them so there's no way to notify
	// connected SDKs that a change has happened. Refusing stream requests when this stream is down forces
	// SDKs to poll the Proxy for changes until the stream is healthy again, meaning SDKs won't miss out on
	// changes pulled down via polling.
	if !s.healthySassStream() {
		return domain.StreamResponse{}, fmt.Errorf("%w: streaming endpoint disabled", ErrStreamDisconnected)
	}

	hashedAPIKey := s.hasher.Hash(req.APIKey)
	envID, ok, err := s.authRepo.Get(ctx, domain.NewAuthAPIKey(hashedAPIKey))
	if err != nil {
		s.logger.Error(ctx, "stream handler failed to check if key exists in cache", "err", err)
	}
	if !ok {
		return domain.StreamResponse{}, fmt.Errorf("%w: no environment found for apiKey %q", ErrNotFound, req.APIKey)
	}

	s.sdkStreamConnected(envID)

	return domain.StreamResponse{GripChannel: envID}, nil
}

// Metrics forwards metrics to the analytics service
func (s Service) Metrics(ctx context.Context, req domain.MetricsRequest) error {

	s.logger.Debug(ctx, "got metrics request", "metrics", fmt.Sprintf("%+v", req))
	return s.metricService.StoreMetrics(ctx, req)
}

// Health checks the health of the system
func (s Service) Health(ctx context.Context) (domain.HealthResponse, error) {
	s.logger.Debug(ctx, "got health request")

	healthResp := s.health(ctx)
	if healthResp.ConfigStatus.State == domain.ConfigStateFailedToSync {
		return healthResp, ErrInternal
	}
	return healthResp, nil
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

func (s Service) makeSegmentMap(ctx context.Context, envID string) map[string]*domain.Segment {
	var segmentMap map[string]*domain.Segment

	segments, err := s.segmentRepo.Get(ctx, envID)
	if err != nil {
		// Not much else we can really do here other than log the error
		s.logger.Error(ctx, "makeSegmentMap failed to get segments from cache: ", "err", err)
		return segmentMap
	}

	if len(segments) > 0 {
		segmentMap = make(map[string]*domain.Segment)
		for i := 0; i < len(segments); i++ {
			seg := segments[i]
			segmentMap[seg.Identifier] = &seg
		}
	}

	return segmentMap
}
