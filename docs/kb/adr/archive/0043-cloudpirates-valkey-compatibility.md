# ADR-0043: CloudPirates valkey 0.20.2 호환 정책

- Date: 2026-05-12
- Status: Deprecated (2026-05-21 — 추적 문서 archive)
- Authors: @eightynine01

## Context

사용자 요청으로 `keiailab/valkey-operator` 와 ArtifactHub
`cloudpirates-valkey/valkey` chart 를 교차 검증했다. 확인 대상 chart 는
CloudPirates valkey `0.20.2` / appVersion `9.0.0`.

CloudPirates chart 는 단일 StatefulSet 중심 Helm chart 이며, Valkey data-plane
세부 knob 를 values 로 직접 노출한다. 본 operator 는 CRD + controller 가
data-plane lifecycle 을 소유한다. 따라서 호환 목표는 values 이름을 그대로
복제하는 것이 아니라, 운영에서 안정적으로 의미가 같은 기능을 CRD/control-plane
계약으로 제공하는 것이다.

## Decision

CloudPirates chart 의 운영 유효 기능을 다음 방식으로 수용한다.

1. CRD data-plane knob 확장:
   - `spec.version.imageRef`: digest 포함 전체 image reference.
   - `spec.storage.ephemeral`, `existingClaim`, `accessModes`, `annotations`,
     `labels`.
   - `spec.service.type`, `annotations`, `labels`, `ipFamilyPolicy`,
     `ipFamilies`.
   - `spec.pod.labels`, `annotations`, `imagePullSecrets`, `hostAliases`,
     `extraEnv`, `livenessProbe`, `readinessProbe`, `startupProbe`,
     `terminationGracePeriodSeconds`.
   - `spec.externalReplica`: 외부 Redis/Valkey primary 단방향 복제. v1alpha1 은
     `mode=Standalone` 에 한정한다.
   - `spec.revisionHistoryLimit`: StatefulSet rollout history.

2. Controller 반영:
   - Service / StatefulSet / ConfigMap builder 에 위 필드를 모두 반영한다.
   - `externalReplica.enabled=true` 인 Valkey 는 내부 failover/replication
     reconcile 과 충돌하지 않도록 operator failover 경로를 건너뛴다.
   - `storage.ephemeral` 또는 `storage.existingClaim` 은 PVC template 및 PVC
     resize ownership 에서 제외한다.

3. Helm chart escape hatch:
   - `charts/valkey-operator.values.extraObjects` 를 추가해 CloudPirates
     `extraObjects` 와 같은 release 단위 보조 manifest 배포를 지원한다.

4. 안전한 비채택 경계:
   - Sentinel sidecar 는 직접 채택하지 않는다. 동일 HA 목적은
     `mode=Replication` + operator AutoFailover 로 제공한다. Sentinel client
     사용자는 `docs/operations/sentinel-migration.md` 절차로 Service-aware
     client 로 전환한다.
   - HTTP Ingress / Gateway `HTTPRoute` 는 Valkey TCP wire protocol 에 대해
     일반적으로 안정적인 노출 방식이 아니므로 CRD 1급 필드로 채택하지 않는다.
     운영 노출은 `spec.service.type=LoadBalancer|NodePort` 와 cloud provider
     annotation 을 사용한다. Gateway API 가 필요한 환경은 `extraObjects` 로
     `TCPRoute` 같은 L4 resource 를 GitOps 추적한다.
   - 사용자 전체 config file 교체는 operator 가 생성하는 TLS/Auth/replication
     config 를 깨뜨릴 수 있어 직접 채택하지 않는다. 추가 지시문은
     `spec.additionalConfig` 와 `spec.persistence` 로 병합한다.

## Consequences

**긍정:**
- CloudPirates chart 의 digest 고정, PVC/emptyDir, 기존 PVC, Service type,
  dual-stack, pod metadata/probe/env, 외부 replica 같은 운영 knob 를 operator
  data-plane 에서 사용할 수 있다.
- Sentinel / HTTPRoute 처럼 dual control-plane 또는 L7/L4 mismatch 를 만드는
  표면은 문서화된 안정 대체 경로로 제한된다.
- chart 사용자는 `extraObjects` 로 cluster-local policy / TCPRoute / CSI
  companion manifest 를 같은 release 에 포함할 수 있다.

**부정:**
- CloudPirates values 와 1:1 이름 호환은 제공하지 않는다. Helm values 를
  Valkey CR 로 변환하는 사용자는 매핑 문서를 따라야 한다.
- external replica 는 v1alpha1 에서 Standalone 한정이다. 내부 HA 는 operator
  Replication/Cluster 로 다룬다.

## Alternatives Considered

1. **CloudPirates chart 를 그대로 subchart 로 포함**: 두 control-plane 이 같은
   StatefulSet 을 소유하게 되어 upgrade/failover/storage ownership 이 충돌한다.
   거절.

2. **Sentinel sidecar 직접 추가**: operator AutoFailover 와 Sentinel quorum 이
   동시에 primary 를 바꾸는 dual failover control-plane 이 된다. ADR-0017 과
   sentinel migration runbook 에 맞춰 거절.

3. **HTTPRoute/Ingress 1급 필드 추가**: Valkey 는 HTTP backend 가 아니므로
   ingress controller 별 TCP annotation 에 의존한다. provider-neutral 안정
   API 로 보기 어려워 거절. L4 Gateway API 는 `extraObjects` 로 허용한다.

## Status

Deprecated (2026-05-21). 본 ADR 이 도출한 CRD 확장 (digest imageRef,
storage knobs, service ipFamilies, pod metadata/probes/env,
externalReplica, revisionHistoryLimit, chart extraObjects) 은 *모두 GA*
되어 현재 CRD 정의에 포함되어 있다. 외부 chart 호환 매핑 문서는 사용자
대면 콘텐츠에서 외부 서비스 참조를 정리하는 과정에서 *제거* 되었다.
의사결정 자체는 history 로 본 ADR 에 보존된다.
