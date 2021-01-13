#!/bin/bash

# Run coverage tests
./bazel.sh --bazelrc=.buildkite-bazelrc coverage --config=remote-cache --features=norace --test_tag_filters="-race_on" --nocache_test_results -k  //...

# Collect all coverage results into a single file (for deepsource).
find $(./bazel.sh --bazelrc=.buildkite-bazelrc info bazel-testlogs) -iname coverage.dat | xargs -t -rd '\n' ./bazel.sh --bazelrc=.buildkite-bazelrc run //tools/gocovmerge:gocovmerge -- > /tmp/cover.out

# Download deepsource CLI
curl https://deepsource.io/cli | sh

# Upload to deepsource (requires DEEPSOURCE_DSN environment variable)
./bin/deepsource report --analyzer test-coverage --key go --value-file /tmp/cover.out

# Upload to codecov (requires CODECOV_TOKEN environment variable)
bash <(curl -s https://codecov.io/bash) -s $(./bazel.sh info bazel-testlogs) -f '**/coverage.dat'
