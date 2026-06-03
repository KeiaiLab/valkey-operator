# ADR-0058: Online Schema-less Migration (RDB diff / LWW) — Research Spike Gate

- Date: 2026-06-03
- Status: Proposed
- Authors: @phil

## Context

ROADMAP "Next (2.x line) → Features → Online schema-less migration":
RDB diff tool · LWW conflict resolution. **This is the least-defined item
on the 2.x line** — it is not yet clear it belongs in the operator at
all.

Three hard problems sit under this single ROADMAP bullet:

1. **Valkey has no native Last-Write-Wins (LWW).** LWW for conflicting
   concurrent writes across replicas/clusters is CRDT-grade territory
   (cf. Redis active-active / CRDB), which Valkey OSS does not provide.
   Building it is a research problem, not a feature.
2. **There is no RDB binary parser in the codebase.** "RDB diff" implies
   parsing the RDB on-disk format and computing a delta. The operator
   today only *moves* RDB blobs (backup/restore, ADR-0015/0016/0023); it
   never *interprets* their contents. A correct, version-tracking RDB
   parser is a substantial dependency or a large new component.
3. **Control-plane vs data-plane ambiguity.** It is unspecified whether
   this is an operator control-plane feature (reconcile orchestrates a
   migration) or a standalone data-plane tool a user runs — or whether it
   belongs outside this operator entirely.

Because of (1)–(3), committing to a CRD or controller now would be
premature; the realistic first step is a research spike, not code.

## Decision

**Proposed — gate this behind a research spike. No CRD, no controller, no
RDB parser is committed. The decision is that a spike must precede *any*
design, and that "ship it outside the operator" is a first-class possible
outcome of that spike.**

The spike must answer, with evidence, at minimum:

- Is online schema-less migration **in scope for the operator**, a
  **standalone tool**, or **out of scope** entirely?
- What does "schema-less migration" concretely mean for a key/value store
  with no schema — key-space remapping? cross-version RDB upgrade?
  cross-cluster data sync? The term is currently ambiguous.
- Is LWW actually required, or does an offline/stop-the-world migration
  (no conflict window, hence no LWW) cover the real use cases?
- What is the RDB-parsing strategy — adopt an existing library, depend on
  Valkey tooling, or avoid parsing RDB altogether (e.g. replicate via the
  replication protocol / `DUMP`/`RESTORE` per key)?

## Consequences

Positive (of gating with a spike):

- Avoids sinking implementation effort into a CRD/controller before the
  problem is even bounded.
- Makes "this does not belong in the operator" a legitimate, low-cost
  conclusion.

Negative / trade-offs:

- The feature stays undelivered until the spike resolves scope — but
  given the unknowns, premature delivery would be worse.
- A real LWW path, if required, is a multi-quarter research commitment far
  beyond the operator's current scope and skill surface.

## Alternatives Considered

1. **Offline (stop-the-world) migration instead of online.** Quiesce
   writes, migrate, resume. Eliminates the conflict window → **no LWW
   needed** → removes the hardest sub-problem. Likely the pragmatic
   first deliverable if migration is in scope at all.
2. **Standalone CLI tool outside the operator.** Ship RDB-diff /
   migration as a separate binary or `kubectl` plugin; keep the operator
   focused on lifecycle. Strong candidate — decouples a research-heavy
   feature from the production operator's release cadence.
3. **Delegate to Valkey-native replication / `DUMP`+`RESTORE`.** Use the
   replication protocol or per-key `DUMP`/`RESTORE` rather than parsing
   RDB bytes. Avoids writing an RDB parser; bounded by what the protocol
   exposes.
4. **Drop from the operator roadmap.** Move to Non-Goals if the spike
   shows it is out of scope. Explicitly on the table.

## Open Questions (resolve in/after the spike — blockers to any design)

- Operator control-plane feature, standalone tool, or out of scope?
- What does "schema-less migration" mean concretely here?
- Online (LWW required) vs offline (no conflict window)? Are there real
  use cases that *force* online?
- RDB parsing: adopt a library, use Valkey tooling, or avoid parsing
  entirely via the replication/`DUMP` path?
- If LWW is truly required, is the operator the right home for
  CRDT-grade conflict resolution at all?

## Refs

- ROADMAP "Next (2.x line) → Features → Online schema-less migration"
- ADR-0015 / ADR-0016 / ADR-0023 (current RDB handling — *move* only,
  never *interpret*; no parser exists)
- `workflow.md §7.5` (migration requires design ADR before code — here
  satisfied as a spike gate)
- `principles.md §2` (Simplicity First — prefer offline / out-of-operator
  unless online + in-operator is justified by evidence)
