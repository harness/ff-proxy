package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/harness/ff-proxy/tests/e2e/testhelpers"

	"github.com/joho/godotenv"

	log "github.com/sirupsen/logrus"
)

const (
	createFilePermissionLevel = 0644
	onlineTestFileName        = "tests/e2e/env/.env.online"
	onlineInMemoryProxy       = ".env.online_in_mem"
	onlineRedisProxy          = ".env.online_redis"
	generateOfflineConfig     = ".env.generate_offline"
	offlineConfig             = ".env.offline"
)

var onlineTestTemplate = `SERVER_API_KEY=%s
STREAM_URL=http://localhost:7000
ONLINE=true
REMOTE_URL=%s
ACCOUNT_IDENTIFIER=%s
ORG_IDENTIFIER=%s
PROJECT_IDENTIFIER=%s
ENVIRONMENT_IDENTIFIER=%s
USER_ACCESS_TOKEN=%s`

var onlineProxyInMemTemplate = `ACCOUNT_IDENTIFIER=%s
ORG_IDENTIFIER=%s
ADMIN_SERVICE_TOKEN=%s
API_KEYS=%s
TLS_ENABLED=true
TLS_CERT=certs/cert.crt
TLS_KEY=certs/cert.key`

var onlineProxyRedisTemplate = `ACCOUNT_IDENTIFIER=%s
ORG_IDENTIFIER=%s
ADMIN_SERVICE_TOKEN=%s
API_KEYS=%s
AUTH_SECRET=my_secret
REDIS_ADDRESS=redis:6379
PORT=9000
TARGET_POLL_DURATION=0`

var generateOfflineConfigTemplate = `ACCOUNT_IDENTIFIER=%s
ORG_IDENTIFIER=%s
ADMIN_SERVICE_TOKEN=%s
API_KEYS=%s
AUTH_SECRET=my_secret
GENERATE_OFFLINE_CONFIG=true`

var offlineConfigTemplate = `OFFLINE=true`

func main() {
	// setup
	log.Infof("Global Test Setup")
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

	project, err := testhelpers.SetupTestProject()
	if err != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}

	// write .env for online test config
	onlineTestFile, err := os.OpenFile(fmt.Sprintf(onlineTestFileName), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	if err != nil {
		onlineTestFile.Close()
		log.Fatalf("failed to open %s: %s", onlineTestFileName, err)
	}

	_, err = io.WriteString(onlineTestFile, fmt.Sprintf(onlineTestTemplate, project.Environment.Keys[0].ApiKey, testhelpers.GetClientURL(), project.Account, project.Organization, project.ProjectIdentifier, project.Environment.Identifier, testhelpers.GetUserAccessToken()))
	if err != nil {
		log.Fatalf("failed to write to %s: %s", onlineTestFileName, err)
	}

	// write .env for proxy online in memory mode
	onlineInMemProxyFile, err := os.OpenFile(fmt.Sprintf(onlineInMemoryProxy), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	if err != nil {
		onlineInMemProxyFile.Close()
		log.Fatalf("failed to open %s: %s", onlineInMemoryProxy, err)
	}

	_, err = io.WriteString(onlineInMemProxyFile, fmt.Sprintf(onlineProxyInMemTemplate, testhelpers.GetDefaultAccount(), testhelpers.GetDefaultOrg(), testhelpers.GetUserAccessToken(), project.Environment.Keys[0].ApiKey))
	if err != nil {
		log.Fatalf("failed to write to %s: %s", onlineInMemoryProxy, err)
	}

	// write .env for proxy online redis mode
	onlineProxyRedisFile, err := os.OpenFile(fmt.Sprintf(onlineRedisProxy), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	if err != nil {
		onlineProxyRedisFile.Close()
		log.Fatalf("failed to open %s: %s", onlineRedisProxy, err)
	}

	_, err = io.WriteString(onlineProxyRedisFile, fmt.Sprintf(onlineProxyRedisTemplate, testhelpers.GetDefaultAccount(), testhelpers.GetDefaultOrg(), testhelpers.GetUserAccessToken(), project.Environment.Keys[0].ApiKey))
	if err != nil {
		log.Fatalf("failed to write to %s: %s", onlineRedisProxy, err)
	}

	// write .env for proxy generate offline config mode
	generateOfflineFile, err := os.OpenFile(fmt.Sprintf(generateOfflineConfig), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	if err != nil {
		generateOfflineFile.Close()
		log.Fatalf("failed to open %s: %s", generateOfflineConfig, err)
	}

	_, err = io.WriteString(generateOfflineFile, fmt.Sprintf(generateOfflineConfigTemplate, testhelpers.GetDefaultAccount(), testhelpers.GetDefaultOrg(), testhelpers.GetUserAccessToken(), project.Environment.Keys[0].ApiKey))
	if err != nil {
		log.Fatalf("failed to write to %s: %s", generateOfflineConfig, err)
	}

	// write .env for proxy offline config mode
	offlineFile, err := os.OpenFile(fmt.Sprintf(offlineConfig), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
	if err != nil {
		offlineFile.Close()
		log.Fatalf("failed to open %s: %s", offlineConfig, err)
	}

	_, err = io.WriteString(offlineFile, offlineConfigTemplate)
	if err != nil {
		log.Fatalf("failed to write to %s: %s", offlineConfig, err)
	}
}
