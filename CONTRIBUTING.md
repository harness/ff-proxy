## Prerequisites

To build and run the project locally you need to have

- golang >=1.16
- make

## Clone and initialise the repo
To submit pr's as a contributor outside of Harness you should fork the repo and change the git clone command below to your own forked repo url.
```
git clone --recursive https://github.com/harness/ff-proxy.git
git submodule init
git submodule update
```

## Build

The build target will generate the required code and build a binary named ff-proxy

```
make build
```

## Build docker image
The image target will generate a docker image called harness/ff-proxy:latest

```
make image
```

## Run the proxy
You can run the proxy by executing the binary or running the docker image and configure it by passing various flag values listed in the docs [here](https://developer.harness.io/docs/feature-flags/ff-using-flags/relay-proxy/#configuration-variables). They can also be found in [main.go](https://github.com/harness/ff-proxy/blob/main/cmd/ff-proxy/main.go)

```
./ff-proxy -admin-service-token $SERVICE_TOKEN -auth-secret $AUTH_TOKEN -account-identifier $ACCOUNT_IDENTIFIER -org-identifier $ORG_IDENTIFIER -api-keys $API_KEYS
```

```
docker run -d -p 7000:7000 --env-file .env harness/ff-proxy:latest 
```

## Testing
### Unit tests
Run unit tests by running 
```make test```

### E2E Tests
The end to end test suites have a few modes to test all the different modes the relay proxy can run in.
#### Offline E2E test
The simplest and least impactful of these to run is the offline tests which use a pre-generated offline config stored in [our test directory](/tests/e2e/testdata/config). This can be run by running 
```
make e2e-offline
```
This will spin up a relay proxy in offline mode using this offline config and run a few validation tests against it.

#### Online E2E tests
**Note:** These tests run automatically for all raised pr's as a quality guard. It's only really worth setting them up and running if you need to debug a specific issue that's causing test failures in your pr. Otherwise it's best to see what test failed and reproduce by running the proxy directly.

**Warning:** The test setup command below for these e2e tests will use the provided credentials to create a project and add test flags to your account on SaaS, only run it if you're aware of this and are happy to delete the project after.

In addition to the offline tests above, these run in 3 modes currently, online in memory, online with redis, and then generating offline config and using it to run offline.

**1. Configure .env.setup file**

Populate the [testhelpers .env.setup file](/tests/e2e/testhelpers/setup/.env.setup) with valid account credentials i.e. populate the ``ACCOUNT_IDENTIFIER`` and ``USER_ACCESS_TOKEN`` fields with a valid account and service token for that account.

**2. Populate test data**

This will create the test project and flag on Saas as well as generating .env files with the correct configuration to run all the e2e test modes. These will be saved in the root directory as ```.env.generate_offline```, ``.env.online_in_mem``, ``.env.online_redis``
```
make generate-e2e-env-files
```
**3. Run tests**

You can run each of the tests using the specified make commands, 
```
make e2e-online-in-mem
make e2e-online-redis
make e2e-generate-offline-config
```


### Generating test coverage report
You can generate a test report by running ```make test-report```. This will output a html test coverage report in the base directory.