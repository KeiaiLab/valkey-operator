# ADR-0056: Multi-Cluster Federation — ValkeyFederation CRD (Design Gate)

- Date: 2026-06-03
- Status: Proposed
- Authors: @phil

## Context

ROADMAP "Next (2.x line) → Features → Multi-cluster federation":
Separate ClusterRoles · Topology-aware routing · new CRD
`ValkeyFederation`. This is an **XL, net-new product surface** —
`workflow.md §7.5` requires a design-gate ADR *before* any federation
code.

Current reality the design must start from:

- The operator runs against **a single in-cluster Kubernetes API**:
  `cmd/main.go:231` builds the manager from `ctrl.GetConfigOrDie()` and
  all five controllers reconcile resources in that one cluster. There is
  no concept of a remote cluster, a remote kubeconfig, or cross-cluster
  identity.
- The Valkey data clients (`internal/valkey/`) dial pod IPs/Services
  resolved inside the local cluster only.

A federation introduces a fundamentally different control surface
(multiple clusters, multiple trust domains, cross-cluster networking)
that does not exist anywhere in the codebase today.

### Governance tension to record (Non-Goals vs federation)

The ROADMAP Non-Goals deliberately scope **multi-tenancy isolation** out
("namespace-level only … stronger isolation belongs to a separate
cluster"), while the federation item itself lists **Separate
ClusterRoles**. These two intents are in tension and must be reconciled
explicitly: a federation that spans clusters must define whether it is a
single-tenant fabric (one owner, many clusters) or whether "Separate
ClusterRoles" implies a step toward cross-tenant separation — which the
Non-Goals forbid. The ADR flags this; it does not resolve it.

## Decision

**Proposed — frame the federation as a new `ValkeyFederation` CRD whose
topology, auth model, and routing ownership are Open Questions. No CRD
schema, no controller, no RBAC is defined in this ADR.** The decision is
to require this gate; the architecture is undecided.

Axes the design must commit to before implementation:

1. **Topology model** — hub-spoke (one control cluster owns spokes) vs
   mesh (peers, no central owner). Drives failure domains, the CRD's
   placement, and who holds the source of truth.
2. **Cross-cluster authentication** — how a control plane in cluster A is
   trusted by cluster B. Candidates: per-cluster kubeconfig Secrets,
   workload identity federation, a mesh (e.g. service mesh / overlay)
   trust domain. This is the highest-risk axis (it is a new credential
   surface; per `enforcement.md §self-repair`, credential design is not
   self-repair scope and needs explicit sign-off).
3. **Routing ownership** — does the operator route cross-cluster traffic
   *in-operator* (it becomes a data-path component), or does it only
   *publish topology* and delegate routing to clients / a mesh / global
   load balancer (`delegated`)? In-operator routing pulls the operator
   onto the data path, a large reliability and scope commitment.

## Consequences

Positive (if pursued well):

- A first-class federation CRD gives users a declarative multi-cluster
  Valkey footprint instead of hand-wiring per-cluster operators.

Negative / trade-offs:

- **Scope explosion.** Cross-cluster networking, identity, and partial
  failure handling are each large subsystems with no current foothold in
  the code.
- **New trust surface.** Cross-cluster credentials are a security
  decision requiring explicit human approval, not autonomous design.
- **Reliability blast radius.** In-operator routing would make the
  operator a data-path dependency; an operator outage could degrade
  serving, which is not true today.
- **Non-Goals friction.** "Separate ClusterRoles" risks drifting toward
  the multi-tenancy isolation the ROADMAP explicitly excludes.

## Alternatives Considered

1. **No federation CRD — document a manual multi-operator pattern.**
   Users run one operator per cluster and stitch topology externally.
   Cheapest; preserves single-cluster simplicity. Strong default if the
   Open Questions do not resolve cleanly.
2. **Delegated routing only (publish topology, never route).** The
   operator exposes endpoints/weights and topology hints; all routing is
   client- or mesh-driven. Keeps the operator off the data path; smaller
   surface than in-operator routing. Pairs naturally with ADR-0059
   (weighted read-replica routing).
3. **Mesh-native federation (rely on an external service mesh).**
   Offload cross-cluster identity + routing to the mesh; the CRD only
   declares intent. Reduces in-operator complexity but adds a hard
   external dependency.

## Open Questions (resolve before implementation)

- Hub-spoke or mesh? Where does the `ValkeyFederation` object live, and
  what is the single source of truth?
- Cross-cluster auth model — kubeconfig Secrets, workload identity, or
  mesh trust domain? Who provisions and rotates those credentials?
- Routing: in-operator vs delegated? If in-operator, what is the data-
  path SLA the operator must now meet?
- How does "Separate ClusterRoles" coexist with the Non-Goals
  multi-tenancy exclusion — single-tenant fabric only?
- Failure semantics: what happens to a spoke when the hub (or a peer) is
  unreachable? Split-brain handling for cross-cluster Valkey topology?
- Does this require operator-commons changes (shared multi-cluster
  client), affecting the other operators in the family?

## Refs

- ROADMAP "Next (2.x line) → Features → Multi-cluster federation"
- ROADMAP "Non-Goals → Multi-tenancy isolation" (tension to reconcile)
- `cmd/main.go:231` (`ctrl.GetConfigOrDie()` — single in-cluster API)
- `internal/valkey/` (data clients, in-cluster only)
- ADR-0059 (weighted read-replica routing — delegated-routing sibling)
- `workflow.md §7.5` (federation requires design ADR before code)
- `enforcement.md §self-repair` (cross-cluster credentials = explicit
  approval, not self-repair)
