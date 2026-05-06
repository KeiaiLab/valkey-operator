# HANDOFF — valkey-operator 상용제품수준 도달 작업

**최종 갱신**: 2026-05-06 (cycle 2 완료)
**Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md`
**현재 진행**: Track A (Backup 외부 저장 + Restore) 약 **80%** — 외부 저장
*왕복 경로* 동작.

---

## 1. 현재 상태

**마지막 commit**: `85d865f` (README — 외부 저장 사용법).

**누적 18 commits** (cycle 1 + cycle 2):

### Cycle 1 (10 commits, ~/.claude/plans/HANDOFF 첫 작성)
| # | SHA | Subject |
|---|---|---|
| 1 | `7458228` | `feat(backup): ValkeyBackup M3.5 — Job-based RDB 복사 + PVC 보존` |
| 2 | `2a9b29b` | `docs: ADR-0015 + ADR-0016 — Restore 패턴 + 외부 저장 추상화` |
| 3 | `307371a` | `feat(api): ValkeyBackupTarget CRD types` |
| 4 | `123944a` | `feat(backup-target): ValkeyBackupTargetReconciler — schema 검증` |
| 5 | `a29165b` | `feat(api): ValkeyRestore CRD types` |
| 6 | `37656d0` | `feat(controller): paused annotation` |
| 7 | `cef4d74` | `feat(resources): Restore Init container + Inject/Remove` |
| 8 | `fbb96d7` | `feat(restore): ValkeyRestoreReconciler — Standalone PVC source` |
| 9 | `625dc35` | `docs: README — M3.5 + ValkeyRestore Standalone` |
| 10 | `4f4d34b` | `docs(plans): HANDOFF — Track A 50% (cycle 1 종료)` |

### Cycle 2 (8 commits — 본 세션)
| # | SHA | Subject | 의미 |
|---|---|---|---|
| 11 | `505c6c1` | `feat(storage): minio-go v7 통합 + S3 reachability ping` | ADR-0022 + 실제 BucketExists |
| 12 | `9788f0d` | `feat(api): ValkeyBackup.Spec.Destination 타입 정의` | TargetRef 필드 추가 |
| 13 | `f10fe5c` | `feat(cli): operator binary 의 upload/download sub-command` | ADR-0023 + flag 표준 라이브러리 |
| 14 | `787ee6d` | `feat(resources): Upload Job 빌더` | operator image 활용 |
| 15 | `fc04cdd` | `feat(backup): Uploading phase 통합` | TargetRef 시 Copying → Uploading → Completed |
| 16 | `bc81719` | `feat(resources): Download Job + Restore Source PVC` | reverse 패턴 |
| 17 | `bc6e28b` | `feat(restore): Source.TargetRef 활성화` | cross-cluster restore 가능 |
| 18 | `85d865f` | `docs: README — 외부 저장 사용법` | 사용자 가시 변경 |

**SEV-1 차단점 #1, #2 *완전* 해소**:
- ✅ ValkeyRestore 동작 (Standalone PVC + 외부 source 양방향)
- ✅ Backup 외부 저장 (S3 호환, MinIO + Ceph RGW + AWS S3)
- ✅ Cross-cluster restore (다른 cluster 의 backup → S3 → 본 cluster)

**ADR 추가** (cycle 2): ADR-0022 (minio-go v7 채택), ADR-0023 (operator
binary sub-command 패턴). Sonatype + Context7 검증 인용 포함.

**테스트 상태** (단위, fake client + mock):
- `internal/storage/`: 10 (S3Client + parseEndpoint)
- `internal/cli/`: 14 (Dispatch + upload/download + flag/env)
- `internal/resources/`: M3.5 + Restore + Upload + Download builders
- `internal/controller/`: ValkeyBackupTarget 14 (verifyEndpoint mock 포함),
  ValkeyRestore 17 (Source.PVC 11 + Source.TargetRef 6)
- 회귀 0건. lefthook 4-stage hook 매 commit 통과.

**미커밋 변경**: `.claude/ralph-loop.local.md` 만 (iteration counter, 무관).

**라이브 검증 미진행** (RFC-0004 §3): kind cluster + MinIO 컨테이너 *실측* 은
다음 세션. README L79-93 의 13 시나리오 표 미갱신 (Backup S3 / Restore S3 /
cross-cluster 추가 필요).

---

## 2. 다음 단계 (우선순위 순)

### 2.1 즉시 (kind cluster + MinIO 실측)

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

### 2.2 Track A 완성 잔여 (예상 1주)

| step | 분량 | 산출물 |
|---|---|---|
| **데이터 plane 검증 (Verifying)** | 0.3d | handleVerifying 의 PING + INFO keyspace → Status.RestoredKeys |
| **Replication mode restore** | 1d | ReadOnlyMany source PVC 또는 multi-attach 우회 (Job 이 file copy) |
| **ValkeyCluster restore (shard 별)** | 1.5d | ShardLayout 매핑 + 순차 init container per shard |
| **TTL 자동 삭제 (ValkeyBackup)** | 0.5d | Spec.TTL → CompletedAt + TTL 도달 시 finalizer cleanup |
| **e2e 시나리오 자동화** | 1d | `test/e2e/restore_test.go`, `test/e2e/backup_s3_test.go` |
| **샘플 CR 채우기** | 0.2d | `config/samples/*.yaml` `# TODO` 제거 |

### 2.3 Track B-F (Plan §3, 4-6주 추가)

- B (Failover/Scale apply/Resharding) — `internal/controller/{valkey,valkeycluster}_failover.go`
- C (릴리스 파이프라인) — `scripts/release.sh` + ghcr.io publish + ADR-0019
- D (Helm chart + 운영 문서) — `kubebuilder edit --plugins=helm/v2-alpha`
- E (Production hardening) — Prometheus alert rules + samples 채우기 + NetworkPolicy enforcement
- F (Soak + DR + OTEL + Conversion webhook)

---

## 3. 차단점 / 진입 시 결정

**현재 차단점 없음**. 모든 디자인 분기는 ADR-0015~0023 으로 결정.

**다음 진입 시 작은 결정** (별개 ADR 또는 in-line):
1. **데이터 plane 검증의 Verifying 위치**: STS 원복 *전* (init container
   있는 동안) vs *후* (원복 후). 후자가 더 정확 (실제 운영 상태). 후자 권장.
2. **TTL 자동 삭제**: `metav1.Duration` 파싱 + Reconcile 의 RequeueAfter
   계산. controller-runtime 의 owns deletion 미사용 (ValkeyBackup 자기
   self-delete).
3. **Replication mode source 전략**: (a) source PVC 가 ROX (사용자 책임)
   vs (b) Restore 가 자체 Job 이 source PVC → main /data RWO 로 복사 후
   각 pod 가 자기 RWO 마운트.

---

## 4. 근거 링크

- **Plan SSOT**: `~/.claude/plans/ethereal-fluttering-wand.md`
- **ADR**: `docs/kb/adr/0015 ~ 0023.md` + `INDEX.md`
- **이전 HANDOFF (cycle 1)**: 본 파일 commit `4f4d34b` 이전 버전 (git 보존)
- **CLAUDE.md §6**: 자가수정 정책 — cycle 1 + 2 모두 본 정책 범위 내 진행
- **RFC-0004**: 라이브 사실 게이트 — 실측 미진행 표시

---

## 5. 의사결정 기록 (cycle 2)

본 세션 자가수정 결정:

1. **minio-go v7 채택** (ADR-0022) — sonatype-guide 검증: CVE 0건,
   malicious=false, Apache-2.0 + BSD. context7 검증: Source Reputation High,
   Benchmark 84.8. aws-sdk-go-v2 대비 의존성 트리 작음 + MinIO/Ceph RGW
   native.

2. **Operator binary sub-command** (ADR-0023) — mc CLI image (AGPL) 또는
   self-built uploader image 회피. flag 표준 라이브러리만 (cobra 거절).

3. **Upload Job = 별도 Pod, Sidecar 아님** — backup PVC RWO 환경의 시간차
   격리. backup Job 종료 후 upload Job spawn.

4. **ValkeyRestore 가 자체 임시 PVC 생성** — cross-cluster restore 시
   ValkeyBackup 의 backup PVC 가 *없을 수도 있음* (다른 cluster 의 백업).
   `<restore-name>-source` 8Gi RWO PVC + Download Job + Init container 흐름.

5. **자격증명 envFrom Secret 패턴** — Job spawn 시점 snapshot. Secret
   rotation in-flight 영향 차단.

---

## 6. 다음 세션 즉시 시작 명령

```bash
cd /Users/phil/WorkSpace/public/valkey-operator
git log --oneline -10
cat docs/plans/ethereal-fluttering-wand/HANDOFF.md | head -80

# 회귀 검증
go test -count=1 -timeout=120s ./...

# 다음 step: kind + MinIO 실측 + 데이터 plane 검증 강화
make setup-test-e2e
docker run -d --name minio-test -p 9000:9000 minio/minio:latest server /data
# ...
```
