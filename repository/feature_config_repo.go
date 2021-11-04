package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/domain"
)

// FeatureConfigRepo is a repository that stores FeatureConfigs
type FeatureConfigRepo struct {
	cache Cache
}

// NewFeatureConfigRepo creates a FeatureConfigRepo. It can optionally preload the repo with data
// from the passed config
func NewFeatureConfigRepo(c Cache, config map[domain.FeatureConfigKey][]domain.FeatureConfig) (FeatureConfigRepo, error) {
	fcr := FeatureConfigRepo{cache: c}
	if config == nil {
		return fcr, nil
	}

	for key, cfg := range config {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := fcr.Add(ctx, key, cfg...); err != nil {
			cancel()
			return FeatureConfigRepo{}, fmt.Errorf("failed to add config: %s", err)
		}
		cancel()
	}
	return fcr, nil
}

// Add adds a target or multiple targets to the given key
func (f FeatureConfigRepo) Add(ctx context.Context, key domain.FeatureConfigKey, values ...domain.FeatureConfig) error {
	errs := []error{}
	for _, v := range values {
		if err := f.cache.Set(ctx, string(key), v.Feature, &v); err != nil {
			errs = append(errs, addErr{string(key), v.Feature, err})
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add featureConfig(s) to repo: %v", errs)
	}
	return nil
}

// Get gets all of the FeatureConfig for a given key
func (f FeatureConfigRepo) Get(ctx context.Context, key domain.FeatureConfigKey) ([]domain.FeatureConfig, error) {
	results, err := f.cache.GetAll(ctx, string(key))
	if err != nil {
		return []domain.FeatureConfig{}, err
	}

	featureConfigs := []domain.FeatureConfig{}
	for _, b := range results {
		featureConfig := &domain.FeatureConfig{}
		if err := featureConfig.UnmarshalBinary(b); err != nil {
			return []domain.FeatureConfig{}, err
		}
		featureConfigs = append(featureConfigs, *featureConfig)
	}
	return featureConfigs, nil
}

// GetByIdentifier gets a FeatureConfig for a given key and identifer
func (f FeatureConfigRepo) GetByIdentifier(ctx context.Context, key domain.FeatureConfigKey, identifier string) (domain.FeatureConfig, error) {
	featureConfig := domain.FeatureConfig{}
	if err := f.cache.Get(ctx, string(key), identifier, &featureConfig); err != nil {
		return domain.FeatureConfig{}, err
	}
	return featureConfig, nil
}
