# HANDOFF — valkey-operator 상용제품수준 도달 작업

**작성**: 2026-05-06
**Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md` (진단 + Day-1/N₀/N₁/N₂ 게이트 + 6 트랙 정의)
**현재 진행**: Track A (Backup 외부 저장 + Restore) 약 50% — Standalone PVC restore 동작.

---

## 1. 현재 상태

**마지막 commit**: `625dc35` (`docs: README — M3.5 + ValkeyRestore Standalone 완료 반영 + Restore 사용법`).

**누적 9 commits** (이번 세션):

| # | SHA | Subject | 영향 |
|---|---|---|---|
| 1 | `7458228` | `feat(backup): ValkeyBackup M3.5 — Job-based RDB 복사 + PVC 보존` | M3.5 완료. `valkey-cli --rdb` Job 이 SYNC 프로토콜 fresh RDB 를 별도 PVC 로 저장. SEV-1 #2 부분 해소. |
| 2 | `2a9b29b` | `docs: ADR-0015 + ADR-0016 — Restore 패턴 + 외부 저장 추상화 결정` | Init Container 패턴 (ADR-0015) + ValkeyBackupTarget CRD (ADR-0016) 결정 기록. |
| 3 | `307371a` | `feat(api): ValkeyBackupTarget CRD — S3 외부 저장 추상화 타입 정의` | CRD types only. controller 별개. |
| 4 | `123944a` | `feat(backup-target): ValkeyBackupTargetReconciler — schema 기반 자격증명 검증` | 실제 S3 ping 미구현 (AWS SDK 별개). 9 단위 테스트. |
| 5 | `a29165b` | `feat(api): ValkeyRestore CRD — backup PVC/외부 target 기반 RDB 복원 타입` | CRD types. 5 phase + Source.PVC/TargetRef. |
| 6 | `37656d0` | `feat(controller): paused annotation — ValkeyRestore 와의 STS 충돌 방지` | `cache.keiailab.io/paused=true` 시 ValkeyController/ClusterController no-op. 6 테스트. |
| 7 | `cef4d74` | `feat(resources): Restore Init container + Source volume + Inject/Remove 헬퍼` | STS PodTemplate patch helpers. 7 테스트. |
| 8 | `fbb96d7` | `feat(restore): ValkeyRestoreReconciler — Standalone PVC source 첫 동작` | Phase 전이 5단계 + finalizer + 11 테스트. **SEV-1 #1 부분 해소**. |
| 9 | `625dc35` | `docs: README — M3.5 + ValkeyRestore Standalone 완료 반영 + Restore 사용법` | 사용자 가시 문서 갱신. |

**미커밋 변경**: `.claude/ralph-loop.local.md` 만 (iteration counter 메타데이터, 무관).

**테스트 상태** (단위, fake client):
- `internal/resources/...`: 7 신규 (restore_test) PASS
- `internal/controller/...`: paused 6 + BackupTarget 9 + Restore 11 = 26 신규 PASS
- 회귀: 기존 모든 테스트 정상 (lefthook pre-commit 의 govet/gofmt/golangci-lint 매 commit 통과)

**라이브 검증 미진행** (RFC-0004 §3): kind cluster 에서의 *실측 통과* 는 미수행. README L79-93 의 13 시나리오 표 갱신 안 됨.

---

## 2. 다음 단계 (우선순위 순)

### 2.1 즉시 가능 (이번 세션 마지막 progress 다음)

```bash
# 현재 commit 그래프 확인
git log --oneline 625dc35~9..625dc35

# 단위 테스트 전수 — 회귀 확인
go test -count=1 -timeout=120s ./...

# kind cluster 에서 ValkeyRestore Standalone 실측
make setup-test-e2e
make docker-build IMG=valkey-operator:dev
kind load docker-image valkey-operator:dev --name valkey-operator-test-e2e
make install
make deploy IMG=valkey-operator:dev
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml      # standalone CR
# (대기 — Phase=Running)
kubectl apply -f config/samples/cache_v1alpha1_valkeybackup.yaml # backup 트리거
kubectl wait --for=jsonpath='{.status.phase}'=Completed valkeybackup/...
# (그 다음 cluster 데이터 추가 후) ValkeyRestore CR 작성 + apply
# Phase=Completed 확인 + kubectl exec valkey-...-0 -- valkey-cli get <key>
```

→ 결과를 README "운영 시나리오 검증 (실측)" 표에 추가 + RFC-0004 §3
   `<!-- live-verified: YYYY-MM-DD -->` 마커 + `kubectl get application` /
   `curl healthz` 인용.

### 2.2 Track A 잔여 작업 (예상 1-2주)

| step | 분량 | 의존성 | 산출물 |
|---|---|---|---|
| **AWS SDK 통합 + S3 reachability ping** | 0.5d | sonatype-guide 스킬 의존성 검증 의무 | `internal/storage/s3_client.go` + ValkeyBackupTargetReconciler.verifyEndpoint() 추가 |
| **ValkeyBackup.Spec.Destination 필드** | 0.3d | 위 | CRD field + webhook validation (`type=PVC | TargetRef`) |
| **BackupJob step 2 — 외부 업로드** | 0.7d | 위 | `valkey-cli --rdb` 후 `mc cp /backup/dump.rdb s3://<target>/` 또는 `aws s3 cp` sidecar |
| **ValkeyRestore Source.TargetRef 활성화** | 0.5d | 위 | Mounting phase 가 외부에서 PVC 로 다운로드 후 기존 init container 흐름 |
| **데이터 plane 검증 (Verifying)** | 0.5d | — | `valkey-cli ping` + `INFO keyspace` 호출 — Status.RestoredKeys 채움 |
| **Replication mode restore (ReadOnlyMany source)** | 1d | source PVC accessMode 변경 매커니즘 | Source.PVC.Name 확인 시 PVC accessMode 가 ROX 인지 검증 |
| **ValkeyCluster restore (shard 별 source)** | 2d | 위 + ShardLayout 매핑 | Shard 별 init container 또는 단일 source + per-shard cp |
| **e2e 시나리오 자동화** | 1d | 위 모두 | `test/e2e/restore_test.go` |

### 2.3 Track B-F 진입 (Track A 완료 후, 5-6주)

Plan §3 참조. P1-P3:
- B (Failover/Scale apply/Resharding) — `internal/controller/{valkey,valkeycluster}_failover.go`
- C (릴리스 파이프라인) — `scripts/release.sh` + ghcr.io publish
- D (Helm chart + 운영 문서) — `kubebuilder edit --plugins=helm/v2-alpha`
- E (Production hardening) — Prometheus alert rules + samples 채우기
- F (Soak + DR + OTEL) — 24h 시나리오 + tracing

---

## 3. 차단점

**현재 없음**. 모든 디자인 분기는 ADR-0015/0016 으로 결정. 다음 세션 즉시 진입 가능.

**진입 시 결정 필요** (작은 분기):
1. **S3 client library**: `aws-sdk-go-v2` vs `minio-go`. 추천 `minio-go` (작음 + MinIO 호환 native). ADR-0022 작성 필요.
2. **CLAUDE.md §2 충돌 검토**: valkey-operator 는 *공개 OSS*. CLAUDE.md "GitHub Actions 영구 금지" + "default builder" 가 적용되는지? 본 repo 에 글로벌 표준 적용 여부 확인. 적용 시 Track C (릴리스 파이프라인) 의 ghcr.io publish 가 *예외 §7-③* 형태로만 허용 — ADR-0019 작성 필요.

---

## 4. 근거 링크

- **Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md` (진단 + 게이트 + 트랙 정의 + 디자인 분기 + 검증 방법)
- **ADR-0015**: `docs/kb/adr/0015-valkeyrestore-init-container-pattern.md` (Init Container vs Restore Job vs PVC swap 거절 사유)
- **ADR-0016**: `docs/kb/adr/0016-valkeybackuptarget-crd-external-storage.md` (ValkeyBackupTarget CRD vs Job sidecar/별도 Upload Job 거절 사유)
- **CLAUDE.md §2**: GitHub Actions 영구 금지 (RFC 0002, 2026-04-29) — 적용 여부 확인 필요
- **CLAUDE.md §8**: auto-cycle 7-phase + atomic commit 정책 (이번 세션 준수)
- **standards/parallelization.md**: 4단계 진단 (격리/테스트/충돌/관찰) — Track A/B/C 병렬 가능 검증 완료

---

## 5. 의사결정 기록

본 세션에서 자가수정 정책 (CLAUDE.md §6) 범위 내 결정:

1. **Restore 패턴: Init Container** (ADR-0015) — RESTORE 명령 / 별도 Job / PVC swap
   대비. valkey-server 의 표준 RDB 자동 로드 매커니즘 활용이 가장 신뢰성 높음.
   Cluster 재시작 (downtime) 발생하지만 disaster recovery 시나리오에서 허용.

2. **외부 저장: ValkeyBackupTarget CRD** (ADR-0016) — Job step sidecar / 별도
   Upload Job 대비. DRY (다중 backup이 동일 target 공유) + Backup ↔ Restore
   대칭성 + 자격증명 분리 통제. 첫 type=S3 + MinIO 호환 (forcePathStyle).

3. **Restore Standalone-only 첫 commit** — Replication / Cluster (multi-pod
   ReadOnlyMany source 충돌) 회피. 단일 PVC RWO 환경에서 작동 보장. 후속
   commit 에서 확장.

4. **paused annotation** — ValkeyRestoreReconciler 가 STS 직접 patch 시
   ValkeyController 가 init container 를 *제거* 하는 충돌 방지. Valkey CRD
   변경 없이 (외과 수술식, §3 정합) 매커니즘 추가.

5. **AWS SDK 도입 시점 분리** — ValkeyBackupTargetReconciler 첫 commit 은
   *schema 기반 검증* 만. SDK 통합은 별개 commit (의존성 검토 단독 PR).

6. **finalizer 추가 후 명시적 requeue** — fake client 의 ResourceVersion
   conflict 회피 + controller-runtime 관용 패턴. 다른 controller (Valkey,
   ValkeyCluster, ValkeyBackup) 패턴은 그대로 유지 (envtest 환경 차이).

---

## 6. 다음 세션 즉시 시작 명령

```bash
# 1. 위치 + 상태 확인
cd /Users/phil/WorkSpace/public/valkey-operator
git log --oneline -10
cat docs/plans/ethereal-fluttering-wand/HANDOFF.md | head -60

# 2. 회귀 검증
go test -count=1 -timeout=120s ./...

# 3. 다음 step 진입 — sonatype-guide 스킬 의무 호출 (의존성 추가 시)
# AWS SDK 통합 시작
```
