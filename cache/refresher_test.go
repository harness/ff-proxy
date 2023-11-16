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
		"Given I have an SSEMessage with the domain 'proxy' event 'deleteProxyKey'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainProxy,
					Event:  domain.EventProxyKeyDeleted,
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
		FetchSegmentConfigForEnvironmentFn: func(ctx context.Context, authToken, envId string) ([]clientgen.Segment, error) {
			return []clientgen.Segment{}, nil
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
		getKeysForEnvironmentFn: func(ctx context.Context, envID string) ([]string, error) {
			return []string{}, nil
		},
	}
	flagRepo := mockFlagRepo{
		removeAllFeaturesForEnvironmentFn: func(ctx context.Context, id string) error {
			return nil
		},
		removeFn: func(ctx context.Context, id string) error {
			return nil
		},
		addFn: func(ctx context.Context, values ...domain.FlagConfig) error {
			return nil
		},
		getFeatureConfigForEnvironmentFn: func(ctx context.Context, envID string) ([]domain.FeatureFlag, bool) {
			return []domain.FeatureFlag{}, true
		},
	}
	segmentRepo := mockSegmentRepo{

		addFn: func(ctx context.Context, values ...domain.SegmentConfig) error {
			return nil
		},
		removeFn: func(ctx context.Context, id string) error {
			return nil
		},
		removeAllSegmentsForEnvironmentFn: func(ctx context.Context, id string) error {
			return nil
		},
		getSegmentsForEnvironmentFn: func(ctx context.Context, envID string) ([]domain.Segment, bool) {
			return []domain.Segment{}, true
		},
	}
	config := mockConfig{
		populate: func(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
			return nil
		},
		setProxyConfigFn: func(proxyConfig []domain.ProxyConfig) {},
	}
	inventoryRepo := mockInventoryRepo{
		addFn: func(ctx context.Context, key string, assets map[string]string) error {
			return nil
		},
		removeFn: func(ctx context.Context, key string) error {
			return nil
		},
		getFn: func(ctx context.Context, key string) (map[string]string, error) {
			return map[string]string{}, nil
		},
		patchFn: func(ctx context.Context, key string, patch func(assets map[string]string) (map[string]string, error)) error {
			return nil
		},
		buildAssetListFromConfigFn: func(config []domain.ProxyConfig) (map[string]string, error) {
			return map[string]string{}, nil
		},
		cleanupFn: func(ctx context.Context, key string, config []domain.ProxyConfig) error {
			return nil
		},
		getKeysForEnvironmentFn: func(ctx context.Context, env string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			r := NewRefresher(log.NewNoOpLogger(), config, mockClient, inventoryRepo, authRepo, flagRepo, segmentRepo)
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

	inventoryRepo := mockInventoryRepo{
		patchFn: func(ctx context.Context, key string, patch func(assets map[string]string) (map[string]string, error)) error {
			return nil
		},

		buildAssetListFromConfigFn: func(config []domain.ProxyConfig) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			r := NewRefresher(log.NewNoOpLogger(), config, tc.args.clientService, inventoryRepo, authRepo, flagRepo, segmentRepo)
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

	type mocks struct {
		authRepo      mockAuthRepo
		flagRepo      mockFlagRepo
		segmentRepo   mockSegmentRepo
		inventoryRepo mockInventoryRepo
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
		key: func() string {
			return "key"
		},
	}
	inventoryRepo := mockInventoryRepo{
		patchFn: func(ctx context.Context, key string, patch func(assets map[string]string) (map[string]string, error)) error {
			return nil
		},
		buildAssetListFromConfigFn: func(config []domain.ProxyConfig) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
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
			mocks: mocks{
				authRepo:      authRepo,
				flagRepo:      flagRepo,
				segmentRepo:   segmentRepo,
				inventoryRepo: inventoryRepo,
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
			mocks: mocks{
				authRepo:      authRepo,
				flagRepo:      flagRepo,
				segmentRepo:   segmentRepo,
				inventoryRepo: inventoryRepo,
			},
			expected:  expected{err: nil},
			shouldErr: false,
		},
		// This tests that we'll call all repo.Remove funcs and not just error out on the first one
		// that returns a domain.ErrCacheNotFound
		"Given I call handleRemoveEnvironmentEvent and all the repos return domain.ErrCacheNotFound": {
			args: args{
				message: domain.SSEMessage{
					Domain:       domain.MsgDomainProxy,
					Event:        domain.EventEnvironmentRemoved,
					Environments: []string{uuid.NewString(), uuid.NewString()},
				},
				clientService: mockClientService{
					PageProxyConfigFn: func(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error) {
						return []domain.ProxyConfig{}, nil
					},
				},
			},
			mocks: mocks{
				authRepo: mockAuthRepo{
					getKeysForEnvironmentFn: func(ctx context.Context, envID string) ([]string, error) {
						return []string{}, domain.ErrCacheNotFound
					},
					removeAllKeysForEnvironmentFn: func(ctx context.Context, envID string) error {
						return domain.ErrCacheNotFound
					},
				},
				flagRepo: mockFlagRepo{
					removeAllFeaturesForEnvironmentFn: func(ctx context.Context, id string) error {
						return domain.ErrCacheNotFound
					},
				},
				segmentRepo: mockSegmentRepo{
					removeAllSegmentsForEnvironmentFn: func(ctx context.Context, envID string) error {
						return domain.ErrCacheNotFound
					},
				},
				inventoryRepo: mockInventoryRepo{
					patchFn: func(ctx context.Context, key string, patch func(assets map[string]string) (map[string]string, error)) error {
						return domain.ErrCacheInternal
					},
					getKeysForEnvironmentFn: func(ctx context.Context, env string) (map[string]string, error) {
						return map[string]string{}, domain.ErrCacheNotFound
					},
				},
			},
			expected:  expected{err: domain.ErrCacheInternal},
			shouldErr: true,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			r := NewRefresher(log.NewNoOpLogger(), config, tc.args.clientService, tc.mocks.inventoryRepo, tc.mocks.authRepo, tc.mocks.flagRepo, tc.mocks.segmentRepo)
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
	fetchAndPopulate func(ctx context.Context, inventoryRepo domain.InventoryRepo, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error

	populate func(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error
	// Key returns proxyKey
	key func() string

	// Token returns the authToken that the Config uses to communicate with Harness SaaS
	token func() string

	refreshToken func() (string, error)

	// ClusterIdentifier returns the identifier of the cluster that the Config authenticated against
	clusterIdentifier func() string

	// SetProxyConfig the member
	setProxyConfigFn func(proxyConfig []domain.ProxyConfig)
}

type mockInventoryRepo struct {
	addFn                      func(ctx context.Context, key string, assets map[string]string) error
	removeFn                   func(ctx context.Context, key string) error
	getFn                      func(ctx context.Context, key string) (map[string]string, error)
	patchFn                    func(ctx context.Context, key string, patch func(assets map[string]string) (map[string]string, error)) error
	buildAssetListFromConfigFn func(config []domain.ProxyConfig) (map[string]string, error)
	cleanupFn                  func(ctx context.Context, key string, config []domain.ProxyConfig) error
	keyExistsFn                func(ctx context.Context, key string) bool
	getKeysForEnvironmentFn    func(ctx context.Context, env string) (map[string]string, error)
}

func (m mockInventoryRepo) Add(ctx context.Context, key string, assets map[string]string) error {
	return m.addFn(ctx, key, assets)
}

func (m mockInventoryRepo) Remove(ctx context.Context, key string) error {
	return m.removeFn(ctx, key)
}

func (m mockInventoryRepo) Get(ctx context.Context, key string) (map[string]string, error) {
	return m.getFn(ctx, key)
}

func (m mockInventoryRepo) Patch(ctx context.Context, key string, patch func(assets map[string]string) (map[string]string, error)) error {
	return m.patchFn(ctx, key, patch)
}

func (m mockInventoryRepo) BuildAssetListFromConfig(config []domain.ProxyConfig) (map[string]string, error) {
	return m.buildAssetListFromConfigFn(config)
}

func (m mockInventoryRepo) Cleanup(ctx context.Context, key string, config []domain.ProxyConfig) error {
	return m.cleanupFn(ctx, key, config)
}

func (m mockInventoryRepo) KeyExists(ctx context.Context, key string) bool {
	return m.keyExistsFn(ctx, key)
}

func (m mockInventoryRepo) GetKeysForEnvironment(ctx context.Context, env string) (map[string]string, error) {
	return m.getKeysForEnvironmentFn(ctx, env)
}

func (m mockConfig) FetchAndPopulate(ctx context.Context, inventory domain.InventoryRepo, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
	return m.fetchAndPopulate(ctx, inventory, authRepo, flagRepo, segmentRepo)
}

func (m mockConfig) Populate(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
	return m.populate(ctx, authRepo, flagRepo, segmentRepo)
}

func (m mockConfig) RefreshToken() (string, error) {
	if m.refreshToken == nil {
		return "", nil
	}
	return m.refreshToken()
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
	getKeysForEnvironmentFn        func(ctx context.Context, envID string) ([]string, error)
}

func (m mockAuthRepo) GetKeysForEnvironment(ctx context.Context, envID string) ([]string, error) {
	return m.getKeysForEnvironmentFn(ctx, envID)
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
	removeFn                          func(ctx context.Context, id string) error
	removeAllFeaturesForEnvironmentFn func(ctx context.Context, id string) error
	getFeatureConfigForEnvironmentFn  func(ctx context.Context, envID string) ([]domain.FeatureFlag, bool)
}

func (m mockFlagRepo) GetFeatureConfigForEnvironment(ctx context.Context, envID string) ([]domain.FeatureFlag, bool) {
	return m.getFeatureConfigForEnvironmentFn(ctx, envID)
}

func (m mockFlagRepo) RemoveAllFeaturesForEnvironment(ctx context.Context, id string) error {
	return m.removeAllFeaturesForEnvironmentFn(ctx, id)
}

func (m mockFlagRepo) Remove(ctx context.Context, id string) error {
	return m.removeFn(ctx, id)
}
func (m mockFlagRepo) Add(ctx context.Context, values ...domain.FlagConfig) error {
	return m.addFn(ctx, values...)
}

type mockSegmentRepo struct {
	addFn                             func(ctx context.Context, values ...domain.SegmentConfig) error
	removeFn                          func(ctx context.Context, id string) error
	removeAllSegmentsForEnvironmentFn func(ctx context.Context, envID string) error
	getSegmentsForEnvironmentFn       func(ctx context.Context, envID string) ([]domain.Segment, bool)
}

func (m mockSegmentRepo) GetSegmentsForEnvironment(ctx context.Context, envID string) ([]domain.Segment, bool) {
	return m.getSegmentsForEnvironmentFn(ctx, envID)
}

func (m mockSegmentRepo) Remove(ctx context.Context, id string) error {
	return m.removeFn(ctx, id)
}

func (m mockSegmentRepo) RemoveAllSegmentsForEnvironment(ctx context.Context, id string) error {
	return m.removeAllSegmentsForEnvironmentFn(ctx, id)
}

func (m mockSegmentRepo) Add(ctx context.Context, values ...domain.SegmentConfig) error {
	return m.addFn(ctx, values...)
}

type mockClientService struct {
	PageProxyConfigFn                  func(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error)
	FetchFeatureConfigForEnvironmentFn func(ctx context.Context, authToken, envId string) ([]clientgen.FeatureConfig, error)
	FetchSegmentConfigForEnvironmentFn func(ctx context.Context, authToken, envId string) ([]clientgen.Segment, error)
}

func (c mockClientService) FetchSegmentConfigForEnvironment(ctx context.Context, authToken, envId string) ([]clientgen.Segment, error) {
	return c.FetchSegmentConfigForEnvironmentFn(ctx, authToken, envId)
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
