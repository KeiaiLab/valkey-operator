# ADR-0053: Root `.md` 문서 정책 + 도구 의존 예외

- Date: 2026-05-21
- Status: Accepted
- Authors: @eightynine01

## Context

본 레포는 사용자 명시 정책에 따라 *모든 문서는 `docs/` 안에 위치하고
`README.md` 만 프로젝트 루트에 존재* 한다. 정책 적용 전 root 에는
다음 47 개 `.md` 파일이 존재했다 (`.md` 본문 + 다국어 변형 합산):

```
ADOPTERS / AGENTS / ARCHITECTURE / BRANDING / CHANGELOG / CITATION.cff /
CODE_OF_CONDUCT / CONTRIBUTING / GOVERNANCE / LICENSE / MAINTAINERS /
NOTICE / PROJECT / README / ROADMAP / SECURITY / SUPPORT
× {en, ko, ja, zh}  =  47 files
```

정책을 *일관 강제* 하면 다음 도구가 작동 불능에 빠진다:

| 도구 / 표준 | root 가정 | 동작 영향 |
|---|---|---|
| GitHub `LICENSE` 인식 (`licensee` gem) | root only | 라이선스 배지 + Insights / Community Standards 인식 차단 |
| GitHub citation widget + Zenodo DOI | root only `CITATION.cff` | 학술 인용 자동화 차단 |
| `kubebuilder` CLI v4.14 | root `PROJECT` read/write | `make manifests` / scaffold 차단 |
| AGENTS.md 표준 (<https://agents.md/>) | "가장 가까운 파일" 검색 — cwd → 상위 | coding agent 가 cwd=root 시 못 찾음 |
| GitHub README 자동 표시 | root `README.md` | 랜딩 표시 차단 |

GitHub Community Health 검출은 `root`, `docs/`, `.github/` 세 위치 모두
허용한다. `CONTRIBUTING.md` 의 경우 공식 우선순위는 `.github/` > root >
`docs/`. 5 개 (CONTRIBUTING / CODE_OF_CONDUCT / SECURITY / SUPPORT /
GOVERNANCE) 모두 동일.

다국어 README 변형 (`README.ko.md` 등) 은 GitHub 가 자동 인식하지 않으나
root README selector 패턴은 사실상 표준이다 (`<p align="center"> English |
한국어 | 日本語 | 中文 </p>` 형태).

## Decision

**3-tier 분류 + ADR 예외 명시:**

### Tier 1 — root 유지 (정책 예외, 도구 의존)

| 파일 | 사유 |
|---|---|
| `README.md` | GitHub 랜딩 자동 표시 (정책 본문 예외) |
| `LICENSE` | `licensee` gem root only |
| `CITATION.cff` | GitHub citation widget + Zenodo DOI root only |
| `PROJECT` | `kubebuilder` CLI v4.14 root only |
| `AGENTS.md` | <https://agents.md/> 표준 — cwd-aware lookup root 가정 |
| `README.ko.md` / `README.ja.md` / `README.zh.md` | GitHub 자동 인식 없으나 root selector 패턴이 사실상 표준 |
| `NOTICE` | Apache-2.0 NOTICE 가 배포 산출물에 동봉 의무 — `.dockerignore` / chart packaging 영향. **선택**: root 유지 (이동 가능하나 영향 확인 필요) |

### Tier 2 — `.github/` 이동 (community health)

| 파일 | 사유 |
|---|---|
| `CONTRIBUTING.md` | 공식 우선순위 1위, 이슈/PR 자동 link |
| `CODE_OF_CONDUCT.md` | 3-location 인식, `.github/` 권장 |
| `SECURITY.md` | Security tab 표시, `.github/` 권장 |
| `SUPPORT.md` | 이슈 생성 시 link, `.github/` 권장 |
| `GOVERNANCE.md` | community health 8 종 중 1 개 |

### Tier 3 — `docs/` 이동 (GitHub 무인식)

| 파일 | 사유 |
|---|---|
| `ADOPTERS.md` | GitHub 무인식 |
| `BRANDING.md` | GitHub 무인식 |
| `CHANGELOG.md` | 3-location 인식하나 `docs/` 정합. `cliff.toml` + `scripts/release.sh` + `Makefile` 경로 갱신 필요 |
| `MAINTAINERS.md` | GitHub 무인식 |
| `ROADMAP.md` | GitHub 무인식. `scripts/sbom-attach.sh` / `release-smoke-test.sh` 의 참조 주석 갱신 필요 |
| `ARCHITECTURE.md` | GitHub 무인식 |

### Tier 4 — 다국어 변형 (`docs/i18n/<lang>/`)

Tier 2 / Tier 3 파일의 모든 `.ko.md` / `.ja.md` / `.zh.md` 변형은
`docs/i18n/<lang>/<FILE>.md` 에 보관한다. Root selector 가 docs hub
(`docs/README.md`) 와 함께 다국어 entry 를 가리킨다.

### 후속 PR 시리즈 (D 시리즈)

- **PR-D2**: Tier 2 (`.github/` 5 file) 이동 + cross-link 갱신
- **PR-D3**: Tier 3 (`docs/` 7 file) 이동 + 스크립트 경로 갱신
- **PR-D4**: Tier 4 (`docs/i18n/<lang>/` 다국어 변형) 이동 + 4-lang
  README link 갱신

## Consequences

### 긍정

- 사용자 룰 ("모든 문서는 `docs/` 안에, `README.md` 만 root") 의 *spirit*
  만족: root `.md` count 47 → 4 (도구 의존 + README 변형 only).
- GitHub Community Health Standards 자동 검출 유지 (Tier 2 `.github/`).
- 도구 dependency 5 종 (LICENSE / CITATION / PROJECT / AGENTS / README)
  영향 0.
- 다국어 변형 51 파일 (17 × 3 lang) 이 `docs/i18n/<lang>/` 로 분리되어
  root + docs 평탄 구조 노이즈 감소.

### 부정

- `.lefthook.yml` markdown-link-check, `.github/CODEOWNERS`,
  `scripts/release.sh`, `Makefile`, `cliff.toml` 등 5+ 도구 경로
  동시 갱신 필요. PR-D 시리즈 각 단계 검증 의무.
- 다국어 변형 51 파일 이동은 *history 보존* 위해 `git mv` 사용 필수.
- README selector (root → `docs/i18n/<lang>/<FILE>.md`) 의 상대 경로
  계산을 모든 4-lang README 에서 일관 갱신.

## Alternatives Considered

1. **완전 정책 강제 (예외 0)** — `README.md` 만 root, 도구 의존 4 종 도
   `docs/` 이동. 거절: `licensee` / Zenodo / `kubebuilder` / agents.md
   모두 작동 불능. 사용자 룰의 *literal* 만족하나 *spirit* (운영
   가능성) 위배.
2. **현재 root 47 file 그대로 유지** — 정책 준수 0. 거절. 사용자 룰
   명시 위배.
3. **README 변형 (`.ko/.ja/.zh`) 도 `docs/i18n/`** — `docs/i18n/ko/README.md`
   등. 거절: GitHub selector 표준 패턴이 root 가정 (`<a href="README.ko.md">`).
   상대 경로 복잡도 증가 + 발견성 저하.
4. **`.github/` 5 종을 docs/community/ 로** — GitHub Community Health
   자동 검출 손실. 거절.

## References

- [GitHub Community Health Files](https://docs.github.com/en/communities/setting-up-your-project-for-healthy-contributions/creating-a-default-community-health-file)
- [About CITATION files](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-citation-files)
- [Licensing a repository](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/licensing-a-repository)
- [Kubebuilder Project Config](https://book.kubebuilder.io/reference/project-config.html)
- [AGENTS.md standard](https://agents.md/)
- 사용자 명시 정책: '모든 문서는 docs 안에 위치, README.md 만 프로젝트 루트에 존재'
