package stream

import (
	"context"
	"fmt"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/r3labs/sse/v2"
	"gopkg.in/cenkalti/backoff.v1"
)

// SSE is the interface for the underlying SSE client we're using
type SSE interface {
	SubscribeWithContext(ctx context.Context, stream string, handler func(msg *sse.Event)) error
}

// SSEClient is an implementation of the Subscriber interface for interacting with SSE Streams
type SSEClient struct {
	log log.Logger
	sse SSE
}

// NewSSEClient creates an SSEClient
func NewSSEClient(l log.Logger, url string, key string, token string) *SSEClient {
	c := sse.NewClient(url)
	c.Headers = map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"API-Key":       key,
	}

	// don't use the default exponentialBackoff strategy - we'll have our own disconnect logic
	// that we'll implement
	c.ReconnectStrategy = &backoff.StopBackOff{}

	return &SSEClient{
		log: l,
		sse: c,
	}
}

// Sub makes SSEClient implement the Stream & Subscriber interfaces
func (s *SSEClient) Sub(ctx context.Context, channel string, _ string, fn domain.HandleMessageFn) error {
	err := s.sse.SubscribeWithContext(ctx, channel, func(msg *sse.Event) {
		// If we get a message with no data we just want to carry on and receive the next message
		if len(msg.Data) <= 0 {
			return
		}

		// If the callback handling the message errors we probably don't want to bubble
		// the error up and kill the subscription so just log it and carry on
		if err := fn("", msg); err != nil {
			s.log.Warn("failed to handle message", "err", err)
		}
	})
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSubscribing, err)
	}
	return nil
}

// Pub ...
// TODO: Temporarily adding this to make this type implement the Stream interface. There's some
// cleaner refactoring I can do around this and the pushpin type but I don't want to make this
// PR bigger than it needs to be.
func (s *SSEClient) Pub(_ context.Context, _ string, _ interface{}) error {
	return nil
}
