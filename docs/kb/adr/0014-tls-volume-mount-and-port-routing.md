# ADR-0014: TLS Secret 마운트 + operator 가 TLS port (6380) 로 접속 + ServerName FQDN + mTLS client cert

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil

## Context

Spec.TLS.Enabled=true 시 두 결함 발견:

1. **STS 빌더 가 TLS Secret 을 마운트하지 않음**: `internal/resources/statefulset.go::BuildStatefulSet`
   이 `data` + `config` 만 마운트. configmap 템플릿 (`valkey.conf.tmpl`) 은
   `tls-cert-file /tls/tls.crt` 를 렌더하지만 `/tls` 에 아무 것도 없어
   `Failed to load certificate: /tls/tls.crt: error:80000002:system library::No
   such file or directory` 로 컨테이너 즉시 종료 → CrashLoopBackOff.

2. **operator 가 plain port 6379 로 TLS 연결 시도**: `dialPod` + `tlsConfigForCluster`
   가 함께 작용하면 redis client 가 `UseTLS=true` + `Address=hostname:6379`.
   Valkey 는 plain `port 6379` 와 `tls-port 6380` 을 분리 listen — operator 가
   plain port 에 TLS handshake → 서버가 즉시 close → CLUSTER MEET timeout.

## Decision

1. **STS 빌더 에 TLSSecretName 필드 추가** + `tlsVolumeMounts` /
   `tlsVolumes` 헬퍼. ValkeyCluster controller 가 다음 우선순위로 secret 이름 결정:
   - `Spec.TLS.CustomCert.SecretName` (사용자 제공 우선)
   - `resources.CertificateSecretName(vc.Name)` (cert-manager 발급)
   - 둘 다 비어 있으면 마운트 없음 (TLS 비활성).

2. **`podAddresses` 가 TLS 활성 시 PortTLS (6380) 반환**: operator 의 모든
   in-cluster 통신 (CLUSTER MEET / NODES / ADDSLOTS / REPLICATE / FLUSHSLOTS) 이
   tls-port 사용. 평문 6379 는 *내부 사이드카 (metrics-exporter)* 가 그대로 사용.

## Consequences

긍정:
- TLS 활성 시 cert 가 즉시 마운트되어 컨테이너 시작.
- operator → cluster 모든 control-plane 트래픽이 자동 암호화.
- ValkeyClusterValidator 의 TLS 조합 검증과 일관성 유지 (CustomCert XOR CertManager).

부정:
- *standalone Valkey controller* 는 본 변경에 포함 안 됨 (별도 PR 필요).
  ValkeyController 는 `BuildCertificate` 호출 자체가 없음. 추후 `BuildCertificateForValkey`
  + `Valkey.Spec.TLS` reconcile 통합 필요.
- 6379 와 6380 동시 listen → 사용자가 plain port 로 외부 접속 가능 ("TLS 강제"
  아님). NetworkPolicy 로 ingress 제한 권장.

## Alternatives Considered

1. **`port 0` + `tls-port 6379` 로 plain 비활성화**: cleaner 하지만 cluster bus
   port (16379) 와의 매핑·기존 호환성 깨짐. 거절.
2. **operator 가 TLS 비활성 → plain 으로 control-plane 통신**: 보안 후퇴. 거절.

## 추가 결함 (iter 7 발견)

위 두 결함 수정 후 두 차례 더 발견:

3. **ServerName 이 short name 사용**: `tlsConfigForCluster` 가 `ServerName:
   resources.HeadlessServiceName(vc.Name)` (`vc-tls-headless`) 만 설정 → cert
   SAN (`vc-tls-headless.default.svc`, `*.vc-tls-headless.default.svc`) 과
   불일치 → x509 verification 실패. *fix*: `<headless>.<ns>.svc` 로 FQDN 설정.

4. **tls-auth-clients yes 가 mTLS 강제**: configmap 템플릿 의 기본값.
   operator 가 RootCAs 만 설정하면 TLS handshake 시 server 가
   `tls: certificate required` 응답. *fix*: `loadClientCert` 추가 — Secret 의
   tls.crt + tls.key 를 `tls.X509KeyPair` 로 로드 후 `cfg.Certificates` 에 설정.

## 검증 (iter 7 완료)

```
NAME     SHARDS   PHASE     CLUSTER   SLOTS   VERSION
vc-tls   3        Running   ok        16384   8.1.6
```

- mTLS handshake 성공 (operator → 모든 6 nodes)
- CLUSTER MEET / ADDSLOTS / REPLICATE 모두 TLS 통과
- 데이터 plane SET/GET (cluster mode -c, 다중 shard) 성공

## Action Items

- [x] AI-001: STS 빌더 + cluster controller 수정 (volume mount)
- [x] AI-002: 단위 테스트 통과 (`internal/{controller,resources,valkey,webhook}`)
- [x] AI-003: ServerName FQDN + mTLS client cert
- [x] AI-004: e2e 검증 (TLS cluster Running + slots 16384)
- [ ] AI-005: ValkeyController 의 TLS 통합 (별도 PR — `BuildCertificateForValkey` 추가)
- [ ] AI-006: standalone Valkey TLS 동작 가능하도록 후속 패치
- [ ] AI-007: `test/e2e/tls_test.go` 자동화 (현재는 수동 검증)

Refs: ADR-0008, ADR-0010
