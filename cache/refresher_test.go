package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/stretchr/testify/assert"
)

func TestRefresher_HandleMessage(t *testing.T) {
	type args struct {
		message domain.SSEMessage
	}

	type expected struct {
		err error
	}

	testCases := map[string]struct {
		args      args
		expected  expected
		shouldErr bool
	}{
		"Given I have an SSEMessage with the domain 'Foo'": {
			args: args{
				message: domain.SSEMessage{Domain: "Foo"},
			},
			expected:  expected{err: ErrUnexpectedMessageDomain},
			shouldErr: true,
		},
		"Given I have an SSEMessage with the domain 'flag' event 'foo'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainFeature,
					Event:  "foo",
				},
			},
			expected:  expected{err: ErrUnexpectedEventType},
			shouldErr: true,
		},
		"Given I have an SSEMessage with the domain 'target-segment' event 'foo'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainSegment,
					Event:  "foo",
				},
			},
			expected:  expected{err: ErrUnexpectedEventType},
			shouldErr: true,
		},
		"Given I have an SSEMessage with the domain 'flag' event 'patch'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainFeature,
					Event:  patchEvent,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'flag' event 'create'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainFeature,
					Event:  createEvent,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'flag' event 'delete'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainFeature,
					Event:  deleteEvent,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'target-segment' event 'patch'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainSegment,
					Event:  patchEvent,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'target-segment' event 'create'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainSegment,
					Event:  createEvent,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'target-segment' event 'delete'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainSegment,
					Event:  deleteEvent,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'foo'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainProxy,
					Event:  "foo",
				},
			},
			expected:  expected{err: ErrUnexpectedEventType},
			shouldErr: true,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'proxyKeyDeleted'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainProxy,
					Event:  proxyKeyDeleted,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'environmentsAdded'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainProxy,
					Event:  environmentsAdded,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'environmentsRemoved'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainProxy,
					Event:  environmentsRemoved,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'apiKeyAdded'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainProxy,
					Event:  apiKeyAdded,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'apiKeyRemoved'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domainProxy,
					Event:  apiKeyRemoved,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			r := NewRefresher(log.NewNoOpLogger())
			err := r.HandleMessage(context.Background(), tc.args.message)
			if tc.shouldErr {
				assert.NotNil(t, err)
				assert.True(t, errors.Is(err, tc.expected.err))
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
