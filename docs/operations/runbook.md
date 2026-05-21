# Operations Runbook — valkey-operator

> 한국어 버전: [runbook.ko.md](runbook.ko.md)

Incident response and day-to-day operational procedures. Only the
core scenarios live here — defer detail to the cited ADRs / issues.

## 1. Health check

```sh
# Operator pod status
kubectl -n valkey-operator-system get pods -l control-plane=controller-manager

# Every CR's phase
kubectl get vk,vc,vkb,vbt,vkr -A

# Operator metrics (HTTPS:8443)
kubectl -n valkey-operator-system port-forward \
  svc/valkey-operator-controller-manager-metrics-service 8443:8443
curl -k https://localhost:8443/metrics | grep valkey_cluster_state_ok
```

## 2. General failure response

### 2.1 `Phase=Failed` on a CR

```sh
kubectl describe vk <name>     # Reason / Message in Status.Conditions
kubectl get events --field-selector involvedObject.name=<name>
```

Procedure:
1. Classify the `Reason` (TargetNotFound / AuthSecret / ConfigMap /
   TLS / …).
2. Resolve the cause, then either recreate the CR or trigger a
   re-reconcile with
   `kubectl annotate ... cache.keiailab.io/retry=true`.

### 2.2 Pod CrashLoopBackOff

```sh
kubectl logs <pod> -p          # previous container logs
kubectl logs <pod> -c valkey
```

Common root causes:

- TLS Secret not mounted → see ADR-0014
  (`/tls/tls.crt: No such file`).
- Auth-password mismatch → regenerate the Auth Secret (delete +
  recreate the CR).

### 2.3 `ValkeyCluster cluster_state=fail`

**Self-heal (ADR-0039, 2026-05-10)**: when the operator sees
`ClusterInitialized=true` but `cluster_state != ok` (or `slots !=
16384`), it automatically re-invokes `ensureClusterMeet`. This
section covers the case where self-heal itself fails (stuck for 5
minutes or more) and a human has to intervene.

```sh
PASS=$(kubectl get secret <name>-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster info
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster nodes
```

#### Diagnostic order

1. **Pods ready?**
   `kubectl get pod -n <ns> -l app.kubernetes.io/instance=<name>`.
   If any pod is not `Running 2/2`, fix that first (PVC pending,
   NetworkPolicy denial, etc.).
2. **Operator log**:
   `kubectl logs -n <op-ns> deploy/<op-name>` and look for the
   *INC-0001 self-heal* attempt line:
   ```
   ValkeyCluster post-init fail detected; attempting re-bootstrap (INC-0001 self-heal)
     state=fail slotsAssigned=0 slotsOK=0
   ```
   If this line repeats for 30 minutes or more, self-heal has
   failed — continue to step 3.
3. **`nodes.conf` `myself` IP**: on each pod's
   `/data/nodes.conf`, confirm the `myself` line's IP matches the
   actual pod IP (`kubectl get pod -o wide`). A mismatch means
   INC-0001 has recurred — go to step 4.

#### Manual recovery (INC-0001 pattern)

Data-loss assessment first:

```sh
# Key count per pod
for i in 0 1 2 3 4 5; do
  echo "pod-$i: $(kubectl exec <name>-$i -- valkey-cli -a "$PASS" dbsize | tail -1)"
done

# Sample keys (identify whether this is production data)
kubectl exec <name>-0 -- valkey-cli -a "$PASS" --scan | head -20
```

**If production data is present**: take a backup **before**
recovery (`make backup`, or `valkey-cli BGSAVE`). A `ValkeyBackup`
CR is the preferred path.

**If only test data is present (or loss is tolerated)**:

```sh
# 1. Wipe PVCs on all six pods (AOF + nodes.conf)
for i in 0 1 2 3 4 5; do
  kubectl exec <name>-$i -- sh -c 'rm -rf /data/appendonlydir /data/nodes.conf /data/dump.rdb'
done

# 2. Restart all pods at once (the controller recreates the STS)
kubectl delete pod <name>-0 <name>-1 <name>-2 <name>-3 <name>-4 <name>-5 --wait=false

# 3. Force ClusterInitialized=false (re-enter the controller's bootstrap path)
kubectl patch valkeycluster <name> --type=json --subresource=status \
  -p='[{"op":"replace","path":"/status/clusterInitialized","value":false},
       {"op":"replace","path":"/status/shards","value":[]},
       {"op":"replace","path":"/status/clusterState","value":""}]'

# 4. Mutate the spec to trigger a reconcile event
kubectl patch valkeycluster <name> --type=merge \
  -p '{"spec":{"nodeTimeoutMillis":15001}}'

# 5. Wait 60 s and verify
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster info
# Expect: cluster_state:ok, cluster_slots_assigned:16384, cluster_slots_ok:16384
```

#### References

- INC-0001:
  `docs/kb/incident/INC-0001-cluster-fail-bootstrap-skip.md`
  (2026-05-09, 19-hour failure).
- ADR-0039: `docs/kb/adr/0039-cluster-self-heal-post-init.md`
  (permanent fix).
- Alert: PrometheusRule `ValkeyClusterStateNotOK`
  (`for: 5m`, severity critical) — entry point for this section.

## 3. Backup / restore

### 3.1 Daily backup (PVC retained)

```sh
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata: { name: vkb-$(date +%Y%m%d), namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  type: RDB
  retainPVC: true
  ttl: 168h        # 7 days
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Completed valkeybackup/vkb-...
```

### 3.2 External-storage backup (S3)

```sh
# Prerequisite: create a ValkeyBackupTarget + credentials Secret
# (see config/samples/).
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
  ttl: 720h        # 30 days
EOF
```

### 3.3 Restore (disaster recovery)

**Warning**: `ValkeyRestore` **overwrites** the target cluster's
existing data. Take an independent backup first.

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

# Verify
kubectl get vkr vkr-recovery -o jsonpath='{.status.restoredKeys}'
```

Monitor progress with
`kubectl get vkr vkr-recovery -o jsonpath='{.status.phase}'` →
Pending → Mounting → Restoring → Verifying → Completed.

## 4. Scaling

### 4.1 Replication scale (replicas N → M)

```sh
kubectl patch vk valkey-prod --type=merge -p '{"spec":{"replicas":5}}'
# The operator applies the new STS replicas; new replicas wait
# until master_link_status reports up.
```

### 4.2 ValkeyCluster shard expansion

Not yet implemented — see ROADMAP Phase B (Track B). Manual path:

```sh
# Create new shard pods, then issue CLUSTER MEET and reshard
# manually. Operational guide tracked separately.
```

## 5. Upgrade

### 5.1 Valkey version upgrade

```sh
kubectl patch vk valkey-prod --type=merge -p '{"spec":{"version":{"version":"8.1.7"}}}'
# The operator sets Phase=Upgrading and performs an STS rolling
# restart. In Replication mode the order is replicas first, then
# primary — watch master_link_status throughout.
```

### 5.2 Operator version upgrade

`make deploy IMG=...` or the Helm chart (separate commits).

## 6. Emergency procedures

### 6.1 Force-restart the operator manager

```sh
kubectl -n valkey-operator-system rollout restart deploy/valkey-operator-controller-manager
```

### 6.2 Abort a misfired `ValkeyRestore`

```sh
# Restore adds an init container to the STS and sets the
# paused annotation. Manual cleanup (when the operator cannot
# self-recover):
kubectl delete vkr <name>                                # finalizer reverts the STS + removes paused
# If the finalizer is itself stuck (rare):
kubectl patch vkr <name> -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl annotate vk <target> cache.keiailab.io/paused-     # remove paused annotation manually
kubectl edit sts <target>                                  # remove the "valkey-restore-init" init container
```

### 6.3 Direct data-plane access

```sh
PASS=$(kubectl get secret <cr-name>-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec -it <cr-name>-0 -- valkey-cli -a "$PASS"
# With TLS enabled: valkey-cli --tls --cacert /tls/ca.crt --cert /tls/tls.crt --key /tls/tls.key -p 6380
```

## 7. Observability conventions

- **Metrics** (subsystem `valkey_cluster_*`): `state_ok`,
  `assigned_slots`, `shards`, `ready_replicas`, `reconcile_total`,
  `reconcile_errors_total`, `phase`, `backup_total`,
  `restore_total`, `failover_total`, `build_info` (cycle 57).
  ServiceMonitor is auto-registered when
  `Spec.Monitoring.ServiceMonitor.Enabled`.
- **Events**: `kubectl get events --field-selector involvedObject.kind=Valkey`.
- **Logs**: structured (`zap`).
  `kubectl logs <operator-pod> -f --tail=100`.

### 7.0 Prometheus ServiceMonitor TLS — production hardening (cycle 100)

**Default**: the ServiceMonitor in
`config/prometheus/monitor.yaml` ships with
`insecureSkipVerify: true` — Kubebuilder's default. **In production
this is a MITM attack surface.** Switch on cert-manager verification:

```sh
# 1. Install cert-manager cluster-wide first.
# 2. Uncomment the patches block in config/prometheus/kustomization.yaml.
sed -i '' 's|^#patches:|patches:|; s|^#  - path: monitor_tls_patch.yaml|  - path: monitor_tls_patch.yaml|; s|^#    target:|    target:|; s|^#      kind: ServiceMonitor|      kind: ServiceMonitor|' \
  config/prometheus/kustomization.yaml
# 3. Also uncomment the [METRICS WITH CERTMANAGER] patches in
#    config/default/kustomization.yaml.
# 4. Re-deploy via make build-installer or make deploy.
```

After validation, `monitor_tls_patch.yaml` flips to
`insecureSkipVerify: false` and references the
cert-manager-issued `metrics-server-cert` Secret, providing
**verifiable mutual TLS**. This is ADR-0003's
"TLS InsecureSkipVerify temporary" graduating to production.

### 7.1 Operator environment variables (cycle 80)

Use these to diagnose *which reconcilers are actually running* in a
given cluster:

| Env | Default | Effect |
|---|---|---|
| `ENABLE_CLUSTER_RECONCILER` | `true` | `false` skips the ValkeyClusterReconciler — auto-injected when chart `features.cluster.enabled=false`. |
| `ENABLE_BACKUP_RECONCILER` | `true` | `false` skips the ValkeyBackup / BackupTarget / Restore reconcilers — auto-injected when chart `features.backup.enabled=false`. |
| `ENABLE_WEBHOOKS` | `true` | `false` disables ValkeyWebhook + ValkeyClusterWebhook registration. **envtest only**; never set this in production. |
| `WATCH_NAMESPACES` | unset (cluster-wide) | Comma-separated list (`ns1,ns2`). Sets `cache.DefaultNamespaces` — auto-injected from chart `watch.namespaces`. |
| `OPERATOR_IMAGE` | `controller:latest` | Image used by the Upload/Download Jobs — auto-injected by the chart `valkey-operator.image` helper (cycle 64). Leaving it unset risks ImagePullBackOff. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | unset (no-op) | OTLP gRPC endpoint — auto-injected by chart `tracing.endpoint` (cycle 65). When unset, the 22 spans cost zero. |
| `OTEL_SERVICE_NAME` | `valkey-operator` | OTEL service identifier — auto-injected by chart `tracing.serviceName`. Used as the service id in Jaeger / Tempo UIs. |

**Note**: `ENABLE_*_RECONCILER` / `ENABLE_WEBHOOKS` are
**case-sensitive** — only the lowercase string `"false"` disables
them. Other casings (`"FALSE"`, `"False"`) or other "falsy" strings
(`"0"`, `"no"`) are treated as **enabled**, per Kubebuilder
convention.

Diagnostic commands:

```sh
# Current env
kubectl exec -n valkey-operator-system <operator-pod> -- env | \
  grep -E "ENABLE_|WATCH_NAMESPACES|OPERATOR_IMAGE|OTEL_"

# Startup log (which reconcilers were skipped, watch scope, version)
kubectl logs -n valkey-operator-system <operator-pod> | head -20
```

## 8. ADR / RFC references

- ADR-0010 cert-manager auto-detection / ADR-0013 always-on Auth /
  ADR-0014 TLS volume mount
- ADR-0015 Restore init-container pattern / ADR-0016
  `ValkeyBackupTarget`
- ADR-0022 minio-go / ADR-0023 subcommand pattern
- ADR-0045 GH Actions restoration / ADR-0046 SLSA-3 + cosign

Full INDEX: `docs/kb/adr/INDEX.md`.

## 9. Per-alert response (Prometheus alert → MTTR)

Every alert's `runbook_url` annotation points here. On-call walks
each alert as **Trigger → Diagnosis → Mitigation → Escalation**.

### 9.1 ValkeyClusterStateNotOK
- **Trigger**: `valkey_cluster_state_ok == 0` for 5m. `cluster_state` in `CLUSTER INFO` ≠ ok.
- **Self-heal (ADR-0039)**: the operator re-invokes
  `ensureClusterMeet` even after `ClusterInitialized=true`. Look for
  the "INC-0001 self-heal" line in the operator log. If recovery
  does not happen within the 5-minute window, self-heal itself has
  failed — switch to manual.
- **Diagnosis**: follow §2.3 ("ValkeyCluster cluster_state=fail")
  exactly (diagnostic order + manual recovery).
- **Mitigation**:
  1. Identify the missing slots and run `CLUSTER ADDSLOTS` (recovery
     expected within 5 minutes).
  2. If `nodes.conf` is stale, run the §2.3 "Manual recovery"
     procedure (PVC wipe + `clusterInitialized` reset). Back up data
     first.
- **References**: INC-0001, ADR-0039.

### 9.2 ValkeyClusterSlotsMismatch
- **Trigger**: `valkey_cluster_assigned_slots != 16384` for 5m.
- **Diagnosis**: check slot distribution with
  `valkey-cli cluster nodes`. May be a transient resharding.
- **Mitigation**: if it persists 5+ minutes, run `CLUSTER ADDSLOTS`
  manually or restart the operator.

### 9.3 ValkeyClusterNoReadyReplicas
- **Trigger**: `valkey_cluster_ready_replicas == 0` for 5m. Every pod NotReady.
- **Diagnosis**: §2.2 (CrashLoopBackOff) plus node-level signals
  (disk pressure, etc.).
  `kubectl get pods -l app.kubernetes.io/name=valkey` + describe.
- **Mitigation**: PVC re-bind, image-pull issues, OOMKilled, etc.
  Follow §2 per root cause.
- **Escalation**: if a cluster node is down, add a node or
  reschedule onto another.

### 9.4 ValkeyClusterDegraded
- **Trigger**: `0 < ready_replicas < 2` for 5m. Some pods NotReady.
- **Diagnosis**: logs + events on each NotReady pod.
- **Mitigation**: usually the §2.2 pattern.

### 9.5 ValkeyClusterPhaseFailed
- **Trigger**: `valkey_cluster_phase{phase="Failed"} == 1` for 1m.
- **Diagnosis**: §2.1 ("Phase=Failed CR"). Inspect `LastError` in
  Conditions.
- **Mitigation**: handle per error (typically admission / RBAC /
  StorageClass).

### 9.6 ValkeyOperatorReconcileErrorsHigh
- **Trigger**: `rate(valkey_cluster_reconcile_errors_total[5m]) > 0.1` for 5m.
- **Diagnosis**: grep `level=error` in operator logs + kubectl
  events. Typical: RBAC, API-server load, CR-validation rejections.
- **Mitigation**: transient ones self-recover. If persistent, run
  §6.1 (operator restart).

### 9.7 ValkeyOperatorDown
- **Trigger**: `up{job=~"valkey-operator.*"} == 0` for 2m.
- **Diagnosis**: §6.1 ("Force-restart the operator manager").
  Deployment Available status, Pod state, node state.
- **Mitigation**: rollout restart per §6.1. If `ImagePullBackOff`,
  check the image.
- **Escalation**: every reconcile is stopped — no new CRs, no phase
  transitions. SEV-1.

### 9.8 ValkeyBackupFailureRateHigh
- **Trigger**: `rate(valkey_cluster_backup_total{phase="Failed"}[1h]) > 0.0017` (~6/hour) for 10m.
- **Diagnosis**: §3 ("Backup / restore"). Look at `LastError` on
  the Failed `ValkeyBackup` plus the Job / Upload Pod logs.
  Credentials, S3 bucket permissions, or disk space are the usual
  suspects.
- **Mitigation**: rotate credentials or change the
  `BackupTarget` endpoint. Re-run after evaluating the impact on
  retention (TTL).

### 9.9 ValkeyRestoreFailureRateHigh
- **Trigger**: `rate(valkey_cluster_restore_total{phase="Failed"}[1h]) > 0.0017` for 10m.
- **Diagnosis**: §3.3 ("Restore"). Check source RDB integrity, init
  container logs, and the PVC ROX mount.
- **Mitigation**: abort the misfired Restore via §6.2 then re-run.
  Delete the Failed Restore CR after finalizer cleanup.

### 9.10 ValkeyFailoverHigh
- **Trigger**: `rate(valkey_cluster_failover_total[1h]) > 0.005` (~18/hour) for 10m.
- **Diagnosis**: frequent failover = primary instability. Check the
  primary for OOMKilled, network partition, or replication lag.
  `valkey-cli info replication`.
- **Mitigation**: adjust resource limits, audit network policy,
  shift load off the primary (use read replicas). Check disk-I/O
  bottlenecks.
- **Escalation**: suspected split-brain → §2.3 procedure plus
  ADR-0017 review.

### 9.11 ValkeyOperatorReconcileLatencyP95High
- **Trigger**: reconcile-success p95 > 1 s for 10m.
- **Diagnosis**: cluster API-server load, operator pod CPU
  throttling, or external-call timeouts inside the reconciler.
  `kubectl top pod -n valkey-operator-system` for CPU.
- **Mitigation**: raise the operator pod's `resources.requests.cpu`
  or shield it from another controller's API burst.

### 9.12 ValkeyOperatorReconcileLatencyP99Critical
- **Trigger**: reconcile (success + error) p99 > 5 s for 10m.
  Dangerous proximity to the controller-runtime default context
  timeout (30 s).
- **Diagnosis**: same as 9.11 but with **severe saturation**. Check
  the operator pod's state and the component-label distribution of
  `reconcile_errors_total`.
- **Mitigation**: restart the operator pod immediately
  (`kubectl rollout restart deploy/valkey-operator-controller-manager -n valkey-operator-system`).

### 9.13 ValkeyOperatorReconcileErrorRateHigh
- **Trigger**: reconcile error rate > 5 % for 10m.
- **Diagnosis**: split `valkey_cluster_reconcile_errors_total` by
  the `component` label to find which stage (secret / sts / svc /
  tls / backup) is failing.
- **Mitigation**: apply the §2.1 `Phase=Failed CR` procedure
  (classify by the Conditions Reason).

## 10. Replication mode automatic failover

ADR-0017 (replication failover — replica with the largest offset).
The operator owns the failover decision; there is no Sentinel sidecar.

### 10.1 Activation conditions

- `Spec.Mode == Replication` and `Spec.Replicas > 1`
- `Spec.AutoFailover == true` (default — set to `false` to disable)

### 10.2 Algorithm (7 steps)

1. Read the current primary (`Status.CurrentPrimary` or `<name>-0`)
   and verify pod readiness.
2. If `NotReady` for less than 30 s, treat as a transient flap and
   ignore.
3. Issue `INFO replication` against every replica (excluding the
   primary) and harvest `master_repl_offset`.
4. `selectFailoverCandidate` picks the replica with the largest
   offset (ties broken by smaller ordinal).
5. `PromoteToPrimary` issues `REPLICAOF NO ONE` against the candidate
   → it becomes the new primary.
6. For every other `Ready` replica, issue `EnsureReplicaOf <new-primary>`.
7. Update `Status.CurrentPrimary` in memory; the next reconcile
   persists it to the CR.

OTel spans for each phase: `Failover/INFO_replication`,
`Failover/PromoteToPrimary`, `Failover/EnsureReplicaOf_all`
(see [`observability/otel.md`](../observability/otel.md)).

### 10.3 Disabling

```yaml
spec:
  autoFailover: false   # operator skips the failover path; manual recovery only
```

### 10.4 Known limits

- **No hard split-brain guarantee under network partition** — two
  primaries can be elected. Mitigation: the 30 s `NotReady` threshold
  + operator leader election (single replica).
- **ValkeyCluster mode is N/A** — Valkey's native cluster mode uses
  `cluster_replica_validity_factor`. `ValkeyClusterReconciler` is
  intentionally not involved in failover.
- **No e2e automation yet** — replication-failover e2e is a separate cycle.

## 11. Verified operational scenarios

Verified behavior captured during release-validation runs. Each row
represents a scenario exercised against the live operator; the
"Behavior" column states what was observed.

| Scenario | Behavior | Data |
|---|---|---|
| primary pod kill (force) | STS re-creates; operator re-promotes pod-0 | PVC retained |
| replica scale up (3 → 5) | new replica auto-joins `master link up` | — |
| replica scale down (5 → 2) | surplus pods cleaned up | existing data retained |
| ValkeyCluster shard pod kill | `cluster_state=ok` holds (replica takes over) | all slots preserved |
| TLS + mTLS ValkeyCluster (cert-manager) | `Phase=Running`, `slots=16384`, data-plane SET/GET ✓ | — |
| TLS + mTLS Valkey Standalone (cert-manager) | `Phase=Running`, ping/set/get on port 6380 ✓ | — |
| TLS + mTLS Valkey Replication (3 replicas) | `master_link_status:up`, write propagation across replicas ✓ | — |
| ValkeyBackup (RDB) | Pending → InProgress → Completed; `/data/dump.rdb` 89 B generated | — |
| ValkeyBackup M3.5 (Job-based PVC) | Copying → Completed; `<name>-backup` PVC retains `dump.rdb` | TLS propagated automatically |
| ValkeyRestore (Standalone PVC) | Mounting → Restoring → Verifying → Completed; init container cp's `/data/dump.rdb` | populates `Status.RestoredKeys` |
| ValkeyRestore (`Source.TargetRef` S3) | ephemeral PVC + Download Job → standard init-container flow | cross-cluster restore |
| ValkeyRestore (ValkeyCluster, ROX) | shard ordinal → `SHARD_IDX` shell mapping → per-pod cp | ROX source PVC required |
| Replication auto failover (ADR-0017) | primary `NotReady` 30 s+ → largest-offset replica `REPLICAOF NO ONE` → `Status.CurrentPrimary` updated | e2e: `test/e2e/failover_test.go` |
| NetworkPolicy resource creation | selfPeer + 6379 / 16379 ingress + `ownerReferences` | (CNI-dependent) |
| Operator metrics endpoint (HTTPS:8443) | `controller_runtime_*` + `valkey_cluster_*` exposed | — |
| Prometheus alert rules | 13 alerts (state / slots / replicas / phase / errors / latency / operator down) | `config/prometheus/alert-rules.yaml` |
