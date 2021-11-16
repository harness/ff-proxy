package config

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/harness/ff-proxy/domain"
	admingen "github.com/harness/ff-proxy/gen/admin"
	"github.com/harness/ff-proxy/log"
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

// RemoteConfig is a type that can retrieve config from the adminService for a
// given account and org
type RemoteConfig struct {
	accountIdentifer string
	orgIdentifier    string
	client           admingen.ClientWithResponsesInterface
	log              log.Logger
	concurrency      int
}

// NewRemoteConfig creates a RemoteConfig that can be used to retrieve config from the adminService for the passed account and org
func NewRemoteConfig(accountIdentifer string, orgIdentifier string, client admingen.ClientWithResponsesInterface, opts ...RemoteOption) RemoteConfig {
	rc := RemoteConfig{
		accountIdentifer: accountIdentifer,
		orgIdentifier:    orgIdentifier,
		client:           client,
	}

	for _, opt := range opts {
		opt(&rc)
	}

	if rc.log == nil {
		rc.log = log.NewLogger(os.Stderr, false)
	}

	if rc.concurrency == 0 {
		rc.concurrency = 10
	}
	rc.log = log.With(rc.log, "component", "RemoteConfig", "account_identifier", accountIdentifer, "org_identifier", orgIdentifier)
	return rc
}

// AuthConfig retrieves a map of APIKeys to EnvironmentIDs from the ff-admin service.
// To do this it first has to get all of the projects for the account and org and then
// it can get all of the environments for each projects which brings back the enivronmentID
// and all of the APIKeys in that environment.
func (r RemoteConfig) AuthConfig(ctx context.Context) (map[domain.AuthAPIKey]string, error) {
	identifiers, err := r.getProjectIdentifiers(ctx)
	if err != nil {
		r.log.Error("msg", "AuthConfig failed to get projects", "err", err)
		return map[domain.AuthAPIKey]string{}, err
	}

	type authInfo struct {
		Key domain.AuthAPIKey
		ID  string
	}

	wg := sync.WaitGroup{}
	sem := make(chan struct{}, r.concurrency)
	c := make(chan authInfo, r.concurrency)

	for _, identifier := range identifiers {
		wg.Add(1)
		sem <- struct{}{}

		go func(ident string) {
			defer func() {
				wg.Done()
				<-sem
			}()

			environment, err := r.getEnvironment(ctx, ident)
			if err != nil {
				r.log.Error("msg", "AuthConfig failed to get environments", "err", err)
				return
			}

			for _, e := range environment {
				for _, apiKey := range e.APIKeys {
					c <- authInfo{Key: domain.AuthAPIKey(apiKey), ID: e.ID}
				}
			}
		}(identifier)
	}

	// Make sure all goroutines retreiving environment information have finished
	// before closing so we can't accidentally write to a closed channel
	go func() {
		wg.Wait()
		close(sem)
		close(c)
	}()

	authConfig := make(map[domain.AuthAPIKey]string)
	for f := range c {
		authConfig[f.Key] = f.ID
	}
	return authConfig, nil
}

// getProjectIdentifiers pages over all of the projects for the given account and
// org and returns a slicce containing their identifiers
func (r RemoteConfig) getProjectIdentifiers(ctx context.Context) ([]string, error) {
	pageNumber := 0
	pageSize := 100
	identifiers := []string{}

	done := false
	for !done {
		result, err := r.pageProjects(ctx, pageNumber, pageSize)
		done = result.finished
		if err != nil {
			r.log.Error("msg", "error paging projects", "err", err)
			return identifiers, err
		}

		for _, project := range result.projects {
			identifiers = append(identifiers, project.Identifier)
		}
		pageNumber++
	}
	return identifiers, nil
}

// environment is a type containing an environmentID and the APIkeys that it has
type environment struct {
	ID      string
	APIKeys []string
}

// getEnvironments pages over all of the environments for a project to get all
// the API keys it has.
func (r RemoteConfig) getEnvironment(ctx context.Context, projectIdentifer string) ([]environment, error) {
	pageNumber := 0
	pageSize := 100
	environments := []environment{}

	done := false
	for !done {
		result, err := r.pageEnvironments(ctx, projectIdentifer, pageSize, pageNumber)
		done = result.finished
		if err != nil {
			r.log.Error("msg", "error paging environments", "err", err)
			return environments, err
		}

		for _, env := range result.environments {
			e := environment{ID: *env.Id}
			for _, key := range *env.ApiKeys.ApiKeys {
				e.APIKeys = append(e.APIKeys, *key.Key)
			}
			environments = append(environments, e)
		}
		pageNumber++
	}

	return environments, nil

}

type envPageResult struct {
	environments []admingen.Environment
	finished     bool
}

// pageEnvironments can be used to page over the environments for a project
func (r RemoteConfig) pageEnvironments(ctx context.Context, projectIdentifier string, pageSize int, pageNumber int) (envPageResult, error) {
	pn := admingen.PageNumber(pageNumber)
	ps := admingen.PageSize(pageSize)

	r.log.Debug("msg", "GetAllEnvironmentsWithResponse", "projectIdentifier", projectIdentifier, "pageSize", pageSize, "pageNumber", pageNumber)
	resp, err := r.client.GetAllEnvironmentsWithResponse(ctx, &admingen.GetAllEnvironmentsParams{
		AccountIdentifier: admingen.AccountQueryParam(r.accountIdentifer),
		Org:               admingen.OrgQueryParam(r.orgIdentifier),
		Project:           admingen.ProjectQueryParam(projectIdentifier),
		PageNumber:        &pn,
		PageSize:          &ps,
	})
	if err != nil {
		return envPageResult{finished: true}, err
	}

	// TODO: Could make this better and add some retry logic in but for
	// now just error out
	if resp.JSON200 == nil {
		return envPageResult{finished: true}, fmt.Errorf("got non 200 response, status: %s, body: %s", resp.Status(), string(resp.Body))
	}

	// If there are no environments in the response then there are either none
	// to retrieve or we've paged over them all so we're done
	if *resp.JSON200.Data.Environments != nil && len(*resp.JSON200.Data.Environments) == 0 {
		return envPageResult{finished: true}, nil
	}

	return envPageResult{environments: *resp.JSON200.Data.Environments, finished: false}, nil
}

type projPageResult struct {
	projects []admingen.Project
	finished bool
}

// pageProjects pages over all the projects for a account
func (r RemoteConfig) pageProjects(ctx context.Context, pageNumber int, pageSize int) (projPageResult, error) {
	pn := admingen.PageNumber(pageNumber)
	ps := admingen.PageSize(pageSize)

	r.log.Debug("msg", "GetAllProjectsWithResponse", "pageSize", pageSize, "pageNumber", pageNumber)
	resp, err := r.client.GetAllProjectsWithResponse(ctx, &admingen.GetAllProjectsParams{
		AccountIdentifier: admingen.AccountQueryParam(r.accountIdentifer),
		Org:               admingen.OrgQueryParam(r.orgIdentifier),
		PageNumber:        &pn,
		PageSize:          &ps,
	})
	if err != nil {
		return projPageResult{finished: true}, err
	}

	// TODO: Could make this better and add some retry logic in but for
	// now just error out
	if resp.JSON200 == nil {
		return projPageResult{finished: true}, fmt.Errorf("got non 200 response, status: %s, body: %s", resp.Status(), string(resp.Body))
	}

	// If there are no projects in the response then there are either none
	// to retrieve or we've paged over them all so we're done
	if *resp.JSON200.Data.Projects != nil && len(*resp.JSON200.Data.Projects) == 0 {
		return projPageResult{finished: true}, nil
	}

	return projPageResult{projects: *resp.JSON200.Data.Projects, finished: false}, nil
}
