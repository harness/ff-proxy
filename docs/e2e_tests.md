# E2E Tests

The E2E test suite has a few differnet modes that it can run in, howver Proxy V2 only supports one of these modes while Proxy V1 supports all of these modes

**supported modes**

| Mode                         | Proxy V1 | Proxy V2 | make target |
|------------------------------|----------|----------|---------|
| Online with Redis            | ✅        | ✅        | `make e2e-online-redis` |
| Online with in memory cache  | ✅        | ❌        | `make e2e-online-in-mem` |
| Offline with Redis           | ✅        | ❌        | `make e2e-offline-redis ` |
| Offline with in memory cache | ✅        | ❌        | `make e2e-offline-in-mem` |

## Running E2E Tests

### Setup

Before we can run the E2E tests we first have to manually edit [testhelpers .env.setup file](/tests/e2e/testhelpers/setup/.env.setup) and populate the `ACCOUNT_IDENTIFER` and `USER_ACCESS_TOKEN` fields. We have to do this because the tests rely on a new projects, environments, flags, sdk keys and ProxyKeys that get created each time and the `ACCOUNT_IDENTIFIER` determines which account these are created it and the `USER_ACCESS_TOKEN` gives us permissions to create the resources we need in the account.

### Generating .env files for test modes

The next thing we need to is run `make generate-e2e-env-files`. This will create the necessary resources in Harness Saas as well as the `.env` files used for all the different E2E test modes.

### Running the tests

First confirm that the version of the Proxy in the [docker-compose.yml](../docker-compose.yml) file is the version you want your tests to run against.

Once you're happy with the version of the Proxy the tests will be running against you can then run one of the make targets to execute the tests e.g `make e2e-online-redis`





