# Migration Plan: bitnami/valkey → keiailab/valkey-operator

> **Status: Draft** — 2026-05-06. ralph-loop iter1. **운영 데이터 영향** = 사용자 명시 결정 후 실행.

## Context

argos 클러스터의 valkey 워크로드 현 상태 (kubectl 라이브 인벤토리):

```
$ kubectl get application platform-data-valkey -n argocd -o jsonpath='{.spec.source.helm.valueFiles}{"\n"}{.spec.source.path}{"\n"}'
[values.yaml]
valkey
$ kubectl get statefulset -n data | grep valkey
# (현재는 INC-0032 차단으로 valkey StatefulSet 미부팅)
```

운영 정의 (argos-platform-data/valkey/Chart.yaml):
- `argos-valkey` umbrella chart 가 `bitnami/valkey 5.6.1` (replication 1+1) 의존.
- Auth ESS = `infisical-cloud-valkey-shared-valkey-auth` (Infisical Cloud).
- 사용 처: GitLab Sidekiq queue + web session shared store (argos-platform-data/application.yaml 주석 인용).

마이그레이션 대상: `keiailab/valkey-operator` 0.1.0-alpha.1 + ValkeyCluster CR (sharded 3×1, ADR-0029, deploy/valkey-cluster.yaml).

## 의도된 변경 차이

| 항목 | bitnami (현) | keiailab (대상) | 영향 |
|---|---|---|---|
| 토폴로지 | replication primary 1 + replica 1 | sharded 3 shards × 1 replica = 6 노드 | **3배 노드 자원 + sharded slot 분배** |
| API | RESP, single endpoint | RESP, sharded cluster mode | **클라이언트 라이브러리 cluster-aware 필요** |
| Auth | ExternalSecret (Infisical) `valkey-password` | operator 자동 생성 (ADR-0013) | secret key 변경 → consumer 재구성 |
| TLS | 비활성 | 비활성 (cert-manager 미설정 환경) | 동일 |
| Storage | 8Gi (default) | 10Gi ceph-rbd | + 50Gi 총 (3 shard × 1 replica × 10Gi 추가 oversight) |
| Backup | 없음 | ValkeyBackupTarget CRD + S3 (ADR-0016) | 신규 자산 |

## 사전 조건 (마이그 전)

- [ ] **INC-0032 해소** — Infisical machine-identity Secret 회복으로 ExternalSecret 동작.
- [ ] **GitLab Sidekiq cluster-aware client** — Sidekiq 의 `redis://` connection string 이 redis-cluster 모드 라이브러리 (예: `redis-rb` 5.x 의 `:cluster` config) 지원 검증. *bitnami-replication 클라이언트는 sharded 호환 안 됨*.
- [ ] **운영 다운타임 윈도** 또는 **dual-write 패턴** 결정.
- [ ] valkey-operator F-* 진척 검증 — sharded cluster 의 CLUSTER MEET (ADR-0012 IP-based) + auth (ADR-0013) + replication failover (ADR-0017) 동작.
- [ ] data 백업 — `redis-cli --rdb` 또는 BGSAVE → S3 (ADR-0016 ValkeyBackupTarget 활용).

## 마이그레이션 절차 (4 phase)

### Phase 1: keiailab valkey-operator + 별도 ValkeyCluster 병렬 부팅 (zero downtime 시작)

1. argos-platform-data PR — `valkey-keiailab/` 디렉터리 신설 (별개 ApplicationSet path) + Chart.yaml 의존 = `keiailab/valkey-operator 0.1.0-alpha.1`. *기존 valkey/ 는 그대로 둠*.
2. ApplicationSet directories 에 `- path: valkey-keiailab` 추가 (활성). main → stable 승격.
3. ArgoCD sync → operator + ValkeyCluster CR (이름 = `valkey-keiailab`, ns=data) 생성. 6 pod 부팅 (3 shard × 2).
4. **검증 게이트**: `kubectl exec valkey-keiailab-0-0 -- valkey-cli cluster info` → `cluster_state:ok`. `valkey-cli cluster slots` → 16384 slot 분배 완료.

### Phase 2: 데이터 동기화 (BGSAVE + RESTORE 또는 redis-shake)

옵션 A — **BGSAVE/RESTORE** (다운타임 ~분, 데이터 < 1GB 추천):
1. `kubectl exec shared-valkey-primary-0 -- valkey-cli BGSAVE`
2. RDB 파일 추출 → `kubectl cp` 로 로컬 → `valkey-keiailab` cluster 로 `--pipe` 적용. 단 sharded → slot 재할당 필요. **redis-shake** (Alibaba 오픈소스) 가 sharded 호환.
3. `redis-shake` job (kubernetes Job) 으로 `bitnami → keiailab-cluster` mode=sync.

옵션 B — **dual-write (zero downtime, 복잡도 높음)**:
1. GitLab Sidekiq config 에 *secondary writer* 추가 — bitnami + keiailab 양쪽 write.
2. read 는 bitnami 우선, *miss 시 keiailab fallback* + warm-up.
3. 점진 read traffic shift 24~48h.
4. consumer 절단 후 bitnami archive.

### Phase 3: Cutover

1. GitLab Sidekiq/Web `REDIS_URL` 을 `valkey-keiailab` headless Service 로 교체 (ConfigMap/Secret 갱신).
2. Sidekiq/Web rollout — 각 pod 재시작 + redis-cluster 재연결 검증.
3. 30분 모니터링 — Sidekiq job retry rate, web session error rate, valkey cluster_state.

### Phase 4: bitnami 폐기

1. argos-platform-data application.yaml 의 `- path: valkey` 라인 *주석 처리* (또는 제거).
2. main → stable 승격.
3. ApplicationSet `platform-data-valkey` 자동 prune → bitnami StatefulSet/Service/PVC 삭제.
4. PV reclaim (ceph-rbd Retain 정책 → manual delete).

## 롤백 절차

각 phase 별 rollback:

| Phase | rollback 명령 | 영향 |
|---|---|---|
| Phase 1 | `- path: valkey-keiailab` 라인 제거 + stable 승격 | keiailab cluster prune. bitnami 영향 0. |
| Phase 2 | redis-shake job 중단 + RDB 적용 결과 `FLUSHALL` (keiailab cluster) | data loss (keiailab 측만). bitnami 무영향. |
| Phase 3 | Sidekiq/Web `REDIS_URL` 원복 + rollout | 재연결 시간 (~분). 일부 in-flight job 손실 가능. |
| Phase 4 | git revert 후 stable 승격 → bitnami 복원 시도 | PVC 재생성 시 데이터 손실 (Retain 정책 의존). 권장 안 됨 — Phase 4 는 *비가역적*. |

## 자율 진행 한계

본 plan 의 적용은 **사용자 명시 결정 영역**:

- **GitLab 운영 영향** (Sidekiq queue 손실 → CI 작업 retry 폭증).
- **Infisical 의존** (ESS migration + secret key 변경 — INC-0032 해소 선결).
- **redis-shake or dual-write 복잡도** — 운영 검증 도구 (latency, sharded slot consistency).
- **다운타임 윈도** — argos 사용자 트래픽 패턴 의존.

ralph-loop 자율 위임 범위가 *비가역 운영 변경* 까지 확장되지 않으므로, 본 plan 은 **draft 상태로 보관** 하고 사용자 명시 GO 후 실행. 실행 시 별도 ralph-loop iteration 또는 run-book 으로 분리.

## Refs

- ADR-0013: auth always-enabled
- ADR-0016: ValkeyBackupTarget CRD (S3 외부 저장)
- ADR-0017: Replication failover (replica with largest offset)
- ADR-0029: GitOps deploy 오버레이 도입
- INC-0032: Infisical machine-identity Secret 부재 (마이그 사전 조건 해소 게이트)
- argos-platform-data/application.yaml: ApplicationSet 정의
- argos-platform-data/valkey/Chart.yaml: 현 bitnami umbrella
