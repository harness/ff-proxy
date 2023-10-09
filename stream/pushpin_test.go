package stream

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fanout/go-pubcontrol"
)

type mockGripStream struct {
	pub func() error
}

func (m mockGripStream) PublishHttpStream(channel string, content interface{}, id string, prevID string) error {
	return m.pub()
}

func (m mockGripStream) Publish(channel string, item *pubcontrol.Item) error {
	return nil
}

func TestPushpin_Pub(t *testing.T) {
	type args struct {
		message interface{}
	}
	type mocks struct {
		gripStream mockGripStream
	}

	type expected struct {
		err error
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given I call Pub and the gripStream client errors": {
			args: args{message: "foo"},
			mocks: mocks{gripStream: mockGripStream{
				pub: func() error {
					return errors.New("an error")
				},
			}},
			shouldErr: true,
			expected:  expected{err: ErrPublishing},
		},
		"Given I call Pub and the gripStream client doesn't error": {
			args: args{message: "foo"},
			mocks: mocks{gripStream: mockGripStream{
				pub: func() error {
					return nil
				},
			}},
			shouldErr: false,
			expected:  expected{err: nil},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			p := NewPushpin(tc.mocks.gripStream)
			err := p.Pub(context.Background(), "", "foo")
			if tc.shouldErr {
				assert.NotNil(t, err)
				assert.True(t, errors.Is(err, tc.expected.err))
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
