# Debugging

## Connectivity 
The most common issues people have when setting up a Relay Proxy for the first time are connectivity issues. These can be either between the Relay Proxy and Harness SaaS or between connecting sdks and the Relay Proxy itself. It helps to focus on one of these at a time.

### Relay Proxy to Harness Saas
The first thing to check is if the Relay Proxy itself has started correctly.

#### Startup process
To help debug any setup/connectivity issues within the proxy it helps to know the steps it goes through on startup. Here we’ll also show the log messages that these steps produce so you can step through and see what’s happening.

**1. Fetch config**: This connects to the Harness SaaS feature flag service and fetches required info like projects/environment data etc. 

``{"level":"info","ts":"2022-06-06T10:41:10+01:00","caller":"ff-proxy/main.go:331","msg":"retrieving config from ff-server..."}``

if this succeeds you’ll see the log message

``{"level":"info","ts":"2022-06-06T10:41:14+01:00","caller":"ff-proxy/main.go:345","msg":"successfully retrieved config from FeatureFlags"}``

If this fails you’ll see a log message like this:

``{"level":"error","ts":"2022-06-06T11:42:33+01:00","caller":"ff-proxy/main.go:343","msg":"error(s) encountered fetching config from FeatureFlags, startup will continue but the Proxy may be missing required config","errors":"${error message goes here}"}``

**N.B:** If you encounter an error at this point setting up the Relay Proxy it means we can’t fetch config from Harness SaaS and it’s unlikely any other functionality will work. Fixing this should be the top priority to unblock. The detailed issue that caused the problem will be in the errors component of the message ^. It may be that the url provided is incorrect, the service token isn’t valid, or firewall rules are blocking requests.

**2. Start SDK’s to connect to Harness SaaS**: Under the hood the proxy starts an instance of the go-sdk for each api key that the user has configured. This is how for each environment the proxy fetches the initial config and gets updates via sse. This goes through 2 stages.

- **Fetch feature/target group config.** You’ll see log messages for starting/finishing the individual requests here. When both finish successfully you’ll see this log message:

``{"level":"info","ts":"2022-06-06T12:03:11+01:00","caller":"client/client.go:158","msg":"Data poll finished successfully","component":"SDK","apiKey":"key","environmentID":"830019c6-23a5-48d4-ab8c-48fa767f1deb","environment_identifier":"dev","project_identifier":"testproj"}``

Note that this log message also specifies which project_identifier and environment_identifier we’re fetching the flags for. This is useful if you want to search for logs for a particular project/environment if you’re having issues. If you don’t see this message anywhere it’s likely some config is incorrect or a server sdk key hasn’t been provided for this environment. 

- **Start stream connection** to receive events any changes made on Harness SaaS. If you see this message the stream has started successfully:

``{"level":"info","ts":"2022-06-06T12:03:11+01:00","caller":"stream/sse.go:68","msg":"Start subscribing to Stream","component":"SDK","apiKey":"key","environmentID":"830019c6-23a5-48d4-ab8c-48fa767f1deb","environment_identifier":"dev","project_identifier":"testproj"}``


**Receiving updates**

When you make a change to a flag on Harness Saas you should see the update event within the proxy logs like this:

``{"level":"info","ts":"2022-06-06T10:41:55+01:00","caller":"stream/sse.go:79","msg":"Event received: {\"event\":\"patch\",\"domain\":\"flag\",\"identifier\":\"flag\",\"version\":4}","component":"SDK","apiKey":"key","environmentID":"830019c6-23a5-48d4-ab8c-48fa767f1deb","environment_identifier":"dev","project_identifier":"testproj"}``

Note that you will only see these updates for any environments you’ve configured the proxy for (i.e. api keys you’ve added when you started the proxy). If you’re toggling a flag on Harness Saas and don’t see this event coming through see the section below to test these connectivity issues. 

