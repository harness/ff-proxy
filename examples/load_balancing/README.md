# Load Balancing Example
Here we will spin up:
- 3 Relay Proxy instances
- A basic nginx instance that load balances traffic between them

For this configuration all Relay Proxies will use the same .env file, this is required as all proxies need to be able to serve identical data if you want to load balance across them.

Note this is only a quickstart example config and should not be used for production purposes.

### Running
This example can be run by:
1. Add your config to the .env file
2. `docker-compose --env-file .env up --remove-orphans --scale proxy=3`
3. Connect sdks to `http://localhost:8000` if running locally or whatever url your nginx is listening on otherwise.
An example of this for the Golang sdk would be:

```
	client, err := harness.NewCfClient(sdkKey, 
		harness.WithURL("http://localhost:8000"), 
		harness.WithEventsURL("http://localhost:8000")
	)
```