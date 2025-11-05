# E2E Test Dependencies

This directory contains the E2E test infrastructure for the Grove Operator, including dependency management for external components.

## Managing Dependencies

E2E test dependencies (container images and Helm charts) are managed in `dependencies.yaml`, similar to how Go dependencies are managed in `go.mod`.

### File: `dependencies.yaml`

This file defines all external dependencies used in E2E tests:

- **Container Images**: Images that are pre-pulled into the test cluster to speed up test execution
- **Helm Charts**: External Helm charts (Kai Scheduler, NVIDIA GPU Operator) with their versions and configuration

### Updating Dependencies

To update a dependency version:

1. Edit `e2e/dependencies.yaml`
2. Update the `version` field for the desired component
3. Run tests to verify: `cd e2e && go test -v`

#### Example: Updating Kai Scheduler

```yaml
helmCharts:
  kaiScheduler:
    releaseName: kai-scheduler
    chartRef: oci://ghcr.io/nvidia/kai-scheduler/kai-scheduler
    version: v0.9.4  # <- Update this version
    namespace: kai-scheduler
```

#### Example: Adding a New Image to Pre-pull

```yaml
images:
  # ... existing images ...
  - name: docker.io/myorg/myimage
    version: v1.2.3
```

### Why This Approach?

**Benefits:**
- ? **Centralized**: All versions in one place
- ? **Easy to Update**: Just edit YAML, no code changes needed
- ? **Type-Safe**: Go structs with validation
- ? **Testable**: Automated tests verify dependency loading
- ? **Self-Documenting**: YAML is human-readable and includes comments
- ? **Embedded**: Uses `go:embed` so dependencies are always available

**Similar to go.mod:**
- `go.mod` manages Go library dependencies
- `dependencies.yaml` manages E2E test infrastructure dependencies

## Architecture

```
e2e/
??? dependencies.yaml        # Dependency definitions (like go.mod)
??? dependencies.go          # Loader and types (embeds YAML at compile time)
??? dependencies_test.go     # Tests for dependency loading
??? setup/
?   ??? k8s_clusters.go      # Uses loaded dependencies
??? README.md                # This file
```

The `dependencies.go` file uses Go's `embed` package to embed `dependencies.yaml` directly into the binary at compile time:

```go
//go:embed dependencies.yaml
var dependenciesYAML []byte
```

This means:
- ? No runtime file path resolution needed
- ? Works regardless of where the binary is run from
- ? Dependencies are always available
- ? Clean and idiomatic Go code

## Testing

Run dependency tests:

```bash
cd e2e
go test -v
```

Run all e2e package tests:

```bash
go test ./e2e/...
```

## Troubleshooting

### Missing Images Warning

During cluster setup, the system checks if all deployed images are in the prepull list. If you see a warning about missing images:

```
??  Found 2 images not in prepull list. Consider adding these to e2e/dependencies.yaml:
    - some-new-image:v1.0.0
```

Add the missing images to `dependencies.yaml` to speed up future test runs.

## See Also

- [debugging-e2e-operator-logs.md](../docs/debugging-e2e-operator-logs.md) - Debugging E2E tests
- [general_index.md](../docs/general_index.md) - Codebase file index
