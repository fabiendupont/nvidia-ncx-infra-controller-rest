# NEP-0006: NICo OpenTofu/Terraform Provider

| Field | Value |
|-------|-------|
| **Title** | NICo OpenTofu/Terraform Provider |
| **Status** | Implemented (Phase 1) |
| **Authors** | Red Hat NCP Team |
| **Created** | 2026-04-07 |
| **Target Release** | TBD |
| **Repository** | [github.com/fabiendupont/terraform-provider-nico](https://github.com/fabiendupont/terraform-provider-nico) |
| **Depends on** | NEP-0001 (Extensible Architecture) |

## Summary

Create an OpenTofu/Terraform provider for NICo that enables
Infrastructure-as-Code management of NICo resources. The
provider translates HCL resource declarations to NICo REST API
calls, enabling NCPs to use their existing IaC workflows for
GPU infrastructure provisioning.

This is a **separate Go module** (not a NICo provider plugin).
It runs as a standard Terraform/OpenTofu provider binary and
communicates with NICo's REST API. No changes to NICo itself
are required.

## Motivation

### NCPs already use IaC

Most NCPs have existing Terraform/OpenTofu workflows for
infrastructure management. They manage VMs, networks, storage,
and Kubernetes clusters with HCL. Adding GPU bare-metal
infrastructure should fit into the same workflow, not require
a separate CLI or API integration.

### Precedent: nvidia.bare_metal Ansible collection

The `nvidia.bare_metal` Ansible collection
([nvidia-ncx-infra-controller-ansible](https://github.com/fabiendupont/nvidia-ncx-infra-controller-ansible))
demonstrates that NICo's REST API surface is fully automatable:

- 56 modules covering every NICo resource type
- Generated from the OpenAPI spec (`make generate`)
- Idempotent CRUD with `state: present/absent`
- Wait/poll for async operations
- Dynamic inventory from instance discovery
- Auto-pagination for large collections

The same generation approach works for an OpenTofu provider.
The OpenAPI spec defines the resources; the generator produces
Go code with the Terraform Plugin Framework.

### Composable blueprints (NEP-0002) future path

NEP-0002 proposes a blueprint compiler that can accept OpenTofu
HCL as input. The OpenTofu provider is the prerequisite — it
defines the NICo resource types in HCL. Once the provider
exists, blueprints can be authored in HCL and compiled to
NICo's Temporal workflow DAG.

## Design

### Resource mapping

Every NICo API resource maps to a Terraform resource and data
source. The naming convention follows `nico_{resource}`:

| NICo Resource | Terraform Resource | Terraform Data Source |
|---|---|---|
| VPC | `nico_vpc` | `data.nico_vpc` |
| VPC Prefix | `nico_vpc_prefix` | `data.nico_vpc_prefix` |
| Subnet | `nico_subnet` | `data.nico_subnet` |
| Instance | `nico_instance` | `data.nico_instance` |
| Instance Type | `nico_instance_type` | `data.nico_instance_type` |
| Instance Batch | `nico_instance_batch` | — |
| Machine | `nico_machine` (update/delete only) | `data.nico_machine` |
| Allocation | `nico_allocation` | `data.nico_allocation` |
| IP Block | `nico_ip_block` | `data.nico_ip_block` |
| Network Security Group | `nico_network_security_group` | `data.nico_network_security_group` |
| Operating System | `nico_operating_system` | `data.nico_operating_system` |
| SSH Key | `nico_ssh_key` | `data.nico_ssh_key` |
| SSH Key Group | `nico_ssh_key_group` | `data.nico_ssh_key_group` |
| InfiniBand Partition | `nico_infiniband_partition` | `data.nico_infiniband_partition` |
| NVLink Logical Partition | `nico_nvlink_logical_partition` | `data.nico_nvlink_logical_partition` |
| Site | `nico_site` | `data.nico_site` |
| DPU Extension Service | `nico_dpu_extension_service` | `data.nico_dpu_extension_service` |
| Expected Machine | `nico_expected_machine` | `data.nico_expected_machine` |
| Tenant Account | `nico_tenant_account` | `data.nico_tenant_account` |

### Provider configuration

```hcl
terraform {
  required_providers {
    nico = {
      source  = "nvidia/nico"
      version = "~> 1.0"
    }
  }
}

provider "nico" {
  api_url    = "https://nico-api.example.com"
  api_token  = var.nico_token
  org        = "my-org"

  # Optional: "carbide" (direct) or "forge" (via NVIDIA proxy)
  api_path_prefix = "carbide"
}
```

Authentication uses a JWT bearer token (same as the Ansible
collection). The token can come from:
- Environment variable: `NICO_API_TOKEN`
- Provider config: `api_token`
- Keycloak SSA exchange (helper script)

### Example: GPU training cluster

```hcl
data "nico_site" "alpha" {
  name = "site-alpha"
}

data "nico_instance_type" "dgx" {
  name = "DGX-H100-80G"
}

data "nico_operating_system" "rhel" {
  name = "RHEL 9.4"
}

resource "nico_vpc" "training" {
  name    = "gpu-training-vpc"
  site_id = data.nico_site.alpha.id
}

resource "nico_vpc_prefix" "compute" {
  name         = "compute-prefix"
  vpc_id       = nico_vpc.training.id
  ip_block_id  = var.ip_block_id
  prefix_length = 24
}

resource "nico_instance" "worker" {
  count = 4

  name               = "gpu-worker-${count.index}"
  vpc_id             = nico_vpc.training.id
  instance_type_id   = data.nico_instance_type.dgx.id
  operating_system_id = data.nico_operating_system.rhel.id

  interfaces {
    vpc_prefix_id = nico_vpc_prefix.compute.id
    is_physical   = true
  }

  ssh_key_group_ids = [var.ssh_key_group_id]

  labels = {
    env  = "training"
    team = "ml-platform"
  }
}

resource "nico_infiniband_partition" "training" {
  name    = "training-ib"
  site_id = data.nico_site.alpha.id
}

output "worker_ips" {
  value = [for w in nico_instance.worker : w.interfaces[0].ip_addresses[0]]
}
```

### CRUD mapping

| Terraform Operation | NICo API Call | Notes |
|---|---|---|
| Create | `POST /v2/org/{org}/carbide/{resource}` | Returns resource with ID |
| Read | `GET /v2/org/{org}/carbide/{resource}/{id}` | Refreshes state |
| Update | `PATCH /v2/org/{org}/carbide/{resource}/{id}` | Sends only changed fields |
| Delete | `DELETE /v2/org/{org}/carbide/{resource}/{id}` | Async — polls until 404 |
| Import | `GET /v2/org/{org}/carbide/{resource}/{id}` | Imports existing resource |

### Async operations

NICo's create and delete operations are asynchronous — the
API returns immediately with a `Pending` or `Terminating` status.
The provider polls until the resource reaches a terminal state:

```go
func (r *InstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    // POST to create
    instance, err := r.client.Create(ctx, createReq)

    // Poll until Ready or Error
    instance, err = r.client.WaitForStatus(ctx, instance.ID,
        []string{"Ready"}, []string{"Error"}, timeout)

    // Store in state
    resp.State.Set(ctx, instance)
}
```

This matches the Ansible collection's `wait: true` behavior.

### Code generation from OpenAPI

Like the Ansible collection, the provider can be generated from
NICo's OpenAPI spec:

```
openapi/spec.yaml
       │
       ▼
  generator (Go)
       │
       ├── internal/provider/resources/nico_vpc.go
       ├── internal/provider/resources/nico_instance.go
       ├── internal/provider/datasources/nico_vpc.go
       └── ... (one file per resource/data source)
```

The generator reads the OpenAPI spec, identifies resources by
tag, extracts create/update schemas, and produces:
- Resource schema (Terraform Plugin Framework attributes)
- CRUD methods (Create, Read, Update, Delete)
- Data source Read methods
- Import support

Hand-written code lives in `internal/client/` (HTTP client,
auth, pagination) — same structure as the Ansible collection's
`module_utils/`.

### Comparison with Ansible collection

| Aspect | Ansible (`nvidia.bare_metal`) | OpenTofu (`nvidia/nico`) |
|---|---|---|
| Language | Python | Go |
| Framework | Ansible module API | Terraform Plugin Framework |
| Generation source | OpenAPI spec | OpenAPI spec |
| State management | None (idempotent per-run) | Terraform state file |
| Dependency graph | Task ordering in playbook | HCL `depends_on` + implicit refs |
| Async handling | `wait: true` with polling | Create/Read polling |
| Drift detection | Module re-runs compare | `terraform plan` shows diff |
| Import | N/A (lookup by name) | `terraform import` by ID |
| Inventory | Dynamic inventory plugin | Data sources |

Both approaches cover the same API surface. They serve
different operational models:
- **Ansible**: imperative, task-oriented, good for day-2 ops
- **OpenTofu**: declarative, state-tracked, good for provisioning

### Relation to NEP-0002 (Composable Blueprints)

The OpenTofu provider establishes NICo resource types in HCL.
NEP-0002's blueprint compiler could accept HCL modules as input:

```
HCL module (NICo resources)
       │
       ▼
  Blueprint compiler
       │
       ▼
  Temporal workflow DAG
       │
       ▼
  NICo providers execute
```

This gives NCPs three paths to the same outcome:
1. **REST API** — direct, programmatic
2. **OpenTofu** — declarative, state-tracked
3. **Blueprints** — NICo-native, catalog-integrated

All three ultimately call the same NICo REST API.

## Implementation

### Repository

Separate repository: `nvidia/terraform-provider-nico`

Following Terraform provider conventions:
```
terraform-provider-nico/
├── main.go
├── internal/
│   ├── client/          ← HTTP client (like Ansible's module_utils)
│   │   ├── client.go
│   │   ├── pagination.go
│   │   └── wait.go
│   └── provider/
│       ├── provider.go  ← Provider configuration
│       ├── resources/   ← One file per resource
│       └── datasources/ ← One file per data source
├── tools/
│   └── generator/       ← OpenAPI → Go code generator
└── examples/
    ├── gpu-cluster/
    └── multi-site/
```

### Phases

| Phase | Scope |
|---|---|
| 1 | HTTP client, provider config, VPC + Subnet + Instance resources |
| 2 | All remaining resources (generated from OpenAPI) |
| 3 | Data sources for all resources |
| 4 | Import support |
| 5 | Publish to Terraform Registry |

### Dependencies

- NICo REST API (no changes needed)
- OpenAPI spec (`openapi/spec.yaml`)
- Terraform Plugin Framework v1.x
- Go 1.25+

## Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| OpenAPI spec incomplete | Missing resources | Generate from spec + manual additions |
| Async timeout on large batches | Create fails | Configurable timeout per resource type |
| State drift from out-of-band changes | Plan shows unexpected diffs | Read refreshes from API on every plan |
| Provider not in Terraform Registry | Adoption friction | Publish early; support local install |

## References

- [NEP-0001: Extensible Architecture](0001-extensible-architecture.md)
- [NEP-0002: Composable Blueprints](0002-composable-blueprints.md)
- [nvidia.bare_metal Ansible collection](https://github.com/fabiendupont/nvidia-ncx-infra-controller-ansible)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Netris Terraform Provider](https://registry.terraform.io/providers/netrisai/netris/latest/docs) (prior art)
