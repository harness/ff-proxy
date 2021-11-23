package config

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/services"
	"github.com/wings-software/ff-server/pkg/hash"
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
	client       adminClient
	log          log.Logger
	concurrency  int
	authConfig   map[domain.AuthAPIKey]string
	targetConfig map[domain.TargetKey][]domain.Target
}

// NewRemoteConfig creates a RemoteConfig and retrieves the configuration for
// the given Account, Org and APIKeys from the Feature Flags Service
func NewRemoteConfig(ctx context.Context, accountIdentifer string, orgIdentifier string, apiKeys []string, hasher hash.Hasher, client adminClient, opts ...RemoteOption) RemoteConfig {
	rc := &RemoteConfig{
		client:       client,
		authConfig:   make(map[domain.AuthAPIKey]string),
		targetConfig: make(map[domain.TargetKey][]domain.Target),
	}

	allowedAPIKeys := map[string]struct{}{}
	for _, key := range apiKeys {
		allowedAPIKeys[hasher.Hash(key)] = struct{}{}
	}

	for _, opt := range opts {
		opt(rc)
	}

	if rc.log == nil {
		rc.log = log.NewLogger(os.Stderr, false)
	}

	if rc.concurrency == 0 {
		rc.concurrency = 10
	}
	rc.log = log.With(rc.log, "component", "RemoteConfig", "account_identifier", accountIdentifer, "org_identifier", orgIdentifier)
	rc.load(ctx, accountIdentifer, orgIdentifier, allowedAPIKeys)
	return *rc
}

// TargetConfig returns the Target information that was retrieved from the Feature Flags Service
func (r RemoteConfig) TargetConfig() map[domain.TargetKey][]domain.Target {
	return r.targetConfig
}

// AuthConfig returns the AuthConfig that was retrived from the Feature Flags Service
func (r RemoteConfig) AuthConfig() map[domain.AuthAPIKey]string {
	return r.authConfig
}

// load setups a pipeline to retrieve the Project, Environment and Target informatino
// from the AdminService and then uses the config that was built up by the pipeline
// to build the auth and target config.
//
// The first stage of the pipeline retrieves the project information and then after
// that we fan out to get the Environment and Target config to reduce the time it
// takes to retrieve the config.
func (r *RemoteConfig) load(ctx context.Context, accountIdentifer string, orgIdentifier string, allowedAPIKeys map[string]struct{}) error {
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

	// Read all the config from the pipeline and build up the authConfig and
	// targetConfig maps
	r.authConfig, r.targetConfig = makeConfigs(fanIn(ctx, pipelineResults...))
	return nil
}

func makeConfigs(results <-chan configPipeline) (map[domain.AuthAPIKey]string, map[domain.TargetKey][]domain.Target) {
	authConfig := map[domain.AuthAPIKey]string{}
	targetConfig := map[domain.TargetKey][]domain.Target{}

	for result := range results {
		for _, key := range result.APIKeys {
			authConfig[domain.AuthAPIKey(key)] = result.EnvironmentID
		}

		targetKey := domain.NewTargetKey(result.EnvironmentID)
		targetConfig[targetKey] = append(targetConfig[targetKey], result.Targets...)
	}
	return authConfig, targetConfig
}

// configPipeline is the input and output for each stage of the pipeline that
// retreives config from the FeatureFlags service.
type configPipeline struct {
	AccountIdentifier     string
	OrgIdentifier         string
	AllowedKeys           map[string]struct{}
	ProjectIdentifier     string
	EnvironmentID         string
	EnvironmnetIdentifier string
	APIKeys               []string
	Targets               []domain.Target
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
				r.log.Error("msg", "error paging projects", "err", err)
				return
			}

			for _, project := range result.Projects {
				input.ProjectIdentifier = project.Identifier

				select {
				case <-ctx.Done():
					return
				case out <- input:
				}
			}
			projectInput.PageNumber++
		}
	}()
	return out
}

// addEnvironmentConfig is the stage of the pipeline that adds the EnvironmentID,
// Identifier and APIKeys to the ConfigPipeline
func (r RemoteConfig) addEnvironmentConfig(ctx context.Context, inputs <-chan configPipeline) <-chan configPipeline {
	out := make(chan configPipeline)

	go func() {
		defer close(out)

		for input := range inputs {

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
					r.log.Error("msg", "error paging environments", "err", err)
					continue
				}

				for _, env := range result.Environments {
					// No point continuing if there's no ID, identifier or ApiKeys
					// since we we need these further down the pipeline
					if env.Id == nil || env.Identifier == "" || env.ApiKeys.ApiKeys == nil {
						continue
					}
					input.EnvironmentID = *env.Id
					input.EnvironmnetIdentifier = env.Identifier
					input.APIKeys = []string{}
					for _, key := range *env.ApiKeys.ApiKeys {
						if key.Key != nil {
							input.APIKeys = append(input.APIKeys, *key.Key)
						}
					}

					select {
					case <-ctx.Done():
						return
					case out <- input:
					}
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
			targetInput := services.PageTargetsInput{
				AccountIdentifier:     input.AccountIdentifier,
				OrgIdentifier:         input.OrgIdentifier,
				ProjectIdentifier:     input.ProjectIdentifier,
				EnvironmentIdentifier: input.EnvironmnetIdentifier,
				PageNumber:            0,
				PageSize:              100,
			}

			done := false
			for !done {
				result, err := r.client.PageTargets(ctx, targetInput)
				done = result.Finished
				if err != nil {
					r.log.Error("msg", "error paging targets", "err", err, "input", fmt.Sprintf("%+v", targetInput))
					continue
				}

				targets := []domain.Target{}
				for _, t := range result.Targets {
					targets = append(targets, domain.Target{Target: t})
				}
				input.Targets = targets

				select {
				case <-ctx.Done():
					return
				case out <- input:
				}

				targetInput.PageNumber++
			}
		}
	}()

	return out
}

// filterOnAllowedAPIKeys filters the pipeline by the AllowedKeys. If an input in
// the pipeline only contains APIKeys that don't exist in the AllowedKeys then
// it will not be passed further down the pipeline. If an input has two keys and
// one of them exists in the AllowedKeys then the one that doesn't exist will be
// removed and the input will be passed down the pipeline with just the one APIKey.
func (r RemoteConfig) filterOnAllowedAPIKeys(ctx context.Context, inputs <-chan configPipeline) <-chan configPipeline {
	out := make(chan configPipeline)

	go func() {
		defer close(out)

		for input := range inputs {
			// if no allowed keys skip
			if len(input.AllowedKeys) == 0 || input.AllowedKeys == nil {
				continue
			}

			keys := []string{}

			// filter found api keys - only keep ones specified in AllowedKeys
			for _, key := range input.APIKeys {
				if _, ok := input.AllowedKeys[key]; ok {
					keys = append(keys, key)
				}
			}

			if len(keys) == 0 {
				continue
			}

			input.APIKeys = keys
			out <- input
		}
	}()
	return out
}
