# Point-In-Time Recovery (PITR) — 운영 가이드

ADR-0040 commercial parity 의 *최대 단일 차이* 였던 PITR 의 phase 1 (API +
webhook) 가이드 + phase 2 (reconciler 통합) 진입 절차.

## 현재 상태 (2026-05-10)

| 영역 | 상태 |
|---|---|
| AOF backup (생성) | ✅ GA (BgRewriteAOF, ADR-0016 + minio-go/GCS/Azure) |
| RDB backup (생성) | ✅ GA |
| ValkeyRestore.Spec.PointInTime API | ✅ GA (PR #54) |
| Webhook validation (Source 3-type + PointInTime+RDB reject) | ✅ GA (PR #54) |
| **AOF replay-to-timestamp reconciler dispatch** | ❌ phase 2 |
| 수동 PITR (operator 외부 도구 사용) | ✅ 가능 |

## Phase 1 사용법 — 전체 AOF replay (PointInTime nil)

가장 일반적인 backup 시점 전체 복원. PR #54 이전과 동일 동작:

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
  restoreType: AOF   # backup 도 AOF 로 만든 경우
```

reconciler 가 AOF 를 다운로드 → init container 가 valkey 데이터 디렉토리에
배치 → STS 재시작 → valkey 가 AOF 전체 replay (booting 시 자동).

## Phase 1 사용법 — PITR API (PointInTime 명시, dispatch 미구현)

webhook 통과 + status 보존 만 GA. reconciler 가 *전체 AOF replay 와 동일 동작*
(PointInTime 무시) — phase 2 까지 *fail-safe* 동작:

```yaml
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef:
      name: s3-prod
      path: vk-prod/2026-05-10T00:00:00Z/dump.aof
  restoreType: AOF
  pointInTime: "2026-05-10T14:30:00Z"   # 원하는 복원 시각
```

**현재 동작**: webhook 가 invariants 검증 (RDB 면 reject). reconciler 는
PointInTime 무시하고 전체 replay → AOF 의 더 이전 시점을 원하면 backup AOF 의
*더 짧은* 버전을 사용. **phase 2 (별도 epic)** 가 본 시각까지만 replay 하는 dispatch 추가.

## 수동 PITR (phase 2 대안)

phase 2 까지의 임시 운영 절차 — operator 외부 도구 사용:

1. **AOF 다운로드**:
   ```sh
   aws s3 cp s3://vk-prod-backups/2026-05-10T00:00:00Z/dump.aof ./dump.aof
   ```

2. **AOF truncate** — 본 시각까지만 남기기 (Valkey AOF 형식 직접 truncate):
   ```sh
   # AOF entries 의 timestamp 추출 (TIMESTAMP-aware AOF 만 가능 — Valkey 8.0+
   # `set aof-timestamp-enabled yes`).
   valkey-aof-trim --until "2026-05-10T14:30:00Z" dump.aof > dump-truncated.aof
   ```
   *주의*: `valkey-aof-trim` 은 외부 도구 또는 사용자 작성 스크립트. Valkey 공식
   유틸은 9.x 에서 추가 예정.

3. **truncated AOF 업로드**:
   ```sh
   aws s3 cp dump-truncated.aof s3://vk-prod-backups/pitr-2026-05-10T14:30:00Z/dump.aof
   ```

4. **truncated AOF 로 ValkeyRestore**:
   ```yaml
   spec:
     source:
       targetRef:
         name: s3-prod
         path: pitr-2026-05-10T14:30:00Z/dump.aof
     restoreType: AOF
   ```

phase 2 가 위 1-3 단계를 *operator 가 자동* 처리.

## Phase 2 진입 조건 (별도 epic 후보)

본 가이드의 phase 2 dispatch 활성화 위해:

1. ~~**AOF timestamp parse 라이브러리**~~ → ✅ **PR #68** `internal/aoftime` 패키지 GA
2. ~~**File-level helper for reconciler 통합**~~ → ✅ **PR #69** `TruncateAOFFile` GA
3. ~~**Reconciler dispatch — download Job 의 cli 가 in-place truncate**~~ → ✅ **PR #70**
   (DownloadJobParams.PITRCutoff + cli download `--pitr-cutoff` flag.
   reconciler 가 PointInTime + RestoreType=AOF 시 자동 dispatch)
4. **valkey-cli --pipe 통합** — 현재는 init container 가 cluster 부팅 시 AOF 자동 load
   (Valkey 의 default `appendonly yes` 동작). `valkey-cli --pipe` 별도 통합은
   *streaming replay* 가 필요한 케이스 (현재 init container 방식 충분).
5. **PointInTime ≤ backup CompletedAt invariant** webhook (후속) — backup 시점
   초과 PointInTime 은 의미적 모순 (없는 미래 데이터 요청).
6. **rollback** — replay 중 실패 시 backup 시점으로 fallback (후속).

**현재 상태 (PR #70 후)**: AOF restore + PointInTime 명시 시 reconciler 가 자동
download → truncate → init container 가 truncated AOF 로 cluster 부팅. *완전
자동 PITR 동작*. 남은 작업은 webhook invariant + rollback (운영 안전성).

## PR #70 사용 예 (실제 동작)

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

내부 동작:
1. handlePending: webhook (PR #54) 가 invariants 검증 → Mounting
2. handleMounting: download Job 생성 with `--pitr-cutoff=2026-05-10T14:30:00Z`
3. cli download (PR #70): S3 → /backup/dump.aof → in-place truncate to cutoff
4. handleRestoring: 기존 init container path → cluster 부팅 시 truncated AOF replay
5. Verifying → Completed

## PR #68 사용 예 (Go 코드 통합)

```go
import "github.com/keiailab/valkey-operator/internal/aoftime"

aofBytes, _ := os.ReadFile("dump.aof")
if !aoftime.HasTimestamps(aofBytes) {
    // PITR 불가 — 전체 replay 만 가능
    return errors.New("AOF lacks timestamps (set aof-timestamp-enabled yes for PITR)")
}
cutoff := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)
offset := aoftime.TruncateOffset(aofBytes, cutoff)
truncated := aofBytes[:offset]
// truncated 를 valkey-cli --pipe 에 stream → cutoff 시각까지의 데이터만 복원
```

## 후속 가이드

- runbook §3.3 Restore (재해 복구)
- ADR-0015 (ValkeyRestore init container 패턴)
- ADR-0016 (ValkeyBackupTarget 외부 저장)
- PR #54 (PointInTime API + webhook)

## 참조

- Valkey AOF spec: https://valkey.io/topics/persistence/
- AOF timestamp-enabled (8.0+): `aof-timestamp-enabled` directive
- 외부 도구: `redis-cli --pipe` (Valkey 호환)
