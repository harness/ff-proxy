### Steps to run:
1. Run the proxy using a .env file from e2e/env/proxy e.g. using the `.env.qa` file to run against qa environment
2. Run the e2e tests using the matching .env file. This can be done with the make command as well e.g. `make e2e-qa` to run against the qa environment

### Areas to test:

~~Embedded SDK startup~~ (server SDK test)

~~Config fetching~~ (server SDK test)

Streaming

Proxy endpoints
- ~~PostAuthenticate~~ (server sdk test)
- ~~GetFeatureConfigs~~ (server sdk test)
- GetFeatureConfigsByIdentifier (streaming test with server sdk)
- ~~GetTargetSegments~~ (server sdk)
- GetTargetSegmentsByIdentifier (streaming test with server sdk)
- GetEvaluations (client sdk test)
- GetEvaluationsByFeature (streaming test with client sdk)
- GetStream (streaming test)
- PostMetrics
- ~~Health~~ (test setup)

### Planned tests:
- ~~Server SDK~~
- Server SDK streaming
- Client SDK
- Client SDK streaming