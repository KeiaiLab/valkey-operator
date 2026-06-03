# ADR-0057: Cross-Region Backup Replication — Key Management, CRR & Lifecycle (Design Gate)

- Date: 2026-06-03
- Status: Proposed
- Authors: @phil

## Context

ROADMAP "Next (2.x line) → Features → Cross-region backup replication":
S3 SSE-KMS key management · Automatic lifecycle policies. This ADR is the
design gate for the **durability / disaster-recovery** layer on top of
the existing backup path.

What already exists (so the ADR does not re-decide it):

- `ValkeyBackupTarget` abstracts S3-compatible external storage
  (ADR-0016), with `ServerSideEncryption string` already present on the
  type (`api/v1alpha2/valkeybackuptarget_types.go:64`).
- Backup upload/download runs via the operator binary subcommand
  (ADR-0023) using minio-go (ADR-0022).
- **Scope boundary:** basic SSE *field wiring* (plumbing the existing
  `ServerSideEncryption` value through to the S3 PutObject call) is being
  handled in a **separate feature PR** (`feat/s3-sse-kms-wiring`). This
  ADR therefore covers **only what sits above that**: KMS *key
  management*, cross-region *replication* (CRR), and *lifecycle*
  policies.

The gap: a single-bucket, single-region backup is not disaster-recovery.
Surviving a region loss requires the backup to exist in a second region,
which means either bucket-level replication or dual-target writes — and
that decision drags in KMS key topology and lifecycle/expiry, both of
which are **bucket/account-level concerns** that expand the operator's
IAM footprint well beyond "write one object".

## Decision

**Proposed — gate the cross-region durability design. The decision is to
require this ADR before implementation; the replication mechanism, key
topology, and IAM surface are Open Questions. No IAM policy, no CRR
config, no lifecycle rule is defined here.**

Primary axes to commit to:

1. **Replication mechanism**
   - *Operator-managed bucket replication (CRR):* the operator configures
     S3 Cross-Region Replication on the target bucket. Powerful but
     requires bucket-level + replication-role IAM the operator does not
     have today.
   - *Dual-target write:* the operator writes each backup to two
     `ValkeyBackupTarget`s in different regions. Stays at object-level
     IAM (no new bucket admin rights) but doubles upload cost/bandwidth
     and needs idempotency + partial-failure handling.
2. **SSE-KMS key management** — same key in both regions (simpler,
   single key policy) vs per-region CMK (regional isolation, but
   cross-region replication must re-encrypt). Who owns key creation and
   rotation — the operator, or ESO/OpenBao per the ROADMAP Non-Goal on
   secret rotation?
3. **Lifecycle / expiry** — lifecycle and replication are
   **bucket-level**, not per-object. An operator that manages them must
   hold bucket-administration IAM, materially widening its blast radius
   versus the current object-write-only posture.

## Consequences

Positive:

- Real DR: a backup survives the loss of one region.
- Lifecycle automation prevents unbounded backup-storage growth/cost.

Negative / trade-offs:

- **IAM surface expansion.** CRR and lifecycle require bucket-level
  (and a replication IAM role) permissions — a large step up from today's
  object PUT. This is the central cost of the feature.
- **KMS complexity.** Cross-region + KMS means key policy spanning
  regions or re-encryption on replicate; a misconfigured key policy
  silently breaks either backup or restore.
- **Cost/bandwidth** (dual-target) or **provider lock-in** (CRR is
  S3-semantics; MinIO/other S3-compatible backends may not implement it
  identically — and `ValkeyBackupTarget` is intentionally
  S3-*compatible*, not AWS-only).
- **Restore-side parity.** Whatever encrypts/replicates must be
  transparently decryptable by the restore path (ADR-0015 init-container
  load).

## Alternatives Considered

1. **Dual-target write (object-level only).** No bucket-admin IAM; the
   operator just writes twice. Strongest fit with the current
   minimal-IAM posture; weaker than native CRR for large datasets.
   Likely the conservative default.
2. **Operator-managed CRR.** Native, efficient, but pulls bucket-level
   IAM into the operator and assumes true S3 CRR support on the backend.
3. **Out-of-operator replication (document only).** Tell users to
   configure CRR/lifecycle on their bucket themselves; operator stays
   single-region. Zero new IAM; pushes DR responsibility to the user.
4. **Operator-owned KMS key creation/rotation.** Rejected as a leading
   option — collides with the ROADMAP Non-Goal delegating secret rotation
   to ESO + OpenBao. Key *reference* yes; key *lifecycle* likely no.

## Open Questions (resolve before implementation)

- CRR vs dual-target write — which becomes the supported path? Both?
- KMS: shared key across regions or per-region CMK? Re-encryption on
  replicate?
- Does the operator create/rotate KMS keys, or only reference keys
  provisioned by ESO/OpenBao (Non-Goal alignment)?
- Lifecycle/replication require bucket-admin IAM — is that acceptable, or
  must it be delegated to the user/platform?
- How does S3-compatible (MinIO, Ceph RGW) behavior differ from AWS S3
  for CRR + SSE-KMS? What is the portability contract?
- Restore-path parity: is every replicated+encrypted artifact provably
  restorable through the ADR-0015 path?

## Refs

- ROADMAP "Next (2.x line) → Features → Cross-region backup replication"
- ROADMAP "Non-Goals → In-house secret rotation logic (ESO + OpenBao)"
- `feat/s3-sse-kms-wiring` (basic SSE field wiring — separate PR, *below*
  this ADR)
- `api/v1alpha2/valkeybackuptarget_types.go:64` (`ServerSideEncryption`)
- ADR-0016 (ValkeyBackupTarget CRD — S3-compatible external storage)
- ADR-0022 (minio-go S3 client)
- ADR-0023 (operator binary upload/download subcommand)
- ADR-0015 (ValkeyRestore init-container load — restore-side parity)
