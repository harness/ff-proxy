To run the proxy in online mode connected to FeatureFlags in UAT create a `.env` file in this directory with these environment variables

```
DEBUG=false
OFFLINE=false
BYPASS_AUTH=false
PORT=7000
ACCOUNT_IDENTIFIER=AQ8xhfNCRtGIUjq5bSM8Fg
ORG_IDENTIFIER=FF_SDK_Tests
ADMIN_SERVICE=https://uat.harness.io/gateway/cf

# You need to generate an ADMIN_SERVICE_TOKEN yourself and add it here. If you
# are unsure of how to do this reach out to someone in the FeatureFlags team.
ADMIN_SERVICE_TOKEN=

CLIENT_SERVICE=https://config.feature-flags.uat.harness.io/api/1.0
AUTH_SECRET=secret
SDK_BASE_URL=https://config.feature-flags.uat.harness.io/api/1.0
SDK_EVENTS_URL=https://event.feature-flags.uat.harness.io/api/1.0
REDIS_ADDRESS=docker.for.mac.localhost:6379

# These two keys are a client and server key - if they aren't working someone
# might have deleted them from UAT so you'll have to create new ones and add them here
API_KEYS=e8063c58-c9b2-4954-920f-15fe5319622f,bf581ae1-231e-4d91-aaa6-e1d36aed595d
TARGET_POLL_DURATION=60
```

Then run docker-compose up from this directory
`$ docker-compose up`

You can use the example-sdk to evaluate flags. If you don't have it built then you can navigate back to the root directory to build and run it. 
```
$ cd ..
$ make build-example-sdk
$ ./ff-example-sdk -api-key bf581ae1-231e-4d91-aaa6-e1d36aed595d -feature-flag bool_flag
```

The SDK will then begin polling for changes every minute and evaluating the value for [this](https://uat.harness.io/ng/#/account/AQ8xhfNCRtGIUjq5bSM8Fg/cf/orgs/FF_SDK_Tests/projects/FF_SDK_Test_Project/feature-flags?activeEnvironment=test_env) flag. If you have access to this project then you can toggle the flag on/off via the UI and watch the SDK get the new value and log it out.

```
KeyFeature flag 'bool_flag' is false for this user

// Toggle flag on in UI and wait for sdk to poll

KeyFeature flag 'bool_flag' is true for this user
```


