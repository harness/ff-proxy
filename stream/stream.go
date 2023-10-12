package stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	jsoniter "github.com/json-iterator/go"
	"github.com/r3labs/sse/v2"
)

var (
	// ErrPublishing is the error returned when we fail to publish an event to a stream
	ErrPublishing = errors.New("failed to publish to stream")

	ErrSubscribing = errors.New("failed to subscribe to stream")
)

// Publisher defines the interface for publishing to a stream
type Publisher interface {
	Pub(ctx context.Context, channel string, value interface{}) error
}

// Subscriber defines the interface for subscribing to a stream
type Subscriber interface {
	Sub(ctx context.Context, channel string, id string, message HandleMessageFn) error
}

// HandleMessageFn is the function that gets called whenever a subscriber receives a message on a stream
type HandleMessageFn func(id string, v interface{}) error

// Stream defines a type that can subscribe to a stream and handle events that come off it
type Stream struct {
	log            log.Logger
	topic          string
	subscriber     Subscriber
	messageHandler domain.MessageHandler
}

// NewStream opens a subscription to the client service's stream endpoint
func NewStream(l log.Logger, topic string, s Subscriber, m domain.MessageHandler) Stream {
	l = l.With("component", "Stream", "topic", topic)
	return Stream{
		log:            l,
		topic:          topic,
		subscriber:     s,
		messageHandler: m,
	}
}

// Subscribe connects to the stream and registers a handler to handle events coming off the stream.
// If the stream disconnects backoff and attempt to subscribe again.
func (s Stream) Subscribe(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msgID := ""
				err := s.subscriber.Sub(ctx, s.topic, msgID, func(id string, v interface{}) error {
					msg, err := parseMessage(v)
					if err != nil {
						return nil
					}

					msgID = id

					if err := s.messageHandler.HandleMessage(ctx, msg); err != nil {
						return nil
					}

					return nil
				})
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}

					s.log.Warn("disconnected from stream, backing off and retrying in 30 seconds: %s", err)
					time.Sleep(30 * time.Second)
				}
			}
		}
	}()
}

// This logic should maybe move to be inside the specific messageHandler implementations, that way we could
// keep the Stream type completely generic/reusable for different types of messages instead of constantly
// having to update this to decode different message formats
func parseMessage(v interface{}) (domain.SSEMessage, error) {
	if s, ok := v.(string); ok {
		m := domain.SSEMessage{}
		if err := jsoniter.Unmarshal([]byte(s), &m); err != nil {
			return m, err
		}
		return m, nil
	}

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
