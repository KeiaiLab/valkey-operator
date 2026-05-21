# Valkey StatefulSet → ValkeyCluster Migration

> ValkeyCluster CR 도입 시 기존 StatefulSet 기반 배포를 *서비스 중단 없이* 이전하는 절차 카탈로그.

## 문서 (실행 순서)

| 문서 | 시나리오 |
|---|---|
| [zero-downtime.md](zero-downtime.md) | 일반 마이그레이션 (5단계, < 30분, 0초 다운타임) |
| [secondary-promote.md](secondary-promote.md) | Replica → Primary promote 후 기존 primary 폐기 (데이터 정합성 우선) |
| [rollback.md](rollback.md) | 마이그레이션 중 client 이상 발견 시 즉시 원복 |

## Pre-flight checklist

- [ ] ValkeyBackup CR 정상 동작 확인 (operator 설치 후)
- [ ] cert-manager Certificate Ready (operator webhook TLS)
- [ ] 기존 StatefulSet 의 PVC `accessModes: [ReadWriteOnce]`
- [ ] Application client 의 connection retry 정책 활성
- [ ] PrometheusAlert `ValkeyClusterDegraded` 알람 채널 확인

## SLO 요약

| 시나리오 | 다운타임 | 전체 시간 | 데이터 갭 |
|---|---|---|---|
| zero-downtime | 0초 | < 30분 | 0 |
| secondary-promote | 0초 (read-only window 5초) | < 15분 | 0 |
| rollback | < 5분 | < 10분 | < 30초 |

## Refs

- [ROADMAP.md](../../ROADMAP.md)
- [ADR-0015](../kb/adr/0015-valkeyrestore-init-container-pattern.md) (restore via init container)
- [ADR-0016](../kb/adr/0016-valkeybackuptarget-crd-external-storage.md) (ValkeyBackupTarget S3 abstraction)
