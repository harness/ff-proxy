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

	cachedState domain.StreamStatus
}

func (m *mockCache) Get(ctx context.Context, key string, value interface{}) error {
	return m.getFn(value)
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}) error {
	t, ok := value.(domain.StreamStatus)
	if !ok {
		panic("ah")
	}
	m.cachedState = t
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

			h := PrimaryHealth{
				log:         log.NoOpLogger{},
				c:           tc.mocks.cache,
				key:         "foo",
				inMemStatus: domain.NewSafeStreamStatus(tc.args.inMemStatus),
			}

			ctx, cancel := context.WithTimeout(context.Background(), tc.args.interval*2)
			defer cancel()

			h.VerifyStreamStatus(ctx, tc.args.interval)

			actual := tc.mocks.cache.cachedState

			assert.Equal(t, tc.expected.streamStatus, actual)
		})
	}

}

func TestHealth_SetHealthy(t *testing.T) {

	type args struct {
		startingInMemStatus  domain.StreamStatus
		startingCachedStatus domain.StreamStatus
	}

	type mocks struct {
		cache *mockCache
	}

	type expected struct {
		inMemStatus  domain.StreamStatus
		cachedStatus domain.StreamStatus
	}

	testCases := map[string]struct {
		then      string
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given the cachedState and inMemoryState=DISCONNECTED, I call SetHealthy and the Cache returns ErrInternal": {
			then: "Then the inMemoryState shoud be CONNECTED and the cachedState should be DISCONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return domain.ErrCacheInternal
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			shouldErr: true,
		},
		"Given the cachedState and inMemoryState=DISCONNECTED, I call SetHealthy and the Cache returns context.Canceled": {
			then: "Then the inMemoryState shoud be CONNECTED and the cachedState should be DISCONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return context.Canceled
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			shouldErr: true,
		},
		"Given the cachedState and inMemoryState=DISCONNECTED, I call SetHealthy and the Cache returns context.DeadlineExceeded": {
			then: "Then the inMemoryState shoud be CONNECTED and the cachedState should be DISCONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return context.DeadlineExceeded
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			shouldErr: true,
		},
		"Given the cachedState and inMemoryState=DISCONNECTED, I call SetHealthy and the Cache returns no error": {
			then: "Then the inMemoryState shoud be CONNECTED and the cachedState should be CONNNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return nil
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			shouldErr: false,
		},
		"Given the cachedState and inMemoryState=CONNECTED, I call SetHealthy and the Cache returns no error": {
			then: "Then the inMemoryState shoud be CONNECTED and the cachedState should be CONNNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return nil
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			shouldErr: false,
		},
		"Given the cachedState=CONNECTED and inMemoryState=DISCONNECTED, I call SetHealthy and the Cache returns no error": {
			then: "Then the inMemoryState shoud be CONNECTED and the cachedState should be CONNNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return nil
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			shouldErr: false,
		},
		"Given the cachedState=DISCONNECTED and inMemoryState=CONNECTED, I call SetHealthy and the Cache returns no error": {
			then: "Then the inMemoryState shoud be CONNECTED and the cachedState should be CONNNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return nil
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
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

			tc.mocks.cache.cachedState = tc.args.startingCachedStatus
			h := PrimaryHealth{
				log:         log.NoOpLogger{},
				c:           tc.mocks.cache,
				key:         "foo",
				inMemStatus: domain.NewSafeStreamStatus(tc.args.startingInMemStatus),
			}

			err := h.SetHealthy(context.Background())
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			t.Log(tc.then)
			assert.Equal(t, tc.expected.inMemStatus.State, h.inMemStatus.Get().State)
			assert.Equal(t, tc.expected.cachedStatus.State, tc.mocks.cache.cachedState.State)
		})
	}
}

func TestHealth_SetUnhealthy(t *testing.T) {

	type args struct {
		startingInMemStatus  domain.StreamStatus
		startingCachedStatus domain.StreamStatus
	}

	type mocks struct {
		cache *mockCache
	}

	type expected struct {
		inMemStatus  domain.StreamStatus
		cachedStatus domain.StreamStatus
	}

	testCases := map[string]struct {
		then      string
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given the cachedState and inMemoryState=CONNECTED, I call SetUnhealthy and the Cache returns ErrInternal": {
			then: "Then the inMemoryState should be DISCONNECTED and the cachedState should be CONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return domain.ErrCacheInternal
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			shouldErr: true,
		},
		"Given the cachedState and inMemoryState=CONNECTED, I call SetUnhealthy and the Cache returns context.Canceled": {
			then: "Then the inMemoryState should be DISCONNECTED and the cachedState should be CONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return context.Canceled
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			shouldErr: true,
		},
		"Given the cachedState and inMemoryState=CONNECTED, I call SetUnhealthy and the Cache returns context.DeadlineExceeded": {
			then: "Then the inMemoryState should be DISCONNECTED and the cachedState should be CONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return context.DeadlineExceeded
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			shouldErr: true,
		},
		"Given the cachedState and inMemoryState=CONNECTED, I call SetUnhealthy and the Cache returns no error": {
			then: "Then the inMemoryState should be DISCONNECTED and the cachedState should be DISCONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return nil
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			shouldErr: false,
		},
		"Given the cachedState and inMemoryState=DISCONNECTED, I call SetUnhealthy and the Cache returns no error": {
			then: "Then the inMemoryState should be DISCONNECTED and the cachedState should be DISCONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return nil
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			shouldErr: false,
		},
		"Given the cachedState=DISCONNECTED and inMemoryState=CONNECTED, I call SetUnhealthy and the Cache returns no error": {
			then: "Then the inMemoryState should be DISCONNECTED and the cachedState should be DISCONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return nil
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
			},
			shouldErr: false,
		},
		"Given the cachedState=CONNECTED and inMemoryState=DISCONNECTED, I call SetUnhealthy and the Cache returns no error": {
			then: "Then the inMemoryState should be DISCONNECTED and the cachedState should be DISCONNECTED",
			args: args{
				startingInMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				startingCachedStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 123,
				},
			},
			mocks: mocks{
				cache: &mockCache{
					getFn: func(value interface{}) error {
						return nil
					},
				},
			},
			expected: expected{
				inMemStatus: domain.StreamStatus{
					State: domain.StreamStateDisconnected,
					Since: 123,
				},
				cachedStatus: domain.StreamStatus{
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

			tc.mocks.cache.cachedState = tc.args.startingCachedStatus
			h := PrimaryHealth{
				log: log.NoOpLogger{},
				c:   tc.mocks.cache,
				key: "foo",
				inMemStatus: domain.NewSafeStreamStatus(domain.StreamStatus{
					State: tc.args.startingInMemStatus.State,
					Since: tc.args.startingInMemStatus.Since,
				}),
			}

			err := h.SetUnhealthy(context.Background())
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			t.Log(tc.then)
			assert.Equal(t, tc.expected.inMemStatus.State, h.inMemStatus.Get().State)
			assert.Equal(t, tc.expected.cachedStatus.State, tc.mocks.cache.cachedState.State)
		})
	}
}
