# NEP-0002: Composable Blueprints

| Field | Value |
|-------|-------|
| **Title** | Composable Blueprints for Service Catalog |
| **Status** | Proposal |
| **Authors** | Fabien Dupont |
| **Created** | 2026-03-28 |
| **Target Release** | TBD |
| **Depends on** | NEP-0001 (Extensible Architecture) |

## Summary

Replace the flat service template model in the NICo catalog provider
with composable blueprints. A blueprint is a directed acyclic graph
(DAG) of NICo resources that declares what to provision, in what
order, and under what conditions. Blueprints can embed other
blueprints, enabling reuse across service tiers.

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

### Desired state

Blueprints are the native unit of service definition. They declare
resource graphs using NICo's feature vocabulary. The fulfillment
provider executes any blueprint without custom code. Roles are
derivable from blueprints. Multiple input formats (native YAML,
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

### New endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/catalog/blueprints` | List active blueprints |
| GET | `/catalog/blueprints/:id` | Get blueprint by ID |
| POST | `/catalog/blueprints` | Create blueprint |
| PATCH | `/catalog/blueprints/:id` | Update blueprint |
| DELETE | `/catalog/blueprints/:id` | Soft-delete blueprint |
| GET | `/catalog/resource-types` | Available resource types from capabilities |
| POST | `/catalog/blueprints/:id/validate` | Validate blueprint graph |
| POST | `/catalog/blueprints/:id/generate-role` | Generate Keycloak role |
| GET | `/self/permissions` | Current user's permissions |

### Backward compatibility

The existing `/catalog/templates` endpoints remain as aliases.
A template is a blueprint with an empty `resources` array — the
fulfillment provider falls back to the legacy hardcoded logic
when no resource graph is defined.

## Implementation phases

| Phase | Scope |
|---|---|
| 1 | Blueprint schema, API endpoints, mock data, UI designer |
| 2 | Temporal graph executor (replaces hardcoded fulfillment) |
| 3 | Blueprint composition (nested blueprints) |
| 4 | RBAC integration (permission derivation, role generation) |
| 5 | OpenTofu bridge (optional, NICo Terraform provider) |

Phase 1 (this work) delivers the schema, API, mock server, and
visual designer. Phases 2-5 require changes to the NICo REST
backend.

## Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Blueprint schema too complex for NCPs | Low adoption | Start with simple examples; add complexity in later phases |
| Circular blueprint references | Infinite expansion | Validate DAG at blueprint creation; reject cycles |
| Expression language injection | Security | Sandbox expression evaluator; no function calls, only property access |
| Temporal workflow depth limits | Deeply nested blueprints fail | Cap nesting depth at 3 levels |
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
