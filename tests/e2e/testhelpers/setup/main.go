package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/harness/ff-proxy/v2/tests/e2e/testhelpers"

	"github.com/joho/godotenv"

	log "github.com/sirupsen/logrus"
)

const (
	createFilePermissionLevel = 0644
	onlineTestFileName        = "tests/e2e/env/.env.online"
	cleanupTestFileName       = "tests/e2e/env/.env.cleanup"
	onlineInMemoryProxy       = ".env.online_in_mem"
	onlineRedisProxy          = ".env.online_redis"
	generateOfflineConfig     = ".env.generate_offline"
	offlineConfig             = ".env.offline"
)

var onlineTestTemplate = `
STREAM_URL=http://localhost:7000
ONLINE=true
REMOTE_URL=%s
ACCOUNT_IDENTIFIER=%s
ORG_IDENTIFIER=%s
SECONDARY_ORG_IDENTIFIER=%s
PROJECT_IDENTIFIER=%s
SECONDARY_PROJECT_IDENTIFIER=%s
ENVIRONMENT_IDENTIFIER=%s
CLIENT_URL=https://app.harness.io/gateway/cf
PROXY_KEY=%s
PROXY_AUTH_KEY=%s
SERVER_API_KEY=%s
EMPTY_PROJECT_API_KEY=%s`

// var onlineProxyInMemTemplate = `ACCOUNT_IDENTIFIER=%s
// ORG_IDENTIFIER=%s
// TLS_ENABLED=true
// TLS_CERT=certs/cert.crt
// TLS_KEY=certs/cert.key
// HEARTBEAT_INTERVAL=0
// METRIC_POST_DURATION=5
// PROXY_KEY=%s`

var onlineProxyRedisTemplate = `ACCOUNT_IDENTIFIER=%s
ORG_IDENTIFIER=%s
SECONDARY_ORG_IDENTIFIER=%s
AUTH_SECRET=my_secret
REDIS_ADDRESS=redis:6379
PORT=9000
TARGET_POLL_DURATION=0
PROXY_KEY=%s
PROXY_AUTH_KEY=%s
API_KEY=%s
EMPTY_PROJECT_API_KEY=%s`

//var generateOfflineConfigTemplate = `ACCOUNT_IDENTIFIER=%s
//ORG_IDENTIFIER=%s
//ADMIN_SERVICE_TOKEN=%s
//API_KEYS=%s
//AUTH_SECRET=my_secret
//GENERATE_OFFLINE_CONFIG=true`
//
//var offlineConfigTemplate = `OFFLINE=true`

func main() {

	log.Infof("Global Test Setup")
	var env string
	cleanUpAssets := make(map[string]string)
	// default to .env.local file if none specified
	flag.StringVar(&env, "env", ".env.setup", "env file name")
	flag.Parse()
	log.Debug(env)
	err := godotenv.Load(fmt.Sprintf("tests/e2e/testhelpers/setup/%s", env))
	if err != nil {
		log.Fatal(err)
	}

	for _, x := range os.Environ() {
		log.Infof("%s", x)
	}

	testhelpers.SetupAuth()

	orgs := []string{testhelpers.GetDefaultOrg(), testhelpers.GetSecondaryOrg()}
	projects := []testhelpers.TestProject{}

	for _, org := range orgs {
		project, err := testhelpers.SetupTestProject(org)
		if err != nil {
			log.Errorf(err.Error())
			os.Exit(1)
		}
		projects = append(projects, project)
	}

	//// setup empty project
	empty, err := testhelpers.SetupTestEmptyProject(orgs[0])
	if err != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}
	//append empty project
	projects = append(projects, empty)
	//setup empty project
	proxyKeyIdentifier := fmt.Sprintf("%s-%d", "ProxyE2ETestsProxyKey", rand.Intn(1000))

	project := projects[0]

	time.Sleep(time.Second * 15)

	proxyKey, proxyAuthToken, err := testhelpers.CreateProxyKeyAndAuthForMultipleOrgs(context.Background(), proxyKeyIdentifier, projects)
	if err != nil {
		log.Fatalf("failed to create proxy key: %s", err)
	}
	fmt.Printf("created key? [%v] [%v] ", proxyKey, proxyAuthToken)
	testhelpers.SetProxyAuthToken(proxyKey)

	cleanUpAssets["ProxyKey"] = proxyKeyIdentifier
	cleanUpAssets["ProxyAuth"] = proxyAuthToken

	for _, p := range projects {
		cleanUpAssets[p.ProjectIdentifier] = p.Organization
	}
	//dump the projects for a cleanup.
	saveFilesForCleanup(cleanUpAssets)

	//defer func() {
	//	cleanUp()
	//}()

	// write .env for online test config
	onlineTestFile, err := os.OpenFile(fmt.Sprintf(onlineTestFileName), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	if err != nil {
		onlineTestFile.Close()
		log.Fatalf("failed to open %s: %s", onlineTestFileName, err)
	}

	_, err = io.WriteString(onlineTestFile, fmt.Sprintf(onlineTestTemplate, testhelpers.GetAdminURL(), projects[0].Account, projects[0].Organization, projects[1].Organization, projects[0].ProjectIdentifier, projects[1].ProjectIdentifier, projects[0].Environment.Identifier, proxyKey, proxyAuthToken, project.Environment.Keys[0].ApiKey, empty.Environment.Keys[0].ApiKey))
	if err != nil {
		log.Fatalf("failed to write to %s: %s", onlineTestFileName, err)
	}

	//We don't care about supporting inMem atm in v2
	//write .env for proxy online in memory mode
	//onlineInMemProxyFile, err := os.OpenFile(fmt.Sprintf(onlineInMemoryProxy), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	//if err != nil {
	//	onlineInMemProxyFile.Close()
	//	log.Fatalf("failed to open %s: %s", onlineInMemoryProxy, err)
	//}
	////
	////We don't care about supporting inMem atm in v2
	//_, err = io.WriteString(onlineInMemProxyFile, fmt.Sprintf(onlineProxyInMemTemplate, testhelpers.GetDefaultAccount(), testhelpers.GetDefaultOrg(), "todo-proxykey"))
	//if err != nil {
	//	log.Fatalf("failed to write to %s: %s", onlineInMemoryProxy, err)
	//}

	// write .env for proxy online redis mode
	onlineProxyRedisFile, err := os.OpenFile(fmt.Sprintf(onlineRedisProxy), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	if err != nil {
		onlineProxyRedisFile.Close()
		log.Fatalf("failed to open %s: %s", onlineRedisProxy, err)
	}

	_, err = io.WriteString(onlineProxyRedisFile, fmt.Sprintf(onlineProxyRedisTemplate, testhelpers.GetDefaultAccount(), projects[0].Organization, projects[1].Organization, proxyKey, proxyAuthToken, project.Environment.Keys[0].ApiKey, empty.Environment.Keys[0].ApiKey))
	if err != nil {
		log.Fatalf("failed to write to %s: %s", onlineRedisProxy, err)
	}

	// We also don't care about supporting offline mode atm
	//
	// write .env for proxy generate offline config mode
	//generateOfflineFile, err := os.OpenFile(fmt.Sprintf(generateOfflineConfig), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	//if err != nil {
	//	generateOfflineFile.Close()
	//	log.Fatalf("failed to open %s: %s", generateOfflineConfig, err)
	//}
	//
	//_, err = io.WriteString(generateOfflineFile, fmt.Sprintf(generateOfflineConfigTemplate, testhelpers.GetDefaultAccount(), testhelpers.GetDefaultOrg(), testhelpers.GetUserAccessToken(), project.Environment.Keys[0].ApiKey))
	//if err != nil {
	//	log.Fatalf("failed to write to %s: %s", generateOfflineConfig, err)
	//}
	//
	//// write .env for proxy offline config mode
	//offlineFile, err := os.OpenFile(fmt.Sprintf(offlineConfig), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	//if err != nil {
	//	offlineFile.Close()
	//	log.Fatalf("failed to open %s: %s", offlineConfig, err)
	//}
	//
	//_, err = io.WriteString(offlineFile, offlineConfigTemplate)
	//if err != nil {
	//	log.Fatalf("failed to write to %s: %s", offlineConfig, err)
	//}
}

func cleanUp() error {
	//parse the map.
	cleanUp := getCleanupFile()

	fmt.Println("Sleeping")
	time.Sleep(time.Second * 10)
	fmt.Println("Attempting to delete the tests")
	proxyKey := cleanUp["ProxyKey"]
	proxyAuth := cleanUp["ProxyAuth"]

	testhelpers.SetProxyAuthToken(proxyAuth)

	err := testhelpers.DeleteProxyKey(context.Background(), testhelpers.GetDefaultAccount(), proxyKey)
	if err != nil {
		return err
	}
	//delete key from the mp
	delete(cleanUp, "ProxyKey")
	//delete all the projects.
	fmt.Println("Attempting to delete the projects")
	for k, v := range cleanUp {
		fmt.Printf("Attempting to delete the projects %s %s", k, v)
		resp, err := testhelpers.DeleteProjectForOrg(k, v)
		if err != nil {
			log.Errorf("Unable to delete project %s with code %s", err.Error(), resp.StatusCode)
		}

	}
	return nil
}

func getCleanupFile() map[string]string {
	content, err := os.ReadFile(cleanupTestFileName)
	if err != nil {
		return nil
	}
	strContent := string(content)

	m := map[string]string{}
	if err := json.Unmarshal([]byte(strContent), &m); err != nil {
		panic(err)
	}
	fmt.Println(m)
	return m
}

func saveFilesForCleanup(cleanupFiles map[string]string) {

	jsonString, _ := json.Marshal(cleanupFiles)
	cleanupTestFile, err := os.OpenFile(fmt.Sprintf(cleanupTestFileName), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	if err != nil {
		cleanupTestFile.Close()
		log.Fatalf("failed to open %s: %s", onlineTestFileName, err)
	}
	_, err = io.WriteString(cleanupTestFile, fmt.Sprintf("%s", jsonString))
	if err != nil {
		log.Fatalf("failed to write to %s: %s", onlineTestFileName, err)
	}
}
