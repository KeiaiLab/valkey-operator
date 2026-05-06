# HANDOFF — valkey-operator 상용제품수준 도달 작업

**최종 갱신**: 2026-05-06 (cycle 5 완료)
**Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md`
**현재 진행**: Track A **100%** + Track C 50% + Track D 사용자 외부 작업 진행
+ Track E 50%.

---

## 1. 현재 상태

**마지막 commit (본 세션)**: `08b28a2` (README + INDEX 갱신).

**누적 36 commits** (cycle 1+2+3+4+5):

### Cycle 5 (5 commits — 본 세션)
| # | SHA | Subject | 의미 |
|---|---|---|---|
| 32 | `73d5ac7` | `feat(resources): ValkeyCluster mode init container 빌더 — ordinal → shard 매핑` | 단일 STS 의 모든 pod 가 동일 init container, shell 에서 ordinal 추출 → shard index. 5 신규 테스트. |
| 33 | `8767800` | `feat(restore): ValkeyCluster mode 활성화 — Track A 100% 완성` | handlePending Kind=ValkeyCluster 허용 + handleRestoring 분기 + parseShardLayout. 4 신규 테스트. |
| 34 | `0f2f77b` | `docs: ADR-0021 — Helm Chart kubebuilder helm/v2-alpha plugin 채택` | 외부 source 에서 즉시 Superseded by ADR-0024 처리됨 (사용자 외부 작업). |
| 35 | `08b28a2` | `docs: README — ValkeyCluster restore 사용법 + INDEX status 정합성` | Track A 100% 완성 사용자 가시 반영. |

**Track A 완성** — Standalone + Replication + ValkeyCluster 모든 모드 +
PVC/외부 source + 데이터 plane 검증 + TTL/finalizer + cross-cluster restore.
SEV-1 차단점 #1 + #2 *완전 해소*.

**Day-N₀ (staging) 게이트**: ✅ 완전 충족.

**Day-N₁ (제한적 production) 게이트 — ~85% 진행**:
- ✅ Track A 100% (cluster mode 추가)
- ✅ Track C 50% (수동 release pipeline)
- 🔶 Track D: **사용자 외부 작업 진행 중** — ADR-0024 (수기 chart +
  ArtifactHub publish, 3-repo 통일) + `charts/valkey-operator/` +
  `Makefile release pipeline` + lefthook helm hooks
- ✅ Track E 50% (alerts + NP 수동 검증)
- ❌ e2e 시나리오 자동화 (kind + MinIO)
- ❌ Track B (Failover / Scale apply / Resharding)
- ❌ Track F (Soak / DR / OTEL / Conversion webhook)

**테스트 누적** (cycle 1+2+3+4+5):
- `internal/storage/`: 10
- `internal/cli/`: 14
- `internal/resources/`: M3.5 + Restore (Standalone + Cluster) + Upload +
  Download + RWO/ROX (~30)
- `internal/controller/`: ValkeyBackupTarget 14 + ValkeyRestore 25
  (PVC 11 + TargetRef 6 + Replication 4 + ValkeyCluster 3 + parseShardLayout)
  + paused 6 + Backup phase + 5 lifecycle (~50)
- `internal/valkey/`: 8 parseKeyspaceKeys
- 합계: **~110 단위 테스트**, 회귀 0건. lefthook 4-stage hook 매 commit 통과.

**미커밋 변경**: `.claude/ralph-loop.local.md` + `Makefile` + `.lefthook.yml`
+ `charts/` (사용자 외부 작업 — 본 세션에서 손대지 않음).

---

## 2. 다음 단계 (우선순위 순)

### 2.1 사용자 외부 작업 통합 (Track D)
- 사용자가 진행 중 — `charts/valkey-operator/` (수기 helm chart) +
  `Makefile release pipeline` + ADR-0024.
- 본 세션에서 *건드리지 않음*. 사용자가 commit 후 다음 cycle 에서 통합.

### 2.2 즉시 가능 (Track A 완성 검증)
```bash
# kind cluster + MinIO 실측 — Track A 100% 의 라이브 사실 게이트.
docker run -d --name minio-test -p 9000:9000 minio/minio:latest server /data
mc mb local/valkey-backups

# kind cluster:
make setup-test-e2e && make docker-build IMG=valkey-operator:dev
kind load docker-image valkey-operator:dev --name valkey-operator-test-e2e
make install && make deploy IMG=valkey-operator:dev

# ValkeyCluster + Backup S3 + Restore S3 시나리오
# → README "운영 시나리오 검증 (실측)" 표 + RFC-0004 §3 marker.
```

### 2.3 Track B-F 진입 (Plan §3, 3-5주)

| 트랙 | step | 분량 | 사전조건 |
|---|---|---|---|
| **B** | Auto failover | 1.5d | ADR-0017 (replica election 알고리즘) |
| **B** | Scale apply | 1d | ADR-0006 (Deliberate=false default) 활용 |
| **B** | Cluster resharding | 2d | ADR-0018 (slot batch + ASKING 처리) |
| **C 잔여** | 첫 v0.1.0 release | 0.5d | Track D 완료 후 |
| **E 잔여** | promtool check rules | 0.2d | helm install kube-prometheus-stack |
| **F** | OTEL tracing | 1d | tracer provider 통합 |
| **F** | Conversion webhook (v1alpha1 → v1beta1 준비) | 1d | API 안정화 시점 |
| **F** | Soak test 24h | 2d+ | kind cluster 장기 가동 |

### 2.4 e2e 시나리오 자동화 (~1.5d)
`test/e2e/` 에 Backup S3 + Restore (PVC + TargetRef) + ValkeyCluster restore
시나리오. MinIO 컨테이너 in-cluster 또는 외부.

---

## 3. 차단점 / 진입 시 결정

**현재 차단점 없음**.

**다음 진입 시 결정**:
1. **Track B Failover 알고리즘** (ADR-0017): replica replication offset 기준
   (most recent) vs healthy 기준 (Sentinel 스타일). 전자가 데이터 손실
   최소화.
2. **e2e MinIO 환경**: in-cluster (kind 안에 minio Pod) vs 외부 (docker run).
   in-cluster 가 portable.

---

## 4. 근거 링크

- **Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md`
- **ADR**: `docs/kb/adr/INDEX.md` (0015~0024, 10건. 0021 superseded by 0024.)
- **Cycle 1/2/3/4 HANDOFF**: 본 파일 git history 보존
- **사용자 외부 작업**: `charts/valkey-operator/` + `Makefile release` +
  `.lefthook.yml helm hooks` (cycle 5 종료 시점 미커밋, 사용자 책임)

---

## 5. 의사결정 기록 (cycle 5)

본 세션 자가수정 결정:

1. **ValkeyCluster ordinal → shard index 매핑** — 단일 STS 의 모든 pod 가
   동일 init container 사용. shell 의 `${HOSTNAME##*-}` ordinal 추출 +
   arithmetic. ValkeyClusterReconciler 의 ordinal 매핑 (primary
   0..shards-1, replica shards..total-1) 활용.

2. **단일 source PVC + ShardLayout map** — Source.PVC.ShardLayout 미명시
   시 default `shard-{N}/dump.rdb`. Shard 별 별개 PVC 거절 (storage
   class 의존성 + size 조절 미래로 미룸).

3. **parseShardLayout key 형식 다중 허용** — "0", "shard-0", "shard0" 모두
   인식. 사용자 친화 + robust.

4. **dial helpers refactor 후 thin wrapper** (cycle 4) → ValkeyCluster
   mode 진입 시 *추가 helper 작성 없이* 동일 dial 패턴 활용.

5. **사용자 외부 작업 (Track D charts/) 충돌 회피** — ADR-0021 commit
   직후 외부에서 Superseded 처리됨. 본 세션은 *internal source* (controller +
   resources + tests) 만 작업. charts/ 디렉토리는 손대지 않음.

---

## 6. 다음 세션 즉시 시작 명령

```bash
cd /Users/phil/WorkSpace/public/valkey-operator
git log --oneline -10               # 36 commits 확인
cat docs/plans/ethereal-fluttering-wand/HANDOFF.md | head -80

# 사용자 외부 작업 통합 확인
git status --short                  # charts/ + Makefile + .lefthook.yml + ADR-0024 진행도

# 회귀 검증
go test -count=1 -timeout=120s ./...

# 다음 step 후보:
# A. 사용자 charts/ 작업 commit 통합 (Track D)
# B. kind + MinIO 실측 (RFC-0004 §3 라이브 사실 게이트)
# C. e2e 시나리오 자동화 (test/e2e/)
# D. Track B Failover (ADR-0017 작성 후)
```
