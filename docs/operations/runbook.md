# 운영 Runbook — valkey-operator

장애 대응 + 일상 운영 절차. 핵심 시나리오만 — 디테일은 각 ADR / Issue 참조.

## 1. Health Check

```sh
# operator pod 상태
kubectl -n valkey-operator-system get pods -l control-plane=controller-manager

# CR 전체 phase
kubectl get vk,vc,vkb,vbt,vkr -A

# operator metrics (HTTPS:8443)
kubectl -n valkey-operator-system port-forward svc/valkey-operator-controller-manager-metrics-service 8443:8443
curl -k https://localhost:8443/metrics | grep valkey_cluster_state_ok
```

## 2. 일반 장애 대응

### 2.1 Phase=Failed CR

```sh
kubectl describe vk <name>     # Status.Conditions 의 Reason/Message
kubectl get events --field-selector involvedObject.name=<name>
```

대응:
1. `Reason` 분류 (TargetNotFound / AuthSecret / ConfigMap / TLS / ...)
2. 원인 해소 후 CR 재생성 또는 `kubectl annotate cache.keiailab.io/retry=true`

### 2.2 Pod CrashLoopBackOff

```sh
kubectl logs <pod> -p          # 이전 컨테이너 logs
kubectl logs <pod> -c valkey
```

자주 발생:
- TLS Secret 미마운트 → ADR-0014 참조 (`/tls/tls.crt: No such file`)
- Auth password 불일치 → Auth Secret 재생성 (CR delete + recreate)

### 2.3 ValkeyCluster cluster_state=fail

```sh
PASS=$(kubectl get secret <name>-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster info
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster nodes
```

대응:
1. 슬롯 분배 확인 (16384 vs 실제) — `cluster_slots_assigned` 점검
2. 노드 멤버십 — `cluster nodes` 의 master/replica 매핑
3. 필요 시 `cluster reset` (주의: 데이터 손실 가능) — 운영 데이터 백업 후만

## 3. Backup / Restore

### 3.1 일상 백업 (PVC 보존)

```sh
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata: { name: vkb-$(date +%Y%m%d), namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  type: RDB
  retainPVC: true
  ttl: 168h        # 7일
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Completed valkeybackup/vkb-...
```

### 3.2 외부 저장 백업 (S3)

```sh
# 사전: ValkeyBackupTarget + 자격증명 Secret 생성 (config/samples/ 참조)
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata: { name: vkb-s3-$(date +%Y%m%d), namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  destination:
    type: TargetRef
    targetRef:
      name: s3-prod
      path: $(date +%Y/%m/%d)/dump.rdb
  ttl: 720h        # 30일
EOF
```

### 3.3 Restore (재해 복구)

**주의**: Restore 는 *대상 cluster 의 기존 데이터 덮어씀*. 사전에 별도
backup 권장.

```sh
# Standalone Valkey, PVC source.
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata: { name: vkr-recovery, namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  source:
    pvc:
      name: vkb-20260506-backup
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Completed valkeyrestore/vkr-recovery --timeout=10m

# 검증
kubectl get vkr vkr-recovery -o jsonpath='{.status.restoredKeys}'
```

진행 단계 모니터링: `kubectl get vkr vkr-recovery -o jsonpath='{.status.phase}'`
→ Pending → Mounting → Restoring → Verifying → Completed.

## 4. Scaling

### 4.1 Replication 확장 (replicas N → M)

```sh
kubectl patch vk valkey-prod --type=merge -p '{"spec":{"replicas":5}}'
# operator 가 STS replicas 적용 → 새 replica 가 master_link_status:up 까지 대기.
```

### 4.2 ValkeyCluster shard 확장

현재 미구현 — Plan §3 Track B 참조. 수동:
```sh
# 새 shard pod 생성 후 수동 cluster meet + reshard. 운영 가이드 별개.
```

## 5. Upgrade

### 5.1 Valkey 버전 업그레이드

```sh
kubectl patch vk valkey-prod --type=merge -p '{"spec":{"version":{"version":"8.1.7"}}}'
# operator 가 Phase=Upgrading set + STS rolling restart.
# Replication 모드: replica → primary 순으로 rolling. master_link_status 모니터링.
```

### 5.2 operator 버전 업그레이드

`Makefile deploy IMG=...` 또는 Helm chart (별개 commit).

## 6. 응급 조치 (Emergency)

### 6.1 Operator manager 강제 재시작

```sh
kubectl -n valkey-operator-system rollout restart deploy/valkey-operator-controller-manager
```

### 6.2 잘못된 ValkeyRestore 중단

```sh
# Restore 가 STS 에 init container 추가 + paused annotation set 한 상태.
# 수동 정리 (operator 가 정상 처리 못 할 경우):
kubectl delete vkr <name>                                # finalizer 가 STS 원복 + paused 제거
# 만약 finalizer 가 멈춰 있으면 (rare):
kubectl patch vkr <name> -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl annotate vk <target> cache.keiailab.io/paused-     # paused annotation 수동 제거
kubectl edit sts <target>                                # init container "valkey-restore-init" 제거
```

### 6.3 데이터 plane 직접 접근

```sh
PASS=$(kubectl get secret <cr-name>-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec -it <cr-name>-0 -- valkey-cli -a "$PASS"
# TLS 활성 시: valkey-cli --tls --cacert /tls/ca.crt --cert /tls/tls.crt --key /tls/tls.key -p 6380
```

## 7. 관측성 표준

- **Metrics**: Prometheus `valkey_cluster_state_ok`, `valkey_assigned_slots`,
  `valkey_ready_replicas`, `valkey_reconcile_total`, `valkey_reconcile_errors`,
  `valkey_phase`. ServiceMonitor 자동 등록 (`Spec.Monitoring.ServiceMonitor.Enabled`).
- **Events**: `kubectl get events --field-selector involvedObject.kind=Valkey`.
- **Logs**: 구조화 (zap). `kubectl logs <operator-pod> -f --tail=100`.

## 8. ADR / RFC 참조

- ADR-0010 cert-manager 자동 인식 / ADR-0013 Auth 강제 / ADR-0014 TLS volume mount
- ADR-0015 Restore Init container 패턴 / ADR-0016 ValkeyBackupTarget
- ADR-0022 minio-go / ADR-0023 sub-command pattern

전체 INDEX: `docs/kb/adr/INDEX.md`.
