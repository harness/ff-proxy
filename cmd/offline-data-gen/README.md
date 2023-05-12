# Offline Data Gen

This is a simple cli tool for generating Feature, Segment and Target config that can be side loaded into the proxy when it's running in offline mode. This could be useful if you just want to generate different data for local dev or if you want to generate a bunch of data for load testing or for running the [benchmarks](../../config/bench-test). The way it works is it will generate a specified number of environments with a set number of features/segments/targets per env or a different number of features/targets/segments per env depending on the value of the `-factor` flag. The `-factor` flag gets applied to each env so if you want to create three environments and set the baseline `-feature 10` and have a `-factor 2` then the first environment will have 10 features, the second will have 20 and the third will have 40.

```
Usage of ./offline-data-gen:
  -environments int
    	the number of environments to generate (default 2)
  -factor int
    	the factor to apply to the number of features, segments and targets for each environment that's created (default 2)
  -features int
    	baseline number of features to generate (default 1)
  -segments int
    	baseline number of segments to generate (default 1)
  -targets int
    	baseline number of targets to generate (default 1)
```

## Examples

Generating two environments and each environment has 10 Features, 20 Targets and 30 Segments

```
./offline-data-gen -environments 2 -factor 1 -features 10 -targets 20 -segments 30
```

Generating two environments and the first environment has 10 features, 20 targets, 30 segments and the second environment has 20 features, 40 targets, 60 segments
```
./offline-data-gen -environments 2 -factor 2 -features 10 -targets 20 -segments 30
```
