package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	admingen "github.com/harness/ff-proxy/gen/admin"
	"log"
	"math/rand"
	"net/http"
	"time"
)

var (
	accountIdentifier string
	orgIdentifier    string
	remoteService     string
	serviceToken     string
	bearerToken string
	projectIdentifier string
	adminService      string
	platformService string
	flagNumber int
	envNumber int
	targetNumber int
	segmentNumber int
)

func init() {
	// account info
	flag.StringVar(&accountIdentifier, "account-identifier", "AQ8xhfNCRtGIUjq5bSM8Fg", "account identifier to load remote config for")
	flag.StringVar(&orgIdentifier, "org-identifier", "default", "org identifier to load remote config for")
	flag.StringVar(&projectIdentifier, "project-identifier", "proxyloadtestsmall", "project identifier to load remote config for")

	flag.StringVar(&remoteService, "remote-service", "https://uat.harness.io", "harness environment url")

	// tokens
	flag.StringVar(&serviceToken, "service-token", "", "token to use with the ff service")
	// we need a bearer token to hit platform endpoints to create environments - can this also be done with the service token?
	flag.StringVar(&bearerToken, "bearer-token", "", "platform bearer token")

	// how many resources to generate
	flag.IntVar(&flagNumber, "flag-number", 5, "how many flags do you want created?")
	flag.IntVar(&envNumber, "env-number", 2, "how many environments do you want created?")
	flag.IntVar(&targetNumber, "target-number", 5, "how many targets do you want created per environment?")
	flag.IntVar(&segmentNumber, "segment-number", 5, "how many segments do you want created per environment?")


	adminService = fmt.Sprintf("%s/gateway/cf", remoteService)
	platformService = fmt.Sprintf("%s/ng/api", remoteService)
	flag.Parse()
}

func strPtr(s string) *string { return &s }

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


func main() {
	// create client
	c, err := admingen.NewClientWithResponses(
		adminService,
		admingen.WithHTTPClient(doer{c: http.DefaultClient, token: serviceToken}),
	)
	if err != nil {
		log.Fatal("Couldn't setup admin client")
	}


	// create flags
	createFlags(c)

	// create envs, targets and segments
	createEnvironments(c)

	// create and save api keys
	keys := createAPIKeys(c)
	log.Printf("%v", keys)
	// print keys in format needed for proxy service
	keysString := ""
	for _, key := range keys {
		keysString += fmt.Sprintf("-apiKey %s ", key)
	}
	log.Printf(keysString)

}

func createFlags(c *admingen.ClientWithResponses) {
	for i:= 0; i < flagNumber; i++ {
		flagName := fmt.Sprintf("flag%d", i)
		log.Printf("Creating flag %s", flagName)
		res, err := c.CreateFeatureFlag(context.Background(), &admingen.CreateFeatureFlagParams{
			AccountIdentifier: admingen.AccountQueryParam(accountIdentifier),
			Org:               admingen.OrgQueryParam(orgIdentifier),
		}, admingen.CreateFeatureFlagJSONRequestBody{
			DefaultOffVariation: "false",
			DefaultOnVariation:  "true",
			Description:         &flagName,
			Identifier:          flagName,
			Kind:                "boolean",
			Name:                flagName,
			Permanent:           false,
			Project:             projectIdentifier,
			Variations: []admingen.Variation{
				{Identifier: "true", Name: strPtr("True"), Value: "true"},
				{Identifier: "false", Name: strPtr("False"), Value: "false"},
			},
		})

		if err != nil {
			log.Printf("error creating flag %s: %v", flagName, err)
		}

		if res.StatusCode != 201 {
			log.Printf("Flag create failed, error code %v", res.StatusCode)
		}

	}
}

func createTargets(env string, c *admingen.ClientWithResponses) {
	for i:= 0; i < targetNumber; i++ {
		targetName := fmt.Sprintf("target%d", i)
		log.Printf("Creating target %s", targetName)
		res, err := c.CreateTarget(context.Background(), &admingen.CreateTargetParams{
			AccountIdentifier: admingen.AccountQueryParam(accountIdentifier),
			Org:               admingen.OrgQueryParam(orgIdentifier),
		}, admingen.CreateTargetJSONRequestBody{
			Account:     accountIdentifier,
			Environment: env,
			Identifier:  targetName,
			Name:        targetName,
			Org:         orgIdentifier,
			Project:     projectIdentifier,
		})

		if err != nil {
			log.Printf("error creating target %s: %v", targetName, err)
		}

		if res.StatusCode != 201 {
			log.Printf("Target create failed, error code %v", res.StatusCode)
		}

	}
}

func createSegments(env string, c *admingen.ClientWithResponses) {
	for i:= 0; i < segmentNumber; i++ {
		segmentName := fmt.Sprintf("segment%d", i)
		log.Printf("Creating segment %s", segmentName)
		res, err := c.CreateSegment(context.Background(), &admingen.CreateSegmentParams{
			AccountIdentifier: admingen.AccountQueryParam(accountIdentifier),
			Org:               admingen.OrgQueryParam(orgIdentifier),
		}, admingen.CreateSegmentJSONRequestBody{
			Environment: env,
			Identifier:  &segmentName,
			Name:        segmentName,
			Project:     projectIdentifier,
		})

		if err != nil {
			log.Printf("error creating segment %s: %v", segmentName, err)
		}

		if res.StatusCode != 201 {
			log.Printf("Segment create failed, error code %v", res.StatusCode)
		}

	}
}

func createEnvironments(c *admingen.ClientWithResponses) {
	for i:= 0; i < envNumber; i++ {
		envName := fmt.Sprintf("env%d", i)
		log.Printf("Creating environment %s", envName)
		createEnvironment(envName)

		// create targets
		createTargets(envName, c)

		// create segments
		createSegments(envName, c)
	}
}

type apiKey struct {
	ApiKey string `json:"apiKey"`
}

func createAPIKeys(c *admingen.ClientWithResponses) []string {
	apiKeys := []string{}
	rand.Seed(time.Now().UnixNano())
	keyName := fmt.Sprintf("key%s", rand.Intn(10000))
	for i:= 0; i < envNumber; i++ {
		var key apiKey
		envName := fmt.Sprintf("env%d", i)

		log.Printf("Creating api key for env %s", envName)
		res, err := c.AddAPIKey(context.Background(), &admingen.AddAPIKeyParams{
			AccountIdentifier: admingen.AccountQueryParam(accountIdentifier),
			Org:               admingen.OrgQueryParam(orgIdentifier),
			Project: admingen.ProjectQueryParam(projectIdentifier),
			Environment: admingen.EnvironmentQueryParam(envName),
		}, admingen.AddAPIKeyJSONRequestBody{
			Identifier:  keyName,
			Name:        keyName,
			Type:        "Server",
		})

		if err != nil {
			log.Printf("error creating api key for env %s: %v", envName, err)
			continue
		}

		if res.StatusCode != 201 {
			log.Printf("API key create failed, error code %v", res.StatusCode)
			continue
		}

		err = json.NewDecoder(res.Body).Decode(&key)
		if err != nil {
			log.Printf("Error decoding api key response: %s", err)
			continue
		}

		apiKeys = append(apiKeys, key.ApiKey)


	}
	return apiKeys
}




// Hitting the platform endpoints is all fairly ugly - can we include some generated code for this too?

// PlatformEnvironment ...
type PlatformEnvironment struct {
	Name              string `json:"name"`
	Identifier        string `json:"identifier"`
	ProjectIdentifier string `json:"projectIdentifier"`
	OrgIdentifier     string `json:"orgIdentifier"`
	Type              string `json:"type"`
}


// createEnvironment ...
func createEnvironment(environment string) {
	//Encode the data
	body := PlatformEnvironment{
		Name:              environment,
		OrgIdentifier:     orgIdentifier,
		ProjectIdentifier: projectIdentifier,
		Identifier:        environment,
		Type:              "PreProduction",
	}
	postBody, err := json.Marshal(body)
	if err != nil {
		log.Printf("couldn't marshal json")
		return
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/environments?accountId=%s", platformService, accountIdentifier), bytes.NewReader(postBody))
	if err != nil {
		log.Printf("An Error Occured %v", err)
		return
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", bearerToken))
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Printf("An Error Occured %v", err)
		return
	}

	switch resp.StatusCode {
	case 200:
		log.Printf("Created environment %s", environment)
		break
	case 409:
		log.Printf("Environment %s already exists", environment)
		break
	default:
		log.Printf("Error creating environment %s: %d", environment, resp.StatusCode)
	}
}
