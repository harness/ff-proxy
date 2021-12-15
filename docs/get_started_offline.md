To run the proxy in offline mode create a .env in this  directoy with these environment variables

```
DEBUG=false
OFFLINE=true
BYPASS_AUTH=false
HOST=localhost
PORT=7000
AUTH_SECRET=somethingSecret
REDIS_ADDRESS=docker.for.mac.localhost:6379
```

Then run docker-compose from this directory

`$ docker-compose up`

You can use the example-sdk to evaluate flags. If you don't have it built then you can navigate back to the root directory to build and run it.
```
$ cd ..
$ make build-example-sdk
$ ./ff-example-sdk
```

The example SDK polls every 30 seconds and logs out the flag evaluation for the `harnessappdemodarkmode` flag so after 30 seconds you should see this log `KeyFeature flag 'harnessappdemodarkmode' is true for this user`.

If you want to toggle the flag to off you can do that by changing it's state to `off` [here](../config/env-94ef7361-1f2d-40af-9b2c-c1145d537e5a/feature_config.json#L276)

After changing it's state you will want to restart the Proxy so it picks up the new config so run these commands from this `docs` directory.

```
$ docker-compose down
$ docker-compose up
```

Then navigate back to the root directory and run the example sdk and you should see it log out a different evaluation for the `harnessappdemodarkmode` flag.

```
$ cd ..
$ ./ff-example-sdk
```

And after 30 seconds you should see it log out false
```
KeyFeature flag 'harnessappdemodarkmode' is false for this user
```
