# ADR-0031: Password Rotation reflect path (AuthSpec.RotationPolicy enum)

- Date: 2026-05-09
- Status: Accepted (PR-B7.1 — type module, controller 분기는 PR-B7.2 후속)
- Authors: @eightynine01
- Refs: Plan §2 D6 (`~/.claude/plans/1-https-artifacthub-io-packages-helm-clo-synthetic-gem.md`), ADR-0034 (Auth Optional v1alpha2)

## Context

ArtifactHub 비교 분석 (Plan §1 Phase 1) 결과 *3 차트 모두 password rotation
미지원* (Bitnami / Cloudpirates / valkey-operator). 사용자 시나리오:
- 외부 Secret manager (ESO + OpenBao / SealedSecrets) 가 주기적으로 Secret
  rotate.
- Secret 변경 시 *operator 가 무중단 valkey CONFIG SET requirepass 발행*
  필요. 미반영 시 client 가 *기존 password 로 fail*.

운영 규약 (Plan §2 D6 + valkey ROADMAP Non-Goals): *operator 자체는 회전
수행 안 함* (외부 ESO/OpenBao 위임). 단 *외부 회전 반영* path 는 operator
책임.

## Decision

1. **`api/v1alpha2/common_types.go` 의 `AuthSpec`** 에 `RotationPolicy
   string` 필드 신규. enum: `["Manual", "OnSecretChange"]`, default
   `"Manual"`.

2. **동작**:
   - `Manual` (default): 사용자가 외부에서 PasswordSecretRef Secret 변경
     시 operator 무영향. 회전은 사용자 책임 (운영 매뉴얼).
   - `OnSecretChange`: Secret resourceVersion 변경 감지 시 operator 가
     자동 valkey `CONFIG SET requirepass <new>` 발행. replication mode 에
     서는 *replica 먼저 reauth* + *primary 마지막* (race 방지).

3. **PR 분할**:
   - **PR-B7.1** (본 ADR): v1alpha2 type module 에 enum 필드 추가. controller
     미수정.
   - **PR-B7.2** (별도): controller 의 reconcile loop 에 *Secret
     resourceVersion watch* + `rotatePassword` helper. PR-A2.2 (v1alpha2
     hub 전환) 의존.

4. **enum string 사용 vs typed alias**: Go const 로 string 표현 — Kubernetes
   API convention 정합 (kubebuilder Enum validation 지원).

## Consequences

### Positive

- 외부 Secret rotation 시나리오 (ESO + OpenBao) 지원 — Plan §1 Phase 1
  Gap B 해소.
- 운영 규약 (operator 회전 수행 X) 보존 — *반영* 만 추가.
- default Manual — 기존 사용자 무중단.

### Negative

- v1alpha2 AuthSpec 표면 +1 enum. doc + chart values 갱신 의무 (PR-B7.2).
- OnSecretChange 모드의 *race 위험* (replica reauth 도중 primary 변경) —
  PR-B7.2 controller 가 *순서 강제* (replica → primary).

### Trade-offs

- *enum string* (본 ADR) vs *bool toggle* (`AutoReflect *bool`) — 후자는
  default false 시 *manual reauth*. 본 ADR 의 enum 이 *명시 의미*.

## Alternatives Considered

1. **operator 가 회전 *수행*** — 거부.
   - Plan §2 D6 + ROADMAP Non-Goals: *외부 ESO/OpenBao 위임* 결정 보존.
   - operator 가 *random password* 생성 + Secret update 시 *외부 ESO 와 충돌*.

2. **AuthSpec.AutoReflect *bool** — 거부.
   - 향후 추가 mode (예: `OnAnnotationTrigger`) 도입 시 enum 이 유연.

3. **별 type AuthRotationSpec** — 거부.
   - 단일 enum 만으로 충분 — 별 type 은 over-engineering.

## Refs

- Plan §2 D6 (Sprint B PR-B7).
- ADR-0034 (Auth Optional v1alpha2): AuthSpec 진화 부모.
- valkey ROADMAP Non-Goals: operator 자체 시크릿 회전 수행 안 함.
- 외부 패턴: ESO (External Secrets Operator) + OpenBao Secret rotation.
- 후속 PR-B7.2: controller reconcile + rotatePassword helper.
