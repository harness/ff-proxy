package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/harness/ff-proxy/v2/tests/e2e/testhelpers"

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
	// setup
	log.Infof("Global Test cleanup")
	var env string
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

	log.Infof("attempting to delete keys and projects")

	err = cleanUp()
	if err != nil {
		log.Fatal("unable to cleanup files ", "error ", err)
	}

	log.Infof("cleanup done")
}

func cleanUp() error {
	//parse the map.
	cleanUp, err := getCleanupFile()
	if err != nil {
		return err
	}

	fmt.Println("attempting to delete the tests")
	err = testhelpers.DeleteProxyKey(context.Background(), testhelpers.GetDefaultAccount(), cleanUp["ProxyKey"])
	if err != nil {
		return err
	}
	// delete key from the mp
	delete(cleanUp, "ProxyKey")
	// delete all the projects.
	fmt.Println("attempting to delete the projects")
	for k, v := range cleanUp {
		fmt.Printf("attempting to delete the projects %s %s", k, v)
		resp, err := testhelpers.DeleteProjectForOrg(k, v)
		if err != nil {
			log.Errorf("unable to delete project %s with code %s", err.Error(), resp.StatusCode)
		}
	}
	return nil
}

func getCleanupFile() (map[string]string, error) {
	content, err := os.ReadFile(cleanupTestFileName)
	if err != nil {
		return nil, err
	}
	strContent := string(content)

	m := map[string]string{}
	if err := json.Unmarshal([]byte(strContent), &m); err != nil {
		return nil, err
	}
	fmt.Println(m)
	return m, nil
}
