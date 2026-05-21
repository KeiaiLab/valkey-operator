# Changelog (日本語)

> English: [CHANGELOG.md](CHANGELOG.md) — canonical / 正本

> 注目すべき変更すべての履歴。英語版が正本で、本ファイルは日本語の
> 運用トーンに沿った訳出版。リリースごとのサマリは GitHub Release
> ページを参照。

本プロジェクトのすべての主要な変更は、本ファイルに記録する。
形式は [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)、
バージョニングは
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) に従う。

自動生成: `git-cliff` (P1 §2.3 標準) — release tag のタイミングで
PR を自動更新する。

## [Unreleased]

## [1.0.13] - 2026-05-13

### Added

- ADR-0045: OSS CI 向け GitHub Actions workflow の部分復活 (RFC-0002
  のスコープ限定逸脱) (#89)。
- ADR-0046: release artifact (image + chart + SBOM) に対する SLSA-3
  provenance + cosign keyless 署名 (#92)。
- `.github/FUNDING.yml` — GitHub Sponsors 開示 (#91)。
- `.github/workflows/scorecard.yml` — 毎週の OpenSSF Scorecard
  解析 + SARIF アップロード (#94)。
- `.github/workflows/dependency-review.yml` — High+ CVE または
  非許可ライセンスの依存を持ち込む PR を遮断 (#94)。
- `.github/workflows/codeql.yml` — `security-extended` クエリ
  セット付き CodeQL Go SAST (#99)。
- `.github/workflows/dco.yml` — サーバ側 DCO sign-off チェック。
  ローカルの lefthook commit-msg hook と同じ規約をミラー (#99)。
- `.github/ISSUE_TEMPLATE/config.yml` — 空 issue を禁止し、
  Security Advisory / Discussions / Runbook contact のリンクを
  表に出す (#96)。
- 英語版の README / CONTRIBUTING / SECURITY / GOVERNANCE /
  MAINTAINERS / ADOPTERS を canonical に昇格。既存の日本語/韓国語
  原典は `.ko.md` 兄弟ファイルとして保存 (#93, #97, #98)。
- README に "Known limitations" セクションを新設 — SECURITY.md
  への cross-link を有効に保つため (#93 follow-up)。
- `.editorconfig` を新設 — Go (tab)、YAML / JSON / Markdown
  (2-space、trim ポリシー)、Makefile (tab 必須)、shell スクリプト
  までをまとめて統制。

### Changed

- 8 つの workflow の GitHub Actions reference をすべて commit SHA
  + 末尾のバージョン注釈で pin。OpenSSF Scorecard の
  `Pinned-Dependencies` を満たす (#95)。
- `setup-go check-latest: true` + `go.mod` で
  `toolchain go1.26.3` を採用 — stdlib CVE 修正 (1.26.2 / 1.26.3
  合計 16 件) を CI 実行のたびに自動反映 (#92)。
- `security-scan.yml` を *すべての PR* で実行するよう変更。以前は
  diff が go.mod / Dockerfile に触れた場合にのみ走っていた — diff
  の場所を理由にセキュリティスキャンが省略されることは絶対に避ける
  (#92)。
- `main` の branch protection を強化。必須 status check 7 種
  (golangci-lint, unit + envtest, build, govulncheck, trivy-fs,
  trivy-image, Review dependencies) + strict mode + linear
  history + conversation resolution + force-push / 削除の禁止
  + enforce-admins を有効化。
- リポジトリのセキュリティ系トグル: Dependency graph、Automated
  security fixes、Secret scanning、Secret-scanning push
  protection をすべて有効化。
- README の Go バッジを 1.25+ → 1.26+ (#90)。
- CONTRIBUTING.md の Go 前提条件を 1.26 に引き上げ —
  `TestGoVersionDockerfileVsGoMod` の回帰ガードを green に保つ
  ため (#84 follow-up)。
- `release.yml` の `sbom` job が `contents: read` +
  `packages: read` をあわせて宣言するように変更。syft が GHCR の
  manifest にアクセス可能なまま OIDC トークンも維持する (#92
  レビュー follow-up)。
- `cosign --certificate-identity-regexp` を、リポジトリ内の任意の
  workflow ではなく `release.yml` の workflow に厳密に限定する
  正規表現へ絞り込み (#92 レビュー follow-up)。

### Security

- 署名された image / chart / SBOM すべてに対し、v1.0.13 から
  Sigstore Rekor 透過ログのエントリが生成される。
- コンテナ image に対して `slsa-framework/slsa-github-generator`
  で SLSA-3 provenance attestation を発行 (v1.0.13 から)。
- SECURITY.md に、artifact の検証に必要な `cosign verify` /
  `slsa-verifier verify-image` の正確なコマンドおよび証明書
  identity の regex を明記。

### Dependencies

- `actions/setup-go check-latest: true` — CI の Go runtime が
  stdlib CVE 修正を自動で取り込む。
- dependabot による 7 件の更新を main へマージ:
  - Docker base: `golang 1.26.3`、`distroless/static@e3f9456`
    (#80, #81)。
  - Go modules: k8s 0.36 + controller-runtime 0.24 + utils +
    operator-commons 0.7.0 + otel グループ + ginkgo 2.28.3 +
    gomega 1.40.0 (#84–#88)。

## [1.0.12] - 2026-05-12

### Changed

- 既存の `v1.0.11` タグが未完成の release commit を指していたため、
  クリーンな Artifact Hub 再配信リリースを公開。
- 公開済み chart のメタデータをリフレッシュし、Alpine 3.23 ベース
  の Valkey runtime image と operator image tag `1.0.12` を露出。
- chart README と Artifact Hub 上の公開メタデータは英語のまま維持。

### Fixed

- 最終 release commit より前に誤って混入した、禁止対象の GitHub
  Actions workflow ファイル群を削除。

## [1.0.11] - 2026-05-12

### Changed

- Artifact Hub の chart メタデータをリフレッシュ — 公開パッケージ
  が Alpine 3.23 ベースの Valkey runtime image を露出するように修正。
- 現在の release surface 向けに chart / app バージョン `1.0.11` を
  公開。
- Artifact Hub の trust-badge ドキュメントは英語のまま維持。

### Fixed

- いまだに `docker.io/valkey/valkey:9.0.4` を案内していた古い Helm
  リポジトリのパッケージを正す。

## [1.0.10] - 2026-05-10

### Added

- OperatorHub.io bundle 雛形 + ADR-0037 (PR-B9 first cut, #21)。
- `alm-examples` のインライン JSON サンプル 5 件追加 (PR-B9.2,
  #22)。
- CITATION.cff を追加 (OSS メタデータ, #20)。

### Changed

- Chart を v1.0.10 に bump + amd64-only build (CLAUDE.md §2 整合,
  #27)。
- `v1alpha2 zz_generated.deepcopy.go` を再生成 (controller-gen
  同期, cfd0398)。
- bundle: `generate-kustomize-manifests` ステップを削除 (PR-B9.4,
  mongodb ADR-0023 整合, #23)。

### Fixed

- `ValkeyCluster` post-init self-heal — INC-0001 の恒久 fix
  (ADR-0039, #25)。
- v1alpha1 に `storageversion` マーカーを追加 + controller-gen 再生成
  (PR-A2.2.5, #19)。
- `ReadOnlyRootFilesystem=true` を有効化 — modern security
  baseline の最後のレイヤー (3aa5480)。

### Docs

- INC-0001 ValkeyCluster bootstrap skip — 運用クラスタが 19 時間
  fail 状態だった事象からの復旧記録 (#24)。
- INC-0001 `cluster_state=fail` 復旧 runbook + ADR-0039 self-heal
  明記 (AI-0004, #26)。
- ADR-0026 の部分復旧の進捗を明記 (PR-A2.2.* 累積, #18)。
- HANDOFF を PR-A2.2.5 マージ結果と次の進入点で更新
  (1818031)。

## [1.0.9] - 2026-05-10

### Added

- v1alpha2 type 定義モジュール + `AuthSpec.Required` toggle
  (PR-A2.1, #6)。
- v1alpha2 Hub マーカー 5 type を追加 (PR-A2.2.1, #15)。
- v1alpha1 の `ConvertTo` / `ConvertFrom` を 5 type 分実装
  (PR-A2.2.2, #16)。
- `cmd`: v1alpha2 SchemeBuilder の登録 (PR-A2.2.3.a, #17)。
- Valkey Custom Modules type を追加 (v1alpha2, PR-C6.1,
  ADR-0032, #14)。
- `AuthSpec.RotationPolicy` の enum を追加 (v1alpha2, PR-B7.1,
  ADR-0031, #12)。
- `PodSecurity` Restricted の任意トグル (v1alpha2, PR-A3.2,
  ADR-0036, #10)。
- `NetworkPolicy.AutoCreate` の任意トグル (v1alpha2, PR-A3.1,
  ADR-0035, #9)。
- RFC-0018 の `pkg/finalizer` 移行 (controller, PR-A6 first cut,
  ADR-0038, #8)。
- release: cosign 署名 + SLSA L2 in-toto attestation + ADR-0033
  (PR-A4, #5)。

### Changed

- operator-commons v0.5.0 → v0.6.0 — RFC-0018 の `SetAvailable`
  + `SetReadyFalse` が使えるように (#7)。

### Docs

- ADR-0018 を正式版として追加 — Cluster Auto-Resharding (PR-B8.1,
  #13)。
- Sentinel migration runbook を新設 (PR-C7, ADR-0017 否決の補強,
  #11)。

## [1.0.8] - 2026-05-09

### Fixed

- `monitoring.exporter.resources` が metrics sidecar まで
  reconcile されていなかった運用統合時点の不具合を修正
  (1eb6faf):
  - `STSParams.ExporterResources corev1.ResourceRequirements`
    という新フィールド (internal/resources/statefulset.go)。
  - `BuildStatefulSet` の metrics container に
    `p.ExporterResources` を適用。
  - Valkey + ValkeyCluster controller が
    `exporterResources(spec.Monitoring)` ヘルパー経由で値を渡す。
  - 空の ResourceRequirements (default) → K8s Burstable QoS。
    以前の挙動と等価な互換性を維持。

### Changed

- Chart を 1.0.7 に bump (8408005)。

## [1.0.7] - 2026-05-09

### Changed

- audit (4-repo cross-cut, 2026-05-09): RFC-0017 採択 —
  `.golangci.yml` + `.custom-gcl.yml` を新設 (postgres 標準
  cp + depguard の整理)、Makefile に `validate` ターゲットを追加
  (kustomize + helm lint + helm template)。ADR-0030 を登録。本
  リポジトリの `.lefthook.yml` は RFC-0017 §3.1 の標準原本に昇格
  (変更なし, 0aea740)。
- operator-commons v0.4.0 → v0.5.0 (4833f13)。
- `.codecov.yml` を新設 — 4-repo の target 70% 絶対 floor を統一
  (d381587)。

### Fixed

- `.golangci.yml` の depguard を一時的に無効化 (golangci-lint
  v2.8 schema が空の deny list を拒絶) — valkey の internal
  boundary 導入後に ADR とともに再有効化予定。17 種の linter を
  有効化 (logcheck plugin を含む, 9dae535)。
- lint: valkey 残 37 件で lint 0 issue を達成 (goconst 17 +
  unparam 17 + gocyclo 3, 8ba60a7)。
- lint: lll / prealloc / revive を 5 件修正 (安全な cleanup,
  5d16c94)。
- lint: modernize 20 + copyloopvar 2 件を自動 fix
  (`slices.Contains` ほか, 8820460)。

### Docs

- CHANGELOG エントリと deps log (audit 仕上げ, bd667ad)。

## [1.0.6] - 2026-05-08

### Added

- TLS の `clientAuth` フィールドを新設 — required / optional /
  disabled の mTLS トグル (0c804c9)。
- renovate: auto-update PR の入口 (Go modules + image tag,
  ba3c9af)。

## [1.0.5] - 2026-05-08

### Fixed

- Artifact Hub が `1.0.4` chart の icon
  `https://valkey.io/img/Valkey-Logo-RGB-Color.svg` を取得しよう
  として 404 となり、tracking warning が発生していた問題を修正。
  現在 Valkey サイトで 200 を返す
  `https://valkey.io/img/valkey-horizontal.svg` に差し替え。

## [1.0.4] - 2026-05-08

### Added

- Service builder: TLS 有効時に client-tls (6380) port を公開
  する。BuildClientService + BuildHeadlessService に tlsEnabled
  引数を追加。外部 client は `rediss://` スキームで connect 可能
  (`tls-auth-clients=yes` の場合、client cert は別途発行が必要 —
  本パッチは server-side TLS の外部公開インフラのみを扱う)。

## [1.0.1] - 2026-05-07

### Fixed

- `ValkeyBackup` が `ValkeyCluster` 対象に対して 1 つ目の pod の
  `dump.rdb` のみを保存していた問題を修正。shard ごとの primary
  pod を基準に `shard-N/dump.rdb` 構造を生成し、cluster restore の
  既定 shard layout と直接互換になった。
- `ValkeyRestore` が `ValkeyCluster` 対象の復元中に pause /
  unpause annotation を `Valkey` CR にしか適用していなかった問題を
  修正。
- multi-pod restore における既存 source PVC の検証で、
  `ReadOnlyMany` だけでなく、read-only mount 可能な
  `ReadWriteMany` PVC も許可する。

## [1.0.0] - 2026-05-07

### Added

- 初の stable リリース。Valkey `9.0.4` を既定値とし、`8.0.9` /
  `8.1.7` milestone との互換、`ValkeyCluster` の sharded HA、
  自動 failover、`ValkeyBackup`、`ValkeyRestore`、
  `ValkeyBackupTarget`、restricted PodSecurity の既定有効化、
  `linux/amd64` / `linux/arm64` の multi-arch operator image を
  同梱する。

## [0.1.0-alpha.5] - 2026-05-07

### Fixed

- **Runtime P0 — restricted PodSecurity の namespace で Valkey
  Pod の作成に失敗** (`internal/resources/statefulset.go`):
  Valkey StatefulSet のコンテナが
  `allowPrivilegeEscalation=false`、`capabilities.drop=[ALL]`、
  `seccompProfile.type=RuntimeDefault` のデフォルトを持っていな
  かったため、`data-staging` namespace で Pod 作成が拒否されていた。
  既定の Valkey コンテナと metrics sidecar に restricted な
  SecurityContext を注入した。

## [0.1.0-alpha.4] - 2026-05-07

### Fixed

- **Release P0 — operator image の build metadata 欠落**
  (`Makefile`):
  `make release` が Dockerfile の `VERSION` / `COMMIT` /
  `BUILD_DATE` build args を渡しておらず、実配信 image の
  `/manager --version` と `valkey_cluster_build_info` が
  `dev/none/unknown` で露出していた。release target で tag、git
  commit、UTC build date を注入するように修正。
- **Release P0 — chart affinity と image platform の不一致**
  (`Makefile`): chart の既定 affinity は `linux/amd64` と
  `linux/arm64` の両方を許可するが、release image は
  `linux/amd64` のみを push していた。release build を
  `linux/amd64,linux/arm64` の multi-arch に変更。

### Added

- release target が build metadata と multi-arch platform を強制
  していることを確かめる回帰テストを追加。

## [0.1.0-alpha.3] - 2026-05-07

### Added

- Valkey latest default の整列: API default、CRD default、Helm
  values、ArtifactHub の examples / images、samples、GitOps の
  workload CR を `9.0.4` に更新。
- `SupportedValkeyVersions` whitelist を `8.0.9`、`8.1.6`、
  `8.1.7`、`9.0.4` で明示し、最新 + 8.0 / 8.1 milestone patch との
  互換基準を文書化。
- ValkeyCluster 9.0.4 sharded 3x1 の Kind smoke: 6 pod が Ready、
  `cluster_state=ok`、16384 slots、SET / GET を確認。

### Fixed

- Redis 8.2.x の RDB を Valkey 9.0.4 に直接 restore する際、RDB
  format の不一致で CrashLoopBackOff となる経路を、
  `ValkeyRestore.status.phase=Failed` で fail-fast する処理に
  置き換え。

## [0.1.0-alpha.2] - 2026-05-07

ADR-0057 の Phase A1 (運用クラスタ事前配備) の途中で発見した
chart RBAC の不具合を修正。

### Fixed
- **chart RBAC P0 — `features.{cluster,backup}.enabled=false` の
  ときに informer startup が失敗**
  (`charts/valkey-operator/templates/clusterrole.yaml`):
  従来 chart は `features.cluster.enabled` /
  `features.backup.enabled` を条件として
  `valkeyclusters` / `valkeybackups` / `valkeybackuptargets` /
  `valkeyrestores` の RBAC を付与していた — しかし operator manager
  (`cmd/main.go`) は *常に* 全 controller を登録するため、
  flag=false の状況だと informer が `forbidden` で startup に失敗
  していた。RBAC とコードの mismatch は production-grade では決定
  的な阻害要因になる。RBAC を *常にすべての CRD 権限を付与* する
  単純化に揃え、feature flag は controller のコード側でのみ扱う形に
  した。

### Verified (運用クラスタの Phase A1 + A2)
- valkey-operator pod 1/1 Running、Certificate / Issuer /
  ValidatingWebhookConfiguration が Ready。
- Valkey CR `valkey-test` (Standalone、valkey 8.1.6、1Gi
  ceph-rbd) が 1/1 Running。
- SET / GET smoke:
  `SET phase-a2-smoke "OK-2026-05-07"` → `OK`、`GET` → 正常
  round-trip。
- `INFO server`: valkey_version=8.1.6、tcp_port=6379。

### Refs
- ADR-0057 (インフラ bootstrap 43fd542): self-hosted
  valkey-operator 採用ロードマップ。
- 運用障害分析 + Phase A 進捗: keiailab/mongodb-operator
  HANDOFF.md (2026-05-07)。

### Added (GitOps deploy 整合)

- `deploy/overlays/prod/` という GitOps の進入点を追加 —
  `config/{crd,rbac,manager}` を prod ns へ整列させ、自動生成された
  Namespace を除去。ArgoCD による単方向同期を前提とする。
- `deploy/valkey-cluster.yaml` — production 向けの ValkeyCluster
  サンプル (db ns、shards=3、replicasPerShard=1、ceph-block、
  auth.enabled=true)。
- `deploy/README.md` — 運用ランブック。
- ADR-0029 — GitOps deploy オーバーレイの導入 (mongodb-operator
  / postgresql-operator との 3-repo 整合)。

### Added (cycles 20-90 — Quality システム + production-grade UX)

**Quality システム (SSOT ゲート 39 種)**:
- ADR governance (4 ゲート): file / INDEX / Status / Superseded /
  Nygard の 3-section。
- Alert rules (4): schema / fields / metric / runbook anchor の
  同期。
- RBAC (2 方向): `kubebuilder:rbac` ↔ `role.yaml`。
- Sample CR (3): strict unmarshal + dir-mapping + metadata。
- ClusterRef.Kind (2 — 3-way): enum ↔ switch case。
- LICENSE + Chart annotation (2)。
- Chart artifacts (6): images / CRDExamples / CRD sync / values /
  NOTES / README YAML。
- Markdown link + anchor (2)。
- Webhook + Reconciler 登録 (2)。
- `dist/install.yaml` (2): 構造 + `OPERATOR_IMAGE` env。
- Release-checklist の self-sync (1、双方向 cycle 60)。
- Kustomize ↔ chart sync family (3): resources / probes /
  securityContext。
- Cross-feature interaction family (3): NP + webhook / tracing /
  backup。
- `features.*` の RBAC + reconciler 同期 (1)。
- value ↔ template binding (1)。
- chart args ↔ operator flag (1)。

**自動化 (ミスの発生自体を遮断)**:
- `make manifests` が chart CRD を自動同期。
- pre-push lefthook の 6-hook (full-lint + gitleaks +
  go-mod-tidy + helm-lint + helm-template + unit-test)。
- `make sbom` (syft SPDX) + trivy post-scan を release pipeline
  に自動添付。

**Production-grade UX**:
- ldflags chain (cycles 53-57): `cmd/main.go` → Dockerfile →
  `docker-build` → `docker-buildx` → `release.sh` → Prometheus
  `build_info` gauge。
- chart features 5 種 (cycles 65 / 72 / 73 / 74 / 82):
  tracing + NetworkPolicy + webhook + watch.namespaces +
  autoscaling まで正直に表示。
- 6-layer documentation: README + chart README + NOTES.txt +
  CONTRIBUTING + release-checklist + HANDOFF (ユーザ役割ごとの
  entry point を網羅)。
- runbook §7.1 — 環境変数の診断ガイド。
- 3-layer DX: lefthook auto + `make ssot-check` (1.4s) +
  `make gate` (30s)。

**実装された機能 (cycles 72-74 — chart 4 種の未使用 value のうち
3 種を解消)**:
- `charts/valkey-operator/templates/networkpolicy.yaml` —
  operator pod の default-deny。
- `charts/valkey-operator/templates/webhook.yaml` — cert-manager
  に依存する admission webhook。
- `WATCH_NAMESPACES` env — namespace-scoped watch
  (`cache.DefaultNamespaces`)。

**実装された機能 (cycles 99-106 — kubebuilder boilerplate の
completion + Helm parity)**:
- cycle 100 — runbook §7.0 production TLS 強化ガイド
  (`insecureSkipVerify` → cert-manager)。
- cycle 101 — `config/manager` + chart values の nodeAffinity
  (amd64 + arm64 + linux) — mixed-arch の ImagePullBackOff を
  防止。
- cycle 102 — `config/default/kustomization.yaml` の
  `- ../prometheus` を有効化 — kustomize ユーザにも ServiceMonitor
  + PrometheusRule が自動でインストールされる。
- cycle 103 — `charts/.../prometheusrule.yaml` — Helm ユーザの
  10 alerts の silent loss を防止。
- cycle 104 — `charts/.../metrics-auth-rbac.yaml` — secure
  metrics で Prometheus が 401 で silent fail するのを防止。
- cycle 106 — `charts/.../deployment.yaml` の webhook serving
  config (`--webhook-cert-path` + 9443 + cert mount) —
  webhook を有効化したときに operator が 9443 で正しく listen
  するようになった。

**production gap の発見 · 修正 (27 件)** + **内部負債の cleanup
(3 件)** + **hot-path benchmark 5 種** + **8 つの欠陥 family の
progressive completion**。

### Added (iter 7+ — bootstrap · 検証サイクル)
- README quickstart (kind ベース): 5 ステップ bootstrap + データ
  plane の smoke + 運用シナリオのマトリクス。 [iter 6]
- ADR-0011: Required フィールド (omitempty 不在) に対する
  mutating webhook defaulting パターン。 [iter 4]
- ADR-0012: CLUSTER MEET が hostname を受け付けない → DNS 解決
  後 IP を使う。 [iter 4]
- ADR-0013: `Auth.Enabled` を強制 true に (オプション A 採用)。
  [iter 5]
- `internal/valkey/cluster.go::resolveAddrIP`: hostname → IP の
  正規化 (IPv4 を優先)。
- `internal/webhook/v1alpha1/valkey_webhook.go`: Version +
  `Auth.Enabled` の正規化。
- `internal/webhook/v1alpha1/valkeycluster_webhook.go`: Shards /
  ReplicasPerShard / Version / Auth の defaulting。
- `api/v1alpha1/common_types.go`: `DefaultValkeyVersion` /
  `DefaultValkeyImage` の定数。
- `internal/controller/valkeycluster_controller.go`: pods RBAC
  を追加 (status reconciliation 用)。
- `config/samples/cache_v1alpha1_valkeybackup.yaml`: 意味のある
  ClusterRef を埋める。
- `.dockerignore`: `*.tmpl`、`*.lua`、`*.sh` のパターン — embed
  資産を保護。
- lefthook を有効化 (pre-commit + pre-push + commit-msg) +
  Conventional Commits パターン。

### Fixed (iter 7+)
- ValkeyBackup controller のテスト fixture で ClusterRef が抜け
  ており、webhook validation を通過しなかった件。
- ValkeyCluster bootstrap の無限 retry: CLUSTER MEET が hostname
  を拒否 → DNS 解決で吸収。
- defaulting webhook が required フィールド (Version / Shards /
  ReplicasPerShard) を埋めなかったため、無限 reconcile ループに
  陥っていた問題。
- pods RBAC の欠落で ValkeyCluster の status を更新できなかった
  問題。
- lefthook の commit-msg が `$1` でなく `{1}` を使っていた件。
- lefthook の golangci-lint が cross-directory の staged files で
  エラーになっていた件。

### Verified (iter 7+ 実測)
- e2e suite: 5/5 PASS (manager 起動、metrics endpoint、
  cert-manager、mutating / validating webhook の CA injection)。
- integration test: 14 ケース PASS (実 valkey:8 コンテナ + 6
  ノードクラスタの bootstrap)。
- unit test: 4 パッケージ PASS
  (`internal/{controller,resources,valkey,webhook}`)。
- 回復性: primary pod の force kill → STS 再生成 → operator の
  再 promote → データ保持 (canary `preserved`)。
- scale up / down: 3 → 5 → 2、`master_link_status:up`、データ
  保持。
- クラスタモード: 3 shards × 2 instances、`cluster_state:ok`、
  `slots:16384/16384` OK。
- TLS + mTLS クラスタ (cert-manager + selfsigned ClusterIssuer):
  Phase=Running、slots=16384/16384 OK、データ plane の SET /
  GET 成功 (cluster mode `-c`、複数 shard 分散)。
- NetworkPolicy リソースの整合性: deny-by-default + selfPeer
  ingress (6379) + ownerReferences (Standalone)。cluster mode
  時は 16379 を追加。強制動作の検証には Calico / Cilium CNI が
  必要 (kindnet は非対応)。
- operator metrics endpoint (HTTPS:8443、ServiceAccount トークン
  認証): `controller_runtime_*` メトリクスを正常に露出。
  カスタムの `valkey_cluster_*` メトリクスは ValkeyCluster
  reconcile 時に emit される。

### Added (iter 1-6 — 以前のサイクル)
- ValkeyCluster Reconcile を 14 ステップで実装 (cluster mode CRD
  bootstrap → CLUSTER MEET / ADDSLOTS / REPLICATE → status
  polling)。 [iter 1]
- `internal/valkey/cluster.go`: `CreateCluster` を段階ごとに
  冪等分割 (`ensureMeet` / `ensureSlots` / `ensureReplicas`)。
  partial-state からの復旧を可能に。 [iter 2]
- `internal/valkey/nodes.go`: `CLUSTER NODES` 応答パーサ
  (`NodeView`、`SlotRange`)。 [iter 2]
- 統合テスト (`//go:build integration`): 実 valkey:8 コンテナ 6
  ノードクラスタ — 4 シナリオ PASS。 [iter 2-4]
- Finalizer の graceful cleanup: `gracefulClusterTeardown`
  (best-effort の `CLUSTER FORGET`、30s timeout)。 [iter 2]
- Prometheus metrics: 7 系列 (`state_ok`、`assigned_slots`、
  `shards`、`ready_replicas`、`reconcile_total`、
  `reconcile_errors_total`、`phase`)。 [iter 3]
- `ScalePolicy.Deliberate` のガード: 未同意のときは
  `Status.PendingScale` を記録 + STS の replicas を保持。
  [iter 3]
- ServiceMonitor (`monitoring.coreos.com/v1` の unstructured) を
  自動生成 + metrics Service を分離。 [iter 3]
- AutoFailover を ConfigMap directive で統合
  (`cluster-replica-no-failover yes`)。 [iter 3]
- `make integration-test` Makefile target。 [iter 2]
- `buildShardStatusFromNodes`: CLUSTER NODES ベースの ShardStatus
  (failover を正確に反映、ADR-0007)。 [iter 4]
- TLS RootCAs のロード:
  `Spec.TLS.CustomCert.SecretName.ca.crt` → x509 CertPool
  (ADR-0008)。 [iter 4]
- Validating + Mutating Webhook (両 CRD): 8 組み合わせ検証 +
  immutable ガード (Mode、Storage、TLS toggle)。 [iter 5]
- ShardStatus の pod ordinal マッピング: `buildPodAddrMap`
  (K8s Pod list → "vk-N")。 [iter 5]
- cert-manager Certificate の自動生成:
  `Spec.TLS.CertManager.IssuerRef` を明示すると Certificate CR を
  自動 + secretName も自動検出 (ADR-0010)。 [iter 6]
- Version upgrade detection: `decidePhase` が
  `Spec.Version != Status.Version` を検知して Phase=Upgrading に
  する。 [iter 6]
- ADR 0001-0010 (10 件、2 件 supersede): defaulting → webhook
  (0001 → 0009)、ShardStatus spec → NODES (0004 → 0007)、TLS の
  段階的統合 (0003 → 0008 → 0010)。 [iter 1-6]
- lefthook の設定 (`.lefthook.yml`)。 [iter 3]

### Changed
- `valkey/replication.go`: `SlaveOf` → `ReplicaOf` (Redis 5.0+
  で deprecated API)。modern Valkey の `role:replica` 認識を追加。
  [iter 1]
- `valkeycluster_controller.go`: `pollClusterState` が全ノード
  fallback (`queryAnyNode`) — pod-0 SPOF を解消。 [iter 1]
- `dialPod` が `Spec.TLS.Enabled` を渡すように (以前は無視されて
  いた)。 [iter 1]
- SetupWithManager: `Owns(PDB, NetworkPolicy)` を追加 — drift
  検知。 [iter 1]

### Fixed
- コンパイルエラー: `&appsv1StatefulSet{}.s` →
  `(&appsv1StatefulSet{}).Inner()` (Go の struct literal の
  addressability)。 [iter 1]
- `ensureReplicas` に gossip 収束 retry を追加 —
  `replicateWithRetry` (10 回の backoff、"Unknown node" を吸収)。
  [iter 2]
- `parseReplicationInfo` が modern Valkey の `role:replica` を
  認識するように — 以前は reconcile のたびに ReplicaOf を再呼出し
  していた (冪等性の欠陥)。 [iter 1]

### Documentation
- ADR インデックス (`docs/kb/adr/INDEX.md`)。
- 本 CHANGELOG。 [iter 3、iter 6 更新]

### Test Coverage Snapshot (iter 6 末)
- `internal/controller`: 50.5%。
- `internal/resources`: 33%+。
- `internal/valkey`: 33.7%。
- **`internal/webhook/v1alpha1`: 80.7%** (新規パッケージ)。
- 単体テスト: 60 件以上。
- 統合テスト: 4 シナリオ (実 Valkey 6 ノード)。

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
