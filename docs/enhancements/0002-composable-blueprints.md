# NEP-0002: Composable Blueprints

| Field | Value |
|-------|-------|
| **Title** | Composable Blueprints for Service Catalog |
| **Status** | Proposal |
| **Authors** | Fabien Dupont |
| **Created** | 2026-03-28 |
| **Updated** | 2026-04-10 |
| **Target Release** | TBD |
| **Depends on** | NEP-0001 (Extensible Architecture) |

## Summary

Replace the flat service template model in the NICo catalog provider
with a unified blueprint abstraction. A blueprint is a directed
acyclic graph (DAG) of NICo resources that declares what to
provision, in what order, and under what conditions. Blueprints can
embed other blueprints, enabling reuse across service tiers.

**Blueprint is the universal catalog entity.** Every item in the
catalog — from a single GPU slice to a multi-node training cluster
— is a blueprint. An atomic blueprint wraps a single resource (or a
small group of related resources) and hides infrastructure details.
A composed blueprint references other blueprints. There is no
separate "template" or "SKU" entity; the blueprint is the SKU.

**Pricing lives on blueprints, not resources.** A blueprint carries
an optional pricing specification (rate, unit, currency, billing
interval). Composed blueprints derive their cost by summing the
prices of their constituent blueprints. Individual NICo resource
types (`nico/vpc`, `nico/instance`) do not carry prices.

**Tenants can author blueprints.** A tenant admin with the
`blueprint-author` role can compose new blueprints from published
provider blueprints, or take an existing blueprint and create a
variant with locked parameter values to enforce organizational
practices. Tenant blueprints are scoped to their organization.

This turns the catalog provider into a declarative service definition
layer, the fulfillment provider into a generic graph executor, and
exposes the permission model needed for role-based catalog access.

## Motivation

### Current state

NEP-0001 introduced the catalog provider with `ServiceTemplate` — a
flat list of parameters and an opaque fulfillment workflow. The
fulfillment provider contains hardcoded provisioning logic per
template type. Adding a new service tier requires writing new Go
code in the fulfillment provider.

### Problems

1. **Templates cannot express dependencies.** A GPU training cluster
   needs a VPC before subnets, subnets before instances, and
   instances before InfiniBand partitions. This ordering lives in
   Go code, not in the template definition.

2. **Templates cannot compose.** A "GPU Cluster with Monitoring"
   cannot reuse an existing "Observability Stack" template as a
   building block. Every combination requires new fulfillment code.

3. **Templates are opaque to tenants.** The tenant sees parameter
   inputs and a status badge. They cannot preview what resources
   will be created, what permissions are required, or how the
   provisioning sequence works.

4. **Roles cannot be derived from templates.** An NCP admin must
   manually define RBAC roles. There is no way to say "this role
   grants access to order from this template" because the template
   does not declare its permission requirements.

5. **No path to existing IaC tooling.** NCPs who use Terraform or
   OpenTofu cannot leverage their existing workflows because the
   template model has no resource graph to translate.

6. **No tenant self-service.** Tenants cannot define their own
   service tiers or enforce organizational defaults. Every
   customization requires the NCP admin to create a new template.

7. **No pricing model.** Templates carry no cost information.
   There is no way for tenants to estimate costs before ordering,
   and no foundation for billing or showback.

### Desired state

Blueprints are the universal catalog entity and the native unit of
service definition. Every catalog item — atomic or composed — is a
blueprint. They declare resource graphs using NICo's feature
vocabulary. The fulfillment provider executes any blueprint without
custom code. Roles are derivable from blueprints. Pricing is
attached to blueprints. Tenants can compose their own blueprints
from published building blocks. Multiple input formats (native YAML,
OpenTofu HCL) compile to the same execution model.

## Design

### Blueprint schema

A blueprint is a JSON/YAML document with three sections:
parameters, resources, and metadata.

```yaml
name: GPU Training Cluster
version: 1.2.0
description: >
  Multi-node GPU cluster with InfiniBand interconnect
  for distributed training workloads.

parameters:
  gpu_count:
    type: integer
    description: Number of GPUs (A100 80GB)
    required: true
    default: 8
    min: 1
    max: 64
  cluster_name:
    type: string
    description: Name for the provisioned cluster
    required: true
  isolation:
    type: string
    description: Tenant isolation level
    required: true
    default: namespace
    enum: [namespace, cluster, bare-metal]
  storage_tier:
    type: string
    default: standard
    enum: [standard, premium, ultra]
  enable_rdma:
    type: boolean
    default: true

resources:
  vpc:
    type: nico/vpc
    properties:
      name: "{{ cluster_name }}-vpc"

  compute-subnet:
    type: nico/subnet
    depends_on: [vpc]
    properties:
      vpc: "{{ vpc.id }}"
      cidr: 10.100.1.0/24

  gpu-workers:
    type: nico/instance
    count: "{{ gpu_count / 8 }}"
    depends_on: [compute-subnet]
    properties:
      vpc: "{{ vpc.id }}"
      subnet: "{{ compute-subnet.id }}"
      instance_type: gpu-8xa100-80g

  ib-partition:
    type: nico/infiniband-partition
    condition: "{{ gpu_count > 8 }}"
    depends_on: [gpu-workers]
    properties:
      instances: "{{ gpu-workers[*].id }}"

  monitoring:
    type: blueprint/observability-stack
    depends_on: [gpu-workers]
    properties:
      targets: "{{ gpu-workers[*].id }}"

labels:
  category: training
  tier: enterprise
```

### Resource types

Every NICo feature exposes one or more resource types. The
available types are derived from the `/v2/capabilities` endpoint:

| Feature | Resource types |
|---|---|
| networking | `nico/vpc`, `nico/subnet`, `nico/network-security-group`, `nico/vpc-peering` |
| compute | `nico/instance`, `nico/allocation` |
| networking (IB) | `nico/infiniband-partition` |
| networking (NVLink) | `nico/nvlink-partition` |
| dns | `nico/dns-zone`, `nico/dns-record` |
| site | `nico/site` |
| catalog | `blueprint/*` (nested blueprint references) |

Resource types follow the convention `nico/{resource}` for
built-in NICo resources and `blueprint/{name}` for composable
sub-blueprints.

### Dependencies and ordering

Resources declare dependencies through two mechanisms:

1. **Implicit references.** An expression like `{{ vpc.id }}` in
   a property creates an implicit dependency on the `vpc` resource.
   The executor resolves these before starting the dependent resource.

2. **Explicit `depends_on`.** An array of resource IDs that must
   complete before this resource starts, even without property
   references. Useful for ordering constraints that are not
   data-driven (e.g., "wait for DNS before starting instances").

The executor topologically sorts all resources, groups independent
resources for parallel execution, and evaluates `condition`
expressions at runtime after dependencies resolve.

### Conditions

A resource with a `condition` expression is only created when the
expression evaluates to true. Conditions are evaluated after all
`depends_on` resources complete, so they can reference resolved
values from parent resources.

```yaml
ib-partition:
  type: nico/infiniband-partition
  condition: "{{ gpu_count > 8 }}"
  depends_on: [gpu-workers]
```

### Composition

A blueprint can embed another blueprint as a resource with type
`blueprint/{name}`. The embedded blueprint's parameters are
populated from the parent's `properties`. The executor recursively
expands sub-blueprints into the resource graph before execution.

```yaml
monitoring:
  type: blueprint/observability-stack
  depends_on: [gpu-workers]
  properties:
    targets: "{{ gpu-workers[*].id }}"
```

This enables layered service definitions:

```
blueprint/gpu-training-cluster
├── nico/vpc
├── nico/subnet
├── nico/instance (x N)
├── nico/infiniband-partition (conditional)
└── blueprint/observability-stack
    ├── nico/instance (prometheus)
    └── nico/vpc-peering
```

### Atomic blueprints

An atomic blueprint wraps one or more tightly coupled resources
into a single catalog entry. It hides infrastructure complexity
from tenants — the consumer sees "4×A100 GPU" and orders it; the
blueprint creates the instances, NVLink domain, security group,
and fabric subnet transparently.

```yaml
name: GPU 4xA100 Slice
version: 1.0.0
description: >
  Four A100 80GB GPUs with NVLink interconnect.
  Includes dedicated compute subnet and security group.

pricing:
  rate: 10.00
  unit: hour
  currency: USD

parameters:
  name:
    type: string
    required: true
    description: Name for the GPU slice

resources:
  subnet:
    type: nico/subnet
    properties:
      cidr: 10.200.0.0/28

  security-group:
    type: nico/network-security-group
    depends_on: [subnet]
    properties:
      rules:
        - protocol: tcp
          port: 22
          source: 10.0.0.0/8

  gpu-0:
    type: nico/instance
    depends_on: [subnet, security-group]
    properties:
      instance_type: gpu-a100-80g
      subnet: "{{ subnet.id }}"
      security_group: "{{ security-group.id }}"

  gpu-1:
    type: nico/instance
    depends_on: [subnet, security-group]
    properties:
      instance_type: gpu-a100-80g
      subnet: "{{ subnet.id }}"
      security_group: "{{ security-group.id }}"

  gpu-2:
    type: nico/instance
    depends_on: [subnet, security-group]
    properties:
      instance_type: gpu-a100-80g
      subnet: "{{ subnet.id }}"
      security_group: "{{ security-group.id }}"

  gpu-3:
    type: nico/instance
    depends_on: [subnet, security-group]
    properties:
      instance_type: gpu-a100-80g
      subnet: "{{ subnet.id }}"
      security_group: "{{ security-group.id }}"

  nvlink:
    type: nico/nvlink-partition
    depends_on: [gpu-0, gpu-1, gpu-2, gpu-3]
    properties:
      instances:
        - "{{ gpu-0.id }}"
        - "{{ gpu-1.id }}"
        - "{{ gpu-2.id }}"
        - "{{ gpu-3.id }}"

labels:
  category: compute
  gpu: a100
  tier: standard
```

The tenant sees a single catalog card: "GPU 4×A100 Slice — $10/hr".
They do not see or manage the subnet, security group, or NVLink
partition. Those are implementation details of the blueprint.

This means even the simplest resource offering goes through the
blueprint abstraction. The catalog has one entity type. The UI
has one browsing experience. The ordering flow is always the same.

### Pricing

Pricing is an optional section on a blueprint. It describes the
cost of ordering and running the blueprint's resources.

```yaml
pricing:
  rate: 10.00
  unit: hour        # hour, month, one-time
  currency: USD
  billing_interval: 3600  # seconds between billing ticks (optional)
```

#### Cost derivation for composed blueprints

When a blueprint composes other blueprints, its cost is the sum
of the constituent blueprints' prices unless an explicit `pricing`
section overrides it. This allows provider admins to bundle at a
discount or apply markup.

```
blueprint/ml-workstation ($15/hr)
├── blueprint/gpu-4xa100     → $10/hr
├── blueprint/storage-200gb  → $3/hr
└── blueprint/pytorch-stack  → $2/hr
                        sum  = $15/hr (or override with pricing.rate)
```

Tenants see the total price on the catalog card. A cost preview
endpoint returns the estimated cost given specific parameters:

```
POST /catalog/blueprints/:id/estimate
{ "parameters": { "gpu_count": 4, "storage_tier": "premium" } }
→ { "estimated_rate": 18.50, "unit": "hour", "currency": "USD",
    "breakdown": [
      { "blueprint": "gpu-4xa100", "rate": 10.00 },
      { "blueprint": "storage-500gb", "rate": 6.50 },
      { "blueprint": "pytorch-stack", "rate": 2.00 }
    ] }
```

### Tenant blueprint authoring

Tenant admins with the `blueprint-author` role can create
blueprints scoped to their organization. They operate under two
constraints:

1. **Blueprints-only composition.** Tenant blueprints can only
   reference published provider blueprints (type `blueprint/*`),
   never raw resource types (`nico/*`). This ensures tenants
   cannot create arbitrary infrastructure — they compose from
   sanctioned building blocks.

2. **Permission validation at creation time.** When a tenant
   creates a blueprint, the catalog validates that the tenant's
   role grants permissions for all resource types transitively
   used by the referenced blueprints.

#### Variant blueprints with locked parameters

A tenant admin can take an existing blueprint and create a
variant that locks specific parameter values. This enforces
organizational practices without requiring provider admin
involvement.

```yaml
name: Corp ML Workstation
version: 1.0.0
description: >
  Standard ML workstation with corporate security defaults.

based_on: blueprint/ml-workstation@1.2.0

parameter_overrides:
  security_group:
    value: sg-corp-standard
    locked: true            # cannot be changed at order time
  storage_tier:
    value: premium
    locked: true
  name:
    locked: false           # user provides this at order time
    default: "ml-workstation"
```

The `based_on` field references a specific version of the parent
blueprint. The variant inherits all parameters and resources from
the parent. `parameter_overrides` can:

- **Lock a value** (`locked: true`): the parameter is set at
  blueprint creation and cannot be changed when ordering.
- **Set a default** (`locked: false`): the parameter has a
  pre-filled value but the user can change it.
- **Hide a parameter**: locked parameters are not shown in the
  ordering form.

This is a thin composition — the DAG is just one node
(`blueprint/ml-workstation@1.2.0`) with some parameters pinned.
The tenant admin does not need to understand the underlying
resources.

#### Blueprint ownership and visibility

Each blueprint carries ownership and visibility metadata:

```yaml
# Added by the system, not user-authored
owner:
  tenant_id: uuid           # null for provider-published blueprints
  author_id: uuid           # user who created it
visibility: public          # public, organization, private
```

| Visibility | Who can see it | Who can order from it |
|---|---|---|
| `public` | All tenants | All tenants with required permissions |
| `organization` | Same tenant/org only | Same tenant/org with required permissions |
| `private` | Author only | Author only |

Provider-published blueprints are always `public` with no
`tenant_id`. Tenant-authored blueprints default to `organization`.

#### Composition depth and resource limits

To prevent runaway complexity:

- Maximum nesting depth: **5 levels** (configurable per deployment)
- Maximum total resources after DAG expansion: **100** (configurable)
- Both limits are validated at blueprint creation time, not at
  order time, so tenants get immediate feedback

#### Version pinning

Blueprint composition references include a version pin:

```yaml
resources:
  gpu:
    type: blueprint/gpu-4xa100@1.0.0
```

If the provider publishes `gpu-4xa100@1.1.0`, existing tenant
blueprints referencing `@1.0.0` continue to work unchanged.
The catalog API reports when a newer version is available so
tenant admins can update at their own pace.

When a provider deprecates a blueprint version, tenant blueprints
referencing it are marked `needs-update` in the catalog. They
remain functional until the version is removed.

### Temporal execution model

Blueprint execution maps to Temporal's child workflow pattern:

1. **Blueprint Workflow** (parent) receives the order with
   resolved parameters.
2. It topologically sorts the resource graph.
3. Independent resources are grouped for parallel execution.
4. Each resource becomes a **child workflow** dispatched to the
   appropriate feature provider's task queue.
5. Resolved outputs (IDs, addresses) are passed to dependent
   resources via expression evaluation.
6. Conditions are evaluated at runtime after dependencies resolve.
7. Sub-blueprints are expanded and executed as nested parent
   workflows.
8. On failure, compensating actions run in reverse topological
   order (teardown).

```
Blueprint Workflow (parent)
│
├─ Step 1: CreateVPC            ← networking task queue
│
├─ Step 2: CreateSubnet         ← networking task queue
│
├─ Step 3 (parallel):
│   ├─ CreateInstance × N       ← compute task queue
│   └─ CreateInstance × N
│
├─ Step 4 (conditional):
│   └─ CreateIBPartition        ← networking task queue
│
└─ Step 5: Execute sub-blueprint
    └─ observability-stack workflow (recursive)
```

### RBAC model

Each NICo resource type has associated permissions:

| Resource type | Permissions |
|---|---|
| `nico/vpc` | `vpc:create`, `vpc:read`, `vpc:delete` |
| `nico/instance` | `instance:create`, `instance:read`, `instance:delete` |
| `nico/subnet` | `subnet:create`, `subnet:read`, `subnet:delete` |

A blueprint's required permissions are the **union** of all
resource permissions it uses, including recursively embedded
sub-blueprints:

```json
{
  "permissions": [
    { "resource": "vpc", "actions": ["create", "read", "delete"] },
    { "resource": "subnet", "actions": ["create", "read", "delete"] },
    { "resource": "instance", "actions": ["create", "read", "delete"] },
    { "resource": "infiniband-partition", "actions": ["create", "read", "delete"] }
  ]
}
```

#### Role generation from blueprints

An NCP admin can generate a Keycloak role from a blueprint:

```
POST /v2/org/{org}/carbide/catalog/blueprints/{id}/generate-role
→ { "role_name": "gpu-training-cluster-operator", "permissions": [...] }
```

This creates a Keycloak role that grants exactly the permissions
needed to order from this blueprint. The admin can then assign
this role to tenant users.

#### Catalog permission visibility

The catalog UI shows permission status per blueprint:

- **Green lock**: user has all required permissions
- **Orange warning**: user has partial permissions
- **Red lock**: user lacks critical permissions

Missing permissions are listed in a tooltip so the user knows
what to request from their admin.

### Multi-format support

The native blueprint format is JSON/YAML. The fulfillment
provider compiles it to a Temporal workflow DAG. Other input
formats can compile to the same intermediate representation:

```
                    ┌─────────────┐
  YAML blueprint ──→│             │
                    │  Blueprint  │──→ Temporal Workflow DAG
  OpenTofu HCL  ──→│  Compiler   │    (the only execution path)
                    │             │
  TOSCA (future) ──→│             │
                    └─────────────┘
```

#### OpenTofu bridge (future phase)

A NICo OpenTofu provider would expose each resource type as a
Terraform resource:

```hcl
resource "nico_vpc" "training" {
  name = "${var.cluster_name}-vpc"
}

resource "nico_instance" "gpu_worker" {
  count         = var.gpu_count / 8
  vpc_id        = nico_vpc.training.id
  instance_type = "gpu-8xa100-80g"
}
```

The fulfillment provider would execute OpenTofu modules as
Temporal activities, storing state in PostgreSQL.

## API changes

### Blueprint endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/catalog/blueprints` | List active blueprints (filtered by tenant visibility) |
| GET | `/catalog/blueprints/:id` | Get blueprint by ID |
| POST | `/catalog/blueprints` | Create blueprint (provider or tenant-scoped) |
| PATCH | `/catalog/blueprints/:id` | Update blueprint |
| DELETE | `/catalog/blueprints/:id` | Soft-delete blueprint |
| GET | `/catalog/resource-types` | Available resource types from capabilities |
| POST | `/catalog/blueprints/:id/validate` | Validate blueprint DAG |
| POST | `/catalog/blueprints/:id/estimate` | Cost preview with parameters |
| POST | `/catalog/blueprints/:id/generate-role` | Generate Keycloak role |

### Storefront endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/catalog/blueprints` | Browse catalog (tenant-filtered, with pricing) |
| POST | `/catalog/orders` | Order a blueprint |
| GET | `/catalog/orders` | List orders (tenant-filtered, paginated) |
| GET | `/catalog/orders/:id` | Get order status |
| DELETE | `/catalog/orders/:id` | Cancel order |
| GET | `/self/usage` | Tenant usage summary with cost breakdown |
| GET | `/self/quotas` | Tenant quota status |
| GET | `/self/permissions` | Current user's permissions |

### Deprecations

The `/catalog/templates` endpoints are deprecated. Existing
templates are migrated to blueprints with an empty `resources`
map. The fulfillment provider falls back to legacy hardcoded
logic when no resource graph is defined. Templates will be
removed in a future release.

## Implementation phases

| Phase | Scope |
|---|---|
| 1 | PostgreSQL persistence for catalog, fulfillment, showback stores |
| 2 | Blueprint schema with pricing, tenant ownership, visibility |
| 3 | Temporal graph executor (replaces hardcoded fulfillment) |
| 4 | Blueprint composition (nested blueprints, version pinning) |
| 5 | Tenant blueprint authoring (variant blueprints, locked parameters) |
| 6 | RBAC integration (permission derivation, role generation) |
| 7 | Storefront API (cost preview, order history, usage with cost) |
| 8 | OpenTofu bridge (optional, NICo Terraform provider) |

Phase 1 replaces in-memory stores with PostgreSQL-backed DAOs
following existing NICo patterns. Phase 2 extends the blueprint
model with pricing and tenant scoping. Phases 3-4 deliver the
execution engine. Phase 5 enables tenant self-service. Phase 6
adds RBAC enforcement. Phase 7 enriches the storefront API for
the portal UI. Phase 8 is optional.

## Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Blueprint schema too complex for NCPs | Low adoption | Atomic blueprints are simple; composition is opt-in |
| Circular blueprint references | Infinite expansion | Validate DAG at blueprint creation; reject cycles |
| Expression language injection | Security | Sandbox expression evaluator; no function calls, only property access |
| Unbounded composition depth | Resource explosion | Configurable max depth (5) and max resources (100), validated at creation |
| Tenant blueprint sprawl | Catalog noise | Visibility scoping; tenant blueprints invisible to other tenants |
| Version drift on base blueprints | Silent behavior changes | Version pinning (`@1.0.0`); deprecation warnings, not silent removal |
| Cost surprise on composed blueprints | Unexpected bills | Cost preview at creation and order time; confirmation above thresholds |
| RBAC permission explosion | Too many permissions per blueprint | Aggregate at resource-type level, not instance level |

## Prior art

| Project | Extension model | Relevance |
|---|---|---|
| Terraform | Resource graph + provider interface | DAG execution, dependency resolution |
| TOSCA | Node templates + relationships + workflows | Topology-aware orchestration |
| Crossplane | Composite resources from managed resources | K8s-native composition |
| AWS CloudFormation | Resource declarations + DependsOn | Template-driven provisioning |
| Helm | Go templates over K8s manifests | Parameterized deployment |

## References

- [NEP-0001: NICo Extensible Architecture](0001-extensible-architecture.md)
