# Example SDK

This is an example SDK that you can use with a Proxy if you're going through the [quickstart](../../README.md) section of the README. The defaults are configured to work with the values in the basic config that is loaded into the proxy when you run `make run` so that all you have to do is run `./ff-example-sdk` and it will connect to the proxy and start evaluating flags.

```
Usage of ./ff-example-sdk:
  -api-key string
    	api key to use (default "c25e3f4e-9d2d-42d6-a85c-6fb3af062732")
  -baseURL string
    	The base url to use (default "http://localhost:7000")
  -eventsURL string
    	The events url to use (default "http://localhost:7000")
  -feature-flag string
    	the feature flag to use, if left empty defaults are used (default "harnessappdemodarkmode")
  -streaming
    	whether streaming is enabled
  -target-identifier string
    	the identifier of the target you want the SDK to use (default "james")
```
