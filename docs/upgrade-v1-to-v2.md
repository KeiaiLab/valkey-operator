# valkey-operator chart v1.x → v2.0.0 Upgrade Guide

본 문서는 valkey-operator helm chart 의 v1.x.x → v2.0.0 *major breaking* 업그레이드 영향 + opt-out 절차를 codify 한다.

**관련 plan**: `~/.claude/plans/valkey-operator-t0-fix-chart-ha-mirror-gitlab-ci.md` (T0 fix implementation)
**관련 audit**: e104 cordon SPOF RCA + Codex stage 3 adversarial review 9 critical/major

---

## 1. Breaking Changes 요약

| Field | v1.x.x (기존) | v2.0.0 (신규) | 영향 |
|---|---|---|---|
| `replicaCount` | `1` | `2` | operator HA default. resource (CPU/memory) 2x 증가. 단일 node dev cluster 영향. |
| `podDisruptionBudget.enabled` | `false` | `true` | PDB resource *자동 생성*. eviction 시 `allowed disruptions=1` 보호. 단일 replica + PDB 결합 = drain blocker (CDEX-C2 회피 이유 = replicaCount=2). |
| `topologySpreadConstraints` | `[]` | `[zone-based DoNotSchedule]` | pod 가 zone 별 분산 강제. single-zone cluster + 신규 deployment 시 pending pod 가능. |

## 2. 자동 적용되는 user (no-action upgrade)

다음 조건 *모두* 충족 시 `helm upgrade` 만으로 v2.0.0 적용 — 추가 변경 불필요:

- multi-node + multi-zone Kubernetes cluster
- resource budget 충분 (operator pod x 2)
- 기존에 PDB / topology 명시 override 안 한 경우 (`enabled: false` 또는 `topologySpreadConstraints: null` 미명시)

verify:
```bash
kubectl get nodes -L topology.kubernetes.io/zone
helm get values <release-name> | grep -E "replicaCount|podDisruptionBudget|topologySpread"
```

## 3. Opt-out path (기존 v1.x 동작 유지)

기존 deployment 가 *single-replica 의도* + *PDB 의도 부재* + *topology 강제 회피* 시:

```bash
helm upgrade <release> valkey-operator/valkey-operator \
  --version 2.0.0 \
  --set replicaCount=1 \
  --set podDisruptionBudget.enabled=false \
  --set topologySpreadConstraints=null
```

또는 `values.yaml` override:

```yaml
# my-values.yaml
replicaCount: 1
podDisruptionBudget:
  enabled: false
topologySpreadConstraints: null
```

verify:
```bash
helm upgrade <release> valkey-operator/valkey-operator \
  --version 2.0.0 \
  -f my-values.yaml
kubectl get pdb -n <ns>  # 해당 PDB 부재 verify
kubectl get pod -n <ns> -l app.kubernetes.io/name=valkey-operator -o wide  # 1 pod only
```

## 4. 부분 opt-out (HA enable + PDB only)

operator HA (replicaCount=2) 는 적용 + PDB 또는 topology 만 disable:

```bash
helm upgrade <release> valkey-operator/valkey-operator \
  --version 2.0.0 \
  --set podDisruptionBudget.enabled=false  # PDB 만 disable, HA 유지
```

## 5. Rollback (v2.0.0 → v1.x.x)

upgrade 후 *문제 발견* 시 immediate rollback:

```bash
helm rollback <release> <previous-revision>
kubectl get pdb -n <ns>  # PDB 부재 verify (v2 가 생성한 PDB 가 rollback 으로 삭제됨)
```

주의: PDB 가 *operator-created* (Valkey/ValkeyCluster CR 의 PDB) 라면 rollback 영향 받지 않음. chart-created PDB (operator deployment 의 `valkey-operator-prod` PDB) 만 rollback 으로 삭제됨.

## 6. Rationale (왜 breaking 이 정당화되는가)

### 6.1 audit + Codex 진본 발견

본 v2.0.0 의 모든 breaking change 는 다음 진본 발견에 대응:

- **audit Critical C3**: chart HA defaults disabled (`enabled: false` + `topologySpread: []`) = e104 cordon SPOF root cause 의 *chart layer 책임*.
- **Codex CDEX-C2**: chart PDB default-on + replicaCount=1 = operator drain blocker. **replicaCount=2 동시 codify 의무**.
- **Codex CDEX-M3**: `topologySpreadConstraints` line 100 + 311 silent 중복 — line 311 가 line 100 array 를 silent override 했음. v2.0.0 에서 line 311 정리.
- **Codex CDEX-M4**: topology selector hardcode → release/nameOverride 불일치. v2.0.0 default = single label `name: valkey-operator` (release-safe). multi-release per namespace 시 *override 의무*.

### 6.2 e104 cordon 직접 연관

라이브 evidence (2026-05-20): `keiailab-valkey-prod-0..5` 6 pod 가 *전부 e104 node 에 위치* — 진본 root cause = *스케줄링 분산 실패*. 본 v2.0.0 의 topology defaults 가 *신규 deployment* 에서 동일 패턴 *영구 차단*.

### 6.3 operator-commons 정합

operator-commons v0.6.0 + ADR-0040 (commercial parity) 패턴 정합. mongodb-operator + postgres-operator 의 *chart HA default-on* 패턴 sister.

## 7. T1 deferred items (v2.x.y 후속)

본 v2.0.0 scope 외 — Codex Major 5건 중 T1 fork:

- **CDEX-M1**: operator PDB disable/delete path (현재 false 시 기존 PDB 삭제 X) — operator code 변경.
- **CDEX-M2**: ValkeyCluster PDB shard-aware (현재 단일 selector) — architectural redesign.
- **CDEX-M4**: topology selector helper template (multi-release per namespace 정합).
- **CDEX-M5**: helm-publish 단계 자동화 (release tag → GH Action).

## 8. 영향 받은 user 신고

upgrade 후 unexpected behavior 발견 시:
- GitLab issue: `gitlab.keiailab.com:keiailab/upstream/valkey-operator`
- GitHub mirror: `github.com:keiailab/valkey-operator` (mirror, read-only)

## Refs

- T0 fix plan: `~/.claude/plans/valkey-operator-t0-fix-chart-ha-mirror-gitlab-ci.md`
- e104 plan: `~/.claude/plans/2026-05-20-e104-valkey-redis-ha-mig.md`
- audit plan: (archive 2026-05-20)
- Codex stage 3 review: T0 fix plan 본문 "Codex Stage 3 Adversarial Review 결과" section
- RFC: RFC-0027 (helm default toggle) / RFC-0043 (GitLab CI L5) / RFC-0045 (Plan Adversarial Review) / RFC-0040 (GitLab MCP First)
- ADR: ADR-0040 (commercial parity, operator PDB) / ADR-NNNN (mirror auto-sync, 본 cycle 신규)
