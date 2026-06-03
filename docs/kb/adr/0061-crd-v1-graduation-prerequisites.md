# ADR-0061: CRD v1 Graduation (v1alpha2 → v1) — Prerequisite Gate

- Date: 2026-06-03
- Status: Proposed
- Authors: @phil

## Context

ROADMAP "Next (2.x line) → Architecture → CRD v1 graduation": Schema
stabilization · v1alpha2 → v1 conversion webhook · *Verify: six months
with zero BREAKING CHANGEs and 3-repo compatibility*.

This ADR extends **ADR-0026** (conversion webhook deferred until
v1alpha1 stable), which was partially superseded by **ADR-0034**
(v1alpha2 introduced). The graduation to `v1` cannot proceed yet because
of a concrete, verified **prerequisite debt: the v1alpha2 conversion
webhook serving path is not wired.**

Verified current state:

- **v1alpha1 is still the storage version.** All five CRDs carry
  `// +kubebuilder:storageversion` on their *v1alpha1* types
  (`api/v1alpha1/{valkey,valkeycluster,valkeybackup,valkeyrestore,
  valkeybackuptarget}_types.go`). v1alpha2 exists but is not storage.
- **Conversion code exists but is not served.** `api/v1alpha2/
  conversion.go` and `api/v1alpha1/conversion.go` define Hub/
  ConvertTo/ConvertFrom, but:
  - `cmd/main.go:70-74` only registers the v1alpha2 SchemeBuilder
    (`AddToScheme`) and the comments there explicitly state the
    conversion webhook is **not active** ("conversion 미작동",
    "spec.conversion.strategy=Webhook 은 … 후속").
  - `config/crd/` has **no** `spec.conversion.strategy: Webhook`.
  - The Helm chart has no conversion `clientConfig` (the
    `clientConfig` blocks in `charts/valkey-operator/templates/
    webhook.yaml` are the validating/mutating *admission* webhooks, not
    CRD conversion).

So today v1alpha1↔v1alpha2 conversion is **defined but dead** — a v1
graduation on top of an unserved conversion webhook would skip a required
step.

Additionally, the ROADMAP Verify bar ("six months with zero BREAKING
CHANGEs and 3-repo compatibility") is **time-elapsed-based and cannot be
asserted by code** — it is a calendar/stability gate, not a unit test.

## Decision

**Proposed — block v1 graduation behind two prerequisites, recorded as a
gate. No api/v1/ package, no storage flip, no graduation is performed in
this ADR.**

Prerequisite 1 — **finish the v1alpha2 conversion webhook serving path**
(the deferred tail of ADR-0026 / ADR-0034):

- `config/crd/` `spec.conversion.strategy: Webhook` for all five CRDs.
- cert-manager Certificate + conversion `clientConfig` in the chart
  (reusing the cert-manager wiring of ADR-0010/0014).
- Register/serve conversion in `cmd/main.go` (beyond the current
  scheme-only registration).
- **Flip the storage version** v1alpha1 → v1alpha2 once conversion is
  served and round-trip verified.

Prerequisite 2 — **stabilization period**:

- Satisfy the ROADMAP Verify bar (≥6 months, zero BREAKING CHANGEs,
  3-repo compatibility). This is asserted by **release history + a
  human/CI sign-off**, not by an in-repo test, because it is
  time-elapsed.

Only after both: scaffold `api/v1/`, designate it Hub/storage, and add
v1alpha2→v1 conversion — i.e. graduation reuses the *same* spoke/hub
mechanism this ADR requires be proven at the alpha2 stage first.

## Consequences

Positive:

- Graduation rests on a *served and verified* conversion path, not a
  paper one — avoiding a v1 promotion that silently can't convert stored
  objects.
- Flipping storage to v1alpha2 first de-risks the eventual v1 flip
  (the mechanism is exercised once before it matters most).

Negative / trade-offs:

- v1 is gated for at least the stabilization window — but premature v1 is
  worse (v1 implies a stability contract the API has not earned).
- Finishing the conversion serving path is real work (CRD strategy +
  cert + chart + main.go + round-trip tests) that ADR-0026/0034
  deliberately deferred.

## Alternatives Considered

1. **Graduate straight to v1, skip the alpha2 storage flip.** Rejected:
   leaves the conversion path unproven and forces the largest flip
   (→ v1, storage) to be the *first* real exercise of conversion.
2. **Stay on v1alpha-series indefinitely.** Legitimate while there are no
   external stability commitments, but forgoes the production-ready
   signal v1 provides; revisit at the first external GA need.
3. **Webhookless conversion (annotation/manual re-apply).** Rejected as
   the graduation path: does not meet the seamless-conversion expectation
   for a v1 API; the spoke/hub webhook is the standard.

## Open Questions (resolve before implementation)

- **Is the v1alpha2 schema actually final** for the fields graduating to
  v1, or are more `Spec` changes expected (which would reset the
  stabilization clock)?
- Who asserts the time-elapsed Verify bar (zero BREAKING / 6mo / 3-repo),
  and where is that sign-off recorded since it can't be a code test?
- Does the storage flip to v1alpha2 require a migration/backfill of
  already-stored v1alpha1 objects, or does on-read conversion suffice?
- 3-repo compatibility: are mongodb-operator / postgres-operator /
  operator-commons consumers affected by the version bump, and how is
  that validated?
- Should v1 graduation be coordinated across the operator family for a
  consistent stability story?

## Refs

- ROADMAP "Next (2.x line) → Architecture → CRD v1 graduation"
- **ADR-0026** (conversion webhook deferred until v1alpha1 stable — this
  ADR continues its deferred tail)
- **ADR-0034** (Auth optional + v1alpha2 introduced — partial supersede
  of ADR-0026)
- `cmd/main.go:70-74` (scheme-only registration; conversion explicitly
  not served)
- `api/v1alpha1/*_types.go` (`+kubebuilder:storageversion` still on
  v1alpha1)
- `api/v1alpha1/conversion.go`, `api/v1alpha2/conversion.go` (conversion
  defined but unserved)
- ADR-0010 / ADR-0014 (cert-manager wiring reused by the conversion
  webhook)
