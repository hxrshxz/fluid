# Fluid Proper Core Delivery File Set

## Goal
Define a realistic, explicit file-touch scope for proper core-module delivery toward >=75% coverage.

## Architecture
Scope is constrained to the user-approved core module roots only. The file list is explicit (no wildcards), and all listed existing files are in the repository.

## Tech Stack
Go, Ginkgo v2, Gomega, Fluid controllers/DDC/CSI/webhook/dataoperation packages.

## Migration Coding Quality Baseline

All migration coding quality, matcher style, async handling, cleanup discipline, and reviewer checks follow:
- `docs/plans/2026-03-03-fluid-ginkgo-gomega-migration-patterns-and-gotchas.md`

## A) Phased Schedule (11 Weeks + 1 Buffer)

| Phase | Weeks | Objective | Output |
|---|---|---|---|
| Phase 0 | W1 | Baseline freeze, framework audit, scope lock | Approved scope + baseline report |
| Phase 1 | W2-W4 | P0 controller/webhook/fuse migration wave | Core control-plane tests standardized |
| Phase 2 | W5-W6 | DDC base/factory + CSI + data-op status coverage | Critical data-path reliability uplift |
| Phase 3 | W7-W9 | Runtime controller + key engine expansion | P1 critical subset stabilized |
| Phase 4 | W10-W11 | API/dataoperation/test-gap closure + full verification | >=75% core coverage target |
| Buffer | Final week | Flake fixes, blockers, final evidence packaging | Mentor-ready final report |

## B) Framework Evaluation Criteria

Framework migration will be evaluated with practical review criteria:

- Tests are easier to read and maintain using Ginkgo/Gomega structure.
- Failure messages are clearer and help debug faster.
- Setup/teardown is consistent and avoids duplicated boilerplate.
- Test runtime and CI stability do not regress significantly.
- Flake rate is stable or improved.
- Team can review and contribute with consistent patterns.

Go/No-Go: proceed with full migration waves only if early pilot packages confirm stable CI behavior and improved test clarity.

## C) Coverage Milestones (Core Modules)

| Milestone | Time | Target |
|---|---|---:|
| M0 | End W1 | Baseline frozen (~57%) |
| M1 | End W4 | >=64% |
| M2 | End W6 | >=69% |
| M3 | End W9 | >=73% |
| M4 | End W11 | >=75% (required) |
| M5 | Buffer close | 76-78% safety band |

## 1) Final Scope Size

- Mandatory existing files to migrate/harden: 124
- Mandatory new test files to add: 12
- Total mandatory delivery file workload: 136

This includes both migration and missing-test additions for proper core delivery and stays in the planned execution range.

## 2) Explicit Mandatory File List

### Controllers Core
- `pkg/controllers/manager.go`
- `pkg/controllers/operation_controller.go`
- `pkg/controllers/runtime_controller.go`
- `pkg/controllers/deploy/runtime_controllers.go`
- `pkg/controllers/deploy/runtime_controllers_test.go`

### Controllers v1alpha1 (Dataset/FluidApp)
- `pkg/controllers/v1alpha1/dataset/dataset_controller.go`
- `pkg/controllers/v1alpha1/dataset/suite_test.go`
- `pkg/controllers/v1alpha1/fluidapp/fluidapp_controller.go`
- `pkg/controllers/v1alpha1/fluidapp/implement.go`
- `pkg/controllers/v1alpha1/fluidapp/implement_test.go`
- `pkg/controllers/v1alpha1/fluidapp/dataflowaffinity/dataflowaffinity_controller.go`

### Controllers v1alpha1 (Runtime Engines)
- `pkg/controllers/v1alpha1/alluxio/alluxio_runtime_controller.go`
- `pkg/controllers/v1alpha1/alluxio/implement.go`
- `pkg/controllers/v1alpha1/alluxio/suite_test.go`
- `pkg/controllers/v1alpha1/goosefs/goosefs_runtime_controller.go`
- `pkg/controllers/v1alpha1/goosefs/implement.go`
- `pkg/controllers/v1alpha1/goosefs/suite_test.go`
- `pkg/controllers/v1alpha1/jindo/jindoruntime_controller.go`
- `pkg/controllers/v1alpha1/jindo/implement.go`
- `pkg/controllers/v1alpha1/jindo/suite_test.go`
- `pkg/controllers/v1alpha1/juicefs/juicefsruntime_controller.go`
- `pkg/controllers/v1alpha1/juicefs/implement.go`
- `pkg/controllers/v1alpha1/juicefs/suite_test.go`
- `pkg/controllers/v1alpha1/thinruntime/thinruntime_controller.go`
- `pkg/controllers/v1alpha1/thinruntime/implement.go`
- `pkg/controllers/v1alpha1/thinruntime/suite_test.go`
- `pkg/controllers/v1alpha1/vineyard/vineyard_runtime_controller.go`
- `pkg/controllers/v1alpha1/vineyard/implement.go`
- `pkg/controllers/v1alpha1/efc/efcruntime_controller.go`
- `pkg/controllers/v1alpha1/efc/implement.go`
- `pkg/controllers/v1alpha1/efc/suite_test.go`

### Controllers v1alpha1 (Data Operations)
- `pkg/controllers/v1alpha1/dataload/dataload_controller.go`
- `pkg/controllers/v1alpha1/dataload/implement.go`
- `pkg/controllers/v1alpha1/dataload/status_handler.go`
- `pkg/controllers/v1alpha1/datamigrate/datamigrate_controller.go`
- `pkg/controllers/v1alpha1/datamigrate/implement.go`
- `pkg/controllers/v1alpha1/datamigrate/status_handler.go`
- `pkg/controllers/v1alpha1/databackup/databackup_controller.go`
- `pkg/controllers/v1alpha1/databackup/implement.go`
- `pkg/controllers/v1alpha1/databackup/status_handler.go`
- `pkg/controllers/v1alpha1/dataprocess/dataprocess_controller.go`
- `pkg/controllers/v1alpha1/dataprocess/implement.go`
- `pkg/controllers/v1alpha1/dataprocess/status_handler.go`
- `pkg/controllers/v1alpha1/dataflow/dataflow_controller.go`
- `pkg/controllers/v1alpha1/dataflow/operations.go`
- `pkg/controllers/v1alpha1/webhook/webhook_controller.go`

### Webhook Handler + Plugins
- `pkg/webhook/handler/mutating/mutating_handler.go`
- `pkg/webhook/handler/mutating/mutating_handler_test.go`
- `pkg/webhook/handler/mutating/webhook.go`
- `pkg/webhook/plugins/nodeaffinitywithcache/node_affinity_with_cache.go`
- `pkg/webhook/plugins/nodeaffinitywithcache/node_affinity_with_cache_test.go`
- `pkg/webhook/plugins/nodeaffinitywithcache/tiered_locaity.go`
- `pkg/webhook/plugins/fusesidecar/fuse_sidecar.go`
- `pkg/webhook/plugins/fusesidecar/fuse_sidecar_test.go`
- `pkg/webhook/plugins/requirenodewithfuse/require_node_with_fuse.go`
- `pkg/webhook/plugins/requirenodewithfuse/require_node_with_fuse_test.go`
- `pkg/webhook/plugins/prefernodeswithoutcache/prefer_nodes_without_cache.go`
- `pkg/webhook/plugins/prefernodeswithoutcache/prefer_nodes_without_cache_test.go`

### Fuse Injection
- `pkg/application/inject/fuse/container.go`
- `pkg/application/inject/fuse/injector.go`
- `pkg/application/inject/fuse/volume.go`
- `pkg/application/inject/fuse/mutator/mutator.go`

### DDC Base + Factory
- `pkg/ddc/base/dataset.go`
- `pkg/ddc/base/engine.go`
- `pkg/ddc/base/operation.go`
- `pkg/ddc/base/operation_helm.go`
- `pkg/ddc/base/runtime.go`
- `pkg/ddc/base/runtime_helper.go`
- `pkg/ddc/base/setup.go`
- `pkg/ddc/base/validate.go`
- `pkg/ddc/base/volume.go`
- `pkg/ddc/factory.go`

### CSI
- `pkg/csi/register.go`
- `pkg/csi/register_test.go`
- `pkg/csi/plugins/controller.go`
- `pkg/csi/plugins/driver.go`
- `pkg/csi/plugins/nodeserver.go`
- `pkg/csi/plugins/register.go`

### API v1alpha1
- `api/v1alpha1/common.go`
- `api/v1alpha1/constant.go`
- `api/v1alpha1/cacheruntime_types.go`
- `api/v1alpha1/dataset_types.go`
- `api/v1alpha1/dataload_types.go`
- `api/v1alpha1/datamigrate_types.go`
- `api/v1alpha1/databackup_types.go`
- `api/v1alpha1/dataprocess_types.go`
- `api/v1alpha1/alluxioruntime_types.go`
- `api/v1alpha1/goosefsruntime_types.go`
- `api/v1alpha1/jindoruntime_types.go`
- `api/v1alpha1/juicefsruntime_types.go`
- `api/v1alpha1/thinruntime_types.go`
- `api/v1alpha1/vineyardruntime_types.go`

### DDC Runtime Implementations
- `pkg/ddc/alluxio/engine.go`
- `pkg/ddc/alluxio/transform.go`
- `pkg/ddc/alluxio/api_gateway_test.go`
- `pkg/ddc/goosefs/engine.go`
- `pkg/ddc/goosefs/transform.go`
- `pkg/ddc/jindo/engine.go`
- `pkg/ddc/jindo/transform.go`
- `pkg/ddc/jindocache/engine.go`
- `pkg/ddc/jindocache/transform.go`
- `pkg/ddc/jindofsx/engine.go`
- `pkg/ddc/jindofsx/transform.go`
- `pkg/ddc/juicefs/engine.go`
- `pkg/ddc/juicefs/transform.go`
- `pkg/ddc/thin/engine.go`
- `pkg/ddc/thin/transform.go`
- `pkg/ddc/vineyard/engine.go`
- `pkg/ddc/vineyard/transform.go`
- `pkg/ddc/efc/engine.go`
- `pkg/ddc/efc/transform.go`

### Operation / Watch / Utils
- `pkg/dataoperation/interface.go`
- `pkg/dataoperation/constants.go`
- `pkg/ctrl/watch/manager.go`
- `pkg/ctrl/watch/runtime.go`
- `pkg/ctrl/watch/pod.go`
- `pkg/utils/kubeclient/pod.go`
- `pkg/utils/kubeclient/node.go`
- `pkg/utils/kubeclient/service.go`
- `pkg/utils/dataset/lifecycle/node.go`
- `pkg/utils/dataset/volume/create.go`
- `pkg/utils/dataset/volume/get.go`
- `pkg/utils/runtimes/options/init.go`
- `pkg/utils/runtimes/options/controller_sync_runtime.go`

### Mandatory New Test Files (Add)
- `pkg/controllers/runtime_controller_test.go` (new)
- `pkg/controllers/operation_controller_test.go` (new)
- `pkg/controllers/v1alpha1/dataset/dataset_controller_ut_test.go` (new)
- `pkg/controllers/v1alpha1/fluidapp/fluidapp_controller_ut_test.go` (new)
- `pkg/controllers/v1alpha1/dataflow/operations_test.go` (new)
- `pkg/controllers/v1alpha1/webhook/webhook_controller_test.go` (new)
- `pkg/dataoperation/interface_test.go` (new)
- `pkg/dataoperation/context_test.go` (new)
- `pkg/ddc/factory_test.go` (new)
- `pkg/csi/plugins/register_test.go` (new)
- `api/v1alpha1/common_test.go` (new)
- `api/v1alpha1/status_test.go` (new)

## 3) Weekly Touched-File Distribution (11 Weeks + Buffer)

| Week | Files |
|---|---:|
| Week 1 | 10 |
| Week 2 | 11 |
| Week 3 | 12 |
| Week 4 | 12 |
| Week 5 | 12 |
| Week 6 | 12 |
| Week 7 | 12 |
| Week 8 | 12 |
| Week 9 | 11 |
| Week 10 | 12 |
| Week 11 | 12 |
| Buffer | 8 |
| **Total** | **136** |

## 4) Post-LFX Scope (Toward 85%)

During LFX, the committed delivery is core stability and >=75% coverage. After this baseline is complete, coverage can be pushed to 85% in focused hotspot waves.

Execution model for 85%:
- Run coverage diff reports after the >=75% milestone.
- Pick low-coverage/high-impact hotspots first.
- Add targeted branch and error-path tests package by package.
- Keep no-Testify policy and continue Ginkgo/Gomega-only additions.

Suggested stretch milestones:
- Post-LFX phase 1: >=80%
- Post-LFX phase 2: >=85% with stable CI/flake profile
