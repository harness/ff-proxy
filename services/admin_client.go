package services

import (
	"context"
	"fmt"
	"net/http"

	admingen "github.com/harness/ff-proxy/gen/admin"
	"github.com/harness/ff-proxy/log"
)

// doer is a simple http client that gets passed to the generated admin client
// and injects the service token into the header before any requests are made
type doer struct {
	c     *http.Client
	token string
}

// Do injects the api-key header into the request
func (d doer) Do(r *http.Request) (*http.Response, error) {
	r.Header.Add("api-key", fmt.Sprintf("Bearer %s", d.token))
	return d.c.Do(r)
}

// AdminClient is a client for interacting with the admin service
type AdminClient struct {
	log    log.Logger
	client admingen.ClientWithResponsesInterface
}

// NewAdminClient creates an AdminClient
func NewAdminClient(l log.Logger, addr string, serviceToken string) (AdminClient, error) {
	l = log.With(l, "component", "AdminClient")

	c, err := admingen.NewClientWithResponses(
		addr,
		admingen.WithHTTPClient(doer{c: http.DefaultClient, token: serviceToken}),
	)
	if err != nil {
		return AdminClient{}, err
	}

	return AdminClient{log: l, client: c}, nil
}

// PageEnvironmentsInput contains the paramters required to make a PageEnvironments
// request
type PageEnvironmentsInput struct {
	AccountIdentifier string
	OrgIdentifier     string
	ProjectIdentifier string
	PageNumber        int
	PageSize          int
}

// PageEnvironmentsResult is the result of a PageEnvironments request
type PageEnvironmentsResult struct {
	Environments []admingen.Environment
	Finished     bool
}

// PageEnvironments is used for synchronously paging over environments by making
// request to the admin services /admin/environments endpoint
func (r AdminClient) PageEnvironments(ctx context.Context, input PageEnvironmentsInput) (PageEnvironmentsResult, error) {
	pageNumber := admingen.PageNumber(input.PageNumber)
	pageSize := admingen.PageSize(input.PageSize)

	r.log.Debug("msg", "GetAllEnvironmentsWithResponse", "projectIdentifier", input.ProjectIdentifier, "pageSize", input.PageSize, "pageNumber", input.PageNumber)
	resp, err := r.client.GetAllEnvironmentsWithResponse(ctx, &admingen.GetAllEnvironmentsParams{
		AccountIdentifier: admingen.AccountQueryParam(input.AccountIdentifier),
		Org:               admingen.OrgQueryParam(input.OrgIdentifier),
		Project:           admingen.ProjectQueryParam(input.ProjectIdentifier),
		PageNumber:        &pageNumber,
		PageSize:          &pageSize,
	})
	if err != nil {
		return PageEnvironmentsResult{Finished: true}, err
	}

	// TODO: Could make this better and add some retry logic in but for
	// now just error out
	if resp.JSON200 == nil {
		return PageEnvironmentsResult{Finished: true}, fmt.Errorf("got non 200 response, status: %s, body: %s", resp.Status(), string(resp.Body))
	}

	// If there are no environments in the response then there are either none
	// to retrieve or we've paged over them all so we're done
	if *resp.JSON200.Data.Environments != nil && len(*resp.JSON200.Data.Environments) == 0 {
		return PageEnvironmentsResult{Finished: true}, nil
	}

	return PageEnvironmentsResult{Environments: *resp.JSON200.Data.Environments, Finished: false}, nil
}

// PageProjectsInput contains the paramters required to make a PageProjects
// request
type PageProjectsInput struct {
	AccountIdentifier string
	OrgIdentifier     string
	PageNumber        int
	PageSize          int
}

// PageProjectsResult is the result of a PageProjects request
type PageProjectsResult struct {
	Projects []admingen.Project
	Finished bool
}

// PageProjects is used for synchronously paging over projects by making requests
// to the admin services /admin/projects endpoint
func (r AdminClient) PageProjects(ctx context.Context, input PageProjectsInput) (PageProjectsResult, error) {
	pageNumber := admingen.PageNumber(input.PageNumber)
	pageSize := admingen.PageSize(input.PageSize)

	r.log.Debug("msg", "GetAllProjectsWithResponse", "pageSize", input.PageSize, "pageNumber", input.PageNumber)
	resp, err := r.client.GetAllProjectsWithResponse(ctx, &admingen.GetAllProjectsParams{
		AccountIdentifier: admingen.AccountQueryParam(input.AccountIdentifier),
		Org:               admingen.OrgQueryParam(input.OrgIdentifier),
		PageNumber:        &pageNumber,
		PageSize:          &pageSize,
	})
	if err != nil {
		return PageProjectsResult{Finished: true}, err
	}

	// TODO: Could make this better and add some retry logic in but for
	// now just error out
	if resp.JSON200 == nil {
		return PageProjectsResult{Finished: true}, fmt.Errorf("got non 200 response, status: %s, body: %s", resp.Status(), string(resp.Body))
	}

	// If there are no projects in the response then there are either none
	// to retrieve or we've paged over them all so we're done
	if *resp.JSON200.Data.Projects != nil && len(*resp.JSON200.Data.Projects) == 0 {
		return PageProjectsResult{Finished: true}, nil
	}

	return PageProjectsResult{Projects: *resp.JSON200.Data.Projects, Finished: false}, nil
}

// PageTargetsInput contains the paramters required to make a PageTargets
// request
type PageTargetsInput struct {
	AccountIdentifier     string
	OrgIdentifier         string
	ProjectIdentifier     string
	EnvironmentIdentifier string
	PageNumber            int
	PageSize              int
}

// PageTargetsResult contains the paramters required to make a PageTargets
// request
type PageTargetsResult struct {
	Targets  []admingen.Target
	Finished bool
}

// PageTargets is used for synchronously paging over projects by making requests
// to the admin services /admin/targets endpoint.
func (r AdminClient) PageTargets(ctx context.Context, input PageTargetsInput) (PageTargetsResult, error) {
	pageNumber := admingen.PageNumber(input.PageNumber)
	pageSize := admingen.PageSize(input.PageSize)

	r.log.Debug("msg", "GetAllTargetsWithResponse", "project_identifier", input.ProjectIdentifier, "environment_identifier", input.EnvironmentIdentifier, "pageSize", input.PageSize, "pageNumber", input.PageNumber)
	resp, err := r.client.GetAllTargetsWithResponse(ctx, &admingen.GetAllTargetsParams{
		AccountIdentifier: admingen.AccountQueryParam(input.AccountIdentifier),
		Org:               admingen.OrgQueryParam(input.OrgIdentifier),
		Project:           admingen.ProjectQueryParam(input.ProjectIdentifier),
		Environment:       admingen.EnvironmentQueryParam(input.EnvironmentIdentifier),
		PageNumber:        &pageNumber,
		PageSize:          &pageSize,
	})
	if err != nil {
		return PageTargetsResult{Finished: true}, err
	}

	// TODO: Could make this better and add some retry logic in but for
	// now just error out
	if resp.JSON200 == nil {
		return PageTargetsResult{Finished: true}, fmt.Errorf("got non 200 response, status: %s, body: %s", resp.Status(), string(resp.Body))
	}

	// If there are no projects in the response then there are either none
	// to retrieve or we've paged over them all so we're done
	if *resp.JSON200.Targets != nil && len(*resp.JSON200.Targets) == 0 {
		return PageTargetsResult{Finished: true}, nil
	}

	return PageTargetsResult{Targets: *resp.JSON200.Targets, Finished: false}, nil
}