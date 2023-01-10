package proxyservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-golang-server-sdk/rest"
	"github.com/harness/ff-proxy/domain"
)

type QueryStore struct {
	F func(identifier string) (rest.FeatureConfig, error)
	S func(identifier string) (rest.Segment, error)
	L func() ([]rest.FeatureConfig, error)
}

func (q QueryStore) GetFlag(identifier string) (rest.FeatureConfig, error) {
	return q.F(identifier)
}

func (q QueryStore) GetSegment(identifier string) (rest.Segment, error) {
	return q.S(identifier)
}
func (q QueryStore) GetFlags() ([]rest.FeatureConfig, error) {
	return q.L()
}

// GenerateQueryStore returns a QueryStore object which can be passed to the go sdk evaluator
func (s Service) GenerateQueryStore(ctx context.Context, environmentID string) QueryStore {
	flagKey := domain.NewFeatureConfigKey(environmentID)
	segmentKey := domain.NewSegmentKey(environmentID)

	return QueryStore{
		F: func(identifier string) (rest.FeatureConfig, error) {
			// fetch feature
			flag, err := s.featureRepo.GetByIdentifier(ctx, flagKey, identifier)
			if err != nil {
				if errors.Is(err, domain.ErrCacheNotFound) {
					return rest.FeatureConfig{}, ErrNotFound
				}
				return rest.FeatureConfig{}, ErrInternal
			}
			return rest.FeatureConfig(flag), nil
		},
		S: func(identifier string) (rest.Segment, error) {
			// fetch segment
			segment, err := s.segmentRepo.GetByIdentifier(ctx, segmentKey, identifier)
			if err != nil {
				if !errors.Is(err, domain.ErrCacheNotFound) {
					return rest.Segment{}, fmt.Errorf("%w: %s", ErrInternal, err)
				}
				s.logger.Debug(ctx, "segments not found in cache: ", "err", err.Error())
			}
			return rest.Segment(segment), nil
		},
		L: func() ([]rest.FeatureConfig, error) {
			// fetch flags
			flags, err := s.featureRepo.Get(ctx, flagKey)
			if err != nil {
				if !errors.Is(err, domain.ErrCacheNotFound) {
					return nil, fmt.Errorf("%w: %s", ErrInternal, err)
				}
				s.logger.Debug(ctx, "flags not found in cache: ", "err", err.Error())
			}
			// TODO can/should we do this conversion in the repo layer instead?
			var restFlags []rest.FeatureConfig
			for _, flag := range flags {
				restFlags = append(restFlags, rest.FeatureConfig(flag))
			}

			return restFlags, nil
		},
	}
}
