# Troubleshooting — valkey-operator (한국어)

> English: [troubleshooting.md](troubleshooting.md) — canonical / 정본


운영 중 *증상 → 가능 원인 → 진단 명령 → 수습* 의 flow chart 형식. alert 별 MTTR
은 `runbook.md §9` 가 SSOT — 본 문서는 **alert 가 발화하지 않는 (또는 발화 전)
무지각 증상** 에 대응.

문제 발생 시 *증상 분류 → 본 문서의 §X.Y 절 → 진단 → 수습* 순으로 활용.

## 1. CR 가 만들어지지 않거나 phase 진입 실패

### 1.1 `kubectl apply` 직후 CR 자체가 거부됨

**증상**: `apply` 명령이 admission webhook 에러로 reject.

```sh
kubectl apply -f my-valkey.yaml
# Error: admission webhook "vvalkey-v1alpha1.kb.io" denied the request: ...
```

**가능 원인 + 진단**:

| 원인 | 검증 명령 |
|---|---|
| webhook configuration 미설치 / cert 미준비 | `kubectl get validatingwebhookconfiguration -l app.kubernetes.io/name=valkey-operator` + `kubectl describe ...` |
| `Spec.TLS.CertManager` + `CustomCert` 동시 명시 | `kubectl explain valkey.spec.tls` (mutually exclusive — ADR-0010 webhook validation) |
| Version 미지원 (allowlist 외) | `internal/version/matrix.go` 의 SupportedValkeyVersions 확인 (현 8.0.9 / 8.1.6 / 8.1.7 / 9.0.4) |
| ResourceQuota 위반 | `kubectl describe resourcequota -n <ns>` |

**수습**: 위 표의 해당 원인 fix → 재시도. webhook 자체 미동작 시 `runbook.md §6.1`
manager 강제 재시작.

### 1.2 CR 는 생성됐으나 phase 가 Pending 에서 진행 안 함

**증상**: `kubectl get vk` 가 5분 이상 `PHASE: Pending`.

**진단**:

```sh
kubectl describe vk <name>      # Events + Conditions
kubectl logs -n valkey-operator-system -l control-plane=controller-manager \
  --since=10m | grep "<ns>/<name>"
```

**자주 발생**:
- AuthSecret reference 가 다른 namespace → `Reason: SecretNotFound`. operator 는
  단일 namespace scope (cross-namespace secret 미지원).
- StorageClass 미지원 / `volumeBindingMode: WaitForFirstConsumer` + 노드 부족 →
  PVC 가 Pending → STS Pod Pending → reconcile 진척 없음.
- `ENABLE_CLUSTER_RECONCILER=false` 인데 `kind: ValkeyCluster` 생성 시도 (chart
  의 `features.cluster.enabled=false` 환경) → Helm install 시 가드, 수동 CR 시
  무동작.

## 2. 데이터 plane 응답 안 함 (cluster_state=ok 인데도)

### 2.1 client 연결 timeout

**증상**: cluster 는 healthy 인데 application 이 connection refused / timeout.

**진단 순서**:

1. **service endpoints 확인**:
   ```sh
   kubectl get svc,endpointslices -l cache.keiailab.io/cluster=<name>
   ```
   Endpoints 가 0 = pod label selector mismatch (operator 의 STS template 변경
   후 patched-only label 누락). reconcile force trigger:
   `kubectl annotate vk <name> cache.keiailab.io/retry=true --overwrite`.

2. **NetworkPolicy 차단 확인** (ADR-0035 AutoCreate 활성 시):
   ```sh
   kubectl get networkpolicy -l cache.keiailab.io/cluster=<name>
   kubectl describe networkpolicy <name>-allow
   ```
   ingress podSelector 가 client app 의 label 과 일치하지 않으면 deny-by-default.

3. **TLS handshake 실패** (TLS enabled cluster):
   ```sh
   kubectl exec -it <client-pod> -- valkey-cli -h <svc> -p 6379 --tls --insecure ping
   ```
   실패 시 `runbook.md §2.2` Pod CrashLoopBackOff 의 TLS Secret 미마운트 path.

### 2.2 일부 key 만 timeout (Cluster mode)

**증상**: 동일 cluster 에서 key A 는 OK, key B 는 MOVED 또는 timeout.

**원인**: 특정 shard 의 primary 가 응답 불가 + replica 가 자동 promotion 못함.

```sh
# 1. 어느 shard 의 어느 slot 인가
kubectl exec -it <pod> -- valkey-cli -c -h <svc> CLUSTER KEYSLOT <key>

# 2. 그 slot owner shard 식별
kubectl exec -it <pod> -- valkey-cli -c -h <svc> CLUSTER SLOTS | grep <slot번호>

# 3. owner shard pod 의 logs
kubectl logs <shard-N-0>
```

**수습**: master with keys 가 응답 불가 시 ADR-0017 Replica with Largest Offset
선출 정책에 따라 자동 failover. 5분 초과 시 `runbook.md §2.3` cluster_state=fail
회복 절차.

## 3. 성능 저하 (latency 상승)

### 3.1 reconcile thrashing

**증상**: operator pod CPU 100% + `valkey_cluster_reconcile_total` rate 폭증.

**진단**:

```sh
# rate 측정 (Prometheus)
rate(valkey_cluster_reconcile_total[1m])
# > 5/s 면 thrashing

# error component 식별
sum by (component) (rate(valkey_cluster_reconcile_errors_total[5m]))
```

**자주 발생**:
- Status update 무한 루프: spec drift 감지 → patch → reconcile → 동일 drift
  재감지. `metadata.generation` 변경 없는데 reconcile 재진입 시 의심.
- 외부 의존성 flapping: cert-manager Certificate Ready ↔ NotReady 전환 → operator
  마다 재배포.

**수습**: `kubectl logs ... --follow` 로 reconcile 사유 추적. drift 원인 fix
(보통 admission webhook 의 mutating defaulter 와 reconciler 의 desired state
mismatch).

### 3.2 데이터 plane 응답 latency 상승 (slow log)

**증상**: client p95 latency 평소 1ms → 100ms.

**원인 후보** (operator scope 밖이지만 진단 가능):

| 원인 | 진단 |
|---|---|
| AOF rewrite 진행 중 | `valkey-cli INFO persistence \| grep aof_rewrite_in_progress` |
| RDB snapshot 진행 중 | `valkey-cli INFO persistence \| grep rdb_bgsave_in_progress` |
| 큰 key 존재 (BIGKEY) | `valkey-cli --bigkeys` (sample) — 단일 key 가 100MB+ 면 slow O(N) 명령 노출 |
| memory swap 발생 | `kubectl top pod <pod>` + node `vmstat 1` |

수습 방향은 *데이터 plane tuning 영역* — 본 operator 는 직접 개입 안 함.
`runbook.md §6.3` 으로 직접 valkey-cli 진입 후 조사.

## 4. backup / restore 실패

### 4.1 ValkeyBackup phase=Failed

```sh
kubectl describe vkb <name>     # Conditions 의 Reason
kubectl logs job/<backup-job-name>
```

**자주 발생**:
- TargetRef 의 Secret credentials invalid → S3 client 가 `403 SignatureDoesNotMatch`.
  `kubectl describe vbt <target>` 의 `status.reachable` 확인.
- bucket 미존재 → `404 NoSuchBucket`. ValkeyBackupTarget 의 `spec.s3.bucket`
  검증.
- PVC 용량 부족 → `no space left on device`. RDB 크기 추정 = `INFO memory` 의
  `used_memory_rss`.

### 4.2 ValkeyRestore 무한 대기

ADR-B02 에 의해 RDB format mismatch (e.g. Redis 8.2.1 → Valkey 9.0.4) 는
*fail-fast* 처리됨 (Status.Phase=Failed). 그 외 대기:

- pod CrashLoopBackOff 가 5회 이상 = restore container 가 반복 fail. logs 의
  `Can't handle RDB format version` 등 명시적 사유 확인.
- TargetRef 의 RDB 다운로드 timeout (대용량 + 네트워크 느림) — Job 의 timeout
  spec 확장.

## 5. 일반 진단 명령 cheat-sheet

```sh
# 모든 CR phase 한눈에
kubectl get vk,vc,vkb,vbt,vkr -A -o custom-columns=NS:.metadata.namespace,KIND:.kind,NAME:.metadata.name,PHASE:.status.phase

# 최근 5분 reconcile 사유 (operator log)
kubectl -n valkey-operator-system logs -l control-plane=controller-manager --since=5m | jq -R 'fromjson? | select(.msg)' | head -50

# Conditions diff (이전 status vs 현재)
kubectl get vk <name> -o jsonpath='{.status.conditions}' | jq

# events 시간순
kubectl get events --field-selector involvedObject.name=<name> --sort-by='.lastTimestamp'
```

## 6. 최후 수단

- `runbook.md §6` Emergency 절차 (manager 강제 재시작 / Restore 강제 중단 / data
  plane 직접 접근)
- `INC-0001` 같은 cluster-fail 시나리오 → ADR-0039 self-heal 가 *대부분* 자동 수습.
  자동 수습 미발생 시 `runbook.md §2.3.수동 회복` 절차.

## 7. 수렴되지 않는 문제

같은 문제 3회 이상 재발 시 `docs/kb/incident/INC-NNNN-*.md` 작성 + 글로벌 standards
의 `incident-kb.md §2 작성 트리거` 적용. ADR 로 system 변경이 필요하면 RFC →
ADR → 코드 변경의 정공법.
