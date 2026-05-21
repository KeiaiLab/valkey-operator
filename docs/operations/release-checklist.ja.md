# Release Checklist — valkey-operator (日本語)

> English: [release-checklist.md](release-checklist.md) — canonical / 正本

新規 release tag (`v0.1.0` など) を push する直前に本チェックリストを通過させれば、
release グレードの品質が保証される。**各項目** は自動ゲートで担保されており、本書は
*体系を可視化するための一覧* であって、強制点そのものではない。

自動化の SSOT: `scripts/release.sh` (手動エントリポイント) + `make gate` +
lefthook の pre-push hook。本書を読むのは *仕組みを把握するため* であり、実際に
block しているのはゲート自体である。

## 1. ビルドとコード品質 (自動)

- [ ] `make lint` — 0 issues (golangci-lint)。
- [ ] `make test` — unit と envtest がすべて PASS し、回帰ゼロ。
- [ ] `make helm-lint` — chart の構造が valid。
- [ ] `make helm-template` — chart が正常にレンダリングされる。
- [ ] `make audit` — govulncheck + gosec + trivy fs (既知 fix のある
      HIGH / CRITICAL 検出: 0)。
- [ ] `go build ./...` — 全 OS/arch で 0 errors。

上記 6 項目は lefthook の pre-push hook が全 push で実行する。release tag
push も同じゲートを通る。

## 2. SSOT 同期ゲート (in-process unit test、`internal/observability/`)

以下のゲートは、いずれか 2 つの面が drift した時点で PR の merge を block する:

| # | ゲート | 検証内容 |
|---|---|---|
| 1 | `TestADRFilesAllInIndex` | `docs/kb/adr/` ↔ `INDEX.md` の双方向同期 |
| 2 | `TestADRIndexStatusValid` | Status 列 ∈ {Accepted, Proposed, Deprecated, Superseded by NNNN} |
| 3 | `TestADRIndexSupersededReferencesExist` | "Superseded" 参照がすべて実在 ADR に解決すること |
| 4 | `TestADRFilesHaveRequiredSections` | 各 ADR に Nygard の 3 セクション (Context / Decision / Consequences) が揃っていること |
| 5 | `TestAlertRulesSchemaSanity` | PrometheusRule CRD スキーマ (apiVersion / kind / groups) |
| 6 | `TestAlertRulesAllFieldsValid` | 全 alert に prefix `Valkey` + expr + for + severity + annotations が揃っていること |
| 7 | `TestAlertRulesMetricNamesRegistered` | alert の `expr` 内 `valkey_cluster_*` metric が `metrics.go` に登録されていること |
| 8 | `TestAlertRulesRunbookAnchorsExist` | `runbook_url` の anchor が `runbook.md` に解決すること |
| 9 | `TestRBACMarkerResourcesInRole` | `kubebuilder:rbac` marker ↔ `role.yaml` の双方向同期 |
| 10 | `TestSamplesStrictUnmarshal` | `config/samples/` ↔ API 型の strict-decode (unknown field は reject) |
| 11 | `TestSamplesDirHasAllExpected` | 登録済 sample map ↔ on-disk samples ディレクトリの双方向同期 |
| 12 | `TestSamplesMetadataValid` | `apiVersion` / `kind` / `metadata.name` 形式の検証 |
| 13 | `TestClusterRefKindEnumMatchesSSOT` | `ClusterReference.Kind` enum ↔ SSOT slice |
| 14 | `TestClusterRefKindAllHaveSwitchCase` | 各 kind に controller の switch case が存在すること |
| 15 | `TestLicenseFileExistsAndIsApache2` | `LICENSE` ファイル存在 + Apache-2.0 識別子 |
| 16 | `TestChartLicenseAnnotationMatchesLicenseFile` | Chart annotation ↔ `LICENSE` ファイル |
| 17 | `TestChartImagesAnnotationMatchesAppVersion` | `artifacthub.io/images` tag ↔ Chart `AppVersion` |
| 18 | `TestChartIconURLUsesCurrentValkeyAsset` | Chart icon URL が現時点で Artifact Hub から fetch 可能な Valkey logo asset であること |
| 19 | `TestChartCRDExamplesStrictUnmarshal` | `crdsExamples` ↔ API 型の strict-decode |
| 20 | `TestCRDBaseChartSync` | `config/crd/bases/` ↔ `charts/.../crds/` の byte-level (sha256) 一致 |
| 21 | `TestChartValuesValkeyVersionMatchesAPIDefault` | `values.yaml::valkey.version` ↔ API default |
| 22 | `TestChartNotesTxtModeValueValidEnum` | `NOTES.txt` の `mode:` ↔ `ValkeyMode` enum |
| 23 | `TestChartReadmeYAMLCodeblocksValid` | 全 Markdown 中の YAML ブロックの `mode` / `apiVersion` / `kind` が有効 |
| 24 | `TestMarkdownRelativeLinksResolve` | 全 `.md` 内の相対 `.md` link が解決すること |
| 25 | `TestIssueTemplateReadmeAnchorsExist` | issue template の anchor が README に解決すること |
| 26 | `TestWebhookSetupFunctionsRegisteredInMain` | webhook setup 関数 ↔ `main.go` への登録 |
| 27 | `TestReconcilerTypesRegisteredInMain` | Reconciler 型 ↔ `main.go` のインスタンス化 |
| 28 | `TestRBACRoleResourcesInMarker` | `role.yaml` の resource → `kubebuilder:rbac` marker (orphan rule の block) |
| 29 | `TestInstallYAMLStructure` | `dist/install.yaml` の構造 (5 CRD + Deployment + RBAC + Webhook + Service) |
| 30 | `TestKustomizeManifestLabelChainSync` | pod labels ⊇ Deployment selector ⊇ Service selector + ServiceMonitor selector ⊆ Service labels |
| 31 | `TestKustomizeChartResourcesSync` | `manager.yaml` ↔ `values.yaml` の resources (limits + requests × cpu + memory) |
| 32 | `TestKustomizeChartProbesSync` | manager Deployment ↔ chart probe の `initialDelay` と `period` |
| 33 | `TestKustomizeChartSecurityContextInvariants` | Pod Security "restricted" invariant を両側で満たす (runAsNonRoot、seccompProfile、allowPrivilegeEscalation=false、readOnlyRootFilesystem、capabilities.drop=ALL) |
| 34 | `TestInstallYAMLOperatorImageEnvMatchesContainerImage` | `dist/install.yaml` の `OPERATOR_IMAGE` env value ↔ manager container image (Upload/Download Job の ImagePullBackOff を block) |
| 35 | `TestChartArgsMatchOperatorFlags` | chart deployment + `config/manager` の args ↔ `cmd/main.go` の flag 定義 (古い flag → CrashLoopBackOff を block) |
| 36 | `TestValuesTemplateBindingCoverage` | `values.yaml` の top-level key が `templates/` のいずれかで参照されていること (silent ignore を block) |
| 37 | `TestChartFeaturesReconcilerEnvSync` | chart `features.{cluster,backup}.enabled` ↔ `ENABLE_{CLUSTER,BACKUP}_RECONCILER` env (RBAC と reconciler の整合を維持) |
| 38 | `TestNetworkPolicyWebhookPortPresent` | `webhook.enabled` 時の NetworkPolicy ingress 9443 条件付き rule (silent reject を block) |
| 39 | `TestNetworkPolicyTracingEgressPresent` | `tracing.endpoint` 時の NetworkPolicy egress OTLP 4317/4318 条件付き rule (silent な span loss を block) |
| 40 | `TestNetworkPolicyBackupEgressPresent` | `features.backup.enabled` 時の NetworkPolicy egress S3 (443/9000) 条件付き rule (`BackupTarget` の永久 Pending を block) |
| 41 | `TestMetricPhaseLabelsSync` | `metrics.go::allPhases` ↔ API `ValkeyPhase` + `ClusterPhase` enum union (Grafana の phase 時系列の完全性を維持) |
| 42 | `TestGoVersionDockerfileVsGoMod` | `Dockerfile` の `FROM golang:X.Y` ↔ `go.mod` の `go X.Y` ↔ `CONTRIBUTING.md` の Go テーブル |
| 43 | `TestKubernetesVersionSync` | `Chart.yaml` の `kubeVersion` ↔ README バッジ ↔ chart README の Kubernetes 前提条件 |
| 44 | `TestReleaseTargetInjectsBuildMetadataAndAmd64Only` | release image に build metadata が注入され、release は amd64+arm64 のマルチアーキを生成 (ADR-0045 後は release pipeline が `docker/build-push-action` でマルチアーキを生成) |
| 45 | `TestArtifactHubRepositoryMetadataEnablesVerifiedPublisherAndSigningKey` | `artifacthub-repo.yml` の `repositoryID`、`signingKey`、`owners` を同期維持 |
| 46 | `TestReleasePipelineRequiresSignedHelmCharts` | `HELM_SIGN=1` を default 化し、release/helm-publish で `.tgz.prov` を生成 |
| 47 | `TestReleaseSmokeVerifiesHelmProvenance` | GH Release / `gh-pages` の provenance を、公開済 signing key に対する `helm verify` で検証 |

実行コマンド: `go test ./internal/observability/`

## 3. Supply chain (release 時点で強制)

- [ ] `make sbom VERSION=vX.Y.Z` — syft の SPDX-2.3 SBOM を生成する。
- [ ] release pipeline が SBOM を GitHub Release に自動添付する。
- [ ] release pipeline が chart の `.tgz.prov` を GitHub Releases および
      `gh-pages` に自動添付する。
- [ ] container image、Helm chart、SBOM への **cosign keyless 署名**
      (ADR-0046、v1.0.13+)。検証手順は
      [SECURITY.md](../../.github/SECURITY.md#verifying-release-artifacts-signed-releases--v1013) 参照。
- [ ] image に対して `slsa-framework/slsa-github-generator` 経由で
      **SLSA-3 provenance** が attach されていること (ADR-0046)。
- [ ] `bash scripts/release-smoke-test.sh` — 8 ステージ: chart asset、
      SBOM asset、Helm provenance verification、helm pull、image
      manifest、`gh-pages`、trivy CVE scan、cosign verification。
- [ ] マルチアーキ image (linux/amd64 + linux/arm64)、default buildx
      builder で build。

## 4. ドキュメント

- [ ] CHANGELOG.md の `[Unreleased]` を `[vX.Y.Z]` へ promote する
      (git-cliff が自動化)。
- [ ] README §Roadmap の "Next" 項目が実際の release 内容と一致すること。
- [ ] ADR INDEX が最新であること (`TestADRFilesAllInIndex` が強制)。
- [ ] runbook.md §9 の alert ごとの対応手順が、新規 alert 追加時に更新されていること。

## 5. 運用ゲート

- [ ] kubectl 互換性: `kubeVersion ≥ 1.26` (Chart.yaml)。
- [ ] `make manifests` が working tree に対して no-op であること
      (controller-gen drift: 0)。`manifests` ターゲットが chart CRD も
      自動同期する。
- [ ] cert-manager / prometheus-operator が不在のときの graceful fallback
      (NotFound / NoMatch を fail-soft で扱う)。

## 6. ユーザー可視面 (自動検証)

これらは OSS の信頼指標として積み上がる:

- LICENSE Apache-2.0 (ゲート #15、#16)
- SECURITY.md に PGP fingerprint
- CONTRIBUTING.md に前提条件 + PR ワークフロー
- `.github/PULL_REQUEST_TEMPLATE.md` (ゲート #23)
- `.github/ISSUE_TEMPLATE/{bug_report,feature_request,question}.yml`
  (ゲート #24)
- README §Roadmap (ゲート #25 が anchor を強制)
- Artifact Hub の chart README + `crdsExamples` (ゲート #17、#18、#22)
- Issue triage labels (bug/triage の自動付与)

## 7. release tag push の手順

```bash
# 1. §1〜§6 の自動スイートを実行し、green を確認する。
make gate                                # = lint + test + helm + audit
go test ./internal/observability/        # 47 SSOT ゲート
bash scripts/release-smoke-test.sh vX.Y.Z  # 8 ステージ (image/chart 未公開なら skip)

# 2. release.sh を手動実行する。
bash scripts/release.sh vX.Y.Z

# 3. GH Release 本文を手動で publish する (release.sh が
#    .release_notes.md を書き出す)。
gh release create vX.Y.Z --notes-file .release_notes.md \
  dist/install.yaml \
  /tmp/valkey-operator-X.Y.Z.spdx.json

# 4. helm-publish (gh-pages を Artifact Hub が自動検出)。
make helm-publish HELM_SIGN=1 VERSION=vX.Y.Z
```

## 8. v0.1.0 GA 用の追加ゲート (alpha → GA 昇格時)

以下は alpha 期間中は **適用しない**。GA tag 以降:

- [ ] kind cluster 上での 24 時間 soak test: memory leak 0、FD leak 0。
- [ ] end-to-end 自動化 (kind + MinIO + ValkeyCluster restore シナリオ)
      の PASS。
- [ ] Track B (Failover / Resharding) の ADR + 実装の完了。
- [ ] Conversion webhook の準備 (v1alpha1 → v1beta1、ADR-0026 deferred)。
- [ ] HPA 統合 (Replication mode、ADR-0027 deferred)。
- [ ] semver `0.1.0` あるいは `1.0.0` の選択 (プロジェクト方針)。

## 9. 本書自体の自動検証

`docs/operations/release-checklist.md` 自体も SSOT ゲートの対象に含まれる:

- 本書が参照しているゲートが `internal/observability/` に実在することを、
  将来の横断ゲートで検証可能。
- 本書中の壊れた markdown link は `TestMarkdownRelativeLinksResolve` が block する。
