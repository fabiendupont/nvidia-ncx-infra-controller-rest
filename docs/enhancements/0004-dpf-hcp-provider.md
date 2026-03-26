# NEP-0004: DPF HCP Provisioner Provider

| Field | Value |
|-------|-------|
| **Title** | DPF HCP Provisioner Provider |
| **Status** | Implemented (prototype) |
| **Authors** | Red Hat NCP Team |
| **Created** | 2026-04-07 |
| **Target Release** | TBD |
| **Branch** | `extensible-architecture` |
| **Depends on** | NEP-0001 (Extensible Architecture) |

## Summary

Add a `nico-dpfhcp` provider that automates DPU cluster
provisioning as part of NICo's site lifecycle. The provider
creates and monitors `DPFHCPProvisioner` custom resources on the
management OpenShift cluster, delegating HyperShift HostedCluster
orchestration to the
[dpf-hcp-provisioner-operator](https://github.com/rh-ecosystem-edge/dpf-hcp-provisioner-operator).

## Motivation

### Current state

NICo interacts with DPF at two levels:

- **Site setup** (`crates/dpf/` in ncx-infra-controller-core):
  creates BFB, DPUFlavor, DPUSet, and bf.cfg ConfigMap via the
  Forge/Carbide agent on the management cluster.
- **Application-level** (`DpuExtensionService` in REST API):
  deploys Kubernetes pods on DPUs attached to instances.

What is **missing** is the step between these two: creating the
DPUCluster itself and its backing HyperShift HostedCluster.
Today this requires manual creation of a `DPFHCPProvisioner` CR
and waiting for the operator to reconcile it.

### DPF architecture context

DPF provisioning is **one instance per site** — a single
DPUCluster spanning all BlueField DPUs at that site. Network
isolation is handled by DPF's ServiceFunctionChaining (OVN + HBN)
within that single cluster. There is no per-tenant or per-VPC
DPF instance.

### Zero Trust compatibility

The operator does not flash BFB images. It creates the
HostedCluster control plane and generates ignition configuration.
In Zero Trust mode, NICo Core handles network-based BFB flashing
independently. The operator's ignition ConfigMap is consumed by
the DPF provisioning controller regardless of how the BFB was
delivered.

## Design

### Provider identity

| Field | Value |
|-------|-------|
| Name | `nico-dpfhcp` |
| Version | `0.1.0` |
| Features | `["dpf-hcp"]` |
| Dependencies | `["nico-site"]` |
| Interfaces | `APIProvider`, `WorkflowProvider` |
| Profile | `ncp` only |

### API endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/sites/:siteId/dpf-hcp` | Trigger DPF HCP provisioning (201, 409 if exists) |
| GET | `/sites/:siteId/dpf-hcp/status` | Current provisioning status (200, 404) |
| DELETE | `/sites/:siteId/dpf-hcp` | Trigger teardown (202) |

Request fields map 1:1 to `DPFHCPProvisionerSpec`. The provider
does not invent its own abstraction.

### Temporal workflows

**DPFHCPProvisioningWorkflow** (6 steps):
1. ValidateSiteState
2. CreateProvisioningRecord
3. CreateDPFHCPProvisionerCR (K8s API)
4. WaitForPhase(Provisioning) (K8s watch)
5. WaitForPhase(Ready) (K8s watch)
6. UpdateSiteDPFStatus

**DPFHCPTeardownWorkflow** (4 steps):
1. ValidateSiteState
2. DeleteDPFHCPProvisionerCR
3. WaitForCRDeletion
4. DeleteProvisioningRecord

All activities are idempotent for safe Temporal retries.

### Hook integrations

| Type | Event | Behavior |
|------|-------|----------|
| Async reaction | `post-create-site` | Auto-provisions DPF when site is registered |
| Async reaction | `post-delete-site-components` | Auto-teardown |
| Sync pre-hook | `pre-create-instance` | Blocks instance creation if DPF not ready |

The pre-create-instance hook is advisory — if the provider is not
loaded, the hook doesn't exist and instance creation proceeds
normally.

### K8s client

Uses `k8s.io/client-go/dynamic` with locally-defined CR types
(no operator module dependency). Supports in-cluster config and
kubeconfig for management cluster access.

## Interaction with dpf-hcp-provisioner-operator

The operator handles the complex reconciliation internally:

1. Validate DPUCluster reference and secrets
2. Look up BlueField OCP layer image from registry
3. Configure MetalLB (if HA)
4. Create HostedCluster + NodePool (replicas=0)
5. Sync HostedCluster status
6. Inject kubeconfig into DPUCluster CR
7. Generate ignition (download from HyperShift, merge DPUFlavor
   OVS config, create ConfigMap, patch DPFOperatorConfig)
8. Auto-approve CSRs for DPU worker nodes

NICo's provider creates the CR and polls `.status.phase` until
`Ready`. The entire ignition generation and HostedCluster
lifecycle is opaque to NICo.

## Prerequisites

- dpf-hcp-provisioner-operator deployed on management cluster
- DPF Operator deployed (provides DPUCluster CRD)
- HyperShift Operator deployed
- DPUCluster CR created (by DPF Operator)

## References

- [NEP-0001: Extensible Architecture](0001-extensible-architecture.md)
- [DPF HCP Provider Documentation](../dpf-hcp-provider.md)
- [dpf-hcp-provisioner-operator](https://github.com/rh-ecosystem-edge/dpf-hcp-provisioner-operator)
