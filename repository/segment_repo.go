package repository

import (
	"context"
	"fmt"

	"github.com/harness/ff-proxy/v2/cache"

	"github.com/harness/ff-proxy/v2/domain"
)

// SegmentOption defines functional option for a SegmentRepo
type SegmentOption func(s *SegmentRepo)

// WithSegmentConfig populates a SegmentRepo with the given config
func WithSegmentConfig(config map[domain.SegmentKey]interface{}) SegmentOption {
	return func(s *SegmentRepo) {
		for key, value := range config {
			// cleanup all current keys before we add new ones to make sure keys that have been deleted remotely are removed
			_ = s.cache.Delete(context.Background(), string(key))

			// Don't bother saving an empty slice
			if s, ok := value.([]domain.Segment); ok {
				if s == nil || len(s) == 0 {
					return
				}
			}

			// Don't bother adding a nil target to the cache
			if s, ok := value.(*domain.Segment); ok && s == nil {
				return
			}

			s.cache.Set(context.Background(), string(key), value)
		}
	}
}

// SegmentRepo is a repository that stores Segments
type SegmentRepo struct {
	cache cache.Cache
}

// NewSegmentRepo creates a SegmentRepo. It can optionally preload the repo with data
// from the passed config
func NewSegmentRepo(c cache.Cache, opts ...SegmentOption) (SegmentRepo, error) {
	sr := SegmentRepo{cache: c}

	for _, opt := range opts {
		opt(&sr)
	}

	return sr, nil
}

// Get gets all of the Segments for a given key
func (s SegmentRepo) Get(ctx context.Context, envID string) ([]domain.Segment, error) {
	var segments []domain.Segment
	key := domain.NewSegmentsKey(envID)

	err := s.cache.Get(ctx, string(key), &segments)
	if err != nil {
		return []domain.Segment{}, err
	}

	return segments, nil
}

// GetByIdentifier gets a Segment for a given key and identifer
func (s SegmentRepo) GetByIdentifier(ctx context.Context, envID string, identifier string) (domain.Segment, error) {
	segment := domain.Segment{}
	key := domain.NewSegmentKey(envID, identifier)

	if err := s.cache.Get(ctx, string(key), &segment); err != nil {
		return domain.Segment{}, err
	}
	return segment, nil
}

func (s SegmentRepo) Add(ctx context.Context, config ...domain.SegmentConfig) error {
	errs := []error{}

	for _, cfg := range config {
		k := domain.NewSegmentsKey(cfg.EnvironmentID)

		if err := s.cache.Set(ctx, string(k), cfg.Segments); err != nil {
			errs = append(errs, addErr{
				key:        string(k),
				identifier: "segments",
				err:        err,
			})
		}

		for _, seg := range cfg.Segments {
			key := domain.NewSegmentKey(cfg.EnvironmentID, seg.Identifier)

			if err := s.cache.Set(ctx, string(key), seg); err != nil {
				errs = append(errs, addErr{
					key:        string(key),
					identifier: seg.Identifier,
					err:        err,
				})
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add segmentConfig(s) to cache: %v", errs)
	}

	return nil
}
