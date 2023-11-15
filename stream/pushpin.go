package stream

import (
	"context"
	"fmt"

	"github.com/fanout/go-gripcontrol"
	"github.com/fanout/go-pubcontrol"
	jsoniter "github.com/json-iterator/go"
)

// GripStream is the interface for publishing events to a grip channel
type gripStream interface {
	// PublishHttpStream publishes an HTTP stream format message to all the configured PubControlClients
	// with a specified channel, message, and optional ID, previous ID, and callback.
	// Note that the 'http_stream' parameter can be provided as either an HttpStreamFormat
	// instance or a string / byte array (in which case an HttpStreamFormat instance will
	// automatically be created and have the 'content' field set to the specified value).
	PublishHttpStream(channel string, content interface{}, id string, prevID string) error
	Publish(channel string, item *pubcontrol.Item) error
}

// Pushpin is a type that implements the Publisher interface and is used to publish to pushpin channels
type Pushpin struct {
	stream gripStream
}

// NewPushpin creates a new Pushpin
func NewPushpin(gs gripStream) Pushpin {
	return Pushpin{
		stream: gs,
	}
}

// Pub makes Pushpin implement the Publisher interface and is used to publish messages to a pushpin channel
func (p Pushpin) Pub(_ context.Context, channel string, value interface{}) error {
	b, err := jsoniter.Marshal(value)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal message to bytes: %s", ErrPublishing, err)
	}

	content := fmt.Sprintf("event: *\ndata: %s\n\n", b)
	if err := p.stream.PublishHttpStream(channel, content, "", ""); err != nil {
		return fmt.Errorf("PushpinStream: %w: %s", ErrPublishing, err)
	}

	return nil
}

func (p Pushpin) CloseStream(channel string) error {
	closeMsg := pubcontrol.NewItem([]pubcontrol.Formatter{&gripcontrol.HttpStreamFormat{Close: true}}, "", "")
	return p.stream.Publish(channel, closeMsg)
}
