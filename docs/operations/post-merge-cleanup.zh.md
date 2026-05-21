# 合并后清理 — 运维人员指南 (简体中文)

> English: [post-merge-cleanup.md](post-merge-cleanup.md) — canonical / 正本

squash-merge 之后清理**本地分支与合并残留**的操作流程。在 PR #38–#64 系列
日常运维中累积了 25 个 stale 本地分支后补充。

## 问题

`gh pr merge --delete-branch` **只会删除远端分支**。本地分支会被原样保留并
不断堆积:

```sh
$ git branch | wc -l
27   # main + 26 个 stale (squash-merge 完成后仍残留)
```

带来的副作用:

- `git branch` 输出噪声变多。
- IDE 的分支选择器中堆满 stale 项。
- 同名分支被复用时会引发混乱。

## 自动清理 (推荐)

squash-merge 会在 `main` 上生成**一个新的 commit**,因此
`git branch --merged main` 无法识别原始的 feature 分支。正确的判定方式是
"**本地存在但远端已无对应分支**":

```fish
# fish
git fetch -p
git branch -r | sed 's|origin/||' | sort > /tmp/remote_branches.txt
git branch | grep -v '^\*' | tr -d ' ' | sort > /tmp/local_branches.txt
comm -23 /tmp/local_branches.txt /tmp/remote_branches.txt \
  | grep -v '^main$' | xargs -r git branch -D
```

```bash
# bash — 等价写法
```

## 内置 helper (commit-commands)

`commit-commands:clean_gone` skill 已将上述流程自动化:

```sh
# claude-code skill (在项目根目录执行):
/clean_gone
```

推荐与 `gh pr merge --delete-branch` 配套使用的流程:

1. `git push` → 创建 PR。
2. PR review + approve。
3. `gh pr merge --squash --delete-branch <PR#>` (远端清理)。
4. `git checkout main && git pull` (同步 `main`)。
5. `/clean_gone` 或上述 fish 片段 (本地清理)。

## CI 失败邮件排障

### 现象

GitHub 持续推送**同一个 workflow 的失败邮件**。

### 诊断

```sh
# keiailab 各仓库最近 7 天的 failed runs
for repo in valkey-operator mongodb-operator postgres-operator operator-commons; do
  echo "--- $repo ---"
  gh run list --status failure --created ">=$(date -d '-7 days' +%Y-%m-%d)" --limit 5 -R keiailab/$repo
done
```

结果为空 = 没有新增失败 (邮件来自历史 retry)。

### 确认 workflow 是否仍然缺失 (RFC-0002 历史背景)

```sh
gh api repos/keiailab/<repo>/contents/.github/workflows
# 404 == 没有 workflows
```

> **备注 (2026-05-12)**: 本仓库的 workflow 曾在 RFC-0002 下被临时移除,
> 之后依据
> [ADR-0045](../kb/adr/0045-restore-github-actions-for-oss-ci.md)
> **恢复**,以保持 OSS CI 对齐。该日期之后的失败邮件是真实信号,
> 应该排查根因,而不是直接静音。

### 静音通知

GitHub Web UI:

- Profile → Settings → Notifications → Actions。
- 开启 "Send notifications for failed workflows only",或者
- 按仓库调整 Watch 设置。

## 预防本地残留

每个 PR 周期:

```fish
# PR 合并完成后立即执行
git checkout main
git pull
git branch -d <feature-branch-name>   # 即时清理本地分支
```

或者通过 lefthook 的 `post-merge` hook 自动化:

```yaml
# lefthook.yml
post-merge:
  commands:
    cleanup_gone_branches:
      run: git fetch -p && git branch -vv | awk '/: gone]/{print $1}' | xargs -r git branch -D
```

## 本指南的由来

在 PR #65 结案时识别出 25 个 stale 本地分支,因此编写本指南以**防止再次
发生**。ADR-0042 (commercial parity 系列) 收尾的同时,运维 hygiene 也一并
收紧。

## 参考

- ADR-0045: GH Actions 恢复 (`docs/kb/adr/0045-restore-github-actions-for-oss-ci.md`)
- `commit-commands:clean_gone` skill
- 全局 standards: `~/.claude/CLAUDE.md` §2 (Non-Negotiables)
