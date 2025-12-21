# CI Optimization Recommendations

## Current State
The current CI runs 3 sequential jobs:
1. **Lint** (1 min timeout)
2. **Build** (1 min timeout)
3. **Test** (1 min timeout, depends on lint & build)

With job setup overhead and PostgreSQL service startup, total time can exceed 30 seconds.

## Quick Fix (Minimal Changes)

Make these 3 changes to `.github/workflows/ci.yml`:

1. **Remove** `needs: [lint, build]` from the test job (line 39)
2. **Remove** the entire `services:` block (lines 42-55)
3. **Change** the test command from:
   ```yaml
   run: go test -v -timeout 30s -race -coverprofile=coverage.out -covermode=atomic ./...
   ```
   to:
   ```yaml
   run: go test -v -timeout 30s -coverprofile=coverage.out ./...
   ```
4. **Remove** the `env:` block with `DATABASE_URL` (lines 65-66)

This will reduce CI time from ~45s to ~20-25s by:
- Running all 3 jobs in parallel instead of sequentially
- Removing PostgreSQL service startup time (~10-15s)
- Removing race detector overhead (~2-5s)

## Recommended Changes

### Option 1: Run All Checks in Parallel (Fastest)

Replace the current 3-job workflow with a single job that runs lint, build, and tests in parallel:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    name: CI
    runs-on: ubuntu-latest
    timeout-minutes: 2

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Install golangci-lint
        run: |
          curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.62.2

      - name: Run CI checks
        run: |
          set -e

          # Start lint in background
          ($(go env GOPATH)/bin/golangci-lint run --timeout 60s 2>&1 | tee lint.log; echo $? > lint.exit) &
          LINT_PID=$!

          # Start build in background
          (go build -v ./... 2>&1 | tee build.log; echo $? > build.exit) &
          BUILD_PID=$!

          # Run tests (no race detector for speed in CI)
          go test -v -timeout 30s -coverprofile=coverage.out ./... 2>&1 | tee test.log
          TEST_EXIT=$?

          # Wait for background jobs
          wait $LINT_PID || true
          wait $BUILD_PID || true

          # Check results
          LINT_EXIT=$(cat lint.exit 2>/dev/null || echo 1)
          BUILD_EXIT=$(cat build.exit 2>/dev/null || echo 1)

          echo "=== Results ==="
          echo "Lint exit: $LINT_EXIT"
          echo "Build exit: $BUILD_EXIT"
          echo "Test exit: $TEST_EXIT"

          # Fail if any step failed
          if [ "$LINT_EXIT" != "0" ]; then
            echo "::error::Lint failed"
            cat lint.log
            exit 1
          fi

          if [ "$BUILD_EXIT" != "0" ]; then
            echo "::error::Build failed"
            cat build.log
            exit 1
          fi

          if [ "$TEST_EXIT" != "0" ]; then
            echo "::error::Tests failed"
            exit 1
          fi

          echo "All checks passed!"

      - uses: codecov/codecov-action@v4
        if: always()
        with:
          files: ./coverage.out
          fail_ci_if_error: false
```

### Option 2: Keep Separate Jobs but Run in Parallel

If you prefer keeping separate jobs for clearer error reporting:

```yaml
jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    timeout-minutes: 1
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  build:
    name: Build
    runs-on: ubuntu-latest
    timeout-minutes: 1
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - run: go build -v ./...

  test:
    name: Test
    runs-on: ubuntu-latest
    timeout-minutes: 1
    # Remove the "needs: [lint, build]" to run in parallel
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: Run tests
        run: go test -v -timeout 30s -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v4
        with:
          files: ./coverage.out
          fail_ci_if_error: false
```

## Key Optimizations

1. **Remove PostgreSQL service** - Unit tests don't require a database connection
2. **Remove `-race` flag** - Race detection adds ~2-10x overhead; use it only in nightly builds
3. **Run jobs in parallel** - Remove `needs: [lint, build]` dependency or combine into single job
4. **Use Go module caching** - Already enabled with `cache: true`

## Expected Results

| Configuration | Estimated Time |
|--------------|----------------|
| Current (sequential + PostgreSQL) | 45-90 seconds |
| Option 1 (single parallel job) | 15-25 seconds |
| Option 2 (parallel separate jobs) | 20-30 seconds |

## Test Coverage

The new test suite covers:
- `internal/config` - Configuration validation and loading
- `internal/streaming` - Vercel AI SDK protocol implementation
- `internal/middleware` - Authentication middleware
- `internal/services` - Auth, Chat, and LLM services
- `internal/handlers` - HTTP handlers and helpers

All tests are designed to run without external dependencies (database, OpenAI API) for fast execution.
