# ADR-0023: Operator binary 가 upload/download sub-command 지원 — 이미지 통합

- Date: 2026-05-06
- Status: Accepted
- Authors: @phil

## Context

ADR-0016 (ValkeyBackupTarget) + ADR-0022 (minio-go) 로 외부 저장 (S3) 통합
의 *클라이언트* 가 Go 코드로 가능. 그러나 *어디서* 호출하는가의 분기:

1. **별도 OCI image (e.g. `mc` minio CLI)** — Job spec 의 container.image
   가 별도. 추가 의존성 + 라이선스 검토 + 버전 추적.
2. **자체 빌드 별도 image (`valkey-operator-uploader`)** — 두 image 빌드 +
   release 부담.
3. **operator image 가 controller + sub-command 양쪽 역할** — 단일 image,
   `valkey-operator` binary 가 첫 인자로 분기.

요구사항:
- BackupJob 이 RDB 를 외부 저장 에 업로드 (Backup Uploading phase)
- ValkeyRestore Mounting phase 가 외부 저장 에서 PVC 로 다운로드
- 자격증명은 ValkeyBackupTarget 의 Secret 에서만 주입 — Job spec 의
  envFrom + 보안 격리

## Decision

**옵션 3 채택**. operator binary 의 `os.Args[1]` 분기:

```
valkey-operator                              → controller manager (default)
valkey-operator upload --bucket=X --object=Y --file=/backup/dump.rdb
                                             → S3 업로드 sub-command
valkey-operator download --bucket=X --object=Y --file=/restore/dump.rdb
                                             → S3 다운로드 sub-command
```

자격증명 + endpoint 는 env:
- `VALKEY_S3_ENDPOINT` (e.g. https://s3.amazonaws.com)
- `VALKEY_S3_REGION`
- `VALKEY_S3_FORCE_PATH_STYLE` (true|false)
- `VALKEY_S3_ACCESS_KEY_ID`
- `VALKEY_S3_SECRET_ACCESS_KEY`

cobra 같은 CLI library 도입 *안 함* — `flag` standard library 사용.
§2 Simplicity First.

## Consequences

긍정:
- **단일 image** — operator image 만 build / push / pull. Job spec 의
  `image: controller:dev` 가 그대로.
- **자격증명 격리** — envFrom Secret 패턴으로 process 외부 노출 0.
  RDB 파일은 PVC mount, 자격증명은 env — 두 attack surface 분리.
- **재사용** — ValkeyBackup (Uploading) + ValkeyRestore (Mounting) 양쪽
  같은 sub-command 활용.
- **버전 일관성** — controller 와 uploader 가 동일 minio-go 버전 사용
  (한 binary 안). drift 0.

부정:
- **main.go 가 두 codepath** — controller manager / sub-command. 명료성
  약간 감소. *완화*: `internal/cli/` 패키지에 분리, main.go 는 dispatch 만.
- **operator image 크기 증가** — minio-go + 부수 의존성 ~5MB. Distroless
  base 위에서 무시 가능.

## Alternatives Considered

1. **별도 mc image** (옵션 1 거절):
   - 추가 image 의존성 (registry 별 pull policy + license 검토).
   - mc 는 AGPL — *Commercial use* 시 sourceless 가 위험. 회피.

2. **별도 self-built uploader image** (옵션 2 거절):
   - 두 image build / push / version 관리 부담.
   - controller-uploader minio-go version drift 위험.
   - 거절.

3. **cobra 등 CLI library 도입**: 단일 sub-command 분기에 외부 의존성 과다.
   `flag` 표준 라이브러리로 충분. 거절.

## Action Items

- [ ] AI-001: `internal/cli/upload.go` + `download.go` — flag-based 인자
      파싱 + storage.S3Client 호출
- [ ] AI-002: `cmd/main.go` 첫 인자 분기 — `upload`, `download` 인 경우
      `internal/cli` 호출 후 `os.Exit`. 그 외 controller manager.
- [ ] AI-003: ValkeyBackupReconciler 의 Uploading phase — Job 이
      `valkey-operator upload ...` 호출
- [ ] AI-004: ValkeyRestoreReconciler 의 Mounting phase 가 TargetRef 시
      별도 Download Job (또는 init container) spawn
- [ ] AI-005: 단위 테스트 — flag 파싱, 실패 path

Refs: ADR-0016, ADR-0022.
