# ADR-0008: TLS RootCAs loaded from Spec.TLS.CustomCert.SecretName

- Date: 2026-05-05
- Status: Accepted (partial — ADR-0003 부분 supersede)
- Authors: @phil

## Context

ADR-0003 은 TLS 활성화 시 `InsecureSkipVerify=true` 임시 사용. iter 4 에서 사용자가
`Spec.TLS.CustomCert.SecretName` 으로 CA bundle 을 명시할 수 있는 *부분 구현* 을 추가.

## Decision

다음 우선순위로 RootCAs 구성 (operator pod 가 valkey 노드 접속 시):

1. `Spec.TLS.CustomCert.SecretName` 의 `ca.crt` 키 → `x509.NewCertPool().AppendCertsFromPEM`.
2. `Spec.TLS.CertManager` Issuer 가 만든 Secret — 본 ADR 은 *직접 추적 안 함*.
   사용자가 cert-manager Issuer Secret 이름을 CustomCert 로 명시하거나 별도 ADR 후속.
3. 둘 다 미제공 → InsecureSkipVerify fallback (warning log).

CA Secret 은 *operator 가 직접 읽기* — 별도 마운트 없이 K8s API client 로 동적 조회.
RBAC: `secrets get;list;watch` 이미 보유.

## Consequences

**긍정:**
- 사용자 제공 CA 로 정상 cert pinning 가능 — SOC2/PCI 감사 통과 가능 (CustomCert 사용 시).
- `loadCABundle` 단위테스트 (3건: missing, valid PEM, invalid PEM) + e2e
  `TestTLSConfigForCluster_customCertLoaded` 검증.
- Issuer 변경 시 다음 reconcile 에서 자동 반영 (cache miss → 재조회).

**부정:**
- cert-manager 직접 통합 미완 — 사용자가 Issuer 가 만든 Secret 이름을 *수동* 으로
  CustomCert 로 명시해야 함. 자동화는 ADR-0010 (예정).
- CA bundle 캐시 없음 — 매 dial 마다 Secret API 호출. operator 부하 증가 가능 (M3 에서
  manager-level cache + secret event watcher 추가).

## 후속 작업

- ADR-0010: cert-manager Issuer status 추적 → CA Secret 자동 발견.
- M3 Performance: CA bundle in-memory cache + invalidation on Secret event.

## Alternatives Considered

- **operator pod 에 CA Secret 마운트:** Helm chart 수정 + Pod restart 필요. 동적성 손실.
- **InsecureSkipVerify 영구 유지:** SOC2 통과 불가, ADR-0003 의 임시 결정 영구화.
