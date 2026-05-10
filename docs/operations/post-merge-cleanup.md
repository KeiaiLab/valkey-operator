# Post-Merge Cleanup — 운영자 가이드

PR squash-merge 후 *local branch + 머지 흔적* 정리 절차. PR #38-#64 시리즈
운영 중 누적된 25 stale local branch 사례 발견 후 추가.

## 문제

`gh pr merge --delete-branch` 는 **remote branch 만 삭제**. local branch 는
*그대로 남아 누적*:

```sh
$ git branch | wc -l
27   # main + 26 stale (squash-merged 후 잔존)
```

이는 다음 부작용:
- `git branch` 출력 noise
- IDE 의 branch picker 에 stale 항목
- 동일 이름 재사용 시 confusion

## 자동 cleanup (권장)

squash-merge 는 *new commit* 으로 main 에 통합하므로 `git branch --merged main`
이 식별 못함. *origin 에 없는 local branch* 식별이 정확.

```fish
# fish 사용 시:
git fetch -p
git branch -r | sed 's|origin/||' | sort > /tmp/remote_branches.txt
git branch | grep -v '^\*' | tr -d ' ' | sort > /tmp/local_branches.txt
comm -23 /tmp/local_branches.txt /tmp/remote_branches.txt | grep -v '^main$' | xargs -r git branch -D
```

```bash
# bash 동일.
```

## 통합 helper (commit-commands)

`commit-commands:clean_gone` skill 이 이를 자동화:

```sh
# claude-code skill (project root 에서):
/clean_gone
```

`gh pr merge --delete-branch` 와 함께 사용 권장 흐름:

1. `git push` → PR 생성
2. PR review + approve
3. `gh pr merge --squash --delete-branch <PR#>` (remote cleanup)
4. `git checkout main && git pull` (main 동기)
5. `/clean_gone` 또는 위 fish snippet (local cleanup)

## CI 에러 메일 troubleshooting

### 증상

GitHub 에서 *동일 워크플로우 실패 메일이 반복적으로* 도착.

### 진단

```sh
# 모든 keiailab repo 의 최근 7일 failed runs
for repo in valkey-operator mongodb-operator postgres-operator operator-commons; do
  echo "--- $repo ---"
  gh run list --status failure --created ">=$(date -d '-7 days' +%Y-%m-%d)" --limit 5 -R keiailab/$repo
done
```

빈 결과 = 신규 실패 부재 (=> 메일은 historical retry).

### Workflow 자체 부재 확인 (RFC-0002 정합)

```sh
gh api repos/keiailab/<repo>/contents/.github/workflows
# 404 = 정합 OK (workflow 부재)
```

본 4 repo 모두 RFC-0002 (GitHub Actions 영구 금지) 적용 완료 — workflow 부재.
신규 실패 발생 부재. 메일은 GitHub notification system 의 retry 로 추정.

### Notification 정리

GitHub web UI:
- Profile → Settings → Notifications → Actions
- "Send notifications for failed workflows only" 비활성 또는
- repository 별 Watch 설정 변경

## Local cleanup 예방

매 PR cycle 마다:

```fish
# PR merge 직후
git checkout main
git pull
git branch -d <feature-branch-name>   # local cleanup 즉시
```

또는 자동화 hook (lefthook):

```yaml
# lefthook.yml 의 post-merge hook
post-merge:
  commands:
    cleanup_gone_branches:
      run: git fetch -p && git branch -vv | awk '/: gone]/{print $1}' | xargs -r git branch -D
```

## 본 가이드 작성 배경

PR #65 (closure) 시점 25 개 stale local branch 식별 후 *재발 방지* 위해 작성.
ADR-0042 commercial parity series 종료와 함께 *운영 hygiene* 측 closure.

## 참조

- RFC-0002: GitHub Actions 영구 금지 (글로벌 standards)
- commit-commands:clean_gone skill
- `~/.claude/CLAUDE.md` §2 (Non-Negotiables)
