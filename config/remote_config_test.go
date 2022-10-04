package config

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/harness/ff-proxy/domain"
	admingen "github.com/harness/ff-proxy/gen/admin"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/services"
	"github.com/stretchr/testify/assert"
)

type mockHasher struct {
}

func (m mockHasher) Hash(s string) string {
	return s
}

// projects is a map of account-org identifiers to projects
var projects = map[string][]admingen.Project{
	"account1-org1": []admingen.Project{
		{
			Identifier: "FeatureFlagsQA",
		},
		{
			Identifier: "FeatureFlagsDev",
		},
	},
}

// Environments is a map of ProjectIdentifiers to environments
var environments = map[string][]admingen.Environment{
	"FeatureFlagsQA": []admingen.Environment{
		{
			Id:         strPtr("123"),
			Identifier: "QA",
			ApiKeys: admingen.ApiKeys{
				ApiKeys: &[]admingen.ApiKey{
					{
						Key: strPtr("1"),
					},
					{
						Key: strPtr("2"),
					},
					{
						Key: strPtr("3"),
					},
				},
			},
		},
	},
	"FeatureFlagsDev": []admingen.Environment{
		{
			Id:         strPtr("456"),
			Identifier: "Dev",
			ApiKeys: admingen.ApiKeys{
				ApiKeys: &[]admingen.ApiKey{
					{
						Key: strPtr("4"),
					},
				},
			},
		},
	},
}

// targets is a map of Project-EnvIdentifiers to targets
var targets = map[string][]admingen.Target{
	"FeatureFlagsQA-QA": []admingen.Target{
		{
			Identifier: "QA-Target-1",
		},
		{
			Identifier: "QA-Target-2",
		},
	},
	"FeatureFlagsDev-Dev": []admingen.Target{
		{
			Identifier: "Dev-Target-1",
		},
		{
			Identifier: "Dev-Target-2",
		},
	},
}

type mockAdminClient struct {
	projects     map[string][]admingen.Project
	environments map[string][]admingen.Environment
	targets      map[string][]admingen.Target
	hit          int
	*sync.Mutex
}

func (m mockAdminClient) PageProjects(ctx context.Context, input services.PageProjectsInput) (services.PageProjectsResult, error) {
	m.Lock()
	defer m.Unlock()

	key := fmt.Sprintf("%s-%s", input.AccountIdentifier, input.OrgIdentifier)

	projects, ok := m.projects[key]
	if !ok {
		return services.PageProjectsResult{Projects: []admingen.Project{}, Finished: true}, errors.New("project not found")
	}

	return services.PageProjectsResult{Projects: projects, Finished: true}, nil
}

func (m mockAdminClient) PageEnvironments(ctx context.Context, input services.PageEnvironmentsInput) (services.PageEnvironmentsResult, error) {
	m.Lock()
	defer m.Unlock()

	defer func() {
		m.hit++
	}()

	environments, ok := m.environments[input.ProjectIdentifier]
	if !ok {
		return services.PageEnvironmentsResult{Environments: []admingen.Environment{}, Finished: true}, errors.New("environment not found")
	}

	return services.PageEnvironmentsResult{Environments: environments, Finished: true}, nil
}

func (m mockAdminClient) PageTargets(ctx context.Context, input services.PageTargetsInput) (services.PageTargetsResult, error) {
	m.Lock()
	defer m.Unlock()

	key := fmt.Sprintf("%s-%s", input.ProjectIdentifier, input.EnvironmentIdentifier)

	targets, ok := m.targets[key]
	if !ok {
		return services.PageTargetsResult{Targets: []admingen.Target{}, Finished: true}, errors.New("target not found")
	}

	return services.PageTargetsResult{Targets: targets, Finished: true}, nil
}

const (
	accountIdentifer = "account1"
	orgIdentifier    = "org1"
)

var (
	allowAllAPIKeys  = []string{"1", "2", "3", "4"}
	allowSomeAPIKeys = []string{"1", "2", "3"}

	allowAllAPIKeysMap = map[string]struct{}{
		"1": struct{}{},
		"2": struct{}{},
		"3": struct{}{},
		"4": struct{}{},
	}

	expectedAuthConfigAllAPIKeys = map[domain.AuthAPIKey]string{
		domain.AuthAPIKey("1"): "123",
		domain.AuthAPIKey("2"): "123",
		domain.AuthAPIKey("3"): "123",
		domain.AuthAPIKey("4"): "456",
	}

	expectedAuthConfigSomeAPIKeys = map[domain.AuthAPIKey]string{
		domain.AuthAPIKey("1"): "123",
		domain.AuthAPIKey("2"): "123",
		domain.AuthAPIKey("3"): "123",
	}

	expectedTargetConfigAllAPIKeys = map[domain.TargetKey][]domain.Target{
		domain.NewTargetKey("123"): []domain.Target{
			{
				Target: admingen.Target{Identifier: "QA-Target-1"},
			},
			{
				Target: admingen.Target{Identifier: "QA-Target-2"},
			},
		},
		domain.NewTargetKey("456"): []domain.Target{
			{
				Target: admingen.Target{Identifier: "Dev-Target-1"},
			},
			{
				Target: admingen.Target{Identifier: "Dev-Target-2"},
			},
		},
	}

	expectedTargetConfigSomeAPIKeys = map[domain.TargetKey][]domain.Target{
		domain.NewTargetKey("123"): []domain.Target{
			{
				Target: admingen.Target{Identifier: "QA-Target-1"},
			},
			{
				Target: admingen.Target{Identifier: "QA-Target-2"},
			},
		},
	}
)

func TestRemoteConfig(t *testing.T) {
	testCases := map[string]struct {
		accountIdentifier    string
		orgIdentifier        string
		allowedAPIKeys       []string
		cancel               bool
		shouldErr            bool
		expectedAuthConfig   map[domain.AuthAPIKey]string
		expectedTargetConfig map[domain.TargetKey][]domain.Target
	}{
		"Given I try to load config for an account and org that exist and I allow all the possible APIKeys": {
			accountIdentifier:    accountIdentifer,
			orgIdentifier:        orgIdentifier,
			allowedAPIKeys:       allowAllAPIKeys,
			shouldErr:            false,
			expectedAuthConfig:   expectedAuthConfigAllAPIKeys,
			expectedTargetConfig: expectedTargetConfigAllAPIKeys,
		},
		"Given I try to load config for an account and org that exist and I only allow APIKeys 1, 2 and 3": {
			accountIdentifier:    accountIdentifer,
			orgIdentifier:        orgIdentifier,
			allowedAPIKeys:       allowSomeAPIKeys,
			shouldErr:            false,
			expectedAuthConfig:   expectedAuthConfigSomeAPIKeys,
			expectedTargetConfig: expectedTargetConfigSomeAPIKeys,
		},
		"Given I try to load config for an account and org that don't exist": {
			accountIdentifier:    "foo",
			orgIdentifier:        "bar",
			shouldErr:            true,
			expectedAuthConfig:   map[domain.AuthAPIKey]string{},
			expectedTargetConfig: map[domain.TargetKey][]domain.Target{},
		},
		"Given the context is canceled immediately": {
			accountIdentifier:    "account1",
			orgIdentifier:        "org1",
			cancel:               true,
			shouldErr:            false,
			expectedAuthConfig:   map[domain.AuthAPIKey]string{},
			expectedTargetConfig: map[domain.TargetKey][]domain.Target{},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}

			adminClient := mockAdminClient{
				projects:     projects,
				environments: environments,
				targets:      targets,
				Mutex:        &sync.Mutex{},
			}

			rc, err := NewRemoteConfig(ctx, tc.accountIdentifier, tc.orgIdentifier, tc.allowedAPIKeys, mockHasher{}, adminClient, WithConcurrency(1), WithLogger(log.NoOpLogger{}))
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			actualAuthConfig := rc.AuthConfig()
			actualTargetConfig := rc.TargetConfig()

			assert.Equal(t, tc.expectedAuthConfig, actualAuthConfig)
			assert.Equal(t, tc.expectedTargetConfig, actualTargetConfig)
		})
	}
}

func TestMakeConfig(t *testing.T) {

	target1 := domain.Target{
		admingen.Target{
			Name:        "Target-1",
			Environment: "123",
			Org:         "foo",
			Project:     "Bar",
		},
	}

	target2 := domain.Target{
		admingen.Target{
			Name:        "Target-2",
			Environment: "123",
			Org:         "foo",
			Project:     "Bar",
		},
	}

	target3 := domain.Target{
		admingen.Target{
			Name:        "Target-3",
			Environment: "123",
			Org:         "foo",
			Project:     "Bar",
		},
	}

	input := []configPipeline{
		{
			AccountIdentifier:     "account1",
			OrgIdentifier:         "org1",
			EnvironmentID:         "123",
			EnvironmentIdentifier: "env1",
			APIKeys:               []string{"1", "2", "3", "4", "5", "6"},
			Targets:               []domain.Target{target1, target2},
		},
		{
			AccountIdentifier:     "account1",
			OrgIdentifier:         "org1",
			EnvironmentID:         "123",
			EnvironmentIdentifier: "env1",
			APIKeys:               []string{},
			Targets:               []domain.Target{target3},
		},
	}

	results := make(chan configPipeline, len(input))
	for _, i := range input {
		results <- i
	}
	close(results)

	expectedAuth := map[domain.AuthAPIKey]string{
		domain.AuthAPIKey("1"): "123",
		domain.AuthAPIKey("2"): "123",
		domain.AuthAPIKey("3"): "123",
		domain.AuthAPIKey("4"): "123",
		domain.AuthAPIKey("5"): "123",
		domain.AuthAPIKey("6"): "123",
	}

	expectedTargets := map[domain.TargetKey][]domain.Target{
		domain.NewTargetKey("123"): []domain.Target{
			target1, target2, target3,
		},
	}

	expectedProjEnvInfo := map[string]configPipeline{
		"123": {
			AccountIdentifier:     "account1",
			OrgIdentifier:         "org1",
			EnvironmentID:         "123",
			EnvironmentIdentifier: "env1",
		},
	}

	config := makeConfigs(results)

	assert.Equal(t, expectedAuth, config.auth)
	assert.Equal(t, expectedTargets, config.targets)
	assert.Equal(t, expectedProjEnvInfo, config.projectEnvironments)
}

func TestPollTargets(t *testing.T) {
	admingenTarget1 := admingen.Target{
		Identifier:  "target1",
		Name:        "target1",
		Environment: "123",
	}

	newTarget1 := domain.Target{
		Target: admingenTarget1,
	}

	expectedNewTargets := map[domain.TargetKey][]domain.Target{}
	for k, t := range expectedTargetConfigAllAPIKeys {
		expectedNewTargets[k] = t
	}
	key456 := domain.NewTargetKey("456")
	expectedNewTargets[key456] = append(expectedNewTargets[key456], newTarget1)

	testCases := map[string]struct {
		accountIdentifier string
		orgIdentifier     string
		allowedAPIKeys    []string
		cancel            bool
		shouldErr         bool
		targetsToAdd      []admingen.Target
		expectedTargets   map[domain.TargetKey][]domain.Target
	}{
		"Given I have a RemoteConfig with Targets and I don't add new ones to the admin client": {
			accountIdentifier: accountIdentifer,
			orgIdentifier:     orgIdentifier,
			allowedAPIKeys:    allowAllAPIKeys,
			cancel:            false,
			shouldErr:         false,
			expectedTargets:   expectedTargetConfigAllAPIKeys,
		},
		"Given I have a RemoteConfig with Targets and I add a new Target to the admin client": {
			accountIdentifier: accountIdentifer,
			orgIdentifier:     orgIdentifier,
			allowedAPIKeys:    allowAllAPIKeys,
			cancel:            false,
			shouldErr:         false,
			targetsToAdd:      []admingen.Target{admingenTarget1},
			expectedTargets:   expectedNewTargets,
		},
		"Given I have a RemoteConfig with Targets I poll but the context is canceled immediately": {
			accountIdentifier: accountIdentifer,
			orgIdentifier:     orgIdentifier,
			allowedAPIKeys:    allowAllAPIKeys,
			cancel:            true,
			shouldErr:         false,
			targetsToAdd:      []admingen.Target{admingenTarget1},
			expectedTargets:   nil,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ticker := make(chan time.Time)

			if tc.cancel {
				defer close(ticker)
				cancel()
			} else {
				close(ticker)
			}

			targetsCopy := map[string][]admingen.Target{}
			for key, value := range targets{
				targetsCopy[key] = value
			}

			adminClient := mockAdminClient{
				projects:     projects,
				environments: environments,
				targets:      targetsCopy,
				Mutex:        &sync.Mutex{},
			}

			remoteConfig, err := NewRemoteConfig(ctx, tc.accountIdentifier, tc.orgIdentifier, tc.allowedAPIKeys, mockHasher{}, adminClient)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			if len(tc.targetsToAdd) > 0 {
				key := string("FeatureFlagsDev-Dev")

				adminClient.Lock()
				adminClient.targets[key] = append(adminClient.targets[key], tc.targetsToAdd...)
				adminClient.Unlock()
			}

			// Only poll once for testing
			actual := <-remoteConfig.PollTargets(ctx, ticker)
			if !reflect.DeepEqual(tc.expectedTargets, actual) {
				t.Errorf("(%s) expected: %v \n got: %v", desc, tc.expectedTargets, actual)
			}
		})
	}
}
