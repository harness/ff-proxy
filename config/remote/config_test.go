package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/services"
	"github.com/stretchr/testify/assert"
)

const (
	account               = "account"
	org                   = "org"
	project               = "project"
	environmentIdentifier = "env"
	defaultAPIKey         = "key1"
	defaultEnvironmentID  = "0000-0000-0000-0000-0000"
	// this jwt base64 encodes the defaultEnvironmentID, environmentIdentifier and project
	validJWT = "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature"
	validKey = "valid_key"
)

type mockClientService struct {
	authProxyKey    func() (services.AuthenticateProxyKeyResponse, error)
	pageProxyConfig func() ([]domain.ProxyConfig, error)
}

func (m mockClientService) AuthenticateProxyKey(ctx context.Context, key string) (services.AuthenticateProxyKeyResponse, error) {
	return m.authProxyKey()
}

func (m mockClientService) Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error) {
	return "not implemented", nil
}

func (m mockClientService) PageProxyConfig(ctx context.Context, input services.GetProxyConfigInput) ([]domain.ProxyConfig, error) {
	return m.pageProxyConfig()
}

func TestConfig_Populate(t *testing.T) {
	type args struct {
		key string
	}

	type mocks struct {
		clientService mockClientService
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		shouldErr bool
	}{
		"Given I call Populate and the clientService fails to authenticate": {
			args: args{key: "123"},
			mocks: mocks{
				clientService: mockClientService{
					authProxyKey: func() (services.AuthenticateProxyKeyResponse, error) {
						return services.AuthenticateProxyKeyResponse{}, services.ErrUnauthorized
					},
				},
			},
			shouldErr: true,
		},
		"Given I call Populate and the client service errors fetching ProxyConfig": {
			args: args{key: "123"},
			mocks: mocks{
				clientService: mockClientService{
					authProxyKey: func() (services.AuthenticateProxyKeyResponse, error) {
						return services.AuthenticateProxyKeyResponse{}, nil
					},
					pageProxyConfig: func() ([]domain.ProxyConfig, error) {
						return []domain.ProxyConfig{}, errors.New("client service error")
					},
				},
			},
			shouldErr: true,
		},
		"Given I call Populate and the client service doesn't error fetching ProxyConfig": {
			args: args{key: "123"},
			mocks: mocks{
				clientService: mockClientService{
					authProxyKey: func() (services.AuthenticateProxyKeyResponse, error) {
						return services.AuthenticateProxyKeyResponse{}, nil
					},
					pageProxyConfig: func() ([]domain.ProxyConfig, error) {
						return []domain.ProxyConfig{}, nil
					},
				},
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			c := NewConfig(tc.args.key, tc.mocks.clientService)

			err := c.Populate(context.Background())
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
