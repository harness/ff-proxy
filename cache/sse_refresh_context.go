package cache

import (
	"context"

	"github.com/harness/ff-proxy/v2/domain"
)

type sseRefreshCtxKey struct{}

// sseRefreshMeta carries SSE arrival size through FlagRepo/SegmentRepo into HashCache.Set
// so growth metrics reuse the marshal HashCache already performs (no second full encode).
type sseRefreshMeta struct {
	arrivalBytes int
	domain       string
	event        string
	kind         string
}

// ContextWithSSERefresh attaches SSE arrival metadata for HashCache.Set to observe
// collection size / growth without an extra marshal on the Refresher hot path.
func ContextWithSSERefresh(ctx context.Context, msg domain.SSEMessage, kind string) context.Context {
	return context.WithValue(ctx, sseRefreshCtxKey{}, sseRefreshMeta{
		arrivalBytes: msg.RawSize,
		domain:       msg.Domain,
		event:        msg.Event,
		kind:         kind,
	})
}

func sseRefreshMetaFrom(ctx context.Context) (sseRefreshMeta, bool) {
	m, ok := ctx.Value(sseRefreshCtxKey{}).(sseRefreshMeta)
	return m, ok
}
