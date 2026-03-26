# NEP-0003: NICo AAP Provider

| Field | Value |
|-------|-------|
| **Title** | Ansible Automation Platform Provider |
| **Status** | Implemented (prototype) |
| **Authors** | Fabien Dupont |
| **Created** | 2026-03-30 |
| **Target Release** | TBD |
| **Branch** | `extensible-architecture` (proof of concept) |
| **Depends on** | NEP-0001 (Extensible Architecture) |

## Summary

Add an Ansible Automation Platform (AAP) provider to NICo's
extensible architecture. The provider registers Temporal
activities that launch AAP Controller job templates, enabling
NCPs to reuse existing Ansible automation within NICo's
lifecycle workflows. NICo fires a hook, AAP runs the playbook,
NICo gets the result.

## Motivation

### The gap between NICo and existing automation

NICo's Temporal workflows handle GPU infrastructure lifecycle:
provisioning, sanitization, firmware, DPU configuration. But
NCPs also maintain Ansible playbooks for tasks NICo does not
cover:

- BMaaS OS patching (RHEL, Ubuntu, Rocky)
- Certificate rotation on non-K8s services
- Storage array maintenance (VAST, WEKA, DDN vendor APIs)
- Network switch firmware (Spectrum, Cumulus, Arista)
- Compliance evidence collection
- ITSM integration (ServiceNow, PagerDuty)
- Custom tenant onboarding steps (LDAP groups, DNS zones)

Today these run independently. When NICo provisions a machine,
the NCP operator must separately trigger AAP jobs for post-
provisioning hardening. When NICo decommissions a tenant, the
operator must separately run cleanup playbooks. There is no
orchestration between the two.

### Why not just call the AAP API from playbooks?

The AAP day-2 deep dive documents AAP calling NICo's REST API
from playbooks. That works for AAP-initiated workflows. But the
reverse — NICo triggering AAP jobs as part of its own workflows
— requires integration at the Temporal activity level.

### What the extensible architecture enables

NEP-0001's extensible architecture provides the exact patterns
needed:

- **WorkflowProvider** interface for registering Temporal
  activities
- **Hook system** (sync + async) for injecting steps into
  existing lifecycle workflows
- **Provider registry** with dependency resolution
- **Deployment profiles** for optional loading

An AAP provider uses these patterns to bridge NICo lifecycle
events to AAP job templates.

## Design

### Provider structure

```go
type AAPProvider struct {
    client     *aap.ControllerClient
    hooks      *provider.HookRunner
    cfg        AAPConfig
}

func (p *AAPProvider) Name() string         { return "nico-aap" }
func (p *AAPProvider) Version() string      { return "0.1.0" }
func (p *AAPProvider) Features() []string   { return []string{} }
func (p *AAPProvider) Dependencies() []string {
    return []string{"nico-provisioning"}
}
```

The AAP provider does not implement a feature (no REST
routes). It only registers hooks and Temporal activities that
call the AAP Controller API. It depends on `nico-provisioning`
because it hooks into compute lifecycle events.

### Temporal activities

Three activities provide the building blocks for any AAP
integration:

```go
// LaunchJobTemplate starts an AAP job and returns the job ID.
func (p *AAPProvider) LaunchJobTemplate(
    ctx context.Context, input LaunchInput,
) (LaunchOutput, error)

// WaitForJob polls the AAP Controller until the job completes
// or times out. Returns the job status and artifacts.
func (p *AAPProvider) WaitForJob(
    ctx context.Context, input WaitInput,
) (WaitOutput, error)

// GetJobOutput retrieves stdout, artifacts, and extra_vars
// from a completed AAP job.
func (p *AAPProvider) GetJobOutput(
    ctx context.Context, jobID int,
) (JobOutput, error)
```

```go
type LaunchInput struct {
    TemplateName string
    Organization string
    ExtraVars    map[string]interface{}
    Limit        string            // host pattern
    Credential   string            // AAP credential name
    Timeout      time.Duration
}

type LaunchOutput struct {
    JobID  int
    Status string
}
```

### Hook registration

The provider registers hooks at init time. NCPs configure
which hooks are active via the provider config:

```go
func (p *AAPProvider) Init(ctx provider.ProviderContext) error {
    p.client = aap.NewControllerClient(p.cfg.ControllerURL, p.cfg.Token)
    p.hooks = ctx.Hooks

    for _, binding := range p.cfg.Bindings {
        switch binding.Type {
        case "sync":
            ctx.Hooks.RegisterSync(provider.SyncHook{
                Feature: binding.Feature,
                Event:   binding.Event,
                Handler: p.makeSyncHandler(binding),
            })
        case "async":
            ctx.Hooks.RegisterReaction(provider.Reaction{
                Feature:        binding.Feature,
                Event:          binding.Event,
                TargetWorkflow: "aap-job-runner",
                SignalName:     binding.Signal,
            })
        }
    }
    return nil
}
```

### Configuration

The NCP operator configures AAP bindings declaratively:

```yaml
# NICo provider config for AAP
provider: nico-aap
config:
  controller_url: "https://aap-controller.seed.ncp.local"
  token_secret: "nico-aap-token"       # K8s Secret reference
  organization: "ncp-ops"

  bindings:
    # Block instance creation until CIS hardening passes
    - event: PreCreateInstance
      feature: compute
      type: sync
      template: "cis-hardening-check"
      extra_vars_from: instance         # inject instance metadata
      timeout: 15m

    # Run post-provisioning hardening after instance is ready
    - event: PostCreateInstance
      feature: compute
      type: async
      template: "post-provision-hardening"
      extra_vars_from: instance

    # Run tenant cleanup before deletion
    - event: PreDeleteInstance
      feature: compute
      type: sync
      template: "tenant-data-cleanup"
      extra_vars_from: instance
      timeout: 30m

    # Notify ITSM after site provisioning
    - event: PostCreateSite
      feature: site
      type: async
      template: "itsm-site-notification"
      extra_vars_from: site
```

### Data flow

```
NICo Lifecycle Event (e.g. PostCreateInstance)
  │
  ├─ Sync hook? ──→ LaunchJobTemplate ──→ WaitForJob
  │                  │                      │
  │                  │ AAP Controller API   │ Poll until
  │                  │ POST /job_templates/ │ complete
  │                  │ {id}/launch/         │
  │                  │                      │
  │                  └──────────────────────┘
  │                    │
  │                    ├─ Success → continue NICo workflow
  │                    └─ Failure → block operation, return error
  │
  └─ Async reaction? ──→ Temporal signal ──→ aap-job-runner workflow
                           │
                           └─ LaunchJobTemplate (fire and forget)
```

### Sync vs async decision

| Use case | Type | Why |
|---|---|---|
| Compliance check before tenant allocation | Sync | Must pass before GPU access |
| CIS/STIG hardening after provisioning | Async | Non-blocking; instance is usable |
| Data wipe before decommission | Sync | Must complete before machine reuse |
| ITSM ticket creation | Async | Notification, not a gate |
| Storage array prep | Sync | Storage must be ready before instance |
| Certificate rotation on schedule | Async | Background maintenance |

### AAP Controller API mapping

| NICo activity | AAP Controller endpoint | Method |
|---|---|---|
| LaunchJobTemplate | `/api/v2/job_templates/{id}/launch/` | POST |
| WaitForJob | `/api/v2/jobs/{id}/` | GET (poll) |
| GetJobOutput | `/api/v2/jobs/{id}/stdout/` | GET |
| Credential lookup | `/api/v2/credentials/?name={name}` | GET |

### Authentication

The provider authenticates to AAP Controller using a personal
access token or OAuth2 application token stored in a Kubernetes
Secret. The token is read at provider init and refreshed on
401 responses.

For production deployments, use an OAuth2 application token
with a dedicated AAP service account scoped to the NCP
organization. Do not use a personal token tied to a human user.

## NCP use cases

### BMaaS post-provisioning

NICo provisions a RHEL BMaaS node via PXE. After the OS is
installed, the AAP provider fires the `post-provision-hardening`
job template which:

1. Registers the node with RHUI for patching
2. Configures SSSD for LDAP/Keycloak integration
3. Installs and configures Slurm client
4. Mounts Lustre or NFS storage
5. Runs CIS Level 2 hardening playbook
6. Reports compliance status back to NICo via extra_vars

### Coordinated maintenance window

A scheduled NICo firmware update triggers the AAP provider
to:

1. (Sync, pre-firmware) Drain Slurm jobs from affected nodes
2. (Sync, pre-firmware) Notify tenants via ITSM
3. NICo runs firmware update
4. (Async, post-firmware) Re-register nodes with Slurm
5. (Async, post-firmware) Run GPU validation tests

### Tenant offboarding

When NICo deletes a tenant:

1. (Sync, pre-delete) AAP removes LDAP groups and DNS zones
2. (Sync, pre-delete) AAP archives tenant logs to cold storage
3. NICo deletes VPCs, instances, and Keycloak realm
4. (Async, post-delete) AAP closes ITSM ticket

## Design constraints

**AAP Controller must be reachable from NICo.** The provider
calls the AAP Controller REST API over HTTPS. In air-gapped
deployments, AAP Controller runs on the seed cluster alongside
NICo.

**Job template timeout must be bounded.** Temporal activities
have heartbeat timeouts. The `WaitForJob` activity heartbeats
every 30 seconds while polling the AAP job status. If the AAP
job exceeds the configured timeout, the activity fails and
NICo handles it as a hook failure (sync hooks block the
operation, async reactions log the failure).

**Playbooks must be idempotent.** Temporal may retry activities
on transient failures. AAP job templates must handle being
launched multiple times for the same instance without side
effects. Use Ansible's `when` conditions and check mode where
appropriate.

**Do not duplicate NICo capabilities.** If NICo has a provider
for a task (provisioning, firmware, networking), use NICo. The
AAP provider is for tasks outside NICo's scope. Overlap creates
conflicts (e.g., both NICo and AAP trying to reboot a machine).

**Credential isolation.** The AAP provider's token gives access
to AAP job templates. Limit the token's scope to the NCP
organization and the specific templates configured in bindings.
Do not grant cluster-admin or global template access.

## Implementation phases

| Phase | Scope |
|---|---|
| 1 | Temporal activities: `LaunchJobTemplate`, `WaitForJob`, `GetJobOutput` |
| 2 | Provider skeleton: `Provider` interface, `aap-tasks` task queue, `ncp` profile |
| 3 | Hook bindings: declarative config, sync hooks, async reactions |
| 4 | Error handling: AAP failure parsing, retry policy, fail-open/fail-closed |
| 5 | Documentation: example bindings, AAP job template examples |

## Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| AAP Controller unavailability blocks sync hooks | NICo operations stall | Configurable fail-open vs fail-closed per binding |
| AAP job timeout exceeds Temporal heartbeat | Activity marked failed | WaitForJob heartbeats every 30s; configurable timeout per binding |
| Non-idempotent playbooks cause side effects on retry | Duplicate actions | Document idempotency requirement; recommend check mode |
| Credential scope too broad | Security risk | Scope AAP token to NCP org and specific templates |
| Debugging spans two systems | Slower incident response | Correlate NICo workflow ID with AAP job ID in logs |

## Alternatives considered

| Alternative | Why not |
|---|---|
| Call AAP from NICo REST API handlers directly | Bypasses Temporal retry/timeout; no workflow visibility |
| Run Ansible playbooks via `ansible-runner` in NICo | Requires Ansible runtime in NICo container; no AAP audit trail |
| Use EDA (Event-Driven Ansible) webhooks | EDA watches alerts, not NICo lifecycle events; wrong trigger source |
| Keep NICo and AAP fully independent | Status quo; operator must coordinate manually |

## Companion Ansible collections

Two Ansible collections support NCP fabric automation via AAP:

- **`nvidia.nvue`** — Red Hat-certified collection for NVIDIA
  Cumulus Linux (Ethernet switches). NVUE API for interface,
  bridge, VLAN, BGP, EVPN configuration.

- **`nvidia.ufm`** — 39 modules for NVIDIA UFM (InfiniBand
  fabric). PKEY management, QoS policies, SHARP configuration,
  port monitoring, topology discovery.

These collections are used in AAP job templates referenced by
the provider's binding configuration.

## References

- [NEP-0001: NICo Extensible Architecture](0001-extensible-architecture.md)
