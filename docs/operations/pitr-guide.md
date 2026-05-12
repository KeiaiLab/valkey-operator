# Point-In-Time Recovery (PITR) — operator guide

> 한국어 버전: [pitr-guide.ko.md](pitr-guide.ko.md)

PITR was the single largest gap on the ADR-0040 commercial-parity
checklist. This document covers the **phase 1** guide (API +
webhook) plus the entry path to **phase 2** (reconciler dispatch).

## Current status (as of 2026-05-10)

| Area | Status |
|---|---|
| AOF backup (produce) | ✅ GA (BgRewriteAOF, ADR-0016 + minio-go / GCS / Azure) |
| RDB backup (produce) | ✅ GA |
| `ValkeyRestore.Spec.PointInTime` API | ✅ GA (#54) |
| Webhook validation (Source 3-type + PointInTime+RDB reject) | ✅ GA (#54) |
| **AOF replay-to-timestamp reconciler dispatch** | ❌ phase 2 |
| Manual PITR (operator-external tooling) | ✅ available |

## Phase 1 usage — full AOF replay (`PointInTime` nil)

The most common case: restore everything in the backup. Behaviour
matches pre-#54:

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata:
  name: vk-restore-full
  namespace: valkey
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef:
      name: s3-prod
      path: vk-prod/2026-05-10T00:00:00Z/dump.aof
  restoreType: AOF   # when the backup was also AOF
```

The reconciler downloads the AOF → the init container places it
into the Valkey data directory → the STS restarts → Valkey replays
the full AOF on boot.

## Phase 1 usage — PITR API (PointInTime present, dispatch unimplemented)

The webhook accepts the spec and `status` is preserved. The
reconciler currently behaves identically to "full AOF replay"
(PointInTime ignored) — **fail-safe** behaviour until phase 2:

```yaml
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef:
      name: s3-prod
      path: vk-prod/2026-05-10T00:00:00Z/dump.aof
  restoreType: AOF
  pointInTime: "2026-05-10T14:30:00Z"   # target recovery time
```

**Today**: the webhook validates the invariants (rejects RDB + PointInTime). The reconciler ignores `PointInTime` and replays the whole AOF — for an earlier cut-off, supply a **shorter** AOF. **Phase 2 (separate epic)** adds the dispatch that truncates at this exact timestamp.

## Manual PITR (phase 2 workaround)

Operational procedure until phase 2 lands, using tools outside the
operator:

1. **Download the AOF**:
   ```sh
   aws s3 cp s3://vk-prod-backups/2026-05-10T00:00:00Z/dump.aof ./dump.aof
   ```

2. **Truncate the AOF** to keep entries up to the target time
   (direct edit of the Valkey AOF format):
   ```sh
   # Extracts timestamps from AOF entries (works for TIMESTAMP-aware AOF
   # only — Valkey 8.0+ with `set aof-timestamp-enabled yes`).
   valkey-aof-trim --until "2026-05-10T14:30:00Z" dump.aof > dump-truncated.aof
   ```
   **Note**: `valkey-aof-trim` is an external / user-written tool.
   An official Valkey utility is planned for 9.x.

3. **Upload the truncated AOF**:
   ```sh
   aws s3 cp dump-truncated.aof s3://vk-prod-backups/pitr-2026-05-10T14:30:00Z/dump.aof
   ```

4. **Restore with the truncated AOF**:
   ```yaml
   spec:
     source:
       targetRef:
         name: s3-prod
         path: pitr-2026-05-10T14:30:00Z/dump.aof
     restoreType: AOF
   ```

Phase 2 makes steps 1–3 **automatic** inside the operator.

## Phase 2 entry conditions (separate epic candidate)

To activate the dispatch in this guide:

1. ~~**AOF-timestamp parsing library**~~ → ✅ **#68** `internal/aoftime` package GA.
2. ~~**File-level helper for reconciler integration**~~ → ✅ **#69** `TruncateAOFFile` GA.
3. ~~**Reconciler dispatch — the cli in the download Job truncates in place**~~ → ✅ **#70** (`DownloadJobParams.PITRCutoff` + `cli download --pitr-cutoff`; the reconciler dispatches automatically when `PointInTime` is set and `RestoreType=AOF`).
4. **`valkey-cli --pipe` integration** — today the init container loads the AOF at boot (Valkey's default `appendonly yes`). Separate `valkey-cli --pipe` integration is needed only for **streaming replay** scenarios; the init-container path is currently sufficient.
5. **`PointInTime ≤ backup CompletedAt` webhook invariant** (follow-up) — a `PointInTime` after the backup completed is a semantic contradiction (asking for data that does not exist yet).
6. **Rollback** (follow-up) — fall back to the backup point if the replay fails.

**Current status (after #70)**: with `restoreType: AOF` +
`PointInTime`, the reconciler automatically downloads → truncates →
the init container boots from the truncated AOF. **Fully automatic
PITR** is operational. The remaining work is the webhook invariant
and rollback (operational safety).

## Recovery from a failed PITR (#72 rollback)

A failed PITR replay (corrupted AOF, bad timestamp marker, …)
leaves the init container in CrashLoopBackOff. Manual rollback:

### Precondition

The reconciler invokes the download Job with
`--pitr-backup=/backup/dump.aof.original` (#72). The backup file
must be present on the staging PVC.

### Automatic rollback (one-line for the operator)

```sh
# 1. Bring up a helper pod with access to the staging PVC.
kubectl run rollback-helper --rm -it --restart=Never \
  --image=ghcr.io/keiailab/valkey-operator:latest \
  --overrides='{"spec":{"containers":[{"name":"r","image":"ghcr.io/keiailab/valkey-operator:latest","command":["sh","-c","cp /backup/dump.aof.original /backup/dump.aof"],"volumeMounts":[{"name":"b","mountPath":"/backup"}]}],"volumes":[{"name":"b","persistentVolumeClaim":{"claimName":"<staging-pvc>"}}]}}'

# 2. Restart the Valkey STS (the init container performs a full
#    AOF replay).
kubectl rollout restart sts/<cluster-name>
```

### Automation (operator side, separate epic)

Follow-up — the operator will:

1. Detect `Status.Phase=Restoring` + init-container CrashLoopBackOff.
2. Verify the backup file exists.
3. Transition to `Status.Phase=PITRRollbackPending` (automatic after
   explicit user approval).
4. Run the one-liner above from the reconciler.

This automation is **destructive** (overwrites PVC data) and so
requires an ADR plus explicit user approval.

## #70 usage example (live behaviour)

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata: { name: pitr-restore }
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef: { name: s3-prod, path: backup/dump.aof }
  restoreType: AOF
  pointInTime: "2026-05-10T14:30:00Z"
```

Internal flow:

1. `handlePending`: webhook (#54) validates invariants → Mounting.
2. `handleMounting`: create the download Job with
   `--pitr-cutoff=2026-05-10T14:30:00Z`.
3. `cli download` (#70): S3 → `/backup/dump.aof` → in-place
   truncate to the cut-off.
4. `handleRestoring`: existing init-container path → cluster boots
   off the truncated AOF.
5. Verifying → Completed.

## #68 usage example (Go integration)

```go
import "github.com/keiailab/valkey-operator/internal/aoftime"

aofBytes, _ := os.ReadFile("dump.aof")
if !aoftime.HasTimestamps(aofBytes) {
    // PITR not possible — only a full replay is supported.
    return errors.New("AOF lacks timestamps (set aof-timestamp-enabled yes for PITR)")
}
cutoff := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)
offset := aoftime.TruncateOffset(aofBytes, cutoff)
truncated := aofBytes[:offset]
// Stream `truncated` to `valkey-cli --pipe` → only entries up to
// the cut-off are restored.
```

## Related

- runbook §3.3 — Restore (disaster recovery).
- ADR-0015 — `ValkeyRestore` init-container pattern.
- ADR-0016 — `ValkeyBackupTarget` external storage.
- #54 — `PointInTime` API + webhook.

## References

- Valkey AOF spec: <https://valkey.io/topics/persistence/>
- AOF timestamp-enabled (8.0+): `aof-timestamp-enabled` directive.
- External tooling: `redis-cli --pipe` (Valkey-compatible).
