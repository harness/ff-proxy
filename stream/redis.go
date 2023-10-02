package stream

import (
	"context"
	"encoding"
	"fmt"

	"github.com/go-redis/redis/v8"
)

// RedisStream is a implementation of the Stream interface that is used for interacting with redis streams
type RedisStream struct {
	client redis.UniversalClient
	maxLen int64
}

// NewRedisStream creates a new redis streams client
func NewRedisStream(u redis.UniversalClient) RedisStream {
	return RedisStream{
		client: u,
	}
}

// Pub publishes events to a redis stream, if the stream doesn't exist it will create
// the stream and then publish the event
func (r RedisStream) Pub(ctx context.Context, stream string, v interface{}) error {
	var err error
	values := v

	// If the thing we want to publish implements the BinaryMarshaler interface
	// then use it's encoding
	bm, ok := v.(encoding.BinaryMarshaler)
	if ok {
		values, err = bm.MarshalBinary()
		if err != nil {
			return err
		}
	}

	if err := r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		ID:     "*",
		Values: map[string]interface{}{
			"index": values,
		},
		MaxLen: r.maxLen,
	}).Err(); err != nil {
		return fmt.Errorf(":%w: %s", ErrPublishing, err)
	}

	return nil
}
