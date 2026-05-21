# deploy/ — GitOps 배포 디렉터리

ArgoCD (또는 동등 GitOps tool) 가 git → cluster 단방향 동기를 수행하기 위한
매니페스트 진입점. **`config/` 와 별개 경로** — `make deploy` 등 단발성
푸시는 `config/default` 를 사용한다 (ADR-0029).

## 구조

```
deploy/
├── overlays/prod/                 # ArgoCD application path: operator (env=prod)
│   ├── kustomization.yaml         # config/{crd,rbac,manager} → namespace 정렬
│   └── delete-namespace.yaml      # 자동 생성 Namespace 제거
├── valkey-cluster.yaml            # ArgoCD application path: workload (ValkeyCluster)
├── catalog/                       # OLM File-Based Catalog (FBC) source
└── olm/                           # OLM v1 ClusterExtension manifest
```

운영자가 *operator* 와 *workload CR* 의 라이프사이클을 분리하기 위해 각각
별개 ArgoCD Application 으로 분리한다.

## 사전 조건 (cluster)

- [ ] target namespace 사전 생성 (Application destination 과 일치)
- [ ] StorageClass — `Valkey` / `ValkeyCluster` `spec.storage.storageClassName`
      과 일치하는 SC 가 클러스터에 등록되어 있어야 한다
- [ ] cert-manager — TLS 활성화 시 (ADR-0010 cert-manager auto-discovery)
- [ ] auth Secret — operator 가 자동 생성 (ADR-0013 auth always-enabled)

## 적용 (수동 검증)

```sh
# 1) 렌더 검증
kustomize build deploy/overlays/prod | head
kustomize build deploy/overlays/prod | grep -c "kind: Namespace"   # 0

# 2) operator 적용
kustomize build deploy/overlays/prod | kubectl apply -f -
kubectl -n <ns> rollout status deploy/valkey-operator-controller-manager

# 3) workload 적용
kubectl apply -f deploy/valkey-cluster.yaml
kubectl -n <ns> get valkeycluster -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}'
```

## OLM v1 배포 (대안)

`deploy/olm/` 의 ClusterExtension manifest 를 사용하면 OLM v1
(operator-controller + catalogd) 환경에서 self-host catalog 로부터 본
operator 를 설치할 수 있다. 적용 순서:

```sh
kubectl apply -f deploy/olm/installer-rbac.yaml
kubectl apply -f deploy/olm/clusterextension.yaml
```

## 변경 절차

본 디렉터리 변경은 ADR (`docs/kb/adr/`) 작성 후 진행. 매번
`kustomize build deploy/overlays/prod` 렌더 검증.
ADR-0028 (Helm/Kustomize parity invariant) 에 따라 `charts/` 와 동기화
검증 의무.
