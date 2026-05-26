# ADR-0029: GitOps deploy 오버레이 도입 (3-repo 정합)

- Date: 2026-05-06 (revised 2026-05-06: cluster live 인벤토리 반영 — ns 통합 + storageClass)
- Status: Accepted
- Authors: @eightynine01

## Context

`keiailab/{mongodb,postgresql,valkey}-operator` 3 repo 는 모두 kubebuilder/operator-sdk 로 부트스트랩 되어 `config/{crd,rbac,manager,default,...}` kustomize 트리를 공유한다. `config/default` 는 `namespace: <op>-operator-system` 과 `namePrefix: <op>-operator-` 를 강제하며, `make deploy` 단발성 푸시에는 적합하지만 ArgoCD GitOps (git → cluster 단방향 동기) 에서는 다음과 충돌한다:

1. ArgoCD Application 의 `destination.namespace=prod` vs `config/default` 의 `namespace=<op>-operator-system` 영구 drift.
2. 자동 생성된 Namespace 리소스가 prod ns 사전생성 정책과 충돌.
3. 기존에는 `deploy/overlays/prod/` 진입점을 가져 운영자 인지 부하.

### 도출 결정

- **단일 namespace 정책**: ArgoCD Application 의 `destination.namespace` 가
  `config/default` 의 `<op>-operator-system` 과 영구 drift 한다. 본 ADR 은
  `deploy/overlays/prod/kustomization.yaml` 에서 *target namespace 명시* 로
  drift 0 보장.
- **StorageClass 외부 결정**: deploy/valkey-cluster.yaml 의
  `storageClassName` 은 *target 클러스터의 등록 SC* 에 맞춰 운영자가
  결정한다. 본 ADR 은 SC 이름을 강제하지 않는다.

## Decision

표준 구조의 GitOps 오버레이 계층을 도입한다.

```
deploy/
├── overlays/prod/
│   ├── kustomization.yaml      # config/{crd,rbac,manager} 를 prod ns 로 정렬
│   └── delete-namespace.yaml   # 자동 생성 Namespace 제거 (strategic-merge $patch: delete)
└── valkey-cluster.yaml         # ValkeyCluster CR (db ns, 별개 application)
```

- `namespace: prod` 가 모든 namespaced 리소스에 적용.
- `patches.target.name` 은 namePrefix 적용 전 raw name (`system`) — overlay 가 `config/default` 우회하여 `config/manager` 를 직접 import 하기 때문.
- CR 인스턴스 (ValkeyCluster, namespace=db) 는 별도 ArgoCD application 으로 분리하여 operator 와 workload 라이프사이클 독립.

ValkeyCluster sample 은 production 토폴로지 (sharded 3 shards × 1 replica) 를 사용하고 storageClass=ceph-block, auth.enabled=true (ADR-0013) 를 적용한다.

## Consequences

긍정:
- ArgoCD path = `deploy/overlays/prod` 고정 → drift 0.
- `make deploy` 로컬 워크플로 회귀 없음.
- 3 repo 동일 구조.
- ValkeyCluster CR 의 sharded 토폴로지가 sibling operator family 와 운영
  모델 정합 (3 shards).

부정:
- `config/manager/manager.yaml` 의 raw name 이 `system` 인 것에 의존. kubebuilder scaffold 변경 시 patch target 갱신 필요.
- TLS 는 ADR-0010 (cert-manager auto-discovery) 활성 클러스터 가정. 미설정 환경은 deploy/valkey-cluster.yaml 의 tls 블록 주석 그대로 둠.

## Alternatives Considered

1. **config/default 를 ArgoCD source 로** — namespace 강제 변경 + Namespace 자동생성 이슈. 거절.
2. **manager.yaml 의 Namespace name 을 full name 으로 수동 변경 (mongodb 방식)** — kubebuilder regenerate 호환성 저하. 거절.
3. **Helm chart (`charts/valkey-operator`) 를 GitOps source 로** — sibling
   operator family 의 umbrella chart 패턴 (operator chart 를 dependency 로
   흡수) 과 호환되며 helm 경로를 부정하지 않는다. ADR-0028
   (helm/kustomize parity invariant) 가 *두 진입점이 동일 cluster state 를
   산출* 하도록 보장하는 방향으로 후속 작업이 필요하다. 본 ADR 은
   kustomize 진입점 도입에만 한정.
