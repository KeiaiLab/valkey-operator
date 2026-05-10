# Incident Knowledge Base — INDEX

본 디렉터리는 valkey-operator 의 운영 incident 를 비난 없는 postmortem-lite
형식으로 보존한다. 글로벌 표준: `~/Documents/ai-dev/standards/incident-kb.md`.

| ID | 제목 | Severity | Detected | Resolved |
|---|---|---|---|---|
| [INC-0001](INC-0001-cluster-fail-bootstrap-skip.md) | ValkeyCluster cluster_state:fail 상태에서 bootstrap 재실행 안 됨 | SEV-2 | 2026-05-09 14:27 KST | 2026-05-10 09:18 KST |

## 작성 가이드

- 형식: 글로벌 `standards/incident-kb.md §3` (Postmortem-lite).
- 트리거: 운영 장애 / 보안 사건 / 30분+ 디버깅한 비명백 버그 / 패턴 발견 (3회 재발).
- 비난 없는 문화: *시스템* 의 어디가 실패를 허용했는가. Action Items 는 *시스템 변경* 우선.
- KB 신선도: 30일 미수정 INC 가 30%+ 면 알림 (글로벌 §6).
