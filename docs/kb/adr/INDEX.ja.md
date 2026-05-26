# ADR Index — valkey-operator (日本語)

> English: [INDEX.md](INDEX.md) — canonical / 正本

本ディレクトリは valkey-operator の非可逆な決定 (architecture decisions) を Nygard
5 セクション形式で保存する。決定の *理由* がコードよりも長く生き残るようにするためのものである。

パス標準: `<repo>/docs/kb/adr/` (グローバル `standards/adr.md §1`)。

## アクティブな ADR (ID 昇順)

| ID | 題目 | 状態 | 日付 |
|----|------|------|------|
| [0001](0001-operator-side-defaulting.md) | Operator-side defaulting (vs admission webhook) | 0009 によって Superseded | 2026-05-05 |
| [0002](0002-deferred-events-api-migration.md) | client-go events API への移行は延期 | Accepted | 2026-05-05 |
| [0003](0003-tls-insecure-skip-verify-temporary.md) | cert-manager CA 配線が整うまで一時的に InsecureSkipVerify | Accepted | 2026-05-05 |
| [0004](0004-shardstatus-spec-derived.md) | ShardStatus を Spec から導出 (CLUSTER NODES ではなく) | 0007 によって Superseded | 2026-05-05 |
| [0005](0005-graceful-cluster-teardown.md) | best-effort CLUSTER FORGET によるクラスタの優雅な解体 | Accepted | 2026-05-05 |
| [0006](0006-scale-policy-deliberate.md) | ScalePolicy.Deliberate=false を既定値とする | Accepted | 2026-05-05 |
| [0007](0007-shardstatus-from-nodes.md) | ShardStatus を CLUSTER NODES から導出 (0004 を Supersede) | Accepted | 2026-05-05 |
| [0008](0008-tls-ca-bundle-loading.md) | TLS RootCAs を Spec.TLS.CustomCert.SecretName から読み込む | Accepted | 2026-05-05 |
| [0009](0009-webhook-validation-defaulting.md) | Validating + Mutating Webhook (0001 を Supersede) | Accepted | 2026-05-05 |
| [0010](0010-cert-manager-auto-discovery.md) | cert-manager Certificate の自動検出 | Accepted | 2026-05-05 |
| [0011](0011-required-fields-webhook-defaulting.md) | Required フィールドは mutating webhook で直接 default を埋める | Accepted | 2026-05-05 |
| [0012](0012-cluster-meet-requires-ip.md) | CLUSTER MEET は hostname 非対応 → DNS 解決後の IP を使用 | Accepted | 2026-05-05 |
| [0013](0013-auth-always-enabled.md) | Auth.Enabled は事実上常に enabled (option A) | Accepted | 2026-05-05 |
| [0014](0014-tls-volume-mount-and-port-routing.md) | TLS Secret を STS にマウント + operator は 6380 (TLS port) で control-plane に接続 | Accepted | 2026-05-05 |
| [0015](0015-valkeyrestore-init-container-pattern.md) | ValkeyRestore — Init Container 方式の RDB ロード + STS 再起動 | Accepted | 2026-05-06 |
| [0016](0016-valkeybackuptarget-crd-external-storage.md) | ValkeyBackupTarget CRD — S3-compatible 外部ストレージの抽象化 | Accepted | 2026-05-06 |
| [0017](0017-replication-failover-replica-with-largest-offset.md) | Replication Mode Failover — master_repl_offset が最大の replica を昇格 | Accepted | 2026-05-06 |
| [0018](0018-cluster-auto-resharding.md) | Cluster Auto-Resharding (SlotMigrationPolicy Auto 有効化、PR-B8.1 で ADR 正式起草 — controller 実装は PR-B8.2 で後続) | Accepted | 2026-05-09 |
| 0019 | *Reserved (用途未定)*. | Reserved | — |
| 0020 | *Reserved (用途未定)*. | Reserved | — |
| [0021](0021-helm-chart-kubebuilder-helm-plugin.md) | Helm Chart — kubebuilder helm/v2-alpha plugin を採用 | 0024 によって Superseded | 2026-05-06 |
| [0022](0022-s3-client-library-minio-go.md) | S3 Client Library — minio-go v7 採用 (sonatype + context7 検証済み) | Accepted | 2026-05-06 |
| [0023](0023-operator-binary-subcommand-upload-download.md) | Operator binary の upload/download サブコマンド — イメージ統合 | Accepted | 2026-05-06 |
| [0024](0024-helm-chart-manual-pattern-artifacthub.md) | Helm Chart — 手書き + ArtifactHub publish パターン (3-repo 統一、0021 を Supersede) | Accepted | 2026-05-06 |
| [0025](0025-otel-tracer-provider-optional.md) | OTEL Tracer Provider — Optional、OTLP gRPC Exporter | Accepted | 2026-05-06 |
| [0026](0026-conversion-webhook-deferred-until-v1alpha1-stable.md) | Conversion Webhook — v1alpha1 Stable 到達後に v1beta1 を導入 (deferred) | Accepted | 2026-05-06 |
| [0027](0027-hpa-replication-mode-only-deferred.md) | HPA — Replication Mode 限定 + Operator-managed (impl 2026-05-10) | Accepted | 2026-05-10 |
| [0028](0028-helm-kustomize-parity-invariant.md) | Helm vs Kustomize Parity 不変条件 — 5 sibling silent failure family を遮断 | Accepted | 2026-05-06 |
| [0029](0029-gitops-deploy-overlay.md) | GitOps deploy オーバーレイを導入 (3-repo 整合) | Accepted | 2026-05-06 |
| [0030](0030-rfc-0017-tooling-unification-adoption.md) | RFC-0017 operator tooling unification 採択 (.golangci.yml 新規 + Makefile validate + HEALTHCHECK) | Proposed | 2026-05-09 |
| [0031](0031-auth-rotation-policy.md) | Password Rotation reflect path (AuthSpec.RotationPolicy enum、v1alpha2 PR-B7.1 type module — controller 分岐は PR-B7.2 で後続) | Accepted | 2026-05-09 |
| [0032](0032-custom-modules-init-container.md) | Valkey Custom Modules — init container マウント + 公式 preset のみ (v1alpha2 PR-C6.1 type module — controller 分岐は PR-C6.2 で後続) | Accepted | 2026-05-09 |
| [0033](0033-supply-chain-cosign-slsa.md) | Supply Chain — cosign sign + SLSA L2 in-toto attestation (Plan §2 D5、PR-A4) | Accepted | 2026-05-09 |
| [0034](0034-auth-optional-v1alpha2.md) | Auth Optional + v1alpha2 新設 (ADR-0013 を Supersede、PR-A2.1 type module) | Accepted | 2026-05-09 |
| [0035](0035-networkpolicy-autocreate-optional.md) | NetworkPolicy.AutoCreate Optional Toggle (v1alpha2、PR-A3.1 type module — controller 分岐は PR-A3.1.2 で後続) | Accepted | 2026-05-09 |
| [0036](0036-pod-security-restricted-optional.md) | PodSecurity Restricted Optional Toggle (v1alpha2 PodSpec.PodSecurityRestricted、PR-A3.2 type module — controller 分岐は PR-A3.2.2 で後続) | Accepted | 2026-05-09 |
| [0037](0037-operatorhub-bundle-scaffold.md) | OperatorHub.io bundle scaffold — operator-sdk v1.42 + kustomize、5 CRD owned、Makefile bundle/bundle-build ターゲット (PR-B9 first cut、alm-examples + community-operators PR は後続) | Accepted | 2026-05-10 |
| [0038](0038-rfc-0018-pkg-finalizer-migration.md) | RFC-0018 採択 — pkg/finalizer migration (controllerutil → commons、5 controller、PR-A6 first cut、status は別途) | Accepted | 2026-05-09 |
| [0039](0039-cluster-self-heal-post-init.md) | ValkeyCluster post-init self-heal — INC-0001 の恒久 fix、ClusterInitialized=true && state!=ok 時に ensureClusterMeet を再呼び出し | Accepted | 2026-05-10 |
| [0040](0040-helm-chart-vs-operator-adoption.md) | Helm chart vs Operator 採用方針 (外部 chart / 外部 chart / valkey-operator の意思決定マトリクス + 5 gap) | Accepted | 2026-05-10 |
| [0041](0041-chaos-engineering-chaos-mesh.md) | Chaos Engineering — chaos-mesh 採用 (4 シナリオ e2e、ADR-0040 §gap #4) | Accepted | 2026-05-10 |
| [0042](archive/0042-commercial-parity-series-closure.md) | Commercial Parity Series 総括 — archive (history 保存、外部 chart 本文 deprecation 理由) | Deprecated | 2026-05-10 |
| [0044](0044-artifacthub-signed-official-trust-badges.md) | Artifact Hub trust badges — Signed は必須、Official は外部レビュー待ち | Accepted | 2026-05-12 |
| [0045](0045-restore-github-actions-for-oss-ci.md) | OSS CI 向けに GitHub Actions workflows を復活 (RFC-0002 からの scoped deviation) | Accepted | 2026-05-12 |
| [0046](0046-slsa3-cosign-supply-chain.md) | release artifact (image + chart + SBOM) に SLSA-3 provenance + cosign keyless signing を適用 | Accepted | 2026-05-12 |
| [0047](0047-community-operators-sync-automation.md) | community-operators sync 自動化 (RFC 0002 例外 ③ の拡張) | Accepted | 2026-05-14 |
| [0048](0048-gha-retention-for-public-oss.md) | GitHub Actions retention — Public OSS Operator External Trust Gate (operator family ごとの trade-off) | Accepted | 2026-05-21 |
| [0050](0050-audit-augmentation.md) | Audit Augmentation — postgres パターンを cp (lefthook 3 種 + helm-publish + UPGRADING、audit P1-11/12/13 + OP-2 + OP-10 ✅) | Accepted | 2026-05-21 |
| [0051](0051-multi-arch-build-enablement.md) | マルチアーキ build の opt-in 有効化 — `PLATFORMS` env override (default は amd64 維持、ARM node 導入 + 外部 GA に備える、RFC-0048 の sister) — 重複 0043 から renumber | Proposed | 2026-05-19 |
| [0052](0052-v3x-stable-baseline.md) | v3.x-stable baseline 認定 (audit ❌ 0 件達成、CLAUDE.md §7 v3.x-stable 条件) | Accepted | 2026-05-21 |
| [0053](0053-root-md-documentation-policy.md) | Root `.md` ドキュメント方針 + ツール依存例外 (PR-D シリーズの正当化) | Accepted | 2026-05-21 |

## 起草ガイド

- 形式: Nygard 5 セクション (Context / Decision / Consequences / Alternatives Considered / Status)。
- 配置: `docs/kb/adr/NNNN-<英語 kebab-case slug>.md` (グローバル標準)。
- 番号付与: 4 桁 0-padded、一度付与した番号は *再利用禁止*。Reserved スロットは INDEX に明記する。
- 本 INDEX.md は新規 ADR 追加時に *手動更新が義務* — `standards/enforcement.md §2.1`。
- 並び順: ID 昇順 — Reserved 項目も含めて並べる (gap の可視化)。

## Reserved スロット方針

ADR 番号 0018-0020 は plans 段階で予約されたが *未起草* のまま保存する。再利用禁止原則に基づき、新しい ADR は次に空いた番号 (0030+) から付与する。Reserved スロットが起草されたら、INDEX の行を正式項目に置き換える。

## グローバル参照

- グローバル ADR 標準: `~/Documents/ai-dev/standards/adr.md`
- ADR coverage gate: `scripts/check-adr-coverage.sh` (グローバル)
- 強制標準: `~/Documents/ai-dev/standards/enforcement.md §2.1`
