# ARCHITECTURE — valkey-operator (한국어)

> English: [ARCHITECTURE.md](ARCHITECTURE.md) — canonical / 정본

> 단일 페이지 아키텍처 설명서. CRD 표면 / 토폴로지 / reconcile 패턴이 변경되면 같이 갱신.

## 개요

- **목적**: [Valkey](https://valkey.io) (Redis 의 BSD-3 fork) 를 위한 Kubebuilder 기반 K8s 오퍼레이터. 하나의 컨트롤러가 통일된 CRD 표면 뒤에서 세 개의 토폴로지를 관리.
- **범위**: Standalone / Replication / Cluster (16384-slot) 토폴로지 + 백업·복구 + S3 호환 외부 스토리지.
- **안정성 등급**: v1.0.13 (standalone + replication + cluster GA; federation alpha)
- **최신 릴리즈**: v1.0.13 (2026-05-13)
- **라이선스**: Apache-2.0
- **모듈 경로**: `github.com/keiailab/valkey-operator`

## CRD 표면 (5 CRD)

| CRD | apiVersion | 토폴로지 | 설명 |
|---|---|---|---|
| `Valkey` | `valkey.keiailab.com/v1alpha2` | Standalone / Replication | 단일 인스턴스 또는 1 primary + N replicas |
| `ValkeyCluster` | `valkey.keiailab.com/v1alpha2` | 샤드 Cluster (16384 slot) | 3+ 샤드 × (1 primary + 0–5 replicas) |
| `ValkeyBackup` | `valkey.keiailab.com/v1alpha2` | — | PVC + 외부 스토리지로 1회성 RDB 또는 AOF 백업 |
| `ValkeyBackupTarget` | `valkey.keiailab.com/v1alpha2` | — | S3 호환 외부 스토리지 추상화 (ADR-0016) |
| `ValkeyRestore` | `valkey.keiailab.com/v1alpha2` | — | Init Container 패턴으로 RDB 를 Valkey 또는 ValkeyCluster 에 복원 (ADR-0015) |

Conversion webhook 이 v1alpha1 ↔ v1alpha2 변환을 지원.

## Reconcile 흐름

```
Watch CRD events
      │
      ▼
Reconcile loop
      │
      ├── StatefulSet (per shard)
      ├── ConfigMap (valkey.conf)
      ├── Secret (auth + TLS keys)
      ├── Service (headless + ClusterIP)
      ├── PodDisruptionBudget
      ├── NetworkPolicy (deny-by-default)
      ├── cert-manager Certificate (webhook serving + TLS)
      └── Prometheus ServiceMonitor

모든 리소스는 spec-drift 감지와 함께 reconcile.
Cluster 토폴로지: 샤드 스케일 시 slot 재배치 + replica 재선출.
```

## RBAC 범위

- ClusterRole: CRD watch + cert-manager Certificate + Prometheus ServiceMonitor
- Role (네임스페이스 단위): StatefulSet / Service / Secret / ConfigMap / PVC / PDB / NetworkPolicy / Job
- ServiceAccount: `valkey-operator`
- Webhook: validation + conversion (cert-manager 통한 TLS)

## operator-commons import 표면

`operator-commons/ARCHITECTURE.md` 매트릭스 기준 채택률: **8/8 (100%)** — *카본 카피 레퍼런스*.

| 패키지 | 상태 | 용도 |
|---|---|---|
| `pkg/security` | ✅ | restricted PSA SecurityContext (it8) |
| `pkg/version` | ✅ | Valkey 버전 allowlist (it8) |
| `pkg/labels` | ✅ | 권장 라벨 (it29) |
| `pkg/monitoring` | ✅ | ServiceMonitor reconciler (it23) |
| `pkg/networkpolicy` | ✅ | Deny-by-default + 옵션 (it25) |
| `pkg/webhook` | ✅ | Validation 헬퍼 (it31) |
| `pkg/finalizer` | ✅ | `Add` / `Remove` / `Has` |
| `pkg/status` | ✅ | Condition reason |

valkey 는 *최초의 100% 채택자* — mongodb / postgres 는 자신의 이관에서 이 저장소를 레퍼런스로 사용.

## 테스트 계층

| 계층 | 위치 | 커버리지 |
|---|---|---|
| Unit | `internal/**/_test.go`, `api/**/_test.go` | gocovmerge → cover-final.out |
| Integration (envtest) | `test/integration/` | reconcile + conversion + webhook |
| E2E (kind) | `test/e2e/`, `Makefile setup-test-e2e` | 릴리즈 임계 시나리오 |
| Scorecard | `bundle/tests/scorecard/` | OLM v1alpha3 6-test parity |

## 빌드 / 배포

- 컨테이너 이미지: `ghcr.io/keiailab/valkey-operator:v1.0.13`
- Helm chart: `charts/valkey-operator/` (`keiailab.github.io/valkey-operator` 에 게시)
- OLM bundle: `bundle/`
- ArtifactHub: `keiailab-valkey-operator`
- Quickstart: kind 클러스터 + cert-manager 1.16+ (`make setup-test-e2e`)

## 보안 공급망

- **SLSA-3 provenance** (ADR-0046)
- **cosign keyless 서명** (ADR-0046)
- **OpenSSF Scorecard** 활성 (README 배지)
- **CodeQL** + **dependency-review** + **DCO** 워크플로
- **`.gitleaks.toml`** secret 스캔 (42/44 커버리지)
- **go-licenses** 의존성 라이선스 스캔 + allowlist

## ADR 크로스 링크 (45 ADR — 3 오퍼레이터 중 ADR 최다)

주요:
- ADR-0015: Init Container 패턴 기반 Restore
- ADR-0016: ValkeyBackupTarget — S3 추상화
- ADR-0045: GitHub Actions release 파이프라인 복원
- ADR-0046: SLSA-3 + cosign keyless
- ADR-0047: community-operators 업스트림 동기화 자동화 (cycle 25)

전체 목록: `docs/kb/adr/INDEX.md`.

## 로드맵 상태

- 완료: 31 항목 (Cluster 모드 + 백업·복구 + HPA/PDB/NP + 버전 업그레이드 + Valkey 9.x + API 진화 + webhook admission + Helm + SLSA-3 + ServiceMonitor + OpenSSF)
- 대기: 38 항목 (production cluster 채택 + 마이그레이션 runbook + smoke test + Grafana + OTel + SBOM + 9.x 기능 후속 + multi-cluster federation + cross-region 복제 + 온라인 schema-less 마이그레이션 + 가중치 replica 라우팅 + controller v2 + CRD v1 graduation)

## Non-goals

- ❌ Redis 임베드 (우리는 Valkey 를 제공 — 라이선스 호환 BSD-3 fork)
- ❌ third-party Valkey chart 임베드 (우리는 네이티브로 구현)
- ❌ Redis Sentinel 토폴로지 (대신 3-shard cluster 사용)
- ❌ Valkey 8.0 미만 버전

## 참조

- `README.md` / `README.ko.md`
- `ROADMAP.md`
- `CHANGELOG.md`
- `ADOPTERS.md` / `ADOPTERS.ko.md`
- `CONTRIBUTING.md` / `CONTRIBUTING.ko.md`
- `GOVERNANCE.md` / `GOVERNANCE.ko.md`
- `AGENTS.md`
- `docs/kb/adr/INDEX.md`

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
