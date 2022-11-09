package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/stream"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

type mockRedis struct {
	redis.UniversalClient
	xadd  func() *redis.StringCmd
	xread func() *redis.XStreamSliceCmd
}

func (m mockRedis) XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd {
	return m.xadd()
}

func (m mockRedis) XRead(ctx context.Context, args *redis.XReadArgs) *redis.XStreamSliceCmd {
	return m.xread()
}

func TestRedisCache_Pub(t *testing.T) {
	xaddError := func() *redis.StringCmd {
		return redis.NewStringResult("", errors.New("pub error"))
	}

	xaddSuccess := func() *redis.StringCmd {
		return redis.NewStringResult("message", nil)
	}

	testCases := map[string]struct {
		mockRedis mockRedis
		shouldErr bool
	}{
		"Given I have a redis stream that errors when I write to it": {
			mockRedis: mockRedis{
				xadd: xaddError,
			},
			shouldErr: true,
		},
		"Given I have a redis stream that doesn't error when I write to it": {
			mockRedis: mockRedis{
				xadd: xaddSuccess,
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			rc := RedisCache{client: tc.mockRedis}
			event := stream.StreamEvent{
				Values: map[stream.StreamEventValue]string{
					stream.StreamEventValueData: "hello world",
				},
			}

			err := rc.Pub(context.Background(), "foo", event)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}
		})
	}
}

func TestRedisCache_Sub(t *testing.T) {
	xreadError := func() *redis.XStreamSliceCmd {
		return redis.NewXStreamSliceCmdResult([]redis.XStream{}, errors.New("sub error"))
	}

	xreadSuccess := func() *redis.XStreamSliceCmd {
		xstreams := []redis.XStream{
			{
				Stream: "stream-foo",
				Messages: []redis.XMessage{
					{
						ID: "1642764292396-0",
						Values: map[string]interface{}{
							"HashedAPIKey": "123",
							"Data":         "hello world",
						},
					},
					{
						ID: "1642764292396-0",
						Values: map[string]interface{}{
							"HashedAPIKey": "123",
							"Data":         "foo bar",
						},
					},
				},
			},
		}
		return redis.NewXStreamSliceCmdResult(xstreams, nil)
	}

	testCases := map[string]struct {
		mockRedis mockRedis
		shouldErr bool
		expected  []stream.StreamEvent
	}{
		"Given I have a redis client that errors reading from a stream": {
			mockRedis: mockRedis{
				xread: xreadError,
			},
			shouldErr: true,
			expected:  []stream.StreamEvent{},
		},
		"Given I have a redis client that reads from a stream successfully": {
			mockRedis: mockRedis{
				xread: xreadSuccess,
			},
			shouldErr: false,
			expected: []stream.StreamEvent{
				{
					Checkpoint: stream.Checkpoint("1642764292396-0"),
					Values: map[stream.StreamEventValue]string{
						stream.StreamEventValueAPIKey: "123",
						stream.StreamEventValueData:   "hello world",
					},
				},
				{
					Checkpoint: stream.Checkpoint("1642764292396-0"),
					Values: map[stream.StreamEventValue]string{
						stream.StreamEventValueAPIKey: "123",
						stream.StreamEventValueData:   "foo bar",
					},
				},
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			rc := NewRedisCache(tc.mockRedis)

			actual := []stream.StreamEvent{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := rc.Sub(ctx, "foo", "", func(event stream.StreamEvent) {
				actual = append(actual, event)
				if len(actual) == 2 {
					cancel()
				}
			})
			if (err != nil && err != context.Canceled) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, actual, tc.expected)
		})
	}
}
