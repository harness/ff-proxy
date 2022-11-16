package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/services"
)

type adminClient interface {
	PageTargets(ctx context.Context, input services.PageTargetsInput) (services.PageTargetsResult, error)
	PageAPIKeys(ctx context.Context, input services.GetAPIKeysInput) (services.PageAPIKeysResult, error)
}

// RemoteOption is type for passing optional parameters to a RemoteConfig
type RemoteOption func(r *RemoteConfig)

// WithConcurrency sets the maximum amount of concurrent requests that the
// RemoteOption can make. It's default value is 10.
func WithConcurrency(i int) RemoteOption {
	return func(r *RemoteConfig) {
		r.concurrency = i
	}
}

// WithLogger can be used to pass a logger to the RemoteConfig, its default logger
// is one that logs to stderr and has debug logging disabled
func WithLogger(l log.Logger) RemoteOption {
	return func(r *RemoteConfig) {
		r.log = l
	}
}

// WithFetchTargets specifies if the RemoteConfig instance should fetch targets or not
func WithFetchTargets(fetchTargets bool) RemoteOption {
	return func(r *RemoteConfig) {
		r.fetchTargets = fetchTargets
	}
}

// RemoteConfig is a type that retrieves config from the Feature Flags Service
type RemoteConfig struct {
	adminService      adminClient
	clientService     services.ClientService
	log               log.Logger
	concurrency       int
	accountIdentifier string
	orgIdentifier     string
	projEnvInfo       map[string]environmentDetails
	fetchTargets      bool
}

// TargetConfig returns the Target information that was retrieved from the Feature Flags Service
func (r RemoteConfig) TargetConfig() map[domain.TargetKey][]domain.Target {
	targetConfig := make(map[domain.TargetKey][]domain.Target)
	for _, env := range r.projEnvInfo {
		targetKey := domain.NewTargetKey(env.EnvironmentId)
		targetConfig[targetKey] = env.Targets
	}
	return targetConfig
}

// AuthConfig returns the AuthConfig that was retrieved from the Feature Flags Service
func (r RemoteConfig) AuthConfig() map[domain.AuthAPIKey]string {
	authConfig := make(map[domain.AuthAPIKey]string)
	for _, env := range r.projEnvInfo {
		for _, hashedKey := range env.HashedAPIKeys {
			authConfig[domain.AuthAPIKey(hashedKey)] = env.EnvironmentId
		}
	}
	return authConfig
}

// EnvInfo returns the AuthConfig that was retrieved from the Feature Flags Service
func (r RemoteConfig) EnvInfo() map[string]environmentDetails {
	return r.projEnvInfo
}

// NewRemoteConfig creates a RemoteConfig and retrieves the configuration for
// the given Account, Org and APIKeys from the Feature Flags Service
func NewRemoteConfig(ctx context.Context, accountIdentifier string, orgIdentifier string, apiKeys []string, adminService adminClient, clientService services.ClientService, opts ...RemoteOption) (RemoteConfig, error) {

	rc := &RemoteConfig{
		adminService:      adminService,
		clientService:     clientService,
		accountIdentifier: accountIdentifier,
		orgIdentifier:     orgIdentifier,
		fetchTargets:      true,
	}

	for _, opt := range opts {
		opt(rc)
	}

	if rc.log == nil {
		rc.log = log.NoOpLogger{}
	}

	if rc.concurrency == 0 {
		rc.concurrency = 10
	}
	rc.log = rc.log.With("component", "RemoteConfig", "account_identifier", accountIdentifier, "org_identifier", orgIdentifier)

	envInfos := map[string]environmentDetails{}

	for _, key := range apiKeys {
		newConfig, err := rc.getConfigForKey(ctx, key)
		if err != nil {
			rc.log.Error("couldn't fetch info for key, skipping", "api key", key)
			continue
		}
		envInfos[newConfig.EnvironmentId] = newConfig
		rc.log.Error("config for key", "api key", key, "config", fmt.Sprintf("%v", newConfig))
	}

	rc.projEnvInfo = envInfos

	return *rc, nil
}

type environmentDetails struct {
	EnvironmentIdentifier string
	EnvironmentId         string
	ProjectIdentifier     string
	HashedAPIKeys         []string
	APIKey                string
	Targets               []domain.Target
}

func (r RemoteConfig) getConfigForKey(ctx context.Context, apiKey string) (environmentDetails, error) {
	// auth key
	projectIdentifier, environmentIdentifier, environmentID, err := getEnvironmentInfo(ctx, apiKey, r.clientService)
	if err != nil {
		return environmentDetails{}, err
	}
	envInfo := environmentDetails{
		EnvironmentIdentifier: environmentIdentifier,
		EnvironmentId:         environmentID,
		ProjectIdentifier:     projectIdentifier,
		APIKey:                apiKey,
		HashedAPIKeys:         nil,
		Targets:               nil,
	}

	// get api keys
	apiKeys, err := getAPIKeys(ctx, r.accountIdentifier, r.orgIdentifier, projectIdentifier, environmentIdentifier, r.adminService)
	if err != nil {
		return environmentDetails{}, err
	}
	envInfo.HashedAPIKeys = apiKeys

	// get targets
	var targets []domain.Target
	if r.fetchTargets {
		targets, err = getTargets(ctx, r.accountIdentifier, r.orgIdentifier, projectIdentifier, environmentIdentifier, r.adminService)
		if err != nil {
			return environmentDetails{}, err
		}
	}
	envInfo.Targets = targets

	return envInfo, nil
}

func getTargets(ctx context.Context, accountIdentifier, orgIdentifier, projectIdentifier, environmentIdentifier string, adminService adminClient) ([]domain.Target, error) {

	targetInput := services.PageTargetsInput{
		AccountIdentifier:     accountIdentifier,
		OrgIdentifier:         orgIdentifier,
		ProjectIdentifier:     projectIdentifier,
		EnvironmentIdentifier: environmentIdentifier,
		PageNumber:            0,
		PageSize:              100,
	}

	done := false
	targets := []domain.Target{}
	for !done {
		result, err := adminService.PageTargets(ctx, targetInput)
		done = result.Finished
		if err != nil {
			return []domain.Target{}, fmt.Errorf("failed to page targets: %s", err)
		}

		for _, t := range result.Targets {
			targets = append(targets, domain.Target{Target: t})
		}

		targetInput.PageNumber++
	}

	return targets, nil
}

func getAPIKeys(ctx context.Context, accountIdentifier, orgIdentifier, projectIdentifier, environmentIdentifier string, adminService adminClient) ([]string, error) {
	apiKeysInput := services.GetAPIKeysInput{
		AccountIdentifier:     accountIdentifier,
		OrgIdentifier:         orgIdentifier,
		ProjectIdentifier:     projectIdentifier,
		EnvironmentIdentifier: environmentIdentifier,
		PageNumber:            0,
		PageSize:              100,
	}

	done := false
	apiKeys := []string{}
	for !done {
		result, err := adminService.PageAPIKeys(ctx, apiKeysInput)
		done = result.Finished
		if err != nil {
			return []string{}, fmt.Errorf("failed to get api keys: %s", err)
		}

		for _, key := range result.APIKeys {
			if key.Key != nil {
				apiKeys = append(apiKeys, *key.Key)
			}
		}
		apiKeysInput.PageNumber++
	}

	return apiKeys, nil
}

func getEnvironmentInfo(ctx context.Context, apiKey string, clientService services.ClientService) (projectIdentifier, environmentIdentifier, environmentID string, err error) {
	// get bearer token
	result, err := clientService.Authenticate(ctx, apiKey, domain.Target{})
	if err != nil {
		return
	}
	payloadIndex := 1
	payload := strings.Split(result, ".")[payloadIndex]
	payloadData, err := jwt.DecodeSegment(payload)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse token claims for key %s: %s", apiKey, err)
	}

	// extract projectIdentifier, environmentIdentifier, environmentID from token claims
	var claims map[string]interface{}
	if err = json.Unmarshal(payloadData, &claims); err != nil {
		return "", "", "", fmt.Errorf("failed to unmarhal token claims for key %s: %s", apiKey, err)
	}

	var ok bool
	environmentIdentifier, ok = claims["environmentIdentifier"].(string)
	if !ok {
		return "", "", "", fmt.Errorf("environment identifier not present in bearer token")
	}

	environmentID, ok = claims["environment"].(string)
	if !ok {
		return "", "", "", fmt.Errorf("environment id not present in bearer token")
	}

	projectIdentifier, ok = claims["projectIdentifier"].(string)
	if !ok {
		return "", "", "", fmt.Errorf("project identifier not present in bearer token")
	}

	return projectIdentifier, environmentIdentifier, environmentID, nil
}
