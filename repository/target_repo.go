package repository

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/harness/ff-proxy/v2/cache"

	"github.com/harness/ff-proxy/v2/domain"
)

// TargetOption defines functional option for a TargetRepo
type TargetOption func(t *TargetRepo)

// WithTargetConfig populates a TargetRepo with the given config
func WithTargetConfig(config map[domain.TargetKey]interface{}) TargetOption {
	return func(t *TargetRepo) {
		for key, value := range config {

			// cleanup all current keys before we add new ones to make sure keys that have been deleted remotely are removed
			if err := t.cache.Delete(context.Background(), string(key)); err != nil {
				log.Println("failed to clean cache for targets: ", err)
			}

			// Don't bother saving an empty slice
			if s, ok := value.([]domain.Target); ok {
				if s == nil || len(s) == 0 {
					return
				}
			}

			// Don't bother adding a nil target to the cache
			if s, ok := value.(*domain.Target); ok && s == nil {
				return
			}

			if err := t.cache.Set(context.Background(), string(key), value); err != nil {
				log.Println("failed to set target in cache: ", err)
			}
		}
	}
}

// TargetRepo is a repository that stores Targets
type TargetRepo struct {
	cache cache.Cache
}

// NewTargetRepo creates a TargetRepo. It can optionally preload the repo with data
// from the passed config
func NewTargetRepo(c cache.Cache, opts ...TargetOption) (TargetRepo, error) {
	tr := TargetRepo{cache: c}

	for _, opt := range opts {
		opt(&tr)
	}

	return tr, nil
}

func (t TargetRepo) Add(ctx context.Context, envID string, targets ...domain.Target) error {
	if len(targets) == 1 {
		target := targets[0]
		key := domain.NewTargetKey(envID, target.Identifier)
		if err := t.cache.Set(ctx, string(key), target); err != nil {
			// log and contineu
		}
	}

	key := domain.NewTargetsKey(envID)

	var existingTargets []domain.Target
	err := t.cache.Get(ctx, string(key), &existingTargets)
	if err != nil && !errors.Is(err, domain.ErrCacheNotFound) {
		// log and continue
	}

	existingTargets = append(existingTargets, targets...)

	return t.cache.Set(ctx, string(key), targets)
}

// Add adds a target or multiple targets to the given key
func (t TargetRepo) addTarget(ctx context.Context, envID string, target domain.Target) error {
	key := domain.NewTargetKey(envID, target.Identifier)

	return t.cache.Set(ctx, string(key), target)
}

func (t TargetRepo) addTargets(ctx context.Context, envID string, targets ...domain.Target) error {
	key := domain.NewTargetsKey(envID)

	return t.cache.Set(ctx, string(key), targets)
}

// Get gets all of the Targets for a given key
func (t TargetRepo) Get(ctx context.Context, envID string) ([]domain.Target, error) {
	var targets []domain.Target
	key := domain.NewTargetsKey(envID)

	err := t.cache.Get(ctx, string(key), &targets)
	if err != nil {
		return []domain.Target{}, err
	}

	return targets, nil
}

// GetByIdentifier gets a Target for a given key and identifer
func (t TargetRepo) GetByIdentifier(ctx context.Context, envID string, identifier string) (domain.Target, error) {
	target := domain.Target{}
	key := domain.NewTargetKey(envID, identifier)

	if err := t.cache.Get(ctx, string(key), &target); err != nil {
		return domain.Target{}, err
	}
	return target, nil
}

// DeltaAdd adds new Targets and updates existing Targets for the given key if they
// exist in the cache. It will remove any existing Targets from the cache that are not
// in the list of new Targets. If you pass it an empty list of Targets it will return
// an error to avoid wiping all of the Targets from the cache. If we want to remove
// all of the Targets for a given key then we should add an explicit Remove method
// that calls cache.Remove.
func (t TargetRepo) DeltaAdd(ctx context.Context, envID string, targets ...domain.Target) error {
	if len(targets) == 0 {
		return fmt.Errorf("can't perform DeltaAdd with zero targets for environment %s", envID)
	}

	key := domain.NewTargetsKey(envID)
	var results []domain.Target

	err := t.cache.Get(ctx, string(key), &results)
	if err != nil {
		// If the key doesn't already exist in the cache we will want to add it
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return err
		}
	}

	existingTargets := make(map[string]domain.Target, len(results))
	for _, t := range results {
		existingTargets[t.Identifier] = t
	}

	newTargets := make(map[string]domain.Target, len(results))
	for _, target := range targets {
		newTargets[target.Identifier] = target

		if err := t.addTarget(ctx, envID, target); err != nil {
			// TODO:
			return err
		}
	}

	// If there are targets from the cache that aren't in the map of new targets
	// then we'll want to remove them.
	for identifier := range existingTargets {
		if _, ok := newTargets[identifier]; !ok {
			t.cache.Delete(ctx, string(key))
		}
	}

	return t.addTargets(ctx, envID, targets...)
}
