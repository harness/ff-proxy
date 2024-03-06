package stream

import (
	"context"
	"errors"
	"time"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

// Health maintains the health/status of a stream in a cache
type Health struct {
	log log.Logger
	c   cache.Cache
	key string

	// Also keep a copy of the status in memory. This way we can
	// recover if we failed to update the remote status in the cache
	// due to a network error and avoid getting in a stuck state.
	inMemStatus *domain.SafeStreamStatus
}

// NewHealth creates a Health
func NewHealth(k string, c cache.Cache, l log.Logger) Health {
	l = l.With("component", "StreamHealth")

	defaultStreamStatus := domain.StreamStatus{
		State: domain.StreamStateInitializing,
		Since: time.Now().UnixMilli(),
	}

	h := Health{
		log:         l,
		key:         k,
		c:           c,
		inMemStatus: domain.NewSafeStreamStatus(defaultStreamStatus),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// It's fine for us to ignore this error, if we fail to set the status
	// to initialising we'll end up setting it as Connected or Disconnected
	// in our OnConnect/OnDisconnect handlers when we attempt to stream
	h.log.Info("setting stream status in cache", "state", defaultStreamStatus.State, "since", defaultStreamStatus.Since)
	_ = h.c.Set(ctx, h.key, defaultStreamStatus)

	return h
}

// SetHealthy sets the stream status as CONNECTED in the cache.
// If the stream status is already CONNECTED it does nothing.
func (h Health) SetHealthy(ctx context.Context) error {
	streamStatus := domain.StreamStatus{
		State: domain.StreamStateConnected,
		Since: time.Now().UnixMilli(),
	}

	h.log.Info("SetHealthy - Updating streamStatus", "streamStatus.State", streamStatus.State, "streamStatus.Since", streamStatus.Since)

	cachedStatus := domain.StreamStatus{}

	if err := h.c.Get(ctx, h.key, &cachedStatus); err != nil {
		// Ignore NotFound errors for this key because if the key doesn't
		// exist we'll end up setting it at the end of this function
		if !errors.Is(err, domain.ErrCacheNotFound) {
			h.inMemStatus.Set(streamStatus)
			return err
		}
	}

	// If current status is healthy then don't do anything
	if cachedStatus.State == domain.StreamStateConnected {
		return nil
	}

	h.log.Info("setting stream status in cache", "state", streamStatus.State, "since", streamStatus.Since)
	h.inMemStatus.Set(streamStatus)
	return h.c.Set(ctx, h.key, streamStatus)
}

// SetUnhealthy sets the stream status as DISCONNECTED in the cache.
// If the stream status is already DISCONNECTED it does nothing.
func (h Health) SetUnhealthy(ctx context.Context) error {
	streamStatus := domain.StreamStatus{
		State: domain.StreamStateDisconnected,
		Since: time.Now().UnixMilli(),
	}

	cachedStatus := domain.StreamStatus{}
	if err := h.c.Get(ctx, h.key, &cachedStatus); err != nil {
		// Ignore NotFound errors for this key because if the key doesn't
		// exist we'll end up setting it at the end of this function
		if !errors.Is(err, domain.ErrCacheNotFound) {
			h.inMemStatus.Set(streamStatus)
			return err
		}
	}

	// If current status is disconnected then we don't need to do anything
	if cachedStatus.State == domain.StreamStateDisconnected {
		return nil
	}

	h.log.Info("setting stream status in cache", "state", streamStatus.State, "since", streamStatus.Since)
	h.inMemStatus.Set(streamStatus)
	return h.c.Set(ctx, h.key, streamStatus)
}

// VerifyStreamStatus checks that the stream status recorded in the cache matches the stream status recorded in memory.
// There was an issue where if we failed to update the stream status in the cache due to a network error that it could
// be stuck as INITIALIZING until there was a disconnect between the Harness Saas stream and the Proxy. During this time
// where the state was stuck as INITIALIZING any SDK stream requests would be rejected by replicas.
//
// This thread should resolve that issue because the stream status stored in memory isn't affected by network issues so
// should always be up to date.
func (h Health) VerifyStreamStatus(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.log.Info("context canceled, stopping thread that checks the cached stream status matches the in memory status")
			return
		case <-ticker.C:

			var cachedStatus domain.StreamStatus
			inMemStatus := h.inMemStatus.Get()

			if err := h.c.Get(ctx, h.key, &cachedStatus); err != nil {
				h.log.Error("failed to get stream status from cache", "err", err)
			}

			h.log.Info("verifying stream status", "in_mem_status_state", inMemStatus.State, "in_mem_status_since", inMemStatus.Since, "cached_status_state", cachedStatus.State, "cached_status_since", cachedStatus.Since)

			// The inMemState should always be accurate, if there's a difference between it and the
			// cachedState then it's possible there was a network error when we tried to update the
			// cachedState in SetHealthy or SetUnhealthy and we should try to update the cachedState again
			if cachedStatus.State != inMemStatus.State {
				h.log.Info("setting stream status in cache", "state", inMemStatus.State, "since", inMemStatus.Since)
				if err := h.c.Set(ctx, h.key, inMemStatus); err != nil {
					h.log.Error("failed to update cached stream state to match in memory stream state", "err", err)
				}
			}
		}
	}
}

// StreamStatus returns the StreamStatus from the cache
func (h Health) StreamStatus(_ context.Context) (domain.StreamStatus, error) {
	return h.inMemStatus.Get(), nil
}
