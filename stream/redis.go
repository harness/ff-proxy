package stream

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"io"

	"github.com/go-redis/redis/v8"

	"github.com/harness/ff-proxy/v2/domain"
)

// WithMaxLen sets the max length for a redis stream
func WithMaxLen(i int64) func(r *RedisStream) {
	return func(r *RedisStream) {
		r.maxLen = i
	}
}

// RedisStream is a implementation of the Stream interface that is used for interacting with redis streams
type RedisStream struct {
	client redis.UniversalClient
	maxLen int64
}

// NewRedisStream creates a new redis streams client
func NewRedisStream(u redis.UniversalClient, opts ...func(r *RedisStream)) RedisStream {
	r := &RedisStream{
		client: u,
		maxLen: 1000, // Default to 1000 if not set
	}

	for _, opt := range opts {
		opt(r)
	}

	return *r
}

// Pub publishes events to a redis stream, if the stream doesn't exist it will create
// the stream and then publish the event.
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
		Values: formatRedisMessage(values),
		MaxLen: r.maxLen,
	}).Err(); err != nil {
		return fmt.Errorf("RedisStream: %w: %s", ErrPublishing, err)
	}

	return nil
}

// Sub subscribes to a redis stream starting at the id provided. If an id isn't provided then it will start at the last
// message on the stream. Sub only exits if there is an error communicating with
// redis or the context has been cancelled by the caller.
func (r RedisStream) Sub(ctx context.Context, stream string, id string, handleMessage domain.HandleMessageFn) error {
	if id == "" {
		id = "$"
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			xs, err := r.client.XRead(ctx, &redis.XReadArgs{
				Streams: []string{stream, id},
				Count:   0,
				Block:   0,
			}).Result()
			if err != nil {
				return fmt.Errorf("RedisStream: %w: %s", ErrSubscribing, err)
			}

			for _, x := range xs {
				for _, msg := range x.Messages {
					if err := handleMessage(msg.ID, parseRedisMessage(msg.Values)); err != nil {
						// If we get an EOF error then we'll want to bubble this up since this
						// signals that there's been a disconnect
						if errors.Is(err, io.EOF) {
							return err
						}
						continue
					}
				}
			}
		}
	}
}

func (r RedisStream) Close(_ string) error {
	return nil
}

func formatRedisMessage(v interface{}) map[string]interface{} {
	return map[string]interface{}{
		"event": v,
	}
}

func parseRedisMessage(m map[string]interface{}) interface{} {
	v, ok := m["event"]
	if !ok {
		return nil
	}
	return v
}
