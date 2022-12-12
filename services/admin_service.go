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
	r.Header.Add("x-api-key", d.token)
	return d.c.Do(r)
}

// AdminService is a client for interacting with the admin service
type AdminService struct {
	log    log.Logger
	client admingen.ClientWithResponsesInterface
}

// NewAdminService creates an AdminClient
func NewAdminService(l log.Logger, addr string, serviceToken string) (AdminService, error) {
	l = l.With("component", "AdminServiceClient")

	c, err := admingen.NewClientWithResponses(
		addr,
		admingen.WithHTTPClient(doer{c: http.DefaultClient, token: serviceToken}),
	)
	if err != nil {
		return AdminService{}, err
	}

	return AdminService{log: l, client: c}, nil
}

// PageAPIKeysInput contains the paramters required to make a GetEnvironments
// request
type PageAPIKeysInput struct {
	AccountIdentifier     string
	OrgIdentifier         string
	ProjectIdentifier     string
	EnvironmentIdentifier string
	PageNumber            int
	PageSize              int
}

// PageAPIKeysResult contains the parameters required to make a PageAPIKeys
// request
type PageAPIKeysResult struct {
	APIKeys  []admingen.ApiKey
	Finished bool
}

// PageAPIKeys is used to fetch environment info from the
// admin services /admin/apikey endpoint
func (r AdminService) PageAPIKeys(ctx context.Context, input PageAPIKeysInput) (PageAPIKeysResult, error) {
	r.log = r.log.With("method", "PageAPIKeys")

	r.log.Debug("getting api keys", "projectIdentifier", input.ProjectIdentifier, "accountIdentifier", input.AccountIdentifier, "orgIdentifier", input.OrgIdentifier, "environmentIdentifier", input.EnvironmentIdentifier, "pageSize", input.PageSize, "pageNumber", input.PageNumber)
	resp, err := r.client.GetAllAPIKeysWithResponse(ctx, &admingen.GetAllAPIKeysParams{
		AccountIdentifier:     input.AccountIdentifier,
		OrgIdentifier:         input.OrgIdentifier,
		ProjectIdentifier:     input.ProjectIdentifier,
		EnvironmentIdentifier: input.EnvironmentIdentifier,
		PageNumber:            &input.PageNumber,
		PageSize:              &input.PageSize,
	})
	if err != nil {
		return PageAPIKeysResult{Finished: true}, err
	}

	// TODO: Could make this better and add some retry logic in but for
	// now just error out
	if resp.JSON200 == nil {
		return PageAPIKeysResult{Finished: true}, fmt.Errorf("got non 200 response, status: %s, body: %s", resp.Status(), string(resp.Body))
	}

	// If there are no api keys in the response then there are either none
	// to retrieve or we've paged over them all so we're done
	if resp.JSON200.ApiKeys == nil || len(*resp.JSON200.ApiKeys) == 0 {
		return PageAPIKeysResult{Finished: true}, nil
	}

	return PageAPIKeysResult{
		APIKeys:  *resp.JSON200.ApiKeys,
		Finished: false,
	}, nil
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
func (r AdminService) PageTargets(ctx context.Context, input PageTargetsInput) (PageTargetsResult, error) {
	r.log = r.log.With("method", "PageTargets")

	pageNumber := input.PageNumber
	pageSize := input.PageSize

	r.log.Debug("getting targets", "project_identifier", input.ProjectIdentifier, "environment_identifier", input.EnvironmentIdentifier, "pageSize", input.PageSize, "pageNumber", input.PageNumber)
	resp, err := r.client.GetAllTargetsWithResponse(ctx, &admingen.GetAllTargetsParams{
		AccountIdentifier:     input.AccountIdentifier,
		OrgIdentifier:         input.OrgIdentifier,
		ProjectIdentifier:     input.ProjectIdentifier,
		EnvironmentIdentifier: input.EnvironmentIdentifier,
		PageNumber:            &pageNumber,
		PageSize:              &pageSize,
	})
	if err != nil {
		return PageTargetsResult{Finished: true}, err
	}

	// TODO: Could make this better and add some retry logic in but for
	// now just error out
	if resp.JSON200 == nil {
		return PageTargetsResult{Finished: true}, fmt.Errorf("got non 200 response, status: %s, body: %s", resp.Status(), string(resp.Body))
	}

	// If there are no targets in the response then there are either none
	// to retrieve or we've paged over them all so we're done
	if resp.JSON200.Targets != nil && len(*resp.JSON200.Targets) == 0 {
		return PageTargetsResult{Finished: true}, nil
	}

	return PageTargetsResult{Targets: *resp.JSON200.Targets, Finished: false}, nil
}
