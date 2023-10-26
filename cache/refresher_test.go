package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/log"
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
		"Given I have an SSEMessage with the domain 'proxyPatch' event 'foo'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainProxy,
					Event:  "foo",
				},
			},
			expected:  expected{err: ErrUnexpectedEventType},
			shouldErr: true,
		},
		"Given I have an SSEMessage with the event 'proxy' event 'proxyKeyDeleted'": {
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
					Domain:       domain.MsgDomainProxy,
					Event:        domain.EventEnvironmentAdded,
					Environments: []string{"123"},
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'environmentsRemoved'": {
			args: args{
				message: domain.SSEMessage{
					Domain:       domain.MsgDomainProxy,
					Event:        domain.EventEnvironmentRemoved,
					Environments: []string{"123"},
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'apiKeyAdded'": {
			args: args{
				message: domain.SSEMessage{
					Domain:       domain.MsgDomainProxy,
					Event:        domain.EventAPIKeyAdded,
					Environments: []string{"test_env"},
					APIKey:       "test_apikey",
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'proxy' event 'apiKeyRemoved'": {
			args: args{
				message: domain.SSEMessage{
					Domain:       domain.MsgDomainProxy,
					Event:        domain.EventAPIKeyRemoved,
					Environments: []string{"test_env"},
					APIKey:       "test_apikey",
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
	}

	mockClient := mockClientService{

		PageProxyConfigFn: func(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error) {
			return []domain.ProxyConfig{}, nil
		},

		FetchFeatureConfigForEnvironmentFn: func(ctx context.Context, authToken, envId string) ([]clientgen.FeatureConfig, error) {
			return []clientgen.FeatureConfig{}, nil
		},
	}

	authRepo := mockAuthRepo{
		addFn: func(ctx context.Context, values ...domain.AuthConfig) error {
			return nil
		},
		removeFn: func(ctx context.Context, id []string) error {
			return nil
		},
		patchAPIConfigForEnvironmentFn: func(ctx context.Context, envID, apikey, action string) error {
			return nil
		},
		removeAllKeysForEnvironmentFn: func(ctx context.Context, envID string) error {
			return nil
		},
	}
	flagRepo := mockFlagRepo{
		removeAllFeaturesForEnvironmentFn: func(ctx context.Context, id string) error {
			return nil
		},
		removeFn: func(ctx context.Context, env, id string) error {
			return nil
		},
		addFn: func(ctx context.Context, values ...domain.FlagConfig) error {
			return nil
		},
	}
	segmentRepo := mockSegmentRepo{
		removeFn: func(ctx context.Context, id string) error {
			return nil
		},
	}
	config := mockConfig{
		populate: func(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
			return nil
		},
		setProxyConfigFn: func(proxyConfig []domain.ProxyConfig) {},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			r := NewRefresher(log.NewNoOpLogger(), config, mockClient, authRepo, flagRepo, segmentRepo)
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

func TestRefresher_handleAddEnvironmentEvent(t *testing.T) {
	internalErr := errors.New("internal error")
	type args struct {
		message       domain.SSEMessage
		clientService mockClientService
	}

	type expected struct {
		err error
	}

	testCases := map[string]struct {
		args      args
		expected  expected
		shouldErr bool
	}{
		"Given I have error while attempting to fetch proxyConfig": {
			args: args{
				message: domain.SSEMessage{
					Domain:       domain.MsgDomainProxy,
					Event:        domain.EventEnvironmentAdded,
					Environments: []string{uuid.NewString(), uuid.NewString()},
				},
				clientService: mockClientService{
					PageProxyConfigFn: func(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error) {
						return []domain.ProxyConfig{}, internalErr
					},
				},
			},
			expected:  expected{err: internalErr},
			shouldErr: true,
		},
		"Given I have an environment list not empty fetch proxyConfig": {
			args: args{
				message: domain.SSEMessage{
					Domain:       domain.MsgDomainProxy,
					Event:        domain.EventEnvironmentAdded,
					Environments: []string{uuid.NewString(), uuid.NewString()},
				},
				clientService: mockClientService{
					PageProxyConfigFn: func(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error) {
						return []domain.ProxyConfig{}, nil
					},
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
	}

	authRepo := mockAuthRepo{
		addFn: func(ctx context.Context, values ...domain.AuthConfig) error {
			return nil
		},
		patchAPIConfigForEnvironmentFn: func(ctx context.Context, envID, apikey, action string) error {
			return nil
		},
	}
	flagRepo := mockFlagRepo{}
	segmentRepo := mockSegmentRepo{}
	config := mockConfig{
		populate: func(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
			return nil
		},
		setProxyConfigFn: func(proxyConfig []domain.ProxyConfig) {

		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			r := NewRefresher(log.NewNoOpLogger(), config, tc.args.clientService, authRepo, flagRepo, segmentRepo)
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

func TestRefresher_handleRemoveEnvironmentEvent(t *testing.T) {
	internalErr := errors.New("internal error")
	type args struct {
		message       domain.SSEMessage
		clientService mockClientService
	}

	type expected struct {
		err error
	}

	testCases := map[string]struct {
		args      args
		expected  expected
		shouldErr bool
	}{
		"Given I have error while attempting to fetch proxyConfig": {
			args: args{
				message: domain.SSEMessage{
					Domain:       domain.MsgDomainProxy,
					Event:        domain.EventEnvironmentAdded,
					Environments: []string{uuid.NewString(), uuid.NewString()},
				},
				clientService: mockClientService{
					PageProxyConfigFn: func(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error) {
						return []domain.ProxyConfig{}, internalErr
					},
				},
			},
			expected:  expected{err: internalErr},
			shouldErr: true,
		},
		"Given I have an environment list not empty fetch proxyConfig": {
			args: args{
				message: domain.SSEMessage{
					Domain:       domain.MsgDomainProxy,
					Event:        domain.EventEnvironmentAdded,
					Environments: []string{uuid.NewString(), uuid.NewString()},
				},
				clientService: mockClientService{
					PageProxyConfigFn: func(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error) {
						return []domain.ProxyConfig{}, nil
					},
				},
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
	}

	authRepo := mockAuthRepo{
		addFn: func(ctx context.Context, values ...domain.AuthConfig) error {
			return nil
		},
		patchAPIConfigForEnvironmentFn: func(ctx context.Context, envID, apikey, action string) error {
			return nil
		},
		removeAllKeysForEnvironmentFn: func(ctx context.Context, envID string) error {
			return nil
		},
	}
	flagRepo := mockFlagRepo{}
	segmentRepo := mockSegmentRepo{}
	config := mockConfig{
		populate: func(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
			return nil
		},
		setProxyConfigFn: func(proxyConfig []domain.ProxyConfig) {

		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			r := NewRefresher(log.NewNoOpLogger(), config, tc.args.clientService, authRepo, flagRepo, segmentRepo)
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

type mockConfig struct {
	fetchAndPopulate func(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error

	populate func(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error
	// Key returns proxyKey
	key func() string

	// Token returns the authToken that the Config uses to communicate with Harness SaaS
	token func() string

	// ClusterIdentifier returns the identifier of the cluster that the Config authenticated against
	clusterIdentifier func() string

	// SetProxyConfig the member
	setProxyConfigFn func(proxyConfig []domain.ProxyConfig)
}

func (m mockConfig) FetchAndPopulate(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
	return m.fetchAndPopulate(ctx, authRepo, flagRepo, segmentRepo)
}

func (m mockConfig) Populate(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
	return m.populate(ctx, authRepo, flagRepo, segmentRepo)
}

func (m mockConfig) Key() string {
	return "key"
}
func (m mockConfig) Token() string {
	return "token"
}
func (m mockConfig) ClusterIdentifier() string {
	return "1"
}

func (m mockConfig) SetProxyConfig(proxyConfig []domain.ProxyConfig) {
	m.setProxyConfigFn(proxyConfig)
}

type mockAuthRepo struct {
	addFn                          func(ctx context.Context, values ...domain.AuthConfig) error
	patchAPIConfigForEnvironmentFn func(ctx context.Context, envID, apikey, action string) error
	removeFn                       func(ctx context.Context, id []string) error
	removeAllKeysForEnvironmentFn  func(ctx context.Context, envID string) error
	addAPIConfigsForEnvironmentFn  func(ctx context.Context, envID string, apiKeys []string) error
}

func (m mockAuthRepo) AddAPIConfigsForEnvironment(ctx context.Context, envID string, apiKeys []string) error {
	return m.addAPIConfigsForEnvironmentFn(ctx, envID, apiKeys)
}

func (m mockAuthRepo) PatchAPIConfigForEnvironment(ctx context.Context, envID, apikey, action string) error {
	return m.patchAPIConfigForEnvironmentFn(ctx, envID, apikey, action)
}

func (m mockAuthRepo) Remove(ctx context.Context, id []string) error {
	return m.removeFn(ctx, id)
}

func (m mockAuthRepo) RemoveAllKeysForEnvironment(ctx context.Context, envID string) error {
	return m.removeAllKeysForEnvironmentFn(ctx, envID)
}

func (m mockAuthRepo) Add(ctx context.Context, values ...domain.AuthConfig) error {
	return m.addFn(ctx, values...)
}

type mockFlagRepo struct {
	addFn                             func(ctx context.Context, values ...domain.FlagConfig) error
	removeFn                          func(ctx context.Context, env, id string) error
	removeAllFeaturesForEnvironmentFn func(ctx context.Context, id string) error
}

func (m mockFlagRepo) RemoveAllFeaturesForEnvironment(ctx context.Context, id string) error {
	return m.removeAllFeaturesForEnvironmentFn(ctx, id)
}

func (m mockFlagRepo) Remove(ctx context.Context, env, id string) error {
	return m.removeFn(ctx, env, id)
}
func (m mockFlagRepo) Add(ctx context.Context, values ...domain.FlagConfig) error {
	return m.addFn(ctx, values...)
}

type mockSegmentRepo struct {
	addFn    func(ctx context.Context, values ...domain.SegmentConfig) error
	removeFn func(ctx context.Context, id string) error
}

func (m mockSegmentRepo) Remove(ctx context.Context, id string) error {
	return m.removeFn(ctx, id)
}

func (m mockSegmentRepo) Add(ctx context.Context, values ...domain.SegmentConfig) error {
	return m.addFn(ctx, values...)
}

type mockClientService struct {
	PageProxyConfigFn                  func(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error)
	FetchFeatureConfigForEnvironmentFn func(ctx context.Context, authToken, envId string) ([]clientgen.FeatureConfig, error)
}

func (c mockClientService) FetchFeatureConfigForEnvironment(ctx context.Context, authToken, envId string) ([]clientgen.FeatureConfig, error) {
	return c.FetchFeatureConfigForEnvironmentFn(ctx, authToken, envId)
}

func (c mockClientService) AuthenticateProxyKey(ctx context.Context, key string) (domain.AuthenticateProxyKeyResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (c mockClientService) PageProxyConfig(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error) {
	return c.PageProxyConfigFn(ctx, input)
}
