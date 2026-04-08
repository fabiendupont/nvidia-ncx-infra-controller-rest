# NEP-0007: Health Provider — Fault Management and Service Events

| Field | Value |
|-------|-------|
| **Title** | Health Provider — Fault Management and Service Events |
| **Status** | Proposal |
| **Authors** | Fabien Dupont |
| **Created** | 2026-04-07 |
| **Target Release** | TBD |
| **Depends on** | NEP-0001 (Extensible Architecture) |
| **Closes** | GAP-11 (NVSentinel + NICo + RHWA), GAP-15 (Breakfix Event API) |

## Summary

Expand the existing `nico-health` provider from its current
minimal state (system `/healthz` + `/readyz`) into a full
infrastructure health management system. The expanded provider
adds structured fault event tracking, automated remediation
workflows, and tenant-facing service event notifications.

Two new concepts:

- **Fault events** — operator-facing, infrastructure-scoped.
  "GPU 3 on tray 7 in rack A01 had XID 48." Tracks the full
  lifecycle: detection → classification → isolation →
  remediation → validation → resolution or escalation.

- **Service events** — tenant-facing, allocation-scoped.
  "2 of your 8 GPUs are temporarily unavailable. Automated
  recovery in progress. ETA: 15 minutes." Derived from one
  or more fault events. No infrastructure details exposed.

The provider closes two gaps in the NCP reference architecture:

- **GAP-11**: No automated bridge between fault detection
  (NVSentinel, DCGM, powershelf sensors, NVSwitch manager,
  DPU agent, NHC), machine lifecycle (NICo), and node
  remediation (RHWA). Today an operator must manually correlate
  alerts across systems, put machines in maintenance, trigger
  component-specific resets, validate recovery, and restore
  service.

- **GAP-15**: No unified fault query API. Fault data is scattered
  across NHC conditions, AlertManager alerts, NICo machine health
  JSONB, powershelf sensor readings, NVSwitch firmware state,
  DCGM exporters, and Loki logs. Tenants have no visibility
  into service impact.

## Motivation

### What happens today when infrastructure fails

The manual response is the same regardless of component — only
the remediation action differs:

| Component | Detection source | Manual steps |
|-----------|-----------------|--------------|
| GPU (XID, ECC) | DCGM exporter, NVSentinel | Check alert → find machine → maintenance mode → GPU reset → DCGM diag → restore |
| NVSwitch (link down) | NVSwitch manager gRPC | Check NVSwitch status → identify affected tray → power cycle → validate links → restore |
| Power (PSU fault) | Powershelf manager sensors | Check sensor thresholds → identify rack → wait for stabilization → validate → restore |
| DPU (link down, HBN) | forge-dpu-agent probes | Check DPU health → DPU reset or BFB re-flash → validate overlay → restore |
| Network (IB flap) | UFM, machine health probes | Check IB link → identify PKEY impact → reset HCA → validate RDMA → restore |
| Storage (NVMe SMART) | Not currently monitored | Manual check → identify drive → backup data → replace → restore |
| BMC (unreachable) | Site agent discovery | Ping BMC → reset BMC → wait → validate Redfish → restore |
| Cooling (fan, thermal) | Powershelf sensors | Check temperature → throttle workloads → wait → validate → restore |
| CPU/Memory (MCE) | Not currently monitored | Check kernel logs → identify DIMM → isolate node → replace → restore |

On a 10-rack deployment with 720 GPUs, 720 DPUs, 180 NVSwitches,
and 40 power shelves, manual correlation and remediation does
not scale.

### What NICo already tracks

NICo has health data across multiple subsystems, but it is
fragmented:

| Data | Location | Limitation |
|------|----------|------------|
| Machine health probes | `machine.health` JSONB | Unstructured, not queryable by severity or component |
| Machine status | `machine.status` enum | No fault classification (`Error` is a single undifferentiated state) |
| Status history | `status_detail` table | Generic audit trail, no fault-specific fields |
| Network degradation | `machine.is_network_degraded` | Boolean flag with message, no classification or lifecycle |
| Site agent health | Temporal cron (1 min) | Connectivity only — inventory, controller, HA status |
| Powershelf sensors | `powershelf-manager` gRPC | Sensor readings with thresholds (upper_caution, upper_critical, lower_caution, lower_critical) but no alerting |
| NVSwitch status | `nvswitch-manager` gRPC | Firmware state, power control, but no health events |
| DPU health | `forge-dpu-agent` probes | Reports via `HealthProbeAlert` with `classifications[]` and `tenant_message`, but stored in JSONB only |

The `health.proto` already defines a solid probe model —
`HealthProbeAlert` has `id`, `target`, `classifications[]`,
`tenant_message`, `in_alert_since`. The powershelf manager has
`Sensor` with `SensorThresholds`. The data exists but there
is no structured persistence, no lifecycle tracking, no
remediation orchestration, and no tenant-facing API.

### Why expand `nico-health`, not create a new provider

The health provider already owns machine health probes, site
health monitoring, and the `/healthz` + `/readyz` system
endpoints. Faults are health state changes — a fault is the
absence of health. Creating a separate `nico-faultmgmt`
provider would split a single domain into two providers that
constantly call each other via service interfaces.

Expanding `nico-health` keeps the health domain cohesive:
health probes, fault events, remediation workflows, and
service events all in one provider, one DAO, one set of tables.

### What the extensible architecture enables

NEP-0001's provider model gives this proposal:

- **APIProvider** for health/fault/service-event endpoints
- **WorkflowProvider** for remediation workflows on Temporal
- **Hook system** for triggering remediation on fault ingestion
  and integrating with NEP-0003 (AAP) for ITSM escalation
- **MigrationProvider** for new tables
- **Deployment profiles** for optional loading of remediation
  capabilities in the `ncp` profile

## Design

### Provider identity (updated)

| Field | Before | After |
|-------|--------|-------|
| Name | `nico-health` | `nico-health` (unchanged) |
| Version | `1.0.6` | `2.0.0` |
| Features | `["health"]` | `["health", "fault-management"]` |
| Dependencies | `["nico-compute"]` | `["nico-compute"]` (unchanged) |
| Interfaces | None (system routes only) | `APIProvider`, `WorkflowProvider`, `MigrationProvider` |
| Profile | All | All (health), `ncp` (fault-management) |

The `fault-management` feature is only registered in the `ncp`
profile. The base `health` feature (probes, healthz, readyz)
remains available in all profiles. This means NVIDIA-standalone
deployments keep the current behavior; NCP deployments get the
full fault lifecycle.

### Architecture

```
Internal Fault Sources               External Fault Sources
──────────────────────               ──────────────────────

┌──────────────────┐                 ┌──────────────┐
│ powershelf-mgr   │──┐             │  NVSentinel  │──┐
│ (sensor thresh.) │  │             └──────────────┘  │
└──────────────────┘  │                               │
                      │             ┌──────────────┐  │
┌──────────────────┐  │  health     │  DCGM /      │──┤ webhook
│ nvswitch-mgr     │──┤  probes    │  AlertManager │  │
│ (link/fw state)  │  │  (gRPC)    └──────────────┘  │
└──────────────────┘  │                               │
                      │             ┌──────────────┐  │
┌──────────────────┐  │             │  NHC / RHWA  │──┘
│ forge-dpu-agent  │──┤             └──────────────┘
│ (DPU probes)     │  │
└──────────────────┘  │
                      │
┌──────────────────┐  │
│ site-agent       │──┘
│ (connectivity)   │
└──────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────┐
│                   nico-health provider                   │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐ │
│  │ fault_event  │  │service_event │  │ Remediation   │ │
│  │ table        │←→│ table        │  │ Workflows     │ │
│  │ (operator)   │  │ (tenant)     │  │ (Temporal)    │ │
│  └──────┬───────┘  └──────┬───────┘  └───────┬───────┘ │
│         │                 │                   │         │
│         ▼                 ▼                   ▼         │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐ │
│  │ /health/     │  │ /tenant/     │  │ NICo compute  │ │
│  │ events       │  │ {id}/service │  │ (maintenance  │ │
│  │ (operator)   │  │ -events      │  │  mode, reset) │ │
│  └──────────────┘  │ (tenant)     │  └───────┬───────┘ │
│                    └──────────────┘           │         │
│                                       ┌──────▼───────┐ │
│                                       │ AAP provider │ │
│                                       │ (NEP-0003)   │ │
│                                       │ ITSM, notify │ │
│                                       └──────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### Data model

Both models live in `db/pkg/db/model/` alongside all other
NICo models. They use standard bun `rel:belongs-to` relations
for eager loading and JOIN-based queries, consistent with how
the rest of the codebase works (92 existing `belongs-to`
relations across 42 models).

#### fault_event table

```sql
CREATE TABLE fault_event (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES org(id),
    tenant_id       UUID REFERENCES tenant(id),
    site_id         UUID NOT NULL REFERENCES site(id),
    machine_id      UUID REFERENCES machine(id),
    instance_id     UUID REFERENCES instance(id),

    -- Classification
    source          TEXT NOT NULL,
    severity        TEXT NOT NULL,
    component       TEXT NOT NULL,
    classification  TEXT,
    message         TEXT NOT NULL,

    -- Lifecycle
    state           TEXT NOT NULL DEFAULT 'open',
    detected_at     TIMESTAMPTZ NOT NULL,
    acknowledged_at TIMESTAMPTZ,
    resolved_at     TIMESTAMPTZ,
    suppressed_until TIMESTAMPTZ,

    -- Remediation
    remediation_workflow_id TEXT,
    remediation_attempts    INT NOT NULL DEFAULT 0,
    escalation_level        INT NOT NULL DEFAULT 0,

    -- Extensibility
    metadata        JSONB,

    -- Audit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fault_event_org ON fault_event(org_id);
CREATE INDEX idx_fault_event_site ON fault_event(site_id);
CREATE INDEX idx_fault_event_machine ON fault_event(machine_id);
CREATE INDEX idx_fault_event_state ON fault_event(state);
CREATE INDEX idx_fault_event_severity ON fault_event(severity);
CREATE INDEX idx_fault_event_detected ON fault_event(detected_at);
```

#### fault_event Go model

```go
type FaultEvent struct {
    bun.BaseModel `bun:"table:fault_event"`

    ID             string    `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
    OrgID          string    `bun:"org_id,notnull,type:uuid"`
    TenantID       *string   `bun:"tenant_id,type:uuid"`
    SiteID         string    `bun:"site_id,notnull,type:uuid"`
    MachineID      *string   `bun:"machine_id,type:uuid"`
    InstanceID     *string   `bun:"instance_id,type:uuid"`

    Source         string    `bun:"source,notnull"`
    Severity       string    `bun:"severity,notnull"`
    Component      string    `bun:"component,notnull"`
    Classification *string   `bun:"classification"`
    Message        string    `bun:"message,notnull"`

    State          string    `bun:"state,notnull,default:'open'"`
    DetectedAt     time.Time `bun:"detected_at,notnull"`
    AcknowledgedAt *time.Time `bun:"acknowledged_at"`
    ResolvedAt     *time.Time `bun:"resolved_at"`
    SuppressedUntil *time.Time `bun:"suppressed_until"`

    RemediationWorkflowID *string `bun:"remediation_workflow_id"`
    RemediationAttempts   int     `bun:"remediation_attempts,notnull,default:0"`
    EscalationLevel       int     `bun:"escalation_level,notnull,default:0"`

    Metadata       map[string]interface{} `bun:"metadata,type:jsonb"`

    CreatedAt      time.Time `bun:"created_at,notnull,default:current_timestamp"`
    UpdatedAt      time.Time `bun:"updated_at,notnull,default:current_timestamp"`

    // Relations — standard bun eager loading, same as Instance, Allocation, etc.
    Org            *Org      `bun:"rel:belongs-to,join:org_id=id"`
    Tenant         *Tenant   `bun:"rel:belongs-to,join:tenant_id=id"`
    Site           *Site     `bun:"rel:belongs-to,join:site_id=id"`
    Machine        *Machine  `bun:"rel:belongs-to,join:machine_id=id"`
    Instance       *Instance `bun:"rel:belongs-to,join:instance_id=id"`
}
```

The DAO uses `.Relation("Machine").Relation("Site").Relation("Tenant")`
for list queries, producing JOINs on indexed FKs — identical to
how `Instance`, `Allocation`, and `VpcPrefix` queries work today.

#### service_event table

```sql
CREATE TABLE service_event (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES org(id),
    tenant_id       UUID NOT NULL REFERENCES tenant(id),
    instance_id     UUID REFERENCES instance(id),

    -- Tenant-safe description
    summary         TEXT NOT NULL,
    impact          TEXT NOT NULL,
    state           TEXT NOT NULL DEFAULT 'active',

    -- Timing
    started_at      TIMESTAMPTZ NOT NULL,
    estimated_resolution_at TIMESTAMPTZ,
    resolved_at     TIMESTAMPTZ,

    -- Billing
    downtime_excluded BOOLEAN NOT NULL DEFAULT false,

    -- Audit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_service_event_tenant ON service_event(tenant_id);
CREATE INDEX idx_service_event_state ON service_event(state);
CREATE INDEX idx_service_event_started ON service_event(started_at);
```

#### service_event Go model

```go
type ServiceEvent struct {
    bun.BaseModel `bun:"table:service_event"`

    ID            string    `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
    OrgID         string    `bun:"org_id,notnull,type:uuid"`
    TenantID      string    `bun:"tenant_id,notnull,type:uuid"`
    InstanceID    *string   `bun:"instance_id,type:uuid"`

    Summary       string    `bun:"summary,notnull"`
    Impact        string    `bun:"impact,notnull"`
    State         string    `bun:"state,notnull,default:'active'"`

    StartedAt     time.Time `bun:"started_at,notnull"`
    EstimatedResolutionAt *time.Time `bun:"estimated_resolution_at"`
    ResolvedAt    *time.Time `bun:"resolved_at"`

    DowntimeExcluded bool  `bun:"downtime_excluded,notnull,default:false"`

    CreatedAt     time.Time `bun:"created_at,notnull,default:current_timestamp"`
    UpdatedAt     time.Time `bun:"updated_at,notnull,default:current_timestamp"`

    // Relations
    Org           *Org      `bun:"rel:belongs-to,join:org_id=id"`
    Tenant        *Tenant   `bun:"rel:belongs-to,join:tenant_id=id"`
    Instance      *Instance `bun:"rel:belongs-to,join:instance_id=id"`
}
```

#### fault_service_event join table

Multiple fault events can cause one service event (e.g., NVLink
domain failure produces 4 GPU faults but one tenant disruption).
One fault event can affect multiple tenants (e.g., powershelf
fault affects all trays in a rack).

```sql
CREATE TABLE fault_service_event (
    fault_event_id   UUID NOT NULL REFERENCES fault_event(id),
    service_event_id UUID NOT NULL REFERENCES service_event(id),
    PRIMARY KEY (fault_event_id, service_event_id)
);
```

```go
type FaultServiceEvent struct {
    bun.BaseModel   `bun:"table:fault_service_event"`
    FaultEventID    string        `bun:"fault_event_id,pk,type:uuid"`
    ServiceEventID  string        `bun:"service_event_id,pk,type:uuid"`
    FaultEvent      *FaultEvent   `bun:"rel:belongs-to,join:fault_event_id=id"`
    ServiceEvent    *ServiceEvent `bun:"rel:belongs-to,join:service_event_id=id"`
}
```

#### Field semantics

**source** — Origin of the fault signal:

| Value | Description | Internal/External |
|-------|-------------|-------------------|
| `nvsentinel` | NVSentinel GPU fault classification | External (webhook) |
| `dcgm` | DCGM metrics via AlertManager webhook | External (webhook) |
| `nhc` | Node Health Check failure | External (webhook) |
| `rhwa` | Red Hat Workload Availability (FAR) | External (webhook) |
| `nico-agent` | NICo site agent health probe | Internal (health.proto) |
| `nico-power` | NICo powershelf manager sensor threshold | Internal (powershelf-manager gRPC) |
| `nico-nvswitch` | NICo NVSwitch manager state change | Internal (nvswitch-manager gRPC) |
| `nico-dpu` | forge-dpu-agent health probe | Internal (health.proto) |
| `operator` | Manual operator report | API |

**severity** — Impact classification:

| Value | Meaning | Examples |
|-------|---------|---------|
| `critical` | Service impacted, immediate action | GPU fallen off bus, NVLink down, PSU fault, DPU unreachable |
| `warning` | Degraded, action needed soon | ECC SBE trending up, sensor upper_caution, fan speed low |
| `info` | Notable, no action needed | Firmware update available, sensor returned to normal |

**component** — Affected infrastructure element:

| Value | Scope | NICo data source |
|-------|-------|------------------|
| `gpu` | GPU die, HBM, NVLink per-GPU | DCGM, NVSentinel, health.proto probes |
| `nvswitch` | NVSwitch tray, NVLink domain | nvswitch-manager gRPC (firmware state, power) |
| `network` | DPU, InfiniBand HCA, Ethernet NIC | forge-dpu-agent probes, machine.is_network_degraded |
| `storage` | Local NVMe, remote storage path | Future: NVMe SMART via site agent |
| `power` | PSU, PDU outlet, powershelf | powershelf-manager gRPC (Sensor with SensorThresholds) |
| `cooling` | Fan, ambient temperature | powershelf-manager gRPC (Sensor readings) |
| `memory` | System RAM (non-HBM) | Future: MCE via site agent |
| `cpu` | Grace CPU | Future: MCE via site agent |
| `bmc` | BMC/IPMI controller | Site agent discovery, Redfish health |

**state** — Fault lifecycle:

```
open → acknowledged → remediating → resolved
  │         │              │
  │         │              └─→ escalated → resolved
  │         │
  └─→ suppressed ──────────────→ open (when suppression expires)
```

**service_event.state** — Tenant-visible lifecycle:

```
active → resolved
```

Tenants see two states only. The internal fault lifecycle
(acknowledged, remediating, escalated, suppressed) is not
exposed.

**service_event.impact** — Human-readable impact description:

| Example | Derived from |
|---------|-------------|
| `"2 of 8 GPUs temporarily unavailable"` | Count of fault_events linked to this service_event with component=gpu |
| `"Network connectivity degraded"` | fault_event with component=network |
| `"Storage performance reduced"` | fault_event with component=storage |

**classification** — Source-specific fault type:

| Source | Example classifications |
|--------|----------------------|
| NVSentinel | `gpu-xid-48` (double-bit ECC), `gpu-xid-79` (fallen off bus), `gpu-thermal` |
| DCGM | `dcgm-gpu-temp-critical`, `dcgm-ecc-dbe`, `dcgm-pcie-replay` |
| NHC | `nhc-gpu-count-mismatch`, `nhc-ib-link-down`, `nhc-memory-error` |
| NICo agent | `agent-site-unreachable`, `agent-inventory-stale` |
| NICo power | `power-psu-fault`, `power-sensor-upper-critical`, `power-sensor-upper-caution` |
| NICo NVSwitch | `nvswitch-link-down`, `nvswitch-firmware-failed`, `nvswitch-power-off` |
| NICo DPU | `dpu-probe-failure`, `dpu-hbn-down`, `dpu-unreachable` |

### API endpoints

#### Operator endpoints (under `/health`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/health/events/ingest` | Receive fault signal (webhook) |
| GET | `/health/events` | List fault events (filtered, paginated, sorted) |
| GET | `/health/events/:id` | Get fault event detail with related machine, site, tenant |
| PATCH | `/health/events/:id` | Update state (acknowledge, suppress, add notes) |
| GET | `/health/events/summary` | Aggregated counts by severity, component, state, site |
| POST | `/health/events/:id/remediate` | Manually trigger remediation workflow |
| GET | `/health/events/:id/remediation` | Get remediation workflow status |
| GET | `/health/classifications` | List fault classifications and remediation mappings |
| PUT | `/health/classifications/:id` | Configure remediation for a classification |

All paths are prefixed with `/v2/org/{org}/carbide/`.

#### Tenant endpoints (under `/tenant`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/tenant/{tenant}/service-events` | List service events for tenant |
| GET | `/tenant/{tenant}/service-events/active` | Active service events only |
| GET | `/tenant/{tenant}/service-events/:id` | Service event detail |

Tenant responses contain only: `summary`, `impact`, `state`,
`started_at`, `estimated_resolution_at`, `resolved_at`,
`downtime_excluded`. No machine IDs, rack locations, XID codes,
GPU UUIDs, classifications, or remediation details.

#### Ingestion payload

```json
{
  "source": "nvsentinel",
  "severity": "critical",
  "component": "gpu",
  "classification": "gpu-xid-48",
  "message": "Double-bit ECC error on GPU 3, tray 7, rack A01",
  "machine_id": "a1b2c3d4-...",
  "detected_at": "2026-04-07T14:32:00Z",
  "metadata": {
    "xid": 48,
    "gpu_index": 3,
    "gpu_uuid": "GPU-abc123",
    "pci_bus_id": "0000:41:00.0",
    "ecc_dbe_count": 1
  }
}
```

The ingestion handler resolves `machine_id` → `site_id`,
`tenant_id`, `instance_id` via the compute service interface
(`.Relation()` on the Machine model). If the machine is
allocated to a tenant, the handler also creates or updates
a `service_event` linked via `fault_service_event`.

#### Internal ingestion (powershelf, NVSwitch, DPU)

For NICo-internal sources, the provider does not use the
webhook endpoint. Instead, it processes health data from the
existing gRPC interfaces:

**Powershelf sensors:**

The provider polls powershelf-manager's `GetPowershelves` RPC
(already called by the site agent). When a `Sensor.reading`
crosses a `SensorThresholds` boundary:

| Threshold | Severity | Classification |
|-----------|----------|---------------|
| `upper_critical` crossed | `critical` | `power-sensor-upper-critical` |
| `upper_caution` crossed | `warning` | `power-sensor-upper-caution` |
| `lower_critical` crossed | `critical` | `power-sensor-lower-critical` |
| `lower_caution` crossed | `warning` | `power-sensor-lower-caution` |
| PSU `power_state` = false | `critical` | `power-psu-fault` |

**NVSwitch state:**

The provider monitors `nvswitch-manager` state:

| Condition | Severity | Classification |
|-----------|----------|---------------|
| `FirmwareUpdateState` = `FAILED` | `warning` | `nvswitch-firmware-failed` |
| Switch unreachable (gRPC timeout) | `critical` | `nvswitch-unreachable` |
| `PowerAction` result = error | `critical` | `nvswitch-power-failure` |

**DPU / site agent probes:**

The existing `HealthProbeAlert` from `health.proto` already
has `id`, `target`, `classifications[]`, `tenant_message`,
and `in_alert_since`. The provider maps these directly:

| health.proto field | fault_event field |
|-------------------|-------------------|
| `source` | `source` (e.g., `nico-dpu`, `nico-agent`) |
| `alert.id` | `classification` |
| `alert.target` | `metadata.target` |
| `alert.classifications[]` | `metadata.probe_classifications` |
| `alert.message` | `message` |
| `alert.in_alert_since` | `detected_at` |

#### Query filters

```
GET /health/events?severity=critical&component=gpu&state=open&site_id=...
GET /health/events?component=power&component=nvswitch  (multi-value)
GET /health/events?detected_after=2026-04-01T00:00:00Z&detected_before=...
GET /health/events?machine_id=...
GET /health/events?sort=detected_at&order=desc&limit=50&offset=0
```

#### Summary response

```json
{
  "by_severity": { "critical": 3, "warning": 12, "info": 5 },
  "by_component": {
    "gpu": 8, "network": 4, "power": 3,
    "nvswitch": 2, "storage": 1, "cooling": 1, "bmc": 1
  },
  "by_state": { "open": 2, "remediating": 1, "resolved": 15, "escalated": 2 },
  "by_site": [
    { "site_id": "...", "site_name": "site-alpha", "open": 1, "critical": 2 }
  ],
  "mttr_minutes": {
    "gpu": 12,
    "network": 8,
    "nvswitch": 25,
    "power": 45
  }
}
```

`mttr_minutes` is computed from resolved faults over the last
30 days: `avg(resolved_at - detected_at)` grouped by component.

### Hook events

New hook events added to the core event catalog:

```go
EventPostHealthEventIngested  = "post-health-event-ingested"
EventPreFaultRemediation      = "pre-fault-remediation"
EventPostFaultRemediation     = "post-fault-remediation"
EventPostFaultResolved        = "post-fault-resolved"
EventPostFaultEscalated       = "post-fault-escalated"
EventPostMachineHealthChanged = "post-machine-health-changed"
```

### Hook registrations

| Type | Event | Action |
|------|-------|--------|
| Async reaction | `post-health-event-ingested` | Start FaultRemediationWorkflow |
| Sync pre-hook | `pre-create-instance` | Block if target machine has open critical fault |
| Async reaction | `post-fault-escalated` | Signal ITSM workflow (via NEP-0003 AAP binding) |
| Async reaction | `post-fault-resolved` | Update showback (exclude downtime from billing) |

The `pre-create-instance` hook queries the fault_event table
for open critical faults on the target machine. If found, it
returns an error with the fault ID and classification,
preventing tenant workload placement on faulty hardware.

### Temporal workflows

#### FaultRemediationWorkflow

The main remediation workflow. Dispatched by the
`post-health-event-ingested` async hook.

```
FaultRemediationWorkflow(faultEventID)
│
├─ Step 1: ClassifyAndRoute
│   │ Read fault_event with .Relation("Machine").Relation("Site").
│   │ Determine remediation strategy from classification →
│   │ remediation mapping table.
│   │
│   │ If no mapping exists → escalate immediately.
│   │ If suppressed → skip.
│   │ If duplicate (same machine + classification within
│   │   dedup window) → link to existing fault, skip.
│   │
│   └─ Update state → "remediating"
│
├─ Step 2: IsolateFault
│   │ Fire pre-fault-remediation sync hook (can abort).
│   │
│   ├─ Set machine maintenance mode
│   │   PATCH /machine/{id} { is_in_maintenance: true,
│   │     maintenance_message: "Automated: {classification}" }
│   │
│   ├─ Cordon K8s node (if machine has instance with K8s)
│   │   Via site agent gRPC or AAP hook
│   │
│   └─ Create or update service_event for affected tenant
│       (if instance allocated)
│
├─ Step 3: Remediate (per component + classification)
│   │
│   ├─ GPU faults:
│   │   ├─ nvidia-smi --gpu-reset --id={gpu_index}
│   │   │   Via site agent activity
│   │   ├─ Wait 30s for GPU re-initialization
│   │   └─ If reset fails → escalate
│   │
│   ├─ NVSwitch faults:
│   │   ├─ Query nvswitch-manager GetNVSwitches for current state
│   │   ├─ If firmware failed → re-queue firmware update
│   │   │   via nvswitch-manager QueueUpdate RPC
│   │   ├─ If link down → PowerControl(POWER_ACTION_POWER_CYCLE)
│   │   │   via nvswitch-manager PowerControl RPC
│   │   ├─ Wait for switch to come back (poll GetNVSwitches)
│   │   └─ If persistent after power cycle → escalate
│   │
│   ├─ Power faults:
│   │   ├─ Query powershelf-manager GetPowershelves for sensor state
│   │   ├─ If PSU fault → check redundancy (N+1 or N+2)
│   │   │   If redundant → warning only, schedule replacement
│   │   │   If not redundant → critical, migrate workloads
│   │   ├─ If sensor threshold → wait for return to normal
│   │   │   (recheck interval from classification mapping)
│   │   └─ If sustained → escalate with sensor readings in metadata
│   │
│   ├─ DPU / Network faults:
│   │   ├─ Query forge-dpu-agent health via site agent
│   │   ├─ If DPU unreachable → DPU reset via BMC (Redfish)
│   │   ├─ If HBN down → restart HBN services on DPU
│   │   │   Via site agent activity
│   │   ├─ Validate overlay connectivity (VxLAN tunnel check)
│   │   └─ If persistent → escalate (may need BFB re-flash)
│   │
│   ├─ Storage faults:
│   │   ├─ Query NVMe SMART attributes via site agent
│   │   ├─ If recoverable error → clear error state
│   │   ├─ If wear level critical → schedule replacement
│   │   └─ If drive failed → mark for RMA, escalate
│   │
│   ├─ BMC faults:
│   │   ├─ Attempt BMC reset via IPMI (ipmitool mc reset cold)
│   │   ├─ Wait 60s for BMC re-initialization
│   │   ├─ Validate Redfish endpoint responds
│   │   └─ If still unreachable → escalate
│   │
│   └─ Cooling faults:
│       ├─ Query powershelf-manager sensor readings
│       ├─ If ambient temp > upper_caution:
│       │   Throttle workloads (reduce GPU power limit via site agent)
│       ├─ If ambient temp > upper_critical:
│       │   Evacuate workloads from affected trays
│       ├─ Wait for temperature to return below threshold
│       └─ If sustained → escalate (HVAC issue, not NICo's domain)
│
├─ Step 4: ValidateRecovery
│   │ Component-specific validation:
│   │
│   ├─ GPU: DCGM diagnostics (Level 2 or 3)
│   │   dcgmi diag --run {validation_level} --gpu {gpu_index}
│   │
│   ├─ NVSwitch: Verify link state via nvswitch-manager
│   │   GetNVSwitches → check firmware version, BMC reachable
│   │
│   ├─ Power: Verify sensor readings within normal range
│   │   GetPowershelves → all sensors below upper_caution
│   │
│   ├─ Network/DPU: Validate RDMA connectivity
│   │   NCCL bandwidth test or ib_write_bw via site agent
│   │
│   ├─ BMC: Validate Redfish system health endpoint
│   │
│   ├─ If validation fails:
│   │   - Increment remediation_attempts
│   │   - If attempts < max_retries → retry Step 3
│   │   - If attempts >= max_retries → escalate
│   │
│   └─ Fire post-fault-remediation hook
│
├─ Step 5: RestoreService
│   │
│   ├─ Remove maintenance mode
│   │   PATCH /machine/{id} { is_in_maintenance: false }
│   │
│   ├─ Uncordon K8s node (if applicable)
│   │
│   ├─ Update fault_event state → "resolved"
│   │   Set resolved_at = now()
│   │
│   ├─ Update service_event state → "resolved"
│   │   Set resolved_at = now()
│   │
│   ├─ Fire post-fault-resolved hook
│   │
│   └─ Compute MTTR, store in metadata
│
└─ Escalation path (from any step):
    │
    ├─ Update fault_event state → "escalated"
    ├─ Increment escalation_level
    ├─ Update service_event.summary with escalation info
    ├─ Fire post-fault-escalated hook
    │   → AAP provider (NEP-0003) creates ITSM ticket
    └─ Log escalation reason in fault_event metadata
```

#### FaultDeduplicationActivity

Before creating a new fault_event, check for an existing open
fault on the same machine with the same classification within
a configurable deduplication window (default: 15 minutes).

If a duplicate is found:
- Increment the existing fault's `remediation_attempts`
- Add the new detection timestamp to metadata
- Do not create a new fault_event
- Do not start a new remediation workflow

This prevents alert storms from generating hundreds of parallel
remediation workflows for the same underlying issue.

#### ServiceEventCorrelationActivity

When a fault_event is created, determine tenant impact:

1. Look up machine → instance → tenant via bun relations
2. If tenant exists, check for an active service_event for
   this tenant with the same component
3. If found → link via fault_service_event (same disruption)
4. If not found → create new service_event with:
   - `summary`: Generated from component and count
   - `impact`: Derived from fault severity and scope
   - `estimated_resolution_at`: From classification mapping's
     `max_retries * recheck_interval` or default MTTR
5. Link fault_event → service_event via fault_service_event

When the last linked fault_event resolves, the service_event
auto-resolves.

#### FaultRetentionWorkflow (cron)

Runs daily. Moves resolved faults older than a configurable
retention period (default: 90 days) to a `fault_event_archive`
table. Resolved service_events are archived together. Keeps
the active tables small for query performance.

### Classification-to-remediation mapping

Operators configure which remediation strategy applies to each
fault classification. Mappings cover all component types:

```yaml
# Default mappings (built into provider)
classifications:
  # GPU faults
  gpu-xid-48:
    component: gpu
    severity: critical
    remediation: gpu-reset
    max_retries: 2
    validation_level: 3

  gpu-xid-79:
    component: gpu
    severity: critical
    remediation: gpu-reset
    max_retries: 1
    validation_level: 3
    escalate_message: "GPU fallen off bus — likely hardware failure"

  gpu-thermal:
    component: cooling
    severity: warning
    remediation: wait-and-recheck
    recheck_interval: 5m
    max_retries: 6
    escalate_message: "Sustained thermal throttling"

  dcgm-ecc-dbe:
    component: gpu
    severity: critical
    remediation: gpu-reset
    max_retries: 1
    validation_level: 3

  dcgm-ecc-sbe-trending:
    component: gpu
    severity: warning
    remediation: monitor
    recheck_interval: 1h
    max_retries: 24
    escalate_message: "SBE count increasing — schedule replacement"

  # NVSwitch faults
  nvswitch-firmware-failed:
    component: nvswitch
    severity: warning
    remediation: nvswitch-firmware-retry
    max_retries: 2
    escalate_message: "Firmware update failed after retries"

  nvswitch-unreachable:
    component: nvswitch
    severity: critical
    remediation: nvswitch-power-cycle
    max_retries: 1
    escalate_message: "NVSwitch unreachable after power cycle"

  # Power faults
  power-psu-fault:
    component: power
    severity: critical
    remediation: power-redundancy-check
    max_retries: 1
    escalate_message: "PSU failure — check redundancy"

  power-sensor-upper-critical:
    component: power
    severity: critical
    remediation: power-wait-and-recheck
    recheck_interval: 2m
    max_retries: 5
    escalate_message: "Sensor reading sustained above critical threshold"

  power-sensor-upper-caution:
    component: power
    severity: warning
    remediation: power-wait-and-recheck
    recheck_interval: 5m
    max_retries: 12

  # Network / DPU faults
  nhc-ib-link-down:
    component: network
    severity: critical
    remediation: dpu-reset
    max_retries: 2
    validation_level: 2

  dpu-unreachable:
    component: network
    severity: critical
    remediation: dpu-bmc-reset
    max_retries: 1
    escalate_message: "DPU unreachable after BMC reset — may need BFB re-flash"

  dpu-hbn-down:
    component: network
    severity: critical
    remediation: dpu-hbn-restart
    max_retries: 2

  # BMC faults
  bmc-unreachable:
    component: bmc
    severity: warning
    remediation: bmc-reset
    max_retries: 2
    escalate_message: "BMC unreachable after cold reset"

  # Storage faults
  storage-nvme-wear-critical:
    component: storage
    severity: warning
    remediation: schedule-replacement
    escalate_message: "NVMe wear level critical — schedule drive replacement"

  storage-nvme-failed:
    component: storage
    severity: critical
    remediation: escalate-immediately
    escalate_message: "NVMe drive failed — RMA required"
```

Operators can override defaults or add custom classifications
via the API:

```
PUT /health/classifications/gpu-xid-48
{
  "remediation": "gpu-reset",
  "max_retries": 3,
  "validation_level": 2,
  "aap_template": "post-gpu-reset-validation"
}
```

The `aap_template` field integrates with NEP-0003: after the
built-in remediation completes, the AAP provider runs the
specified job template for additional NCP-specific validation.

### Webhook receiver authentication

The ingestion endpoint accepts fault signals from external
systems. Authentication options:

| Method | Use case |
|--------|----------|
| Bearer token | AlertManager webhook config with `bearer_token` |
| mTLS | NVSentinel with client certificate |
| HMAC signature | NHC webhook with shared secret (`X-NICo-Signature` header) |

The provider validates the source against a configurable
allowlist of sources and their authentication method. Unknown
sources are rejected with 403.

```yaml
ingestion:
  sources:
    - name: nvsentinel
      auth: mtls
      client_ca: /etc/nico/certs/nvsentinel-ca.pem

    - name: alertmanager
      auth: bearer
      token_secret: nico-alertmanager-webhook-token

    - name: nhc
      auth: hmac
      hmac_secret: nico-nhc-webhook-secret
```

### AlertManager integration

AlertManager sends alerts to NICo's health endpoint via
its webhook receiver configuration:

```yaml
# AlertManager config
receivers:
  - name: nico-health
    webhook_configs:
      - url: https://nico-api.seed.local/v2/org/ncp/carbide/health/events/ingest
        send_resolved: true
        http_config:
          bearer_token_file: /etc/alertmanager/nico-token

route:
  routes:
    - match:
        alertname: DCGMGpuDoubleBitECC
      receiver: nico-health
    - match:
        alertname: DCGMGpuTemperatureCritical
      receiver: nico-health
```

The provider maps AlertManager alert labels to fault_event
fields:

| AlertManager label | fault_event field |
|--------------------|-------------------|
| `alertname` | `classification` (lowercased, prefixed with `dcgm-`) |
| `severity` | `severity` |
| `gpu` | `metadata.gpu_index` |
| `instance` | Resolved to `machine_id` via hostname lookup |
| `namespace` | `metadata.k8s_namespace` |
| `pod` | `metadata.k8s_pod` |

When AlertManager sends `status: resolved`, the provider
auto-resolves the matching open fault_event and its linked
service_event (if no other unresolved faults remain linked).

### NVSentinel integration

NVSentinel classifies GPU faults and takes immediate action
(cordon, drain). The NICo provider receives the classification
after NVSentinel has already isolated the node:

```json
{
  "source": "nvsentinel",
  "severity": "critical",
  "component": "gpu",
  "classification": "gpu-xid-48",
  "message": "NVSentinel: XID 48 double-bit ECC on GPU-abc123",
  "detected_at": "2026-04-07T14:32:00Z",
  "metadata": {
    "xid": 48,
    "gpu_uuid": "GPU-abc123",
    "node_name": "gpu-worker-07",
    "nvsentinel_action": "cordon_drain",
    "nvsentinel_version": "1.0.0"
  }
}
```

Since NVSentinel has already cordoned the node, the
IsolateFault step detects this and skips the cordon activity
(idempotent). The provider focuses on GPU reset, validation,
and service restoration — the steps NVSentinel does not do.

### NHC / RHWA integration

NHC (Node Health Check) runs periodic checks on OpenShift
nodes. When a check fails, it sets a condition on the node.
FAR (Fence Agent Remediation) acts on those conditions.

The integration has two paths:

**Path A: NHC webhook to NICo**

Configure NHC to POST failures to NICo's health ingestion
endpoint. NICo's health provider handles remediation instead
of (or in addition to) FAR.

**Path B: NICo watches NHC conditions**

A future enhancement could have the provider watch K8s node
conditions for NHC-set labels and ingest them as fault events.
This avoids configuring NHC webhooks but requires K8s API
access from NICo (already available via the DPF HCP provider's
dynamic client).

Path A is recommended for the initial implementation because
it does not require NICo to watch K8s state, which is an
inversion of the normal control flow (NICo manages below K8s).

### Interaction with machine health JSONB

The existing `machine.health` JSONB field contains probe
alerts from the site agent. The expanded health provider
reads this data but does not write to it. Instead, it creates
structured fault_event records from the probe data.

A migration activity runs on provider init to scan existing
machines for open health alerts and create corresponding
fault_event records. This bootstraps the fault table from
pre-existing state.

After bootstrapping, the provider fires
`post-machine-health-changed` when the site agent reports a
new health alert, and the ingestion handler creates a
fault_event from the probe data.

## NCP use cases

### Automated GPU recovery

Typical flow for an ECC error on a tenant's allocated GPU:

1. DCGM exporter fires `DCGMGpuDoubleBitECC` alert
2. AlertManager POSTs to NICo health ingestion endpoint
3. Provider creates fault_event (severity=critical,
   component=gpu, classification=dcgm-ecc-dbe)
4. Provider creates service_event for affected tenant:
   "1 GPU temporarily unavailable. Automated recovery in
   progress. ETA: 15 minutes."
5. `post-health-event-ingested` hook starts
   FaultRemediationWorkflow
6. Workflow isolates (maintenance mode, cordon)
7. Workflow resets GPU (`nvidia-smi --gpu-reset`)
8. Workflow validates (DCGM Level 3 diagnostics pass)
9. Workflow restores (remove maintenance, uncordon)
10. fault_event → resolved, service_event → resolved
11. Total time: ~12 minutes (automated) vs ~45 minutes (manual)
12. Tenant sees via service-events API: "GPU unavailable
    14:32–14:44, 12 min downtime excluded from billing"

### Power shelf sensor alert

1. Powershelf manager reports PSU sensor reading above
   `upper_critical` threshold (e.g., temperature 85°C,
   threshold 80°C)
2. Provider creates fault_event (severity=critical,
   component=power, classification=power-sensor-upper-critical)
3. If PSU is redundant (N+1): severity downgraded to warning,
   schedule replacement via AAP
4. If PSU is not redundant: service_events created for all
   tenants on affected trays
5. Remediation: wait-and-recheck (2 min interval, 5 retries)
6. If sustained → escalate → ITSM ticket for HVAC/facilities

### NVSwitch firmware failure

1. NVSwitch manager reports `FIRMWARE_UPDATE_STATE_FAILED`
   after a firmware update attempt
2. Provider creates fault_event (severity=warning,
   component=nvswitch, classification=nvswitch-firmware-failed)
3. Remediation: re-queue firmware update via
   `nvswitch-manager.QueueUpdate` RPC
4. If second attempt fails → escalate
5. Tenant impact: depends on NVLink domain — if training jobs
   span affected tray, service_event created

### DPU link failure

1. forge-dpu-agent health probe reports `HealthProbeAlert`
   with id=`dpu-hbn-connectivity`, classifications=[`hbn-down`]
2. Provider creates fault_event (severity=critical,
   component=network, classification=dpu-hbn-down)
3. Service_event created for tenant: "Network connectivity
   degraded on 1 node"
4. Remediation: restart HBN services on DPU via site agent
5. Validation: VxLAN tunnel connectivity check
6. If persistent → DPU BMC reset → BFB re-flash → escalate

### Multi-fault correlation

When an NVLink domain fails, multiple GPUs report errors
simultaneously:

1. 4 GPUs on a tray report XID 79 within 2 seconds
2. First fault creates fault_event + service_event +
   remediation workflow
3. Next 3 faults are deduplicated (same machine, same
   classification, within 15-minute window)
4. All 4 fault_events link to the same service_event
5. Service_event shows: "4 GPUs temporarily unavailable"
6. Single remediation workflow handles the tray
7. Operator dashboard shows 4 correlated faults, 1 service
   disruption

### Tenant SLA reporting

The service-events API provides data for SLA calculations:

```
GET /tenant/{tenant}/service-events?state=resolved&started_after=2026-03-01
```

Response includes `started_at`, `resolved_at`, and
`downtime_excluded` for each service event. The tenant or
NCP billing system computes:
- Total downtime per service event
- Availability percentage
- Incidents per GPU-month
- Billing adjustments for excluded downtime

The `post-fault-resolved` hook can trigger showback
adjustments: exclude fault duration from billing via the
showback provider.

## Design constraints

**Fault ingestion must be idempotent.** The same alert may be
sent multiple times (AlertManager retries, NVSentinel restarts).
The deduplication activity prevents duplicate fault_events.

**Remediation must not conflict with NVSentinel.** NVSentinel
runs on the node and takes immediate action (cordon, drain).
NICo's remediation starts after NVSentinel has acted. The
IsolateFault step checks current state before acting
(idempotent cordon, idempotent maintenance mode).

**Tenant data must be sanitized.** Tenants see service_events
with `summary`, `impact`, `state`, and timing fields. They do
not see fault_events, machine IDs, rack locations, XID codes,
GPU UUIDs, classifications, or remediation details.

**Remediation workflows must be cancellable.** An operator
can cancel a running remediation workflow via the API. The
Temporal workflow checks for cancellation between steps.

**The fault table must not grow unbounded.** The retention
workflow archives resolved faults after the configured
retention period. Active faults are never archived.

**Internal sources must not duplicate external sources.** If
NVSentinel reports a GPU fault via webhook, the powershelf
manager's sensor data for the same event should be correlated,
not create a separate remediation workflow. The deduplication
activity handles this by matching on machine_id + component +
time window.

## Implementation phases

| Phase | Scope | Component coverage |
|-------|-------|--------------------|
| 1 | `fault_event` + `service_event` + `fault_service_event` table migrations | All |
| 2 | Health ingestion endpoint, query/filter API, summary | All |
| 3 | Service event API (tenant-facing) | All |
| 4 | Hook events (`post-health-event-ingested`, `post-machine-health-changed`, etc.) | All |
| 5 | GPU remediation workflow (reset → DCGM validate → restore) | GPU |
| 6 | Classification-to-remediation mapping API | All |
| 7 | AlertManager + NVSentinel webhook integration | GPU |
| 8 | Internal ingestion: powershelf sensor threshold monitoring | Power, Cooling |
| 9 | Internal ingestion: NVSwitch state monitoring | NVSwitch |
| 10 | Internal ingestion: DPU/site-agent health probe mapping | Network, DPU |
| 11 | NVSwitch remediation (firmware retry, power cycle) | NVSwitch |
| 12 | Power remediation (redundancy check, wait-and-recheck) | Power |
| 13 | DPU/Network remediation (DPU reset, HBN restart, BFB re-flash) | Network |
| 14 | BMC remediation (IPMI cold reset) | BMC |
| 15 | Storage remediation (SMART monitoring, RMA scheduling) | Storage |
| 16 | Cooling remediation (throttle, evacuate) | Cooling |
| 17 | Fault retention workflow (cron, archival) | All |
| 18 | AAP integration for ITSM escalation (via NEP-0003 hooks) | All |

Phases 1-5 deliver the core value: structured fault tracking,
tenant service events, and automated GPU recovery. Phases 6-7
add external integration. Phases 8-16 extend to all NVL72
components. Phases 17-18 add operational maturity.

## Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Alert storm generates hundreds of faults | Remediation overload | Deduplication activity, rate limiting on ingestion |
| GPU reset fails silently | Machine returned to service with faulty GPU | DCGM Level 3 validation mandatory before restore |
| NVSentinel and NICo both try to remediate | Conflicting actions | NICo checks node state before acting; NVSentinel acts first |
| Tenant sees internal fault details | Information leak | Separate service_event model; tenant API has no access to fault_events |
| Remediation workflow stuck | Machine in maintenance indefinitely | Workflow timeout (configurable, default 1 hour); escalation on timeout |
| Fault table grows too large | Query performance degrades | Retention workflow; indexed FK columns; archive table |
| Webhook endpoint abused | DoS on NICo | Source allowlist, authentication, rate limiting |
| Powershelf sensor polling too frequent | Unnecessary load | Configurable poll interval; threshold comparison with hysteresis |
| Multiple sources report same fault | Duplicate remediation | Dedup by machine_id + component + time window across sources |

## Alternatives considered

| Alternative | Why not |
|---|---|
| Create a separate `nico-faultmgmt` provider | Splits the health domain; fault is a health state change, not a separate concept |
| Use AlertManager as the fault database | No structured lifecycle (acknowledge, remediate, resolve); no tenant scoping; no remediation orchestration |
| Use NHC/FAR as the sole remediation engine | FAR handles node-level remediation (reboot, fence) but not component-specific recovery (GPU reset, NVSwitch power cycle, DPU HBN restart); no tenant visibility |
| Build fault management as a separate microservice | Duplicates auth, tenancy, database; NEP-0001 exists precisely to avoid this |
| Store faults in machine.health JSONB | Unstructured, not queryable by severity/component/state, no lifecycle tracking, no tenant scoping |
| Use Kubernetes Events for fault tracking | NICo operates below K8s; fault data must persist independent of cluster lifecycle |
| Denormalize machine/site/tenant names into fault_event | Goes against NICo's established pattern; all 42 models use bun `rel:belongs-to` JOINs for this |

## References

- [NEP-0001: Extensible Architecture](0001-extensible-architecture.md)
- [NEP-0003: AAP Provider](0003-aap-provider.md)
- [NVIDIA NVSentinel](https://docs.nvidia.com/nvsentinel/)
- [NVIDIA DCGM](https://docs.nvidia.com/datacenter/dcgm/)
- [Red Hat Workload Availability](https://docs.openshift.com/container-platform/latest/nodes/nodes/eco-node-health-check-operator.html)
- [AlertManager Webhook Configuration](https://prometheus.io/docs/alerting/latest/configuration/#webhook_config)
- [NICo health.proto](../../rla/internal/carbideapi/carbideproto/health.proto)
- [NICo powershelf-manager.proto](../../powershelf-manager/internal/proto/v1/powershelf-manager.proto)
- [NICo nvswitch-manager.proto](../../nvswitch-manager/internal/proto/v1/nvswitch-manager.proto)
