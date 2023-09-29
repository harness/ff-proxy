package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/harness/ff-proxy/v2/log"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/stretchr/testify/assert"
)

var errNotFound = errors.New("errNotFound")

type mockService struct {
	clientgen.ClientWithResponsesInterface
	*sync.Mutex

	authWithResp        func() error
	postMetricsWithResp func(environment string) (*clientgen.PostMetricsResponse, error)

	authProxyKey   func() (*clientgen.AuthenticateProxyKeyResponse, error)
	getProxyConfig func(req *clientgen.GetProxyConfigParams) (*clientgen.GetProxyConfigResponse, error)

	getProxyConfigCalls int
}

func (m *mockService) AuthenticateWithResponse(ctx context.Context, req clientgen.AuthenticateJSONRequestBody, fns ...clientgen.RequestEditorFn) (*clientgen.AuthenticateResponse, error) {
	if err := m.authWithResp(); err != nil {
		if err == errNotFound {
			resp := clientgen.AuthenticateResponse{
				JSON404: &clientgen.Error{
					Code:    "404",
					Message: "Not Found",
				},
			}
			return &resp, nil
		}

		return &clientgen.AuthenticateResponse{}, err
	}

	return &clientgen.AuthenticateResponse{JSON200: &clientgen.AuthenticationResponse{AuthToken: "hardcoded-token"}}, nil
}

func (m *mockService) AuthenticateProxyKeyWithResponse(ctx context.Context, req clientgen.AuthenticateProxyKeyJSONRequestBody, fns ...clientgen.RequestEditorFn) (*clientgen.AuthenticateProxyKeyResponse, error) {
	return m.authProxyKey()
}

func (m *mockService) GetProxyConfigWithResponse(ctx context.Context, req *clientgen.GetProxyConfigParams, fns ...clientgen.RequestEditorFn) (*clientgen.GetProxyConfigResponse, error) {
	m.Lock()
	m.getProxyConfigCalls++
	m.Unlock()

	return m.getProxyConfig(req)
}

func (m *mockService) GetProxyConfigCalls() int {
	m.Lock()
	defer m.Unlock()
	return m.getProxyConfigCalls
}

func TestClientService_Authenticate(t *testing.T) {
	testCases := map[string]struct {
		mockService mockService
		shouldErr   bool
		expected    string
	}{
		"Given I have a working ClientService": {
			mockService: mockService{
				authWithResp: func() error {
					return nil
				},
			},
			shouldErr: false,
			expected:  "hardcoded-token",
		},
		"Given I have a ClientService that errors": {
			mockService: mockService{
				authWithResp: func() error {
					return errors.New("uh oh")
				},
			},
			shouldErr: true,
			expected:  "",
		},
		"Given I have a ClientService that returns a NotFound error": {
			mockService: mockService{
				authWithResp: func() error {
					return errNotFound
				},
			},
			shouldErr: true,
			expected:  "",
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			logger, _ := log.NewStructuredLogger("DEBUG")
			clientService, _ := NewClientService(logger, "localhost:8000")
			clientService.client = &tc.mockService

			actual, err := clientService.Authenticate(context.Background(), "", domain.Target{})
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, tc.expected, actual)

		})
	}
}

func TestClientService_AuthenticateProxyKey(t *testing.T) {
	type mocks struct {
		clientService mockService
	}

	type expected struct {
		err      error
		response domain.AuthenticateProxyKeyResponse
	}

	testCases := map[string]struct {
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given the ff-client-server returns a 500": {
			mocks: mocks{mockService{authProxyKey: func() (*clientgen.AuthenticateProxyKeyResponse, error) {
				return &clientgen.AuthenticateProxyKeyResponse{HTTPResponse: &http.Response{StatusCode: http.StatusInternalServerError}}, nil
			}}},
			expected:  expected{err: ErrInternal},
			shouldErr: true,
		},
		"Given the ff-client-server returns a 404": {
			mocks: mocks{mockService{authProxyKey: func() (*clientgen.AuthenticateProxyKeyResponse, error) {
				return &clientgen.AuthenticateProxyKeyResponse{HTTPResponse: &http.Response{StatusCode: http.StatusNotFound}}, nil
			}}},
			expected:  expected{err: ErrNotFound},
			shouldErr: true,
		},
		"Given the ff-client-server returns a 401": {
			mocks: mocks{mockService{authProxyKey: func() (*clientgen.AuthenticateProxyKeyResponse, error) {
				return &clientgen.AuthenticateProxyKeyResponse{HTTPResponse: &http.Response{StatusCode: http.StatusUnauthorized}}, nil
			}}},
			expected:  expected{err: ErrUnauthorized},
			shouldErr: true,
		},
		"Given the ff-client-server returns a 403": {
			mocks: mocks{mockService{authProxyKey: func() (*clientgen.AuthenticateProxyKeyResponse, error) {
				return &clientgen.AuthenticateProxyKeyResponse{HTTPResponse: &http.Response{StatusCode: http.StatusForbidden}}, nil
			}}},
			expected:  expected{err: ErrUnauthorized},
			shouldErr: true,
		},
		"Given the ff-client-server returns 200 and a valid token": {
			mocks: mocks{mockService{authProxyKey: func() (*clientgen.AuthenticateProxyKeyResponse, error) {
				return &clientgen.AuthenticateProxyKeyResponse{
					HTTPResponse: &http.Response{
						StatusCode: http.StatusOK,
					},
					JSON200: &clientgen.AuthenticationResponse{
						AuthToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjbHVzdGVyX2lkZW50aWZpZXIiOiIxIiwiYWNjb3VudCI6ImNzZlIzekZsUnhDQktTRG9Pc3ZYcEEiLCJvcmdhbml6YXRpb24iOiJjNDUzYTdmYi1jOGExLTQyMmMtYWMwNy04ZTQwMzg1YTk3ZjQiLCJrZXlfdHlwZSI6IlByb3h5Iiwia2V5IjoiMTRjMjllNTk4NDVkZGMzYmFhOWE1ODlkODU5NjQzZDIyZTEyYzcyNmY5MjU3NWI2NzkzNDA3YTkxNjNiMTQ0MiIsImVudmlyb25tZW50cyI6WyI3NjMwNjZjNC1kNzc5LTRjMzctOTgwNC1lNWNmMzA0NGE0MWQiLCJkNGEzNjMzZi04OWRmLTQ0YzUtOWYzZS0zMjFiY2EyMDIwM2EiXX0.gNfs74ortBsyOXFdXSW4IqvWuwkhcXIZByH6lCLzEVY",
					},
				}, nil
			}}},
			expected: expected{
				err: ErrUnauthorized,
				response: domain.AuthenticateProxyKeyResponse{
					Token:             "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjbHVzdGVyX2lkZW50aWZpZXIiOiIxIiwiYWNjb3VudCI6ImNzZlIzekZsUnhDQktTRG9Pc3ZYcEEiLCJvcmdhbml6YXRpb24iOiJjNDUzYTdmYi1jOGExLTQyMmMtYWMwNy04ZTQwMzg1YTk3ZjQiLCJrZXlfdHlwZSI6IlByb3h5Iiwia2V5IjoiMTRjMjllNTk4NDVkZGMzYmFhOWE1ODlkODU5NjQzZDIyZTEyYzcyNmY5MjU3NWI2NzkzNDA3YTkxNjNiMTQ0MiIsImVudmlyb25tZW50cyI6WyI3NjMwNjZjNC1kNzc5LTRjMzctOTgwNC1lNWNmMzA0NGE0MWQiLCJkNGEzNjMzZi04OWRmLTQ0YzUtOWYzZS0zMjFiY2EyMDIwM2EiXX0.gNfs74ortBsyOXFdXSW4IqvWuwkhcXIZByH6lCLzEVY",
					ClusterIdentifier: "1",
				},
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c := ClientService{client: &tc.mocks.clientService}
			actual, err := c.AuthenticateProxyKey(context.Background(), "123")
			if tc.shouldErr {
				assert.NotNil(t, err)
				assert.True(t, errors.Is(err, tc.expected.err))
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.response, actual)
		})
	}
}

func TestClientService_PageProxyConfig(t *testing.T) {
	type args struct {
		input domain.GetProxyConfigInput
	}

	type mocks struct {
		clientService *mockService
	}

	type expected struct {
		err                 error
		config              []domain.ProxyConfig
		getProxyConfigCalls int
	}

	environmentsPageOne := []struct {
		ApiKeys        *[]string                  `json:"apiKeys,omitempty"`
		FeatureConfigs *[]clientgen.FeatureConfig `json:"featureConfigs,omitempty"`
		Id             *string                    `json:"id,omitempty"`
		Segments       *[]clientgen.Segment       `json:"segments,omitempty"`
	}{
		{
			Id:      toPtr("515975dd-a4a6-41fe-aefb-ffc088e2b4ec"),
			ApiKeys: toPtr([]string{"123", "456"}),
			FeatureConfigs: toPtr([]clientgen.FeatureConfig{
				{
					Feature: "DarkMode",
				},
				{
					Feature: "PerfEnhancement",
				},
			}),
			Segments: toPtr([]clientgen.Segment{
				{
					Identifier: "GroupOne",
				},
				{
					Identifier: "GroupTwo",
				},
			}),
		},
	}

	environmentsPageTwo := []struct {
		ApiKeys        *[]string                  `json:"apiKeys,omitempty"`
		FeatureConfigs *[]clientgen.FeatureConfig `json:"featureConfigs,omitempty"`
		Id             *string                    `json:"id,omitempty"`
		Segments       *[]clientgen.Segment       `json:"segments,omitempty"`
	}{
		{
			Id:      toPtr("4ce93f1b-ebb6-4477-b91a-1f985079c8d9"),
			ApiKeys: toPtr([]string{"789"}),
			FeatureConfigs: toPtr([]clientgen.FeatureConfig{
				{
					Feature: "DarkMode",
				},
				{
					Feature: "PerfEnhancement",
				},
			}),
			Segments: toPtr([]clientgen.Segment{
				{
					Identifier: "GroupOne",
				},
				{
					Identifier: "GroupTwo",
				},
			}),
		},
	}

	pageOne := clientgen.ProxyConfig{
		Environments: &environmentsPageOne,
		ItemCount:    2,
		PageCount:    2,
		PageIndex:    0,
		PageSize:     1,
		Version:      nil,
	}

	pageTwo := clientgen.ProxyConfig{
		Environments: &environmentsPageTwo,
		ItemCount:    2,
		PageCount:    2,
		PageIndex:    1,
		PageSize:     1,
		Version:      nil,
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given the ff-client-service returns an error": {
			mocks: mocks{
				clientService: &mockService{
					Mutex: &sync.Mutex{},
					getProxyConfig: func(req *clientgen.GetProxyConfigParams) (*clientgen.GetProxyConfigResponse, error) {
						return nil, errors.New("client service error")
					},
				},
			},
			expected: expected{
				err:                 ErrInternal,
				config:              nil,
				getProxyConfigCalls: 1,
			},
			shouldErr: true,
		},
		"Given the ff-client-service returns a 400": {
			mocks: mocks{
				clientService: &mockService{
					Mutex: &sync.Mutex{},
					getProxyConfig: func(req *clientgen.GetProxyConfigParams) (*clientgen.GetProxyConfigResponse, error) {
						return &clientgen.GetProxyConfigResponse{HTTPResponse: &http.Response{StatusCode: http.StatusBadRequest}}, nil
					},
				},
			},
			expected: expected{
				err:                 ErrBadRequest,
				config:              nil,
				getProxyConfigCalls: 1,
			},
			shouldErr: true,
		},
		"Given the ff-client-service returns a 401": {
			mocks: mocks{
				clientService: &mockService{
					Mutex: &sync.Mutex{},
					getProxyConfig: func(req *clientgen.GetProxyConfigParams) (*clientgen.GetProxyConfigResponse, error) {
						return &clientgen.GetProxyConfigResponse{HTTPResponse: &http.Response{StatusCode: http.StatusUnauthorized}}, nil
					},
				},
			},
			expected: expected{
				err:                 ErrUnauthorized,
				config:              nil,
				getProxyConfigCalls: 1,
			},
			shouldErr: true,
		},
		"Given the ff-client-service returns a 403": {
			mocks: mocks{
				clientService: &mockService{
					Mutex: &sync.Mutex{},
					getProxyConfig: func(req *clientgen.GetProxyConfigParams) (*clientgen.GetProxyConfigResponse, error) {
						return &clientgen.GetProxyConfigResponse{HTTPResponse: &http.Response{StatusCode: http.StatusForbidden}}, nil
					},
				},
			},
			expected: expected{
				err:                 ErrUnauthorized,
				config:              nil,
				getProxyConfigCalls: 1,
			},
			shouldErr: true,
		},
		"Given the ff-client-service returns a 404": {
			mocks: mocks{
				clientService: &mockService{
					Mutex: &sync.Mutex{},
					getProxyConfig: func(req *clientgen.GetProxyConfigParams) (*clientgen.GetProxyConfigResponse, error) {
						return &clientgen.GetProxyConfigResponse{HTTPResponse: &http.Response{StatusCode: http.StatusNotFound}}, nil
					},
				},
			},
			expected: expected{
				err:                 ErrNotFound,
				config:              nil,
				getProxyConfigCalls: 1,
			},
			shouldErr: true,
		},
		"Given the ff-client-service returns a 500": {
			mocks: mocks{
				clientService: &mockService{
					Mutex: &sync.Mutex{},
					getProxyConfig: func(req *clientgen.GetProxyConfigParams) (*clientgen.GetProxyConfigResponse, error) {
						return &clientgen.GetProxyConfigResponse{HTTPResponse: &http.Response{StatusCode: http.StatusInternalServerError}}, nil
					},
				},
			},
			expected: expected{
				err:                 ErrInternal,
				config:              nil,
				getProxyConfigCalls: 1,
			},
			shouldErr: true,
		},
		"Given the ff-client-service returns a 200 with two pages": {
			mocks: mocks{
				clientService: &mockService{
					Mutex: &sync.Mutex{},
					getProxyConfig: func(req *clientgen.GetProxyConfigParams) (*clientgen.GetProxyConfigResponse, error) {
						var json200 *clientgen.ProxyConfig

						if *req.PageNumber == 0 {
							json200 = &pageOne
						}

						if *req.PageNumber == 1 {
							json200 = &pageTwo
						}

						return &clientgen.GetProxyConfigResponse{
							HTTPResponse: &http.Response{
								StatusCode: http.StatusOK,
							},
							JSON200: json200,
						}, nil
					},
				},
			},
			expected: expected{
				err: nil,
				config: []domain.ProxyConfig{
					domain.ToProxyConfig(pageOne),
					domain.ToProxyConfig(pageTwo),
				},
				getProxyConfigCalls: 2,
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			c := ClientService{client: tc.mocks.clientService}

			actual, err := c.PageProxyConfig(context.Background(), tc.args.input)
			if tc.shouldErr {
				assert.NotNil(t, err)
				assert.True(t, errors.Is(err, tc.expected.err))
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.config, actual)
			assert.Equal(t, tc.expected.getProxyConfigCalls, tc.mocks.clientService.GetProxyConfigCalls())
		})
	}
}

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func toPtr[T any](t T) *T {
	return &t
}
