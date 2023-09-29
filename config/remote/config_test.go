package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/services"
	"github.com/stretchr/testify/assert"
)

type mockAuthRepo struct {
	config []domain.AuthConfig

	add func(ctx context.Context, config ...domain.AuthConfig) error
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

	add func(ctx context.Context, config ...domain.SegmentConfig) error
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

	add func(ctx context.Context, config ...domain.FlagConfig) error
}

func (m *mockFlagRepo) Add(ctx context.Context, config ...domain.FlagConfig) error {
	if err := m.add(ctx, config...); err != nil {
		return err
	}
	m.config = append(m.config, config...)
	return nil
}

type mockClientService struct {
	authProxyKey    func() (domain.AuthenticateProxyKeyResponse, error)
	pageProxyConfig func() ([]domain.ProxyConfig, error)
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
					ApiKeys: []string{"123", "456"},
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
			APIKey:        domain.AuthAPIKey("123"),
			EnvironmentID: domain.EnvironmentID("2fd10ce3-7ed6-466f-a768-e4df08f566b0"),
		},
		{
			APIKey:        domain.AuthAPIKey("456"),
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
						return domain.AuthenticateProxyKeyResponse{}, services.ErrUnauthorized
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
				},
				flagRepo: &mockFlagRepo{
					add: func(ctx context.Context, config ...domain.FlagConfig) error {
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
				},
				flagRepo: &mockFlagRepo{
					add: func(ctx context.Context, config ...domain.FlagConfig) error {
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
				},
				flagRepo: &mockFlagRepo{
					add: func(ctx context.Context, config ...domain.FlagConfig) error {
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

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			c := NewConfig(tc.args.key, tc.mocks.clientService)

			err := c.Populate(context.Background(), tc.mocks.authRepo, tc.mocks.flagRepo, tc.mocks.segmentRepo)
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
