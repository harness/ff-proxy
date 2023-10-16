package stream

import (
	"context"

	"github.com/harness/ff-proxy/v2/cache"
)

// Health maintains the health/status of a stream in a cache
type Health struct {
	c   cache.Cache
	key string
}

func NewHealth(k string, c cache.Cache) Health {
	return Health{
		key: k,
		c:   c,
	}
}

func (h Health) SetHealthy(ctx context.Context) error {
	return h.c.Set(ctx, h.key, true)
}

func (h Health) SetUnhealthy(ctx context.Context) error {
	return h.c.Set(ctx, h.key, false)
}

func (h Health) StreamHealthy(ctx context.Context) (bool, error) {
	var b bool
	if err := h.c.Get(ctx, h.key, &b); err != nil {
		return b, err
	}

	return b, nil
}
