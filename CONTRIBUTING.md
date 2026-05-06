# Contributing

valkey-operator 에 기여해 주셔서 감사합니다. 본 문서는 PR 절차, 테스트
실행, 디자인 결정 추적 (ADR) 의 개요입니다.

## 시작하기

### 환경 요구사항

| 도구 | 최소 버전 | 비고 |
|---|---|---|
| Go | 1.25 | `go.mod` 와 일치 |
| Docker | 24+ | buildx 기본 빌더 사용 |
| kind | 0.27+ | 로컬 e2e |
| kubectl | 1.34+ | k3s/kind 모두 지원 |
| cert-manager | 1.16+ | webhook serving cert |
| make | 표준 GNU make | Makefile target 사용 |

### 첫 빌드 + 테스트

```sh
git clone https://github.com/keiailab/valkey-operator.git
cd valkey-operator

# pre-commit hooks 설치 (lefthook).
brew install lefthook       # 또는 go install
lefthook install

# 전체 단위 테스트 (envtest 자동 다운로드).
make test

# integration test (실 Valkey 컨테이너 — Docker 필요).
make integration-test

# e2e (kind cluster 에서 manager 배포 + 시나리오).
make test-e2e
```

## PR 절차

1. **Issue 우선** — 큰 변경 (architectural / API) 은 issue 로 사전 논의.
2. **Conventional Commits** — `<type>(<scope>): <subject>` 형식. 예:
   `feat(backup): TTL 자동 삭제`. 본문은 한국어 / 영어 모두 허용.
3. **테스트 동반** — 기능 추가/변경 시 단위 테스트 필수. `make test` 통과
   확인.
4. **lefthook 통과** — pre-commit 의 gofmt / govet / golangci-lint 자동
   실행. 실패 시 commit 차단.
5. **PR 본문**:
   - 사용자 시나리오 (왜 이 변경이 필요한가)
   - 검증 명령 + 출력 인용 (예: `make test`, `kubectl apply -f ...` 결과)
   - 영향 영역 (회귀 검증한 기능 목록)
   - 관련 ADR / Issue 링크
6. **리뷰 SLA**: 24시간 이내 첫 리뷰 (best-effort).

## ADR (Architecture Decision Records)

다음 변경은 **ADR 작성 의무**:

- 새 CRD 추가 / 기존 CRD field 의 의미 변경
- 외부 의존성 추가 (sonatype-guide + context7 검증 인용 의무)
- 보안 / 인증 / 데이터 흐름 변경
- 같은 문제를 3회 이상 다르게 풀고 있는 경우 (수렴 ADR)

ADR 위치: `docs/kb/adr/NNNN-<slug>.md`. Nygard 5섹션 (Context / Decision /
Consequences / Alternatives Considered / Action Items).

INDEX 갱신 의무 — `docs/kb/adr/INDEX.md`.

## 코드 스타일

- **Go**: `gofmt` + `golangci-lint` (lefthook pre-commit). errcheck 강제.
- **주석**: 한국어 / 영어 모두 허용. `왜` 그렇게 했는지 위주 (`무엇을`
  하는지는 코드가 보여줌).
- **테스트**: fake client 우선 (envtest 는 controller 통합 테스트 한정).
  `WithStatusSubresource` 사용 — spec/status 분리.

## 디자인 분기

큰 변경 전:

1. `~/.claude/plans/` 또는 `docs/plans/` 의 plan 파일 확인
2. 디자인 분기 6+ 가 있으면 ADR 사전 작성
3. atomic commit 정책 — 1 step = 1 commit, 각 commit lefthook 4-stage 통과

## 보안 이슈

보안 취약점은 *공개 issue 로 보고하지 마세요*. `SECURITY.md` 의 비공개
보고 경로 사용.

## 라이선스

본 프로젝트는 Apache License 2.0. 기여한 코드도 동일 라이선스로 배포됩니다.
