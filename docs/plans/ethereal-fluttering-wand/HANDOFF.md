# HANDOFF — valkey-operator 상용제품수준 도달 작업

**최종 갱신**: 2026-05-06 (cycle 4 완료)
**Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md`
**현재 진행**: Track A 95% + Track C 50% + Track D 30% + Track E 50%.

---

## 1. 현재 상태

**마지막 commit**: `551d065` (NetworkPolicy CNI 검증 매니페스트).

**누적 30 commits** (cycle 1+2+3+4):

### Cycle 1 (10 commits) — Track A 기반
`7458228` ~ `4f4d34b`. M3.5 + ADR-0015/0016 + ValkeyBackupTarget/Restore CRD
+ paused annotation + Restore Init container + Standalone PVC + 첫 HANDOFF.

### Cycle 2 (9 commits) — 외부 저장 통합
`505c6c1` ~ `84c78c4`. minio-go + ADR-0022 + BucketExists ping + Destination
types + sub-command + Upload/Download Job + Source.TargetRef → cross-cluster.

### Cycle 3 (8 commits) — Day-N₀ 충족 + 문서
`a954a80` ~ `979fca9`. INFO keyspace → RestoredKeys + TTL/finalizer + Replication
mode (ROX) + 5 샘플 + CONTRIBUTING/SECURITY/Runbook + release pipeline.

### Cycle 4 (3 commits — 본 세션)
| # | SHA | Subject | 의미 |
|---|---|---|---|
| 28 | `5a57e2c` | `refactor(controller): dial helpers 공통 추출 — code 중복 ~110줄 해소` | 6 method → 3 receiver-less + 4 thin wrapper. 회귀 0건. |
| 29 | `807fddb` | `feat(prometheus): alert rules — 6 alerts (Day-N₁ Track E)` | Cluster state / slots / replicas / phase / reconcile errors / operator down. runbook cross-link. |
| 30 | `551d065` | `test(network-policy): CNI enforcement 검증 매니페스트` | Calico/Cilium kind 시나리오 + probe pods. README L105 한계 해소. |

**Day-N₀ (staging) 게이트**: ✅ 완전 충족.

**Day-N₁ (제한적 production) 게이트 — ~75% 진행**:
- ✅ Restore (Standalone + Replication, PVC + 외부 source + 데이터 plane)
- ✅ 외부 저장 (Backup S3 업로드 + Restore S3 다운로드)
- ✅ TTL 자동 삭제 + finalizer cleanup
- ✅ 5 CRD 샘플 + CONTRIBUTING + SECURITY + Runbook
- ✅ 수동 release 파이프라인 (scripts/release.sh + cliff.toml)
- ✅ Prometheus alert rules (6 alerts)
- ✅ NetworkPolicy CNI 검증 매니페스트 (수동)
- ✅ dial helpers refactor (code quality)
- ❌ ValkeyCluster mode restore (shard 별 source)
- ❌ Auto failover (Track B)
- ❌ Scale apply / Resharding (Track B)
- ❌ Helm chart (Track D)
- ❌ e2e 시나리오 자동화 (kind + MinIO)

**테스트 상태** (단위, fake client + mock):
- `internal/storage/`: 10 (S3Client + parseEndpoint)
- `internal/cli/`: 14 (Dispatch + upload/download)
- `internal/resources/`: M3.5 + Restore + Upload + Download + 8 RWO/ROX
- `internal/controller/`: ValkeyBackupTarget 14 + ValkeyRestore 21 + paused 6
  + Backup phase + 5 lifecycle = ~50
- `internal/valkey/`: 8 parseKeyspaceKeys
- 회귀 0건. lefthook 4-stage hook 매 commit 통과.

**미커밋 변경**: `.claude/ralph-loop.local.md` 만.

---

## 2. 다음 단계 (우선순위 순)

### 2.1 Track A 완성 잔여 (예상 1-2주)

| step | 분량 | 산출물 |
|---|---|---|
| **ValkeyCluster mode restore** | 2-3d | ordinal → shard index 매핑 + 단일 STS의 per-pod init container + ShardLayout 활용. shell 안에서 `${HOSTNAME}` ordinal 추출. |
| **e2e 시나리오 자동화** | 1.5d | `test/e2e/restore_test.go` + `test/e2e/backup_s3_test.go` (MinIO 컨테이너) |
| **NetworkPolicy enforcement 자동 검증** | 0.5d | `test/network-policy/np_test.go` — 수동 매니페스트 자동화 |

### 2.2 Track B-F 진입 (Plan §3, 3-5주)

- **B (Failover/Scale apply/Resharding)** — `internal/controller/{valkey,valkeycluster}_failover.go`. ADR-0017 (Failover 알고리즘) + ADR-0018 (Resharding 정책) 작성.
- **C 잔여 (릴리스)** — 첫 v0.1.0 release 수행, install.yaml publish 절차 검증.
- **D (Helm chart)** — `kubebuilder edit --plugins=helm/v2-alpha` + `dist/chart/` publish. ADR-0021.
- **E 잔여 (Production hardening)** — promtool check rules, alert manager 통합 검증.
- **F (Soak + DR + OTEL + Conversion webhook)** — 24h 시나리오 + tracing.

### 2.3 즉시 (kind + MinIO 실측)

```bash
docker run -d --name minio-test -p 9000:9000 \
  -e MINIO_ROOT_USER=minio -e MINIO_ROOT_PASSWORD=minio123 \
  minio/minio:latest server /data
mc alias set local http://localhost:9000 minio minio123
mc mb local/valkey-backups

# kind cluster 에서 ValkeyBackupTarget + Backup type=TargetRef → S3
# 그 다음 ValkeyRestore source.targetRef → 데이터 plane 검증
```

→ 결과를 README "운영 시나리오 검증 (실측)" 표 + RFC-0004 §3 marker.

---

## 3. 차단점 / 진입 시 결정

**현재 차단점 없음**. 디자인 분기 ADR-0015~0023 으로 결정.

**다음 진입 시 결정**:
1. **ValkeyCluster shard mapping**: (a) 단일 source PVC + 디렉토리 구조
   `shard-N/dump.rdb` vs (b) Spec.Source.PVC.ShardLayout map. (a) 가 단순
   + (b) 의 super set (default 매핑).
2. **Helm chart 전략**: kubebuilder helm/v2-alpha (auto-generated) vs 수기.
   ADR-0021 작성 후 결정.
3. **e2e 환경**: kind + MinIO (단순) — 권장. minikube + 실 EBS 는 후속.

---

## 4. 근거 링크

- **Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md`
- **ADR**: `docs/kb/adr/INDEX.md` (0015~0023, 9건)
- **Cycle 1/2/3 HANDOFF**: 본 파일 git history 보존
- **CLAUDE.md §6**: 자가수정 정책 — cycle 1+2+3+4 모두 본 정책 범위
- **RFC-0004 §3**: 라이브 사실 게이트 — kind+MinIO 실측 미진행 표시

---

## 5. 의사결정 기록 (cycle 4)

본 세션 자가수정 결정:

1. **dial helpers refactor = thin wrapper 패턴** — 양 controller 의 method
   시그니처 그대로 두고 본문만 helper 호출로 교체. caller 코드 변경 0건 →
   회귀 위험 0. ~110줄 net 감소.

2. **Prometheus alert rules** — kube-prometheus-stack 호환 label
   (prometheus: kube-prometheus + role: alert-rules). 6 alerts: 5 CR-level
   + 1 operator-level (ValkeyOperatorDown). 각 annotation 의 runbook_url 가
   `docs/operations/runbook.md` 의 해당 섹션 cross-link.

3. **NetworkPolicy 검증 매니페스트 = 수동 우선** — 자동 e2e 는 큰 비용
   (kind + Calico/Cilium 셋업). 수동 매니페스트 + README 절차 만 첫 cut.

---

## 6. 다음 세션 즉시 시작 명령

```bash
cd /Users/phil/WorkSpace/public/valkey-operator
git log --oneline -10               # 30 commits 확인
cat docs/plans/ethereal-fluttering-wand/HANDOFF.md | head -80

go test -count=1 -timeout=120s ./...

# 다음 step 후보:
# A. ValkeyCluster mode restore (Track A 완성)  ← 가장 큰 가치
# B. Helm chart (Track D, kubebuilder edit --plugins=helm/v2-alpha)
# C. e2e 시나리오 자동화 (kind + MinIO)
# D. Track B Failover (ADR-0017 작성 후)
```
