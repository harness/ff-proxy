package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/hash"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/services"
)

// RemoteOption is type for passing optional parameters to a RemoteConfig
type RemoteOption func(r *RemoteConfig)

// WithConcurrency sets the maximum amount of concurrent requests that the
// RemoteConfig can make. It's default value is 10.
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

type adminClient interface {
	PageProjects(ctx context.Context, input services.PageProjectsInput) (services.PageProjectsResult, error)
	PageTargets(ctx context.Context, input services.PageTargetsInput) (services.PageTargetsResult, error)
	PageEnvironments(ctx context.Context, input services.PageEnvironmentsInput) (services.PageEnvironmentsResult, error)
}

// RemoteConfig is a type that retrieves config from the Feature Flags Service
type RemoteConfig struct {
	client           adminClient
	log              log.Logger
	concurrency      int
	authConfig       map[domain.AuthAPIKey]string
	targetConfig     map[domain.TargetKey][]domain.Target
	accountIdentifer string
	orgIdentifier    string
	allowedAPIKeys   map[string]struct{}
	// we store project and environment info after the initial load so that the
	// PollTargets functioncan use it and not have to make GetProjects and
	// GetEnvironments requests every time
	projEnvInfo map[string]configPipeline
}

// NewRemoteConfig creates a RemoteConfig and retrieves the configuration for
// the given Account, Org and APIKeys from the Feature Flags Service
func NewRemoteConfig(ctx context.Context, accountIdentifer string, orgIdentifier string, apiKeys []string, hasher hash.Hasher, client adminClient, opts ...RemoteOption) (RemoteConfig, error) {
	allowedAPIKeys := map[string]struct{}{}
	for _, key := range apiKeys {
		allowedAPIKeys[hasher.Hash(key)] = struct{}{}
	}

	rc := &RemoteConfig{
		client:           client,
		authConfig:       make(map[domain.AuthAPIKey]string),
		targetConfig:     make(map[domain.TargetKey][]domain.Target),
		accountIdentifer: accountIdentifer,
		orgIdentifier:    orgIdentifier,
		allowedAPIKeys:   allowedAPIKeys,
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
	rc.log = rc.log.With("component", "RemoteConfig", "account_identifier", accountIdentifer, "org_identifier", orgIdentifier)

	config := makeConfigs(orDone(ctx, rc.load(ctx, accountIdentifer, orgIdentifier, allowedAPIKeys)))
	rc.authConfig = config.auth
	rc.targetConfig = config.targets
	rc.projEnvInfo = config.projectEnvironments

	return *rc, config.err
}

// TargetConfig returns the Target information that was retrieved from the Feature Flags Service
func (r RemoteConfig) TargetConfig() map[domain.TargetKey][]domain.Target {
	return r.targetConfig
}

// AuthConfig returns the AuthConfig that was retrived from the Feature Flags Service
func (r RemoteConfig) AuthConfig() map[domain.AuthAPIKey]string {
	return r.authConfig
}

type projEnvInfo struct {
	EnvironmentIdentifier string
	ProjectIdentifier     string
}

// ProjectEnvironmentInfo returns a map of environmentIDs to structs containing
// the Environment and Project Identifiers
func (r RemoteConfig) ProjectEnvironmentInfo() map[string]projEnvInfo {
	m := map[string]projEnvInfo{}

	for envID, cp := range r.projEnvInfo {
		m[envID] = projEnvInfo{
			EnvironmentIdentifier: cp.EnvironmentIdentifier,
			ProjectIdentifier:     cp.ProjectIdentifier,
		}
	}
	return m
}

// PollTargets polls feature flags to fetch the latest targets at a rate determined
// by the ticker and returns the latest targets on a channel.
func (r RemoteConfig) PollTargets(ctx context.Context, ticker <-chan time.Time) <-chan map[domain.TargetKey][]domain.Target {
	out := make(chan map[domain.TargetKey][]domain.Target)

	go func() {
		defer func() {
			close(out)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker:
				r.log.Debug("polling for new targets")
				config := makeConfigs(r.addTargetConfig(ctx, configPipelineGenerator(ctx, r.projEnvInfo)))
				if config.err != nil {
					r.log.Error("failed to fetch targets", "err", config.err)
				}

				select {
				case <-ctx.Done():
					return
				case out <- config.targets:
				}
			}
		}
	}()

	return out
}

// load setups a pipeline to retrieve the Project, Environment and Target information
// from the AdminService and then uses the config that was built up by the pipeline
// to build the auth and target config.
//
// The first stage of the pipeline retrieves the project information and then after
// that we fan out to get the Environment and Target config to reduce the time it
// takes to retrieve the config.
func (r *RemoteConfig) load(ctx context.Context, accountIdentifer string, orgIdentifier string, allowedAPIKeys map[string]struct{}) <-chan configPipeline {
	pipelineInput := configPipeline{
		AccountIdentifier: accountIdentifer,
		OrgIdentifier:     orgIdentifier,
		AllowedKeys:       allowedAPIKeys,
	}
	stage1 := r.addProjectConfig(ctx, pipelineInput)

	// Fan-out so we've multiple addEnvironmentProcesses reading from
	// the output from stage1 of our pipeline
	stage2 := make([]<-chan configPipeline, r.concurrency)
	for i := 0; i < r.concurrency; i++ {
		stage2[i] = r.addEnvironmentConfig(ctx, stage1)
	}

	// Fan-in the channels in the second stage of the pipeline so that the next
	// stage has a channel to read from
	stage2Result := r.filterOnAllowedAPIKeys(ctx, fanIn(ctx, stage2...))

	// Fan-out again so we've multiple addTargetConfig processes reading
	// the output from stage2 of our pipline
	pipelineResults := make([]<-chan configPipeline, r.concurrency)
	for i := 0; i < r.concurrency; i++ {
		pipelineResults[i] = r.addTargetConfig(ctx, stage2Result)
	}

	return fanIn(ctx, pipelineResults...)
}

// orDone is a helper that encapsulates the logic for reading from a channel
// whilst waiting for a cancellation.
func orDone(ctx context.Context, c <-chan configPipeline) <-chan configPipeline {
	out := make(chan configPipeline)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case cp, ok := <-c:
				if !ok {
					return
				}

				select {
				case <-ctx.Done():
				case out <- cp:
				}
			}
		}
	}()

	return out
}

// configPipelineGenerator is a function for creating and sending a map of
// configPipelines down a channel
func configPipelineGenerator(ctx context.Context, m map[string]configPipeline) <-chan configPipeline {
	out := make(chan configPipeline)

	go func() {
		defer close(out)

		for _, cp := range m {
			select {
			case <-ctx.Done():
				return
			case out <- cp:
			}
		}
	}()

	return out
}

type configResult struct {
	// auth contains the auth config fetched from FeatureFlags
	auth map[domain.AuthAPIKey]string
	// targets contains the targets fetched from FeatureFlags
	targets map[domain.TargetKey][]domain.Target
	// projectEnvironments is a map of environmentIDs to
	projectEnvironments map[string]configPipeline
	// errs contains any errors encountered retreiving the config
	err error
}

func makeConfigs(results <-chan configPipeline) configResult {
	authConfig := map[domain.AuthAPIKey]string{}
	targetConfig := map[domain.TargetKey][]domain.Target{}
	projEnvInfo := map[string]configPipeline{}

	var err error = nil

	for result := range results {
		if result.Err != nil {
			if err == nil {
				err = result.Err
				continue
			}
			err = fmt.Errorf("%w: %s", err, result.Err)
		}

		for _, key := range result.APIKeys {
			authConfig[domain.AuthAPIKey(key)] = result.EnvironmentID
		}

		targetKey := domain.NewTargetKey(result.EnvironmentID)
		targetConfig[targetKey] = append(targetConfig[targetKey], result.Targets...)

		projEnvInfo[result.EnvironmentID] = configPipeline{
			AccountIdentifier:     result.AccountIdentifier,
			OrgIdentifier:         result.OrgIdentifier,
			ProjectIdentifier:     result.ProjectIdentifier,
			EnvironmentID:         result.EnvironmentID,
			EnvironmentIdentifier: result.EnvironmentIdentifier,
		}
	}

	return configResult{
		auth:                authConfig,
		targets:             targetConfig,
		projectEnvironments: projEnvInfo,
		err:                 err,
	}
}

// configPipeline is the input and output for each stage of the pipeline that
// retreives config from the FeatureFlags service.
type configPipeline struct {
	AccountIdentifier     string
	OrgIdentifier         string
	AllowedKeys           map[string]struct{}
	ProjectIdentifier     string
	EnvironmentID         string
	EnvironmentIdentifier string
	APIKeys               []string
	Targets               []domain.Target
	Err                   error
}

// fanIn joins multiple ConfigPipeline channels into a single channel
func fanIn(ctx context.Context, channels ...<-chan configPipeline) <-chan configPipeline {
	wg := sync.WaitGroup{}
	stream := make(chan configPipeline)

	multiplex := func(c <-chan configPipeline) {
		defer wg.Done()
		for i := range c {
			select {
			case <-ctx.Done():
				return
			case stream <- i:
			}
		}
	}

	// Select from all the channels
	wg.Add(len(channels))
	for _, c := range channels {
		go multiplex(c)
	}

	// Wait for all the reads to complete
	go func() {
		wg.Wait()
		close(stream)
	}()

	return stream
}

// addProjectConfig is the stage of the pipeline that adds the ProjectIdentifier
// to the ConfigPipeline
func (r RemoteConfig) addProjectConfig(ctx context.Context, input configPipeline) <-chan configPipeline {
	out := make(chan configPipeline)
	go func() {
		defer close(out)

		projectInput := services.PageProjectsInput{
			AccountIdentifier: input.AccountIdentifier,
			OrgIdentifier:     input.OrgIdentifier,
			PageNumber:        0,
			PageSize:          100,
		}

		done := false
		for !done {
			result, err := r.client.PageProjects(ctx, projectInput)
			done = result.Finished
			if err != nil {
				input.Err = fmt.Errorf("failed to page projects: %s", err)
				sendOrDone(ctx, out, input)
				return
			}

			for _, project := range result.Projects {
				input.ProjectIdentifier = project.Identifier
				sendOrDone(ctx, out, input)
			}
			projectInput.PageNumber++
		}
	}()
	return out
}

// sendOrDone is a helper function that blocks until we've written to the passed
// channel or the context has been cancelled
func sendOrDone(ctx context.Context, ch chan<- configPipeline, value configPipeline) {
	select {
	case <-ctx.Done():
		return
	case ch <- value:
	}
}

// addEnvironmentConfig is the stage of the pipeline that adds the EnvironmentID,
// Identifier and APIKeys to the ConfigPipeline
func (r RemoteConfig) addEnvironmentConfig(ctx context.Context, inputs <-chan configPipeline) <-chan configPipeline {
	out := make(chan configPipeline)

	go func() {
		defer close(out)

		for input := range inputs {
			// If an earlier stage in the pipeline has failed there's no point
			// trying to execute this stage. We still pass the event on so the
			// caller can get the original error
			if input.Err != nil {
				sendOrDone(ctx, out, input)
				continue
			}

			environmentInput := services.PageEnvironmentsInput{
				AccountIdentifier: input.AccountIdentifier,
				OrgIdentifier:     input.OrgIdentifier,
				ProjectIdentifier: input.ProjectIdentifier,
				PageNumber:        0,
				PageSize:          100,
			}

			done := false
			for !done {
				result, err := r.client.PageEnvironments(ctx, environmentInput)
				done = result.Finished
				if err != nil {
					input.Err = fmt.Errorf("failed to page envrionments: %s", err)
					sendOrDone(ctx, out, input)
					continue
				}

				for _, env := range result.Environments {
					// No point continuing if there's no ID, identifier or ApiKeys
					// since we we need these further down the pipeline
					if env.Id == nil || env.Identifier == "" || env.ApiKeys.ApiKeys == nil {
						continue
					}
					input.EnvironmentID = *env.Id
					input.EnvironmentIdentifier = env.Identifier
					input.APIKeys = []string{}
					for _, key := range *env.ApiKeys.ApiKeys {
						if key.Key != nil {
							input.APIKeys = append(input.APIKeys, *key.Key)
						}
					}

					sendOrDone(ctx, out, input)
				}

				environmentInput.PageNumber++
			}
		}
	}()

	return out
}

// addTargetConfig is the stage of the pipeline that adds Targets to the ConfigPipeline
func (r RemoteConfig) addTargetConfig(ctx context.Context, inputs <-chan configPipeline) <-chan configPipeline {
	out := make(chan configPipeline)

	go func() {
		defer close(out)

		for input := range inputs {
			// If an earlier stage in the pipeline has failed there's no point
			// trying to execute this stage. We still pass the event on so the
			// caller can get the original error
			if input.Err != nil {
				sendOrDone(ctx, out, input)
				continue
			}

			targetInput := services.PageTargetsInput{
				AccountIdentifier:     input.AccountIdentifier,
				OrgIdentifier:         input.OrgIdentifier,
				ProjectIdentifier:     input.ProjectIdentifier,
				EnvironmentIdentifier: input.EnvironmentIdentifier,
				PageNumber:            0,
				PageSize:              100,
			}

			done := false
			for !done {
				result, err := r.client.PageTargets(ctx, targetInput)
				done = result.Finished
				if err != nil {
					input.Err = fmt.Errorf("failed to page targets: %s", err)
					sendOrDone(ctx, out, input)
					continue
				}

				targets := []domain.Target{}
				for _, t := range result.Targets {
					targets = append(targets, domain.Target{Target: t})
				}
				input.Targets = targets

				sendOrDone(ctx, out, input)
				targetInput.PageNumber++
			}
		}
	}()

	return out
}

// filterOnAllowedAPIKeys filters the pipeline by the AllowedKeys. If an input in
// the pipeline only contains APIKeys that don't exist in the AllowedKeys then
// it will not be passed further down the pipeline.
func (r RemoteConfig) filterOnAllowedAPIKeys(ctx context.Context, inputs <-chan configPipeline) <-chan configPipeline {
	out := make(chan configPipeline)

	go func() {
		defer close(out)

		for input := range inputs {
			// If an earlier stage in the pipeline has failed there's no point
			// trying to execute this stage. We still pass the event on so the
			// caller can get the original error
			if input.Err != nil {
				sendOrDone(ctx, out, input)
			}
			// if no allowed keys skip
			if len(input.AllowedKeys) == 0 || input.AllowedKeys == nil {
				continue
			}

			keys := []string{}

			// filter found api keys - only keep if any specified in AllowedKeys
			for _, key := range input.APIKeys {
				if _, ok := input.AllowedKeys[key]; ok {
					keys = input.APIKeys
					break
				}
			}

			if len(keys) == 0 {
				continue
			}

			input.APIKeys = keys
			sendOrDone(ctx, out, input)
		}
	}()
	return out
}
