# NICo Extensible Architecture

## Overview

NICo's extensible architecture enables pluggable providers behind
core-defined feature interfaces. A small core provides identity,
multi-tenancy, API framework, workflow orchestration, and storage.
All domain functionality is implemented as providers that register
routes, workflows, and lifecycle hooks at startup.

## Core concepts

### Provider

A provider implements one or more features. It registers with the
core at startup and receives a `ProviderContext` with access to
shared infrastructure (database, Temporal, site client pool).

```go
type Provider interface {
    Name() string
    Version() string
    Features() []string
    Dependencies() []string
    Init(ctx ProviderContext) error
    Shutdown(ctx context.Context) error
}
```

Providers optionally implement additional interfaces:

- **APIProvider** — registers REST routes on the Echo server
- **WorkflowProvider** — registers Temporal workflows and activities
- **MigrationProvider** — brings its own database migrations

### Registry

The registry manages provider lifecycle:

1. **Registration** — providers declare name, features, dependencies
2. **Dependency resolution** — topological sort; error on circular
   or missing dependencies
3. **Initialization** — providers init in dependency order
4. **Shutdown** — reverse order

### Profiles

Profiles select which providers load at startup. Controlled by
the `NICO_PROFILE` environment variable.

| Profile | Providers | Use case |
|---------|-----------|----------|
| `management` | networking, compute, health, firmware, nvswitch | NVIDIA standalone |
| `site` | networking, compute, site, health, nvswitch | Site with agent |
| `management-with-site` | All core providers | Default deployment |
| `ncp` | All core + catalog, fulfillment, showback, dpf-hcp | Full NCP |

### Capability discovery

```
GET /v2/org/:orgName/carbide/capabilities
```

Returns the status of every known feature:

```json
{
  "features": {
    "compute":     {"status": "available", "provider": "nico-compute", "version": "1.0.6"},
    "networking":  {"status": "available", "provider": "nico-networking", "version": "1.0.6"},
    "storage":     {"status": "not_available"},
    "catalog":     {"status": "available", "provider": "nico-catalog", "version": "0.1.0"}
  }
}
```

Features without a registered provider return 501 Not Implemented
with a descriptive JSON body.

### Workflow hooks

Three-tier extensibility for provider integration:

**Sync hooks** run inline in activities. Pre-hooks can abort
operations by returning an error.

```go
registry.RegisterHook(provider.SyncHook{
    Feature: "compute",
    Event:   provider.EventPreCreateInstance,
    Handler: func(ctx context.Context, payload interface{}) error {
        // Return error to block instance creation
    },
})
```

**Async reactions** send Temporal signals to watcher workflows.
Non-blocking, durable (buffered if watcher is down).

```go
registry.RegisterReaction(provider.Reaction{
    Feature:        "compute",
    Event:          provider.EventPostCreateInstance,
    TargetWorkflow: "billing-watcher",
    SignalName:     "instance-created",
})
```

**Orchestration** — providers use Temporal child workflows
directly for multi-step provisioning sequences.

### Service interfaces

Cross-domain access goes through service interfaces in sub-packages:

- `providers/networking/networkingsvc` — VPC, Subnet, NSG, Interface, IPBlock operations
- `providers/compute/computesvc` — Instance, Machine, Allocation operations

Handlers use these interfaces instead of directly instantiating
DAOs from other domains. The interfaces can be backed by different
implementations (e.g., Netris instead of carbide-api for networking).

## Built-in providers

### Core infrastructure

| Provider | Features | Description |
|----------|----------|-------------|
| nico-networking | networking | VPC, Subnet, NSG, Interface, IB/NVLink, IPBlock, Fabric, DPU Extension |
| nico-compute | compute | Instance, Machine, Allocation, InstanceType, OS, SSHKey, SKU, Rack, Tray |
| nico-site | site | Site, ExpectedMachine, ExpectedPowerShelf, ExpectedSwitch |
| nico-health | health | Health checks (system routes stay in core) |
| nico-firmware | firmware | RLA, PowerShelf Manager |
| nico-nvswitch | nvswitch | NVSwitch Manager |

### Service delivery (NCP profile)

| Provider | Features | Description |
|----------|----------|-------------|
| nico-catalog | catalog | Service templates (GPU Training Cluster, etc.) |
| nico-fulfillment | fulfillment | Order management, multi-step provisioning workflows |
| nico-showback | showback | Per-tenant usage tracking, metering via hooks |

### Infrastructure extensions (NCP profile)

| Provider | Features | Description |
|----------|----------|-------------|
| nico-dpfhcp | dpf-hcp | DPU cluster provisioning via DPFHCPProvisioner operator |

## Complementary providers

The architecture supports providers that add capabilities
alongside existing features, reacting to events via hooks.

### Netris fabric management

`providers/netris-fabric/` syncs NICo tenant networking events
to the Netris SDN Controller for physical switch configuration.
It is **complementary** to nico-networking — Netris manages the
physical fabric, NICo manages tenant constructs.

- Reacts to `post-create-vpc` → creates Netris VPC (VRF) on switches
- Reacts to `post-create-subnet` → creates Netris VNET on switches
- Reacts to `post-create-instance` → configures switch port (VLAN, MTU)
- Validates IPAM via `pre-create-subnet` sync hook → prevents IP conflicts
- Reads credentials from `NETRIS_URL`, `NETRIS_USERNAME`, `NETRIS_PASSWORD`

Integration boundary with DPF:

| Layer | Controller |
|-------|-----------|
| Physical switches | Netris |
| DPU hardware + OS | DPF |
| Tenant networking | NICo |
| Workload networking | OpenShift OVN-K |

## Creating a new provider

### Minimal provider

```go
package myprovider

import (
    "context"
    "github.com/NVIDIA/ncx-infra-controller-rest/provider"
)

type MyProvider struct{}

func New() *MyProvider { return &MyProvider{} }

func (p *MyProvider) Name() string           { return "my-provider" }
func (p *MyProvider) Version() string        { return "0.1.0" }
func (p *MyProvider) Features() []string     { return []string{"my-feature"} }
func (p *MyProvider) Dependencies() []string { return nil }

func (p *MyProvider) Init(ctx provider.ProviderContext) error {
    // Access DB, Temporal, Config via ctx
    return nil
}

func (p *MyProvider) Shutdown(_ context.Context) error {
    return nil
}
```

### Adding REST endpoints

Implement `APIProvider`:

```go
func (p *MyProvider) RegisterRoutes(group *echo.Group) {
    prefix := p.apiPathPrefix
    group.GET(prefix+"/my-resource", p.handleList)
    group.POST(prefix+"/my-resource", p.handleCreate)
}
```

### Adding Temporal workflows

Implement `WorkflowProvider`:

```go
func (p *MyProvider) TaskQueue() string { return "my-tasks" }

func (p *MyProvider) RegisterWorkflows(w tsdkWorker.Worker) {
    w.RegisterWorkflow(MyProvisioningWorkflow)
}

func (p *MyProvider) RegisterActivities(w tsdkWorker.Worker) {
    w.RegisterActivity(&MyActivities{db: p.dbSession})
}
```

### Registering hooks

In the provider's `Init` method:

```go
func (p *MyProvider) Init(ctx provider.ProviderContext) error {
    // React to events from other providers
    ctx.Registry.RegisterReaction(provider.Reaction{
        Feature:        "compute",
        Event:          provider.EventPostCreateInstance,
        TargetWorkflow: "my-watcher",
        SignalName:     "instance-created",
    })

    // Gate operations in other providers
    ctx.Registry.RegisterHook(provider.SyncHook{
        Feature: "compute",
        Event:   provider.EventPreCreateInstance,
        Handler: p.validateBeforeInstanceCreate,
    })

    return nil
}
```

## Data model

Models stay in the shared `db/pkg/db/model/` package. This is
intentional — bun ORM `rel:belongs-to` fields serve dual purpose
(eager loading AND SQL JOINs for ORDER BY), so splitting them
across packages would break query functionality.

Cross-domain decoupling happens at the **service interface layer**,
not the model layer. Handlers access other domains through
`networkingsvc.Service` / `computesvc.Service` instead of directly
instantiating DAOs.

## Hook events

| Event | Feature | When fired |
|-------|---------|------------|
| `pre-create-instance` | compute | Before instance creation |
| `post-create-instance` | compute | After instance provisioning dispatched |
| `pre-delete-instance` | compute | Before instance deletion |
| `post-delete-instance` | compute | After instance deletion dispatched |
| `pre-create-vpc` | networking | Before VPC creation |
| `post-create-vpc` | networking | After VPC provisioning dispatched |
| `pre-delete-vpc` | networking | Before VPC deletion |
| `post-delete-vpc` | networking | After VPC deletion dispatched |
| `post-create-site` | site | After site reaches Registered state |
| `post-delete-site` | site | After site components deleted |
| `post-delete-site-components` | site | After site cascade deletion |
