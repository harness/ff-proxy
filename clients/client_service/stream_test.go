package clientservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/harness/ff-proxy/v2/stream"
	"github.com/r3labs/sse/v2"
	"github.com/stretchr/testify/assert"
)

type mockSubscriber struct {
	sub func() (interface{}, error)
}

func (m *mockSubscriber) Sub(ctx context.Context, channel string, id string, fn stream.HandleMessageFn) error {
	v, err := m.sub()
	if err != nil {
		return err
	}

	return fn("", v)
}

type mockMessageHandler struct {
	msg chan struct{}
}

func (m *mockMessageHandler) HandleMessage(msg domain.SSEMessage) error {
	m.msg <- struct{}{}
	return nil
}

func TestStream_Start(t *testing.T) {
	type mocks struct {
		subscriber     *mockSubscriber
		messageHandler *mockMessageHandler
	}

	type expected struct {
		messagesHandled int
	}

	testCases := map[string]struct {
		mocks    mocks
		expected expected
	}{
		"Given I call Start and the subscriber errors": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return nil, errors.New("an error")
					},
				},
				messageHandler: &mockMessageHandler{
					msg: make(chan struct{}),
				},
			},
			expected: expected{messagesHandled: 0},
		},
		"Given I call Start and the subscriber returns a message that isn't an *sse.Event": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return "message", nil
					},
				},
				messageHandler: &mockMessageHandler{
					msg: make(chan struct{}),
				},
			},
			expected: expected{messagesHandled: 0},
		},
		"Given I call Start and the subscriber returns a message that is an *sse.Event": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return &sse.Event{Data: []byte(`{}`)}, nil
					},
				},
				messageHandler: &mockMessageHandler{
					msg: make(chan struct{}),
				},
			},
			expected: expected{messagesHandled: 1},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			p := NewStream(log.NewNoOpLogger(), tc.mocks.subscriber, tc.mocks.messageHandler)
			p.Start(ctx)

			msgsHandled := 0
			for msgsHandled < tc.expected.messagesHandled {
				select {
				case <-ctx.Done():
					t.Errorf("timed out waiting for subscriber to finish")
				case <-tc.mocks.messageHandler.msg:
					msgsHandled++
				}
			}
			cancel()

			assert.Equal(t, tc.expected.messagesHandled, msgsHandled)

		})
	}
}
