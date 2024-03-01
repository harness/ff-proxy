package proxyservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-golang-server-sdk/rest"
	"github.com/harness/ff-proxy/v2/domain"
)

// QueryStore ...
type QueryStore struct {
	F            func(identifier string) (rest.FeatureConfig, error)
	S            func(identifier string) (rest.Segment, error)
	L            func() ([]rest.FeatureConfig, error)
	GetFlagMapFn func() (map[string]*rest.FeatureConfig, error)
}

// GetFlag returns a FeatureConfig from the QueryStore
func (q QueryStore) GetFlag(identifier string) (rest.FeatureConfig, error) {
	return q.F(identifier)
}

// GetSegment returns a Segment from the QueryStore
func (q QueryStore) GetSegment(identifier string) (rest.Segment, error) {
	return q.S(identifier)
}

// GetFlags returns FeatureConfigs from the QueryStore
func (q QueryStore) GetFlags() ([]rest.FeatureConfig, error) {
	return q.L()
}

// GetFlags returns FeatureConfigs from the QueryStore
func (q QueryStore) GetFlagMap() (map[string]*rest.FeatureConfig, error) {
	return q.GetFlagMapFn()
}

// GenerateQueryStore returns a QueryStore object which can be passed to the go sdk evaluator
// nolint:cyclop
func (s Service) GenerateQueryStore(ctx context.Context, environmentID string, segments map[string]domain.Segment) QueryStore {
	return QueryStore{
		F: func(identifier string) (rest.FeatureConfig, error) {
			// fetch feature
			flag, err := s.featureRepo.GetByIdentifier(ctx, environmentID, identifier)
			if err != nil {
				if errors.Is(err, domain.ErrCacheNotFound) {
					return rest.FeatureConfig{}, ErrNotFound
				}
				return rest.FeatureConfig{}, ErrInternal
			}

			return flag.ToSDKFeatureConfig(), nil
		},
		S: func(identifier string) (rest.Segment, error) {
			segment, ok := segments[identifier]
			if !ok {
				//if !errors.Is(err, domain.ErrCacheNotFound) {
				//	return rest.Segment{}, fmt.Errorf("%w: %s", ErrInternal, err)
				//}
				s.logger.Debug(ctx, "segments not found in cache: ")
			}
			return segment.ToSDKSegment(), nil

			// fetch segment
			//segment, err := s.segmentRepo.GetByIdentifier(ctx, environmentID, identifier)
			//if err != nil {
			//	if !errors.Is(err, domain.ErrCacheNotFound) {
			//		return rest.Segment{}, fmt.Errorf("%w: %s", ErrInternal, err)
			//	}
			//	s.logger.Debug(ctx, "segments not found in cache: ", "err", err.Error())
			//}

			//return segment.ToSDKSegment(), nil
		},
		L: func() ([]rest.FeatureConfig, error) {
			// fetch flags
			flags, err := s.featureRepo.Get(ctx, environmentID)
			if err != nil {
				if !errors.Is(err, domain.ErrCacheNotFound) {
					return nil, fmt.Errorf("%w: %s", ErrInternal, err)
				}
				s.logger.Debug(ctx, "flags not found in cache: ", "err", err.Error())
			}
			// TODO can/should we do this conversion in the repo layer instead?
			var restFlags []rest.FeatureConfig
			for _, flag := range flags {
				restFlags = append(restFlags, flag.ToSDKFeatureConfig())
			}

			return restFlags, nil
		},
		GetFlagMapFn: func() (map[string]*rest.FeatureConfig, error) {
			flags, err := s.featureRepo.Get(ctx, environmentID)
			if err != nil {
				if !errors.Is(err, domain.ErrCacheNotFound) {
					return nil, fmt.Errorf("%w: %s", ErrInternal, err)
				}
				s.logger.Debug(ctx, "flags not found in cache: ", "err", err.Error())
			}
			// TODO can/should we do this conversion in the repo layer instead?
			var flagMap = make(map[string]*rest.FeatureConfig, len(flags))
			for _, flag := range flags {
				f := flag.ToSDKFeatureConfig()
				flagMap[f.Feature] = &f
			}

			return flagMap, nil
		},
	}
}
