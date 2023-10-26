package repository

import (
	"context"
	"fmt"

	clientgen "github.com/harness/ff-proxy/v2/gen/client"

	"github.com/harness/ff-proxy/v2/cache"

	"github.com/harness/ff-proxy/v2/domain"
)

// FeatureFlagRepo is a repository that stores FeatureFlags
type FeatureFlagRepo struct {
	cache cache.Cache
}

// NewFeatureFlagRepo creates a FeatureFlagRepo. It can optionally preload the repo with data
// from the passed config
func NewFeatureFlagRepo(c cache.Cache) FeatureFlagRepo {
	return FeatureFlagRepo{cache: c}
}

// Get gets all the FeatureFlag for a given key
func (f FeatureFlagRepo) Get(ctx context.Context, envID string) ([]domain.FeatureFlag, error) {
	var featureFlags []domain.FeatureFlag
	key := domain.NewFeatureConfigsKey(envID)

	err := f.cache.Get(ctx, string(key), &featureFlags)
	if err != nil {
		return []domain.FeatureFlag{}, err
	}

	return featureFlags, nil
}

// GetByIdentifier gets a FeatureFlag for a given key and identifier
func (f FeatureFlagRepo) GetByIdentifier(ctx context.Context, envID string, identifier string) (domain.FeatureFlag, error) {
	featureFlag := domain.FeatureFlag{}
	key := domain.NewFeatureConfigKey(envID, identifier)

	if err := f.cache.Get(ctx, string(key), &featureFlag); err != nil {
		return domain.FeatureFlag{}, err
	}

	// some sdks e.g. .NET don't cope well with being returned a null VariationToTargetMap so we send back an empty struct here for now
	// to match ff-server behaviour
	if featureFlag.VariationToTargetMap == nil {
		emptyVariationMap := []clientgen.VariationMap{}
		featureFlag.VariationToTargetMap = &emptyVariationMap
	}
	return featureFlag, nil
}

// Add stores FlagConfig in the cache
func (f FeatureFlagRepo) Add(ctx context.Context, config ...domain.FlagConfig) error {
	errs := []error{}

	for _, cfg := range config {
		k := domain.NewFeatureConfigsKey(cfg.EnvironmentID)

		if err := f.cache.Set(ctx, string(k), cfg.FeatureConfigs); err != nil {
			errs = append(errs, addError{
				key:        string(k),
				identifier: "feature-configs",
				err:        err,
			})
		}

		for _, flag := range cfg.FeatureConfigs {
			key := domain.NewFeatureConfigKey(cfg.EnvironmentID, flag.Feature)

			if err := f.cache.Set(ctx, string(key), flag); err != nil {
				errs = append(errs, addError{
					key:        string(key),
					identifier: flag.Feature,
					err:        err,
				})
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add flagConfig(s) to cache: %v", errs)
	}

	return nil
}

// Remove removes the feature entry from the cache
func (f FeatureFlagRepo) Remove(ctx context.Context, identifier string) error {

	// remove featureConfigs entry
	fcKey := domain.NewFeatureConfigsKey(identifier)
	if err := f.cache.Delete(ctx, string(fcKey)); err != nil {
		return err
	}
	return nil
}

// RemoveAllFeaturesForEnvironment removes all feature entries for given environment id
func (f FeatureFlagRepo) RemoveAllFeaturesForEnvironment(ctx context.Context, id string) error {

	// get all the feature for given key
	flags, err := f.Get(ctx, id)
	if err != nil {
		return err
	}
	// remove featureConfigs entry
	fcKey := domain.NewFeatureConfigsKey(id)
	if err := f.cache.Delete(ctx, string(fcKey)); err != nil {
		return err
	}
	// remove all individual feature entries for environment
	for _, flag := range flags {

		key := domain.NewFeatureConfigKey(id, flag.Feature)
		if err := f.cache.Delete(ctx, string(key)); err != nil {
			return err
		}
	}
	return nil
}
