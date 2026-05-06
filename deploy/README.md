# deploy/ — GitOps 배포 디렉터리

본 디렉터리는 ArgoCD (또는 동등 GitOps tool) 가 git → cluster 단방향 동기를 수행하기 위한 매니페스트 진입점이다. **`config/` 와 별개 경로** — `make deploy` 등 단발성 푸시는 `config/default` 를 사용한다.

ADR-0029 의 결정에 따라 mongodb-operator / postgresql-operator 와 동일 구조로 정합화되었다.

## 구조

```
deploy/
├── overlays/prod/                 # ArgoCD application path: operator 자체 (envName=prod, ns=data)
│   ├── kustomization.yaml         # config/{crd,rbac,manager} → namespace=data
│   └── delete-namespace.yaml      # 자동 생성 Namespace 제거
└── valkey-cluster.yaml            # ArgoCD application path: workload (ValkeyCluster, ns=data)
```

운영 모델: argos 클러스터 ns 통합 정책 (2026-05-06 cycle: 5 차트 모두 `data` ns 단일) 에 따라 operator 와 CR 이 *동일 data ns* 를 공유한다.

## 현 운영 상태 (2026-05-06)

`keiailab/argos-platform-data/valkey` (ApplicationSet path) 는 **bitnami/valkey 5.6.1** (replication 1+1) 로 운영 중. **keiailab/valkey-operator 는 클러스터 미배포 상태**.

본 디렉터리는 *bitnami → keiailab* 마이그레이션의 **Day-0 GitOps 진입점 후보** 이다. 마이그레이션은 별도 plan (`docs/migration/bitnami-to-keiailab.md` 향후 작성) 에서 다룬다 — 본 디렉터리 단독 적용은 *운영 데이터 영향 (sidekiq queue, web session ESS=infisical-cloud-valkey-shared-valkey-auth)* 위험.

## 사전 조건 (cluster)

- [x] `data` namespace 사전 생성 (argos 2026-05-06 cycle).
- [x] StorageClass `ceph-rbd` (default) — argos 클러스터 검증.
- [ ] cert-manager (TLS 활성화 시 — ADR-0010 cert-manager auto-discovery. 본 sample 은 tls 블록 주석 처리).
- [ ] auth Secret 은 operator 가 자동 생성 (ADR-0013 auth always-enabled).
- [ ] **bitnami/valkey 5.6.1 와의 충돌 방지** — 데이터 마이그레이션 또는 dual-write 패턴 사전 결정.

## 적용 (수동 검증)

```fish
# 1) 렌더 검증
kustomize build deploy/overlays/prod | head
kustomize build deploy/overlays/prod | grep -c "kind: Namespace"   # 0

# 2) operator 적용
kustomize build deploy/overlays/prod | kubectl apply -f -
kubectl -n data rollout status deploy/valkey-operator-controller-manager

# 3) workload 적용 (bitnami 와 충돌 위험 — 마이그레이션 plan 후)
kubectl apply -f deploy/valkey-cluster.yaml
kubectl -n data get valkeycluster valkey-cluster \
    -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

## 변경 절차

본 디렉터리 변경은 ADR 작성 후 진행 (`docs/kb/adr/`). 매번 `kustomize build deploy/overlays/prod` 렌더 검증. ADR-0028 (Helm/Kustomize parity invariant) 에 따라 charts/ 와도 동기화 검증.
