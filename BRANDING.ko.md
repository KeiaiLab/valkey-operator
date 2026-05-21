<p align="center">
  <a href="BRANDING.md">English</a> |
  <b>한국어</b> |
  <a href="BRANDING.ja.md">日本語</a> |
  <a href="BRANDING.zh.md">中文</a>
</p>

# Branding Guide — `valkey-operator` (한국어)

> English: [BRANDING.md](BRANDING.md) — canonical / 정본

> keiailab operator 패밀리의 시각 정체성, 보이스, 톤 정의.

본 문서는 `valkey-operator` 브랜딩 결정의 정본 레퍼런스입니다. README, release note, 마케팅 자료, 그리고 본 프로젝트를 대표하는 모든 제 3 자 커뮤니케이션에 적용됩니다.

## 1. Identity (정체성)

**Organization**: [keiailab](https://keiailab.com) — Kubernetes-native 데이터 플랫폼 operator (Apache-2.0, license-clean, vanilla-upstream 호환).

**Project**: `valkey-operator` — Kubernetes 용 Apache-2.0 Valkey Operator — Standalone + Cluster + Backup/Restore, BSD-3 license-clean.

**Family**: [`operator-commons`](https://github.com/keiailab/operator-commons) 공용 라이브러리를 공유하는 4 개 자매 operator 중 하나:

| Project | Database | Repository |
|---|---|---|
| `postgres-operator` | PostgreSQL 18+ | https://github.com/keiailab/postgres-operator |
| `mongodb-operator` | MongoDB 7.0+ | https://github.com/keiailab/mongodb-operator |
| `valkey-operator` | Valkey 8.0+ (Redis fork, BSD-3) | https://github.com/keiailab/valkey-operator |
| `operator-commons` | Shared Go library | https://github.com/keiailab/operator-commons |

## 2. Logo & Visual Assets (로고 및 시각 자산)

| 자산 | URL | 용도 |
|---|---|---|
| Primary logo (SVG) | `https://keiailab.com/assets/logo.svg` | README header, 슬라이드 |
| Mono mark | `https://keiailab.com/assets/mark.svg` | Favicon, 소셜 카드 |
| Wordmark | `https://keiailab.com/assets/wordmark.svg` | Footer, 어두운 배경 |

**Logo placement**: README 의 상단 중앙, 너비 120px. 항상 https://keiailab.com 으로 링크.

**Clear space**: 로고 주위 최소 여백 = 로고 너비의 25%.

**금지 사항**:
- 로고 색상 변경
- 그림자 또는 필터 추가
- 콘트라스트가 부족한 배경 위에 배치
- keiailab 브랜드 승인 없이 다른 로고와 결합

## 3. Color Palette (컬러 팔레트)

| Role | Hex | Usage |
|---|---|---|
| Primary (keiailab teal) | `#0EA5A8` | 헤더, primary 액션, 링크 |
| Secondary (deep navy) | `#0F172A` | 어두운 배경, 코드 블록 |
| Accent (warm amber) | `#F59E0B` | 강조, badge 액센트 |
| Neutral grey | `#64748B` | 밝은 배경 위의 본문 텍스트 |
| Background light | `#F8FAFC` | 문서 페이지 배경 |
| Background dark | `#020617` | 코드 에디터 테마, 다크 모드 |

GitHub README 의 shield.io badge 는 위 hex 사용 권장.

## 4. Typography (타이포그래피)

- **Headings**: System default (GitHub 의 default `-apple-system, BlinkMacSystemFont, Segoe UI, ...`)
- **Body**: 동일 (GitHub-native 정합)
- **Code**: `ui-monospace, SFMono-Regular, Consolas, ...` (GitHub 의 default monospace)

별도 webfont 사용 안 함 (GitHub README rendering 정합).

## 5. Voice & Tone (보이스 및 톤)

**Audience**: Kubernetes 플랫폼 엔지니어 / DBA / SRE.

**Voice principles (보이스 원칙)**:
- **Direct (직접적)** — 가능한 경우 단락보다 bullet-point
- **Evidence-based (근거 기반)** — 주장에는 benchmark / SLA / 링크 포함
- **Vendor-neutral (벤더 중립)** — upstream (PostgreSQL, MongoDB, Valkey) 참조는 하되 제 3 자 operator 를 embed / wrap 하지 않음
- **License-aware (라이선스 인지)** — Apache-2.0 + BSD/MIT/PG-license 의존성만 사용

**피해야 할 것**:
- 마케팅적 최상급 표현 ("blazing fast", "revolutionary", "best-in-class")
- 모호한 비교 ("X-class quality") — *구체적 메트릭 또는 benchmark 로 자격 부여*
- 로드맵에서의 시간 기반 데드라인 (`standards/roadmap.md §1.1` 의 feature 체크리스트 사용)

## 6. README Header Standard (README 헤더 표준)

모든 README 의 첫 문단은 다음 형식 (Wave 3 표준):

```markdown
<p align="center">
  <img src="https://keiailab.com/assets/logo.svg" alt="keiailab" width="120"/>
</p>

# valkey-operator

> **Apache-2.0 Valkey Operator for Kubernetes — Standalone + Cluster + Backup/Restore, BSD-3 license-clean**

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"/></a>
  <!-- 기존 shield.io badges 유지 + 정합 -->
</p>

<p align="center">
  <b>English</b> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.zh.md">中文</a>
</p>
```

## 7. README Footer Standard (README 푸터 표준)

모든 README + root-level .md 파일의 마지막에 다음 footer 부착 (Wave 3 표준):

```markdown
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
```

## 8. Badges 표준 순서

README 의 shield.io badge 순서 (좌→우):

1. License (Apache-2.0)
2. Go Version (1.25+)
3. Database (e.g. PostgreSQL 18+ / MongoDB 7.0+ / Valkey 8.0+)
4. Kubernetes Version (1.26+)
5. Container Image (ghcr.io/keiailab)
6. Helm Chart (Chart.yaml version + Artifact Hub link)
7. OpenSSF Scorecard
8. GitHub Discussions

## 9. Discussions / Issues / PR Templates

- **Discussions**: `https://github.com/keiailab/valkey-operator/discussions` — 기능 아이디어, Q&A
- **Issues**: 버그 리포트 + 유스 케이스가 포함된 구체적인 feature request
- **PR template**: `.github/PULL_REQUEST_TEMPLATE.md` 표준 (사용자 시나리오 + 검증 명령 인용 의무, `standards/checklist.md §3`)

## 10. Social & External (소셜 및 외부)

- **Website**: https://keiailab.com
- **GitHub Org**: https://github.com/keiailab
- **Artifact Hub** (Helm): https://artifacthub.io/packages/search?repo=keiailab-valkey-operator
- **GHCR** (Container): https://github.com/keiailab/valkey-operator/pkgs/container/valkey-operator

## 11. License & Attribution (라이선스 및 출처 표기)

- License: [Apache-2.0](LICENSE)
- Copyright: © 2026 keiailab contributors
- Third-party attributions: [NOTICE](NOTICE) 참조 (해당 시)

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
