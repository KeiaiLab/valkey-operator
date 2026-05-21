<p align="center">
  <img src="https://keiailab.com/assets/logo.svg" alt="keiailab" width="120"/>
</p>

# valkey-operator

> **Kubernetes 向け Apache-2.0 Valkey Operator — Standalone + Cluster + バックアップ/リストア、BSD-3 ライセンスクリーン**

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
  <a href="https://github.com/keiailab/operator-commons/blob/main/docs/quality/audit-history.md"><img src="https://img.shields.io/badge/keiailab-v3.x--stable-success?style=flat-square" alt="keiailab v3.x-stable"/></a>
  <a href="https://github.com/keiailab/operator-commons/blob/main/scripts/audit-production-grade.sh"><img src="https://img.shields.io/badge/audit-100%25-success?style=flat-square" alt="audit"/></a>
</p>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.ko.md">한국어</a> |
  <b>日本語</b> |
  <a href="README.zh.md">中文</a>
</p>

---

[Valkey](https://valkey.io/) (Redis の BSD-3 フォーク) のための
Kubebuilder ベース Kubernetes Operator です。1 つのコントローラーが
3 種類の運用トポロジーを統一された CRD インターフェース下で管理します。

| CRD | 用途 | トポロジー |
|---|---|---|
| `Valkey` | 単一インスタンス、または 1 つの primary と N 個の replica | Standalone / Replication |
| `ValkeyCluster` | シャード分割された Valkey Cluster (16384 スロット) | 3+ シャード × (1 primary + 0–5 replica) |
| `ValkeyBackup` | 単発の RDB または AOF バックアップ | PVC (`<backup>-backup`)、外部ストレージはオプション |
| `ValkeyBackupTarget` | S3 互換の外部ストレージ抽象化 | Backup と Restore で共有 (ADR-0016) |
| `ValkeyRestore` | RDB を Valkey または ValkeyCluster インスタンスへリストア | Init Container パターン (ADR-0015) |

Operator は `StatefulSet`、`ConfigMap`、`Secret`、`Service` (headless +
ClusterIP)、`PodDisruptionBudget`、`NetworkPolicy`、`cert-manager` の
`Certificate`、Prometheus `ServiceMonitor` を reconcile し、すべてに
spec-drift 検出を備えています。

## クイックスタート (kind)

下記のコマンドはリリースのたびに検証されており、kind クラスターを
ブートストラップする手順がローカル開発の正規パスです。

### 1. 前提条件

| ツール | 最低バージョン | 備考 |
|---|---|---|
| Go | 1.26 | `go.mod` と一致 |
| Docker | 24+ | buildx default ビルダー |
| kind | 0.27+ | ローカルクラスター |
| kubectl | 1.34+ | k3s/kind 互換 |
| cert-manager | 1.16+ | Webhook サービング証明書 |

### 2. kind クラスター + cert-manager

```sh
make setup-test-e2e
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=120s -n cert-manager deploy --all
```

### 3. ビルド・ロード・デプロイ

```sh
make docker-build IMG=valkey-operator:dev
kind load docker-image valkey-operator:dev --name valkey-operator-test-e2e
make install                          # CRD
make deploy IMG=valkey-operator:dev   # operator + RBAC + webhook
kubectl -n valkey-operator-system rollout status deploy/valkey-operator-controller-manager
```

### 4. サンプル CR の適用

```sh
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeycluster.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeybackup.yaml
```

### 5. データプレーンの疎通確認

```sh
PASS=$(kubectl get secret valkey-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" ping     # PONG
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" set k v  # OK
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" get k    # v

# Cluster モード — `-c` は MOVED リダイレクトを自動的に追随
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

Chart は
[Artifact Hub](https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator)
にも `Signed` trust バッジ付きで公開されています (ADR-0044、ADR-0046)。

## 主な機能

- **3 トポロジーを 1 つの Operator で。** Standalone、Replication、
  Valkey Cluster はすべて単一の reconciler セットを共有し、統一された
  status 表面を持ちます。
- **Replication モードの自動フェイルオーバー** — `master_repl_offset` が
  最大の replica を選択し、`REPLICAOF NO ONE` で昇格させます (ADR-0017)。
- **バックアップ / リストア** — RDB または AOF を PVC、S3、または
  S3 互換エンドポイント (MinIO、Ceph RGW) に保存。Restore は Init
  Container パターンを使い、メインコンテナが透過的に RDB をロード
  します (ADR-0015、ADR-0016、ADR-0022、ADR-0023)。
- **TLS + mTLS** — cert-manager 自動検出 (ADR-0010、ADR-0014)、または
  ユーザー指定 `Secret`。
- **常時 auth 有効。** `Auth.Enabled` 未設定時はランダム 32-byte
  パスワードを自動生成 (ADR-0013)。
- **NetworkPolicy** — opt-in、pod 間通信を 6379/16379 に制限
  (CNI による強制)。
- **可観測性。** 22 span の OTEL トレーシング
  (`OTEL_EXPORTER_OTLP_ENDPOINT` 未設定時はオーバーヘッド 0)、
  Prometheus アラートルール、ServiceMonitor 自動生成。
- **サプライチェーン。** v1.0.13 以降、SBOM (syft SPDX) + Trivy スキャン
  + cosign keyless 署名 + SLSA-3 provenance (ADR-0046)。検証コマンド
  については [SECURITY.md](SECURITY.md) 参照。

## ドキュメント

| トピック | 場所 |
|---|---|
| 詳細な韓国語ウォークスルー | [README.ko.md](README.ko.md) |
| Runbook (バックアップ、リストア、スケーリング、アップグレード、緊急対応) | [docs/operations/runbook.md](docs/operations/runbook.md) |
| リリース前チェックリスト | [docs/operations/release-checklist.md](docs/operations/release-checklist.md) |
| Architecture Decision Records | [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md) |
| コントリビューション | [CONTRIBUTING.md](CONTRIBUTING.md) |
| セキュリティポリシー + artifact 検証 | [SECURITY.md](SECURITY.md) |
| プロジェクトガバナンス | [GOVERNANCE.md](GOVERNANCE.md) |
| 採用者 | [ADOPTERS.md](ADOPTERS.md) |

## プロダクションレディネス

本 Operator は `v1alpha1` ですが、商用製品レベルの品質システムを
備えています:

- **29 件の SSOT-parity ゲート** — alert / runbook / RBAC / CRD /
  sample / chart artifact の drift を lefthook pre-push で防止。
- **Chart-CRD 自動同期** — `make manifests` が実行し、`go mod tidy` が
  古い場合は `git push` がブロック。
- **マイクロベンチマーク** — 5 つのホットパスパーサー
  (`go test -bench=. ./internal/valkey/`) を計測。
- **Operator runbook** — 9 セクション + 各 alert ごとの
  Trigger / Diagnosis / Mitigation / Escalation。
- **サプライチェーン。** Apache-2.0 ライセンス、PGP 署名のセキュリティ
  開示、v1.0.13 以降は Helm chart + image に署名付き。
- **再利用可能な慣習** — 兄弟 Operator `mongodb-operator`、
  `postgres-operator`、`operator-commons` で共有。

## ロードマップ

ロードマップは定性的で、カレンダーコミットメントはありません。進捗は
四半期ではなく機能完了で追跡します。

リリース済み (alpha):

- ✅ Standalone / Replication / ValkeyCluster トポロジー
- ✅ PVC および S3 互換ストレージへのバックアップ
- ✅ Init Container 経由のリストア (ADR-0015)
- ✅ Replication 自動フェイルオーバー (ADR-0017)
- ✅ Prometheus アラート + runbook
- ✅ OTEL トレーシング
- ✅ Helm chart + Artifact Hub 公開

次のステップ:

- [ ] kind + MinIO 上での end-to-end 自動化
- [ ] ValkeyCluster の自動 reshard (ADR-0018)
- [ ] Replication モードの HPA 統合 (ADR-0027、v1alpha1 安定後まで延期)
- [ ] `v1beta1` 向け conversion webhook (ADR-0026、延期)
- [ ] Track A/B/E 安定化と 24 時間 soak テスト後の初の `v0.1.0` GA

意思決定の根拠は
[docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md) に格納。機能リクエストは
[Issues](https://github.com/keiailab/valkey-operator/issues) または
GitHub Discussions へ。

## 既知の制限

本ソフトウェアは `v1alpha1` で、リリースごとに検証されていますが
GA には到達していません。現在判明している注意事項:

- `Spec.Auth.Enabled=false` は no-op として扱われます — Operator は
  常に auth をプロビジョニングします (ADR-0013)。認証なしクラスター
  が必要な場合は本 Operator を使わないでください。
- IPv6 only 環境は未検証 — `CLUSTER MEET` は IPv4 ホスト名を優先
  します (ADR-0012)。
- `NetworkPolicy.Enabled` はリソースを emit するのみで、**実際の**
  強制はポリシー対応 CNI (Calico、Cilium) に依存します。
- Replication の自動フェイルオーバーはネットワーク分断下での
  split-brain に対して強い保証を提供しません — トレードオフは
  ADR-0017 参照。
- ValkeyCluster の restore はソース PVC の accessMode が
  `ReadOnlyMany` または `ReadWriteMany` を要求 — RWO は未サポート。
- `cluster-announce-hostname` は使用されません。in-cluster DNS と
  異なる方式で pod ホスト名をルーティング可能 IP に解決する
  Kubernetes-aware DNS サービスを使う場合は再検討してください。

より詳細な韓国語の一覧は
[README.ko.md → 잠재적 운영 이슈](README.ko.md#잠재적-운영-이슈-현재-알려진-한계)
にあります。

## アンインストール

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

## コントリビューション

[CONTRIBUTING.md](CONTRIBUTING.md) 参照。外部コントリビューション
歓迎。非自明な変更については、コードを書く前に API 表面の合意を
取るため、まず issue を立ててください。

すべての Makefile target は `make help` で確認できます。背景知識:
[Kubebuilder book](https://book.kubebuilder.io/introduction.html)。

## 脆弱性報告

公開 issue を**開かないで**ください。[SECURITY.md](SECURITY.md) の
プライベートチャネル — GitHub Security Advisory または
`security@keiailab.com` (PGP key は `artifacthub-repo.yml`) を
ご利用ください。

## ライセンス

Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0
(<http://www.apache.org/licenses/LICENSE-2.0>). 配布は AS IS ベースで、
種類を問わず warranty または condition なし。全文は
[LICENSE](LICENSE) を参照してください。

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
