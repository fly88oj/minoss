#!/usr/bin/env bash

set -e

export GORACE="history_size=7"
export MINIO_API_REQUESTS_MAX=10000

# A single `go test ./...` lets the go tool parallelize across packages;
# -p is capped at 2 because race instrumentation makes each test binary
# memory hungry (5-10x per the race detector docs) and CI runners are
# small. -count=1 keeps re-runs honest - without it, packages with cached
# results would silently skip their race pass.
CGO_ENABLED=1 go test -race -count=1 -p 2 --timeout 100m ./...
