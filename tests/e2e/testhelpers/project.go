package testhelpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	v1 "github.com/harness/ff-proxy/gen/admin"
	log "github.com/sirupsen/logrus"

	"github.com/harness/ff-proxy/gen/admin"

	"github.com/avast/retry-go"
)

// DefaultProjectIdentifier ...
const (
	DefaultProjectIdentifier = "CF_SystemTest"
	DefaultProjectDesc       = "CF System Test project"
	DefaultProjectName       = "CF System Test"
	DefaultSegment           = "group"
	DefaultSegmentName       = "groupName"
)

// TestProject represents a project that we will create during tests
type TestProject struct {
	Account           string
	Organization      string
	ProjectIdentifier string
	Environment       Environment
	Flags             map[string]v1.FeatureFlagRequest
}

// Environment contains entities that we will use to drive test data
type Environment struct {
	Identifier string
	Name       string
	Project    string
	Keys       []v1.ApiKey
	Targets    []v1.Target
	Segments   map[string]Segment
}

// ToEnvironment converts the test environment to a v1.Environment for use in API calls
func (e Environment) ToEnvironment() v1.Environment {
	env := v1.Environment{
		Identifier: e.Identifier,
		Name:       e.Name,
		Project:    e.Project,
	}

	var keys []v1.ApiKey
	for _, key := range e.Keys {
		keys = append(keys, key)
	}
	env.ApiKeys = v1.ApiKeys{ApiKeys: &keys}

	return env
}

// Segment are groups of targets
type Segment struct {
	Included []string
}

var keys = []v1.ApiKey{
	{Identifier: "testserverkey1", Name: "test server key one", Type: APIKeyTypeServer},
	{Identifier: "testclientkey1", Name: "test client key one", Type: APIKeyTypeClient},
}

// SetupTestProject creates a new project and environment for the tests
func SetupTestProject() (TestProject, error) {
	log.Debug("Setup Test data")
	// create project and environment
	projectIdentifier, err := CreateDefaultProject()
	if err != nil {
		return TestProject{}, err
	}
	if projectIdentifier == "" {
		return TestProject{}, fmt.Errorf("empty project identifier")
	}
	env1, err := setupEnvironment(projectIdentifier, GetDefaultEnvironment(), "Primary Env")
	if err != nil {
		return TestProject{}, err
	}

	// create bool flag
	var flags = make(map[string]v1.FeatureFlagRequest)
	boolflag := GenerateBooleanFeatureFlagBody(projectIdentifier, 1)
	flags[boolflag.Identifier] = v1.FeatureFlagRequest(boolflag)

	// create string flag
	stringflag := GenerateStringFeatureFlagBody(projectIdentifier, 1)
	flags[stringflag.Identifier] = v1.FeatureFlagRequest(stringflag)

	for _, flag := range flags {
		_, err := CreateFeatureFlag(v1.CreateFeatureFlagJSONRequestBody(flag))
		if err != nil {
			return TestProject{}, err
		}
	}

	// create target group
	_, err = CreateSegment(GetSegmentRequestBody(projectIdentifier, GetDefaultEnvironment(), DefaultSegment, DefaultSegmentName, nil,
		nil, nil, nil))
	if err != nil {
		return TestProject{}, err
	}

	return TestProject{
		Account:           GetDefaultAccount(),
		Organization:      GetDefaultOrg(),
		ProjectIdentifier: projectIdentifier,
		Environment:       env1,
		Flags:             flags,
	}, nil
}

func setupEnvironment(projectIdentifier, environmentIdentifier, environmentName string) (Environment, error) {
	env1, err := CreateEnvironment(projectIdentifier, environmentIdentifier, environmentName)
	if err != nil {
		return Environment{}, err
	}
	if env1 == nil {
		return Environment{}, fmt.Errorf("environment not created")
	}

	env1Keys := make([]v1.ApiKey, 0, len(keys))
	for _, key := range keys {
		// Generate an identifier based on the env name and key identifier
		keyIdentifier := fmt.Sprintf("%s-%s", environmentIdentifier, key.Identifier)
		resp, err := AddAPIKey(GetAddAPIKeyBody(keyIdentifier, string(key.Type), key.Name, "", APIKeyExpiredAt), projectIdentifier, environmentIdentifier)
		if err != nil {
			return Environment{}, err
		}
		if resp.JSON201 == nil {
			return Environment{}, fmt.Errorf("environment not created")
		}
		env1Keys = append(env1Keys, *resp.JSON201)
	}

	return Environment{
		Identifier: environmentIdentifier,
		Name:       environmentName,
		Project:    projectIdentifier,
		Keys:       env1Keys,
		Targets:    []v1.Target{},
		Segments:   map[string]Segment{},
	}, nil
}

func SetupAuth() {
	log.Info("Setup Admin tests")
	// authentication setup
	if GetUserAccessToken() != "" {
		log.Debugf("Using USER_ACCESS_TOKEN for authentication")
		SetAuthToken(GetUserAccessToken())
	} else {
		log.Fatal("No authentication method set")
	}

}

// PlatformProject ...
type PlatformProject struct {
	Name          string `json:"name"`
	OrgIdentifier string `json:"orgIdentifier"`
	Description   string `json:"description"`
	Identifier    string `json:"identifier"`
}

// CreateRemoteProject ...
type CreateRemoteProject struct {
	Project PlatformProject `json:"project"`
}

// CreateProject ...
func CreateProject(projectReq admin.CreateProjectJSONRequestBody) (*http.Response, error) {
	if IsPlaformEnabled() {
		return CreateProjectRemote(projectReq.Identifier)
	}

	res, err := CreateProjectLocal(projectReq)
	return res.HTTPResponse, err
}

// GenerateProjectIdentifier ...
func GenerateProjectIdentifier(seed string) string {
	// on remote systems we can't create a project with the same name twice
	// so we tag on a random number to the default project name
	// this could be changed to a UUID or any other method to ensure uniqueness
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%s%d", seed, rand.Intn(100000000))
}

// CreateDefaultProject ...
func CreateDefaultProject() (string, error) {
	identifier := GenerateProjectIdentifier(DefaultProjectIdentifier)
	description := DefaultProjectDesc

	_, err := CreateProject(admin.CreateProjectJSONRequestBody{
		Description: &description,
		Identifier:  identifier,
		Name:        DefaultProjectName,

		Tags: &[]admin.Tag{
			{
				Name:  DefaultTagName,
				Value: &identifier,
			},
		},
	})
	if err != nil {
		return "", err
	}
	return identifier, nil
}

// CreateProjectLocal ...
func CreateProjectLocal(projectReq admin.CreateProjectJSONRequestBody) (*admin.CreateProjectResponse, error) {
	client := DefaultClient()

	response, err := client.CreateProject(context.Background(), &admin.CreateProjectParams{
		AccountIdentifier: admin.AccountQueryParam(GetDefaultAccount()),
		OrgIdentifier:     admin.OrgQueryParam(GetDefaultOrg()),
	}, projectReq, AddAuthToken)
	if err != nil {
		return nil, err
	}
	return admin.ParseCreateProjectResponse(response)
}

// CreateProjectRemote ...
func CreateProjectRemote(identifier string) (*http.Response, error) {
	client := DefaultClient()
	url := fmt.Sprintf("%s/projects?accountIdentifier=%s&orgIdentifier=%s", GetPlatformBaseURL(), GetDefaultAccount(), GetDefaultOrg())
	body := CreateRemoteProject{Project: PlatformProject{
		Name:          identifier,
		OrgIdentifier: GetDefaultOrg(),
		Description:   identifier,
		Identifier:    identifier,
	}}
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

	// ensure project is created within cf
	err = retry.Do(
		func() error {
			projectResponse, err := ReadProject(admin.Identifier(identifier))
			if err != nil || projectResponse.StatusCode() != http.StatusOK {
				return errors.New("project not found")
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

// ReadProject ...
func ReadProject(identifier admin.Identifier) (*admin.GetProjectResponse, error) {
	client := DefaultClient()

	response, err := client.GetProject(context.Background(), identifier, &admin.GetProjectParams{
		AccountIdentifier: admin.AccountQueryParam(GetDefaultAccount()),
		OrgIdentifier:     admin.OrgQueryParam(GetDefaultOrg()),
	}, AddAuthToken)
	if err != nil {
		return nil, err
	}

	return admin.ParseGetProjectResponse(response)
}
