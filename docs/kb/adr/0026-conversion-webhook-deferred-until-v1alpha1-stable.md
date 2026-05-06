# ADR-0026: Conversion Webhook — v1alpha1 Stable 도달 후 v1beta1 도입

- Date: 2026-05-06
- Status: Accepted
- Authors: @phil

## Context

Plan §3 Track F 의 *Conversion webhook (v1alpha1 → v1beta1 준비)*. 현재
모든 5 CRD (Valkey, ValkeyCluster, ValkeyBackup, ValkeyRestore,
ValkeyBackupTarget) 가 v1alpha1 단계 — *하위 호환성 보장 없음*.

설계 분기:

1. **즉시 v1beta1 도입 (cycle 16+)** — api/v1beta1/ 패키지 + Hub 지정 +
   v1alpha1 의 ConvertTo/ConvertFrom + webhook 등록. 사용자가 v1alpha1/v1beta1
   양쪽 사용 가능.

2. **v1alpha1 stable 도달 후** — 현재 v1alpha1 의 *stabilization* 우선.
   API 변경이 자주 발생할 시점에 conversion 도입은 *premature integration*.

추가 분기 (옵션 1 채택 시):
- *자동 conversion (controller-runtime built-in spoke/hub)* — v1beta1 이
  hub (storage version), v1alpha1 이 spoke. controller-runtime 의 자동
  webhook + 우리 ConvertTo/ConvertFrom 메서드 작성.
- *수기 conversion API* — 자체 컨버전 logic. 복잡도 큼.

## Decision

**옵션 2 채택 — v1alpha1 stable 도달 후 v1beta1 도입**.

근거:
- v1alpha1 의 *15 cycles 진행* 동안 API 가 *상당히 안정* 했지만, Track A
  완성 후도 Spec/Status 보강 가능성 존재 (HPA, Conversion 등).
- *사용자 base 부재* — 현재 alpha 단계라 *호환성 보장* 사용자 적음. 해당
  사용자도 v1alpha1 → v1beta1 직접 마이그레이션 가능.
- *premature integration* 비용: ConvertTo/ConvertFrom 매 변경마다 양 버전
  동기화 부담.

**v1beta1 도입 trigger 조건** (다음 cycle 진입 시점):
- 첫 *external 사용자* 가 production 운영 시작
- 외부 공개 API 안정화 commit (e.g. `release(v0.1.0)` → `release(v1.0.0)`
  마이그레이션 시점)
- HPA / 추가 CRD field / 의미 변경 등 *breaking change* 발생 시점

위 조건 도달 시 *옵션 1* 의 *자동 conversion (spoke/hub)* 방식 채택.

## Consequences

긍정:
- **현재 단계의 maintenance 비용 0** — v1beta1 도입 비용 deferred.
- **v1alpha1 의 자유 변경** — 호환성 보장 부담 없이 빠른 iteration.
- **사용자 진입 장벽 낮음** — 단일 버전만 학습.

부정:
- **첫 외부 사용자 마이그레이션 부담** — v1alpha1 → v1beta1 수동 (kubectl
  edit / re-apply). 사용자 base 작을 때만 OK.
- **장기 안정 표시 부재** — *production-ready* signal 이 약함. README + ADR
  의 *Day-N₁* / *Day-N₂* 게이트 명시 로 보완.

## Alternatives Considered

1. **즉시 v1beta1 도입** (옵션 1 거절):
   - cycle 16+ 의 부담: api/v1beta1/ 5 CRD + Hub 마커 + ConvertTo/From 5
     × 2 = 10 메서드 + webhook 등록 + 단위 테스트 + e2e.
   - 매 v1alpha1 변경마다 양 버전 동기화 — *2x 변경 비용*.
   - 거절: premature integration. 첫 외부 사용자 부재 시점에 비용 대비
     가치 낮음.

2. **수기 conversion API** (자동 도입 시 거절):
   - controller-runtime 의 spoke/hub 표준 비활용 — 비표준.
   - 거절.

3. **v1alpha2 도입 후 v1beta1** (3-단계):
   - alpha 두 번 거치는 패턴. 일반적이지만 본 프로젝트 단순화 위해 거절.

## Action Items

본 ADR 은 *deferred* — Action Items 는 *trigger 조건* 도달 시 작성:

- [ ] Trigger 도달 후 AI-001: api/v1beta1/ 패키지 scaffold (groupversion_info.go,
      각 CRD type)
- [ ] Trigger 도달 후 AI-002: Hub() 마커 (v1beta1)
- [ ] Trigger 도달 후 AI-003: v1alpha1 의 ConvertTo/ConvertFrom (5 CRD x 2)
- [ ] Trigger 도달 후 AI-004: webhook handler 등록 (controller-runtime)
- [ ] Trigger 도달 후 AI-005: 단위 테스트 (round-trip conversion)
- [ ] Trigger 도달 후 AI-006: e2e — v1alpha1 → v1beta1 read/write 검증
- [ ] Trigger 도달 후 AI-007: README — 마이그레이션 가이드

## 관련 결정

- **Plan §3 Track F**: Conversion webhook 항목 *deferred* 표시
- **HANDOFF.md**: 다음 cycle 진입 권고 갱신 — Conversion webhook 제거,
  HPA / e2e 실측 우선
- v1.0.0 릴리스 시점에 본 ADR 재검토

Refs: Plan §3 Track F, HANDOFF.md cycle 15 §6, kubebuilder
[Conversion webhook docs](https://book.kubebuilder.io/multiversion-tutorial/conversion-webhook).
