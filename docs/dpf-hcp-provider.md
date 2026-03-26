# DPF HCP Provisioner Provider

## Purpose

The `nico-dpfhcp` provider automates DPU cluster provisioning as
part of NICo's site lifecycle. It bridges NICo's REST API and
Temporal workflows with the
[dpf-hcp-provisioner-operator](https://github.com/rh-ecosystem-edge/dpf-hcp-provisioner-operator)
running on the management OpenShift cluster.

## Background

### What DPF HCP provisioning does

When a site has NVIDIA BlueField DPUs, the DPUs need a Kubernetes
cluster to run networking services (OVN, HBN). This DPU cluster
is managed as a HyperShift HostedCluster, providing OpenShift on
the DPUs' ARM cores.

The dpf-hcp-provisioner-operator handles the complex reconciliation:
HostedCluster creation, NodePool setup, ignition generation (merging
HyperShift ignition with DPU-specific config from DPUFlavor),
kubeconfig injection into the DPUCluster CR, CSR auto-approval for
DPU worker nodes, and MetalLB configuration for HA.

### What was manual before

Without this provider, operators had to:
1. Manually create a `DPFHCPProvisioner` CR
2. Wait for the operator to reconcile it through five phases
3. Verify all conditions before proceeding with tenant provisioning

This provider automates steps 1-3 and integrates the result into
NICo's site readiness tracking.

## Architecture

```
NICo REST API
  POST /sites/:siteId/dpf-hcp
    │
    ├─ Creates provisioning record (in-memory store)
    └─ Starts DPFHCPProvisioningWorkflow (Temporal)
         │
         ├─ Step 1: ValidateSiteState
         ├─ Step 2: CreateProvisioningRecord
         ├─ Step 3: CreateDPFHCPProvisionerCR ──→ K8s API
         ├─ Step 4: WaitForPhase(Provisioning) ──→ K8s Watch
         ├─ Step 5: WaitForPhase(Ready)        ──→ K8s Watch
         └─ Step 6: UpdateSiteDPFStatus

    dpf-hcp-provisioner-operator (on management cluster):
      DPFHCPProvisioner CR
        → Validate DPUCluster + secrets
        → Lookup BlueField OCP layer image
        → Configure MetalLB (if HA)
        → Create HostedCluster + NodePool(replicas=0)
        → Sync HostedCluster status
        → Inject kubeconfig into DPUCluster
        → Generate ignition (download from HyperShift,
          merge DPUFlavor OVS config, embed target.ign)
        → Create bfcfg ConfigMap
        → Patch DPFOperatorConfig.bfcfgTemplateConfigMap
        → Auto-approve DPU worker CSRs
        → Phase: Pending → Provisioning → IgnitionGenerating → Ready
```

## API

### POST /v2/org/:orgName/carbide/sites/:siteId/dpf-hcp

Create DPF HCP provisioning for a site. Idempotent — returns 409
if provisioning already exists.

**Request:**

```json
{
  "dpuClusterRef": {
    "name": "site-alpha-dpucluster",
    "namespace": "dpf-operator-system"
  },
  "baseDomain": "clusters.example.com",
  "ocpReleaseImage": "quay.io/openshift-release-dev/ocp-release:4.17.0-rc.1-multi",
  "sshKeySecretRef": "site-alpha-ssh-key",
  "pullSecretRef": "site-alpha-pull-secret",
  "controlPlaneAvailabilityPolicy": "HighlyAvailable",
  "virtualIP": "10.0.100.50",
  "etcdStorageClass": "local-path",
  "flannelEnabled": true,
  "dpuDeploymentRef": {
    "name": "site-alpha-dpudeployment",
    "namespace": "dpf-operator-system"
  }
}
```

Fields map 1:1 to `DPFHCPProvisionerSpec`. The provider does not
invent its own abstraction.

**Response:** `201 Created`

### GET /v2/org/:orgName/carbide/sites/:siteId/dpf-hcp/status

Returns current provisioning status. 404 if no provisioning
requested for this site.

**Response:**

```json
{
  "siteId": "site-alpha",
  "status": "Ready",
  "phase": "Ready",
  "conditions": [
    {"type": "HostedClusterAvailable", "status": "True"},
    {"type": "KubeConfigInjected", "status": "True"},
    {"type": "IgnitionConfigured", "status": "True"},
    {"type": "CSRAutoApprovalActive", "status": "True"}
  ],
  "created": "2026-03-25T10:00:00Z",
  "updated": "2026-03-25T10:12:34Z"
}
```

### DELETE /v2/org/:orgName/carbide/sites/:siteId/dpf-hcp

Trigger teardown. Returns `202 Accepted`.

## Hook integrations

### Auto-provision on site creation

When a site is created and reaches Registered state, the provider
receives a `post-create-site` signal via a long-running watcher
workflow. If the site has DPF HCP configuration, provisioning
starts automatically.

### Block instance creation if DPF not ready

A sync pre-hook on `pre-create-instance` checks whether the
target site has a DPF HCP record. If it does and the status is
not Ready, instance creation is blocked with an error explaining
that DPU infrastructure is still provisioning.

This hook is advisory — if the `nico-dpfhcp` provider is not
loaded (e.g., `management` profile), the hook doesn't exist and
instance creation proceeds normally.

### Auto-teardown on site deletion

When site components are deleted, a `post-delete-site-components`
signal triggers the DPF HCP teardown workflow, which deletes the
DPFHCPProvisioner CR and waits for the operator to clean up.

## Zero Trust compatibility

The operator is compatible with NICo's Zero Trust provisioning
model. The operator does not flash BFB images — it creates the
HostedCluster control plane and generates ignition configuration.

In Zero Trust mode:
- NICo Core (carbide agent) handles network-based BFB flashing
- The DPF HCP provider creates the HostedCluster and ignition
- The DPF Operator's provisioning controller reads the ignition
  ConfigMap regardless of how the BFB was delivered

The NICo REST provider's workflow (`WaitForPhase("Ready")`)
covers all phases including ignition generation — no special
handling needed for Zero Trust vs Trusted Host modes.

## Prerequisites

### Runtime

- dpf-hcp-provisioner-operator deployed on management cluster
- DPF Operator deployed (provides DPUCluster CRD)
- HyperShift Operator deployed
- MetalLB deployed (if using HighlyAvailable control plane)
- DPUCluster CR created (by DPF Operator)
- SSH key and pull secrets created in the CR namespace

### Configuration

- `NICO_PROFILE=ncp` (or custom profile including dpfhcp)
- NICo REST API running in-cluster (for K8s API access) or
  `KUBECONFIG` pointing to management cluster

## Deployment profile

The provider is included in the `ncp` profile only:

```go
provider.RegisterProfileProviders("ncp", []func() provider.Provider{
    // ... core providers ...
    func() provider.Provider { return dpfhcp.New() },
})
```

It is not in `management` or `management-with-site` profiles
because DPF HCP provisioning is specific to NCP deployments
with Red Hat OpenShift and BlueField DPUs.
