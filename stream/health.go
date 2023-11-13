package stream

import (
	"context"
	"errors"
	"time"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
)

// Health maintains the health/status of a stream in a cache
type Health struct {
	c      cache.Cache
	key    string
	status domain.StreamStatus
}

// NewHealth creates a Health
func NewHealth(k string, c cache.Cache) Health {
	h := Health{
		key:    k,
		c:      c,
		status: domain.NewStreamStatus(),
	}

	defaultStreamStatus := domain.StreamStatus{
		State: domain.StreamStateInitializing,
		Since: time.Now().UnixMilli(),
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

// StreamStatus returns the StreamStatus from the cache
func (h Health) StreamStatus(ctx context.Context) (domain.StreamStatus, error) {
	var s domain.StreamStatus
	if err := h.c.Get(ctx, h.key, &s); err != nil {
		return domain.StreamStatus{}, err
	}

	return s, nil
}
