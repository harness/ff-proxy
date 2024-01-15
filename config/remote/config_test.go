package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"runtime"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/cache"
	clientservice "github.com/harness/ff-proxy/v2/clients/client_service"
	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/repository"
)

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

type mockAuthRepo struct {
	config []domain.AuthConfig

	add                           func(ctx context.Context, config ...domain.AuthConfig) error
	addAPIConfigsForEnvironmentFn func(ctx context.Context, envID string, apiKeys []string) error
	getKeysForEnvironmentFn       func(ctx context.Context, envID string) ([]string, error)
}

func (m mockAuthRepo) GetKeysForEnvironment(ctx context.Context, envID string) ([]string, error) {
	return m.getKeysForEnvironmentFn(ctx, envID)
}
func (m mockAuthRepo) Remove(ctx context.Context, id []string) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthRepo) Get(ctx context.Context, key string) (map[string]string, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthRepo) Patch(ctx context.Context, key string, assets []string) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthRepo) BuildAssetListFromConfig(config []domain.ProxyConfig) (map[string]string, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthRepo) Cleanup(ctx context.Context, key string, config []domain.ProxyConfig) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthRepo) AddAPIConfigsForEnvironment(ctx context.Context, envID string, apiKeys []string) error {
	return m.addAPIConfigsForEnvironmentFn(ctx, envID, apiKeys)
}
func (m *mockAuthRepo) PatchAPIConfigForEnvironment(ctx context.Context, envID, apikey, action string) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthRepo) RemoveAllKeysForEnvironment(ctx context.Context, envID string) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthRepo) Add(ctx context.Context, config ...domain.AuthConfig) error {
	if err := m.add(ctx, config...); err != nil {
		return err
	}
	m.config = append(m.config, config...)
	return nil
}

type mockSegmentRepo struct {
	config []domain.SegmentConfig

	add                               func(ctx context.Context, config ...domain.SegmentConfig) error
	removeFn                          func(ctx context.Context, id string) error
	removeAllSegmentsForEnvironmentFn func(ctx context.Context, id string) error
	getSegmentsForEnvironmentFn       func(ctx context.Context, envID string) ([]domain.Segment, bool)
}

func (m *mockSegmentRepo) RemoveAllFeaturesForEnvironment(ctx context.Context, id string) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockSegmentRepo) GetFeatureConfigForEnvironment(ctx context.Context, envID string) ([]domain.FeatureFlag, bool) {
	//TODO implement me
	panic("implement me")
}

func (m *mockSegmentRepo) GetSegmentsForEnvironment(ctx context.Context, envID string) ([]domain.Segment, bool) {
	return m.getSegmentsForEnvironmentFn(ctx, envID)
}

func (m *mockSegmentRepo) RemoveAllSegmentsForEnvironment(ctx context.Context, id string) error {
	return m.removeAllSegmentsForEnvironmentFn(ctx, id)
}

func (m *mockSegmentRepo) Remove(ctx context.Context, id string) error {
	return m.removeFn(ctx, id)
}

func (m *mockSegmentRepo) Add(ctx context.Context, config ...domain.SegmentConfig) error {
	if err := m.add(ctx, config...); err != nil {
		return err
	}
	m.config = append(m.config, config...)
	return nil
}

type mockFlagRepo struct {
	config []domain.FlagConfig

	addFn                             func(ctx context.Context, config ...domain.FlagConfig) error
	removeFn                          func(ctx context.Context, id string) error
	removeAllFeaturesForEnvironmentFn func(ctx context.Context, id string) error
	getFeatureConfigForEnvironmentFn  func(ctx context.Context, envID string) ([]domain.FeatureFlag, bool)
}

func (m *mockFlagRepo) GetFeatureConfigForEnvironment(ctx context.Context, envID string) ([]domain.FeatureFlag, bool) {
	return m.getFeatureConfigForEnvironmentFn(ctx, envID)
}

func (m *mockFlagRepo) RemoveAllFeaturesForEnvironment(ctx context.Context, id string) error {
	return m.removeAllFeaturesForEnvironmentFn(ctx, id)
}

func (m *mockFlagRepo) Remove(ctx context.Context, id string) error {
	return m.removeFn(ctx, id)
}

func (m *mockFlagRepo) Add(ctx context.Context, config ...domain.FlagConfig) error {
	if err := m.addFn(ctx, config...); err != nil {
		return err
	}
	m.config = append(m.config, config...)
	return nil
}

type mockClientService struct {
	authProxyKey    func() (domain.AuthenticateProxyKeyResponse, error)
	pageProxyConfig func() ([]domain.ProxyConfig, error)
}

func (m mockClientService) FetchSegmentConfigForEnvironment(ctx context.Context, authToken, cluster, envID string) ([]clientgen.Segment, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockClientService) FetchFeatureConfigForEnvironment(ctx context.Context, authToken, cluster, envId string) ([]clientgen.FeatureConfig, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockClientService) AuthenticateProxyKey(ctx context.Context, key string) (domain.AuthenticateProxyKeyResponse, error) {
	return m.authProxyKey()
}

func (m mockClientService) Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error) {
	return "not implemented", nil
}

func (m mockClientService) PageProxyConfig(ctx context.Context, input domain.GetProxyConfigInput) ([]domain.ProxyConfig, error) {
	return m.pageProxyConfig()
}

func TestConfig_Populate(t *testing.T) {

	proxyConfig := []domain.ProxyConfig{
		{
			Environments: []domain.Environments{
				{

					ID:      uuid.MustParse("2fd10ce3-7ed6-466f-a768-e4df08f566b0"),
					APIKeys: []string{"123", "456"},
					FeatureConfigs: []domain.FeatureFlag{
						{
							Feature: "Foo",
						},
						{
							Feature: "bar",
						},
					},
					Segments: []domain.Segment{
						{
							Identifier: "One",
						},
						{
							Identifier: "Two",
						},
					},
				},
			},
		},
	}

	expectedAuthConfig := []domain.AuthConfig{
		{
			APIKey:        domain.NewAuthAPIKey("123"),
			EnvironmentID: domain.EnvironmentID("2fd10ce3-7ed6-466f-a768-e4df08f566b0"),
		},
		{
			APIKey:        domain.NewAuthAPIKey("456"),
			EnvironmentID: domain.EnvironmentID("2fd10ce3-7ed6-466f-a768-e4df08f566b0"),
		},
	}
	expectedFlagConfig := []domain.FlagConfig{
		{
			EnvironmentID: "2fd10ce3-7ed6-466f-a768-e4df08f566b0",
			FeatureConfigs: []domain.FeatureFlag{
				{
					Feature: "Foo",
				},
				{
					Feature: "bar",
				},
			},
		},
	}
	expectedSegmentConfig := []domain.SegmentConfig{
		{
			EnvironmentID: "2fd10ce3-7ed6-466f-a768-e4df08f566b0",
			Segments: []domain.Segment{
				{
					Identifier: "One",
				},
				{
					Identifier: "Two",
				},
			},
		},
	}

	type args struct {
		key string
	}

	type mocks struct {
		clientService mockClientService
		authRepo      *mockAuthRepo
		flagRepo      *mockFlagRepo
		segmentRepo   *mockSegmentRepo
	}

	type expected struct {
		authConfig    []domain.AuthConfig
		flagConfig    []domain.FlagConfig
		segmentConfig []domain.SegmentConfig
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		shouldErr bool

		expected expected
	}{
		"Given I call Populate and the clientService fails to authenticate": {
			args: args{key: "123"},
			mocks: mocks{
				clientService: mockClientService{
					authProxyKey: func() (domain.AuthenticateProxyKeyResponse, error) {
						return domain.AuthenticateProxyKeyResponse{}, clientservice.ErrUnauthorized
					},
				},
				authRepo:    &mockAuthRepo{},
				flagRepo:    &mockFlagRepo{},
				segmentRepo: &mockSegmentRepo{},
			},
			shouldErr: true,
		},
		"Given I call Populate and the client service errors fetching ProxyConfig": {
			args: args{key: "123"},
			mocks: mocks{
				clientService: mockClientService{
					authProxyKey: func() (domain.AuthenticateProxyKeyResponse, error) {
						return domain.AuthenticateProxyKeyResponse{}, nil
					},
					pageProxyConfig: func() ([]domain.ProxyConfig, error) {
						return []domain.ProxyConfig{}, errors.New("client service error")
					},
				},
				authRepo:    &mockAuthRepo{},
				flagRepo:    &mockFlagRepo{},
				segmentRepo: &mockSegmentRepo{},
			},
			shouldErr: true,
		},
		"Given I call Populate and the authRepo errors adding config to the cache": {
			args: args{key: "123"},
			mocks: mocks{
				clientService: mockClientService{
					authProxyKey: func() (domain.AuthenticateProxyKeyResponse, error) {
						return domain.AuthenticateProxyKeyResponse{}, nil
					},
					pageProxyConfig: func() ([]domain.ProxyConfig, error) {
						return proxyConfig, nil
					},
				},
				authRepo: &mockAuthRepo{
					add: func(ctx context.Context, config ...domain.AuthConfig) error {
						return errors.New("an error")
					},
				},
				flagRepo:    &mockFlagRepo{},
				segmentRepo: &mockSegmentRepo{},
			},
			shouldErr: true,
		},
		"Given I call Populate and the flagRepo errors adding config to the cache": {
			args: args{key: "123"},
			mocks: mocks{
				clientService: mockClientService{
					authProxyKey: func() (domain.AuthenticateProxyKeyResponse, error) {
						return domain.AuthenticateProxyKeyResponse{}, nil
					},
					pageProxyConfig: func() ([]domain.ProxyConfig, error) {
						return proxyConfig, nil
					},
				},
				authRepo: &mockAuthRepo{
					add: func(ctx context.Context, config ...domain.AuthConfig) error {
						return nil
					},
					addAPIConfigsForEnvironmentFn: func(ctx context.Context, envID string, apiKeys []string) error {
						return nil
					},
				},
				flagRepo: &mockFlagRepo{
					addFn: func(ctx context.Context, config ...domain.FlagConfig) error {
						return errors.New("an error")
					},
				},
				segmentRepo: &mockSegmentRepo{
					add: func(ctx context.Context, config ...domain.SegmentConfig) error {
						return nil
					},
				},
			},
			shouldErr: true,

			expected: expected{
				authConfig:    expectedAuthConfig,
				flagConfig:    nil,
				segmentConfig: nil,
			},
		},
		"Given I call Populate and the segmentRepo errors adding config to the cache": {
			args: args{key: "123"},
			mocks: mocks{
				clientService: mockClientService{
					authProxyKey: func() (domain.AuthenticateProxyKeyResponse, error) {
						return domain.AuthenticateProxyKeyResponse{}, nil
					},
					pageProxyConfig: func() ([]domain.ProxyConfig, error) {
						return proxyConfig, nil
					},
				},

				authRepo: &mockAuthRepo{
					add: func(ctx context.Context, config ...domain.AuthConfig) error {
						return nil
					},
					addAPIConfigsForEnvironmentFn: func(ctx context.Context, envID string, apiKeys []string) error {
						return nil
					},
				},
				flagRepo: &mockFlagRepo{
					addFn: func(ctx context.Context, config ...domain.FlagConfig) error {
						return nil
					},
				},
				segmentRepo: &mockSegmentRepo{
					add: func(ctx context.Context, config ...domain.SegmentConfig) error {
						return errors.New("an error")
					},
				},
			},
			shouldErr: true,

			expected: expected{
				authConfig:    expectedAuthConfig,
				flagConfig:    expectedFlagConfig,
				segmentConfig: nil,
			},
		},
		"Given I call Populate and all repos successfully add config to the cache": {
			args: args{key: "123"},
			mocks: mocks{
				clientService: mockClientService{
					authProxyKey: func() (domain.AuthenticateProxyKeyResponse, error) {
						return domain.AuthenticateProxyKeyResponse{}, nil
					},
					pageProxyConfig: func() ([]domain.ProxyConfig, error) {
						return proxyConfig, nil
					},
				},
				authRepo: &mockAuthRepo{
					add: func(ctx context.Context, config ...domain.AuthConfig) error {
						return nil
					},
					addAPIConfigsForEnvironmentFn: func(ctx context.Context, envID string, apiKeys []string) error {
						return nil
					},
				},
				flagRepo: &mockFlagRepo{
					addFn: func(ctx context.Context, config ...domain.FlagConfig) error {
						return nil
					},
				},
				segmentRepo: &mockSegmentRepo{
					add: func(ctx context.Context, config ...domain.SegmentConfig) error {
						return nil
					},
				},
			},
			shouldErr: false,

			expected: expected{
				authConfig:    expectedAuthConfig,
				flagConfig:    expectedFlagConfig,
				segmentConfig: expectedSegmentConfig,
			},
		},
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
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			c := NewConfig(tc.args.key, tc.mocks.clientService)

			err := c.FetchAndPopulate(context.Background(), inventoryRepo, tc.mocks.authRepo, tc.mocks.flagRepo, tc.mocks.segmentRepo)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.authConfig, tc.mocks.authRepo.config)
			assert.Equal(t, tc.expected.flagConfig, tc.mocks.flagRepo.config)
			assert.Equal(t, tc.expected.segmentConfig, tc.mocks.segmentRepo.config)
		})
	}
}

func getfile(path string) []byte {
	b, err := ioutil.ReadFile(path) // just pass the file name
	if err != nil {
		fmt.Print(err)
	}
	return b
}

func Benchmark_ConfigPopulate100Env30Flags(b *testing.B) {
	proxyConfig := domain.ProxyConfig{}
	rawProxyConfig := getfile("./test-data/testFile_100_envs_30_flags.json")
	if err := json.Unmarshal(rawProxyConfig, &proxyConfig); err != nil {
		b.Fatalf("failed to unmarshal proxy config: %s", err)
	}
	rc := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	r := cache.NewKeyValCache(rc)
	_ = r
	authRepo := repository.NewAuthRepo(r)
	flagRepo := repository.NewFeatureFlagRepo(r)
	segmentRepo := repository.NewSegmentRepo(r)
	c := Config{
		proxyConfig: []domain.ProxyConfig{proxyConfig},
	}

	// Limit to 1 CPU core
	runtime.GOMAXPROCS(1)

	for i := 0; i < b.N; i++ {
		if err := c.Populate(context.Background(), authRepo, flagRepo, segmentRepo); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_ConfigPopulate200Env30Flags(b *testing.B) {
	proxyConfig := domain.ProxyConfig{}
	rawProxyConfig := getfile("./test-data/testFile_200_envs_30_flags.json")
	if err := json.Unmarshal(rawProxyConfig, &proxyConfig); err != nil {
		b.Fatalf("failed to unmarshal proxy config: %s", err)
	}
	rc := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	r := cache.NewKeyValCache(rc)
	_ = r
	authRepo := repository.NewAuthRepo(r)
	flagRepo := repository.NewFeatureFlagRepo(r)
	segmentRepo := repository.NewSegmentRepo(r)
	c := Config{
		proxyConfig: []domain.ProxyConfig{proxyConfig},
	}

	// Limit to 1 CPU core
	runtime.GOMAXPROCS(1)

	for i := 0; i < b.N; i++ {
		if err := c.Populate(context.Background(), authRepo, flagRepo, segmentRepo); err != nil {
			b.Fatal(err)
		}
	}
}
func Benchmark_ConfigPopulate300Env30Flags(b *testing.B) {
	proxyConfigs := make([]domain.ProxyConfig, 0, 300)
	rawProxyConfig := getfile("./test-data/testFile_300_envs_30_flags.json")
	if err := json.Unmarshal(rawProxyConfig, &proxyConfigs); err != nil {
		b.Fatalf("failed to unmarshal proxy config: %s", err)
	}

	rc := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	r := cache.NewKeyValCache(rc)
	_ = r
	authRepo := repository.NewAuthRepo(r)
	flagRepo := repository.NewFeatureFlagRepo(r)
	segmentRepo := repository.NewSegmentRepo(r)
	c := Config{
		proxyConfig: proxyConfigs,
	}

	// Limit to 1 CPU core
	runtime.GOMAXPROCS(1)

	for i := 0; i < b.N; i++ {
		if err := c.Populate(context.Background(), authRepo, flagRepo, segmentRepo); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_ConfigPopulate500Env30Flags(b *testing.B) {
	proxyConfigs := make([]domain.ProxyConfig, 0, 500)
	rawProxyConfig := getfile("./test-data/testFile_500_envs_30_flags.json")
	if err := json.Unmarshal(rawProxyConfig, &proxyConfigs); err != nil {
		b.Fatalf("failed to unmarshal proxy config: %s", err)
	}

	rc := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	r := cache.NewKeyValCache(rc)
	_ = r
	authRepo := repository.NewAuthRepo(r)
	flagRepo := repository.NewFeatureFlagRepo(r)
	segmentRepo := repository.NewSegmentRepo(r)
	c := Config{
		proxyConfig: proxyConfigs,
	}

	// Limit to 1 CPU core
	runtime.GOMAXPROCS(1)

	for i := 0; i < b.N; i++ {
		if err := c.Populate(context.Background(), authRepo, flagRepo, segmentRepo); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_ConfigPopulate1000Env30Flags(b *testing.B) {
	proxyConfigs := make([]domain.ProxyConfig, 0, 1000)
	rawProxyConfig := getfile("./test-data/testFile_1000_envs_30_flags.json")
	if err := json.Unmarshal(rawProxyConfig, &proxyConfigs); err != nil {
		b.Fatalf("failed to unmarshal proxy config: %s", err)
	}

	rc := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	r := cache.NewKeyValCache(rc)
	_ = r
	authRepo := repository.NewAuthRepo(r)
	flagRepo := repository.NewFeatureFlagRepo(r)
	segmentRepo := repository.NewSegmentRepo(r)
	c := Config{
		proxyConfig: proxyConfigs,
	}

	// Limit to 1 CPU core
	runtime.GOMAXPROCS(1)

	for i := 0; i < b.N; i++ {
		if err := c.Populate(context.Background(), authRepo, flagRepo, segmentRepo); err != nil {
			b.Fatal(err)
		}
	}
}
