# Bench Tests

If you want to run the go bench tests you should use the [offline-data-gen tool](../../cmd/offline-data-gen) to populate this directory with environments.

Assuming you have the offline-data-gen tool already built you should be able to run this command from this directory 

```
../../cmd/offline-data-gen/offline-data-gen -environments 2 -factor 2 -features 100 -
segments 100 -targets 100
```

And then if you navigate to the `/proxy-service/` directory you can run the benchmarks with `go test -bench=.`

