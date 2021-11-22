package config

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
}

func (m mockAdminClient) PageProjects(ctx context.Context, input services.PageProjectsInput) (services.PageProjectsResult, error) {
	key := fmt.Sprintf("%s-%s", input.AccountIdentifier, input.OrgIdentifier)

	projects, ok := m.projects[key]
	if !ok {
		return services.PageProjectsResult{Projects: []admingen.Project{}, Finished: true}, errors.New("project not found")
	}

	return services.PageProjectsResult{Projects: projects, Finished: true}, nil
}

func (m mockAdminClient) PageEnvironments(ctx context.Context, input services.PageEnvironmentsInput) (services.PageEnvironmentsResult, error) {
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
	key := fmt.Sprintf("%s-%s", input.ProjectIdentifier, input.EnvironmentIdentifier)

	targets, ok := m.targets[key]
	if !ok {
		return services.PageTargetsResult{Targets: []admingen.Target{}, Finished: true}, errors.New("target not found")
	}

	return services.PageTargetsResult{Targets: targets, Finished: true}, nil
}

func TestRemoteConfig(t *testing.T) {
	const (
		accountIdentifer = "account1"
		orgIdentifier    = "org1"
	)

	allowAllAPIKeys := []string{"1", "2", "3", "4"}
	allowSomeAPIKeys := []string{"1", "2", "3"}

	expectedAuthConfigAllAPIKeys := map[domain.AuthAPIKey]string{
		domain.AuthAPIKey("1"): "123",
		domain.AuthAPIKey("2"): "123",
		domain.AuthAPIKey("3"): "123",
		domain.AuthAPIKey("4"): "456",
	}

	expectedAuthConfigSomeAPIKeys := map[domain.AuthAPIKey]string{
		domain.AuthAPIKey("1"): "123",
		domain.AuthAPIKey("2"): "123",
		domain.AuthAPIKey("3"): "123",
	}

	expectedTargetConfigAllAPIKeys := map[domain.TargetKey][]domain.Target{
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

	expectedTargetConfigSomeAPIKeys := map[domain.TargetKey][]domain.Target{
		domain.NewTargetKey("123"): []domain.Target{
			{
				Target: admingen.Target{Identifier: "QA-Target-1"},
			},
			{
				Target: admingen.Target{Identifier: "QA-Target-2"},
			},
		},
	}

	testCases := map[string]struct {
		accountIdentifier    string
		orgIdentifier        string
		allowedAPIKeys       []string
		cancel               bool
		expectedAuthConfig   map[domain.AuthAPIKey]string
		expectedTargetConfig map[domain.TargetKey][]domain.Target
	}{
		"Given I try to load config for an account and org that exist and I allow all the possible APIKeys": {
			accountIdentifier:    accountIdentifer,
			orgIdentifier:        orgIdentifier,
			allowedAPIKeys:       allowAllAPIKeys,
			expectedAuthConfig:   expectedAuthConfigAllAPIKeys,
			expectedTargetConfig: expectedTargetConfigAllAPIKeys,
		},
		"Given I try to load config for an account and org that exist and I only allow APIKeys 1, 2 and 3": {
			accountIdentifier:    accountIdentifer,
			orgIdentifier:        orgIdentifier,
			allowedAPIKeys:       allowSomeAPIKeys,
			expectedAuthConfig:   expectedAuthConfigSomeAPIKeys,
			expectedTargetConfig: expectedTargetConfigSomeAPIKeys,
		},
		"Given I try to load config for an account and org that don't exist": {
			accountIdentifier:    "foo",
			orgIdentifier:        "bar",
			expectedAuthConfig:   map[domain.AuthAPIKey]string{},
			expectedTargetConfig: map[domain.TargetKey][]domain.Target{},
		},
		"Given the context is canceled immediately": {
			accountIdentifier:    "account1",
			orgIdentifier:        "org1",
			cancel:               true,
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
			}

			rc := NewRemoteConfig(ctx, tc.accountIdentifier, tc.orgIdentifier, tc.allowedAPIKeys, mockHasher{}, adminClient, WithConcurrency(1), WithLogger(log.NoOpLogger{}))
			actualAuthConfig := rc.AuthConfig()
			actualTargetConfig := rc.TargetConfig()

			assert.Equal(t, tc.expectedAuthConfig, actualAuthConfig)
			assert.Equal(t, tc.expectedTargetConfig, actualTargetConfig)
		})
	}
}
