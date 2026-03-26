# NEP-0008: External Provider Sidecar Protocol

| Field | Value |
|-------|-------|
| **Title** | External Provider Sidecar Protocol |
| **Status** | Proposal |
| **Authors** | Red Hat NCP Team |
| **Created** | 2026-04-08 |
| **Target Release** | TBD |
| **Depends on** | NEP-0001 (Extensible Architecture) |

## Summary

Define a gRPC-based protocol that enables partner providers to
run as sidecar containers alongside NICo, communicating over Unix
domain sockets. Partners ship a container image with their
provider binary. NICo discovers, connects, and integrates the
provider at startup — no recompilation, no code merge, no fork.

This extends NEP-0001's built-in provider model with a runtime
extension mechanism. Built-in providers (compiled Go) and external
providers (gRPC sidecars) coexist in the same registry, share the
same hook system, and appear identical to API consumers.

## Motivation

### The recompilation problem

NEP-0001 requires providers to be compiled into the NICo binary.
Adding a partner provider means:

1. Partner writes Go code in NICo's module
2. Code is reviewed and merged
3. NICo binary is rebuilt
4. New container image is shipped

This creates a coupling between NICo's release cycle and partner
development. A partner who wants to ship a storage provider must
wait for NVIDIA to merge and release it.

### What partners actually need

Partners want to:

- Ship their provider as a **container image** on their own registry
- Update their provider independently of NICo releases
- Write in **any language** (not just Go)
- Integrate with NICo's auth, tenancy, hooks, and capability
  discovery without understanding NICo internals
- Have their endpoints appear in the OpenAPI spec and SDK

### Prior art

| System | Extension model | Communication |
|--------|----------------|---------------|
| Envoy | WASM filters, ext_proc gRPC | gRPC over UDS |
| Kubernetes | Webhooks, CSI drivers, CRI | gRPC over UDS |
| Containerd | Snapshotter plugins | gRPC over UDS (TTRPC) |
| HashiCorp (Vault, Terraform) | go-plugin (gRPC) | gRPC over UDS |
| CoreDNS | External plugins | gRPC over UDS |

The pattern is consistent: gRPC over Unix domain sockets for
local, low-latency plugin communication. NICo follows the same
model.

## Design

### Deployment model

Partner providers run as sidecar containers in the same Pod as
NICo. They share a volume for Unix domain sockets:

```yaml
# Kubernetes Pod spec (from Helm chart)
containers:
  - name: nico-api
    image: nvcr.io/nvidia/nico-rest:1.0.6
    volumeMounts:
      - name: provider-sockets
        mountPath: /var/run/nico/providers

  - name: netris-fabric
    image: registry.netris.io/nico-provider:1.0.0
    env:
      - name: NICO_PROVIDER_SOCKET
        value: /var/run/nico/providers/netris-fabric.sock
      - name: NETRIS_URL
        value: https://netris.controller.local
      - name: NETRIS_USERNAME
        valueFrom:
          secretKeyRef:
            name: netris-credentials
            key: username
    volumeMounts:
      - name: provider-sockets
        mountPath: /var/run/nico/providers

volumes:
  - name: provider-sockets
    emptyDir: {}
```

### gRPC service definition

```protobuf
syntax = "proto3";
package nico.provider.v1;

// NicoProvider is the interface that external providers implement.
// NICo core connects to the provider's Unix domain socket and
// calls these methods during startup and request handling.
service NicoProvider {
  // ---- Lifecycle ----

  // GetInfo returns provider metadata (name, version, features,
  // dependencies). Called once at startup during discovery.
  rpc GetInfo(GetInfoRequest) returns (ProviderInfo);

  // Init initializes the provider with core context (DB connection
  // string, Temporal endpoint, etc.). Called after dependency
  // resolution, in topological order.
  rpc Init(InitRequest) returns (InitResponse);

  // Shutdown gracefully stops the provider.
  rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);

  // ---- API Routes ----

  // GetRoutes returns the HTTP routes this provider handles.
  // NICo registers proxy handlers for each route.
  rpc GetRoutes(GetRoutesRequest) returns (RouteList);

  // HandleRequest processes an HTTP request forwarded by NICo.
  // Auth middleware has already run — the request includes
  // tenant context headers.
  rpc HandleRequest(HTTPRequest) returns (HTTPResponse);

  // ---- Hooks ----

  // GetHookRegistrations returns the hooks this provider wants
  // to register (sync hooks and async reactions).
  rpc GetHookRegistrations(GetHookRegistrationsRequest)
      returns (HookRegistrationList);

  // HandleSyncHook is called inline during an activity when a
  // sync hook fires. Must return within the activity timeout.
  rpc HandleSyncHook(HookEvent) returns (HookResult);

  // ---- OpenAPI ----

  // GetOpenAPIFragment returns this provider's OpenAPI spec
  // fragment. NICo merges it into the complete spec at startup.
  rpc GetOpenAPIFragment(GetOpenAPIFragmentRequest)
      returns (OpenAPIFragment);
}

// ---- Messages ----

message GetInfoRequest {}

message ProviderInfo {
  string name = 1;
  string version = 2;
  repeated string features = 3;
  repeated string dependencies = 4;
}

message InitRequest {
  // Connection details for NICo services the provider can call
  string temporal_endpoint = 1;
  string temporal_namespace = 2;
  string temporal_queue = 3;

  // Service interface endpoints (gRPC) for cross-domain access
  string networking_service_endpoint = 4;
  string compute_service_endpoint = 5;

  // Provider-specific config (from providers.yaml)
  map<string, string> config = 10;
}

message InitResponse {
  bool success = 1;
  string error_message = 2;
}

message ShutdownRequest {}
message ShutdownResponse {}

// ---- Routes ----

message GetRoutesRequest {}

message Route {
  string method = 1;    // GET, POST, PATCH, DELETE
  string path = 2;      // e.g., "/storage/volumes"
}

message RouteList {
  repeated Route routes = 1;
}

message HTTPRequest {
  string method = 1;
  string path = 2;
  map<string, string> headers = 3;
  map<string, string> query_params = 4;
  map<string, string> path_params = 5;
  bytes body = 6;

  // Tenant context injected by NICo auth middleware
  string org = 10;
  string tenant_id = 11;
  string user_id = 12;
  repeated string roles = 13;
}

message HTTPResponse {
  int32 status_code = 1;
  map<string, string> headers = 2;
  bytes body = 3;
}

// ---- Hooks ----

message GetHookRegistrationsRequest {}

message HookRegistration {
  string feature = 1;
  string event = 2;
  enum Type {
    SYNC = 0;
    ASYNC = 1;
  }
  Type type = 3;
  // For async reactions
  string target_workflow = 4;
  string signal_name = 5;
}

message HookRegistrationList {
  repeated HookRegistration registrations = 1;
}

message HookEvent {
  string feature = 1;
  string event = 2;
  bytes payload = 3;  // JSON-encoded
}

message HookResult {
  bool success = 1;
  string error_message = 2;  // Non-empty = abort (for pre-hooks)
}

// ---- OpenAPI ----

message GetOpenAPIFragmentRequest {}

message OpenAPIFragment {
  bytes spec_yaml = 1;  // YAML-encoded OpenAPI 3.x fragment
}
```

### Discovery and loading

At startup, NICo discovers external providers from config:

```yaml
# /etc/nico/providers.yaml
providers:
  # Built-in (compiled in, loaded via Go interface)
  - name: nico-networking
    type: builtin

  # External (sidecar, loaded via gRPC)
  - name: netris-fabric
    type: external
    socket: /var/run/nico/providers/netris-fabric.sock
    timeout: 30s

  # Auto-discover all .sock files in a directory
  - name: auto-discover
    type: discover
    directory: /var/run/nico/providers/
    exclude:
      - "nico-*"  # Don't discover built-in provider sockets
```

Loading sequence:

1. Load built-in providers (same as today)
2. Scan socket directory for external providers
3. Connect to each socket, call `GetInfo`
4. Register in the same `Registry` as built-in providers
5. Resolve dependencies (built-in and external mixed)
6. Call `Init` on external providers (with service endpoints)
7. Call `GetRoutes` → register proxy handlers in Echo
8. Call `GetHookRegistrations` → register in hook registry
9. Call `GetOpenAPIFragment` → merge into spec

### Route proxying

NICo registers an Echo handler for each external route that
proxies the HTTP request over gRPC:

```go
func (p *ExternalProvider) proxyHandler(c echo.Context) error {
    req := &pb.HTTPRequest{
        Method:      c.Request().Method,
        Path:        c.Path(),
        Headers:     extractHeaders(c.Request()),
        QueryParams: extractQueryParams(c.QueryParams()),
        PathParams:  extractPathParams(c),
        Body:        readBody(c.Request()),
        Org:         c.Get("org").(string),
        TenantID:    c.Get("tenant_id").(string),
        UserID:      c.Get("user_id").(string),
        Roles:       c.Get("roles").([]string),
    }

    resp, err := p.grpcClient.HandleRequest(c.Request().Context(), req)
    if err != nil {
        return echo.NewHTTPError(502, "provider unavailable")
    }

    for k, v := range resp.Headers {
        c.Response().Header().Set(k, v)
    }
    return c.Blob(int(resp.StatusCode), "application/json", resp.Body)
}
```

The client sees a normal NICo response. Auth middleware runs
before the proxy. The external provider receives tenant context
in the request headers — it doesn't handle auth.

### Cross-domain access for external providers

External providers need to call NICo's service interfaces
(e.g., a storage provider checking VPC existence). NICo exposes
the service interfaces as gRPC services that external providers
call back into:

```protobuf
// Exposed by NICo core for external providers to call
service NicoNetworkingService {
  rpc GetVpcByID(GetVpcRequest) returns (Vpc);
  rpc GetSubnets(GetSubnetsRequest) returns (SubnetList);
  // ... mirrors networkingsvc.Service
}

service NicoComputeService {
  rpc GetInstanceByID(GetInstanceRequest) returns (Instance);
  rpc GetMachineByID(GetMachineRequest) returns (Machine);
  // ... mirrors computesvc.Service
}
```

The endpoints are passed to the provider via `InitRequest`.
This avoids giving external providers direct DB access.

### Temporal workflows for external providers

External providers that need Temporal workflows run their own
Temporal worker (same cluster, own task queue). NICo doesn't
proxy Temporal — the provider connects directly:

```
NICo Temporal cluster
├── cloud queue     ← NICo built-in workflows
├── site queue      ← NICo site agent workflows
├── health-tasks    ← health provider workflows
├── fulfillment-tasks ← fulfillment workflows
└── netris-tasks    ← external netris-fabric provider
```

The external provider gets the Temporal endpoint in `InitRequest`
and creates its own client and worker. For async reactions, NICo
signals the provider's workflow directly (same as built-in
providers).

### OpenAPI spec merging

At startup, NICo collects `GetOpenAPIFragment` from each external
provider and merges them into the complete spec:

```
openapi/spec.yaml (core + built-in provider endpoints)
  +
netris-fabric OpenAPIFragment (fabric sync endpoints)
  +
vast-storage OpenAPIFragment (storage volume endpoints)
  ↓
/v2/org/{org}/carbide/openapi.yaml (merged, served at runtime)
```

The merged spec is served at a new endpoint:

```
GET /v2/openapi.yaml → complete spec with all active providers
```

SDK clients can fetch the spec at runtime and discover all
available endpoints, including those from external providers.

### Security model

| Concern | How it's handled |
|---------|-----------------|
| Auth | NICo middleware runs before proxy — provider receives validated tenant context |
| Network isolation | Unix domain socket (pod-local, no network exposure) |
| DB access | No direct access — service interfaces via gRPC |
| Temporal access | Direct connection (same trust boundary as NICo) |
| Secret management | Provider reads secrets via K8s env vars / volume mounts |
| Resource limits | Container resource limits in Pod spec |
| Health checks | NICo calls `GetInfo` periodically — unhealthy providers are deregistered |

### What a partner ships

A partner provider consists of:

1. **Container image** — gRPC server binary implementing `NicoProvider`
2. **OpenAPI fragment** — embedded in the container, served via `GetOpenAPIFragment`
3. **Helm chart values** — documentation for the operator to enable the provider
4. **README** — configuration, prerequisites, supported features

Example partner SDK structure:

```
netris-nico-provider/
├── cmd/provider/main.go      ← gRPC server entry point
├── internal/
│   ├── provider.go           ← NicoProvider implementation
│   ├── routes.go             ← HTTP handlers
│   ├── hooks.go              ← Hook handlers
│   └── netris/               ← Netris API client
├── openapi-fragment.yaml     ← Endpoint definitions
├── Dockerfile
├── helm/
│   └── values-example.yaml
└── README.md
```

The partner writes ~500 lines of Go (or Python, Rust, Java —
any gRPC language). They don't import NICo, don't understand
NICo internals, and don't need NVIDIA to merge their code.

### Performance

| Operation | Latency | Notes |
|-----------|---------|-------|
| gRPC over UDS (request proxy) | ~0.1-0.5ms | Comparable to in-process function call overhead |
| Sync hook (HandleSyncHook) | ~0.1-0.5ms | Same UDS path |
| GetInfo / GetRoutes | One-time at startup | Not on hot path |
| HandleRequest (full HTTP proxy) | ~1-2ms overhead | Serialization + UDS + deserialization |

For comparison, a REST API call to an external service (Netris,
VAST) takes 10-100ms. The gRPC proxy overhead is negligible
relative to the actual provider work.

## Migration path

The external provider model is **additive** — it doesn't replace
built-in providers. The migration path:

| Phase | Model | Who |
|-------|-------|-----|
| NEP-0001 (current) | Built-in Go providers, compiled in | NVIDIA, Red Hat |
| Config-driven loading | Built-in, selected via YAML config | NVIDIA, Red Hat |
| External sidecars (this NEP) | Built-in + external, mixed | Partners |
| Provider SDK | Partner SDK with templates and conformance tests | Partners (self-service) |

Built-in providers are always faster (no serialization overhead)
and have full DB transaction support. They're the right choice
for core infrastructure providers. External providers are for
partner integrations and optional capabilities where the
flexibility outweighs the overhead.

### Converting between models

A built-in provider can be extracted to an external provider
without API changes:

```
Built-in: handler → service interface → DAO → PostgreSQL
External: proxy → gRPC → provider container → Netris API
```

The route paths, request/response schemas, and hook registrations
are identical. The difference is the communication channel.

## Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Provider container crashes | Routes return 502 | Health check + deregister; NICo core continues |
| gRPC serialization overhead | Higher latency | UDS minimizes network overhead; benchmark shows <1ms |
| Version mismatch (proto) | Incompatible messages | Proto versioning (v1, v2); backward-compatible fields |
| Provider holds request too long | Activity timeout | Per-route timeout config; NICo cancels context |
| Too many sidecar containers | Pod resource pressure | Resource limits per container; limit number of external providers |
| DB transaction loss | Inconsistency on failure | Document which operations need transactions; those stay built-in |
| Socket file permission | Unauthorized access | PodSecurityContext; shared volume mount |

## Alternatives considered

| Alternative | Why not |
|---|---|
| HTTP webhooks (like K8s admission webhooks) | Higher latency; requires TLS for security; no bidirectional streaming |
| WASM plugins (like Envoy) | Limited language support; no access to Temporal; sandboxing constraints |
| Shared library (Go plugin .so) | Fragile (exact Go version match); no multi-language support; poor debugging |
| REST API gateway (like Kong/Ambassador) | Adds infrastructure complexity; doesn't integrate with hooks or Temporal |
| CRD-based (like K8s operators) | NICo operates below K8s; can't depend on K8s API server for core functionality |

## References

- [NEP-0001: Extensible Architecture](0001-extensible-architecture.md)
- [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin) (gRPC plugin model)
- [Kubernetes CSI specification](https://github.com/container-storage-interface/spec) (gRPC over UDS)
- [Envoy External Processing](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter) (gRPC filter)
- [containerd TTRPC](https://github.com/containerd/ttrpc) (low-overhead gRPC variant)
