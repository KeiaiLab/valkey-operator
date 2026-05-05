# ADR-0011: Required 필드는 mutating webhook 에서 직접 default 채움

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil

## Context

`Spec.Version`, `ValkeyClusterSpec.Shards`, `ValkeyClusterSpec.ReplicasPerShard`
등 *required* 필드 (JSON tag 에 `omitempty` 없음) 에 `+kubebuilder:default=...` 만
부착했을 때 다음과 같은 무한 reconcile 루프 발생:

1. 사용자 가 `spec: {}` 또는 일부 필드만 명시한 CR 를 생성.
2. mutating webhook 이 호출되어 `*v1alpha1.Valkey` Go struct 로 unmarshal — 누락
   필드는 zero value (`""`, `0`).
3. webhook 이 그대로 반환하면 controller-runtime 이 다시 marshal — `omitempty`
   부재로 모든 필드가 명시적으로 직렬화되어 admission chain 의 다음 단계 로
   `version: ""` 또는 `shards: 0` 이 흘러간다.
4. CRD schema 의 `default: 3` / `default: "8.1.6"` 는 *unstructured map 에 키 가
   없을 때만* 적용된다. 명시적 zero value 가 있으면 skip.
5. 이후 validation 또는 reconciler 가 필드를 사용하다 실패 → reconcile 루프.

실측: ValkeyCluster sample (`spec: {}`) 적용 시 `spec.shards: Invalid value: 0`
admission rejection. Valkey sample 적용 시 reconciler 가 status update 시
`spec.version.version: Invalid value: ""` 정규식 검증 실패로 5초 단위 재시도.

## Decision

`omitempty` 가 없는 *required* 필드 (즉, 항상 직렬화되는 필드) 의 default 는
mutating webhook 의 `Default()` 함수에서 직접 채운다. CRD marker 의
`+kubebuilder:default=...` 단독으로는 *불충분*함을 명시한다.

구체 적용:

| 필드 | 채우는 곳 |
|---|---|
| `Valkey.Spec.Version.Version` | `ValkeyCustomDefaulter.Default` → `cachev1alpha1.DefaultValkeyVersion` |
| `Valkey.Spec.Version.Image` | 같음 |
| `ValkeyCluster.Spec.Shards` | `ValkeyClusterCustomDefaulter.Default` → `3` |
| `ValkeyCluster.Spec.ReplicasPerShard` | 같음 → `1` |
| `ValkeyCluster.Spec.Version.*` | 같음 |

상수는 `api/v1alpha1/common_types.go` 의 `DefaultValkeyVersion` /
`DefaultValkeyImage` 로 export — CRD marker 와 같은 값으로 동기화 유지.

## Consequences

긍정:
- 사용자 가 *최소한의* spec (e.g. 빈 `spec: {}`) 만으로 동작 가능 — 샘플
  CR 가 그대로 admit 되며 reconcile 루프 진입 없음.
- defaulting 로직이 *한 곳* (webhook) 에 집중 — CRD marker 와 webhook 이
  분산 시 발생하던 동기화 어긋남 제거.

부정 / 트레이드오프:
- CRD marker default 와 webhook default 가 *두 군데* 정의됨 — 향후 변경 시 둘
  다 수정 필요. mitigation: 상수 (`DefaultValkeyVersion`) export 후 양쪽 참조.
- mutating webhook 이 CRD schema validation 보다 무거움 (TLS, 서비스 호출).
  required 필드 가 늘어날수록 webhook 의 mutate 책임 증가. mitigation: required
  필드는 *진짜로 항상 필요한 것* 만 유지 — 가능하면 `omitempty + default` 조합
  으로 처리.

## Alternatives Considered

1. **모든 필드에 `omitempty` 부착**: 검토. 그러나 `Version` 같은 필드는 의미상
   required (없으면 Valkey 컨테이너 이미지 결정 불가) — `omitempty` 는 zero value
   인 빈 문자열 허용 신호로 잘못 해석될 수 있음. 거절.
2. **CRD schema 에 `nullable: false` + `default`**: kubebuilder marker 가 자동
   처리. 그러나 위에 설명된 mutating webhook 직렬화 문제 가 근본 원인이라 schema
   레벨로 해결 불가. 거절.
3. **mutating webhook 제거**: ValkeyCluster 의 SlotMigration default 등 조합
   derivable default 가 있어 webhook 자체 는 필요. 거절.

## Action Items

- [x] AI-001: `Valkey` defaulter 에 Version 필드 채움
- [x] AI-002: `ValkeyCluster` defaulter 에 Shards/ReplicasPerShard/Version 채움
- [x] AI-003: 공통 상수 `DefaultValkeyVersion` / `DefaultValkeyImage` export
- [ ] AI-004: webhook 단위 테스트 에 zero-value 입력 → default 채워짐 케이스 추가
- [ ] AI-005: README 또는 user-facing docs 에 "minimum spec" 예시 추가

Refs: ADR-0009 (webhook validation+defaulting 도입)
