<p align="center">
  <img src="https://keiailab.com/assets/logo.svg" alt="keiailab" width="120"/>
</p>

# valkey-operator

> **Kubernetes 용 Apache-2.0 Valkey Operator — Standalone + Cluster + Backup/Restore, BSD-3 라이선스 클린**

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"/></a>
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go Version"/></a>
  <a href="https://valkey.io/"><img src="https://img.shields.io/badge/Valkey-8.0+-FF4438?logo=redis" alt="Valkey"/></a>
  <a href="https://kubernetes.io/"><img src="https://img.shields.io/badge/Kubernetes-1.26+-326CE5?logo=kubernetes" alt="Kubernetes"/></a>
  <a href="https://github.com/keiailab/valkey-operator/pkgs/container/valkey-operator"><img src="https://img.shields.io/badge/ghcr.io-keiailab%2Fvalkey--operator-blue?logo=github" alt="Container Image"/></a>
  <a href="https://keiailab.github.io/valkey-operator"><img src="https://img.shields.io/badge/dynamic/yaml?url=https://raw.githubusercontent.com/keiailab/valkey-operator/main/charts/valkey-operator/Chart.yaml&label=helm%20v" alt="Helm Chart"/></a>
  <a href="https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/keiailab-valkey-operator" alt="Artifact Hub"/></a>
  <a href="https://scorecard.dev/viewer/?uri=github.com/keiailab/valkey-operator"><img src="https://api.scorecard.dev/projects/github.com/keiailab/valkey-operator/badge" alt="OpenSSF Scorecard"/></a>
  <a href="https://github.com/keiailab/valkey-operator/discussions"><img src="https://img.shields.io/github/discussions/keiailab/valkey-operator?label=discussions&logo=github" alt="GitHub Discussions"/></a>
</p>

<p align="center">
  <a href="README.md">English</a> |
  <b>한국어</b> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.zh.md">中文</a>
</p>

---

[Valkey](https://valkey.io/) (Redis 의 BSD-3 fork) 를 위한 Kubebuilder
기반 Kubernetes operator. 단일 controller 가 세 가지 운영 토폴로지를
균일한 CRD surface 로 관리한다.

| CRD | 용도 | 토폴로지 |
|---|---|---|
| `Valkey` | 단일 인스턴스 또는 1-primary + N-replica | Standalone / Replication |
| `ValkeyCluster` | 샤딩된 Valkey Cluster (16384 슬롯) | 3+ shards × (1 primary + 0–5 replicas) |
| `ValkeyBackup` | 1회성 RDB 또는 AOF 백업 | PVC (`<backup>-backup`), 외부 저장 선택 |
| `ValkeyBackupTarget` | S3 호환 외부 저장 추상화 | Backup 과 Restore 가 공유 (ADR-0016) |
| `ValkeyRestore` | RDB 를 Valkey 또는 ValkeyCluster 로 복원 | Init Container 패턴 (ADR-0015) |

operator 는 `StatefulSet`, `ConfigMap`, `Secret`, `Service` (headless +
ClusterIP), `PodDisruptionBudget`, `NetworkPolicy`, cert-manager
`Certificate`, Prometheus `ServiceMonitor` 를 reconcile 한다 — 모두 spec
drift 감지.

## Quickstart (kind)

본 README 의 모든 명령은 매 릴리즈마다 검증된다. kind 클러스터 부트스트랩은
표준 로컬 개발 경로다.

### 1. 사전 요구사항

| 도구 | 최소 버전 | 비고 |
|---|---|---|
| Go | 1.26 | `go.mod` 와 일치 |
| Docker | 24+ | buildx 기본 빌더 |
| kind | 0.27+ | 로컬 클러스터 |
| kubectl | 1.34+ | k3s/kind 호환 |
| cert-manager | 1.16+ | webhook serving cert |

### 2. kind 클러스터 + cert-manager

```sh
make setup-test-e2e
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=120s -n cert-manager deploy --all
```

### 3. 빌드, 로드, 배포

```sh
make docker-build IMG=valkey-operator:dev
kind load docker-image valkey-operator:dev --name valkey-operator-test-e2e
make install                          # CRD 설치
make deploy IMG=valkey-operator:dev   # operator + RBAC + webhook
kubectl -n valkey-operator-system rollout status deploy/valkey-operator-controller-manager
```

### 4. 샘플 CR 적용

```sh
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeycluster.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeybackup.yaml
```

### 5. 데이터 plane smoke

```sh
PASS=$(kubectl get secret valkey-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" ping     # PONG
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" set k v  # OK
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" get k    # v

# Cluster 모드 — `-c` 옵션이 MOVED redirect 자동 follow
PASS=$(kubectl get secret valkeycluster-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkeycluster-sample-0 -- valkey-cli -a "$PASS" cluster info | head -3
# cluster_state:ok / cluster_slots_assigned:16384 / cluster_slots_ok:16384
```

## Helm

```sh
helm repo add valkey-operator https://keiailab.github.io/valkey-operator
helm install valkey-operator valkey-operator/valkey-operator \
    --namespace valkey-operator-system --create-namespace
```

차트는 [Artifact Hub](https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator)
에 `Signed` 신뢰 배지와 함께 게시되어 있다 (ADR-0044, ADR-0046).

## 핵심 기능

- **세 가지 토폴로지, 하나의 operator.** Standalone / Replication /
  Valkey Cluster 모두 단일 reconciler set 과 균일한 status surface 를 공유.
- **자동 Failover** — Replication 모드. 가장 큰 `master_repl_offset` 을
  가진 replica 를 선출하여 `REPLICAOF NO ONE` 으로 promote (ADR-0017).
- **Backup / Restore** — RDB 또는 AOF 를 PVC, S3, 또는 S3 호환 endpoint
  (MinIO, Ceph RGW) 에. Restore 는 Init Container 패턴을 사용 — main
  컨테이너가 RDB 를 자동 로드 (ADR-0015, ADR-0016, ADR-0022, ADR-0023).
- **TLS + mTLS** — cert-manager 자동 인식 (ADR-0010, ADR-0014) 또는
  사용자 제공 `Secret`.
- **Auth 항상 강제.** `Auth.Enabled` 미설정 시 random 32-byte 패스워드
  자동 생성 (ADR-0013).
- **NetworkPolicy** — opt-in. pod-to-pod 트래픽을 6379 / 16379 로
  제한 (CNI enforce).
- **관측성.** OTEL tracing 22 spans (`OTEL_EXPORTER_OTLP_ENDPOINT`
  미설정 시 zero overhead), Prometheus 알람 규칙, ServiceMonitor 자동 생성.
- **공급망.** SBOM (syft SPDX) + Trivy 스캔 + cosign keyless 서명 +
  SLSA-3 provenance — v1.0.13 부터 적용 (ADR-0046).
  검증 명령은 [SECURITY.md](SECURITY.md) 참조.

## 문서

| 주제 | 위치 |
|---|---|
| 문서 hub (전체) | [docs/README.md](docs/README.md) |
| 운영 runbook (한국어) | [docs/operations/runbook.ko.md](docs/operations/runbook.ko.md) |
| 릴리즈 사전 검증 체크리스트 (한국어) | [docs/operations/release-checklist.ko.md](docs/operations/release-checklist.ko.md) |
| Architecture Decision Records | [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md) |
| Contributing (한국어) | [CONTRIBUTING.ko.md](CONTRIBUTING.ko.md) |
| 보안 정책 + artifact 검증 (한국어) | [SECURITY.ko.md](SECURITY.ko.md) |
| 프로젝트 거버넌스 (한국어) | [GOVERNANCE.ko.md](GOVERNANCE.ko.md) |
| Adopters (한국어) | [ADOPTERS.ko.md](ADOPTERS.ko.md) |

## Production readiness

본 operator 는 `v1alpha1` 단계이지만 *상용 제품 수준* 의 품질 시스템을
갖추고 있다:

- **29 개 SSOT 정합 게이트** — alert / runbook / RBAC / CRD / sample /
  chart artifact drift 를 lefthook pre-push 가 차단.
- **`make manifests` 가 chart-CRD 자동 동기**, `git push` 가 stale
  `go mod tidy` 차단.
- **마이크로벤치마크** — 5 개 hot-path parser
  (`go test -bench=. ./internal/valkey/`).
- **운영 runbook** — 9 + 운영 시나리오 + Failover 알고리즘, 알람별
  Trigger / Diagnosis / Mitigation / Escalation.
- **공급망.** Apache-2.0 라이선스, PGP 서명된 보안 공개, v1.0.13 부터
  서명된 Helm chart + image.
- **재사용 conventions** 가 sibling operator (`mongodb-operator`,
  `postgres-operator`, `operator-commons`) 와 공유.

## Roadmap

본 roadmap 은 정성적이다 — 일정 약속 없음. 진척은 기능 완성도로 추적,
분기로 추적 안 함.

이미 출시 (alpha):

- ✅ Standalone / Replication / ValkeyCluster 토폴로지
- ✅ Backup → PVC, S3 호환 저장
- ✅ Restore (Init Container, ADR-0015)
- ✅ Replication 자동 Failover (ADR-0017)
- ✅ Prometheus 알람 + runbook
- ✅ OTEL tracing
- ✅ Helm chart + Artifact Hub 게시

다음:

- [ ] kind + MinIO end-to-end 자동화
- [ ] ValkeyCluster 자동 resharding (ADR-0018)
- [ ] Replication 모드 HPA (ADR-0027, v1alpha1 stable 후로 deferred)
- [ ] `v1beta1` Conversion webhook (ADR-0026, deferred)
- [ ] Track A/B/E 안정 + 24h soak 후 첫 `v0.1.0` GA

의사결정 근거는 [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md). 기능 요청은
[Issues](https://github.com/keiailab/valkey-operator/issues) 또는 GitHub
Discussions.

## 알려진 한계

본 operator 는 `v1alpha1` 소프트웨어이며 매 릴리즈마다 검증되지만 아직
GA 가 아니다. 현재 알려진 caveat:

- `Spec.Auth.Enabled=false` 는 no-op 처리 — operator 가 항상 auth 강제
  (ADR-0013). 비인증 클러스터가 필요하면 본 operator 를 배포하지 마세요.
- IPv6-only 환경 미테스트 — `CLUSTER MEET` 가 IPv4 hostname 선호
  (ADR-0012).
- `NetworkPolicy.Enabled` 는 리소스 발행만; *실제* 강제는 정책 인식
  CNI (Calico, Cilium) 에 의존.
- Replication 자동 Failover 는 네트워크 분단 시 강력한 split-brain
  보장 없음 — trade-off 는 ADR-0017 참조.
- ValkeyCluster restore 는 `ReadOnlyMany` 또는 `ReadWriteMany` source
  PVC accessMode 필수; RWO 미지원.
- `cluster-announce-hostname` 미사용; pod hostname 을 in-cluster DNS
  와 다르게 라우팅 가능한 IP 로 resolve 하는 Kubernetes-aware DNS
  서비스 환경이면 재검토.

상세 운영 한계는 [docs/operations/runbook.ko.md §10–§11](docs/operations/runbook.ko.md).

## Uninstall

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

## Contributing

[CONTRIBUTING.ko.md](CONTRIBUTING.ko.md) 참조. 외부 기여 환영 — non-trivial
변경은 코드 작성 전 issue 를 먼저 열어 API surface 정합을 맞춰주세요.

`make help` 로 모든 Makefile target 확인. 배경 자료:
[Kubebuilder book](https://book.kubebuilder.io/introduction.html).

## 취약점 보고

공개 issue 를 **열지 마세요**. [SECURITY.ko.md](SECURITY.ko.md) 의 비공개
채널 사용 — GitHub Security Advisory 또는 `security@keiailab.com`
(`artifacthub-repo.yml` 의 PGP key).

## License

Copyright 2026 Keiailab.

Apache License, Version 2.0 (<http://www.apache.org/licenses/LICENSE-2.0>)
하에 라이선스됨. 보증 없이 "AS IS" 로 배포. 전체 문구는 [LICENSE](LICENSE)
파일 참조.

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a>
</p>

<p align="center">
  © 2026 keiailab · <a href="LICENSE">Apache-2.0</a> · <a href="https://keiailab.com">keiailab.com</a>
</p>
