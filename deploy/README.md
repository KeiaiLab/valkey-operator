# deploy/ — GitOps 배포 디렉터리

본 디렉터리는 ArgoCD (또는 동등 GitOps tool) 가 git → cluster 단방향 동기를 수행하기 위한 매니페스트 진입점이다. **`config/` 와 별개 경로** — `make deploy` 등 단발성 푸시는 `config/default` 를 사용한다.

ADR-0029 의 결정에 따라 mongodb-operator / postgresql-operator 와 동일 구조로 정합화되었다.

## 구조

```
deploy/
├── overlays/prod/                 # ArgoCD application path: operator 자체
│   ├── kustomization.yaml         # config/{crd,rbac,manager} → namespace=prod
│   └── delete-namespace.yaml      # 자동 생성 Namespace 제거
└── valkey-cluster.yaml            # ArgoCD application path: workload (ValkeyCluster, db ns)
```

운영 모델: **operator 와 workload 는 별개 ArgoCD application** — operator 는 prod ns, 데이터는 db ns.

## 사전 조건 (cluster)

- [ ] `prod` namespace 사전 생성.
- [ ] `db` namespace 사전 생성.
- [ ] StorageClass `ceph-block` 이용 가능.
- [ ] cert-manager (TLS 활성화 시 — ADR-0010 cert-manager auto-discovery 가정. 본 sample 은 tls 블록 주석 처리).
- [ ] auth Secret 은 operator 가 자동 생성 (ADR-0013 auth always-enabled).

## 적용 (수동 검증)

```fish
# 1) 렌더 검증
kustomize build deploy/overlays/prod | head
kustomize build deploy/overlays/prod | grep -c "kind: Namespace"   # 0

# 2) operator 적용
kustomize build deploy/overlays/prod | kubectl apply -f -
kubectl -n prod rollout status deploy/valkey-operator-controller-manager

# 3) workload 적용
kubectl apply -f deploy/valkey-cluster.yaml
kubectl -n db get valkeycluster valkey-cluster \
    -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

## 변경 절차

본 디렉터리 변경은 ADR 작성 후 진행 (`docs/kb/adr/`). 매번 `kustomize build deploy/overlays/prod` 렌더 검증. ADR-0028 (Helm/Kustomize parity invariant) 에 따라 charts/ 와도 동기화 검증.
