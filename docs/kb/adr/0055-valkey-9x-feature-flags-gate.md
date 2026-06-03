# ADR-0055: Valkey 9.x Feature Flags — Version-Gated Config Rendering

- Date: 2026-06-03
- Status: Proposed
- Authors: @phil

## Context

ROADMAP "Next (2.x line) → Features → Valkey 9.x feature follow-up
(flags / cluster-mode changes)".

**Important framing — 9.x support already ships.** The operator already
defaults to Valkey `9.0.4`
(`api/v1alpha2/common_types.go:30` `DefaultValkeyVersion = "9.0.4"`,
`supportedValkeyList = ["8.0.9", "8.1.6", "8.1.7", "9.0.4"]`). Running a
9.x image is not the open work. The open work is a **deliberate opt-in
gate for 9.x-only config directives** that must not render on an 8.x
image (where they would be rejected at startup).

Today `valkey.conf` rendering is version-agnostic:

- `internal/resources/configmap.go` `RenderValkeyConf(ConfigData)` parses
  `internal/assets/valkey.conf.tmpl` (34 `{{` conditionals) and emits the
  same directive set regardless of the resolved image version.
- `ConfigData` (`configmap.go:31`) has **no `MajorVersion` field** — the
  template literally cannot branch on the running version.

Cluster-mode behavioral changes in Valkey 9.x (slot migration semantics,
new `CLUSTER` subcommands) similarly have no version-aware path in
`internal/valkey/cluster.go`.

A 9.x directive written unconditionally into the rendered conf would make
an 8.x pod (still in `supportedValkeyList`) fail to boot — a silent
regression for users pinned to 8.x.

## Decision

**Proposed — version-gated rendering via a `MajorVersion` input + opt-in
template guards. No directive is gated yet; the *mechanism* is the
decision, the *directive set* is an Open Question.**

Direction under consideration:

1. Add a parsed `MajorVersion` (and possibly `MinorVersion`) field to
   `ConfigData`, derived from the resolved image tag, so
   `valkey.conf.tmpl` can branch:
   ```
   {{- if ge .MajorVersion 9 }}
   # 9.x-only directive — opt-in
   {{- end }}
   ```
2. Gate **only** directives that are (a) 9.x-only AND (b) opt-in via an
   explicit Spec field — never auto-enable a 9.x feature just because the
   image happens to be 9.x. Default-off preserves upgrade safety.
3. Keep cluster-mode 9.x changes behind the same version gate in
   `internal/valkey/`, falling back to current 8.x behavior below the
   threshold.

This ADR records the **gate as the integration point**. It deliberately
does not enumerate the 9.x directives, because that list requires an
upstream changelog triage that has not been done.

## Consequences

Positive:

- Users pinned to 8.x are unaffected — gated directives never render for
  them.
- Adding a 9.x feature later is a localized template + Spec change, not a
  rendering-pipeline refactor.

Negative / trade-offs:

- `ConfigData` gains version coupling — every render now depends on
  correctly parsing the image tag (parse failure handling is itself a
  design question).
- Image tag is a weak version source (custom registries, digests,
  `:latest`). A misparse silently picks the wrong branch.
- Test surface grows: rendering must be exercised at each supported major
  to guard both directions of the gate.

## Alternatives Considered

1. **No gate — render 9.x directives unconditionally.** Rejected: breaks
   8.x pods that are still supported. This is the failure mode the ADR
   exists to prevent.
2. **Drop 8.x support, render 9.x-only.** Rejected: `supportedValkeyList`
   intentionally keeps 8.0.9/8.1.6/8.1.7 as milestone baselines; dropping
   them is a separate breaking decision, not a feature-flag mechanism.
3. **User-supplied raw conf passthrough for 9.x directives.** Rejected as
   the *primary* path: defeats operator validation and the
   Helm/Kustomize parity invariant (ADR-0028). May remain an escape
   hatch, out of scope here.

## Open Questions (resolve before implementation)

- **Which 9.x directives get gated?** Requires triage of the upstream
  Valkey 9.x changelog. Currently undefined — this is the blocker.
- **Version source of truth:** parse the image tag, add an explicit
  `Spec.Version` major hint, or read `INFO server` from a running pod?
- **Parse-failure policy:** when the tag cannot be parsed, fail closed
  (assume lowest major, gate everything off) or surface a webhook error?
- **Opt-in surface:** per-directive Spec fields vs a single
  `featureGates` map vs a `valkeyVersionFeatures` block?
- **Cluster-mode 9.x changes:** do any require a coordinated rolling
  upgrade path, or are they backward-compatible at the protocol level?

## Refs

- ROADMAP "Next (2.x line) → Features → Valkey 9.x feature follow-up"
- `internal/resources/configmap.go` (`ConfigData`, `RenderValkeyConf`)
- `internal/assets/valkey.conf.tmpl` (template gate site)
- `api/v1alpha2/common_types.go` (`DefaultValkeyVersion`,
  `supportedValkeyList`)
- ADR-0028 (Helm vs Kustomize parity invariant — gated rendering must
  stay parity-safe)
