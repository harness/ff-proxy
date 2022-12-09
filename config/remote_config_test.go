package config

import (
	"context"
	"fmt"
	"testing"

	"github.com/harness/ff-proxy/gen/admin"

	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/services"

	"github.com/stretchr/testify/assert"
)

const (
	account     = "account"
	org         = "org"
	project     = "project"
	environment = "environment"
)

type mockAdminService struct {
	pageTargets func(input services.PageTargetsInput) (services.PageTargetsResult, error)
	pageApiKeys func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error)
}

func (m mockAdminService) PageTargets(ctx context.Context, input services.PageTargetsInput) (services.PageTargetsResult, error) {
	return m.pageTargets(input)
}

func (m mockAdminService) PageAPIKeys(ctx context.Context, input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
	return m.pageApiKeys(input)
}

type mockClientService struct {
	authenticate func(apiKey string) (string, error)
}

func (m mockClientService) Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error) {
	return m.authenticate(apiKey)
}

func TestRemoteConfig_NewRemoteConfig(t *testing.T) {
	validKey := "valid_key"
	validKeyEnv2 := "valid_key_env2"
	invalidKey := "invalid_key"
	type NewRemoteConfigInput struct {
		accountIdentifier string
		orgIdentifier     string
		apiKeys           []string
		adminService      adminService
		clientService     mockClientService
	}
	type NewRemoteConfigOutput struct {
		accountIdentifier string
		orgIdentifier     string
		projEnvInfo       map[string]environmentDetails
		authConfig        map[domain.AuthAPIKey]string
		targetConfig      map[domain.TargetKey][]domain.Target
	}

	testCases := map[string]struct {
		input     NewRemoteConfigInput
		shouldErr bool
		expected  NewRemoteConfigOutput
	}{
		"NewRemoteConfig returns empty with no api keys": {
			shouldErr: false,
			input: NewRemoteConfigInput{
				accountIdentifier: account,
				orgIdentifier:     org,
				apiKeys:           []string{},
				adminService:      nil,
				clientService:     mockClientService{},
			},
			expected: NewRemoteConfigOutput{
				accountIdentifier: account,
				orgIdentifier:     org,
				projEnvInfo:       map[string]environmentDetails{},
				authConfig:        map[domain.AuthAPIKey]string{},
				targetConfig:      map[domain.TargetKey][]domain.Target{},
			},
		},
		"NewRemoteConfig returns valid data for one key": {
			shouldErr: false,
			input: NewRemoteConfigInput{
				accountIdentifier: account,
				orgIdentifier:     org,
				apiKeys:           []string{validKey},
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					return services.PageAPIKeysResult{
						APIKeys: []admin.ApiKey{{
							Key: strPtr("key1"),
						}},
						Finished: true,
					}, nil
				}, pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					return services.PageTargetsResult{
						Targets: []admin.Target{{
							Identifier: "target1",
						}},
						Finished: true,
					}, nil
				}},
			},
			expected: NewRemoteConfigOutput{
				accountIdentifier: account,
				orgIdentifier:     org,
				projEnvInfo: map[string]environmentDetails{"0000-0000-0000-0000-0000": {
					EnvironmentIdentifier: "env",
					EnvironmentId:         "0000-0000-0000-0000-0000",
					ProjectIdentifier:     "project",
					HashedAPIKeys:         []string{"key1"},
					APIKey:                validKey,
					Targets: []domain.Target{{admin.Target{
						Identifier: "target1",
					}}},
				}},
				authConfig: map[domain.AuthAPIKey]string{"key1": "0000-0000-0000-0000-0000"},
				targetConfig: map[domain.TargetKey][]domain.Target{"env-0000-0000-0000-0000-0000-target-config": {{admin.Target{
					Identifier: "target1",
				}}}},
			},
		},
		"NewRemoteConfig returns one set of data if given two keys for same environment": {
			shouldErr: false,
			input: NewRemoteConfigInput{
				accountIdentifier: account,
				orgIdentifier:     org,
				apiKeys:           []string{"valid_key_same_env", validKey},
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					return services.PageAPIKeysResult{
						APIKeys: []admin.ApiKey{{
							Key: strPtr("key1"),
						}},
						Finished: true,
					}, nil
				}, pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					return services.PageTargetsResult{
						Targets: []admin.Target{{
							Identifier: "target1",
						}},
						Finished: true,
					}, nil
				}},
			},
			expected: NewRemoteConfigOutput{
				accountIdentifier: account,
				orgIdentifier:     org,
				projEnvInfo: map[string]environmentDetails{"0000-0000-0000-0000-0000": {
					EnvironmentIdentifier: "env",
					EnvironmentId:         "0000-0000-0000-0000-0000",
					ProjectIdentifier:     "project",
					HashedAPIKeys:         []string{"key1"},
					APIKey:                validKey,
					Targets: []domain.Target{{admin.Target{
						Identifier: "target1",
					}}},
				}},
				authConfig: map[domain.AuthAPIKey]string{"key1": "0000-0000-0000-0000-0000"},
				targetConfig: map[domain.TargetKey][]domain.Target{"env-0000-0000-0000-0000-0000-target-config": {{admin.Target{
					Identifier: "target1",
				}}}},
			},
		},
		"NewRemoteConfig returns one set of data if one key fails": {
			shouldErr: false,
			input: NewRemoteConfigInput{
				accountIdentifier: account,
				orgIdentifier:     org,
				apiKeys:           []string{invalidKey, validKey},
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					if apiKey == invalidKey {
						return "", fmt.Errorf("request failed")
					}
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					return services.PageAPIKeysResult{
						APIKeys: []admin.ApiKey{{
							Key: strPtr("key1"),
						}},
						Finished: true,
					}, nil
				}, pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					return services.PageTargetsResult{
						Targets: []admin.Target{{
							Identifier: "target1",
						}},
						Finished: true,
					}, nil
				}},
			},
			expected: NewRemoteConfigOutput{
				accountIdentifier: account,
				orgIdentifier:     org,
				projEnvInfo: map[string]environmentDetails{"0000-0000-0000-0000-0000": {
					EnvironmentIdentifier: "env",
					EnvironmentId:         "0000-0000-0000-0000-0000",
					ProjectIdentifier:     "project",
					HashedAPIKeys:         []string{"key1"},
					APIKey:                validKey,
					Targets: []domain.Target{{admin.Target{
						Identifier: "target1",
					}}},
				}},
				authConfig: map[domain.AuthAPIKey]string{"key1": "0000-0000-0000-0000-0000"},
				targetConfig: map[domain.TargetKey][]domain.Target{"env-0000-0000-0000-0000-0000-target-config": {{admin.Target{
					Identifier: "target1",
				}}}},
			},
		},
		"NewRemoteConfig returns data for multiple envs": {
			shouldErr: false,
			input: NewRemoteConfigInput{
				accountIdentifier: account,
				orgIdentifier:     org,
				apiKeys:           []string{validKey, validKeyEnv2},
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					if apiKey == validKeyEnv2 {
						return "header.eyJlbnZpcm9ubWVudCI6IjExMTEtMTExMS0xMTExLTExMTEtMTExMSIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudjIiLCJwcm9qZWN0SWRlbnRpZmllciI6InByb2plY3QyIiwiY2x1c3RlcklkZW50aWZpZXIiOiIyIn0.signature", nil
					}
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					if input.EnvironmentIdentifier == "env2" {
						return services.PageAPIKeysResult{
							APIKeys: []admin.ApiKey{{
								Key: strPtr("key2"),
							}},
							Finished: true,
						}, nil
					}
					return services.PageAPIKeysResult{
						APIKeys: []admin.ApiKey{{
							Key: strPtr("key1"),
						}},
						Finished: true,
					}, nil
				}, pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					return services.PageTargetsResult{
						Targets: []admin.Target{{
							Identifier: "target1",
						}},
						Finished: true,
					}, nil
				}},
			},
			expected: NewRemoteConfigOutput{
				accountIdentifier: account,
				orgIdentifier:     org,
				projEnvInfo: map[string]environmentDetails{"0000-0000-0000-0000-0000": {
					EnvironmentIdentifier: "env",
					EnvironmentId:         "0000-0000-0000-0000-0000",
					ProjectIdentifier:     "project",
					HashedAPIKeys:         []string{"key1"},
					APIKey:                validKey,
					Targets: []domain.Target{{admin.Target{
						Identifier: "target1",
					}}},
				}, "1111-1111-1111-1111-1111": {
					EnvironmentIdentifier: "env2",
					EnvironmentId:         "1111-1111-1111-1111-1111",
					ProjectIdentifier:     "project2",
					HashedAPIKeys:         []string{"key2"},
					APIKey:                validKeyEnv2,
					Targets: []domain.Target{{admin.Target{
						Identifier: "target1",
					}}},
				}},
				authConfig: map[domain.AuthAPIKey]string{"key1": "0000-0000-0000-0000-0000", "key2": "1111-1111-1111-1111-1111"},
				targetConfig: map[domain.TargetKey][]domain.Target{"env-0000-0000-0000-0000-0000-target-config": {{admin.Target{
					Identifier: "target1",
				}}}, "env-1111-1111-1111-1111-1111-target-config": {{admin.Target{
					Identifier: "target1",
				}}}},
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			actual, err := NewRemoteConfig(context.Background(), tc.input.accountIdentifier, tc.input.orgIdentifier, tc.input.apiKeys, tc.input.adminService, tc.input.clientService, WithLogger(nil), WithFetchTargets(true))

			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			// check remoteConfig results
			assert.Equal(t, tc.expected.accountIdentifier, actual.accountIdentifier)
			assert.Equal(t, tc.expected.orgIdentifier, actual.orgIdentifier)
			assert.Equal(t, tc.expected.projEnvInfo, actual.EnvInfo())

			// check AuthConfig and TargetConfig produced
			assert.Equal(t, tc.expected.authConfig, actual.AuthConfig())
			assert.Equal(t, tc.expected.targetConfig, actual.TargetConfig())
		})
	}
}

func TestRemoteConfig_getEnvironmentInfo(t *testing.T) {
	validKey := "valid_key"
	type GetEnvironmentInput struct {
		apiKey        string
		clientService mockClientService
	}

	type GetEnvironmentResp struct {
		projectIdentifier     string
		environmentIdentifier string
		environmentID         string
		err                   error
	}

	testCases := map[string]struct {
		input     GetEnvironmentInput
		shouldErr bool
		expected  GetEnvironmentResp
	}{
		"Given clientService.Authenticate returns err": {
			shouldErr: true,
			input: GetEnvironmentInput{
				apiKey: validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "", fmt.Errorf("request failed")
				}},
			},
			expected: GetEnvironmentResp{
				projectIdentifier:     "",
				environmentIdentifier: "",
				environmentID:         "",
				err:                   fmt.Errorf("error sending client authentication request: request failed"),
			},
		},
		"Given empty jwt returned": {
			shouldErr: true,
			input: GetEnvironmentInput{
				apiKey: validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "", nil
				}},
			},
			expected: GetEnvironmentResp{
				projectIdentifier:     "",
				environmentIdentifier: "",
				environmentID:         "",
				err:                   fmt.Errorf("invalid jwt received %s", ""),
			},
		},
		"Given invalid base64 jwt returned": {
			shouldErr: true,
			input: GetEnvironmentInput{
				apiKey: validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.invalid_b@s£64.signature", nil
				}},
			},
			expected: GetEnvironmentResp{
				projectIdentifier:     "",
				environmentIdentifier: "",
				environmentID:         "",
				err:                   fmt.Errorf("failed to parse token claims for key valid_key: illegal base64 data at input byte 9"),
			},
		},
		"Given invalid non json jwt returned": {
			shouldErr: true,
			input: GetEnvironmentInput{
				apiKey: validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.dGVzdA.signature", nil
				}},
			},
			expected: GetEnvironmentResp{
				projectIdentifier:     "",
				environmentIdentifier: "",
				environmentID:         "",
				err:                   fmt.Errorf("failed to unmarshal token claims for key valid_key: invalid character 'e' in literal true (expecting 'r')"),
			},
		},
		"Given payload without environmentIdentifier returned": {
			shouldErr: true,
			input: GetEnvironmentInput{
				apiKey: validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
			},
			expected: GetEnvironmentResp{
				projectIdentifier:     "",
				environmentIdentifier: "",
				environmentID:         "",
				err:                   fmt.Errorf("environment identifier not present in bearer token"),
			},
		},
		"Given payload without environment returned": {
			shouldErr: true,
			input: GetEnvironmentInput{
				apiKey: validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudElkZW50aWZpZXIiOiJlbnYiLCJwcm9qZWN0SWRlbnRpZmllciI6InByb2plY3QiLCJjbHVzdGVySWRlbnRpZmllciI6IjIifQ.signature", nil
				}},
			},
			expected: GetEnvironmentResp{
				projectIdentifier:     "",
				environmentIdentifier: "",
				environmentID:         "",
				err:                   fmt.Errorf("environment id not present in bearer token"),
			},
		},
		"Given payload without project returned": {
			shouldErr: true,
			input: GetEnvironmentInput{
				apiKey: validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
			},
			expected: GetEnvironmentResp{
				projectIdentifier:     "",
				environmentIdentifier: "",
				environmentID:         "",
				err:                   fmt.Errorf("project identifier not present in bearer token"),
			},
		},
		"Given valid payload returned": {
			shouldErr: false,
			input: GetEnvironmentInput{
				apiKey: validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
			},
			expected: GetEnvironmentResp{
				projectIdentifier:     "project",
				environmentIdentifier: "env",
				environmentID:         "0000-0000-0000-0000-0000",
				err:                   nil,
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			projectIdentifier, environmentIdentifier, environmentID, err := getEnvironmentInfo(context.Background(), tc.input.apiKey, tc.input.clientService)

			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			// check results
			assert.Equal(t, tc.expected.err, err)
			assert.Equal(t, tc.expected.projectIdentifier, projectIdentifier)
			assert.Equal(t, tc.expected.environmentIdentifier, environmentIdentifier)
			assert.Equal(t, tc.expected.environmentID, environmentID)
		})
	}
}

func TestRemoteConfig_getApiKeys(t *testing.T) {
	type GetApiKeysInput struct {
		accountIdentifier     string
		orgIdentifier         string
		projectIdentifier     string
		environmentIdentifier string
		adminService          mockAdminService
	}

	type GetApiKeysResp struct {
		hashedApiKeys []string
		err           error
	}

	testCases := map[string]struct {
		input     GetApiKeysInput
		shouldErr bool
		expected  GetApiKeysResp
	}{
		"Given adminService.PageAPIKeys returns err": {
			shouldErr: true,
			input: GetApiKeysInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					return services.PageAPIKeysResult{}, fmt.Errorf("request failed")
				}},
			},
			expected: GetApiKeysResp{
				hashedApiKeys: []string{},
				err:           fmt.Errorf("failed to get api keys: request failed"),
			},
		},
		"Given adminService.PageAPIKeys returns one page of results": {
			shouldErr: false,
			input: GetApiKeysInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					return services.PageAPIKeysResult{
						APIKeys: []admin.ApiKey{{
							Key: strPtr("key1"),
						}},
						Finished: true,
					}, nil
				}},
			},
			expected: GetApiKeysResp{
				hashedApiKeys: []string{"key1"},
				err:           nil,
			},
		},
		"Given adminService.PageAPIKeys returns two pages of results": {
			shouldErr: false,
			input: GetApiKeysInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					// first page results
					if input.PageNumber == 0 {
						return services.PageAPIKeysResult{
							APIKeys: []admin.ApiKey{{
								Key: strPtr("key1"),
							}},
							Finished: false,
						}, nil
					}
					// second page results
					if input.PageNumber == 1 {
						return services.PageAPIKeysResult{
							APIKeys: []admin.ApiKey{{
								Key: strPtr("key2"),
							}},
							Finished: true,
						}, nil
					}

					// won't ever be hit
					return services.PageAPIKeysResult{}, fmt.Errorf("this won't be hit")
				}},
			},
			expected: GetApiKeysResp{
				hashedApiKeys: []string{"key1", "key2"},
				err:           nil,
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			actualKeys, err := getAPIKeys(context.Background(), tc.input.accountIdentifier, tc.input.orgIdentifier, tc.input.projectIdentifier, tc.input.environmentIdentifier, tc.input.adminService)

			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			// check results
			assert.Equal(t, tc.expected.err, err)
			assert.Equal(t, tc.expected.hashedApiKeys, actualKeys)
		})
	}
}

func TestRemoteConfig_getTargets(t *testing.T) {
	type GetTargetsInput struct {
		accountIdentifier     string
		orgIdentifier         string
		projectIdentifier     string
		environmentIdentifier string
		adminService          mockAdminService
	}

	type GetTargetsResp struct {
		targets []domain.Target
		err     error
	}

	testCases := map[string]struct {
		input     GetTargetsInput
		shouldErr bool
		expected  GetTargetsResp
	}{
		"Given adminService.PageTargets returns err": {
			shouldErr: true,
			input: GetTargetsInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				adminService: mockAdminService{pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					return services.PageTargetsResult{}, fmt.Errorf("request failed")
				}},
			},
			expected: GetTargetsResp{
				targets: []domain.Target{},
				err:     fmt.Errorf("failed to get targets: request failed"),
			},
		},
		"Given adminService.PageTargets returns one page of results": {
			shouldErr: false,
			input: GetTargetsInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				adminService: mockAdminService{pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					return services.PageTargetsResult{
						Targets: []admin.Target{{
							Identifier: "target1",
						}},
						Finished: true,
					}, nil
				}},
			},
			expected: GetTargetsResp{
				targets: []domain.Target{{admin.Target{
					Identifier: "target1",
				}}},
				err: nil,
			},
		},
		"Given adminService.PageTargets returns two pages of results": {
			shouldErr: false,
			input: GetTargetsInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				adminService: mockAdminService{pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					// first page
					if input.PageNumber == 0 {
						return services.PageTargetsResult{
							Targets: []admin.Target{{
								Identifier: "target1",
							}},
							Finished: false,
						}, nil
					}

					// second page
					if input.PageNumber == 1 {
						return services.PageTargetsResult{
							Targets: []admin.Target{{
								Identifier: "target2",
							}},
							Finished: true,
						}, nil
					}

					// won't happen
					return services.PageTargetsResult{
						Finished: true,
					}, fmt.Errorf("this won't be hit")

				}},
			},
			expected: GetTargetsResp{
				targets: []domain.Target{{admin.Target{
					Identifier: "target1",
				}}, {admin.Target{
					Identifier: "target2",
				}}},
				err: nil,
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			actualTargets, err := getTargets(context.Background(), tc.input.accountIdentifier, tc.input.orgIdentifier, tc.input.projectIdentifier, tc.input.environmentIdentifier, tc.input.adminService)

			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			// check results
			assert.Equal(t, tc.expected.err, err)
			assert.Equal(t, tc.expected.targets, actualTargets)
		})
	}
}

func TestRemoteConfig_getConfigForKey(t *testing.T) {
	validKey := "valid_key"
	type GetConfigForKeyInput struct {
		accountIdentifier     string
		orgIdentifier         string
		projectIdentifier     string
		environmentIdentifier string
		apiKey                string
		fetchTargets          bool
		clientService         mockClientService
		adminService          mockAdminService
	}

	type GetConfigForKeyResp struct {
		environmentDetails environmentDetails
		err                error
	}

	testCases := map[string]struct {
		input     GetConfigForKeyInput
		shouldErr bool
		expected  GetConfigForKeyResp
	}{
		"Given getEnvironmentInfo returns err empty environmentDetails is returned": {
			shouldErr: true,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				apiKey:                validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "", fmt.Errorf("request failed")
				}},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: environmentDetails{},
				err:                fmt.Errorf("failed to fetch environment details for key valid_key: error sending client authentication request: request failed"),
			},
		},
		"Given getAPIKeys returns err empty environmentDetails is returned": {
			shouldErr: true,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				apiKey:                validKey,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					return services.PageAPIKeysResult{}, fmt.Errorf("request failed")
				}},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: environmentDetails{},
				err:                fmt.Errorf("failed to get api keys: request failed"),
			},
		},
		"Given getTargets returns err empty environmentDetails is returned": {
			shouldErr: true,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				apiKey:                validKey,
				fetchTargets:          true,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					return services.PageAPIKeysResult{
						APIKeys: []admin.ApiKey{{
							Key: strPtr("key1"),
						}},
						Finished: true,
					}, nil
				}, pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					return services.PageTargetsResult{}, fmt.Errorf("request failed")
				}},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: environmentDetails{},
				err:                fmt.Errorf("failed to get targets: request failed"),
			},
		},
		"Given all requests succeed valid environmentDetails is returned": {
			shouldErr: false,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				apiKey:                validKey,
				fetchTargets:          true,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					return services.PageAPIKeysResult{
						APIKeys: []admin.ApiKey{{
							Key: strPtr("key1"),
						}},
						Finished: true,
					}, nil
				}, pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					return services.PageTargetsResult{
						Targets: []admin.Target{{
							Identifier: "target1",
						}},
						Finished: true,
					}, nil
				}},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: environmentDetails{
					EnvironmentIdentifier: "env",
					EnvironmentId:         "0000-0000-0000-0000-0000",
					ProjectIdentifier:     "project",
					HashedAPIKeys:         []string{"key1"},
					APIKey:                validKey,
					Targets: []domain.Target{{admin.Target{
						Identifier: "target1",
					}}},
				},
				err: nil,
			},
		},
		"getTargets is skipped if fetchTargets is false": {
			shouldErr: false,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environment,
				apiKey:                validKey,
				fetchTargets:          false,
				clientService: mockClientService{authenticate: func(apiKey string) (string, error) {
					return "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature", nil
				}},
				adminService: mockAdminService{pageApiKeys: func(input services.GetAPIKeysInput) (services.PageAPIKeysResult, error) {
					return services.PageAPIKeysResult{
						APIKeys: []admin.ApiKey{{
							Key: strPtr("key1"),
						}},
						Finished: true,
					}, nil
				}, pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					return services.PageTargetsResult{}, fmt.Errorf("request failed")
				}},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: environmentDetails{
					EnvironmentIdentifier: "env",
					EnvironmentId:         "0000-0000-0000-0000-0000",
					ProjectIdentifier:     "project",
					HashedAPIKeys:         []string{"key1"},
					APIKey:                validKey,
					Targets:               []domain.Target(nil),
				},
				err: nil,
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			rc := RemoteConfig{
				clientService:     tc.input.clientService,
				adminService:      tc.input.adminService,
				accountIdentifier: tc.input.accountIdentifier,
				orgIdentifier:     tc.input.orgIdentifier,
				fetchTargets:      tc.input.fetchTargets,
			}
			actualEnvDetails, err := rc.getConfigForKey(context.Background(), tc.input.apiKey)

			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			// check results
			assert.Equal(t, tc.expected.err, err)
			assert.Equal(t, tc.expected.environmentDetails, actualEnvDetails)
		})
	}
}
