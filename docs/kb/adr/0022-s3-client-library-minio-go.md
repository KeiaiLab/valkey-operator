# ADR-0022: S3 Client Library — minio-go v7 채택

- Date: 2026-05-06
- Status: Accepted
- Authors: @phil

## Context

ADR-0016 (ValkeyBackupTarget CRD) 의 후속 — *실제 S3 reachability + Backup
업로드 + Restore 다운로드* 구현 시점이 도래. Go 의 S3-compatible client
라이브러리 선택이 필요.

후보:
1. **`github.com/aws/aws-sdk-go-v2`** + `service/s3` — AWS 공식, 모듈식.
2. **`github.com/minio/minio-go/v7`** — MinIO 공식, AWS S3 + MinIO + Ceph
   RGW 호환.
3. **`github.com/aws/aws-sdk-go`** (v1) — Legacy, maintenance mode.

요구사항:
- BucketExists (reachability) — HEAD bucket 호출
- FPutObject (Backup Job step 2 — `/backup/dump.rdb` → s3://target/path)
- FGetObject (Restore Mounting phase — s3://target/path → backup PVC)
- forcePathStyle 옵션 (MinIO + Ceph RGW)
- 자격증명: static (Secret 의 access/secret key) — IAM/STS 우선순위 낮음
- 8GB+ 파일 스트리밍 (메모리 OOM 방지)

## Decision

**minio-go v7.1.0** 채택.

### 의사결정 근거 (검증 인용)

**Sonatype 검증** (2026-05-06):
```
pkg:golang/github.com%2Fminio%2Fminio-go%2Fv7
- v7.1.0 (latest, 2026-04-26)
- vulnerabilities: {} (CVE 0건)
- malicious: false
- licenses: Apache-2.0 + BSD-UNSPECIFIED
- endOfLife: false
```

**Context7 검증** (2026-05-06):
- Library ID: `/minio/minio-go`
- Source Reputation: **High**
- Benchmark Score: 84.8
- Code Snippets: 354

**API 매핑**:
- `minio.New(endpoint, &minio.Options{Creds, Secure, Region, BucketLookup})`
- `BucketExists(ctx, bucketName)` → reachability
- `FPutObject(ctx, bucket, object, filePath, PutObjectOptions{})` → 업로드
- `FGetObject(ctx, bucket, object, filePath, GetObjectOptions{})` → 다운로드
- `BucketLookup: minio.BucketLookupPath` → forcePathStyle=true 매핑

## Consequences

긍정:
- **의존성 트리 작음**: 단일 모듈 + 표준 라이브러리만 사용. binary 크기
  영향 ~3-5 MB.
- **MinIO + AWS S3 + Ceph RGW + IBM COS 모두 호환** — 사내 / 클라우드 양방향.
- **forcePathStyle native 지원** (BucketLookup 옵션) — ADR-0016 의 사용자
  field 직접 매핑.
- **F-prefix 파일 API**: PutObject/GetObject 와 별개로 FPutObject/FGetObject
  가 파일 경로 직접 받음 — 메모리 streaming 거치지 않음, 8GB+ RDB 안전.
- **Apache-2.0 + BSD 라이선스**: valkey-operator 의 MIT 라이선스와 호환,
  GPL 오염 없음.

부정:
- **AWS 의 advanced 기능 부재**: S3 Object Lambda, Multi-Region Access
  Points, CloudWatch 직접 통합 등 AWS-specific 기능은 별도 구현 필요. 현
  요구사항 (BucketExists / FPut / FGet) 에는 무관.
- **STS / IAM Role 자격증명 native 지원 약함** — minio-go 는 static
  credentials 우선. 추후 IAM/IRSA (Kubernetes Pod-level role) 지원이 필요해
  지면 별도 SDK 통합 또는 wrapper 작성. 현재는 Secret-based static 만
  지원하는 ADR-0016 와 정합.

## Alternatives Considered

1. **aws-sdk-go-v2 + service/s3** (옵션 1 거절):
   - 의존성 트리 큼 — `aws-sdk-go-v2` core + `aws-sdk-go-v2/config` +
     `aws-sdk-go-v2/credentials` + `aws-sdk-go-v2/service/s3` 등. binary
     크기 ~10-15 MB 추가.
   - MinIO 호환 위해 `EndpointResolver` 직접 작성 필요 (forcePathStyle
     매핑이 native 아님).
   - AWS-only advanced 기능 (CloudWatch / Object Lambda) 는 본 프로젝트
     요구사항 외.
   - 거절: 의존성 / 복잡도 비용 vs 추가 기능 의 trade-off 가 *추가 기능
     불필요* 측에서 minio-go 우위.

2. **aws-sdk-go v1** (옵션 3 거절):
   - Maintenance mode (AWS 공식 v2 권장). 신규 프로젝트 도입 시 사후 마이
     그레이션 부담.
   - 거절.

3. **자체 HTTP client + AWS SigV4 직접 구현**: 가장 작은 의존성이지만
   *재발명* 비용 + 보안 위험 (SigV4 구현 오류 위험). 거절.

## Action Items

- [x] AI-001: `go get github.com/minio/minio-go/v7@v7.1.0`
- [ ] AI-002: `internal/storage/s3_client.go` — wrapper (Build, BucketExists,
      FPutObject, FGetObject) + ValkeyBackupTarget.Spec.S3 → minio.Options
      변환
- [ ] AI-003: ValkeyBackupTargetReconciler 의 `verifyEndpoint()` 추가 —
      `BucketExists` 호출 + Phase=Reachable 갱신
- [ ] AI-004: 단위 테스트 — httptest server 또는 minio test container
- [ ] AI-005: BackupJob step 2 — `mc cp` 또는 `valkey-operator` binary 의
      sub-command 로 업로드 (image 추가 부담 vs 단일 binary 결정 필요 —
      별개 ADR 가능)
- [ ] AI-006: ValkeyRestore Source.TargetRef 활성화 — Mounting phase 의
      외부 다운로드

Refs: ADR-0016 (ValkeyBackupTarget CRD — 본 ADR 의 활용 정의).
