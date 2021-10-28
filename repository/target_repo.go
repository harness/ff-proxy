package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/domain"
)

// TargetRepo is a repository that stores Targets
type TargetRepo struct {
	cache Cache
}

// NewTargetRepo creates a TargetRepo. It can optionally preload the repo with data
// from the passed config
func NewTargetRepo(c Cache, config map[domain.TargetKey][]domain.Target) (TargetRepo, error) {
	tr := TargetRepo{cache: c}
	if config == nil {
		return tr, nil
	}

	for key, cfg := range config {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
