package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/cache"

	"github.com/harness/ff-proxy/domain"
)

// FeatureFlagRepo is a repository that stores FeatureFlags
type FeatureFlagRepo struct {
	cache cache.Cache
}

// NewFeatureFlagRepo creates a FeatureFlagRepo. It can optionally preload the repo with data
// from the passed config
func NewFeatureFlagRepo(c cache.Cache, config map[domain.FeatureFlagKey][]domain.FeatureFlag) (FeatureFlagRepo, error) {
	fcr := FeatureFlagRepo{cache: c}
	if config == nil {
		return fcr, nil
	}

	for key, cfg := range config {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		// cleanup all current keys before we add new ones to make sure keys that have been deleted remotely are removed
		fcr.cache.RemoveAll(ctx, string(key))
		if err := fcr.Add(ctx, key, cfg...); err != nil {
			cancel()
			return FeatureFlagRepo{}, fmt.Errorf("failed to add flag: %s", err)
		}
		cancel()
	}
	return fcr, nil
}

// Add adds a target or multiple targets to the given key
func (f FeatureFlagRepo) Add(ctx context.Context, key domain.FeatureFlagKey, values ...domain.FeatureFlag) error {
	errs := []error{}
	for _, v := range values {
		if err := f.cache.Set(ctx, string(key), v.Feature, &v); err != nil {
			errs = append(errs, addErr{string(key), v.Feature, err})
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add featureFlag(s) to repo: %v", errs)
	}
	return nil
}

// Get gets all of the FeatureFlag for a given key
func (f FeatureFlagRepo) Get(ctx context.Context, key domain.FeatureFlagKey) ([]domain.FeatureFlag, error) {
	results, err := f.cache.GetAll(ctx, string(key))
	if err != nil {
		return []domain.FeatureFlag{}, err
	}

	featureFlags := make([]domain.FeatureFlag, len(results))

	idx := 0
	for _, b := range results {
		featureFlag := &domain.FeatureFlag{}
		if err := featureFlag.UnmarshalBinary(b); err != nil {
			return []domain.FeatureFlag{}, err
		}

		featureFlags[idx] = *featureFlag
		idx++
	}
	return featureFlags, nil
}

// GetByIdentifier gets a FeatureFlag for a given key and identifier
func (f FeatureFlagRepo) GetByIdentifier(ctx context.Context, key domain.FeatureFlagKey, identifier string) (domain.FeatureFlag, error) {
	featureFlag := domain.FeatureFlag{}
	if err := f.cache.Get(ctx, string(key), identifier, &featureFlag); err != nil {
		return domain.FeatureFlag{}, err
	}
	return featureFlag, nil
}
