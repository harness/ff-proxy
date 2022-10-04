package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/cache"

	"github.com/harness/ff-proxy/domain"
)

// SegmentRepo is a repository that stores Segments
type SegmentRepo struct {
	cache cache.Cache
}

// NewSegmentRepo creates a SegmentRepo. It can optionally preload the repo with data
// from the passed config
func NewSegmentRepo(c cache.Cache, config map[domain.SegmentKey][]domain.Segment) (SegmentRepo, error) {
	sr := SegmentRepo{cache: c}
	if config == nil {
		return sr, nil
	}

	for key, cfg := range config {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		// cleanup all current keys before we add new ones to make sure keys that have been deleted remotely are removed
		sr.cache.RemoveAll(ctx, string(key))
		if err := sr.Add(ctx, key, cfg...); err != nil {
			cancel()
			return SegmentRepo{}, fmt.Errorf("failed to add config: %s", err)
		}
		cancel()
	}
	return sr, nil
}

// Add adds a segment or multiple segments to the given key
func (t SegmentRepo) Add(ctx context.Context, key domain.SegmentKey, values ...domain.Segment) error {
	errs := []error{}
	for _, v := range values {
		if err := t.cache.Set(ctx, string(key), v.Identifier, &v); err != nil {
			errs = append(errs, addErr{string(key), v.Identifier, err})
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add segment(s) to repo: %v", errs)
	}
	return nil
}

// Get gets all of the Segments for a given key
func (t SegmentRepo) Get(ctx context.Context, key domain.SegmentKey) ([]domain.Segment, error) {
	results, err := t.cache.GetAll(ctx, string(key))
	if err != nil {
		return []domain.Segment{}, err
	}

	segments := make([]domain.Segment, len(results))

	idx := 0
	for _, b := range results {
		segment := &domain.Segment{}
		if err := segment.UnmarshalBinary(b); err != nil {
			return []domain.Segment{}, err
		}
		segments[idx] = *segment
		idx++
	}

	return segments, nil
}

// GetAsMap gets all of the Segments for a given key and returns them in a map
func (t SegmentRepo) GetAsMap(ctx context.Context, key domain.SegmentKey) (map[string]domain.Segment, error) {
	results, err := t.cache.GetAll(ctx, string(key))
	if err != nil {
		return map[string]domain.Segment{}, err
	}

	segments := make(map[string]domain.Segment, len(results))

	for _, b := range results {
		segment := &domain.Segment{}
		if err := segment.UnmarshalBinary(b); err != nil {
			return map[string]domain.Segment{}, err
		}
		segments[segment.Identifier] = *segment
	}

	return segments, nil
}

// GetByIdentifier gets a Segment for a given key and identifer
func (t SegmentRepo) GetByIdentifier(ctx context.Context, key domain.SegmentKey, identifier string) (domain.Segment, error) {
	segment := domain.Segment{}
	if err := t.cache.Get(ctx, string(key), identifier, &segment); err != nil {
		return domain.Segment{}, err
	}
	return segment, nil
}
