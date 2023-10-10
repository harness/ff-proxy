package clientservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/harness/ff-proxy/v2/stream"
	jsoniter "github.com/json-iterator/go"
	"github.com/r3labs/sse/v2"
)

type messageHandler interface {
	HandleMessage(ctx context.Context, m domain.SSEMessage) error
}

// Stream is the type that subscribes to stream that relays Proxy events and handles events coming off the stream
type Stream struct {
	log            log.Logger
	subscriber     stream.Subscriber
	messageHandler messageHandler
}

// NewStream opens a subscription to the client service's stream endpoint
func NewStream(l log.Logger, s stream.Subscriber, m messageHandler) Stream {
	l = l.With("component", "Start")
	return Stream{
		log:            l,
		subscriber:     s,
		messageHandler: m,
	}
}

// Start connects to the stream and registers a handler to handle events coming off the stream.
// If the stream disconnects backoff and attempt to subscribe again.
func (s Stream) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				err := s.subscriber.Sub(ctx, "*", "", func(id string, v interface{}) error {
					msg, err := parseMessage(v)
					if err != nil {
						return nil
					}

					if err := s.messageHandler.HandleMessage(ctx, msg); err != nil {
						return nil
					}

					return nil
				})
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}

					time.Sleep(30 * time.Second)
				}
			}
		}
	}()
}

func parseMessage(v interface{}) (domain.SSEMessage, error) {
	event, ok := v.(*sse.Event)
	if !ok {
		return domain.SSEMessage{}, fmt.Errorf("expected message type to be *sse.Event, got=%T", v)
	}

	m := domain.SSEMessage{}
	if err := jsoniter.Unmarshal(event.Data, &m); err != nil {
		return domain.SSEMessage{}, err
	}

	return m, nil
}
