# ADR-0059: Weighted Read-Replica Routing (Latency-Aware) — Design Gate

- Date: 2026-06-03
- Status: Proposed
- Authors: @phil

## Context

ROADMAP "Next (2.x line) → Features → Weighted read-replica routing
(latency-aware)". The goal is to send reads to the lowest-latency
replica. The design gate exists because **read routing in Valkey/Redis is
traditionally client-driven, which structurally limits what an operator
can do.**

Key constraint to anchor on:

- Read-from-replica is a **client decision** (`READONLY` on the
  connection, client-side replica selection). The server does not route
  reads to replicas on the client's behalf. An operator therefore cannot
  *make* a client read from a nearby replica — it can only **expose the
  topology and weights** and let the client (or a proxy) choose.
- Today the operator publishes Services for the workload
  (`internal/resources/service.go`); it does not differentiate
  read-replica endpoints or attach topology/latency metadata.

So the realistic operator contribution is *endpoint + weight exposure*,
not active per-request routing — unless a proxy/sidecar is introduced,
which is a much larger commitment.

## Decision

**Proposed — gate the routing design. The decision is to require this
ADR; the routing mechanism is an Open Question. A concrete *minimal
slice* is proposed below as the likely starting point, but no
implementation is committed here.**

Mechanisms under consideration (increasing operator involvement):

1. **EndpointSlice topology hints (static).** Operator emits a
   replica-only read Service and lets Kubernetes
   topology-aware routing / topology hints steer clients to
   same-zone endpoints. Static (zone-level), not true per-request latency
   — but zero new data-path component and native to Kubernetes.
2. **Sidecar / proxy (dynamic, latency-aware).** A per-pod or per-client
   proxy measures latency and routes reads. True latency-awareness, but
   introduces a data-path component the operator must own, deploy, and
   keep reliable — a large scope and reliability commitment.
3. **Documentation-only.** Operator exposes the replica endpoints; users
   configure client-side replica routing themselves. Smallest scope;
   honest about the client-driven nature of read routing.

### Proposed minimal slice (starting point, not a commitment)

A **replica-only read Service + EndpointSlice topology hints**. This
delivers zone-aware read locality with no new data-path component.
**True latency-aware (dynamic) routing is explicitly lower priority** —
it is the sidecar/proxy path and should only be pursued if the static
slice proves insufficient with evidence.

## Consequences

Positive:

- A replica-only read Service is independently useful (clean
  read/write endpoint separation) even before any weighting.
- Topology hints give zone-level locality "for free" via Kubernetes.

Negative / trade-offs:

- **Static ≠ latency-aware.** Topology hints route by zone, not measured
  latency; they will not satisfy a strict "lowest-latency replica"
  requirement. Setting expectations matters.
- **Dynamic routing = data-path ownership.** A sidecar/proxy makes the
  operator responsible for a serving-path component (upgrades, failure
  modes, overhead). This is the dominant cost of the latency-aware
  variant.
- **Client cooperation still required.** Even with a read Service, the
  client must opt into reading from it / `READONLY`; the operator cannot
  force it.

## Alternatives Considered

1. **Documentation-only client-side routing.** Cheapest, most honest
   about read-routing being client-driven; provides no operator
   ergonomics beyond endpoint exposure.
2. **EndpointSlice topology hints (static).** Native, no data-path
   component; zone-level locality only. Proposed as the minimal slice.
3. **Sidecar/proxy (dynamic latency-aware).** Meets the literal
   latency-aware goal; large reliability/scope cost. Deferred behind the
   static slice.
4. **Server-side routing in Valkey.** Not available — read routing is not
   a server responsibility in Valkey. Out of the operator's control.

## Open Questions (resolve before implementation)

- Is **zone-level (static)** locality acceptable, or is **measured
  per-request latency** a hard requirement? This decides static vs
  sidecar.
- If a proxy/sidecar is required, is the operator the right owner of a
  data-path component, given it is off the data path today?
- How are "weights" expressed and consumed — EndpointSlice hints,
  annotations, or a Spec field — and which clients actually honor them?
- Interaction with failover (ADR-0017): when a replica is promoted, how
  do read endpoints/weights update without serving stale reads?
- Does this compose with delegated federation routing (ADR-0056
  alternative 2) for cross-cluster read locality?

## Refs

- ROADMAP "Next (2.x line) → Features → Weighted read-replica routing"
- `internal/resources/service.go` (current Service exposure — no
  read-replica differentiation)
- ADR-0017 (replication failover — read endpoints must track promotion)
- ADR-0056 (multi-cluster federation — delegated-routing relationship)
- Kubernetes EndpointSlice topology-aware routing (static mechanism
  candidate)
