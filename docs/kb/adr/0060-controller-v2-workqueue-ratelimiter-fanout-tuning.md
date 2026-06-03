# ADR-0060: Controller v2 — Workqueue Rate-Limiter / Reconcile Fan-out Tuning (Design Gate)

- Date: 2026-06-03
- Status: Proposed
- Authors: @phil

## Context

ROADMAP "Next (2.x line) → Architecture → Controller v2": workqueue
rate-limiter tuning · reconcile fan-out optimization. This ADR gates the
tuning so it does **not** start as premature optimization.

Current state (the baseline any change is measured against):

- All five controllers use controller-runtime **defaults**. Each ends its
  setup with a plain `.Complete(r)` and sets **no** `WithOptions`, **no**
  `MaxConcurrentReconciles`, and **no** custom rate-limiter:
  - `internal/controller/valkey_controller.go:726`
  - `internal/controller/valkeycluster_controller.go:1123`
  - `internal/controller/valkeybackup_controller.go:779`
  - `internal/controller/valkeyrestore_controller.go:296` (target)
  - `internal/controller/valkeyrestore_controller.go:939`
- Effective concurrency is therefore `MaxConcurrentReconciles=1` per
  controller, with controller-runtime's default exponential
  rate-limiter.

The risk this ADR guards against: **there is no benchmark and no incident
indicating the defaults are a bottleneck.** Picking a
`MaxConcurrentReconciles` value or a custom rate-limiter curve *without
measurement* is choosing arbitrary numbers — `principles.md §6` /
`principles.md §2` explicitly flag "tuning without evidence" as an
anti-signal.

## Decision

**Proposed — do NOT tune yet. The decision is to require a reproducible
load-test / measurement methodology, and a concrete Verify bar, *before*
any rate-limiter or concurrency value is changed.** This ADR sets the
gate, not the values.

Mandatory before implementation:

1. **Define the Verify criterion** — a *reproducible* load test (e.g. N
   CRs created/updated/churned over T, measured reconcile latency and
   queue depth) that demonstrates the defaults are insufficient. No load
   test, no change.
2. **Establish a baseline** with the current defaults under that load
   test, so any tuned value is justified by a measured before/after
   delta.
3. Only then consider, per controller (they are not equivalent —
   `ValkeyCluster` reconcile does far more work than `ValkeyBackupTarget`):
   - `MaxConcurrentReconciles > 1` where reconciles are independent and
     safe to parallelize (object-isolation must be verified — concurrent
     reconciles of the same object must not race).
   - A custom workqueue rate-limiter only if the default backoff is
     demonstrably wrong for the observed churn pattern.

## Consequences

Positive (of gating):

- Prevents shipping arbitrary tuning that could *worsen* behavior (e.g.
  raising concurrency on a controller whose reconciles share state →
  races; an aggressive rate-limiter → API server pressure).
- Any future change arrives with evidence, satisfying DoD's
  "performance verified" requirement.

Negative / trade-offs:

- No throughput improvement until a load test exists — acceptable,
  because there is no evidence one is needed.
- Writing a representative load test for an operator (realistic CR churn,
  cluster-mode topology changes) is itself non-trivial work.

## Alternatives Considered

1. **Tune now to "sensible" values** (e.g. `MaxConcurrentReconciles=2`).
   Rejected: no measurement → arbitrary; risks regressions and violates
   the no-premature-optimization principle.
2. **Per-controller concurrency only, no rate-limiter change.** Possibly
   the smallest safe first step *after* measurement, since the
   controllers differ in workload. Still gated on the load test.
3. **Leave defaults permanently.** Legitimate outcome if the load test
   shows the defaults are adequate at expected scale — then this item
   moves to Non-Goals.

## Open Questions (resolve before implementation)

- **What is the reproducible load test** that defines "the defaults are
  insufficient"? Until this exists, work does not start.
- Which controllers (if any) actually exhibit queue backpressure under
  realistic CR counts? `ValkeyCluster` vs `ValkeyBackup` are very
  different.
- For any controller raised above `MaxConcurrentReconciles=1`, is
  same-object reconcile provably race-free (status updates, finalizers,
  external Valkey state)?
- Does increased reconcile concurrency create Kubernetes API-server or
  Valkey control-plane pressure that nets out worse?
- Should tuning be operator-family-wide (operator-commons) or
  per-operator?

## Refs

- ROADMAP "Next (2.x line) → Architecture → Controller v2"
- `internal/controller/*_controller.go` (all five `.Complete(r)`,
  controller-runtime defaults, no `WithOptions`)
- `principles.md §2` (Simplicity First — no unrequested tuning)
- `principles.md §6` (anti-signal: optimization without evidence)
- `checklist.md` DoD (performance impact must be verified)
