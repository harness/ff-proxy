package stream

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRedisStream_Pub(t *testing.T) {
	type args struct {
		stream string
		msg    string
	}

	type expected struct {
		stream  string
		message map[string]interface{}
		err     error
	}

	testCases := map[string]struct {
		args      args
		shouldErr bool
		setErr    func(m *miniredis.Miniredis)
		expected  expected
	}{
		"Given I call Pub and the redis client doesn't error": {
			shouldErr: false,
			args: args{
				stream: "test-stream",
				msg:    "foo",
			},
			expected: expected{
				stream: "test-stream",
				message: map[string]interface{}{
					"event": "foo",
				},
			},
		},
		"Given I call Pub and the redis client errors": {
			shouldErr: true,
			args: args{
				stream: "test-stream",
				msg:    "foo",
			},
			setErr: func(m *miniredis.Miniredis) {
				m.SetError("an error")
			},
			expected: expected{
				err: ErrPublishing,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		m := miniredis.RunT(t)
		rc := redis.NewClient(&redis.Options{
			Addr: m.Addr(),
		})

		t.Run(desc, func(t *testing.T) {
			if tc.setErr != nil {
				tc.setErr(m)
			}

			rs := NewRedisStream(rc)
			err := rs.Pub(context.Background(), tc.args.stream, tc.args.msg)
			if tc.shouldErr {
				assert.NotNil(t, err)
				assert.True(t, errors.Is(err, tc.expected.err))
			} else {
				assert.Nil(t, err)

				xs, err := rc.XRead(context.Background(), &redis.XReadArgs{
					Streams: []string{"test-stream", "0"},
					Count:   0,
					Block:   0,
				}).Result()
				assert.Nil(t, err)

				for _, x := range xs {
					assert.Equal(t, "test-stream", x.Stream)
					for _, msg := range x.Messages {
						assert.Equal(t, tc.expected.message, msg.Values)
					}
				}
			}
		})
	}
}

func TestRedisStream_Sub(t *testing.T) {
	type expected struct {
		messages []interface{}
		err      error
	}

	testCases := map[string]struct {
		stream      string
		messages    []string
		setErr      func(m *miniredis.Miniredis)
		callbackErr error

		shouldErr bool

		expected expected
	}{
		"Given I have two messages and redis errors": {
			stream:   "test-stream",
			messages: []string{"foo", "bar"},
			setErr: func(m *miniredis.Miniredis) {
				m.SetError("an error")
			},

			shouldErr: true,
			expected: expected{
				messages: nil,
				err:      ErrSubscribing,
			},
		},
		"Given I have two messages and the callback errors I will still get both messages": {
			stream:      "test-stream",
			messages:    []string{"foo", "bar"},
			callbackErr: errors.New("callback error"),

			shouldErr: true,
			expected: expected{
				messages: []interface{}{"foo", "bar"},
				err:      context.Canceled,
			},
		},
		"Given I have two messages and the callback errors with EOF then I will NOT get both messages": {
			stream:      "test-stream",
			messages:    []string{"foo", "bar"},
			callbackErr: io.EOF,

			shouldErr: true,
			expected: expected{
				messages: []interface{}{"foo"},
				err:      io.EOF,
			},
		},
		"Given I have two messages and redis doesn't error": {
			stream:   "test-stream",
			messages: []string{"foo", "bar"},

			shouldErr: true,
			expected: expected{
				messages: []interface{}{"foo", "bar"},
				err:      context.Canceled,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		m := miniredis.RunT(t)
		rc := redis.NewClient(&redis.Options{
			Addr: m.Addr(),
		})

		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// First publish some messages so we have some to read when we subscribe
			redisStream := NewRedisStream(rc)
			for _, msg := range tc.messages {
				err := redisStream.Pub(ctx, tc.stream, msg)
				assert.Nil(t, err)
			}

			// SetErr in miniredis after we've published
			if tc.setErr != nil {
				tc.setErr(m)
			}

			actualMessages := []interface{}{}

			// Sub only exists when there's been a redis error or the context is canceled by the caller
			// so cancel the context once we've received all the messages we were expecting to prevent
			// the test from blocking
			err := redisStream.Sub(ctx, tc.stream, "0", func(id string, v interface{}) error {
				actualMessages = append(actualMessages, v)
				if len(actualMessages) == len(tc.expected.messages) {
					cancel()
				}
				return tc.callbackErr
			})
			if tc.shouldErr {
				assert.NotNil(t, err)
				assert.True(t, errors.Is(err, tc.expected.err))
			} else {
				assert.Nil(t, err)
			}

		})
	}
}
