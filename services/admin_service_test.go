package services

import (
	"context"
	"net/http"
	"testing"

	admingen "github.com/harness/ff-proxy/gen/admin"
	"github.com/harness/ff-proxy/log"

	"github.com/stretchr/testify/assert"
)

type mockAdminService struct {
	admingen.ClientWithResponsesInterface
	getTargetsWithResp func() (*admingen.GetAllTargetsResponse, error)
	getApiKeysWithResp func() (*admingen.GetAllAPIKeysResponse, error)
}

func (m mockAdminService) GetAllTargetsWithResponse(ctx context.Context, params *admingen.GetAllTargetsParams, reqEditors ...admingen.RequestEditorFn) (*admingen.GetAllTargetsResponse, error) {
	return m.getTargetsWithResp()
}

func (m mockAdminService) GetAllAPIKeysWithResponse(ctx context.Context, params *admingen.GetAllAPIKeysParams, reqEditors ...admingen.RequestEditorFn) (*admingen.GetAllAPIKeysResponse, error) {
	return m.getApiKeysWithResp()
}

func TestAdminService_PageTargets(t *testing.T) {
	pageTargetsInput := PageTargetsInput{
		AccountIdentifier:     "account",
		OrgIdentifier:         "org",
		ProjectIdentifier:     "proj",
		PageNumber:            0,
		PageSize:              2,
		EnvironmentIdentifier: "env",
	}
	target := admingen.Target{
		Account:     "account",
		Environment: "env",
		Identifier:  "target1",
		Name:        "targetName",
		Org:         "org",
		Project:     "proj",
	}

	testCases := map[string]struct {
		mockService mockAdminService
		input       PageTargetsInput
		shouldErr   bool
		expected    PageTargetsResult
	}{
		"Given PageTargets returns error": {
			mockService: mockAdminService{
				getTargetsWithResp: func() (*admingen.GetAllTargetsResponse, error) {
					return nil, errNotFound
				},
			},
			shouldErr: true,
			input:     pageTargetsInput,
			expected: PageTargetsResult{
				Finished: true,
			},
		},
		"Given PageTargets returns non 200 response": {
			mockService: mockAdminService{
				getTargetsWithResp: func() (*admingen.GetAllTargetsResponse, error) {
					return &admingen.GetAllTargetsResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusUnauthorized},
						JSON401: &admingen.Error{
							Code:    "401",
							Message: "Unauthorized",
						},
					}, nil
				},
			},
			shouldErr: true,
			input:     pageTargetsInput,
			expected: PageTargetsResult{
				Finished: true,
			},
		},
		"Given PageTargets returns 200 with no Targets": {
			mockService: mockAdminService{
				getTargetsWithResp: func() (*admingen.GetAllTargetsResponse, error) {
					return &admingen.GetAllTargetsResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusOK},
						JSON200: &admingen.Targets{
							Targets: &[]admingen.Target{},
						},
					}, nil
				},
			},
			shouldErr: false,
			input:     pageTargetsInput,
			expected: PageTargetsResult{
				Finished: true,
			},
		},
		"Given PageTargets returns 200 with Targets": {
			mockService: mockAdminService{
				getTargetsWithResp: func() (*admingen.GetAllTargetsResponse, error) {
					return &admingen.GetAllTargetsResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusUnauthorized},
						JSON200: &admingen.Targets{
							Targets: &[]admingen.Target{target},
						},
					}, nil
				},
			},
			shouldErr: false,
			input:     pageTargetsInput,
			expected: PageTargetsResult{
				Finished: false,
				Targets:  []admingen.Target{target},
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			logger, _ := log.NewStructuredLogger(true)
			adminService, _ := NewAdminService(logger, "localhost:8000", "svc-token")
			adminService.client = tc.mockService

			actual, err := adminService.PageTargets(context.Background(), tc.input)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, tc.expected, actual)

		})
	}
}

func TestAdminService_GetAPIKeys(t *testing.T) {
	pageAPIKeysInput := GetAPIKeysInput{
		AccountIdentifier:     "account",
		OrgIdentifier:         "org",
		ProjectIdentifier:     "proj",
		EnvironmentIdentifier: "env",
	}
	hashedKey := "1234"
	apiKey := admingen.ApiKey{
		ApiKey:     "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		Identifier: "mykey",
		Key:        &hashedKey,
		Name:       "mykey",
		Type:       "Server",
	}

	testCases := map[string]struct {
		mockService mockAdminService
		input       GetAPIKeysInput
		shouldErr   bool
		expected    PageAPIKeysResult
	}{
		"Given PageAPIKeys returns error": {
			mockService: mockAdminService{
				getApiKeysWithResp: func() (*admingen.GetAllAPIKeysResponse, error) {
					return nil, errNotFound
				},
			},
			shouldErr: true,
			input:     pageAPIKeysInput,
			expected: PageAPIKeysResult{
				Finished: true,
			},
		},
		"Given PageAPIKeys returns non 200 response": {
			mockService: mockAdminService{
				getApiKeysWithResp: func() (*admingen.GetAllAPIKeysResponse, error) {
					return &admingen.GetAllAPIKeysResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusUnauthorized},
						JSON401: &admingen.Error{
							Code:    "401",
							Message: "Unauthorized",
						},
					}, nil
				},
			},
			shouldErr: true,
			input:     pageAPIKeysInput,
			expected: PageAPIKeysResult{
				Finished: true,
			},
		},
		"Given PageAPIKeys returns 200 with no Keys": {
			mockService: mockAdminService{
				getApiKeysWithResp: func() (*admingen.GetAllAPIKeysResponse, error) {
					return &admingen.GetAllAPIKeysResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusOK},
						JSON200: &admingen.ApiKeys{
							ApiKeys: &[]admingen.ApiKey{},
						},
					}, nil
				},
			},
			shouldErr: false,
			input:     pageAPIKeysInput,
			expected: PageAPIKeysResult{
				Finished: true,
			},
		},
		"Given PageAPIKeys returns 200 with API Keys": {
			mockService: mockAdminService{
				getApiKeysWithResp: func() (*admingen.GetAllAPIKeysResponse, error) {
					return &admingen.GetAllAPIKeysResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusUnauthorized},
						JSON200: &admingen.ApiKeys{
							ApiKeys: &[]admingen.ApiKey{apiKey},
						},
					}, nil
				},
			},
			shouldErr: false,
			input:     pageAPIKeysInput,
			expected: PageAPIKeysResult{
				Finished: false,
				APIKeys:  []admingen.ApiKey{apiKey},
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			logger, _ := log.NewStructuredLogger(true)
			adminService, _ := NewAdminService(logger, "localhost:8000", "svc-token")
			adminService.client = tc.mockService

			actual, err := adminService.PageAPIKeys(context.Background(), tc.input)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, tc.expected, actual)

		})
	}
}
