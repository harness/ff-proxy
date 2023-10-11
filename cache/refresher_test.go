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
					Domain: domain.MsgDomainFeature,
					Event:  "foo",
				},
			},
			expected:  expected{err: ErrUnexpectedEventType},
			shouldErr: true,
		},
		"Given I have an SSEMessage with the domain 'target-segment' event 'foo'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainSegment,
					Event:  "foo",
				},
			},
			expected:  expected{err: ErrUnexpectedEventType},
			shouldErr: true,
		},
		"Given I have an SSEMessage with the domain 'flag' event 'patch'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainFeature,
					Event:  domain.EventPatch,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'flag' event 'create'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainFeature,
					Event:  domain.EventCreate,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'flag' event 'delete'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainFeature,
					Event:  domain.EventDelete,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'target-segment' event 'patch'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainSegment,
					Event:  domain.EventPatch,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'target-segment' event 'create'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainSegment,
					Event:  domain.EventCreate,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'target-segment' event 'delete'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainSegment,
					Event:  domain.EventDelete,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'foo'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainProxy,
					Event:  "foo",
				},
			},
			expected:  expected{err: ErrUnexpectedEventType},
			shouldErr: true,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'proxyKeyDeleted'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainProxy,
					Event:  domain.EventProxyKeyDeleted,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'environmentsAdded'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainProxy,
					Event:  domain.EventEnvironmentAdded,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'environmentsRemoved'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainProxy,
					Event:  domain.EventEnvironmentRemoved,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'apiKeyAdded'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainProxy,
					Event:  domain.EventAPIKeyAdded,
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'apiKeyRemoved'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainProxy,
					Event:  domain.EventAPIKeyRemoved,
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
