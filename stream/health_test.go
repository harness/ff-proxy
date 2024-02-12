package stream

import (
	"context"
	"testing"
	"time"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/stretchr/testify/assert"
)

type mockCache struct {
	cache.Cache
	getFn func(value interface{}) error

	value interface{}
}

func (m *mockCache) Get(ctx context.Context, key string, value interface{}) error {
	return m.getFn(value)
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}) error {
	m.value = value
	return nil
}

func TestHealth_VerifyStreamStatus(t *testing.T) {
	type args struct {
		interval    time.Duration
		inMemStatus domain.StreamStatus
	}

	type mocks struct {
		cache *mockCache
	}

	type expected struct {
		streamStatus domain.StreamStatus
	}

	testCases := map[string]struct {
		args     args
		mocks    mocks
		expected expected
	}{
		"Given the inMemoryStreamStatus is different from the cachedStreamStatus": {
			args: args{
				interval: 2 * time.Second,
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 456,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						value = domain.StreamStatus{
							State: domain.StreamStateInitializing,
							Since: 123,
						}
						return nil
					},
				},
			},
			expected: expected{
				streamStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 456,
				},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			h := Health{
				log:         log.NoOpLogger{},
				c:           tc.mocks.cache,
				key:         "foo",
				inMemStatus: domain.NewSafeStreamStatus(tc.args.inMemStatus),
			}

			ctx, cancel := context.WithTimeout(context.Background(), tc.args.interval*2)
			defer cancel()

			h.VerifyStreamStatus(ctx, tc.args.interval)

			actual := tc.mocks.cache.value

			assert.Equal(t, tc.expected.streamStatus, actual)
		})
	}

}
