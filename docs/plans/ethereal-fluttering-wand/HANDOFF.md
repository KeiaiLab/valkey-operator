# HANDOFF — valkey-operator 상용제품수준 도달 작업

**최종 갱신**: 2026-05-06 (cycle 3 완료)
**Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md`
**현재 진행**: Track A 약 **95%** + Track C (release pipeline) 50% +
Track D (문서) 일부.

---

## 1. 현재 상태

**마지막 commit**: `dc1dac6` (release pipeline).

**누적 26 commits** (cycle 1+2+3):

### Cycle 1 (10 commits) — Track A 기반 + cycle 인계
`7458228` ~ `4f4d34b`. Plan + ADR-0015/0016 + ValkeyBackupTarget/Restore CRD
+ paused annotation + Restore Init container + ValkeyRestore Standalone PVC
+ README + 첫 HANDOFF.

### Cycle 2 (9 commits) — 외부 저장 통합
`505c6c1` ~ `4f4d34b 다음 → 85d865f` 이전. minio-go + ADR-0022 + 실제
BucketExists ping + Destination types + sub-command + Upload/Download Job
+ Source.TargetRef → cross-cluster restore 가능.

### Cycle 3 (7 commits — 본 세션)
| # | SHA | Subject | 의미 |
|---|---|---|---|
| 20 | `a954a80` | `feat(restore): 데이터 plane 검증 — INFO keyspace → RestoredKeys` | non-blocking dial helpers + parseKeyspaceKeys 8 단위 테스트 |
| 21 | `277483e` | `feat(backup): TTL 자동 삭제 + finalizer + RetainPVC cleanup` | Spec.TTL self-delete + Job/PVC cleanup, 5 lifecycle 테스트 |
| 22 | `b49d278` | `feat(restore): Replication mode 활성화 — ROX source PVC 검증` | replicas>1 거절 완화 + ROX 강제, 4 신규 테스트 |
| 23 | `863ff1d` | `docs(samples): 5 CRD 샘플 채우기` | TODO 제거 + ValkeyBackupTarget/Restore 신규 |
| 24 | `ecda4c0` | `docs: CONTRIBUTING + SECURITY + 운영 Runbook` | Day-N₁ 문서 게이트 충족 |
| 25 | `1ff67a2` | `docs: README — Contributing TODO 제거, 신규 문서 cross-link` | L171 placeholder 해소 |
| 26 | `dc1dac6` | `feat(release): scripts/release.sh + cliff.toml` | Track C 첫 step — 수동 release 파이프라인 |

**Day-N₀ (staging) 게이트**: ✅ 충족. Backup 외부 저장 + Restore + 샘플 +
Resource limits (samples) 모두 동작.

**Day-N₁ (제한적 production) 게이트 — 부분 진행**:
- ✅ Restore 가능 (Standalone + Replication PVC/TargetRef)
- ✅ 외부 저장 (Backup S3 업로드 + Restore S3 다운로드)
- ✅ TTL 자동 삭제
- ✅ 데이터 plane 검증 (RestoredKeys)
- ✅ CONTRIBUTING/SECURITY/Runbook 문서
- ✅ 샘플 CR 5건
- ✅ 수동 release 스크립트 (Track C 첫 step)
- ❌ ValkeyCluster mode restore (shard 별 source)
- ❌ Auto failover (Track B)
- ❌ Scale apply / Resharding (Track B)
- ❌ Helm chart (Track D)
- ❌ Prometheus alert rules (Track E)
- ❌ NetworkPolicy enforcement 검증 (Calico/Cilium)
- ❌ e2e 시나리오 자동화 (kind + MinIO)
- ❌ 공통 helper 추출 (refactor)

**테스트 상태** (단위, fake client):
- `internal/storage/`: 10
- `internal/cli/`: 14
- `internal/resources/`: M3.5 + Restore + Upload + Download + 8 Backup Source PVC RWO/ROX
- `internal/controller/`: ValkeyBackupTarget 14 + ValkeyRestore 21 (PVC 11 +
  TargetRef 6 + Replication 4) + paused 6 + Backup phase + 5 lifecycle
- `internal/valkey/`: 8 parseKeyspaceKeys
- 회귀 0건. lefthook 4-stage hook 매 commit 통과.

**미커밋 변경**: `.claude/ralph-loop.local.md` 만 (iteration counter, 무관).

---

## 2. 다음 단계 (우선순위 순)

### 2.1 Track A 완성 잔여 (예상 1-2주)

| step | 분량 | 산출물 |
|---|---|---|
| **ValkeyCluster mode restore** | 2d | shard 별 source 매핑 + Init container per shard. ShardLayout map 활용. |
| **공통 dial helpers refactor** | 0.5d | `internal/controller/dial_helpers.go` (receiver-less) — ValkeyBackup/ValkeyRestore 양쪽 활용. |
| **e2e 시나리오 자동화** | 1.5d | `test/e2e/restore_test.go` (Standalone PVC) + `test/e2e/backup_s3_test.go` (MinIO 컨테이너) |
| **NetworkPolicy CNI enforcement 검증** | 0.5d | Calico/Cilium kind cluster 매니페스트 + cross-pod 차단 검증 |

### 2.2 Track B-F 진입 (Plan §3, 3-5주)

- **B (Failover/Scale apply/Resharding)** — `internal/controller/{valkey,valkeycluster}_failover.go`
- **C 잔여 (릴리스 파이프라인)** — `dist/install.yaml` publish 절차 검증, 첫 v0.1.0 release 수행, ADR-0019 (GHA 예외 §7-③ 사용 시)
- **D (Helm chart)** — `kubebuilder edit --plugins=helm/v2-alpha` + `dist/chart/` publish
- **E (Production hardening)** — `config/prometheus/alert-rules.yaml` (cluster_state_ok / master_link_status / phase=Failed)
- **F (Soak + DR + OTEL + Conversion webhook)** — 24h 시나리오 + tracing

### 2.3 즉시 (kind + MinIO 실측)

```bash
# MinIO 컨테이너 + Backup S3 + Restore S3 시나리오 (e2e)
docker run -d --name minio-test -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minio -e MINIO_ROOT_PASSWORD=minio123 \
  minio/minio:latest server /data --console-address ":9001"
mc alias set local http://localhost:9000 minio minio123
mc mb local/valkey-backups

# kind cluster 에서 ValkeyBackupTarget + ValkeyBackup type=TargetRef → S3
# 그 다음 ValkeyRestore source.targetRef → 데이터 plane 검증
```

→ 결과를 README "운영 시나리오 검증 (실측)" 표 + RFC-0004 §3 marker.

---

## 3. 차단점 / 진입 시 결정

**현재 차단점 없음**. 모든 디자인 분기는 ADR-0015~0023 으로 결정.

**다음 진입 시 결정**:
1. **ValkeyCluster mode shard 별 source**: (a) 단일 PVC 의 `shard-0/dump.rdb`,
   `shard-1/...` 디렉토리 매핑 vs (b) 별개 N PVC. (a) 가 단순 + RWO 회피 (한
   PVC ROX). (b) 는 shard 별 size 조절 가능.
2. **e2e 환경**: kind cluster + MinIO 컨테이너 (단순) vs minikube + 실제 EBS
   (실 cluster 호환). kind 권장.
3. **GHA 예외 §7-③** (release tag → GitHub Release 자동 생성): 사용자 승인
   필요 — 본 프로젝트가 글로벌 RFC 0002 적용 대상인지 결정.
4. **Helm chart vs OLM bundle**: `kubebuilder edit --plugins=helm/v2-alpha`
   생성물 첫 채택 권장.

---

## 4. 근거 링크

- **Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md`
- **ADR**: `docs/kb/adr/INDEX.md` (0015~0023, 9건)
- **이전 HANDOFF (cycle 1, cycle 2)**: 본 파일의 git history 보존
- **CLAUDE.md §6**: 자가수정 정책 — cycle 1+2+3 모두 본 정책 범위 내 진행
- **RFC-0004 §3**: 라이브 사실 게이트 — kind+MinIO 실측 미진행 표시

---

## 5. 의사결정 기록 (cycle 3)

본 세션 자가수정 결정:

1. **데이터 plane 검증 = non-blocking** — dial 또는 INFO 실패 시 warn log +
   skip. Restore 자체 성공 보장. 이 패턴이 *fake client 테스트 환경* 에서도
   회귀 안 일으킴 (기존 11 Restore 테스트 변경 없이 통과).

2. **TTL self-delete** — finalizer 가 cleanup. handleBackupTerminal 가
   RequeueAfter 로 deadline 까지 wake. TTL 파싱 실패 시 *보존* (operator
   가 자동 삭제 시도 안 함 — 안전 default).

3. **Replication mode ROX 강제** — RWO source 는 multi-pod 동시 mount 불가
   (Pod lifecycle 동안 volume 고정). storage class 가 ROX 지원 여부는 *사용자
   책임*.

4. **CONTRIBUTING + SECURITY + Runbook** = OSS 기본 — 사용자 진입 장벽 해소.

5. **release.sh 수동 스크립트** — RFC 0002 (GHA 영구 금지) 준수. release
   tag → GitHub Release 자동 publish 는 *예외 §7-③* (1-step short workflow)
   영역이지만, 본 프로젝트의 글로벌 RFC 적용 여부 사용자 결정 필요.

6. **공통 helper refactor 미루기** — *동작 우선*. ValkeyRestoreReconciler 가
   ValkeyBackupReconciler 의 dial helpers 를 복제 (~120줄). 별개 commit 에서
   `internal/controller/dial_helpers.go` (receiver-less) 추출 예정.

---

## 6. 다음 세션 즉시 시작 명령

```bash
cd /Users/phil/WorkSpace/public/valkey-operator
git log --oneline -10               # 26 commits 확인
cat docs/plans/ethereal-fluttering-wand/HANDOFF.md | head -80

# 회귀
go test -count=1 -timeout=120s ./...

# 다음 step 후보 (우선순위):
# A. kind + MinIO 실측 (RFC-0004 §3)
# B. ValkeyCluster mode restore (Track A 완성)
# C. 공통 dial helpers refactor
# D. Helm chart (Track D)
```
