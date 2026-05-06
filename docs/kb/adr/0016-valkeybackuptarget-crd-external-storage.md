# ADR-0016: ValkeyBackupTarget CRD 도입 — S3-compatible 외부 저장 추상화

- Date: 2026-05-06
- Status: Accepted
- Authors: @phil

## Context

ValkeyBackup M3.5 (commit 7458228) 까지 *backup PVC 보존* 만 동작.
그러나 **외부 저장 (S3/GCS/Azure) 미구현** — RDB 가 Kubernetes PVC 에만
저장되어 *cluster 자체 손실* (e.g. 전체 namespace 삭제, etcd 손실, region
장애) 시 backup 도 함께 소실 (plan ethereal-fluttering-wand SEV-1 차단점 #2
*완전 해소* 미달성).

외부 저장 구현 패턴 3안:

1. **Job step 에 aws-cli sidecar**: ValkeyBackup.Spec 에 직접 S3 endpoint /
   bucket / 자격증명 secret 명시. backup Job 의 후속 step (또는 별도 container)
   이 PVC → S3 업로드.

2. **별도 Upload Job**: backup Job 종료 후 controller 가 별도 Upload Job spawn.

3. **ValkeyBackupTarget CRD 추상화**: 외부 저장 endpoint / bucket / 자격증명을
   별도 CR 로 정의. ValkeyBackup.Spec.Destination.TargetRef 가 이를 참조.

## Decision

**ValkeyBackupTarget CRD 채택** (옵션 3):

1. **신규 CRD `ValkeyBackupTarget`**:
   ```yaml
   apiVersion: cache.keiailab.io/v1alpha1
   kind: ValkeyBackupTarget
   metadata:
     name: s3-prod
     namespace: valkey-prod
   spec:
     type: S3        # 첫 구현. GCS/Azure 추후 type 분기 추가.
     s3:
       endpoint: https://s3.amazonaws.com   # MinIO 호환 endpoint 도 가능
       region: ap-northeast-2
       bucket: valkey-backups-prod
       prefix: cluster-A/
       forcePathStyle: false                # MinIO 시 true
       credentialsSecretRef:                # access_key / secret_key
         name: s3-prod-creds
         keys:
           accessKeyID: AWS_ACCESS_KEY_ID
           secretAccessKey: AWS_SECRET_ACCESS_KEY
       # serverSideEncryption: aws:kms (옵션)
   status:
     reachable: true
     lastVerifiedAt: 2026-05-06T...
     conditions: [...]
   ```

2. **ValkeyBackup.Spec 확장**:
   ```yaml
   spec:
     clusterRef: { kind: ValkeyCluster, name: vc-prod }
     type: RDB
     destination:
       type: PVC | TargetRef         # default: PVC (M3.5 호환)
       targetRef:                    # type=TargetRef 시
         name: s3-prod
         pathTemplate: "{{.ClusterName}}/{{.Date}}/dump.rdb"
       # PVC fields (M3.5) 는 그대로 유지 — type=PVC 의 default
   ```

3. **backup Job 의 단계 확장**:
   - Step 1: `valkey-cli --rdb /backup/dump.rdb` (M3.5 — PVC 저장)
   - Step 2 (TargetRef 시): `mc cp /backup/dump.rdb <target>/<path>` 또는
     `aws s3 cp` — 별도 sidecar 또는 동일 container 의 후속 명령.

4. **ValkeyRestore (ADR-0015) 와의 대칭성**: ValkeyRestore.Spec.Source 도
   동일 ValkeyBackupTarget 참조 가능 → 외부에서 RDB 다운로드 후 init
   container 로 cluster 에 복원.

## Consequences

긍정:
- **DRY**: 다수 backup 이 동일 target 공유 (자격증명 1회 정의).
- **Backup ↔ Restore 대칭성**: 동일 CRD 가 양방향 참조 가능 → 사용자 인지
  부담 감소.
- **Type-별 확장**: S3 1차, GCS / Azure / WebDAV 추가 시 `Spec.type` 분기 +
  새 sub-spec 만 — 기존 ValkeyBackup CRD 변경 불필요.
- **자격증명 보안**: Secret reference 분리 — RBAC 으로 ValkeyBackupTarget
  접근 만 별도 통제 가능.
- **MinIO 호환**: `endpoint` + `forcePathStyle` 로 사내 S3-compatible
  스토리지 (Ceph RGW, MinIO) 지원.

부정:
- **추가 CRD = 추가 webhook + RBAC + 학습 비용**: 사용자가 backup 1회
  사용하려 해도 target CR 먼저 만들어야 함. *완화*: type=PVC 는 target 불필요
  (M3.5 호환).
- **status.reachable 검증** 매커니즘 필요 — controller 가 주기적 ping (HEAD
  bucket) 또는 첫 사용 시 검증.
- **자격증명 회전**: secret 변경 시 모든 in-flight backup Job 영향. *완화*:
  Job 은 spawn 시점 secret snapshot 사용 (envFrom Secret) — 회전 후 새 Job 만
  새 자격증명.

## Alternatives Considered

1. **Job step 에 직접 명시** (옵션 1 거절):
   - 단순하지만 동일 자격증명 / endpoint 가 모든 ValkeyBackup CR 에 반복 →
     DRY 위반.
   - Restore 시 같은 정보 다시 명시 — 대칭성 깨짐.
   - 거절.

2. **별도 Upload Job** (옵션 2 거절):
   - 두 Job (backup + upload) 의존성 controller 가 관리해야 함.
   - 두 Job pod 가 동일 PVC 마운트 → ReadWriteOnce 환경에서 시퀀싱 어려움.
   - 거절: 옵션 3 + 단일 Job 의 다단계 step 이 더 단순.

3. **CSI driver 기반 PVC → object storage 자동 동기화**: 매우 인프라-종속
   (CSI driver 별 매커니즘 다름). 거절.

4. **Velero 같은 외부 도구 의존**: 사용자 환경 가정 너무 강함. 거절.

## Action Items

- [ ] AI-001: ValkeyBackupTarget CRD 정의 (`api/v1alpha1/valkeybackuptarget_types.go`)
- [ ] AI-002: ValkeyBackupTargetReconciler — status.reachable 검증
- [ ] AI-003: ValkeyBackup.Spec.Destination 필드 확장
- [ ] AI-004: BackupJob 빌더 확장 — step 2 (mc cp 또는 aws s3 cp)
- [ ] AI-005: 자격증명 envFrom 패턴 + 자격증명 secret rotation 시 영향
      문서화 (`docs/operations/backup.md`)
- [ ] AI-006: ValkeyRestore.Spec.Source.TargetRef 도 동일 CRD 참조
- [ ] AI-007: e2e — MinIO 컨테이너 + ValkeyBackupTarget 검증
- [ ] AI-008: README 운영 시나리오 표 갱신

Refs: ADR-0015 (ValkeyRestore — 동일 target 참조 대칭성),
plan ethereal-fluttering-wand Track A SEV-1 차단점 #2.
