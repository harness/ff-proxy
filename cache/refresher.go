package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-proxy/v2/log"

	"github.com/harness/ff-proxy/v2/domain"
)

const (
	// domainFeature identifies flag messages from ff server or stream
	domainFeature = "flag"

	// domainSegment identifies segment messages from ff server or stream
	domainSegment = "target-segment"

	// domainProxy identifiers proxy messages from the ff server
	domainProxy = "proxy"

	// patchEvent identifies a patch event from the SSE stream
	patchEvent = "patch"

	// deleteEvent identifies a delete event from the SSE stream
	deleteEvent = "delete"

	//createEvent identifies a create event from the SSE stream
	createEvent = "create"

	proxyKeyDeleted     = "proxyKeyDeleted"
	environmentsAdded   = "environmentsAdded"
	environmentsRemoved = "environmentsRemoved"
	apiKeyAdded         = "apiKeyAdded"
	apiKeyRemoved       = "apiKeyRemoved"
)

var (
	// ErrUnexpectedMessageDomain is the error returned when an SSE message has a message domain we aren't expecting
	ErrUnexpectedMessageDomain = errors.New("unexpected message domain")

	// ErrUnexpectedEventType is the error returned when an SSE message has an event type we aren't expecting
	ErrUnexpectedEventType = errors.New("unexpected event type")
)

// Refresher is a type for handling SSE events from Harness Saas
type Refresher struct {
	log log.Logger
}

// NewRefresher creates a Refresher
func NewRefresher(l log.Logger) Refresher {
	l = l.With("component", "Refresher")
	return Refresher{log: l}
}

// HandleMessage makes Refresher implement the MessageHandler interface
func (s Refresher) HandleMessage(ctx context.Context, msg domain.SSEMessage) error {
	switch msg.Domain {
	case domainFeature:
		return handleFeatureMessage(ctx, msg)
	case domainSegment:
		return handleSegmentMessage(ctx, msg)
	case domainProxy:
		return handleProxyMessage(ctx, msg)
	default:
		return fmt.Errorf("%w: %s", ErrUnexpectedMessageDomain, msg.Domain)
	}

}

func handleFeatureMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case deleteEvent:
	case patchEvent, createEvent:

	default:
		return fmt.Errorf("%w %q for FeatureMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

func handleSegmentMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case deleteEvent:
	case patchEvent, createEvent:

	default:
		return fmt.Errorf("%w %q for SegmentMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

func handleProxyMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case proxyKeyDeleted:
	case environmentsAdded:
	case environmentsRemoved:
	case apiKeyAdded:
	case apiKeyRemoved:
	default:
		return fmt.Errorf("%w %q for Proxymessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}
