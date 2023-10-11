package stream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/r3labs/sse/v2"
	"github.com/stretchr/testify/assert"
)

type mockSubscriber struct {
	sub func() (interface{}, error)
}

func (m *mockSubscriber) Sub(ctx context.Context, channel string, id string, fn HandleMessageFn) error {
	v, err := m.sub()
	if err != nil {
		return err
	}

	return fn("", v)
}

type mockMsgHandler struct {
	msg chan struct{}
}

func (m *mockMsgHandler) HandleMessage(ctx context.Context, msg domain.SSEMessage) error {
	m.msg <- struct{}{}
	return nil
}

func TestStream_Subscribe(t *testing.T) {
	type mocks struct {
		subscriber     *mockSubscriber
		messageHandler *mockMsgHandler
	}

	type expected struct {
		messagesHandled int
	}

	testCases := map[string]struct {
		mocks    mocks
		expected expected
	}{
		"Given I call Subscribe and the subscriber errors": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return nil, errors.New("an error")
					},
				},
				messageHandler: &mockMsgHandler{
					msg: make(chan struct{}),
				},
			},
			expected: expected{messagesHandled: 0},
		},
		"Given I call Subscribe and the subscriber returns a message that isn't an *sse.Event": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return "message", nil
					},
				},
				messageHandler: &mockMsgHandler{
					msg: make(chan struct{}),
				},
			},
			expected: expected{messagesHandled: 0},
		},
		"Given I call Subscribe and the subscriber returns a message that is an *sse.Event": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return &sse.Event{Data: []byte(`{}`)}, nil
					},
				},
				messageHandler: &mockMsgHandler{
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

			p := NewStream(log.NewNoOpLogger(), "foo", tc.mocks.subscriber, tc.mocks.messageHandler)
			p.Subscribe(ctx)

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
