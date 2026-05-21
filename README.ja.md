# valkey-operator (日本語)

> English README: [README.md](README.md) — canonical / 正本
>
> 한국어 README: [README.ko.md](README.ko.md)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org/)
[![Valkey](https://img.shields.io/badge/Valkey-8.0+-FF4438?logo=redis)](https://valkey.io/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.26+-326CE5?logo=kubernetes)](https://kubernetes.io/)
[![Container Image](https://img.shields.io/badge/ghcr.io-keiailab%2Fvalkey--operator-blue?logo=github)](https://github.com/keiailab/valkey-operator/pkgs/container/valkey-operator)
[![Helm Chart](https://img.shields.io/badge/dynamic/yaml?url=https://raw.githubusercontent.com/keiailab/valkey-operator/main/charts/valkey-operator/Chart.yaml&label=helm%20v)](https://keiailab.github.io/valkey-operator)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/keiailab-valkey-operator)](https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/keiailab/valkey-operator/badge)](https://scorecard.dev/viewer/?uri=github.com/keiailab/valkey-operator)
[![GitHub Discussions](https://img.shields.io/github/discussions/keiailab/valkey-operator?label=discussions&logo=github)](https://github.com/keiailab/valkey-operator/discussions)

Kubebuilder ベースの Kubernetes Operator です。[Valkey](https://valkey.io/) (Redis の BSD-3 フォーク) の 3 つの運用トポロジーを、統一された CRD 表面の下にある単一のコントローラーセットで管理します。

| CRD | 用途 | トポロジー |
|---|---|---|
| `Valkey` | 単一インスタンス、または 1 primary + N replica | Standalone / Replication |
| `ValkeyCluster` | シャーディングされた Valkey Cluster (16384 スロット) | 3+ shards × (1 primary + 0〜5 replicas) |
| `ValkeyBackup` | 一回限りの RDB または AOF バックアップ | PVC (`<backup>-backup`)、外部ストレージはオプション |
| `ValkeyBackupTarget` | S3 互換外部ストレージの抽象化 | Backup と Restore 間で共有 (ADR-0016) |
| `ValkeyRestore` | Valkey または ValkeyCluster インスタンスへの RDB リストア | Init Container パターン (ADR-0015) |

Operator は `StatefulSet`、`ConfigMap`、`Secret`、`Service` (headless + ClusterIP)、`PodDisruptionBudget`、`NetworkPolicy`、`cert-manager` の `Certificate`、Prometheus `ServiceMonitor` を調整します — すべてにおいて spec ドリフト検出を備えています。

## クイックスタート (kind)

以下のすべてのコマンドは各リリースで検証されています。kind クラスターのブートストラップは正本となるローカル開発パスです。

### 1. 前提条件

| ツール | 最小バージョン | 備考 |
|---|---|---|
| Go | 1.26 | `go.mod` と一致 |
| Docker | 24+ | buildx default builder |
| kind | 0.27+ | ローカルクラスター |
| kubectl | 1.34+ | k3s / kind 互換 |
| cert-manager | 1.16+ | Webhook 用 serving 証明書 |

### 2. kind クラスター + cert-manager

```sh
make setup-test-e2e
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=120s -n cert-manager deploy --all
```

### 3. ビルド、ロード、デプロイ

```sh
make docker-build IMG=valkey-operator:dev
kind load docker-image valkey-operator:dev --name valkey-operator-test-e2e
make install                          # CRDs
make deploy IMG=valkey-operator:dev   # operator + RBAC + webhook
kubectl -n valkey-operator-system rollout status deploy/valkey-operator-controller-manager
```

### 4. サンプル CR を適用

```sh
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeycluster.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeybackup.yaml
```

### 5. データプレーンの動作確認

```sh
PASS=$(kubectl get secret valkey-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" ping     # PONG
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" set k v  # OK
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" get k    # v

# Cluster モード — `-c` は MOVED リダイレクトを自動的に追跡
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

このチャートは [Artifact Hub](https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator) にも `Signed` トラストバッジ付きで公開されています (ADR-0044、ADR-0046)。

## 主な機能

- **3 つのトポロジーを 1 つの Operator で。** Standalone、Replication、Valkey Cluster はすべて単一の reconciler セットを共有し、ステータス表面も統一されています。
- **自動フェイルオーバー** (Replication モード) — `master_repl_offset` が最大のレプリカを選出し、`REPLICAOF NO ONE` で昇格します (ADR-0017)。
- **バックアップ / リストア** — RDB または AOF を PVC、S3、または任意の S3 互換エンドポイント (MinIO、Ceph RGW) へ。リストアは Init Container パターンを使用するため、メインコンテナは透過的に RDB をロードします (ADR-0015、ADR-0016、ADR-0022、ADR-0023)。
- **TLS + mTLS** — cert-manager の自動検出 (ADR-0010、ADR-0014) またはユーザー提供の `Secret` 経由。
- **常時オン認証。** `Auth.Enabled` が未設定の場合、ランダムな 32 バイトのパスワードが生成されます (ADR-0013)。
- **NetworkPolicy** — opt-in、Pod 間トラフィックを 6379 / 16379 に制限 (CNI で強制)。
- **可観測性。** 22 スパンを持つ OTEL トレーシング (`OTEL_EXPORTER_OTLP_ENDPOINT` が未設定の場合はオーバーヘッドゼロ)、Prometheus アラートルール、ServiceMonitor の自動生成。
- **サプライチェーン。** v1.0.13 以降、SBOM (syft SPDX) + Trivy スキャン + cosign keyless 署名 + SLSA-3 provenance (ADR-0046)。検証コマンドは [SECURITY.md](SECURITY.md) を参照してください。

## ドキュメント

| トピック | 場所 |
|---|---|
| 韓国語による詳細ウォークスルー | [README.ko.md](README.ko.md) |
| Runbook (Backup、Restore、Scaling、Upgrade、Emergency) | [docs/operations/runbook.md](docs/operations/runbook.md) |
| リリース前チェックリスト | [docs/operations/release-checklist.md](docs/operations/release-checklist.md) |
| Architecture Decision Records | [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md) |
| コントリビュート | [CONTRIBUTING.md](CONTRIBUTING.md) |
| セキュリティポリシーとアーティファクト検証 | [SECURITY.md](SECURITY.md) |
| プロジェクトガバナンス | [GOVERNANCE.md](GOVERNANCE.md) |
| 採用者 | [ADOPTERS.md](ADOPTERS.md) |

## プロダクションレディネス

本 Operator は `v1alpha1` ですが、商用製品レベルの品質システムを備えています:

- **29 個の SSOT パリティゲート** — アラート / runbook / RBAC / CRD / サンプル / チャートアーティファクトのドリフトを lefthook の pre-push でブロック。
- **チャート - CRD 自動同期** — `make manifests` で実行。`git mod tidy` が古い状態だと `git push` がブロックされます。
- **マイクロベンチマーク** — ホットパスの 5 パーサに対して (`go test -bench=. ./internal/valkey/`)。
- **Operator runbook** — 9 セクションに加え、アラートごとに Trigger / Diagnosis / Mitigation / Escalation を整備。
- **サプライチェーン。** Apache-2.0 ライセンス、PGP 署名済みのセキュリティ開示、v1.0.13 以降は署名済み Helm チャート + イメージ。
- **再利用可能な規約** — 兄弟 Operator (`mongodb-operator`、`postgres-operator`、`operator-commons`) と共有。

## ロードマップ

以下のロードマップは定性的なものです — カレンダー上の約束はありません。進捗は機能の完了度で追跡し、四半期では区切りません。

すでに出荷済み (alpha):

- ✅ Standalone / Replication / ValkeyCluster トポロジー
- ✅ PVC および S3 互換ストレージへのバックアップ
- ✅ Init Container 経由のリストア (ADR-0015)
- ✅ Replication 自動フェイルオーバー (ADR-0017)
- ✅ Prometheus アラート + runbook
- ✅ OTEL トレーシング
- ✅ Helm チャート + Artifact Hub への公開

次の予定:

- [ ] kind + MinIO 上のエンドツーエンド自動化
- [ ] ValkeyCluster の自動リシャーディング (ADR-0018)
- [ ] Replication モードへの HPA 統合 (ADR-0027、v1alpha1 安定化まで保留)
- [ ] `v1beta1` 向け conversion webhook (ADR-0026、保留)
- [ ] Track A/B/E 安定化と 24 時間 soak テスト後の最初の `v0.1.0` GA

意思決定の根拠は [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md) にあります。機能要望は [Issues](https://github.com/keiailab/valkey-operator/issues) または GitHub Discussions に投稿してください。

## 既知の制限

本ソフトウェアは `v1alpha1` であり、リリースごとに検証されてはいるものの、まだ GA ではありません。現時点の既知の注意事項は以下のとおりです:

- `Spec.Auth.Enabled=false` は no-op として扱われます — Operator は常に認証をプロビジョニングします (ADR-0013)。認証なしのクラスターが必要であれば、本 Operator をデプロイしないでください。
- IPv6 のみの環境はテストされていません。`CLUSTER MEET` は IPv4 のホスト名を優先します (ADR-0012)。
- `NetworkPolicy.Enabled` は単にリソースを発行するだけです。*実際の* 強制は、ポリシー対応の CNI (Calico、Cilium) に依存します。
- Replication の自動フェイルオーバーは、ネットワークパーティション下での強い split-brain 保証を提供しません — トレードオフは ADR-0017 を参照してください。
- ValkeyCluster の リストアは、ソース PVC の accessMode として `ReadOnlyMany` または `ReadWriteMany` が必要です。RWO は未サポートです。
- `cluster-announce-hostname` は使用しません。Operator がすでに使用しているクラスター内 DNS とは異なる方法で Pod ホスト名をルーティング可能な IP に解決する Kubernetes 対応 DNS サービスを運用する場合は、再検討してください。

韓国語による詳細な一覧は [README.ko.md → 잠재적 운영 이슈](README.ko.md#잠재적-운영-이슈-현재-알려진-한계) にあります。

## アンインストール

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

## コントリビュート

[CONTRIBUTING.md](CONTRIBUTING.md) を参照してください。外部からのコントリビュートを歓迎します。非自明な変更については、コードを書く前に API 表面のすり合わせができるよう、まず Issue を開いてください。

すべての Makefile ターゲットを確認するには `make help` を実行してください。背景情報としては [Kubebuilder book](https://book.kubebuilder.io/introduction.html) を参照してください。

## 脆弱性の報告

公開 Issue を **開かないでください**。[SECURITY.md](SECURITY.md) の非公開チャネル — GitHub Security Advisory または `security@keiailab.com` (PGP 鍵は `artifacthub-repo.yml`) を利用してください。

## ライセンス

Copyright 2026 Keiailab.

Apache License, Version 2.0 のもとでライセンスされています (<http://www.apache.org/licenses/LICENSE-2.0>)。本ソフトウェアは "AS IS" ベースで提供され、明示または黙示を問わずいかなる保証も条件も付与されません。全文は [LICENSE](LICENSE) ファイルを参照してください。
