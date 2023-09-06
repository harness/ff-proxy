package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/harness/ff-proxy/v2/services"
)

type adminService interface {
	PageTargets(ctx context.Context, input services.PageTargetsInput) (services.PageTargetsResult, error)
	PageAPIKeys(ctx context.Context, input services.PageAPIKeysInput) (services.PageAPIKeysResult, error)
}

type clientService interface {
	Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error)
}

// RemoteOption is type for passing optional parameters to a RemoteConfig
type RemoteOption func(r *RemoteConfig)

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
	adminService      adminService
	clientService     clientService
	log               log.Logger
	accountIdentifier string
	orgIdentifier     string
	projEnvInfo       map[string]EnvironmentDetails
	fetchTargets      bool
}

// TargetConfig returns the Target information that was retrieved from the Feature Flags Service
func (r RemoteConfig) TargetConfig() map[domain.TargetKey]interface{} {
	targetConfig := make(map[domain.TargetKey]interface{})

	for _, env := range r.projEnvInfo {
		targetKey := domain.NewTargetsKey(env.EnvironmentID)
		targetConfig[targetKey] = env.Targets

		for _, t := range env.Targets {
			k := domain.NewTargetKey(env.EnvironmentID, t.Identifier)
			targetConfig[k] = t
		}
	}
	return targetConfig
}

// AuthConfig returns the AuthConfig that was retrieved from the Feature Flags Service
func (r RemoteConfig) AuthConfig() map[domain.AuthAPIKey]string {
	authConfig := make(map[domain.AuthAPIKey]string)
	for _, env := range r.projEnvInfo {
		for _, hashedKey := range env.HashedAPIKeys {
			authConfig[domain.NewAuthAPIKey(hashedKey)] = env.EnvironmentID
		}
	}
	return authConfig
}

// Tokens returns the map of environment ids to auth tokens that was retrieved from the Feature Flags Service
func (r RemoteConfig) Tokens() map[string]string {
	tokens := make(map[string]string)
	for _, env := range r.projEnvInfo {
		tokens[env.EnvironmentID] = env.Token
	}
	return tokens
}

// EnvInfo returns the EnvironmentDetails that was retrieved from the Feature Flags Service
func (r RemoteConfig) EnvInfo() map[string]EnvironmentDetails {
	return r.projEnvInfo
}

// NewRemoteConfig creates a RemoteConfig and retrieves the configuration for
// the given Account, Org and APIKeys from the Feature Flags Service
func NewRemoteConfig(ctx context.Context, accountIdentifier string, orgIdentifier string, apiKeys []string, adminService adminService, clientService clientService, opts ...RemoteOption) (RemoteConfig, error) {

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

	rc.log = rc.log.With("component", "RemoteConfig", "account_identifier", accountIdentifier, "org_identifier", orgIdentifier)

	envInfos := map[string]EnvironmentDetails{}

	for _, key := range apiKeys {
		envConfig, err := rc.getConfigForKey(ctx, key)
		if err != nil {
			rc.log.Error("couldn't fetch info for key, skipping", "api key", key, "err", err)
			continue
		}
		// warn if data has already been set for this environment - this means a user has added 2 keys for the same env
		if _, ok := envInfos[envConfig.EnvironmentID]; ok {
			rc.log.Warn("environment already configured, have you added multiple keys for the same environment?", "environmentID", envConfig.EnvironmentID, "environment identifier", envConfig.EnvironmentIdentifier, "projectID", envConfig.ProjectIdentifier)
		}
		envInfos[envConfig.EnvironmentID] = envConfig
	}

	rc.projEnvInfo = envInfos

	return *rc, nil
}

// EnvironmentDetails contains details about a configured environment
type EnvironmentDetails struct {
	EnvironmentIdentifier string
	EnvironmentID         string
	ProjectIdentifier     string
	HashedAPIKeys         []string
	APIKey                string
	Token                 string
	Targets               []domain.Target
}

func (r RemoteConfig) getConfigForKey(ctx context.Context, apiKey string) (EnvironmentDetails, error) {
	// authenticate key and get env/project identifiers
	projectIdentifier, environmentIdentifier, environmentID, token, err := getEnvironmentInfo(ctx, apiKey, r.clientService)
	if err != nil {
		return EnvironmentDetails{}, fmt.Errorf("failed to fetch environment details for key %s: %s", apiKey, err)
	}
	envInfo := EnvironmentDetails{
		EnvironmentIdentifier: environmentIdentifier,
		EnvironmentID:         environmentID,
		ProjectIdentifier:     projectIdentifier,
		APIKey:                apiKey,
		Token:                 token,
		HashedAPIKeys:         nil,
		Targets:               nil,
	}

	// get hashed api keys for environment
	apiKeys, err := getAPIKeys(ctx, r.accountIdentifier, r.orgIdentifier, projectIdentifier, environmentIdentifier, r.adminService)
	if err != nil {
		return EnvironmentDetails{}, err
	}
	envInfo.HashedAPIKeys = apiKeys

	// get targets for environment
	var targets []domain.Target
	if r.fetchTargets {
		targets, err = GetTargets(ctx, r.accountIdentifier, r.orgIdentifier, projectIdentifier, environmentIdentifier, r.adminService)
		if err != nil {
			return EnvironmentDetails{}, err
		}
	}
	envInfo.Targets = targets

	return envInfo, nil
}

// GetTargets retrieves all targets for a given environment
func GetTargets(ctx context.Context, accountIdentifier, orgIdentifier, projectIdentifier, environmentIdentifier string, adminService adminService) ([]domain.Target, error) {

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
			return []domain.Target{}, fmt.Errorf("failed to get targets: %s", err)
		}

		for _, t := range result.Targets {
			targets = append(targets, domain.Target{Target: t})
		}

		targetInput.PageNumber++
	}

	return targets, nil
}

// getAPIKeys retrieves the hashed api keys for an environment
func getAPIKeys(ctx context.Context, accountIdentifier, orgIdentifier, projectIdentifier, environmentIdentifier string, adminService adminService) ([]string, error) {
	apiKeysInput := services.PageAPIKeysInput{
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

// getEnvironmentInfo authenticates an api key and retrieves the project identifier, environment identifier and environment ID from it
func getEnvironmentInfo(ctx context.Context, apiKey string, clientService clientService) (projectIdentifier, environmentIdentifier, environmentID, token string, err error) {
	// get bearer token
	result, err := clientService.Authenticate(ctx, apiKey, domain.Target{})
	if err != nil {
		return "", "", "", "", fmt.Errorf("error sending client authentication request: %s", err)
	}

	// get payload data
	payloadIndex := 1
	if len(strings.Split(result, ".")) < 2 {
		return "", "", "", "", fmt.Errorf("invalid jwt received %s", result)
	}
	payload := strings.Split(result, ".")[payloadIndex]
	payloadData, err := jwt.DecodeSegment(payload)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to parse token claims for key %s: %s", apiKey, err)
	}

	// extract projectIdentifier, environmentIdentifier, environmentID from token claims
	var claims map[string]interface{}
	if err = json.Unmarshal(payloadData, &claims); err != nil {
		return "", "", "", "", fmt.Errorf("failed to unmarshal token claims for key %s: %s", apiKey, err)
	}

	var ok bool
	environmentIdentifier, ok = claims["environmentIdentifier"].(string)
	if !ok {
		return "", "", "", "", fmt.Errorf("environment identifier not present in bearer token")
	}

	environmentID, ok = claims["environment"].(string)
	if !ok {
		return "", "", "", "", fmt.Errorf("environment id not present in bearer token")
	}

	projectIdentifier, ok = claims["projectIdentifier"].(string)
	if !ok {
		return "", "", "", "", fmt.Errorf("project identifier not present in bearer token")
	}

	return projectIdentifier, environmentIdentifier, environmentID, result, nil
}
