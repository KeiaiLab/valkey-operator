<p align="center">
  <img src="https://keiailab.com/assets/logo.svg" alt="keiailab" width="120"/>
</p>

<p align="center">
  <a href="family.md">English</a> |
  <b>한국어</b> |
  <a href="family.ja.md">日本語</a> |
  <a href="family.zh.md">中文</a>
</p>

# keiailab 오퍼레이터 패밀리

> 공유 기반 (operator-commons Go 라이브러리 + Helm partial + Apache-2.0 스택) 위에 구축된 네 자매 Kubernetes 오퍼레이터.

이 문서는 `valkey-operator` 저장소에서 작성되었으며, 패밀리 전체의 정본 크로스 링크 페이지입니다.

## 패밀리 개요

| 프로젝트 | 데이터베이스 | 상태 | 저장소 |
|---|---|---|---|
| **`postgres-operator`** | PostgreSQL 18+ | active | https://github.com/keiailab/postgres-operator |
| **`mongodb-operator`** | MongoDB 7.0+ | active | https://github.com/keiailab/mongodb-operator |
| **`valkey-operator`** | Valkey 8.0+ (Redis fork, BSD-3) | active | https://github.com/keiailab/valkey-operator |
| **`operator-commons`** | 공유 Go 라이브러리 | v0.7.0 | https://github.com/keiailab/operator-commons |

## 공유하는 것

네 프로젝트 모두 동일한 운영 프리미티브로 수렴합니다:

- **Apache-2.0** 일관 — SSPL 없음, SaaS 영역에 copyleft 없음
- **`operator-commons`** 공유 Go 라이브러리 (v0.7.0+) — finalizer, label, status sugar, security context builder, NetworkPolicy / ServiceMonitor partial
- **Helm chart skeleton** — RFC-0027 `default` falsy-toggle 방지, RFC-0026 component-keyed values, cycle 26 hardening 6 marker (priorityClassName / lifecycle / SA / minReadySeconds / automount / revisionHistoryLimit)
- **OLM bundle parity** — scorecard v1alpha3 6-test matrix
- **i18n** — README + 11개 canonical 문서를 영어 / 한국어 / 日本語 / 中文 으로 (cleanup supercycle 2026-05-21 의 Wave 4)

## 하지 않는 것

- ❌ **third-party 오퍼레이터 임베드 또는 wrapping** — license-clean, copyleft 의무 없음
- ❌ **GitHub Actions 를 release gate 로 사용** — 로컬 4-layer hook 시스템 (RFC-0002 참조)
- ❌ **시간 기반 roadmap deadline** — 기능 체크리스트 + 완성도 백분율
- ❌ **벤더 종속 컨테이너 이미지** — keiailab-published Apache-2.0 이미지만 사용

## 어디서 시작하나

| 작업 | 시작점 |
|---|---|
| Kubernetes 에 `valkey-operator` 배포 | [README.md](../README.md) Quickstart 섹션 |
| 아키텍처 읽기 | [ARCHITECTURE.md](ARCHITECTURE.md) |
| issue 또는 기능 요청 | https://github.com/keiailab/valkey-operator/issues |
| 디자인 또는 로드맵 논의 | https://github.com/keiailab/valkey-operator/discussions |
| 코드 기여 | [CONTRIBUTING.md](../.github/CONTRIBUTING.md) |
| 보안 이슈 보고 | [SECURITY.md](../.github/SECURITY.md) |
| 브랜드 / 보이스 학습 | [BRANDING.md](BRANDING.md) |
| adopter / 사용자 추적 | [ADOPTERS.md](ADOPTERS.md) |
| maintainer 찾기 | [MAINTAINERS.md](MAINTAINERS.md) |
| 거버넌스 모델 검토 | [GOVERNANCE.md](../.github/GOVERNANCE.md) |
| 향후 작업 확인 | [ROADMAP.md](ROADMAP.md) |

## 패밀리 간 호환성 (operator-commons)

세 데이터베이스 오퍼레이터 모두 일치하는 버전 (현재 `v0.7.0+`) 의 `github.com/keiailab/operator-commons` 를 import 합니다:

```go
import (
    "github.com/keiailab/operator-commons/pkg/version"
    "github.com/keiailab/operator-commons/pkg/security"
    "github.com/keiailab/operator-commons/pkg/labels"
    "github.com/keiailab/operator-commons/pkg/monitoring"
    "github.com/keiailab/operator-commons/pkg/finalizer"
    "github.com/keiailab/operator-commons/pkg/status"
)
```

`operator-commons` 의 breaking change 는 세 데이터베이스 오퍼레이터 모두에 동기화된 bump 가 필요 — supercycle Wave 5 의 `make cross-validation` target 으로 검증.

## i18n

본 페이지 (및 모든 정본 프로젝트 문서) 는 네 가지 언어로 제공됩니다:

- [English](family.md) (정본, canonical)
- **한국어** (이 파일)
- [日本語](family.ja.md)
- [中文](family.zh.md)

기술 내용에 대해서는 영어 버전이 정본이며, 현지화된 버전은 같은 의사결정을 native 표현으로 반영합니다.

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a>
</p>

<p align="center">
  © 2026 keiailab · <a href="../LICENSE">Apache-2.0</a> · <a href="https://keiailab.com">keiailab.com</a>
</p>
