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

func TestHealth_SetHealthy(t *testing.T) {

	type args struct {
	}

	type mocks struct {
		cache *mockCache
	}

	type expected struct {
		inMemStatus domain.StreamStatus
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given I call health but the cache errors getting the current status": {
			mocks: mocks{cache: &mockCache{getFn: func(value interface{}) error {
				return domain.ErrCacheInternal
			}}},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			shouldErr: true,
		},
		"Given the status of the stream in the cache is DISCONNECTED": {
			mocks: mocks{cache: &mockCache{getFn: func(value interface{}) error {
				value = domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 456,
				}
				return nil
			}}},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			h := Health{
				log: log.NoOpLogger{},
				c:   tc.mocks.cache,
				key: "foo",
				inMemStatus: domain.NewSafeStreamStatus(domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				}),
			}

			err := h.SetHealthy(context.Background())
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.inMemStatus.State, h.inMemStatus.Get().State)
		})
	}
}

func TestHealth_SetUnHealthy(t *testing.T) {

	type args struct {
		startingState domain.StreamState
	}

	type mocks struct {
		cache *mockCache
	}

	type expected struct {
		inMemStatus domain.StreamStatus
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given I call SetUnhealthy but the cache errors getting the current status": {
			args: args{
				startingState: domain.StreamStateConnected,
			},
			mocks: mocks{cache: &mockCache{getFn: func(value interface{}) error {
				return domain.ErrCacheInternal
			}}},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			shouldErr: true,
		},
		"Given the status of the stream in the cache is CONNECTED": {
			mocks: mocks{cache: &mockCache{getFn: func(value interface{}) error {
				value = domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 456,
				}
				return nil
			}}},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			h := Health{
				log: log.NoOpLogger{},
				c:   tc.mocks.cache,
				key: "foo",
				inMemStatus: domain.NewSafeStreamStatus(domain.StreamStatus{
					State: tc.args.startingState,
					Since: 123,
				}),
			}

			err := h.SetUnhealthy(context.Background())
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.inMemStatus.State, h.inMemStatus.Get().State)
		})
	}
}
