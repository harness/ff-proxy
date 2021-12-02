package services

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
	"github.com/stretchr/testify/assert"
)

var errNotFound = errors.New("errNotFound")

type mockService struct {
	clientgen.ClientWithResponsesInterface
	authWithResp func() error
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

			clientService := ClientService{client: tc.mockService}

			actual, err := clientService.Authenticate(context.Background(), "", domain.Target{})
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, tc.expected, actual)

		})
	}

}
