package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-proxy/v2/log"

	"github.com/harness/ff-proxy/v2/domain"
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
	case domain.MsgDomainFeature:
		return handleFeatureMessage(ctx, msg)
	case domain.MsgDomainSegment:
		return handleSegmentMessage(ctx, msg)
	case domain.MsgDomainProxy:
		return handleProxyMessage(ctx, msg)
	default:
		return fmt.Errorf("%w: %s", ErrUnexpectedMessageDomain, msg.Domain)
	}

}

func handleFeatureMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventDelete:
	case domain.EventPatch, domain.EventCreate:

	default:
		return fmt.Errorf("%w %q for FeatureMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

func handleSegmentMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventDelete:
	case domain.EventPatch, domain.EventCreate:

	default:
		return fmt.Errorf("%w %q for SegmentMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

func handleProxyMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventProxyKeyDeleted:
	case domain.EventEnvironmentAdded:
	case domain.EventEnvironmentRemoved:
	case domain.EventAPIKeyAdded:
	case domain.EventAPIKeyRemoved:
	default:
		return fmt.Errorf("%w %q for Proxymessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}
