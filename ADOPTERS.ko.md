<p align="center">
  <a href="ADOPTERS.md">English</a> |
  <b>한국어</b> |
  <a href="ADOPTERS.ja.md">日本語</a> |
  <a href="ADOPTERS.zh.md">中文</a>
</p>

# Adopters of valkey-operator (한국어)

> English: [ADOPTERS.md](ADOPTERS.md) — canonical / 정본


본 문서는 `keiailab/valkey-operator` 를 운영 환경 또는 평가 환경에서 사용하는 조직/프로젝트의 *공개* 목록입니다. 자가 등록을 환영합니다 — PR 로 row 를 추가해주세요.

## Production Users

운영 환경에서 valkey-operator 를 *production-grade SLA* 로 사용하는 사용자.

| 사용자 | 컴포넌트 | 사용 패턴 | 시작 버전 | 현재 버전 | 등재 일자 |
|---|---|---|---|---|---|
| **argos-platform-data** ([keiailab](https://github.com/keiailab)) | Valkey 9.0.4 (Standalone + Cluster sharded 3×1) | argos 의 캐시 / pub-sub 레이어. ValkeyCluster 6 pod, `cluster_state=ok`, ServiceMonitor + alert-rules.yaml + PodSecurity restricted. | v1.0.0 | v1.0.3 | 2026-05-07 |

## Evaluators

POC / 평가 / Bitnami redis-cluster 마이그레이션 검토 사용자.

| 사용자 | 단계 | 비고 |
|---|---|---|
| _자가 등록 환영_ | — | PR 로 row 추가. Redis 8.2 → Valkey 9.0 RDB 호환성 제약 (ValkeyRestore docs 참조) |

## How to add yourself

PR 을 열어 위 표에 한 row 추가:

```markdown
| **<조직 / 프로젝트>** ([profile](<URL>)) | <컴포넌트 + 토폴로지> | <사용 패턴> | <시작 버전> | <현재 버전> | <등재 일자 YYYY-MM-DD> |
```

비공개 또는 익명 등재를 원하시면 SECURITY.md 의 보안 채널로 알려주시면 maintainer 가 *organization-anonymized* row 로 등재합니다.

## CNCF Sandbox Reference

본 ADOPTERS 목록은 CNCF graduation criteria 의 "≥1 public adopter" 요구사항을 충족하기 위한 공개 reference 로도 활용됩니다.

## Bitnami redis-cluster Migration

Bitnami redis-cluster (Redis 7.x/8.x) 사용자가 Valkey 로 migration 검토 시 ROADMAP.md 의 *Phase B (RDB 호환성 / 대안 마이그레이션 경로)* 섹션 참조. 일부 Redis 8.2.x RDB 는 Valkey 9.0.4 직접 restore 불가 — `ValkeyRestore` 가 fail-fast 처리하므로 운영자가 무한 대기하지 않음.

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
