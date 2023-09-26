package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/harness/ff-proxy/v2/log"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/stretchr/testify/assert"
)

var errNotFound = errors.New("errNotFound")

type mockService struct {
	clientgen.ClientWithResponsesInterface
	authWithResp        func() error
	postMetricsWithResp func(environment string) (*clientgen.PostMetricsResponse, error)

	authProxyKey func() (*clientgen.AuthenticateProxyKeyResponse, error)
}

func (m mockService) AuthenticateWithResponse(ctx context.Context, req clientgen.AuthenticateJSONRequestBody, fns ...clientgen.RequestEditorFn) (*clientgen.AuthenticateResponse, error) {
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

func (m mockService) AuthenticateProxyKeyWithResponse(ctx context.Context, req clientgen.AuthenticateProxyKeyJSONRequestBody, fns ...clientgen.RequestEditorFn) (*clientgen.AuthenticateProxyKeyResponse, error) {
	return m.authProxyKey()
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
			clientService.client = tc.mockService

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
		response AuthenticateProxyKeyResponse
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
				response: AuthenticateProxyKeyResponse{
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

			c := ClientService{client: tc.mocks.clientService}
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
