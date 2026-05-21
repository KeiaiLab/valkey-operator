# Artifact Hub Trust Badge 運用 — valkey-operator (日本語)

> English: [artifacthub-trust.md](artifacthub-trust.md) — canonical / 正本

Artifact Hub 上の `valkey-operator` パッケージに対する `Signed` / `Official`
バッジを取得・維持するための運用手順。本書は **再現可能な確認手順と申請
フロー** を SSOT として固定するためのものである。

## 現状

2026-05-12 時点の確認:

```bash
curl -fsSL https://artifacthub.io/api/v1/packages/helm/keiailab-valkey-operator/valkey-operator \
  | jq '{version, signed, official, repository:{verified_publisher:.repository.verified_publisher, official:.repository.official}}'
```

観測値:

- `repository.verified_publisher=true`
- `signed=false`
- `repository.official=false`
- `https://keiailab.github.io/valkey-operator/valkey-operator-1.0.10.tgz.prov` は 404

リポジトリの所有権検証は **すでに完了している**。残っているのは
**Helm provenance の公開** と **Artifact Hub 公式ステータスの審査** の
2 軸である。

## Signed バッジを点灯させる

Artifact Hub は chart アーカイブの隣に `<chart>-<version>.tgz.prov` が
**並んで存在する場合に限り** Helm の `Signed` 状態を表示する。`.tgz.prov`
は `helm package --sign` によって生成される。

標準的な release コマンド:

```bash
make release VERSION=vX.Y.Z
```

`Makefile` は `HELM_SIGN=1` を既定で強制するため、以下の前提が **すべて**
揃っていないと release は失敗する:

- `~/.gnupg/secring.gpg`
- `HELM_GPG_KEY=Keiailab Helm`
- `HELM_GPG_FINGERPRINT=89A409476828CB992338C378651E51AF520BCB78`

新しい開発マシンで secret key を準備する場合:

```bash
gpg --import <keiailab-helm-private-key.asc>
gpg --export-secret-keys > ~/.gnupg/secring.gpg
make helm-signing-preflight
```

release 直後の検証:

```bash
bash scripts/release-smoke-test.sh vX.Y.Z
```

このスモークテストは、(a) GitHub Releases 上の `.tgz.prov` アセット、
(b) `gh-pages` ブランチ上の `.tgz.prov` ファイル、(c) `artifacthub-repo.yml`
に登録された公開 signing key を用いた `helm verify` の結果、という **3 面**
をまとめて確認する。

## Official バッジを点灯させる

`Official` はリポジトリ内のファイル操作だけでは有効化できない。これは
Artifact Hub による **外部審査ステータス** であり、「publisher が当該
パッケージの主対象となるソフトウェアを直接所有していること」を表す。

すでに満たしている前提:

- Artifact Hub repository ID:
  `16085dd0-0f19-4c6b-ab90-bd97105bdf42`
- Verified publisher: `true`
- Chart README: `charts/valkey-operator/README.md`
- リポジトリメタデータの配信:
  `https://keiailab.github.io/valkey-operator/artifacthub-repo.yml`

残作業:

1. Artifact Hub の publisher または organization メンバーが
   `artifacthub/hub` に official status request の issue を起票する。
2. **repository 単位** での official status を申請する。
3. 申請本文に以下の事実をそのまま含める。

```text
Repository name: keiailab-valkey-operator
Package name: valkey-operator
Repository URL: https://keiailab.github.io/valkey-operator
Package URL: https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator
Publisher owns the software: yes, keiailab publishes and maintains valkey-operator itself.
Verified publisher: true
Documentation: charts/valkey-operator/README.md
```

Artifact Hub 側の審査が完了するまで、リポジトリのコード変更や
`gh-pages` への再公開で `Official` バッジを `true` に切り替える方法は
存在しない。本工程は **完全に外部判断の待ち時間** である。
