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
	cleanupTestFileName = "tests/e2e/env/.env.cleanup"
)

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
	log.Info("cleanup done")
}

func cleanUp() error {
	//parse the map.
	cleanUp, err := getCleanupFile()
	if err != nil {
		return err
	}

	err = testhelpers.DeleteProxyKey(context.Background(), testhelpers.GetDefaultAccount(), cleanUp["ProxyKey"])
	if err != nil {
		return err
	}
	// delete key from the mp
	delete(cleanUp, "ProxyKey")
	// delete all the projects.
	log.Info("attempting to delete the projects")
	for k, v := range cleanUp {
		log.Infof("attempting to delete the projects %s %s", k, v)
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
