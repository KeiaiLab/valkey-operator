# valkey-operator のアップグレード

> English: [UPGRADING.md](UPGRADING.md) — canonical / 正本

本書は valkey-operator のマイナー / メジャー版アップグレード時に必要な
マイグレーション作業をまとめたものである。Helm 利用者は chart の
アップグレードだけで全変更が反映されるが、静的 manifest (`kubectl apply -f`)
利用者は RBAC など一部項目を手動で patch する必要がある。

## 0. バージョン方針 (semver)

| 変更種別 | semver bump | 例 |
|---|---|---|
| 新規 controller / CR / API の追加 | minor (v1.X → v1.X+1) | ValkeyBackupTarget 新設 |
| 既存 API シグネチャの変更 (breaking) | major (v1.X → v2.0) | ValkeyCluster.spec.storage の struct 変更 |
| bug fix / 依存 bump | patch (v1.X.Y → v1.X.Y+1) | controller-runtime 0.19→0.20 |
| operator-commons 依存の bump | minor (commons v0.X → v0.X+1) | pkg/pvc + pkg/topology の採用 |

## 1. v1.0.x → v1.0.13 (現行)

### Helm 利用者

```bash
helm repo update
helm upgrade valkey-operator keiailab-valkey-operator/valkey-operator \
  --namespace valkey-operator-system \
  --version 1.0.13
```

chart 自体が RBAC / CRD / Deployment をすべて同期する。追加作業は不要。

### 静的 manifest 利用者 — RBAC マイグレーション

`make build-installer` が生成する `dist/install.yaml` の差分を確認する:

```bash
kubectl diff -f dist/install.yaml
kubectl apply -f dist/install.yaml
```

既存 ClusterRole への新規権限追加 (現 patch では RBAC 変更なし):

| API group | Resource | 理由 | 追加時点 |
|---|---|---|---|
| (なし) | — | — | — |

### v1alpha1 → v1alpha2 conversion webhook

v1alpha2 を新規導入。v1alpha1 の CR は conversion webhook で自動変換される
ため利用者の対応は不要。ただし `kubectl apply -f` で v1alpha1 manifest を
新規作成すると *deprecated* 警告が出るため、v1alpha2 の利用を推奨する。

## 2. Sprint 1 の採用 (operator-commons v0.9.0)

ADR-0049 (`docs/kb/adr/0049-sprint-1-commons-pvc-topology-adoption.md`)。

```bash
# go.mod の operator-commons 依存を bump したのち
go mod tidy
```

- **新規 import**: `github.com/keiailab/operator-commons/pkg/pvc`, `pkg/topology`
- **削除されたコード**: `internal/controller/pvc_resize.go` (-136 LOC) + テスト
  (-166 LOC) + `internal/resources/statefulset.go` の inline
  `defaultTopologySpread` (-22 LOC) → 合計 -322 LOC
- **callsite の置換**:
  - `valkey_controller.go:235` — `commonspvc.ExpandDataPVCs(ctx, c, ns, []string{crName}, size)`
  - `valkeycluster_controller.go:239` — 同上
  - `statefulset.go` — `commonstopology.Defaulted(constraints, replicas, selector)`

マイグレーションの影響:
- Reconcile の挙動は同一 (リファクタのみで、外部から見える挙動の変更なし)
- CRD spec の変更なし
- Helm chart への影響なし

## 3. v1.0.x → v2.0.0 (予定 — v3.x-stable 宣言時)

CLAUDE.md §7 にいう *商用製品レベル* (P0+P1+P2+OP+C すべて ✅) に到達した
段階で実施する。

- 全 CR の API stability を `Stable` (v1) に昇格
- breaking change は *最小化* — major bump 自体が *意味のあるシグナル*
- 5 repo 間の整合性を保証: `commons/docs/quality/production-grade-checklist.md` を参照

詳細は operator-commons ADR-0013 (audit-production-grade.sh)。

## 4. GHA dual-track 方針 (ADR-0048)

本 repo は RFC-0002 (GitHub Actions 永久禁止) の *例外* である。public OSS
operator として external trust gate (CodeQL / OpenSSF Scorecard / cosign /
SLSA / Artifact Hub trust badge) が必要なため、GHA 14 workflow を維持しつつ
ローカル 4 階層 (lefthook) との dual-track 運用を行う (ADR-0048)。

アップグレード時の GHA workflow 変更は `dependabot/github_actions/*` PR
で自動化される。*人手の PR* で `.github/workflows/` に新規ファイルを追加
する場合は *別 ADR* + 利用者の承認が必須となる。

## 5. 一般的なマイグレーション・チェックリスト

アップグレード前:
- [ ] CRD 変更 (`api/v1alpha1/` と v1alpha2 conversion webhook の互換性)
- [ ] `make verify` (lint + test + build + audit) PASS
- [ ] 既存 e2e スイート PASS (`make integration-test`)
- [ ] chaos-mesh シナリオ PASS (ADR-0041、4 シナリオ)
- [ ] dependabot 依存 bump PR の統合確認

アップグレード後:
- [ ] Helm chart の `dependencies:` (keiailab-commons library chart) を更新
- [ ] 各 CR の spec 互換性を検証 (とくに storage / resources)
- [ ] reconcile 結果を検証 (`kubectl get valkey,valkeycluster -A`)
- [ ] 運用 metric (`Reconcile{Total,Latency,Errors}`) が正常
- [ ] cluster mode: `ClusterInitialized=true` + `state=ok` の確認 (ADR-0039)

## 6. 非互換変更の周知方針

- **Deprecation**: 新規 minor で `// Deprecated:` コメントを付与し、2 minor 後に削除
- **Breaking**: major bump + 本 UPGRADING.md に専用節 + ADR を作成
- **事後通知は行わない**: すべての breaking 変更は *最低 1 minor* の事前 deprecation を経る

## 参考

- ADR 一覧: `docs/kb/adr/INDEX.md`
- operator-commons の UPGRADING: https://github.com/keiailab/operator-commons/blob/main/docs/UPGRADING.md
- audit: `make audit-quality` (5 repo を計測、commons ADR-0013)
- i18n: `commons/docs/i18n/README.md`
- ファミリー: `docs/family.md`
- Helm chart: https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator
