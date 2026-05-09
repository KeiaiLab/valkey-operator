# ADR-0032: Valkey Custom Modules — init container mount + 공식 preset only

- Date: 2026-05-09
- Status: Accepted (PR-C6.1 — type module, controller 분기는 PR-C6.2 후속)
- Authors: @eightynine01
- Refs: Plan §2 D9 (`~/.claude/plans/1-https-artifacthub-io-packages-helm-clo-synthetic-gem.md`), ADR-0036 (PSS Restricted)

## Context

ArtifactHub 비교 (Plan §1 Phase 1) 결과 Bitnami redis v25.5.2 가
*Redis Stack* 의 RediSearch / RedisBloom / RedisJSON / RedisTimeSeries
module 을 chart 에서 활성 — 사용자 시나리오 (전문 검색, vector embedding,
JSON document 등) 차별점.

라이선스 호환성:
- Bitnami Redis Stack module: **RSALv2 / SSPL** — Valkey 의 BSD-3 license
  와 *영구 호환 불가* (Linux Foundation Valkey 의 *vanilla BSD only* 정책).
- Valkey 공식 module: BSD — `valkey-search` (8.1+, GA), `valkey-json`,
  `valkey-bloom` 진행 중.

본 ADR 은 *Valkey 공식 module preset only* + *bring-your-own custom
module* (사용자 책임) 두 path 보존.

## Decision

1. **`api/v1alpha2/valkey_types.go`** 에 `ModuleSpec` type + `ValkeySpec.Modules
   []ModuleSpec` 필드 신규.

2. **두 모드**:
   - `Name` 만 지정: Valkey 공식 module preset. operator 가 *allow-list*
     검증 (예: "valkey-search", "valkey-json", "valkey-bloom") + 공식
     image 자동 resolve.
   - `Image` 명시: bring-your-own custom module. init container 가 해당
     image 의 `/modules/<name>.so` 를 emptyDir 로 mount, valkey
     container 가 `--loadmodule /modules/<name>.so <args>` 로 적재.

3. **PSS Restricted 정합** (ADR-0036): module image 가 privileged syscall
   요구 시 webhook 거부. Sonatype Trust Score 검증 권장 (custom module 의
   supply chain 보증).

4. **Bitnami Redis Stack module *영구 미지원***: RSALv2/SSPL 라이선스
   호환 불가. 사용자가 RediSearch 필요 시 *external Redis Stack
   인스턴스* 사용 권장 (별도 운영).

5. **PR 분할**:
   - **PR-C6.1** (본 PR): v1alpha2 type module + ADR.
   - **PR-C6.2** (후속): controller 의 statefulset.go 분기 (init container
     mount + valkey-cli MODULE LIST 검증) + e2e (valkey-search FT.SEARCH).
     PR-A2.2 (v1alpha2 hub 전환) 의존.

## Consequences

### Positive

- Valkey 8.1+ 의 native module (search/json/bloom) 활용 path — Bitnami
  Redis Stack 사용자 *부분 마이그레이션* 가능 (search 만 이전).
- 라이선스 호환성 명시 — Linux Foundation Valkey 정책 + 사용자 운영
  매뉴얼 정합.
- bring-your-own custom module path — 사용자 self-managed module 지원.

### Negative

- module 도입 → image attack surface 증가. PSS Restricted webhook +
  Sonatype 검증으로 완화하나 *사용자 책임* 영역 명시 의무.
- v1alpha2 ValkeySpec 표면 +1 type. doc + chart values 갱신 의무 (PR-C6.2).

### Trade-offs

- *Valkey 공식 only* (본 ADR) vs *Bitnami Redis Stack 호환 layer* — 후자는
  라이선스 영구 호환 불가. 본 ADR 의 *공식 only* 가 정합.

## Alternatives Considered

1. **Module 미지원** — 거부.
   - Plan §1 Phase 1 Gap D 미해소. Bitnami / Cloudpirates 와의 차별점 부재.

2. **Bitnami Redis Stack module 직접 호환 layer** — 거부.
   - RSALv2/SSPL 라이선스 — Valkey BSD-3 와 *영구 호환 불가*.

3. **module 을 별도 CRD (ValkeyModule)** — 거부.
   - module 이 *Valkey instance 의 일부* — Spec 에 통합이 정합.

## Refs

- Plan §2 D9 (Sprint C PR-C6).
- ADR-0036 (PSS Restricted Optional) — module image 보안 정합.
- 외부 비교: Bitnami v25.5.2 의 Redis Stack module (RSALv2/SSPL — 호환 불가).
- Valkey module ecosystem: <https://valkey.io/topics/modules.html>
- 후속 PR-C6.2: controller statefulset.go init container mount + e2e.
