package ffproxy

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/log"
	"github.com/r3labs/sse"
	"github.com/stretchr/testify/assert"
	"github.com/wings-software/ff-server/pkg/hash"
)

type mockStream struct {
	data map[string][]map[string]interface{}
	err  error
}

func (m mockStream) Pub(ctx context.Context, topic string, values map[string]interface{}) error {
	if m.err != nil {
		return m.err
	}

	m.data[topic] = append(m.data[topic], values)
	return nil
}

func TestEventListener_Pub(t *testing.T) {
	apiKey := "123"
	hashedAPIKey := hash.NewSha256().Hash(apiKey)

	testCases := map[string]struct {
		stream    mockStream
		event     stream.Event
		shouldErr bool
		expected  map[string][]map[string]interface{}
	}{
		"Given I try to publish an event containing a nil SSEEvent": {
			stream: mockStream{
				data: make(map[string][]map[string]interface{}),
				err:  nil,
			},
			event: stream.Event{
				APIKey:      apiKey,
				Environment: "abc",
				SSEEvent:    nil,
			},
			shouldErr: true,
			expected:  make(map[string][]map[string]interface{}),
		},
		"Given I have a mockStream that errors when the EventListener tries to publish to it": {
			stream: mockStream{
				data: make(map[string][]map[string]interface{}),
				err:  errors.New("pub err"),
			},
			event: stream.Event{
				APIKey:      apiKey,
				Environment: "abc",
				SSEEvent: &sse.Event{
					ID:    []byte("1"),
					Data:  []byte("foo"),
					Event: []byte("patch"),
					Retry: []byte("nope"),
				},
			},
			shouldErr: true,
			expected:  make(map[string][]map[string]interface{}),
		},
		"Given I have a mockStream that doesn't error when the EventListener tries to publish to it": {
			stream: mockStream{
				data: make(map[string][]map[string]interface{}),
				err:  nil,
			},
			event: stream.Event{
				APIKey:      apiKey,
				Environment: "abc",
				SSEEvent: &sse.Event{
					ID:    []byte("1"),
					Data:  []byte("foo"),
					Event: []byte("patch"),
					Retry: []byte("nope"),
				},
			},
			shouldErr: false,
			expected: map[string][]map[string]interface{}{
				hashedAPIKey: []map[string]interface{}{
					{
						"ID":    []byte("1"),
						"Data":  []byte("foo"),
						"Event": []byte("patch"),
						"Retry": []byte("nope"),
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
