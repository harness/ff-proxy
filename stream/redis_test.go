package stream

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
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
					"index": "foo",
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
