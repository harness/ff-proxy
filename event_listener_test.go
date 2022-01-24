package ffproxy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	"github.com/r3labs/sse"
	"github.com/stretchr/testify/assert"
	"github.com/wings-software/ff-server/pkg/hash"
)

type mockStream struct {
	data map[string][]domain.StreamEvent
	err  error
}

func newMockStream(err error, topic string, events ...domain.StreamEvent) mockStream {
	return mockStream{
		data: map[string][]domain.StreamEvent{
			topic: events,
		},
		err: err,
	}
}

func (m mockStream) Sub(ctx context.Context, topic string, checkpoint string, onReceive func(domain.StreamEvent)) error {
	if m.err != nil {
		return m.err
	}

	events, ok := m.data[topic]
	if !ok {
		return fmt.Errorf("topic %q not found in mockStream", topic)
	}

	for _, event := range events {
		if domain.Checkpoint(event.Checkpoint).IsOlder(domain.Checkpoint(checkpoint)) {
			continue
		}

		onReceive(event)
	}
	return nil
}

func (m mockStream) Pub(ctx context.Context, topic string, event domain.StreamEvent) error {
	if m.err != nil {
		return m.err
	}

	m.data[topic] = append(m.data[topic], event)
	return nil
}

func TestEventListener_Pub(t *testing.T) {
	apiKey := "123"
	envID := "abc"

	testCases := map[string]struct {
		stream    mockStream
		event     stream.Event
		shouldErr bool
		expected  map[string][]domain.StreamEvent
	}{
		"Given I try to publish an event containing a nil SSEEvent": {
			stream: mockStream{
				data: make(map[string][]domain.StreamEvent),
				err:  nil,
			},
			event: stream.Event{
				APIKey:      apiKey,
				Environment: envID,
				SSEEvent:    nil,
			},
			shouldErr: true,
			expected:  map[string][]domain.StreamEvent{},
		},
		"Given I have a mockStream that errors when the EventListener tries to publish to it": {
			stream: mockStream{
				data: make(map[string][]domain.StreamEvent),
				err:  errors.New("pub err"),
			},
			event: stream.Event{
				APIKey:      apiKey,
				Environment: envID,
				SSEEvent: &sse.Event{
					ID:    []byte("1"),
					Data:  []byte("foo"),
					Event: []byte("patch"),
					Retry: []byte("nope"),
				},
			},
			shouldErr: true,
			expected:  map[string][]domain.StreamEvent{},
		},
		"Given I have a mockStream that doesn't error when the EventListener tries to publish to it": {
			stream: mockStream{
				data: make(map[string][]domain.StreamEvent),
				err:  nil,
			},
			event: stream.Event{
				APIKey:      apiKey,
				Environment: envID,
				SSEEvent: &sse.Event{
					ID:    []byte("1"),
					Data:  []byte("foo"),
					Event: []byte("patch"),
					Retry: []byte("nope"),
				},
			},
			shouldErr: false,
			expected: map[string][]domain.StreamEvent{
				envID: []domain.StreamEvent{
					{
						Checkpoint: domain.Checkpoint(""),
						Values: map[domain.StreamEventValue]string{
							domain.StreamEventValueAPIKey: envID,
							domain.StreamEventValueData:   fmt.Sprintf("event: *\ndata: foo\n\n"),
						},
					},
				},
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			el := NewEventListener(log.NoOpLogger{}, tc.stream, hash.NewSha256())

			err := el.Pub(context.Background(), tc.event)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}
			assert.Equal(t, tc.expected, tc.stream.data)
		})
	}
}
