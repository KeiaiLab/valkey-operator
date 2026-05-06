# valkey-operator

Kubebuilder 기반 Kubernetes operator. Valkey (Redis fork, BSD-3) 의 세 가지
운용 토폴로지를 단일 controller 로 관리한다:

| CRD | 용도 | 토폴로지 |
|---|---|---|
| `Valkey` | 단일 인스턴스 또는 1-primary + N-replica | Standalone / Replication |
| `ValkeyCluster` | 샤딩된 Valkey Cluster (16384 슬롯) | 3+ shards × (1 primary + 0~5 replicas) |
| `ValkeyBackup` | 일회성 RDB 또는 AOF 백업 | PVC (`<backup>-backup`) — 외부 저장 별개 |
| `ValkeyBackupTarget` | 외부 저장 (S3/GCS/Azure) endpoint + 자격증명 추상화 | Backup ↔ Restore 공유 — ADR-0016 |
| `ValkeyRestore` | Backup PVC 의 RDB 를 cluster 로 복원 | Init Container 패턴 — ADR-0015 |

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

## ValkeyBackupTarget + 외부 저장 (S3) 사용법 — 신규

ADR-0016 + ADR-0022 + ADR-0023. ValkeyBackupTarget 으로 외부 저장 endpoint /
자격증명을 *추상화* — Backup ↔ Restore 가 동일 target 참조 (대칭).

### 1. ValkeyBackupTarget + 자격증명 Secret 생성

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: s3-creds
  namespace: default
stringData:
  AWS_ACCESS_KEY_ID:     "AKIAEXAMPLE..."
  AWS_SECRET_ACCESS_KEY: "secret..."
---
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackupTarget
metadata:
  name: s3-prod
  namespace: default
spec:
  type: S3
  s3:
    endpoint: https://s3.ap-northeast-2.amazonaws.com  # MinIO 사내: https://minio.local:9000
    region: ap-northeast-2
    bucket: valkey-backups-prod
    prefix: cluster-A/
    forcePathStyle: false   # MinIO/Ceph RGW 시 true
    credentialsSecretRef:
      name: s3-creds
```

operator 가 BucketExists 호출하여 reachability 검증 → Status.Phase=Reachable.

### 2. ValkeyBackup → S3 업로드

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata: { name: vkb-s3, namespace: default }
spec:
  clusterRef: { kind: Valkey, name: vk-standalone }
  type: RDB
  destination:
    type: TargetRef
    targetRef:
      name: s3-prod
      path: 2026-05-06/dump.rdb     # 미명시 시 default <backup>/<startedAt>/dump.rdb
```

흐름: Pending → InProgress → Copying (PVC 저장) → **Uploading** (Upload Job
이 operator image 의 `upload` sub-command 호출 → S3 업로드) → Completed.

### 3. ValkeyRestore from S3 (cross-cluster 가능)

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata: { name: vkr-from-s3, namespace: default }
spec:
  clusterRef: { kind: Valkey, name: vk-standalone }
  source:
    targetRef:
      name: s3-prod
      path: 2026-05-06/dump.rdb     # ValkeyBackup 의 path 와 동일
  restoreType: RDB
```

흐름: Pending → **Mounting** (target Reachable 검증 + 임시 PVC 생성 +
Download Job spawn → S3 → 임시 PVC) → Restoring (init container 가 PVC 의
RDB 를 /data/dump.rdb 로 cp) → Verifying → Completed.

## ValkeyCluster Restore — 신규 (cycle 5)

ADR-0015 의 *Standalone-only* 제약 완전 해소. ValkeyCluster mode 도 restore
가능 — 단일 STS 의 ordinal 기반 shard mapping init container 활용.

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata:
  name: vkr-cluster-recovery
  namespace: default
spec:
  clusterRef:
    kind: ValkeyCluster
    name: valkeycluster-sample
  source:
    pvc:
      name: backup-cluster-pvc           # ROX accessMode 필수 (multi-pod 동시 mount)
      shardLayout:                       # shard 별 source path 매핑 (옵션)
        "0": shard-0/dump.rdb
        "1": shard-1/dump.rdb
        "2": shard-2/dump.rdb
        # 미명시 시 default `shard-{N}/dump.rdb` 자동 적용
  restoreType: RDB
```

흐름: 단일 STS 의 모든 pod 가 동일 init container 사용 → shell 에서
`${HOSTNAME##*-}` ordinal 추출 → shard index 계산 (0..shards-1 = primary,
shards..total-1 = replica) → 적절한 source path 의 RDB cp.

**ROX accessMode 필수** — RWO source 는 multi-pod 동시 mount 불가. EFS /
NFS / GlusterFS / Ceph FS 등 file system 기반 storage class 사용. 외부
저장 (Source.TargetRef) 사용 시 `Spec.SourcePVCAccessMode: ReadOnlyMany`
명시 필수.

## Replication 자동 Failover — 신규 (cycle 7)

ADR-0017 채택. Replication mode (replicas≥2) 에서 primary pod NotReady
30s+ 감지 시 *operator 가 자동 failover*. 데이터 손실 최소화 — replica 의
master_repl_offset / slave_repl_offset 가장 큰 노드 선출 → REPLICAOF NO ONE.

### 동작 조건

- `Spec.Mode == Replication` + `Spec.Replicas > 1`
- `Spec.AutoFailover == true` (default — `false` 명시 시 비활성)

### 알고리즘

1. 현재 primary (`Status.CurrentPrimary` 또는 `<name>-0`) Pod Ready 검증.
2. NotReady 가 30s+ 미만이면 transient flap 으로 무시.
3. 모든 replica (primary 제외) 의 INFO replication 호출 → offset 추출.
4. `selectFailoverCandidate` 로 가장 큰 offset replica 선출 (tie 시
   ordinal 작은 것).
5. PromoteToPrimary (`REPLICAOF NO ONE`) 발행 → 새 primary.
6. 다른 Ready replicas 에 `EnsureReplicaOf <new-primary>` 발행.
7. `Status.CurrentPrimary` 갱신 (in-memory, 다음 reconcile 의 Status update
   에 반영).

### 비활성

```yaml
spec:
  autoFailover: false   # primary kill 시 사용자가 수동 처리
```

### 한계

- **Split-brain 강력 보장 없음** — 네트워크 분단 시 두 primary 발생 가능.
  *완화*: NotReady 30s 임계값 + operator leader-elect (단일 인스턴스).
  Stronger 일관성은 별개 ADR (Sentinel/Raft 추후).
- **ValkeyCluster 모드는 N/A** — valkey native cluster mode 의
  `cluster_replica_validity_factor` 가 처리. ValkeyClusterReconciler 는
  관여 안 함.
- **e2e 자동 검증 미진행** (별개 cycle).

## ValkeyRestore (Standalone PVC) 사용법 — 신규

전제: 이미 ValkeyBackup 으로 backup PVC (`<backup-name>-backup`) 가 생성됨.

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata:
  name: vk-standalone-restore
  namespace: default
spec:
  clusterRef:
    kind: Valkey         # ValkeyCluster 는 후속 commit
    name: vk-standalone  # 대상 Valkey CR (Mode=Standalone, replicas=1)
  source:
    pvc:
      name: vkb-rdb-backup  # ValkeyBackup 결과 PVC
      path: dump.rdb        # 기본값
  restoreType: RDB
```

동작:
1. operator 가 대상 Valkey 에 `cache.keiailab.io/paused=true` annotation set
   → ValkeyController 의 정상 reconcile 차단.
2. STS PodTemplate 에 init container `valkey-restore-init` + source volume 추가
   → STS rolling restart 자동 → init container 가 `/restore/dump.rdb` 를
   `/data/dump.rdb` 로 cp.
3. Main valkey-server 가 시작 시 *자동 RDB 로드* (Valkey 의 표준 매커니즘).
4. 모든 pod Ready → STS 원복 (init container 제거) → 두 번째 rolling →
   paused annotation 제거 → Phase=Completed.

```sh
kubectl apply -f valkeyrestore.yaml
kubectl wait --for=jsonpath='{.status.phase}'=Completed valkeyrestore/vk-standalone-restore
PASS=$(kubectl get secret vk-standalone-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec vk-standalone-0 -- valkey-cli -a "$PASS" get <키>
```

## 운영 시나리오 검증 (실측)

| 시나리오 | 동작 | 데이터 |
|---|---|---|
| primary pod kill (force) | STS 재생성 → operator 가 pod-0 재 promote | PVC 보존 |
| replica scale up (3→5) | 새 replica 가 자동으로 master link up | — |
| replica scale down (5→2) | 잉여 pod 정리 | 기존 데이터 유지 |
| ValkeyCluster shard pod kill | cluster_state=ok 유지 (replica 가 즉시 take over) | 모든 슬롯 보존 |
| TLS+mTLS ValkeyCluster (cert-manager) | Phase=Running, slots=16384, 데이터 plane SET/GET ✓ | — |
| TLS+mTLS Valkey Standalone (cert-manager) | Phase=Running, ping/set/get on 6380 ✓ | — |
| TLS+mTLS Valkey Replication (3 replicas) | master_link_status:up, write propagation 모든 replica ✓ | — |
| ValkeyBackup (RDB) | Pending → InProgress → Completed, /data/dump.rdb 89 bytes 생성 | — |
| NetworkPolicy 리소스 생성 | selfPeer + 6379(/16379) ingress + ownerReferences | (CNI 의존) |
| operator metrics endpoint (HTTPS:8443) | controller_runtime_* + valkey_cluster_* 노출 | — |

## 잠재적 운영 이슈 (현재 알려진 한계)

- `Spec.Auth.Enabled=false` 가 무시됨 — ADR-0013 옵션 A. operator 항상 auth 강제.
- IPv6-only 환경 미테스트 (CLUSTER MEET 의 IPv4 prefer, ADR-0012).
- `cluster-announce-hostname` 미사용 (필요 시 별도 RFC 검토).
- **ValkeyBackup M3.5 완료** (commit 7458228): `valkey-cli --rdb` Job 이
  SYNC 프로토콜로 fresh RDB 를 별도 PVC (`<backup-name>-backup`) 에 저장. Phase
  Copying 추가. TLS 자동 전파. Job TTL 24h.
- **ValkeyRestore Standalone 첫 동작** (commit fbb96d7, ADR-0015):
  Init Container 패턴 — backup PVC 를 mount 후 `/data/dump.rdb` 로 cp →
  valkey-server 가 표준 RDB 자동 로드. paused annotation 으로 ValkeyController
  와의 충돌 차단. **현재 한계**: Standalone Valkey (replicas=1) + Source.PVC
  만 지원. Replication / Cluster + 외부 source (TargetRef) 는 후속 commit.
- **외부 저장 (S3 호환) 통합 완료** (commits 505c6c1, fc04cdd, bc6e28b,
  ADR-0022 + ADR-0023): minio-go v7 채택 (CVE 0건 검증). ValkeyBackupTarget
  실제 BucketExists ping. ValkeyBackup.Spec.Destination.Type=TargetRef 시
  Uploading phase 가 별도 Upload Job 으로 외부 저장 업로드. ValkeyRestore.
  Source.TargetRef 시 임시 PVC + Download Job 으로 다운로드 → cross-cluster
  restore 가능. **TTL 기반 자동 삭제 + GCS/Azure** 는 별개. AWS S3 + MinIO
  + Ceph RGW 호환 (forcePathStyle 옵션).
- ~~Replication 모드 + TLS 미구현~~ → **iter 9 에서 구현 완료** (ADR-0014 AI-007):
  `tlsConfigForValkey` + `dialValkey` 추가 + `tls-replication yes` 디렉티브.
  3-replica + cert-manager 검증 통과 (master_link_status:up, write propagation).
- **NetworkPolicy 강제 동작 검증 부재**: 리소스 정의 정합성만 확인.
  Calico / Cilium 등 NP-enforcing CNI 클러스터 에서 별도 검증 필요.

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

[CONTRIBUTING.md](CONTRIBUTING.md) 참조 — 환경 요구사항, PR 절차,
Conventional Commits, ADR 작성 의무 시점, 코드 스타일.

보안 취약점은 **공개 issue 로 보고하지 마세요**. [SECURITY.md](SECURITY.md)
의 비공개 보고 경로 사용 (GitHub Security Advisory 또는 email).

운영 절차 (Backup / Restore / Scaling / Upgrade / 응급 조치) 는
[docs/operations/runbook.md](docs/operations/runbook.md).

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

