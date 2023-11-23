package testhelpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	log "github.com/sirupsen/logrus"

	admin "github.com/harness/ff-proxy/v2/gen/admin"

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
func CreateEnvironment(org string, projectIdentifier string, environment, environmentName string) (*http.Response, string, error) {
	return CreateEnvironmentRemote(org, projectIdentifier, environment, environmentName)
}

// CreateEnvironmentRemote ...
func CreateEnvironmentRemote(org string, projectIdentifier string, environment, environmentName string) (*http.Response, string, error) {
	client := DefaultClient()
	url := fmt.Sprintf("%s/environments?accountId=%s", GetPlatformBaseURL(), GetDefaultAccount())
	body := PlatformEnvironment{
		Name:              environmentName,
		OrgIdentifier:     org,
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
		return nil, "", err
	}
	req.Header.Set("content-type", "application/json")
	AddAuthToken(context.Background(), req)
	res, err := client.Client.Do(req)
	if err != nil {
		log.Error(err)
		return nil, "", err
	}

	var id string

	// ensure environment is created within cf
	err = retry.Do(
		func() error {
			environmentResponse, err := GetEnvironment(org, projectIdentifier, environment)
			if err != nil || environmentResponse.StatusCode() != http.StatusOK {
				return errors.New("environment not found")
			}

			if environmentResponse.JSON200 != nil {
				if environmentResponse.JSON200.Data != nil {
					id = domain.SafePtrDereference(environmentResponse.JSON200.Data.Id)
				}

			}

			return nil
		},
		retry.Attempts(5), retry.Delay(500*time.Millisecond),
	)

	if err != nil {
		log.Error(err)
		return nil, "", err
	}

	return res, id, err
}

// GetEnvironment ...
func GetEnvironment(org string, projectIdentifier string, environment string) (*admin.GetEnvironmentResponse, error) {
	client := DefaultClient()
	pqp := admin.ProjectQueryParam(projectIdentifier)
	response, err := client.GetEnvironment(context.Background(), admin.Identifier(environment), &admin.GetEnvironmentParams{
		ProjectIdentifier: pqp,
		AccountIdentifier: admin.AccountQueryParam(GetDefaultAccount()),
		OrgIdentifier:     admin.OrgQueryParam(org),
	}, AddAuthToken)
	if err != nil {
		return nil, err
	}

	return admin.ParseGetEnvironmentResponse(response)
}

func DeleteEnvironment(org string, projectIdentifier string, environment string) (*http.Response, error) {
	return deleteEnvironmentRemote(org, projectIdentifier, environment)
}

func deleteEnvironmentRemote(org string, project string, identifier string) (*http.Response, error) {
	reqUrl := fmt.Sprintf("%s/environmentsV2/%s", GetPlatformBaseURL(), identifier)
	req, err := http.NewRequest(http.MethodDelete, reqUrl, nil)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	query.Add("accountIdentifier", GetDefaultAccount())
	query.Add("orgIdentifier", org)
	query.Add("projectIdentifier", project)
	query.Add("forceDelete", "false")
	req.URL.RawQuery = query.Encode()

	req.Header.Set("content-type", "application/json")
	AddAuthToken(context.Background(), req)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println(res)
	fmt.Println(string(body))

	client := DefaultClient()

	resp, err := client.Client.Do(req)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// ensure environment is deleted within cf
	err = retry.Do(
		func() error {
			environmentResponse, err := GetEnvironment(org, project, identifier)
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

	return resp, nil
}
