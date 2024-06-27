package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/log"

	"github.com/harness/ff-proxy/v2/domain"
)

// TargetRepo is a repository that stores Targets
type TargetRepo struct {
	log   log.Logger
	cache cache.Cache
}

// NewTargetRepo creates a TargetRepo. It can optionally preload the repo with data
// from the passed config
func NewTargetRepo(c cache.Cache, l log.Logger) TargetRepo {
	l = l.With("component", "TargetRepo")
	return TargetRepo{
		cache: c,
		log:   l,
	}
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
// nolint:cyclop
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
			return err
		}
	}

	// If there are targets from the cache that aren't in the map of new targets
	// then we'll want to remove them.
	for identifier := range existingTargets {
		if _, ok := newTargets[identifier]; !ok {
			if err2 := t.cache.Delete(ctx, string(key)); err != nil {

				// We don't want to log context cancelled as an error in here
				if !errors.Is(err2, context.Canceled) {
					t.log.Info("context was cancelled during DeltaAdd", "err", err)
					continue
				}

				t.log.Error("failed to flush stale target from cache during DeltaAdd", "err", err)
			}
		}
	}

	return t.addTargets(ctx, envID, targets...)
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
