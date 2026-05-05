# ADR-0003: Temporary InsecureSkipVerify until cert-manager CA wiring

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil

## Context

`Spec.TLS.Enabled=true` 시 operator 가 Valkey 노드에 TLS 로 접속해야 한다.
Valkey TLS 모드는 server cert 만으로 동작 (mTLS 는 별도 옵션).

Cert chain 검증을 위해 operator pod 가 *Issuer 의 CA 번들* 을 신뢰해야 함.
완전한 구현은 아래를 요구:

1. cert-manager `Issuer` / `ClusterIssuer` 가 CA Secret 을 생성.
2. Operator pod 의 STS Volume 에 해당 CA Secret 을 마운트.
3. `crypto/tls.Config.RootCAs` 에 `x509.NewCertPool()` 로 로드.
4. `cmd/main.go` 에서 startup 시 watch — CA 회전 시 reload.

## Decision

본 PR 에서는 `tls.Config.InsecureSkipVerify=true` + `ServerName` 설정 (SNI 만 보장).
주석 + ADR 참조로 명시. Beta 단계 (M3) 에서 cert-manager 통합 PR.

근거:
- TLS 가 *비활성화* 되어 있던 기존 상태 (`dialPod` 가 Spec.TLS 무시) 보다는 *유의미한
  진보* — 최소한 traffic encryption 이 동작.
- CA bundle 마운트는 Helm chart / Kustomize overlay 변경까지 동반 — Surgical Changes 위반.
- InsecureSkipVerify 의 위협 모델: cluster 내부 pod-to-pod 트래픽이 이미 K8s NetworkPolicy
  + CNI 격리 하에 있어 *MITM 위험은 namespace 내부 침입자 한정*.

## Consequences

**긍정:**
- TLS 활성화 사용자가 *최소한 평문 노출은 회피*.
- 추후 RootCAs 로드 추가 시 단일 함수 (`tlsConfigForCluster`) 만 변경.

**부정:**
- gosec G402 alert — `nolint:gosec` 주석으로 무음.
- 운영 환경에서 사용 시 *실제 cert pinning 미보장* — compliance 요구사항 (SOC2/PCI)
  통과 어려움.

## 후속 작업

- Trigger: 첫 사용자가 `Spec.TLS.Enabled=true` 활성화 + 운영 배포 보고.
- 작업: ADR-0003 → Superseded by ADR-0010 (cert-manager 통합).
- 코드 변경: `tlsConfigForCluster` 의 `InsecureSkipVerify=true` 제거 + `RootCAs` 주입.
