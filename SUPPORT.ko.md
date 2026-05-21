<p align="center">
  <a href="SUPPORT.md">English</a> |
  <b>한국어</b> |
  <a href="SUPPORT.ja.md">日本語</a> |
  <a href="SUPPORT.zh.md">中文</a>
</p>

# Support (지원)

> 한국어 사용자: 본 문서의 채널은 영어와 한국어 모두 환영합니다.

`valkey-operator` 를 사용해 주셔서 감사합니다. 본 페이지는 도움을 받을 수 있는 곳을 안내합니다.

## Decide what you need (어떤 도움이 필요한지 결정하기)

| 상황 | 이동 위치 |
|---|---|
| **보안 취약점을 발견한 것 같습니다.** | **공개 issue 를 열지 마세요.** [SECURITY.md](.github/SECURITY.md) — GitHub Security Advisory 또는 `security@keiailab.com` (PGP 서명) 사용. |
| "이게 X 처럼 동작하는 게 맞나요?" 또는 "Y 는 어떻게 설정하나요?" 류의 질문이 있습니다. | [GitHub Discussions](https://github.com/keiailab/valkey-operator/discussions). 검색 가능하며 향후 운영자들에게 색인됩니다. |
| 버그를 발견했습니다 — 문서와 다르게 동작합니다. | **Bug report** 템플릿을 사용해 [issue 를 여세요](https://github.com/keiailab/valkey-operator/issues/new/choose). |
| 기능 추가 또는 동작 변경을 원합니다. | **Feature request** 템플릿을 사용해 [issue 를 여세요](https://github.com/keiailab/valkey-operator/issues/new/choose). 이미 계획되어 있는지 [ROADMAP.md](ROADMAP.md) 를 먼저 확인하세요. |
| "이건 FAQ 에 있어야 한다" 류의 질문이 있습니다. | **Question** 템플릿을 사용해 [issue 를 여세요](https://github.com/keiailab/valkey-operator/issues/new/choose). |
| Prometheus 알람에 걸려서 MTTR 절차가 필요합니다. | [`docs/operations/runbook.md`](docs/operations/runbook.md) §9 (모든 알람의 `runbook_url` annotation 이 여기를 가리킵니다). |
| 알람은 없지만 이상한 동작이 보입니다. | [`docs/operations/troubleshooting.md`](docs/operations/troubleshooting.md) — 증상 → 원인 → 진단 → 조치 흐름도. |
| 코드나 문서에 기여하고 싶습니다. | [CONTRIBUTING.md](.github/CONTRIBUTING.md). |

## Before opening an issue, please (Issue 를 열기 전에)

1. [기존 issues](https://github.com/keiailab/valkey-operator/issues?q=is%3Aissue) 와 [Discussions](https://github.com/keiailab/valkey-operator/discussions) 를 검색하세요 — 이미 답변되었을 수 있습니다.
2. [troubleshooting 흐름도](docs/operations/troubleshooting.md) 를 시도하세요.
3. 보고서에 다음을 준비해 두세요:
   - `valkey-operator` 버전 (`kubectl get deploy -n valkey-operator-system -o jsonpath='{.items[0].spec.template.spec.containers[0].image}'`)
   - Kubernetes 버전 (`kubectl version`)
   - Helm chart 버전 (`helm list -A | grep valkey-operator`)
   - 만들 수 있는 가장 작은 재현 사례
   - `kubectl describe <Valkey|ValkeyCluster> <name>` 의 출력

## Response expectations (응답 기대치)

본 프로젝트는 best-effort 시간으로 유지되는 오픈소스 프로젝트입니다.
[GOVERNANCE.md](.github/GOVERNANCE.md) 가 의사결정 및
리뷰 프로세스를 설명합니다. 일반적으로 issue 에는 영업일 며칠 내로
응답합니다; 보안 보고는
[SECURITY.md](.github/SECURITY.md) 의 SLA 에 따라 처리됩니다 (initial ack 72 h 이내, severity triage
7 일 이내).

유료 지원 관계 또는 hard SLA 가 필요하다면
`security@keiailab.com` 으로 연락 주시면 옵션을 논의할 수 있습니다.

## Commercial vendors (상용 벤더)

`valkey-operator` 는 현재 유료 지원 벤더를 추천하지 않습니다.
변경 시 벤더의 약관과 지원하는
upstream 기능과 함께 항목이 추가됩니다.

## Code of Conduct (행동 강령)

위의 모든 채널은
[Code of Conduct](.github/CODE_OF_CONDUCT.md) 의 적용을 받습니다. 참여 전에
반드시 읽어 주세요.

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
