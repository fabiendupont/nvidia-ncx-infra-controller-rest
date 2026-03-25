# NEP-0001: NICo Extensible Architecture

| Field | Value |
|-------|-------|
| **Title** | Extensible Architecture with Pluggable Providers |
| **Status** | Proposal |
| **Authors** | Red Hat NCP Team |
| **Created** | 2026-03-25 |
| **Target Release** | 1.1.0 |
| **Branch** | `extensible-architecture` (proof of concept) |

## Summary

Refactor NICo from a monolithic application into an extensible
platform: a small core providing identity, multi-tenancy, API
framework, workflow orchestration, and storage — with all domain
functionality implemented as pluggable providers behind
core-defined feature interfaces.

This enables NICo to serve as the single management plane for
NCP deployments, with built-in providers for NVIDIA hardware
management and a clear extension path for partner integrations
(storage, SDN, DCIM) and new capabilities (service catalog,
fulfillment, showback).

## Motivation

### Problems

1. **Partners can't extend NICo.** Adding a storage provider or
   SDN controller requires modifying NICo's codebase. Partners
   must build separate systems and integrate via REST API calls,
   creating operational complexity (two deployments, two auth
   systems, two databases).

2. **Service delivery requires a separate project.** Tenant
   lifecycle management, service catalog, and self-service
   capabilities must be built as a separate application (OSAC)
   because NICo has no extension mechanism. This doubles the
   operational surface.

3. **NCPs can't customize.** Every NCP has unique requirements
   (custom billing, regulatory compliance, specific DCIM).
   Without extensibility, these become bespoke integrations
   outside NICo.

4. **Gaps are opaque.** Features that don't exist (storage, DCIM,
   showback) are documented in external gap analyses. There's no
   runtime mechanism to discover what's available and what's not.

### Goals

- Enable partners to add features by implementing a Go interface
- Enable service delivery (catalog, fulfillment, showback) as
  built-in providers, not a separate project
- Enable runtime capability discovery via API
- Enable deployment profiles that select which features load
- Maintain full backward compatibility — same API, same DB, same
  Temporal workflows

### Non-goals

- Database schema changes
- Temporal workflow type name changes (running workflows must
  continue)
- Changes to the OpenAPI spec (structural refactor only)
- Go plugin mechanism (compile-time registration only)
- gRPC sidecar architecture for partner providers

## Design

### Architectural model

NICo's architecture is structurally equivalent to Kubernetes'
CRD + Controller pattern, but uses REST + Temporal instead:

| Concern | Kubernetes | NICo |
|---------|-----------|------|
| Schema definition | CRD (OpenAPI v3) | Feature interface + OpenAPI spec |
| State storage | etcd | PostgreSQL |
| CRUD operations | API server | REST handlers |
| Desired state | CR spec | POST/PUT request body |
| State transitions | Controller reconcile loop | Temporal workflow |
| Namespace scoping | Namespace | Org/Tenant |

### Core (always present, not pluggable)

The core provides six capabilities that every feature depends on:

1. **Identity** — JWT validation, Keycloak realm lifecycle, OIDC
2. **Multi-tenancy** — Org/Tenant CRUD, tenant-scoped queries
3. **API framework** — Echo server, auth middleware, RBAC, route
   registration
4. **Workflow framework** — Temporal client, worker management,
   context propagation
5. **Storage framework** — PostgreSQL connection pool, migration
   runner, audit logging
6. **Provider registry** — Loading, dependency resolution, feature
   mapping, hook dispatch

### Provider interface

```go
type Provider interface {
    Name() string
    Version() string
    Features() []string
    Dependencies() []string
    Init(ctx ProviderContext) error
    Shutdown(ctx context.Context) error
}

type APIProvider interface {
    Provider
    RegisterRoutes(group *echo.Group)
}

type WorkflowProvider interface {
    Provider
    TaskQueue() string
    RegisterWorkflows(w tsdkWorker.Worker)
    RegisterActivities(w tsdkWorker.Worker)
}

type MigrationProvider interface {
    Provider
    MigrationSource() fs.FS
}
```

### ProviderContext

What core provides to providers at initialization:

```go
type ProviderContext struct {
    DB                     *cdb.Session
    Temporal               tsdkClient.Client
    TemporalNS             tsdkClient.NamespaceClient
    SiteClientPool         *sc.ClientPool
    Config                 interface{}
    Registry               *Registry
    APIPathPrefix          string
    TemporalNamespace      string
    TemporalQueue          string
    WorkflowSiteClientPool interface{}
    Hooks                  *HookRunner
}
```

### Registry

The registry manages provider lifecycle with four operations:

1. **Register** — store provider, map features, detect conflicts
2. **ResolveDependencies** — topological sort; error on circular
   or missing deps
3. **InitAll** — initialize in dependency order
4. **ShutdownAll** — shutdown in reverse order

### Deployment profiles

Profiles select which providers load, controlled by `NICO_PROFILE`:

| Profile | Providers | Use case |
|---------|-----------|----------|
| `management` | networking, compute, health, firmware, nvswitch | NVIDIA standalone |
| `site` | networking, compute, site, health, nvswitch | Site agent |
| `management-with-site` | All core | Default |
| `ncp` | All core + catalog, fulfillment, showback, dpf-hcp | Full NCP |

### Capability discovery

```
GET /v2/org/:orgName/carbide/capabilities
```

Returns status of all known features. Two states: `available`
(provider registered) or `not_available` (no provider, returns
501). UIs adapt dynamically.

### Service interfaces for cross-domain access

Handlers access other domains through service interfaces, not
direct DAO instantiation:

```go
// In providers/networking/networkingsvc/
type Service interface {
    GetVpcByID(ctx, tx, id) (*cdbm.Vpc, error)
    GetSubnets(ctx, tx, filter, page) ([]cdbm.Subnet, int, error)
    // ... 30 methods total
}

// In providers/compute/computesvc/
type Service interface {
    GetInstanceByID(ctx, tx, id) (*cdbm.Instance, error)
    GetInstances(ctx, tx, filter, page) ([]cdbm.Instance, int, error)
    GetAllocationsCount(ctx, tx, filter) (int, error)
    GetMachineByID(ctx, tx, id) (*cdbm.Machine, error)
}
```

Service interfaces live in sub-packages (`networkingsvc/`,
`computesvc/`) to avoid import cycles between handler and
provider packages.

### Workflow hooks

Three-tier extensibility for loose coupling between providers:

**Tier 1: Sync hooks (validation)**

Run inline in activities. Pre-hooks can abort operations.

```go
registry.RegisterHook(SyncHook{
    Feature: "compute",
    Event:   EventPreCreateInstance,
    Handler: quotaCheck,
})
```

**Tier 2: Async reactions (Temporal signals)**

Send signals to watcher workflows. Non-blocking, durable.

```go
registry.RegisterReaction(Reaction{
    Feature:        "compute",
    Event:          EventPostCreateInstance,
    TargetWorkflow: "billing-watcher",
    SignalName:     "instance-created",
})
```

**Tier 3: Orchestration (child workflows)**

Providers use Temporal directly for multi-step sequences.

### Data model strategy

Models stay in the shared `db/pkg/db/model/` package. This is
intentional — bun ORM `rel:belongs-to` fields serve dual purpose:

1. **Eager loading** — `Relation("Vpc")` loads the VPC with the
   instance in a single query
2. **SQL JOINs** — `orderBy=network_security_group.name` requires
   a JOIN that bun performs via the struct relation

Removing struct fields would break ORDER BY on related columns.
Decoupling happens at the service interface layer instead.

#### Analysis

- 42 models, 92 `belongs-to` relations, zero `has-many`
- 6 cross-domain struct references (networking↔compute)
- Core entities: Site (21 refs), Tenant (18), InfraProvider (14)
- All cross-domain handler DAO calls migrated to service interfaces

## Implementation

### What changes

| Component | Before | After |
|-----------|--------|-------|
| `api/pkg/api/routes.go` | 922 lines, 80 routes | 145 lines, 19 core routes |
| `workflow/cmd/workflow/main.go` | 503 lines, 60 imports | 347 lines, ~10 imports |
| `api/cmd/api/main.go` | Direct initialization | Provider registry + profiles |
| `api/internal/server/server.go` | All routes registered | Core + provider routes |
| `api/internal/config/` | Internal package | `api/pkg/config/` (public) |
| `workflow/internal/config/` | Internal package | `workflow/pkg/config/` (public) |
| Handler cross-domain calls | Direct DAO instantiation | Service interfaces |

### What stays the same

- All REST API routes, paths, request/response schemas
- All HTTP status codes
- All Temporal workflow type names and task queue names
- All database tables and schemas
- All OpenAPI spec
- Auth middleware, RBAC enforcement
- Site agent communication (gRPC to carbide-api)

### Phased implementation

**Phase 1: Provider framework** (additive, no behavior changes)

Create `provider/` package with interfaces, registry, dependency
resolution, capability endpoint, stubs. All new files.

**Phase 2: Service interfaces** (additive)

Create `providers/networking/networkingsvc/` and
`providers/compute/computesvc/` with service interfaces and SQL
implementations. New files only.

**Phase 3: Route migration** (behavioral, backward compatible)

Move route registration from monolithic `routes.go` into
per-provider `RegisterRoutes`. Same handlers, same paths.
`routes.go` shrinks to core routes only.

**Phase 4: Workflow migration** (behavioral, backward compatible)

Move workflow/activity registration from monolithic `main.go`
into per-provider `RegisterWorkflows`/`RegisterActivities`.
Core workflows (User, Tenant) stay in main.

**Phase 5: Handler migration** (internal, no API changes)

Replace direct cross-domain DAO calls in handlers with service
interface calls. Same behavior, different call path.

**Phase 6: Hooks** (additive)

Add sync and async hook system. Fire hooks in key activities.
No hooks registered by default — no behavior change until a
provider registers one.

**Phase 7: New providers** (additive)

Add catalog, fulfillment, showback, dpf-hcp as NCP-only
providers. New routes, new features, registered in NCP profile.

### Testing strategy

Each phase must pass:
- `go build ./...` (compilation)
- `make test` (full test suite with PostgreSQL)
- `diff openapi/spec.yaml openapi/spec.yaml.baseline` (API compat)

Pre-existing test failures (infrastructure-dependent tests
requiring Temporal/Keycloak) are baselined and excluded from
regression comparison.

## Provider catalog

### Built-in (NVIDIA)

| Provider | Features | Profile |
|----------|----------|---------|
| nico-networking | networking | All |
| nico-compute | compute | All |
| nico-site | site | site, management-with-site, ncp |
| nico-health | health | All |
| nico-firmware | firmware | management, management-with-site, ncp |
| nico-nvswitch | nvswitch | All |

### Service delivery (NCP)

| Provider | Features | Profile |
|----------|----------|---------|
| nico-catalog | catalog | ncp |
| nico-fulfillment | fulfillment | ncp |
| nico-showback | showback | ncp |
| nico-dpfhcp | dpf-hcp | ncp |

### Partner (examples)

Providers can be **complementary** (adding a new capability
alongside existing features) or **alternative** (replacing an
existing feature with a different backend).

| Provider | Features | Type | Description |
|----------|----------|------|-------------|
| netris-fabric | fabric | Complementary | Syncs NICo VPC/Subnet events to Netris Controller for physical switch configuration. Runs alongside nico-networking, not instead of it. |
| vast-storage | storage | New feature | Volume management via VAST Data API |
| weka-storage | storage | New feature | Parallel filesystem via WEKA API |
| infoblox-dns | dns | Alternative | DNS management via Infoblox instead of built-in |
| netbox-dcim | dcim | New feature | Asset tracking via NetBox |

#### Netris fabric integration model

Netris manages the **physical switch fabric** (Spectrum-X, SONiC,
Arista) — a layer below NICo's tenant networking. The integration
is event-driven via hooks:

```
NICo creates VPC    →  post-create-vpc hook  →  Netris creates VPC (VRF) on switches
NICo creates Subnet →  post-create-subnet   →  Netris creates VNET on switches
NICo provisions Instance → post-create-instance → Netris configures switch port
NICo creates Subnet →  pre-create-subnet    →  Validate no IPAM conflict with Netris
```

This model avoids the "three competing control planes" problem
(OpenShift, DPF, Netris) by giving each layer a clear boundary:

| Layer | Controller |
|-------|-----------|
| Physical switches | Netris |
| DPU hardware + OS | DPF |
| Tenant networking (VPC/Subnet) | NICo |
| Workload networking (pods) | OpenShift OVN-K |

## Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| API behavior change during refactor | Breaks clients | Diff OpenAPI spec before/after each phase |
| Temporal workflow name change | Running workflows fail | Workflow functions stay in same packages |
| Import cycles (handler↔provider) | Build failure | Service interfaces in sub-packages |
| Internal package visibility | Providers can't import config | Move to public packages |
| Cross-domain ORDER BY breaks | Sorting on related fields fails | Keep models in shared package |
| Performance regression (service layer) | Slower cross-domain queries | Service methods delegate directly to DAOs |
| Profile misconfiguration | Missing features at runtime | Dependency resolution at startup |

## Alternatives considered

### Go plugin mechanism

Go's `plugin` package allows loading `.so` files at runtime.
Rejected because:
- Fragile (exact Go version, same build flags required)
- Poor debugging experience
- No static type checking between plugin and host

### gRPC sidecar architecture

Partner providers run as separate processes, communicating with
NICo core via gRPC. Rejected because:
- Adds deployment complexity (separate containers, networking)
- Loses transaction boundaries (DB operations can't be atomic)
- Higher latency for in-process operations
- Duplicates auth/tenancy infrastructure

### Compile-time registration (chosen)

Providers are Go packages imported in `main.go` and selected via
profiles. Requires recompiling to add a provider but:
- Full static type safety
- Zero runtime overhead
- Same debugging experience as monolith
- Transaction boundaries preserved
- Auth/tenancy inherited from core middleware

A future enhancement could add config-file-driven profile
selection (which compiled-in providers to activate) without
changing the compile-time registration model.

### Microservices decomposition

Split NICo into separate services per domain. Rejected because:
- NICo operates below Kubernetes (can't depend on K8s for
  service discovery)
- Cross-domain transactions (instance creation touches VPC,
  Machine, Subnet) would require saga patterns
- Multiplies operational complexity

## Proof of concept

The `extensible-architecture` branch contains a working
implementation with 16 commits, demonstrating:

- 11 provider packages (6 core + 3 OSAC + 1 partner + 1 DPF HCP)
- Full route migration (922→145 lines in routes.go)
- Full workflow migration (503→347 lines in main.go)
- 55 cross-domain handler calls migrated to service interfaces
- 3-tier workflow hooks with firing in instance/VPC/site activities
- Netris SDN as an alternative networking provider
- DPF HCP provisioner with K8s dynamic client and Temporal workflows
- Zero test regressions against `main` branch

## References

- [NICo Extensible Architecture Design](../extensible-architecture.md)
- [DPF HCP Provider Documentation](../dpf-hcp-provider.md)
- [dpf-hcp-provisioner-operator](https://github.com/rh-ecosystem-edge/dpf-hcp-provisioner-operator)
- [Kubernetes Enhancement Proposals](https://github.com/kubernetes/enhancements)
