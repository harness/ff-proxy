package repository

import (
	"context"
	"fmt"

	"github.com/harness/ff-proxy/v2/cache"

	"github.com/harness/ff-proxy/v2/domain"
)

// SegmentRepo is a repository that stores Segments
type SegmentRepo struct {
	cache cache.HashCacher
}

// NewSegmentRepo creates a SegmentRepo. It can optionally preload the repo with data
// from the passed config
func NewSegmentRepo(c *cache.HashCacher) SegmentRepo {
	return SegmentRepo{cache: *c}
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

// Add stores SegmentConfig in the cache
func (s SegmentRepo) Add(ctx context.Context, config ...domain.SegmentConfig) error {
	errs := []error{}

	for _, cfg := range config {
		k := domain.NewSegmentsKey(cfg.EnvironmentID)

		if err := s.cache.Set(ctx, string(k), cfg.Segments); err != nil {
			errs = append(errs, addError{
				key:        string(k),
				identifier: "segments",
				err:        err,
			})
		}

		hashK, err := s.cache.AddHashKey(ctx, string(k), cfg.Segments)
		if err != nil {
			errs = append(errs, addError{
				key:        hashK,
				identifier: "segments",
				err:        err,
			})
		}

		for _, seg := range cfg.Segments {
			key := domain.NewSegmentKey(cfg.EnvironmentID, seg.Identifier)
			if err := s.cache.Set(ctx, string(key), seg); err != nil {
				errs = append(errs, addError{
					key:        string(key),
					identifier: seg.Identifier,
					err:        err,
				})
			}
			hashKey, err := s.cache.AddHashKey(ctx, string(key), cfg.Segments)
			if err != nil {
				errs = append(errs, addError{
					key:        hashKey,
					identifier: string(key),
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

// GetSegmentsForEnvironment gets all the segments associated with environment id
func (s SegmentRepo) GetSegmentsForEnvironment(ctx context.Context, envID string) ([]domain.Segment, bool) {
	var segments []domain.Segment
	key := domain.NewSegmentsKey(envID)
	if err := s.cache.Get(ctx, string(key), &segments); err != nil {
		return segments, false
	}
	return segments, true
}

// RemoveAllSegmentsForEnvironment removes all segments entries for given environment id
func (s SegmentRepo) RemoveAllSegmentsForEnvironment(ctx context.Context, id string) error {
	//get all the segments for given key
	segments, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	// remove segmentConfig entry
	sKey := domain.NewSegmentsKey(id)
	if err := s.cache.Delete(ctx, string(sKey)); err != nil {
		return err
	}
	// remove all individual segment entries for environment
	for _, segment := range segments {

		key := domain.NewSegmentKey(id, segment.Identifier)
		if err := s.cache.Delete(ctx, string(key)); err != nil {
			return err
		}
	}
	return nil
}

// Remove removes the Segment entry from the cache
func (s SegmentRepo) Remove(ctx context.Context, identifier string) error {
	return s.cache.Delete(ctx, identifier)
}
