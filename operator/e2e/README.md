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
