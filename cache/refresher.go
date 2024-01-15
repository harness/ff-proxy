package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

var (
	// ErrUnexpectedMessageDomain is the error returned when an SSE message has a message domain we aren't expecting
	ErrUnexpectedMessageDomain = errors.New("unexpected message domain")

	// ErrUnexpectedEventType is the error returned when an SSE message has an event type we aren't expecting
	ErrUnexpectedEventType = errors.New("unexpected event type")
)

type config interface {
	// FetchAndPopulate authenticates, fetches and populates the config.
	FetchAndPopulate(ctx context.Context, inventoryRepo domain.InventoryRepo, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error

	// Populate populates the repos with the config
	Populate(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error

	// Key returns proxyKey
	Key() string

	// Token returns the authToken that the Config uses to communicate with Harness SaaS
	Token() string

	// RefreshToken refreshes the auth token that the Config uses for fetching env config
	RefreshToken() (string, error)

	// ClusterIdentifier returns the identifier of the cluster that the Config authenticated against
	ClusterIdentifier() string

	// SetProxyConfig sets the proxyConfig member
	SetProxyConfig(proxyConfig []domain.ProxyConfig)
}

// Refresher is a type for handling SSE events from Harness Saas
type Refresher struct {
	proxyKey          string
	authToken         string
	clusterIdentifier string
	log               log.Logger
	clientService     domain.ClientService
	config            config
	proxyConfig       []domain.ProxyConfig
	inventory         domain.InventoryRepo
	authRepo          domain.AuthRepo
	flagRepo          domain.FlagRepo
	segmentRepo       domain.SegmentRepo
}

// NewRefresher creates a Refresher
func NewRefresher(l log.Logger, config config, client domain.ClientService, inventory domain.InventoryRepo, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) Refresher {
	l = l.With("component", "Refresher")
	return Refresher{log: l, config: config, clientService: client, inventory: inventory, authRepo: authRepo, flagRepo: flagRepo, segmentRepo: segmentRepo}
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
		if err := s.handleDeleteProxyKeyEvent(ctx); err != nil {
			s.log.Error("failed to handle deleteKeyEvent", "err", err)
			return err
		}
	case domain.EventEnvironmentAdded:
		if err := s.handleAddEnvironmentEvent(ctx, msg.Environments); err != nil {
			s.log.Error("failed to handle addEnvironmentEvent", "err", err)
			return err
		}
	case domain.EventEnvironmentRemoved:
		if err := s.handleRemoveEnvironmentEvent(ctx, msg.Environments); err != nil {
			s.log.Error("failed to handle removeEnvironmentEvent", "err", err)
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

	// First refresh the auth token, the auth token contains a list of environments
	// in the claims that the ProxyKey has access to, if we don't refresh this when
	// a new env is added then our current token won't be authorised to fetch config
	// for the new env
	authToken, err := s.config.RefreshToken()
	if err != nil {
		return fmt.Errorf("failed to refresh auth token to fetch new environment config: %s", err)
	}

	for _, env := range environments {
		input := domain.GetProxyConfigInput{
			Key:               s.config.Key(),
			EnvID:             env,
			AuthToken:         authToken,
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
		// update key inventory for environment.
		if err := s.inventory.Patch(ctx, s.config.Key(), func(assets map[string]string) (map[string]string, error) {
			newAssets, err := s.inventory.BuildAssetListFromConfig(proxyConfig)
			if err != nil {
				return newAssets, err
			}
			for k := range newAssets {
				if _, ok := assets[k]; !ok {
					assets[k] = ""
				}
			}
			return assets, nil
		}); err != nil {
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
			if !errors.Is(err, domain.ErrCacheNotFound) {
				return fmt.Errorf("failed to remove apikey configs from cache for environment %s with error %s", env, err)
			}
		}

		if err := s.flagRepo.RemoveAllFeaturesForEnvironment(ctx, env); err != nil {
			if !errors.Is(err, domain.ErrCacheNotFound) {
				return fmt.Errorf("failed to remove flag config from cache for environment %s with error %s", env, err)
			}
		}

		if err := s.segmentRepo.RemoveAllSegmentsForEnvironment(ctx, env); err != nil {
			if !errors.Is(err, domain.ErrCacheNotFound) {
				return fmt.Errorf("failed to remove segment config from cache for environment %s with error %s", env, err)
			}
		}

		if err := s.removeAssets(ctx, env); err != nil {
			return err
		}
	}
	return nil
}

func (s Refresher) removeAssets(ctx context.Context, env string) error {
	// lest grab the keys we are about to delete from cache.
	assetsToDelete, err := s.getAssetsToBeDeletedForEnvironment(ctx, env)
	if err != nil && !errors.Is(err, domain.ErrCacheNotFound) {
		return err
	}

	if err := s.inventory.Patch(ctx, s.config.Key(), func(assets map[string]string) (map[string]string, error) {
		// remove deleted keys from the assets
		for k := range assetsToDelete {
			delete(assets, k)

		}
		return assets, nil
	}); err != nil {
		return err
	}

	return nil
}

func (s Refresher) getAssetsToBeDeletedForEnvironment(ctx context.Context, env string) (map[string]string, error) {
	apiKeys, err := s.authRepo.GetKeysForEnvironment(ctx, env)
	if err != nil {
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return map[string]string{}, err
		}
	}

	assetsToDelete, err := s.inventory.GetKeysForEnvironment(ctx, env)
	if err != nil {
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return map[string]string{}, err
		}
	}
	// insert all APIKeys to the map.
	for _, key := range apiKeys {
		assetsToDelete[key] = ""

	}
	return assetsToDelete, nil
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

	if err := s.authRepo.PatchAPIConfigForEnvironment(ctx, env, apiKey, domain.EventAPIKeyAdded); err != nil {
		return err
	}

	// add key to the invetnory if does not exits
	return s.inventory.Patch(ctx, s.config.Key(), func(assets map[string]string) (map[string]string, error) {
		apiKeyEntry := string(domain.NewAuthAPIKey(apiKey))
		apiConfigsEntry := string(domain.NewAPIConfigsKey(env))
		return s.addItems(assets, apiKeyEntry, apiConfigsEntry)
	})

}

// handleRemoveApiKeyEvent removes apiKeys from cache as well as removes the key from the list of keys for given environment.
func (s Refresher) handleRemoveAPIKeyEvent(ctx context.Context, env, apiKey string) error {
	s.log.Debug("removing apikey entry for env", "environment", env)
	apiKeyEntry := string(domain.NewAuthAPIKey(apiKey))
	apiConfigsEntry := string(domain.NewAPIConfigsKey(env))
	k := fmt.Sprintf("auth-key-%s", apiKey)

	if err := s.authRepo.Remove(ctx, []string{k}); err != nil {
		return err
	}
	if err := s.authRepo.PatchAPIConfigForEnvironment(ctx, env, apiKey, domain.EventAPIKeyRemoved); err != nil {
		return err
	}

	return s.inventory.Patch(ctx, s.config.Key(), func(assets map[string]string) (map[string]string, error) {
		_, ok := assets[apiKeyEntry]
		if ok {
			delete(assets, apiKeyEntry)
		}
		if !s.inventory.KeyExists(ctx, apiConfigsEntry) {
			delete(assets, apiConfigsEntry)
		}
		return assets, nil
	})
}

func (s Refresher) handleFetchFeatureEvent(ctx context.Context, env, id string) error {
	s.log.Debug("updating featureConfig entry", "environment", env, "identifier", id)

	featureConfigs, err := s.clientService.FetchFeatureConfigForEnvironment(ctx, s.config.Token(), s.config.ClusterIdentifier(), env)
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
	// patch the inventory
	return s.inventory.Patch(ctx, s.config.Key(), func(assets map[string]string) (map[string]string, error) {
		featureConfigEntry := string(domain.NewFeatureConfigKey(env, id))
		featureConfigsEntry := string(domain.NewFeatureConfigsKey(env))
		return s.addItems(assets, featureConfigEntry, featureConfigsEntry)
	})
}

func (s Refresher) handleDeleteFeatureEvent(ctx context.Context, env, identifier string) error {
	s.log.Debug("removing featureConfig entry", "environment", env, "identifier", identifier)
	featureConfigEntry := string(domain.NewFeatureConfigKey(env, identifier))
	featureConfigsEntry := string(domain.NewFeatureConfigsKey(env))
	// fetch and reset config map and delete the entry.
	features, _ := s.flagRepo.GetFeatureConfigForEnvironment(ctx, env)

	if len(features) > 1 {
		//delete the identifier
		err := s.updateFeatureConfigsEntry(ctx, env, identifier, features)
		if err != nil {
			return err
		}
	}
	// remove deleted flag entry.
	if err := s.flagRepo.
		Remove(ctx, featureConfigEntry); err != nil {
		return err
	}
	// remove configs entry if last flag was deleted
	if len(features) == 1 && features[0].Feature == identifier {
		err := s.flagRepo.Remove(ctx, featureConfigsEntry)
		if err != nil {
			return err
		}
	}

	return s.inventory.Patch(ctx, s.config.Key(), func(assets map[string]string) (map[string]string, error) {
		_, ok := assets[featureConfigEntry]
		if ok {
			delete(assets, featureConfigEntry)
		}
		// if we only had one element in featureConfigs and it's the one deleted we remove config from inventory
		if len(features) == 1 && features[0].Feature == identifier {
			delete(assets, featureConfigsEntry)
		}
		return assets, nil

	})
}

func (s Refresher) updateFeatureConfigsEntry(ctx context.Context, env string, identifier string, features []domain.FeatureFlag) error {
	updatedFeatures := make([]domain.FeatureFlag, 0, len(features))
	for _, f := range features {
		if f.Feature != identifier {
			updatedFeatures = append(updatedFeatures, f)
		}
	}
	return s.flagRepo.Add(ctx, domain.FlagConfig{
		EnvironmentID:  env,
		FeatureConfigs: updatedFeatures,
	})
}

func (s Refresher) handleFetchSegmentEvent(ctx context.Context, env, id string) error {
	s.log.Debug("updating featureConfig entry", "environment", env, "identifier", id)

	segmentConfig, err := s.clientService.FetchSegmentConfigForEnvironment(ctx, s.config.Token(), s.config.ClusterIdentifier(), env)
	if err != nil {
		return err
	}
	segments := make([]domain.Segment, 0, len(segmentConfig))
	for _, v := range segmentConfig {
		segments = append(segments, domain.Segment(v))
	}

	if err := s.segmentRepo.Add(ctx, domain.SegmentConfig{
		EnvironmentID: env,
		Segments:      segments,
	}); err != nil {
		return err
	}
	// patch the inventory
	return s.inventory.Patch(ctx, s.config.Key(), func(assets map[string]string) (map[string]string, error) {
		segmentConfigEntry := string(domain.NewSegmentKey(env, id))
		segmentConfigsEntry := string(domain.NewSegmentsKey(env))
		return s.addItems(assets, segmentConfigEntry, segmentConfigsEntry)
	})
}

func (s Refresher) handleDeleteSegmentEvent(ctx context.Context, env, identifier string) error {
	s.log.Debug("removing featureConfig entry", "environment", env, "identifier", identifier)
	segmentConfig := string(domain.NewSegmentKey(env, identifier))
	segmentConfigs := string(domain.NewSegmentsKey(env))
	// get the segment entry for the environment and update it.
	segments, _ := s.segmentRepo.GetSegmentsForEnvironment(ctx, env)
	if len(segments) > 1 {
		//delete the identifier
		err := s.updateSegmentConfigsEntry(ctx, env, identifier, segments)
		if err != nil {
			return err
		}
	}

	// remove deleted segment entry.
	if err := s.segmentRepo.Remove(ctx, identifier); err != nil {
		return err
	}
	if len(segments) == 1 && segments[0].Identifier == identifier {
		err := s.flagRepo.Remove(ctx, segmentConfigs)
		if err != nil {
			return err
		}
	}

	return s.inventory.Patch(ctx, s.config.Key(), func(assets map[string]string) (map[string]string, error) {
		_, ok := assets[segmentConfig]
		if ok {
			delete(assets, segmentConfig)
		}
		// delete segment configs entry
		if len(segments) == 1 && segments[0].Identifier == identifier {
			delete(assets, segmentConfigs)
		}
		return assets, nil
	})

}

func (s Refresher) updateSegmentConfigsEntry(ctx context.Context, env string, identifier string, segments []domain.Segment) error {
	updatedSegment := make([]domain.Segment, 0, len(segments))
	for _, s := range segments {
		if s.Identifier != identifier {
			updatedSegment = append(updatedSegment, s)
		}
	}
	return s.segmentRepo.Add(ctx, domain.SegmentConfig{
		EnvironmentID: env,
		Segments:      updatedSegment,
	})
}

func (s Refresher) addItems(assets map[string]string, configKey, configsKey string) (map[string]string, error) {
	_, ok := assets[configKey]
	if !ok {
		assets[configKey] = ""
	}
	_, ok = assets[configsKey]
	if !ok {
		assets[configsKey] = ""
	}
	return assets, nil
}

// NOTE: This is currently working with assumption that there is a single redis instance for proxy.
// logic will have to be chanced if redis instance is shared between proxy instances.
func (s Refresher) handleDeleteProxyKeyEvent(ctx context.Context) error {
	// function will delete all the keys for the proxy key deleted.
	proxyKey := s.config.Key()
	//delete all the assets
	inventory, err := s.inventory.Get(ctx, proxyKey)
	if err != nil {
		return err
	}

	for k := range inventory {
		err := s.inventory.Remove(ctx, k)
		if err != nil {
			return err
		}
	}
	keyInventoryEntry := string(domain.NewKeyInventory(proxyKey))
	return s.inventory.Remove(ctx, keyInventoryEntry)

}
