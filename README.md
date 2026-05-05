# valkey-operator

Kubebuilder 기반 Kubernetes operator. Valkey (Redis fork, BSD-3) 의 세 가지
운용 토폴로지를 단일 controller 로 관리한다:

| CRD | 용도 | 토폴로지 |
|---|---|---|
| `Valkey` | 단일 인스턴스 또는 1-primary + N-replica | Standalone / Replication |
| `ValkeyCluster` | 샤딩된 Valkey Cluster (16384 슬롯) | 3+ shards × (1 primary + 0~5 replicas) |
| `ValkeyBackup` | 일회성 RDB 또는 AOF 백업 | PVC 또는 외부 스토리지 |

자동화 범위: STS / ConfigMap / Secret / Service (headless + clusterIP) /
PodDisruptionBudget / NetworkPolicy / cert-manager Certificate /
Prometheus ServiceMonitor — 모두 Spec drift 감지.

## Quickstart (kind)

검증된 로컬 부트스트랩 시퀀스. 본 README 의 모든 명령은 *실측 통과 버전* 이다.

### 1. 사전 요구사항

| 도구 | 최소 버전 | 비고 |
|---|---|---|
| Go | 1.25 | `go.mod` 와 일치 |
| Docker | 24+ | buildx 기본 빌더 사용 |
| kind | 0.27+ | 로컬 클러스터 |
| kubectl | 1.34+ | k3s/kind 모두 지원 |
| cert-manager | 1.16+ | webhook serving cert |

### 2. kind 클러스터 + cert-manager

```sh
make setup-test-e2e                   # kind cluster "valkey-operator-test-e2e" 생성
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=120s -n cert-manager deploy --all
```

### 3. 이미지 빌드 + 로드 + 배포

```sh
make docker-build IMG=valkey-operator:dev
kind load docker-image valkey-operator:dev --name valkey-operator-test-e2e
make install                          # CRD 설치
make deploy IMG=valkey-operator:dev   # operator + RBAC + webhook 배포
kubectl -n valkey-operator-system rollout status deploy/valkey-operator-controller-manager
```

### 4. 샘플 CR 적용

```sh
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeycluster.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeybackup.yaml
```

### 5. 데이터 plane 검증

```sh
PASS=$(kubectl get secret valkey-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" ping        # → PONG
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" set k v     # → OK
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" get k       # → v

# 클러스터 모드 — `-c` 로 MOVED 자동 follow
PASS=$(kubectl get secret valkeycluster-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkeycluster-sample-0 -- valkey-cli -a "$PASS" cluster info | head -3
# cluster_state:ok / cluster_slots_assigned:16384 / cluster_slots_ok:16384
```

## 보안 동작

- **Auth 항상 강제** (ADR-0013): `Spec.Auth.Enabled` 값과 무관하게 random 32B
  password 생성, `requirepass` + `masterauth` 설정.
- **TLS** 는 `Spec.TLS.Enabled=true` 에서 cert-manager Certificate 자동 발급
  (ADR-0010) 또는 사용자 제공 Secret (`Spec.TLS.CustomCert.SecretName`).
- **NetworkPolicy** (`Spec.NetworkPolicy.Enabled=true`): pod-to-pod 6379/16379
  외 모든 인그레스 차단.

## 운영 시나리오 검증 (실측)

| 시나리오 | 동작 | 데이터 |
|---|---|---|
| primary pod kill (force) | STS 재생성 → operator 가 pod-0 재 promote | PVC 보존 |
| replica scale up (3→5) | 새 replica 가 자동으로 master link up | — |
| replica scale down (5→2) | 잉여 pod 정리 | 기존 데이터 유지 |
| ValkeyCluster shard pod kill | cluster_state=ok 유지 (replica 가 즉시 take over) | 모든 슬롯 보존 |

## 잠재적 운영 이슈 (현재 알려진 한계)

- `Spec.Auth.Enabled=false` 가 무시됨 — ADR-0013 옵션 A. operator 항상 auth 강제.
- IPv6-only 환경 미테스트 (CLUSTER MEET 의 IPv4 prefer, ADR-0012).
- `cluster-announce-hostname` 미사용 (필요 시 별도 RFC 검토).

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/valkey-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/valkey-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v2-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

