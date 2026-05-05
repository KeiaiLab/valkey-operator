# ADR-0006: ScalePolicy.Deliberate=false 기본값

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil

## Context

`ValkeyCluster.Spec.{Shards, ReplicasPerShard}` 변경은 cluster topology 변경.

영향:
1. **shard 추가/제거** → 16384 slot 재분배 (수십 분 ~ 수 시간).
2. 재분배 중 일부 키가 `MOVED` 응답 → 클라이언트 retry 필수.
3. **replicasPerShard 변경** → STS replicas 갑작스러운 증감 → kubelet 의 pod 동시
   생성/삭제 → 일시적 cluster size 불일치.

따라서 *사용자 명시 동의 없이는 즉시 적용하지 않는다* 가 안전.

## Decision

`Spec.ScalePolicy.Deliberate=false` 가 기본값 (=`Spec.ScalePolicy` 미지정 시).
변경 의도가 감지되면 `Status.PendingScale` 에 기록하고 STS replicas 는 *보존*.

사용자가 `Spec.ScalePolicy.Deliberate=true` 로 명시 동의 시 다음 reconcile 에서 적용.

## Consequences

**긍정:**
- *우발적 scale 사고 방지* — 사용자가 spec 일부를 잘못 수정해도 즉시 적용되지 않음.
- `Status.PendingScale.Reason` 메시지로 *왜 적용 안 되는지* 사용자에게 명확히 전달.

**부정:**
- 일반 K8s 워크로드 (Deployment) 와 다른 동작 — 사용자 학습 비용.
- GitOps (ArgoCD/Flux) 와 결합 시 두 단계 PR (Spec 수정 + Deliberate flag) 필요.

## 후속 작업

- M3 (Beta) 에서 `Status.PendingScale` 가 set 되어 있을 때 Kubernetes Event +
  warning condition 발행 → kubectl describe 로 가시화.
- M4 에서 ValidatingWebhook 도입 시: `Spec.ScalePolicy.Deliberate=false` 면서 동시에
  Spec.Shards 변경하는 PUT 을 webhook 에서 reject 하는 옵션 검토.

## Alternatives Considered

- **Deliberate 기본 true (즉시 적용):** 일반 K8s 패턴과 일치. 그러나 Valkey cluster
  의 중량성 (slot 재분배) 고려 시 너무 위험.
- **3 단계 confirmation (Spec.Shards + Deliberate=true + Status.PendingScale 자동
  clear 대기):** 너무 복잡, GitOps 와 친화성 떨어짐.
