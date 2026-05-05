# ADR-0010: cert-manager Certificate auto-discovery

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil
- Refs: ADR-0008 (TLS RootCAs from CustomCert)

## Context

ADR-0008 은 사용자가 `Spec.TLS.CustomCert.SecretName` 으로 CA Secret 을 *수동 명시*
해야 RootCAs 가 채워지는 부분 구현이었다. cert-manager 통합은 별도 ADR 로 분리.

iter 6 시점 cert-manager 통합 가능 시그널:
- ServiceMonitor unstructured 패턴이 동작 검증 — 같은 패턴 재사용 가능.
- `applyServiceMonitor` 가 `meta.IsNoMatchError` fail-soft 처리 — CRD 미설치 시
  자연스럽게 무시.

## Decision

`Spec.TLS.CertManager.IssuerRef.Name` 명시 시 operator 가 *자동으로*:

1. cert-manager `Certificate` CR (`cert-manager.io/v1`) 을 unstructured 로 생성.
   - secretName: `<crName>-tls`
   - dnsNames: client / headless / pod 모두 커버
   - issuerRef: 사용자 명시 (Issuer 또는 ClusterIssuer)
2. `tlsConfigForCluster` 가 위 secretName 의 `ca.crt` 를 자동으로 RootCAs 에 로드.

사용자는 `CustomCert.SecretName` 을 *명시할 필요 없음* — Issuer 만 알려주면 끝.

## Consequences

**긍정:**
- cert-manager 통합이 1 단계 (Issuer 명시) 로 단순화.
- 인증서 회전 시 cert-manager 가 Secret 갱신 → 다음 reconcile 에서 자동 reload.
- ADR-0003 의 InsecureSkipVerify 임시 결정이 *대부분 케이스* 에서 사라짐 (CertManager
  미명시 + CustomCert 미명시 시에만 fallback).

**부정:**
- cert-manager CRD 미설치 시 Certificate apply 가 NoMatchError → fail-soft 무시.
  사용자에게 "왜 TLS 가 동작 안 하는지" 명시적 통지 부재 (M3 에서 condition 추가).
- Secret 가 Ready 되기 전 reconcile 실행 시 InsecureSkipVerify fallback 잠시 활성 →
  TLS pinning gap (수 초). 운영에서 큰 문제 아님.

## 후속 작업

- M3: `Status.Conditions` 에 `CertReady` condition 추가 (cert-manager Certificate 의
  Ready status 추적).
- M4: PeriodicCertWatch — Secret 변경 시 reconcile trigger 위해 `Owns(&corev1.Secret{})`
  추가 검토 (현재는 30s polling 으로 자연 갱신).

## Alternatives Considered

- **ADR-0008 의 CustomCert 만 유지:** 사용자 부담 (Issuer 가 만든 Secret 이름 추측).
- **cert-manager 직접 의존성 추가:** import "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
  → 무거움 + 버전 lock-in. unstructured 가 깔끔.
