package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/cache"

	"github.com/harness/ff-proxy/domain"
)

// TargetRepo is a repository that stores Targets
type TargetRepo struct {
	cache cache.Cache
}

// NewTargetRepo creates a TargetRepo. It can optionally preload the repo with data
// from the passed config
func NewTargetRepo(c cache.Cache, config map[domain.TargetKey][]domain.Target) (TargetRepo, error) {
	tr := TargetRepo{cache: c}
	if config == nil {
		return tr, nil
	}

	for key, cfg := range config {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		// cleanup all current keys before we add new ones to make sure keys that have been deleted remotely are removed
		tr.cache.RemoveAll(ctx, string(key))
		if err := tr.Add(ctx, key, cfg...); err != nil {
			cancel()
			return TargetRepo{}, fmt.Errorf("failed to add config: %s", err)
		}
		cancel()
	}
	return tr, nil
}

// Add adds a target or multiple targets to the given key
func (t TargetRepo) Add(ctx context.Context, key domain.TargetKey, values ...domain.Target) error {
	errs := []error{}
	for _, v := range values {
		if err := t.cache.Set(ctx, string(key), v.Identifier, &v); err != nil {
			errs = append(errs, addErr{string(key), v.Identifier, err})
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add target(s) to repo: %v", errs)
	}
	return nil
}

// Get gets all of the Targets for a given key
func (t TargetRepo) Get(ctx context.Context, key domain.TargetKey) ([]domain.Target, error) {
	results, err := t.cache.GetAll(ctx, string(key))
	if err != nil {
		return []domain.Target{}, err
	}

	targets := []domain.Target{}
	for _, b := range results {
		target := &domain.Target{}
		if err := target.UnmarshalBinary(b); err != nil {
			return []domain.Target{}, err
		}
		targets = append(targets, *target)
	}

	return targets, nil
}

// GetByIdentifier gets a Target for a given key and identifer
func (t TargetRepo) GetByIdentifier(ctx context.Context, key domain.TargetKey, identifier string) (domain.Target, error) {
	target := domain.Target{}
	if err := t.cache.Get(ctx, string(key), identifier, &target); err != nil {
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
func (t TargetRepo) DeltaAdd(ctx context.Context, key domain.TargetKey, targets ...domain.Target) error {
	if len(targets) == 0 {
		return fmt.Errorf("can't perform DeltaAdd with zero targets for key %s", key)
	}

	results, err := t.cache.GetAll(ctx, string(key))
	if err != nil {
		// If the key doesn't already exist in the cache we will want to add it
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return err
		}
	}

	existingTargets := map[string]domain.Target{}
	for _, b := range results {
		target := &domain.Target{}
		if err := target.UnmarshalBinary(b); err != nil {
			return err
		}
		existingTargets[target.Identifier] = *target
	}

	newTargets := map[string]domain.Target{}
	for _, target := range targets {
		newTargets[target.Identifier] = target
	}

	// If there are targets from the cache that aren't in the map of new targets
	// then we'll want to remove them.
	for identifier := range existingTargets {
		if _, ok := newTargets[identifier]; !ok {
			t.cache.Remove(ctx, string(key), identifier)
		}
	}

	return t.Add(ctx, key, targets...)
}
