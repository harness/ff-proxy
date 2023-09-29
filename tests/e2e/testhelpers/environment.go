package testhelpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/harness/ff-proxy/v2/gen/admin"

	"github.com/avast/retry-go"
)

// PlatformEnvironment ...
type PlatformEnvironment struct {
	Name              string `json:"name"`
	Identifier        string `json:"identifier"`
	ProjectIdentifier string `json:"projectIdentifier"`
	OrgIdentifier     string `json:"orgIdentifier"`
	Type              string `json:"type"`
}

// CreateEnvironment ...
func CreateEnvironment(projectIdentifier string, environment, environmentName string) (*http.Response, error) {
	return CreateEnvironmentRemote(projectIdentifier, environment, environmentName)
}

// CreateEnvironmentRemote ...
func CreateEnvironmentRemote(projectIdentifier string, environment, environmentName string) (*http.Response, error) {
	client := DefaultClient()
	url := fmt.Sprintf("%s/environments?accountId=%s", GetPlatformBaseURL(), GetDefaultAccount())
	body := PlatformEnvironment{
		Name:              environmentName,
		OrgIdentifier:     GetDefaultOrg(),
		ProjectIdentifier: projectIdentifier,
		Identifier:        environment,
		Type:              "PreProduction",
	}
	s, err := json.Marshal(body)
	if err != nil {
		fmt.Println(err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(s))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	AddAuthToken(context.Background(), req)
	res, err := client.Client.Do(req)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// ensure environment is created within cf
	err = retry.Do(
		func() error {
			environmentResponse, err := GetEnvironment(projectIdentifier, environment)
			if err != nil || environmentResponse.StatusCode() != http.StatusOK {
				return errors.New("environment not found")
			}
			return nil
		},
		retry.Attempts(5), retry.Delay(500*time.Millisecond),
	)

	if err != nil {
		log.Error(err)
		return nil, err
	}

	return res, err
}

// GetEnvironment ...
func GetEnvironment(projectIdentifier string, environment string) (*admin.GetEnvironmentResponse, error) {
	client := DefaultClient()
	pqp := admin.ProjectQueryParam(projectIdentifier)
	response, err := client.GetEnvironment(context.Background(), admin.Identifier(environment), &admin.GetEnvironmentParams{
		ProjectIdentifier: pqp,
		AccountIdentifier: admin.AccountQueryParam(GetDefaultAccount()),
		OrgIdentifier:     admin.OrgQueryParam(GetDefaultOrg()),
	}, AddAuthToken)
	if err != nil {
		return nil, err
	}

	return admin.ParseGetEnvironmentResponse(response)
}
