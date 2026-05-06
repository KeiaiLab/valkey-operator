# Security Policy

## 취약점 보고

**공개 issue 로 보고하지 마세요.** 보안 취약점이 패치되기 전에 공개 노출
되면 사용자 피해 가능성이 큽니다.

### 보고 경로

다음 중 하나로 비공개 보고:

1. **GitHub Security Advisory** (권장):
   `https://github.com/keiailab/valkey-operator/security/advisories/new`
2. **이메일**: `security@keiailab.com` (PGP 옵션 — 키는 GitHub
   profile 의 verified PGP key)

### 포함 정보

- 영향받는 버전 (release tag 또는 commit SHA)
- 재현 단계 (가능한 한 minimal repro)
- 영향 평가 (CVSS 자체 평가 시 포함)
- 발견자 — 공로 인정 원하시면 명시

## 응답 SLA

| 단계 | 시간 |
|---|---|
| 초기 응답 (수신 확인) | 72시간 이내 |
| 심각도 평가 | 7일 이내 |
| 패치 release | severity 따라 (Critical: 14일, High: 30일, Medium: 60일) |
| 공개 disclosure | 패치 release 후 14일 (사전 협의 가능) |

## 지원 버전

| Version | Supported |
|---------|-----------|
| 0.x (alpha) | ✅ 최신 minor 만 |
| (1.0+ stable) | (TBD — 첫 stable release 후 갱신) |

현재 v1alpha1 단계. *하위 호환성 보장 없음* — 보안 패치는 *최신* 버전에만.

## 보안 모범 사례 (사용자 측)

valkey-operator 운영 시:

1. **TLS 강제** — `Spec.TLS.Enabled=true` (cert-manager 또는 CustomCert).
   ADR-0010, ADR-0014.
2. **Auth 강제** — ADR-0013 에 따라 사실상 항상 enabled. 32B random
   password 자동 생성.
3. **NetworkPolicy** — `Spec.NetworkPolicy.Enabled=true` 로 pod-to-pod
   ingress 제한. CNI 가 NP 강제 (Calico/Cilium) 환경에서 검증.
4. **PSS Restricted** — namespace 에 `pod-security.kubernetes.io/enforce=restricted`.
5. **자격증명 Secret 분리** — ValkeyBackupTarget 의 S3 credentials 는
   별도 Secret + RBAC 으로 접근 통제. ADR-0016.
6. **Backup 외부 저장 권장** — `Destination.Type=TargetRef` + 외부 S3.
   PVC-only 는 cluster 자체 손실 시 backup 도 사라짐.
7. **컨테이너 이미지 검증** — operator image 는 Sonatype + context7 검증
   통과한 의존성만 (ADR-0022 참조). 자체 빌드 시 trivy / grype 스캔 권장.

## 의존성 보안

본 프로젝트의 의존성은 ADR 작성 시 *Sonatype Trust Score* + *Context7
검증* 인용을 원칙으로 합니다 (`docs/kb/adr/0022-*.md` 참조).

Dependabot / Renovate 자동 업데이트 PR 은 우선 review.

## 알려진 한계 (현재 버전)

`README.md` 의 "잠재적 운영 이슈" 섹션 + GitHub Issues label `security` 참조.
