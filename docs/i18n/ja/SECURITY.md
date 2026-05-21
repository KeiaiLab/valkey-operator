<p align="center">
  <a href="SECURITY.md">English</a> |
  <a href="SECURITY.ko.md">한국어</a> |
  <b>日本語</b> |
  <a href="SECURITY.zh.md">中文</a>
</p>

# セキュリティポリシー

> 英語版: [SECURITY.md](../../../.github/SECURITY.md) — canonical / 正本

## 脆弱性の報告

**セキュリティ報告に公開 issue を使用しないでください。** パッチがリリースされる前に脆弱性が公開されると、すべての adopter にリスクが及びます。

### 非公開での報告チャネル

以下のいずれかを選択してください:

1. **GitHub Security Advisory** (推奨):
   <https://github.com/keiailab/valkey-operator/security/advisories/new>
2. **メール**: `security@keiailab.com` (PGP オプション):
   - PGP fingerprint:
     `89A4 0947 6828 CB99 2338  C378 651E 51AF 520B CB78`
   - Public key: `gh-pages` ブランチの `artifacthub-repo.yml`、または
     <https://keiailab.github.io/valkey-operator/artifacthub-repo.yml>
   - 同じ key が `mongodb-operator` と `postgres-operator` でも
     使用されています (3-repo 統一鍵)。

### 報告に含めるべき内容

- 影響を受けるバージョン (release tag または commit SHA)
- 再現手順 (再現可能な最小ケース)
- 影響評価 (可能であれば CVSS のセルフスコアを含める)
- 報告者の身元 — クレジット表記を希望される場合はその旨をお知らせください

## 応答 SLA

| 段階 | 目標 |
|---|---|
| 初回受信確認 | 72 時間以内 |
| 重大度のトリアージ | 7 日以内 |
| パッチリリース | 重大度に応じて (Critical: 14 日、High: 30 日、Medium: 60 日) |
| 公開開示 | パッチリリース後 14 日 (要望に応じて協調開示) |

## サポート対象バージョン

| Version | Supported |
|---------|-----------|
| 0.x (alpha) | ✅ 最新 minor のみ |
| 1.0+ (stable) | TBD — 初回の stable release 後に更新 |

本プロジェクトは現在 `v1alpha1` の段階です。**後方互換性の保証はありません**。セキュリティ修正は最新の release にのみ適用されます。

## 運用上のセキュリティ推奨事項

`valkey-operator` を運用する際は:

1. **TLS の強制**: `Spec.TLS.Enabled=true` を設定してください
   (cert-manager もしくはユーザー提供の `CustomCert`)。ADR-0010 と
   ADR-0014 を参照。
2. **Auth は事実上常に有効**: ADR-0013 に基づき、operator は
   `Spec.Auth.Enabled` の値にかかわらず 32 バイトのランダムパスワード
   を払い出します。
3. **NetworkPolicy**: `Spec.NetworkPolicy.Enabled=true` を設定して
   pod-to-pod ingress を制限してください。NetworkPolicy を実際に強制する
   CNI (Calico、Cilium) 上で検証してください。
4. **Pod Security Standard: restricted**: ご利用の namespace に
   `pod-security.kubernetes.io/enforce=restricted` を適用してください。
5. **認証情報は専用の Secret に分離**: `ValkeyBackupTarget` の
   S3 credentials は RBAC でアクセス制御された専用 `Secret` に
   配置してください (ADR-0016)。
6. **バックアップは外部ストレージを推奨**: 外部 S3 と組み合わせた
   `Destination.Type=TargetRef` を使用してください。PVC のみのバックアップ
   はクラスター自体が失われると同時に失われます。
7. **コンテナイメージの検証**: operator image は Sonatype と Context7 の
   レビューを通過した依存関係のみで構築されます (ADR-0022)。独自の
   variant をビルドする場合は、生成物に対して `trivy` または `grype` を
   実行してください。

## 依存関係のセキュリティ

依存関係を導入するすべての ADR は、関連する **Sonatype Trust Score** と
**Context7** 検証を引用します (正典の例は `docs/kb/adr/0022-*.md` を
参照)。

Dependabot および Renovate による自動更新 PR は、キューの先頭で
レビューします。

## リリース成果物の検証 (署名済みリリース — v1.0.13+)

**v1.0.13** 以降、公開されるすべての container image、Helm chart、
SPDX SBOM は **Sigstore cosign** の keyless OIDC で署名され、
**SLSA-3 provenance attestation** が付与されています (ADR-0045、
ADR-0046)。v1.0.13 より前のリリースは未署名のため、以下の検証コマンド
は想定通り失敗します。

### Container image の検証

```bash
COSIGN_EXPERIMENTAL=1 cosign verify \
  --certificate-identity-regexp '^https://github\.com/keiailab/valkey-operator/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/keiailab/valkey-operator:<version>
```

### Image の SLSA-3 provenance の検証

```bash
slsa-verifier verify-image \
  --source-uri github.com/keiailab/valkey-operator \
  --source-tag v<version> \
  ghcr.io/keiailab/valkey-operator:<version>
```

### Helm chart の検証

GitHub Release ページから `valkey-operator-<version>.tgz`、`.tgz.sig`、
`.tgz.pem` をダウンロードしたうえで:

```bash
cosign verify-blob \
  --certificate   valkey-operator-<version>.tgz.pem \
  --signature     valkey-operator-<version>.tgz.sig \
  --certificate-identity-regexp '^https://github\.com/keiailab/valkey-operator/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  valkey-operator-<version>.tgz
```

### SBOM の検証

`.spdx.json` / `.sig` / `.pem` のトリプルに対して同じ `cosign verify-blob`
パターンを適用します。SBOM 署名は bill-of-materials を image を生成した
ビルドと正確に紐付けます。

### 検証成功が意味するもの

- 当該成果物が、本リポジトリの GitHub Actions ワークフローによって
  生成されたこと (証明書 identity が OIDC subject を証明)。
- 署名以降に成果物が改変されていないこと (Sigstore Rekor 透明性ログの
  エントリは改竄検知可能)。
- container image については、SLSA-3 attestation がさらに、ビルドが
  isolated でホストされた GitHub runner 上で文書化された `release.yml`
  ワークフローを用いて実行されたことを証明します。

## 既知の制限事項

- 英語: [README.md → "Known limitations"](../../../README.md#known-limitations)
- 韓国語: [README.ko.md → "잠재적 운영 이슈"](../../../README.ko.md#잠재적-운영-이슈-현재-알려진-한계)
- 関連: GitHub Issues で `security` ラベルを参照してください。

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
