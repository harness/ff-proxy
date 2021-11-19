package repository

import (
	"context"
	"fmt"
	"github.com/harness/ff-proxy/cache"
	"time"

	"github.com/harness/ff-proxy/domain"
)

// SegmentRepo is a repository that stores Segments
type SegmentRepo struct {
	cache cache.Cache
}

// NewSegmentRepo creates a SegmentRepo. It can optionally preload the repo with data
// from the passed config
func NewSegmentRepo(c cache.Cache, config map[domain.SegmentKey][]domain.Segment) (SegmentRepo, error) {
	tr := SegmentRepo{cache: c}
	if config == nil {
		return tr, nil
	}

	for key, cfg := range config {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := tr.Add(ctx, key, cfg...); err != nil {
			cancel()
			return SegmentRepo{}, fmt.Errorf("failed to add config: %s", err)
		}
		cancel()
	}
	return tr, nil
}

// Add adds a target or multiple targets to the given key
func (t SegmentRepo) Add(ctx context.Context, key domain.SegmentKey, values ...domain.Segment) error {
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

// Get gets all of the Segments for a given key
func (t SegmentRepo) Get(ctx context.Context, key domain.SegmentKey) ([]domain.Segment, error) {
	results, err := t.cache.GetAll(ctx, string(key))
	if err != nil {
		return []domain.Segment{}, err
	}

	targets := []domain.Segment{}
	for _, b := range results {
		target := &domain.Segment{}
		if err := target.UnmarshalBinary(b); err != nil {
			return []domain.Segment{}, err
		}
		targets = append(targets, *target)
	}

	return targets, nil
}

// GetByIdentifier gets a Segment for a given key and identifer
func (t SegmentRepo) GetByIdentifier(ctx context.Context, key domain.SegmentKey, identifier string) (domain.Segment, error) {
	target := domain.Segment{}
	if err := t.cache.Get(ctx, string(key), identifier, &target); err != nil {
		return domain.Segment{}, err
	}
	return target, nil
}
