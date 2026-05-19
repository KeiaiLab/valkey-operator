# valkey-operator OLM Cut-over Runbook

> ADR-0137 D4 (Atomic switch, ≤90s downtime) 의 production cluster 실행 절차.
> 본 runbook 은 *사용자 명시 GO* 후에만 실행. AI 자율 진행 금지 (비가역 운영).
>
> Refs: ADR-0137 (infra/bootstrap), incident-kb.md §10 (INC-0002 SA token mount
> sister 차단), RFC-0004 §3 (라이브 사실 게이트).

## 사전 조건 (Pre-flight, MUST)

다음 4 게이트 모두 PASS 확인 후 진입:

```bash
# 1. cluster context
kubectl config current-context              # expected: argos
kubectl get ns data                          # Active

# 2. 라이브 valkey-operator-prod (Helm release)
kubectl get deploy -n data valkey-operator-prod
# expected: 1/1 ready, age 9d+

# 3. ArgoCD app
kubectl get application -n argocd platform-data-valkey-operator \
  -o jsonpath='{.status.sync.status}/{.status.health.status}'
# expected: Synced/Healthy

# 4. OLM v1 controllers
kubectl get deploy -n olmv1-system operator-controller-controller-manager \
  catalogd-controller-manager
# expected: 양 deployment 1/1 ready
```

## Phase 0 — Bundle + Catalog image publish (선행, 1회)

```bash
# 0.1 bundle image (valkey-operator repo)
cd ~/WorkSpace/public/valkey-operator
make bundle-build bundle-push  # ghcr.io/keiailab/valkey-operator-bundle:v1.0.13

# 0.2 catalog image (별도 catalog repo 또는 keiailab-operator-catalog)
# (mongodb-operator-catalog 정합 패턴 — 별도 FBC 작업)
```

## Phase 1 — RGW S3 snapshot (사전 backup, ≤30s)

```bash
# infisical-valkey AOF snapshot (INC-0002 sister 차단)
kubectl exec -n data infisical-valkey-0 -- \
  redis-cli BGSAVE

# 별도 RGW S3 backup CR 적용 (ValkeyBackup)
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha2
kind: ValkeyBackup
metadata:
  name: cutover-pre-${USER}-$(date +%Y%m%d%H%M)
  namespace: data
spec:
  source:
    valkeyRef:
      name: infisical-valkey
  target:
    targetRef:
      name: rgw-s3-keiailab
  mode: RDB
EOF

# 완료 verify
kubectl get valkeybackup -n data --watch  # Phase=Completed
```

## Phase 2 — Helm release scale to 0 (reconcile 정지, ≤30s)

```bash
# ArgoCD app sync 정지 (자동 reconcile 회피)
kubectl patch application -n argocd platform-data-valkey-operator \
  --type=merge -p '{"spec":{"syncPolicy":null}}'

# Helm release scale 0
kubectl scale deploy -n data valkey-operator-prod --replicas=0

# pod 종료 verify
kubectl get pod -n data -l app.kubernetes.io/name=valkey-operator
# expected: 0 pods
```

## Phase 3 — OLM ClusterExtension apply (≤30s)

```bash
# installer SA + RBAC 먼저 적용 (ADR-0133 정합)
kubectl apply -f deploy/olm/installer-rbac.yaml

# ClusterCatalog + ClusterExtension
kubectl apply -f deploy/olm/clusterextension.yaml

# ArgoCD sync (sync-wave 5)
# (별도 ArgoCD Application 으로 wrap 권장 — argos-platform-data PR 별도)
```

## Phase 4 — Leader lease + first reconcile verify (≤30s)

```bash
# Deployment 생성 verify
kubectl get deploy -n data valkey-operator-olm
# expected: 1/1 ready

# Leader election lease (분리 — ADR-0137 D3)
kubectl get lease -n data valkey-operator-olm-leader
# expected: holderIdentity set

# CRD reconcile verify (ValkeyCluster 1건 reconcile)
kubectl get valkeycluster -n data keiailab-valkey-prod \
  -o jsonpath='{.status.observedGeneration}/{.metadata.generation}'
# expected: 동일 값 (observed == metadata generation)
```

## Phase 5 — Status drift verify

```bash
# infisical-valkey + keiailab-valkey-prod 모두 status.ready=true
for v in infisical-valkey keiailab-valkey-prod; do
  kubectl get valkey/$v -n data \
    -o jsonpath='{.metadata.name}: ready={.status.ready} replicas={.status.readyReplicas}{"\n"}'
done
```

## Phase 6 — Helm release uninstall (ApplicationSet path 제거)

```bash
# 별도 commit (platform/data wrapper repo):
# - platform/data/valkey-operator/values.yaml: enabled: false
# - 또는 ApplicationSet generator 에서 valkey-operator 제거
# ArgoCD prune 자동 → Helm release `valkey-operator-prod` Deployment 제거
```

## Phase 7 — 48h 모니터링

| 지표 | 임계 |
|---|---|
| infisical pod 503 응답 | 0건 |
| infisical-valkey AOF 손상 | 0건 |
| `valkey_deployment_name_collision_count` (governance-report) | 0 (48h+ 양 Deployment 공존 시 적색) |
| Prometheus `valkey_operator_reconcile_errors_total` | 평균 < 0.01/min |

## Rollback (Phase 4 이전 실패 시)

```bash
# 1. ClusterExtension + ClusterCatalog 삭제
kubectl delete -f deploy/olm/clusterextension.yaml --ignore-not-found

# 2. Helm release scale 복원
kubectl scale deploy -n data valkey-operator-prod --replicas=1

# 3. ArgoCD app syncPolicy 복원
# (직전 commit 의 syncPolicy 블록 재apply)
```

## 후속 작업 (Phase 7 PASS 후)

- `governance-report` 적색 0 verify (`valkey_deployment_name_collision_count`)
- ADR-0137 status: Proposed → Accepted (cut-over evidence 인용)
- HANDOFF.md 갱신 ("ArgoCD sync 후 운영 cluster `valkey-operator-olm` 가 ClusterExtension v1.0.13 active")
- bundle CSV v1.0.13 bump (현재 bundle/manifests/valkey-operator.clusterserviceversion.yaml 가 v1.0.9 → v1.0.13)
