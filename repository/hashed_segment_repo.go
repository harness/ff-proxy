package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
	gocache "github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
)

type internalCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, v interface{}, d time.Duration)
}

type HashedSegmentRepo struct {
	localcache internalCache
	cache      cache.Cache

	hashHit       *prometheus.CounterVec
	fullcacheRead *prometheus.CounterVec
}

func NewHashedSegmentRepo(c cache.Cache, r prometheus.Registerer) HashedSegmentRepo {
	h := HashedSegmentRepo{
		localcache: gocache.New(1*time.Minute, 2*time.Minute),
		cache:      c,

		hashHit: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "hashed_segment_repo_hash_hit",
			Help: "tracks the number of hash reads",
		},
			[]string{"key"},
		),
		fullcacheRead: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "hashed_segment_repo_full_cache_read",
			Help: "tracks the number of full cache reads",
		},
			[]string{"key"},
		),
	}

	r.MustRegister(h.hashHit, h.fullcacheRead)
	return h
}

func (h HashedSegmentRepo) GetByIdentifier(ctx context.Context, env string, identifier string) (domain.Segment, error) {
	key := domain.NewSegmentKey(env, identifier)
	latestKey := fmt.Sprintf("%s-latest", key)

	h.hashHit.WithLabelValues(latestKey).Inc()
	hash, err := h.cache.GetHash(ctx, latestKey)
	if err != nil {
		return domain.Segment{}, err
	}

	if data, ok := h.localcache.Get(hash); ok {
		if t, ok := data.(domain.Segment); ok {
			return t, nil
		}
		return domain.Segment{}, fmt.Errorf("Ahhh")
	}

	var segment domain.Segment
	h.fullcacheRead.WithLabelValues(string(key)).Inc()
	if err := h.cache.Get(ctx, string(key), segment); err != nil {
		return domain.Segment{}, nil
	}

	h.localcache.Set(hash, segment, 0)
	return segment, nil
}

func (h HashedSegmentRepo) Get(ctx context.Context, env string) ([]domain.Segment, error) {
	key := domain.NewSegmentsKey(env)
	latestKey := fmt.Sprintf("%s-latest", key)

	h.hashHit.WithLabelValues(latestKey).Inc()
	hash, err := h.cache.GetHash(ctx, latestKey)
	if err != nil {
		return []domain.Segment{}, err
	}

	if data, ok := h.localcache.Get(hash); ok {
		if t, ok := data.([]domain.Segment); ok {
			return t, nil
		}
		return []domain.Segment{}, fmt.Errorf("Ahhh")
	}

	var segments []domain.Segment
	h.fullcacheRead.WithLabelValues(string(key)).Inc()
	if err := h.cache.Get(ctx, string(key), &segments); err != nil {
		return []domain.Segment{}, nil
	}

	h.localcache.Set(hash, segments, 0)
	return segments, nil
}
