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

var (
	pageTargetsSuccess = func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
		return services.PageTargetsResult{
			Targets:  []admin.Target{target1},
			Finished: true,
		}, nil
	}

	pageTargetsFail = func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
		return services.PageTargetsResult{}, fmt.Errorf("request failed")
	}

	pageAPIKeysSuccess = func(input services.PageAPIKeysInput) (services.PageAPIKeysResult, error) {
		return services.PageAPIKeysResult{
			APIKeys: []admin.ApiKey{{
				Key: strPtr(defaultAPIKey),
			}},
			Finished: true,
		}, nil
	}

	pageAPIKeysFail = func(input services.PageAPIKeysInput) (services.PageAPIKeysResult, error) {
		return services.PageAPIKeysResult{}, fmt.Errorf("request failed")
	}

	authenticateSuccess = func(apiKey string) (string, error) {
		return validJWT, nil
	}

	authenticateFail = func(apiKey string) (string, error) {
		return "", fmt.Errorf("request failed")
	}

	defaultEnvDetails = EnvironmentDetails{EnvironmentIdentifier: environmentIdentifier,
		EnvironmentID:     defaultEnvironmentID,
		ProjectIdentifier: project,
		HashedAPIKeys:     []string{defaultAPIKey},
		APIKey:            validKey,
		Targets:           []domain.Target{{target1}},
	}

	target1 = admin.Target{
		Identifier: "target1",
	}
)

type mockAdminService struct {
	pageTargets func(input services.PageTargetsInput) (services.PageTargetsResult, error)
	pageAPIKeys func(input services.PageAPIKeysInput) (services.PageAPIKeysResult, error)
}

func (m mockAdminService) PageTargets(ctx context.Context, input services.PageTargetsInput) (services.PageTargetsResult, error) {
	return m.pageTargets(input)
}

func (m mockAdminService) PageAPIKeys(ctx context.Context, input services.PageAPIKeysInput) (services.PageAPIKeysResult, error) {
	return m.pageAPIKeys(input)
}

type mockClientService struct {
	authenticate func(apiKey string) (string, error)
}

func (m mockClientService) Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error) {
	return m.authenticate(apiKey)
}

func TestRemoteConfig_NewRemoteConfig(t *testing.T) {
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
		projEnvInfo       map[string]EnvironmentDetails
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
				projEnvInfo:       map[string]EnvironmentDetails{},
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
				clientService:     mockClientService{authenticate: authenticateSuccess},
				adminService:      mockAdminService{pageAPIKeys: pageAPIKeysSuccess, pageTargets: pageTargetsSuccess},
			},
			expected: NewRemoteConfigOutput{
				accountIdentifier: account,
				orgIdentifier:     org,
				projEnvInfo:       map[string]EnvironmentDetails{defaultEnvironmentID: defaultEnvDetails},
				authConfig:        map[domain.AuthAPIKey]string{defaultAPIKey: defaultEnvironmentID},
				targetConfig:      map[domain.TargetKey][]domain.Target{"env-0000-0000-0000-0000-0000-target-config": {{target1}}},
			},
		},
		"NewRemoteConfig returns one set of data if given two keys for same environment": {
			shouldErr: false,
			input: NewRemoteConfigInput{
				accountIdentifier: account,
				orgIdentifier:     org,
				apiKeys:           []string{"valid_key_same_env", validKey},
				clientService:     mockClientService{authenticate: authenticateSuccess},
				adminService:      mockAdminService{pageAPIKeys: pageAPIKeysSuccess, pageTargets: pageTargetsSuccess},
			},
			expected: NewRemoteConfigOutput{
				accountIdentifier: account,
				orgIdentifier:     org,
				projEnvInfo:       map[string]EnvironmentDetails{defaultEnvironmentID: defaultEnvDetails},
				authConfig:        map[domain.AuthAPIKey]string{defaultAPIKey: defaultEnvironmentID},
				targetConfig:      map[domain.TargetKey][]domain.Target{"env-0000-0000-0000-0000-0000-target-config": {{target1}}},
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
					return validJWT, nil
				}},
				adminService: mockAdminService{pageAPIKeys: pageAPIKeysSuccess, pageTargets: pageTargetsSuccess},
			},
			expected: NewRemoteConfigOutput{
				accountIdentifier: account,
				orgIdentifier:     org,
				projEnvInfo:       map[string]EnvironmentDetails{defaultEnvironmentID: defaultEnvDetails},
				authConfig:        map[domain.AuthAPIKey]string{defaultAPIKey: defaultEnvironmentID},
				targetConfig:      map[domain.TargetKey][]domain.Target{"env-0000-0000-0000-0000-0000-target-config": {{target1}}},
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
					return validJWT, nil
				}},
				adminService: mockAdminService{pageAPIKeys: func(input services.PageAPIKeysInput) (services.PageAPIKeysResult, error) {
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
							Key: strPtr(defaultAPIKey),
						}},
						Finished: true,
					}, nil
				}, pageTargets: pageTargetsSuccess},
			},
			expected: NewRemoteConfigOutput{
				accountIdentifier: account,
				orgIdentifier:     org,
				projEnvInfo: map[string]EnvironmentDetails{defaultEnvironmentID: defaultEnvDetails, "1111-1111-1111-1111-1111": {
					EnvironmentIdentifier: "env2",
					EnvironmentID:         "1111-1111-1111-1111-1111",
					ProjectIdentifier:     "project2",
					HashedAPIKeys:         []string{"key2"},
					APIKey:                validKeyEnv2,
					Targets:               []domain.Target{{target1}},
				}},
				authConfig:   map[domain.AuthAPIKey]string{defaultAPIKey: defaultEnvironmentID, "key2": "1111-1111-1111-1111-1111"},
				targetConfig: map[domain.TargetKey][]domain.Target{"env-0000-0000-0000-0000-0000-target-config": {{target1}}, "env-1111-1111-1111-1111-1111-target-config": {{target1}}},
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
				apiKey:        validKey,
				clientService: mockClientService{authenticateFail},
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
				apiKey:        validKey,
				clientService: mockClientService{authenticate: authenticateSuccess},
			},
			expected: GetEnvironmentResp{
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				environmentID:         defaultEnvironmentID,
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
	type GetAPIKeysInput struct {
		accountIdentifier     string
		orgIdentifier         string
		projectIdentifier     string
		environmentIdentifier string
		adminService          mockAdminService
	}

	type GetAPIKeysResp struct {
		hashedAPIKeys []string
		err           error
	}

	testCases := map[string]struct {
		input     GetAPIKeysInput
		shouldErr bool
		expected  GetAPIKeysResp
	}{
		"Given adminService.PageAPIKeys returns err": {
			shouldErr: true,
			input: GetAPIKeysInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				adminService:          mockAdminService{pageAPIKeys: pageAPIKeysFail},
			},
			expected: GetAPIKeysResp{
				hashedAPIKeys: []string{},
				err:           fmt.Errorf("failed to get api keys: request failed"),
			},
		},
		"Given adminService.PageAPIKeys returns one page of results": {
			shouldErr: false,
			input: GetAPIKeysInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				adminService:          mockAdminService{pageAPIKeys: pageAPIKeysSuccess},
			},
			expected: GetAPIKeysResp{
				hashedAPIKeys: []string{defaultAPIKey},
				err:           nil,
			},
		},
		"Given adminService.PageAPIKeys returns two pages of results": {
			shouldErr: false,
			input: GetAPIKeysInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				adminService: mockAdminService{pageAPIKeys: func(input services.PageAPIKeysInput) (services.PageAPIKeysResult, error) {
					// first page results
					if input.PageNumber == 0 {
						return services.PageAPIKeysResult{
							APIKeys: []admin.ApiKey{{
								Key: strPtr(defaultAPIKey),
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
			expected: GetAPIKeysResp{
				hashedAPIKeys: []string{defaultAPIKey, "key2"},
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
			assert.Equal(t, tc.expected.hashedAPIKeys, actualKeys)
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
				environmentIdentifier: environmentIdentifier,
				adminService:          mockAdminService{pageTargets: pageTargetsFail},
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
				environmentIdentifier: environmentIdentifier,
				adminService:          mockAdminService{pageTargets: pageTargetsSuccess},
			},
			expected: GetTargetsResp{
				targets: []domain.Target{{target1}},
				err:     nil,
			},
		},
		"Given adminService.PageTargets returns two pages of results": {
			shouldErr: false,
			input: GetTargetsInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				adminService: mockAdminService{pageTargets: func(input services.PageTargetsInput) (services.PageTargetsResult, error) {
					// first page
					if input.PageNumber == 0 {
						return services.PageTargetsResult{
							Targets:  []admin.Target{target1},
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
				targets: []domain.Target{{target1}, {admin.Target{
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
		environmentDetails EnvironmentDetails
		err                error
	}

	testCases := map[string]struct {
		input     GetConfigForKeyInput
		shouldErr bool
		expected  GetConfigForKeyResp
	}{
		"Given getEnvironmentInfo returns err empty EnvironmentDetails is returned": {
			shouldErr: true,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				apiKey:                validKey,
				clientService:         mockClientService{authenticateFail},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: EnvironmentDetails{},
				err:                fmt.Errorf("failed to fetch environment details for key valid_key: error sending client authentication request: request failed"),
			},
		},
		"Given getAPIKeys returns err empty EnvironmentDetails is returned": {
			shouldErr: true,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				apiKey:                validKey,
				clientService:         mockClientService{authenticate: authenticateSuccess},
				adminService:          mockAdminService{pageAPIKeys: pageAPIKeysFail},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: EnvironmentDetails{},
				err:                fmt.Errorf("failed to get api keys: request failed"),
			},
		},
		"Given getTargets returns err empty EnvironmentDetails is returned": {
			shouldErr: true,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				apiKey:                validKey,
				fetchTargets:          true,
				clientService:         mockClientService{authenticate: authenticateSuccess},
				adminService:          mockAdminService{pageAPIKeys: pageAPIKeysSuccess, pageTargets: pageTargetsFail},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: EnvironmentDetails{},
				err:                fmt.Errorf("failed to get targets: request failed"),
			},
		},
		"Given all requests succeed valid EnvironmentDetails is returned": {
			shouldErr: false,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				apiKey:                validKey,
				fetchTargets:          true,
				clientService:         mockClientService{authenticate: authenticateSuccess},
				adminService:          mockAdminService{pageAPIKeys: pageAPIKeysSuccess, pageTargets: pageTargetsSuccess},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: defaultEnvDetails,
				err:                nil,
			},
		},
		"getTargets is skipped if fetchTargets is false": {
			shouldErr: false,
			input: GetConfigForKeyInput{
				accountIdentifier:     account,
				orgIdentifier:         org,
				projectIdentifier:     project,
				environmentIdentifier: environmentIdentifier,
				apiKey:                validKey,
				fetchTargets:          false,
				clientService:         mockClientService{authenticate: authenticateSuccess},
				adminService:          mockAdminService{pageAPIKeys: pageAPIKeysSuccess, pageTargets: pageTargetsFail},
			},
			expected: GetConfigForKeyResp{
				environmentDetails: EnvironmentDetails{
					EnvironmentIdentifier: environmentIdentifier,
					EnvironmentID:         defaultEnvironmentID,
					ProjectIdentifier:     project,
					HashedAPIKeys:         []string{defaultAPIKey},
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
