package repository

import (
	"context"
	"fmt"

	clientgen "github.com/harness/ff-proxy/v2/gen/client"

	"github.com/harness/ff-proxy/v2/cache"

	"github.com/harness/ff-proxy/v2/domain"
)

// FeatureFlagOption defines functional option for a FeatureFlagRepo
type FeatureFlagOption func(f *FeatureFlagRepo)

// WithFeatureConfig populates a FeatureFlagRepo with the given config
func WithFeatureConfig(config map[domain.FeatureFlagKey]interface{}) FeatureFlagOption {
	return func(f *FeatureFlagRepo) {
		for key, value := range config {
			// cleanup all current keys before we add new ones to make sure keys that have been deleted remotely are removed
			_ = f.cache.Delete(context.Background(), string(key))

			// Don't bother saving an empty slice
			if s, ok := value.([]domain.FeatureFlag); ok {
				if s == nil || len(s) == 0 {
					return
				}
			}

			// Don't bother adding a nil target to the cache
			if s, ok := value.(*domain.FeatureFlag); ok && s == nil {
				return
			}

			f.cache.Set(context.Background(), string(key), value)
		}
	}
}

// FeatureFlagRepo is a repository that stores FeatureFlags
type FeatureFlagRepo struct {
	cache cache.Cache
}

// NewFeatureFlagRepo creates a FeatureFlagRepo. It can optionally preload the repo with data
// from the passed config
func NewFeatureFlagRepo(c cache.Cache, opts ...FeatureFlagOption) (FeatureFlagRepo, error) {
	fcr := FeatureFlagRepo{cache: c}

	for _, opt := range opts {
		opt(&fcr)
	}

	return fcr, nil
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

func (f FeatureFlagRepo) Add(ctx context.Context, config ...domain.FlagConfig) error {
	errs := []error{}

	for _, cfg := range config {
		k := domain.NewFeatureConfigsKey(cfg.EnvironmentID)

		if err := f.cache.Set(ctx, string(k), cfg.FeatureConfigs); err != nil {
			errs = append(errs, addErr{
				key:        string(k),
				identifier: "feature-configs",
				err:        err,
			})
		}

		for _, flag := range cfg.FeatureConfigs {
			key := domain.NewFeatureConfigKey(cfg.EnvironmentID, flag.Feature)

			if err := f.cache.Set(ctx, string(key), flag); err != nil {
				errs = append(errs, addErr{
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
