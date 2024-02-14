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
	_ = h.c.Set(ctx, h.key, defaultStreamStatus)

	return h
}

// SetHealthy sets the stream status as CONNECTED in the cache.
// If the stream status is already CONNECTED it does nothing.
func (h Health) SetHealthy(ctx context.Context) error {
	var streamStatus domain.StreamStatus

	defer func() {
		h.inMemStatus.Set(streamStatus)
	}()

	if err := h.c.Get(ctx, h.key, &streamStatus); err != nil {
		// Ignore NotFound errors for this key because if the key doesn't
		// exist we'll end up setting it at the end of this function
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return err
		}
	}

	// If current status is healthy then don't do anything
	if streamStatus.State == domain.StreamStateConnected {
		return nil
	}

	streamStatus.State = domain.StreamStateConnected
	streamStatus.Since = time.Now().UnixMilli()

	return h.c.Set(ctx, h.key, streamStatus)
}

// SetUnhealthy sets the stream status as DISCONNECTED in the cache.
// If the stream status is already DISCONNECTED it does nothing.
func (h Health) SetUnhealthy(ctx context.Context) error {
	var streamStatus domain.StreamStatus

	defer func() {
		h.inMemStatus.Set(streamStatus)
	}()

	if err := h.c.Get(ctx, h.key, &streamStatus); err != nil {
		// Ignore NotFound errors for this key because if the key doesn't
		// exist we'll end up setting it at the end of this function
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return err
		}
	}

	// If current status is disconnected then we don't need to do anything
	if streamStatus.State == domain.StreamStateDisconnected {
		return nil
	}

	// Otherwise we update the state and since to be now
	streamStatus.State = domain.StreamStateDisconnected
	streamStatus.Since = time.Now().UnixMilli()

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

			// The inMemState should always be accurate, if there's a difference between it and the
			// cachedState then it's possible there was a network error when we tried to update the
			// cachedState in SetHealthy or SetUnhealthy and we should try to update the cachedState again
			if cachedStatus.State != inMemStatus.State {
				if err := h.c.Set(ctx, h.key, inMemStatus); err != nil {
					h.log.Error("failed to update cached stream state to match in memory stream state", "err", err)
				}
			}
		}
	}
}

// StreamStatus returns the StreamStatus from the cache
func (h Health) StreamStatus(ctx context.Context) (domain.StreamStatus, error) {
	var s domain.StreamStatus
	if err := h.c.Get(ctx, h.key, &s); err != nil {
		return domain.StreamStatus{}, err
	}

	return s, nil
}
