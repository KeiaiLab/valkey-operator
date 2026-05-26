# Post-merge cleanup — operator guide

> 한국어 버전: [post-merge-cleanup.ko.md](post-merge-cleanup.ko.md)

How to clean up **local branches and merge residue** after a
squash-merge. Added after the PR #38–#64 series accumulated 25
stale local branches in our day-to-day.

## The problem

`gh pr merge --delete-branch` **only deletes the remote branch**.
Local branches stay behind and accumulate:

```sh
$ git branch | wc -l
27   # main + 26 stale (squash-merged but still present)
```

Side effects:

- Noisy `git branch` output.
- Stale items in the IDE branch picker.
- Confusion if the same branch name gets reused later.

## Automatic cleanup (recommended)

Squash-merge produces a **new commit** on `main`, so
`git branch --merged main` cannot identify the original feature
branch. The correct heuristic is "**local branches with no remote
counterpart**":

```fish
# fish
git fetch -p
git branch -r | sed 's|origin/||' | sort > /tmp/remote_branches.txt
git branch | grep -v '^\*' | tr -d ' ' | sort > /tmp/local_branches.txt
comm -23 /tmp/local_branches.txt /tmp/remote_branches.txt \
  | grep -v '^main$' | xargs -r git branch -D
```

```bash
# bash — identical
```

## Bundled helper (commit-commands)

The `commit-commands:clean_gone` skill automates the above:

```sh
# claude-code skill (run at the project root):
/clean_gone
```

Recommended workflow alongside `gh pr merge --delete-branch`:

1. `git push` → open the PR.
2. PR review + approve.
3. `gh pr merge --squash --delete-branch <PR#>` (remote cleanup).
4. `git checkout main && git pull` (sync `main`).
5. `/clean_gone` or the fish snippet above (local cleanup).

## CI-failure-email troubleshooting

### Symptom

Repeated GitHub **workflow-failure emails** for the same workflow.

### Diagnosis

```sh
# Recent 7-day failed runs across every keiailab repo
for repo in valkey-operator; do
  echo "--- $repo ---"
  gh run list --status failure --created ">=$(date -d '-7 days' +%Y-%m-%d)" --limit 5 -R keiailab/$repo
done
```

Empty results = no new failures (the emails are historical retry).

### Confirm workflow absence (RFC-0002 historical context)

```sh
gh api repos/keiailab/<repo>/contents/.github/workflows
# 404 == no workflows
```

> **Note (2026-05-12)**: this repo's workflows were temporarily
> removed under RFC-0002, then **restored** under
> [ADR-0045](../kb/adr/0045-restore-github-actions-for-oss-ci.md)
> for OSS CI parity. Failure emails after that date are real
> signals and should be investigated, not silenced.

### Quieting notifications

GitHub web UI:

- Profile → Settings → Notifications → Actions.
- Turn on "Send notifications for failed workflows only", or
- Adjust the per-repository Watch setting.

## Prevent local stragglers

Every PR cycle:

```fish
# Right after the PR is merged
git checkout main
git pull
git branch -d <feature-branch-name>   # immediate local cleanup
```

Or wire it up via a lefthook `post-merge` hook:

```yaml
# lefthook.yml
post-merge:
  commands:
    cleanup_gone_branches:
      run: git fetch -p && git branch -vv | awk '/: gone]/{print $1}' | xargs -r git branch -D
```

## Why this guide exists

We identified 25 stale local branches at PR #65 closure and wrote
this guide to **prevent recurrence**. The closure of ADR-0042
(commercial parity series) is when we tightened operational hygiene
as well.

## References

- ADR-0045: GH Actions restoration (`docs/kb/adr/0045-restore-github-actions-for-oss-ci.md`)
- `commit-commands:clean_gone` skill
- Global standards: `~/.claude/CLAUDE.md` §2 (Non-Negotiables)
