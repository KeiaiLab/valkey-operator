# Post-Merge Cleanup — 運用者ガイド (日本語)

> English: [post-merge-cleanup.md](post-merge-cleanup.md) — canonical / 正本

PR の squash-merge 後に *ローカルブランチとマージ残骸* を片付ける手順。PR #38-#64 シリーズの運用中に 25 件のステイルなローカルブランチが累積した事例を受けて追加された。

## 問題

`gh pr merge --delete-branch` は **リモートブランチのみ削除する**。ローカルブランチはそのまま残り、累積していく:

```sh
$ git branch | wc -l
27   # main + 26 stale (squash-merge 済みだが残存)
```

副作用:

- `git branch` 出力のノイズ。
- IDE のブランチピッカーにステイルな項目が並ぶ。
- 同名ブランチを再利用した際の混乱。

## 自動クリーンアップ (推奨)

squash-merge は `main` 上に **新しいコミット** を生成するため、`git branch --merged main` ではオリジナルの feature ブランチを特定できない。正しいヒューリスティックは「**リモートに対応物のないローカルブランチ**」である:

```fish
# fish
git fetch -p
git branch -r | sed 's|origin/||' | sort > /tmp/remote_branches.txt
git branch | grep -v '^\*' | tr -d ' ' | sort > /tmp/local_branches.txt
comm -23 /tmp/local_branches.txt /tmp/remote_branches.txt \
  | grep -v '^main$' | xargs -r git branch -D
```

```bash
# bash — 同一処理
```

## 同梱ヘルパー (commit-commands)

`commit-commands:clean_gone` skill が上記を自動化する:

```sh
# claude-code skill (プロジェクトルートで実行):
/clean_gone
```

`gh pr merge --delete-branch` と併用する推奨ワークフロー:

1. `git push` → PR 作成。
2. PR レビュー + approve。
3. `gh pr merge --squash --delete-branch <PR#>` (リモートクリーンアップ)。
4. `git checkout main && git pull` (`main` 同期)。
5. `/clean_gone` または上記 fish スニペット (ローカルクリーンアップ)。

## CI 失敗メールのトラブルシューティング

### 症状

同一ワークフローに対する GitHub **ワークフロー失敗メール** が繰り返し届く。

### 診断

```sh
# keiailab の全リポジトリにわたる直近 7 日間の失敗 run
for repo in valkey-operator mongodb-operator postgres-operator operator-commons; do
  echo "--- $repo ---"
  gh run list --status failure --created ">=$(date -d '-7 days' +%Y-%m-%d)" --limit 5 -R keiailab/$repo
done
```

結果が空 = 新規失敗なし (メールは過去分の再送)。

### Workflow 不在の確認 (RFC-0002 の経緯)

```sh
gh api repos/keiailab/<repo>/contents/.github/workflows
# 404 == workflow 不在
```

> **注 (2026-05-12)**: 当リポジトリの workflow は RFC-0002 のもとで一時的に削除されたが、その後 OSS CI 整合のため [ADR-0045](../kb/adr/0045-restore-github-actions-for-oss-ci.md) により **復旧** されている。この日付以降の失敗メールは実信号として扱い、黙殺せず調査すること。

### 通知の抑制

GitHub Web UI:

- Profile → Settings → Notifications → Actions。
- 「Send notifications for failed workflows only」を有効化、または
- リポジトリ単位の Watch 設定を調整する。

## ローカル残骸の予防

PR サイクルごとに:

```fish
# PR がマージされた直後
git checkout main
git pull
git branch -d <feature-branch-name>   # 即座にローカルクリーンアップ
```

または lefthook の `post-merge` フックで自動化する:

```yaml
# lefthook.yml
post-merge:
  commands:
    cleanup_gone_branches:
      run: git fetch -p && git branch -vv | awk '/: gone]/{print $1}' | xargs -r git branch -D
```

## 本ガイドを書いた経緯

PR #65 クローズ時点で 25 件のステイルなローカルブランチを特定し、**再発防止** のために本ガイドを起こした。ADR-0042 (商用整合シリーズ) のクローズを機に、運用衛生面も併せて引き締めた。

## 参照

- ADR-0045: GH Actions 復旧 (`docs/kb/adr/0045-restore-github-actions-for-oss-ci.md`)
- `commit-commands:clean_gone` skill
- グローバル standards: `~/.claude/CLAUDE.md` §2 (Non-Negotiables)
