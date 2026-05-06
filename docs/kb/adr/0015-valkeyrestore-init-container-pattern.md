# ADR-0015: ValkeyRestore — Init Container 기반 RDB 로드 + STS Pod 재시작

- Date: 2026-05-06
- Status: Accepted
- Authors: @phil

## Context

ValkeyBackup M3.5 (commit 7458228) 까지 *백업 PVC 에 dump.rdb 저장* 은 동작.
그러나 **Restore 경로 부재** — RDB 가 있어도 cluster 로 복원할 controller 로직
0줄 (plan ethereal-fluttering-wand SEV-1 차단점 #1).

Restore 구현 패턴 3안 검토:

1. **Init Container 패턴**: STS PodSpec.InitContainers 에 RDB 복사 단계 삽입.
   `<source-pvc>/dump.rdb` 를 valkey 컨테이너 의 `/data/dump.rdb` 로 복사 후
   메인 valkey-server 가 *기동 시 자동 RDB 로드* (Valkey 의 기존 매커니즘).

2. **별도 Restore Job + Cluster 재시작**: Job 이 PVC 에서 RDB 를 직접 읽어
   `valkey-cli RESTORE <key> <ttl> <dump>` 로 key 단위 복원. Cluster 재시작 없음.

3. **valkey-cli `--rdb` 역방향**: 클라이언트 측에서 RDB 를 SYNC 프로토콜 로
   서버 에 push — Valkey 자체가 이를 *공식 지원하지 않음*.

추가 제약:
- ValkeyCluster (sharded) 는 *shard 별 RDB* 가 분리되어야 함 — 각 shard 의
  primary 가 복원 시점에 자기 shard slot 데이터만 가져야.
- TLS 활성 cluster 도 동일 흐름 보장 필요.

## Decision

**Init Container 패턴 채택** (옵션 1):

1. ValkeyRestore CR 생성 시 controller 가 다음 reconcile:
   - Source PVC (또는 외부 — ADR-0016) 에서 RDB 파일 위치 확인.
   - 대상 Valkey/ValkeyCluster CR 의 STS PodTemplate 에 *임시 init container*
     추가 (`busybox` 또는 동일 valkey 이미지). init container 가
     `/source/dump.rdb` → `/data/dump.rdb` 복사.
   - STS rolling restart (init container 변경 = template hash 변경 = pod 재생성).
   - 모든 pod ready + valkey-server 기동 → 자동 RDB 로드.
   - 검증: 임의 key sample 의 SET/GET, ValkeyCluster 면 cluster_state=ok.
   - Restore 완료 → 임시 init container 제거 (STS 재 reconcile, 추가 rolling).

2. ValkeyCluster 의 경우:
   - shard 별 source PVC 매핑 (`Spec.Source.ShardSources[shardIndex] = pvcName`).
   - 또는 단일 source PVC 안 에 `shard-0/dump.rdb`, `shard-1/dump.rdb` 디렉토리 구조.

3. ValkeyRestore.Spec:
   ```yaml
   spec:
     clusterRef:           # Valkey 또는 ValkeyCluster
       kind: ValkeyCluster
       name: vc-prod
     source:
       pvc:
         name: backup-2026-05-05
         shardLayout:      # cluster mode 시 — 단일 PVC 안 디렉토리 매핑
           shard0: shard-0/dump.rdb
           shard1: shard-1/dump.rdb
           shard2: shard-2/dump.rdb
       # 또는 ADR-0016 의 ValkeyBackupTarget 참조
       targetRef:
         name: s3-prod-backup
         path: 2026-05-05/
     restoreType: RDB     # AOF 는 추후
   status:
     phase: Pending|Mounting|Restoring|Verifying|Completed|Failed
     restoredBytes: 12345
     conditions: [...]
   ```

## Consequences

긍정:
- **valkey 의 기존 RDB 로딩 매커니즘 그대로 활용** — 패치/우회 없음.
  Valkey 가 자체적으로 RDB 무결성 검증 + 메모리 로드. 우리는 *파일 배치* 만
  책임.
- **Cluster 토폴로지 보존**: shard 별 RDB 가 자기 shard 에 로드 → cluster
  slot 매핑 유지.
- **TLS / Auth 자동 호환**: STS PodSpec 그대로 — TLS Secret 마운트, Auth
  Secret 환경변수 모두 이미 정의되어 있음.
- **Init container 단순성**: 단일 `cp` 명령 — 복잡한 redis 프로토콜 처리
  불필요.

부정:
- **Cluster 재시작 (downtime) 발생** — rolling 이지만 primary 가 재시작 시
  잠시 unavailable. *허용 가능*: disaster recovery 시나리오는 이미 down 상태
  에서 시작되거나, 사용자가 명시적 restore 요청 시 downtime 수용.
- **STS template 임시 변경 → 원복** 의 두 단계 reconcile 필요. 중간 상태에서
  controller crash 시 init container 가 영구적으로 남을 위험 → finalizer 로
  cleanup 보장.
- **ValkeyCluster 다 shard restore** 는 *모든 shard 가 동시에 재시작* 시
  cluster 일시적 unavailable. 옵션 (A) shard 순차 restore (각 shard ready 후
  다음) — slow but safe / (B) 동시 restore — fast but downtime 길음. 기본 (A).

## Alternatives Considered

1. **별도 Restore Job + key-단위 RESTORE 명령** (옵션 2 거절):
   - `valkey-cli RESTORE` 는 *직렬화된 단일 key* 만 받음 — RDB 파일 통째
     로드 아님. RDB 파싱 후 key 별 RESTORE 호출 필요 → *RDB 라이브러리
     필요* + 성능 저하 (수만 key 각각 RESTORE).
   - cluster 모드 에서 slot ownership 유지 어려움 (key migration 필요).
   - 거절: 복잡도 + 신뢰성 모두 옵션 1 보다 낮음.

2. **valkey-cli --rdb 역방향 streaming** (옵션 3 거절):
   - Valkey 가 클라이언트 → 서버 RDB push 를 공식 지원 안 함. SYNC 프로토콜
     은 마스터→복제 단방향.
   - 거절: 비공식 매커니즘 의존 위험.

3. **PVC swap** (대안):
   - 기존 `<sts-name>-data-<i>` PVC 를 backup PVC 로 교체 후 STS 재시작.
   - PVC 직접 조작 = controller 외부 의 가장 위험한 변경. PVC binding 재조정
     실패 시 데이터 영구 손실 가능.
   - 거절: blast radius 가 너무 큼.

## Action Items

- [ ] AI-001: ValkeyRestore CRD 정의 (`api/v1alpha1/valkeyrestore_types.go`)
- [ ] AI-002: ValkeyRestoreReconciler 구현 (`internal/controller/valkeyrestore_controller.go`)
- [ ] AI-003: 임시 init container 빌더 (`internal/resources/restore_init_container.go`)
- [ ] AI-004: STS template patch + 원복 매커니즘 (finalizer 보장)
- [ ] AI-005: 단위 테스트 (envtest) — Pending → Mounting → Restoring →
      Completed 흐름
- [ ] AI-006: e2e 시나리오 — backup → cluster delete → restore →
      데이터 plane 검증 (SET 한 key 가 보존)
- [ ] AI-007: cluster mode shard 별 restore — 순차 / 동시 옵션
- [ ] AI-008: README 운영 시나리오 표 갱신

Refs: ADR-0010 (cert-manager — TLS Secret 자동 마운트),
ADR-0014 (TLS volume mount — restore 시 그대로 적용),
plan ethereal-fluttering-wand Track A SEV-1 차단점 #1.
