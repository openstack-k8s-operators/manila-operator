# AGENTS.md - manila-operator

## Project overview

manila-operator is a Kubernetes operator that manages
[OpenStack Manila](https://docs.openstack.org/manila/latest/) (the shared
file system service: provisioning and management of file shares over NFS,
CephFS, and other protocols) on OpenShift/Kubernetes. It is part of the
[openstack-k8s-operators](https://github.com/openstack-k8s-operators) project.

Key Manila domain concepts: **backends** (CephFS, CephNFS, NetApp),
**shares** (file system exports), **share types**, **scheduler**,
**DHSS** (driver handles share servers -- affects networking model),
**multi-backend** configurations.

Go module: `github.com/openstack-k8s-operators/manila-operator`
API group: `manila.openstack.org`
API version: `v1beta1`

## Tech stack

| Layer | Technology |
|-------|------------|
| Language | Go (modules, multi-module workspace via `go.work`) |
| Scaffolding | [Kubebuilder v4](https://book.kubebuilder.io/) + [Operator SDK](https://sdk.operatorframework.io/) |
| CRD generation | controller-gen (DeepCopy, CRDs, RBAC, webhooks) |
| Config management | Kustomize |
| Packaging | OLM bundle |
| Testing | Ginkgo/Gomega + envtest (functional), KUTTL (integration) |
| Linting | golangci-lint (`.golangci.yaml`) |
| CI | Zuul (`zuul.d/`), Prow (`.ci-operator.yaml`), GitHub Actions |

## Custom Resources

| Kind | Purpose |
|------|---------|
| `Manila` | Top-level CR. Owns the database, keystone service, transport URL, and spawns sub-CRs for each service component. |
| `ManilaAPI` | Manages the Manila API deployment (httpd/WSGI). |
| `ManilaScheduler` | Manages the scheduler service deployment. |
| `ManilaShare` | Manages share service instances (one per backend). |

The `Manila` CR has defaulting and validating admission webhooks.
Sub-CRs are created and owned by the `Manila` controller -- not intended to
be created directly by users.

## Directory structure

| Directory | Contents |
|-----------|----------|
| `api/v1beta1/` | CRD types (`manila_types.go`, `manilaapi_types.go`, `manilascheduler_types.go`, `manilashare_types.go`), conditions, webhook markers |
| `cmd/` | `main.go` entry point |
| `internal/controller/` | Reconcilers: `manila_controller.go`, `manilaapi_controller.go`, `manilascheduler_controller.go`, `manilashare_controller.go` |
| `internal/manila/` | Manila-level resource builders (db-sync, common helpers) |
| `internal/manilaapi/` | ManilaAPI resource builders |
| `internal/manilascheduler/` | ManilaScheduler resource builders |
| `internal/manilashare/` | ManilaShare resource builders |
| `internal/webhook/v1beta1/` | Webhook implementation |
| `templates/` | Config files and scripts mounted into pods via `OPERATOR_TEMPLATES` env var |
| `config/crd,rbac,manager,webhook/` | Generated Kubernetes manifests (CRDs, RBAC, deployment, webhooks) |
| `config/samples/` | Example CRs (Kustomize overlays). `backends/{cephfs,ceph-nfs,netapp,dhss-netapp,multibackend}` for storage. `layout/{cephfs,multibackend,tls}` for deployment topologies. |
| `test/functional/` | envtest-based Ginkgo/Gomega tests |
| `test/kuttl/` | KUTTL integration tests (basic, multibackend, TLS scenarios) |
| `hack/` | Helper scripts (CRD schema checker, local webhook runner) |
| `docs/` | Probes documentation |

## Build commands

After modifying Go code, always run: `make generate manifests fmt vet`.

## Code style guidelines

- Follow standard openstack-k8s-operators conventions and lib-common patterns.
- Use `lib-common` modules for conditions, endpoints, TLS, storage, and other
  cross-cutting concerns rather than re-implementing them.
- CRD types go in `api/v1beta1/`. Controller logic goes in
  `internal/controller/`. Resource-building helpers go in `internal/manila*`
  packages matching the CR they support.
- Config templates are plain files in `templates/` -- they are mounted at
  runtime via the `OPERATOR_TEMPLATES` environment variable.
- Webhook logic is split between the kubebuilder markers in `api/v1beta1/` and
  the implementation in `internal/webhook/v1beta1/`.

## Testing

- Functional tests use the envtest framework with Ginkgo/Gomega and live in
  `test/functional/`.
- KUTTL integration tests live in `test/kuttl/`.
- Run all functional tests: `make test`.
- When adding a new field or feature, add corresponding test cases in
  `test/functional/` and update `test/functional/manila_test_data.go` with
  fixture data.

## Key dependencies

- [lib-common](https://github.com/openstack-k8s-operators/lib-common): shared modules for conditions, endpoints, database, TLS, secrets, etc.
- [infra-operator](https://github.com/openstack-k8s-operators/infra-operator): RabbitMQ and topology APIs.
- [mariadb-operator](https://github.com/openstack-k8s-operators/mariadb-operator): database provisioning.
- [keystone-operator](https://github.com/openstack-k8s-operators/keystone-operator): identity service registration.
- [gophercloud](https://github.com/gophercloud/gophercloud): Go OpenStack SDK (indirect).
