package services

import (
	"context"
	admingen "github.com/harness/ff-proxy/gen/admin"
	"github.com/harness/ff-proxy/log"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockAdminService struct {
	admingen.ClientWithResponsesInterface
	getEnvsWithResp     func() (*admingen.GetAllEnvironmentsResponse, error)
	getProjectsWithResp func() (*admingen.GetAllProjectsResponse, error)
	getTargetsWithResp  func() (*admingen.GetAllTargetsResponse, error)
}

func (m mockAdminService) GetAllEnvironmentsWithResponse(ctx context.Context, params *admingen.GetAllEnvironmentsParams, reqEditors ...admingen.RequestEditorFn) (*admingen.GetAllEnvironmentsResponse, error) {
	return m.getEnvsWithResp()
}

func (m mockAdminService) GetAllProjectsWithResponse(ctx context.Context, params *admingen.GetAllProjectsParams, reqEditors ...admingen.RequestEditorFn) (*admingen.GetAllProjectsResponse, error) {
	return m.getProjectsWithResp()
}

func (m mockAdminService) GetAllTargetsWithResponse(ctx context.Context, params *admingen.GetAllTargetsParams, reqEditors ...admingen.RequestEditorFn) (*admingen.GetAllTargetsResponse, error) {
	return m.getTargetsWithResp()
}

func TestAdminService_PageEnvironments(t *testing.T) {
	pageEnvironmentsInput := PageEnvironmentsInput{
		AccountIdentifier: "account",
		OrgIdentifier:     "org",
		ProjectIdentifier: "proj",
		PageNumber:        0,
		PageSize:          2,
	}
	envID := "envID"
	environment := admingen.Environment{
		Id:         &envID,
		Identifier: "envIdentifier",
		Name:       "env",
		Project:    "proj",
	}

	testCases := map[string]struct {
		mockService mockAdminService
		input       PageEnvironmentsInput
		shouldErr   bool
		expected    PageEnvironmentsResult
	}{
		"Given PageEnvironments returns error": {
			mockService: mockAdminService{
				getEnvsWithResp: func() (*admingen.GetAllEnvironmentsResponse, error) {
					return nil, errNotFound
				},
			},
			shouldErr: true,
			input:     pageEnvironmentsInput,
			expected: PageEnvironmentsResult{
				Finished: true,
			},
		},
		"Given PageEnvironments returns non 200 response": {
			mockService: mockAdminService{
				getEnvsWithResp: func() (*admingen.GetAllEnvironmentsResponse, error) {
					return &admingen.GetAllEnvironmentsResponse{
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
			input:     pageEnvironmentsInput,
			expected: PageEnvironmentsResult{
				Finished: true,
			},
		},
		"Given PageEnvironments returns 200 with no Environments": {
			mockService: mockAdminService{
				getEnvsWithResp: func() (*admingen.GetAllEnvironmentsResponse, error) {
					return &admingen.GetAllEnvironmentsResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusOK},
						JSON200: &struct {
							CorrelationId string                  `json:"correlationId"`
							Data          admingen.Environments   `json:"data"`
							MetaData      *map[string]interface{} `json:"metaData,omitempty"`
							Status        admingen.Status         `json:"status"`
						}{
							Data: admingen.Environments{Environments: &[]admingen.Environment{}},
						},
					}, nil
				},
			},
			shouldErr: false,
			input:     pageEnvironmentsInput,
			expected: PageEnvironmentsResult{
				Finished: true,
			},
		},
		"Given PageEnvironments returns 200 with Environments": {
			mockService: mockAdminService{
				getEnvsWithResp: func() (*admingen.GetAllEnvironmentsResponse, error) {
					return &admingen.GetAllEnvironmentsResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusOK},
						JSON200: &struct {
							CorrelationId string                  `json:"correlationId"`
							Data          admingen.Environments   `json:"data"`
							MetaData      *map[string]interface{} `json:"metaData,omitempty"`
							Status        admingen.Status         `json:"status"`
						}{
							Data: admingen.Environments{Environments: &[]admingen.Environment{environment}},
						},
					}, nil
				},
			},
			shouldErr: false,
			input:     pageEnvironmentsInput,
			expected: PageEnvironmentsResult{
				Finished:     false,
				Environments: []admingen.Environment{environment},
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			logger, _ := log.NewStructuredLogger(true)
			adminService, _ := NewAdminService(logger, "localhost:8000", "svc-token")
			adminService.client = tc.mockService

			actual, err := adminService.PageEnvironments(context.Background(), tc.input)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, tc.expected, actual)

		})
	}
}

func TestAdminService_PageProjects(t *testing.T) {
	pageProjectsInput := PageProjectsInput{
		AccountIdentifier: "account",
		OrgIdentifier:     "org",
		PageNumber:        0,
		PageSize:          2,
	}
	project := admingen.Project{
		Description: nil,
		Identifier:  "project",
		Name:        "project",
	}

	testCases := map[string]struct {
		mockService mockAdminService
		input       PageProjectsInput
		shouldErr   bool
		expected    PageProjectsResult
	}{
		"Given PageProjects returns error": {
			mockService: mockAdminService{
				getProjectsWithResp: func() (*admingen.GetAllProjectsResponse, error) {
					return nil, errNotFound
				},
			},
			shouldErr: true,
			input:     pageProjectsInput,
			expected: PageProjectsResult{
				Finished: true,
			},
		},
		"Given PageProjects returns non 200 response": {
			mockService: mockAdminService{
				getProjectsWithResp: func() (*admingen.GetAllProjectsResponse, error) {
					return &admingen.GetAllProjectsResponse{
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
			input:     pageProjectsInput,
			expected: PageProjectsResult{
				Finished: true,
			},
		},
		"Given PageProjects returns 200 with no Projects": {
			mockService: mockAdminService{
				getProjectsWithResp: func() (*admingen.GetAllProjectsResponse, error) {
					return &admingen.GetAllProjectsResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusOK},
						JSON200: &struct {
							CorrelationId *string                 `json:"correlationId,omitempty"`
							Data          *admingen.Projects      `json:"data,omitempty"`
							MetaData      *map[string]interface{} `json:"metaData,omitempty"`
							Status        *admingen.Status        `json:"status,omitempty"`
						}{
							Data: &admingen.Projects{Projects: &[]admingen.Project{}},
						},
					}, nil
				},
			},
			shouldErr: false,
			input:     pageProjectsInput,
			expected: PageProjectsResult{
				Finished: true,
			},
		},
		"Given PageProjects returns 200 with Project": {
			mockService: mockAdminService{
				getProjectsWithResp: func() (*admingen.GetAllProjectsResponse, error) {
					return &admingen.GetAllProjectsResponse{
						Body:         nil,
						HTTPResponse: &http.Response{StatusCode: http.StatusOK},
						JSON200: &struct {
							CorrelationId *string                 `json:"correlationId,omitempty"`
							Data          *admingen.Projects      `json:"data,omitempty"`
							MetaData      *map[string]interface{} `json:"metaData,omitempty"`
							Status        *admingen.Status        `json:"status,omitempty"`
						}{
							Data: &admingen.Projects{Projects: &[]admingen.Project{project}},
						},
					}, nil
				},
			},
			shouldErr: false,
			input:     pageProjectsInput,
			expected: PageProjectsResult{
				Finished: false,
				Projects: []admingen.Project{project},
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			logger, _ := log.NewStructuredLogger(true)
			adminService, _ := NewAdminService(logger, "localhost:8000", "svc-token")
			adminService.client = tc.mockService

			actual, err := adminService.PageProjects(context.Background(), tc.input)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, tc.expected, actual)

		})
	}
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
		"Given PageTargets returns 200 with Environments": {
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
