package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-proxy/v2/config"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

var (
	// ErrUnexpectedMessageDomain is the error returned when an SSE message has a message domain we aren't expecting
	ErrUnexpectedMessageDomain = errors.New("unexpected message domain")

	// ErrUnexpectedEventType is the error returned when an SSE message has an event type we aren't expecting
	ErrUnexpectedEventType = errors.New("unexpected event type")
)

// Refresher is a type for handling SSE events from Harness Saas
type Refresher struct {
	proxyKey          string
	authToken         string
	clusterIdentifier string
	log               log.Logger
	clientService     domain.ClientService
	config            config.Config
	proxyConfig       []domain.ProxyConfig
	authRepo          domain.AuthRepo
	flagRepo          domain.FlagRepo
	segmentRepo       domain.SegmentRepo
}

// NewRefresher creates a Refresher
func NewRefresher(l log.Logger, config config.Config, client domain.ClientService, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) Refresher {
	l = l.With("component", "Refresher")
	return Refresher{log: l, config: config, clientService: client, authRepo: authRepo, flagRepo: flagRepo, segmentRepo: segmentRepo}
}

// HandleMessage makes Refresher implement the MessageHandler interface
func (s Refresher) HandleMessage(ctx context.Context, msg domain.SSEMessage) error {
	switch msg.Domain {
	case domain.MsgDomainFeature:
		return s.handleFeatureMessage(ctx, msg)
	case domain.MsgDomainSegment:
		return s.handleSegmentMessage(ctx, msg)
	case domain.MsgDomainProxy:
		return s.handleProxyMessage(ctx, msg)
	default:
		return fmt.Errorf("%w: %s", ErrUnexpectedMessageDomain, msg.Domain)
	}

}

func (s Refresher) handleFeatureMessage(ctx context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventDelete:
		if err := s.handleDeleteFeatureEvent(ctx, msg.Environment, msg.Identifier); err != nil {
			s.log.Error("failed to handle feature delete event", "err", err)
			return err
		}
	case domain.EventPatch, domain.EventCreate:
		if err := s.handleFetchFeatureEvent(ctx, msg.Environment, msg.Identifier); err != nil {
			s.log.Error("failed to handle feature update event", "err", err)
			return err
		}
	default:
		return fmt.Errorf("%w %q for FeatureMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

func (s Refresher) handleSegmentMessage(ctx context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventDelete:
		if err := s.handleDeleteSegmentEvent(ctx, msg.Environment, msg.Identifier); err != nil {
			s.log.Error("failed to handle segment delete event", "err", err)
			return err
		}
	case domain.EventPatch, domain.EventCreate:
		if err := s.handleFetchSegmentEvent(ctx, msg.Environment, msg.Identifier); err != nil {
			s.log.Error("failed to handle segment update event", "err", err)
			return err
		}
	default:
		return fmt.Errorf("%w %q for SegmentMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

//nolint:cyclop
func (s Refresher) handleProxyMessage(ctx context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventProxyKeyDeleted:
		// todo
		return nil
	case domain.EventEnvironmentAdded:
		if err := s.handleAddEnvironmentEvent(ctx, msg.Environments); err != nil {
			s.log.Error("failed to handle addEnvironmentEvent", "err", err)
			return err
		}
	case domain.EventEnvironmentRemoved:
		if err := s.handleRemoveEnvironmentEvent(ctx, msg.Environments); err != nil {
			s.log.Error("failed to handle addEnvironmentEvent", "err", err)
			return err
		}
	case domain.EventAPIKeyAdded:
		if err := s.handleAddAPIKeyEvent(ctx, msg.Environments[0], msg.APIKey); err != nil {
			s.log.Error("failed to handle addApiKeyEvent", "err", err)
			return err
		}
	case domain.EventAPIKeyRemoved:
		if err := s.handleRemoveAPIKeyEvent(ctx, msg.Environments[0], msg.APIKey); err != nil {
			s.log.Error("failed to handle removeApiKeyEvent", "err", err)
			return err
		}
	default:
		return fmt.Errorf("%w %q for Proxymessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

// handleAddEnvironmentEvent fetches proxyConfig for all added environments and sets them on.
func (s Refresher) handleAddEnvironmentEvent(ctx context.Context, environments []string) error {
	// clean the proxyConfig after we are done setting it.
	defer func() {
		s.config.SetProxyConfig([]domain.ProxyConfig{})
	}()

	clusterIdentifier := s.config.ClusterIdentifier()
	if clusterIdentifier == "" {
		clusterIdentifier = "1"
	}

	for _, env := range environments {
		input := domain.GetProxyConfigInput{
			Key:               s.config.Key(),
			EnvID:             env,
			AuthToken:         s.config.Token(),
			ClusterIdentifier: clusterIdentifier,
			PageNumber:        0,
			PageSize:          10,
		}

		proxyConfig, err := s.clientService.PageProxyConfig(ctx, input)
		if err != nil {
			s.log.Error("unable to fetch config for the environment", "environment", env)
			return err
		}
		s.config.SetProxyConfig(proxyConfig)
		if err := s.config.Populate(ctx, s.authRepo, s.flagRepo, s.segmentRepo); err != nil {
			return err
		}
	}

	return nil
}

// handleRemoveEnvironmentEvent removes proxyConfig for all removed environments from cache.
func (s Refresher) handleRemoveEnvironmentEvent(ctx context.Context, environments []string) error {
	for _, env := range environments {
		s.log.Debug("removing entries for env", "environment", env)

		if err := s.authRepo.RemoveAllKeysForEnvironment(ctx, env); err != nil {
			return fmt.Errorf("failed to remove apikey configs from cache for environment %s with error %s", env, err)
		}

		if err := s.flagRepo.RemoveAllFeaturesForEnvironment(ctx, env); err != nil {
			return fmt.Errorf("failed to remove flag config from cache for environment %s with error %s", env, err)
		}

		if err := s.segmentRepo.RemoveAllSegmentsForEnvironment(ctx, env); err != nil {
			return fmt.Errorf("failed to remove segment config from cache for environment %s with error %s", env, err)
		}
	}
	return nil
}

// handleAddApiKeyEvent adds apiKeys to the cache as well as update apikey list for environment.
func (s Refresher) handleAddAPIKeyEvent(ctx context.Context, env, apiKey string) error {
	s.log.Debug("adding apikey entry for env", "environment", env)

	authConfig := []domain.AuthConfig{{
		APIKey:        domain.NewAuthAPIKey(apiKey),
		EnvironmentID: domain.EnvironmentID(env),
	},
	}
	// set the key first
	if err := s.authRepo.Add(ctx, authConfig...); err != nil {
		return err
	}
	return s.authRepo.PatchAPIConfigForEnvironment(ctx, env, apiKey, domain.EventAPIKeyAdded)
}

// handleRemoveApiKeyEvent removes apiKeys from cache as well as removes the key from the list of keys for given environment.
func (s Refresher) handleRemoveAPIKeyEvent(ctx context.Context, env, apiKey string) error {
	s.log.Debug("removing apikey entry for env", "environment", env)

	k := fmt.Sprintf("auth-key-%s", apiKey)
	if err := s.authRepo.Remove(ctx, []string{k}); err != nil {
		return err
	}
	return s.authRepo.PatchAPIConfigForEnvironment(ctx, env, apiKey, domain.EventAPIKeyRemoved)
}

func (s Refresher) handleFetchFeatureEvent(ctx context.Context, env, id string) error {
	s.log.Debug("updating featureConfig entry", "environment", env, "identifier", id)

	featureConfigs, err := s.clientService.FetchFeatureConfigForEnvironment(ctx, s.config.Token(), env)
	if err != nil {
		return err
	}
	features := make([]domain.FeatureFlag, 0, len(featureConfigs))
	for _, v := range featureConfigs {
		features = append(features, domain.FeatureFlag(v))
	}

	// set the config
	return s.flagRepo.Add(ctx, domain.FlagConfig{
		EnvironmentID:  env,
		FeatureConfigs: features,
	})
}

func (s Refresher) handleDeleteFeatureEvent(ctx context.Context, env, identifier string) error {
	s.log.Debug("removing featureConfig entry", "environment", env, "identifier", identifier)
	// fetch and reset config map and delete the entry.
	featureConfigs, err := s.clientService.FetchFeatureConfigForEnvironment(ctx, s.config.Token(), env)
	if err != nil {
		return err
	}
	features := make([]domain.FeatureFlag, 0, len(featureConfigs))
	for _, v := range featureConfigs {
		features = append(features, domain.FeatureFlag(v))
	}

	// set the config
	if err := s.flagRepo.Add(ctx, domain.FlagConfig{
		EnvironmentID:  env,
		FeatureConfigs: features,
	}); err != nil {
		return err
	}
	// remove deleted flag entry.
	return s.flagRepo.Remove(ctx, env, identifier)
}

func (s Refresher) handleFetchSegmentEvent(ctx context.Context, env, id string) error {
	s.log.Debug("updating featureConfig entry", "environment", env, "identifier", id)

	segmentConfig, err := s.clientService.FetchSegmentConfigForEnvironment(ctx, s.config.Token(), env)
	if err != nil {
		return err
	}
	segments := make([]domain.Segment, 0, len(segmentConfig))
	for _, v := range segmentConfig {
		segments = append(segments, domain.Segment(v))
	}

	// set the config
	return s.segmentRepo.Add(ctx, domain.SegmentConfig{
		EnvironmentID: env,
		Segments:      segments,
	})
}

func (s Refresher) handleDeleteSegmentEvent(ctx context.Context, env, identifier string) error {
	s.log.Debug("removing featureConfig entry", "environment", env, "identifier", identifier)
	// fetch and reset config map and delete the entry.
	segmentConfig, err := s.clientService.FetchSegmentConfigForEnvironment(ctx, s.config.Token(), env)
	if err != nil {
		return err
	}
	segments := make([]domain.Segment, 0, len(segmentConfig))
	for _, v := range segmentConfig {
		segments = append(segments, domain.Segment(v))
	}

	// set the config
	if err := s.segmentRepo.Add(ctx, domain.SegmentConfig{
		EnvironmentID: env,
		Segments:      segments,
	}); err != nil {
		return err
	}
	// remove deleted flag entry.
	return s.segmentRepo.Remove(ctx, env, identifier)
}
