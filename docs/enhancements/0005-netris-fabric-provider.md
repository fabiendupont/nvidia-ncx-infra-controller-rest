# NEP-0005: Netris Fabric Management Provider

| Field | Value |
|-------|-------|
| **Title** | Netris Fabric Management Provider |
| **Status** | Implemented (prototype) |
| **Authors** | Red Hat NCP Team |
| **Created** | 2026-04-07 |
| **Target Release** | TBD |
| **Branch** | `extensible-architecture` |
| **Depends on** | NEP-0001 (Extensible Architecture) |

## Summary

Add a `netris-fabric` provider that syncs NICo tenant networking
events to the Netris SDN Controller for physical switch fabric
configuration. The provider is **complementary** to NICo's
built-in networking — it manages the physical layer (switches)
while NICo manages the tenant layer (VPCs, subnets).

## Motivation

### The integration model

Netris is a network automation platform that manages physical
data center switches (NVIDIA Spectrum-X, SONiC, Arista). It
provides cloud-like VPC/VNET abstractions at the fabric level.

In an NCP deployment with Netris, four control planes coexist:

| Layer | Controller |
|-------|-----------|
| Physical switches | Netris |
| DPU hardware + OS | DPF |
| Tenant networking (VPC/Subnet) | NICo |
| Workload networking (pods) | OpenShift OVN-K |

Without integration, operators must manually ensure the Netris
fabric configuration matches NICo's tenant intent. When NICo
creates a VPC, the operator must separately create a matching
Netris VPC (VRF) on the switches.

### Why complementary, not alternative

Netris does NOT replace NICo's networking provider. NICo manages
tenant-level constructs (VPC, Subnet, NSG, Interface) and
delegates to carbide-api on each site for DPU programming. Netris
manages the physical switches those DPUs connect to.

An earlier design explored implementing `networkingsvc.Service`
with Netris as the backend. This was rejected because:

- Netris operates at a different abstraction level (physical
  fabric vs tenant constructs)
- NICo's networking provider handles DPU programming, NSGs,
  InfiniBand partitions — features Netris doesn't have
- The integration boundary should be event-driven, not a
  wholesale replacement of the networking stack

### Friction points addressed

The Netris partnership assessment identified key friction points
when DPF and Netris coexist:

1. **IPAM coordination** — Netris IPAM must not conflict with
   NICo's IP allocations. The provider validates prefixes via
   a sync pre-hook before subnet creation.

2. **Lifecycle desync** — When DPF reboots a DPU, Netris sees
   a port flap. Hook-driven awareness prevents auto-disable.

3. **BGP ownership** — Configuration policy, tracked via provider
   metadata. Not solved by this provider but the hook system
   enables future coordination.

## Design

### Provider identity

| Field | Value |
|-------|-------|
| Name | `netris-fabric` |
| Version | `0.1.0` |
| Features | `["fabric"]` |
| Dependencies | `["nico-networking"]` |
| Interfaces | None (hook-driven only, no REST routes) |
| Profile | Not in any default profile |

### Hook registrations

| Type | Event | Action |
|------|-------|--------|
| Async reaction | `post-create-vpc` | Create Netris VPC (VRF) on switches |
| Async reaction | `post-delete-vpc` | Remove Netris VPC |
| Async reaction | `post-create-subnet` | Create Netris VNET on switches |
| Async reaction | `post-delete-subnet` | Remove Netris VNET |
| Sync pre-hook | `pre-create-subnet` | Validate no IPAM conflict with Netris |

### Fabric sync operations

| Operation | NICo concept | Netris concept |
|-----------|-------------|---------------|
| SyncVPCToFabric | VPC | VPC (VRF) |
| SyncSubnetToFabric | Subnet | VNET (L2/L3 segment) |
| ConfigurePortForMachine | Machine NIC | Switch port (VLAN, MTU) |
| RemoveVPCFromFabric | VPC deletion | VPC deletion |
| RemoveSubnetFromFabric | Subnet deletion | VNET deletion |

All operations are idempotent (check-before-act pattern).

### ID mapping

NICo uses UUIDs; Netris uses integer IDs. The provider maintains
a bidirectional mapping with a thread-safe in-memory map. In
production, this would be persisted in the database.

### IPAM conflict detection

The `pre-create-subnet` sync hook queries Netris IPAM (allocations
and subnets) and checks for CIDR overlap with the requested
prefix. Uses `net.Contains` for overlap detection. Aborts subnet
creation with a descriptive error if a conflict is found.

### Netris client

REST client for the Netris Controller API:

| Resource | Endpoints | Operations |
|----------|-----------|-----------|
| VPC | `/api/v2/vpc` | Create, Get, List, Delete |
| VNET | `/api/v2/vnet` | Create, Get, List, Delete |
| Port | `/api/v2/port` | Get, Update, List |
| Allocation | `/api/v2/ipam/allocation` | Create, List |
| Subnet | `/api/v2/ipam/subnet` | Create, List, Delete |

Authenticates via POST `/api/auth` with username/password.
Credentials from `NETRIS_URL`, `NETRIS_USERNAME`, `NETRIS_PASSWORD`
environment variables.

## Configuration

Operators add the provider to a custom profile:

```go
provider.RegisterProfileProviders("ncp-netris", []func() provider.Provider{
    // ... standard NCP providers ...
    func() provider.Provider { return netrisfabric.NewFromEnv() },
})
```

Or via future config-file-driven profiles:

```yaml
profile: ncp-netris
providers:
  - name: netris-fabric
    config:
      url: https://netris.example.com
      username: admin
```

## References

- [NEP-0001: Extensible Architecture](0001-extensible-architecture.md)
- [Netris Documentation](https://www.netris.io/docs/en/latest/)
