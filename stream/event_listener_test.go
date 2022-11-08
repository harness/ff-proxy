package stream

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/hash"
	"github.com/harness/ff-proxy/log"
	"github.com/r3labs/sse"
	"github.com/stretchr/testify/assert"
)

type mockStream struct {
	data map[string][]StreamEvent
	err  error
}

func newMockStream(err error, topic string, events ...StreamEvent) mockStream {
	return mockStream{
		data: map[string][]StreamEvent{
			topic: events,
		},
		err: err,
	}
}

func (m mockStream) Sub(ctx context.Context, topic string, checkpoint string, onReceive func(StreamEvent)) error {
	if m.err != nil {
		return m.err
	}

	events, ok := m.data[topic]
	if !ok {
		return fmt.Errorf("topic %q not found in mockStream", topic)
	}

	for _, event := range events {
		if Checkpoint(event.Checkpoint).IsOlder(Checkpoint(checkpoint)) {
			continue
		}

		onReceive(event)
	}
	return nil
}

func (m mockStream) Pub(ctx context.Context, topic string, event StreamEvent) error {
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
		expected  map[string][]StreamEvent
	}{
		"Given I try to publish an event containing a nil SSEEvent": {
			stream: mockStream{
				data: make(map[string][]StreamEvent),
				err:  nil,
			},
			event: stream.Event{
				APIKey:      apiKey,
				Environment: envID,
				SSEEvent:    nil,
			},
			shouldErr: true,
			expected:  map[string][]StreamEvent{},
		},
		"Given I have a mockStream that errors when the EventListener tries to publish to it": {
			stream: mockStream{
				data: make(map[string][]StreamEvent),
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
			expected:  map[string][]StreamEvent{},
		},
		"Given I have a mockStream that doesn't error when the EventListener tries to publish to it": {
			stream: mockStream{
				data: make(map[string][]StreamEvent),
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
			expected: map[string][]StreamEvent{
				envID: []StreamEvent{
					{
						Checkpoint: Checkpoint(""),
						Values: map[StreamEventValue]string{
							StreamEventValueAPIKey: envID,
							StreamEventValueData:   fmt.Sprintf("event: *\ndata: foo\n\n"),
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
