# Troubleshooting — valkey-operator

> 한국어 버전: [troubleshooting.ko.md](troubleshooting.ko.md)

A symptom → likely cause → diagnostic command → remediation flowchart
for live operations. Alert-driven MTTR procedures live in
`runbook.md §9` (the SSOT); this document covers the **symptoms that
fire no alert** (or fire before an alert exists).

Use it as: classify the symptom → jump to the relevant §X.Y →
diagnose → remediate.

## 1. The CR never reaches a phase

### 1.1 `kubectl apply` is rejected by the webhook

**Symptom**: `apply` returns an admission-webhook error.

```sh
kubectl apply -f my-valkey.yaml
# Error: admission webhook "vvalkey-v1alpha1.kb.io" denied the request: ...
```

**Possible causes + diagnostics**:

| Cause | Verification |
|---|---|
| Webhook configuration missing or cert not ready | `kubectl get validatingwebhookconfiguration -l app.kubernetes.io/name=valkey-operator` + `kubectl describe ...` |
| `Spec.TLS.CertManager` **and** `CustomCert` set together | `kubectl explain valkey.spec.tls` (mutually exclusive per ADR-0010 webhook validation) |
| Unsupported Valkey version (not on the allowlist) | Check `internal/version/matrix.go::SupportedValkeyVersions` (currently `8.0.9` / `8.1.6` / `8.1.7` / `9.0.4`) |
| ResourceQuota violation | `kubectl describe resourcequota -n <ns>` |

**Fix**: address the matching row above and re-apply. If the webhook
itself is misbehaving, follow `runbook.md §6.1` to force-restart the
manager.

### 1.2 CR is created but the phase stays `Pending`

**Symptom**: `kubectl get vk` shows `PHASE: Pending` for more than 5
minutes.

**Diagnostics**:

```sh
kubectl describe vk <name>      # Events + Conditions
kubectl logs -n valkey-operator-system -l control-plane=controller-manager \
  --since=10m | grep "<ns>/<name>"
```

**Frequent root causes**:

- AuthSecret references a `Secret` in another namespace →
  `Reason: SecretNotFound`. The operator is namespace-scoped;
  cross-namespace Secret references are not supported.
- StorageClass unsupported, or
  `volumeBindingMode: WaitForFirstConsumer` + no available node →
  PVC stays Pending → STS Pod stays Pending → reconcile makes no
  progress.
- `ENABLE_CLUSTER_RECONCILER=false` but a `kind: ValkeyCluster` is
  created (chart `features.cluster.enabled=false` environment). Helm
  installs guard this; a hand-written CR will sit silently.

## 2. Data plane unresponsive even with `cluster_state=ok`

### 2.1 Client connection timeout

**Symptom**: the cluster reports healthy, but the application sees
connection refused / timeout.

**Order of diagnosis**:

1. **Service endpoints**:
   ```sh
   kubectl get svc,endpointslices -l cache.keiailab.io/cluster=<name>
   ```
   `Endpoints: 0` means a label-selector mismatch (the operator
   changed the STS template but a patched-only label was missed).
   Force a reconcile:
   `kubectl annotate vk <name> cache.keiailab.io/retry=true --overwrite`.

2. **NetworkPolicy denial** (when ADR-0035 AutoCreate is on):
   ```sh
   kubectl get networkpolicy -l cache.keiailab.io/cluster=<name>
   kubectl describe networkpolicy <name>-allow
   ```
   If the ingress `podSelector` does not match the client pod's
   labels, traffic is denied by default.

3. **TLS handshake failure** (TLS-enabled clusters):
   ```sh
   kubectl exec -it <client-pod> -- valkey-cli -h <svc> -p 6379 --tls --insecure ping
   ```
   On failure, follow `runbook.md §2.2` for the Pod
   CrashLoopBackOff / TLS Secret-not-mounted path.

### 2.2 Some keys time out (Cluster mode)

**Symptom**: in the same cluster, key A succeeds while key B returns
MOVED or times out.

**Cause**: one shard's primary is unreachable and its replica has
not auto-promoted yet.

```sh
# 1. Which slot is the key on?
kubectl exec -it <pod> -- valkey-cli -c -h <svc> CLUSTER KEYSLOT <key>

# 2. Which shard owns that slot?
kubectl exec -it <pod> -- valkey-cli -c -h <svc> CLUSTER SLOTS | grep <slot>

# 3. Inspect that shard pod's logs
kubectl logs <shard-N-0>
```

**Remediation**: when the primary that holds those keys is
unreachable, ADR-0017 promotes the replica with the largest offset
automatically. If recovery has not happened after 5 minutes, follow
the `runbook.md §2.3` `cluster_state=fail` recovery procedure.

## 3. Performance degradation (rising latency)

### 3.1 Reconcile thrashing

**Symptom**: the operator pod sits at 100 % CPU and the
`valkey_cluster_reconcile_total` rate spikes.

**Diagnostics**:

```sh
# Rate, in Prometheus
rate(valkey_cluster_reconcile_total[1m])
# > 5/s is thrashing

# Which component is erroring?
sum by (component) (rate(valkey_cluster_reconcile_errors_total[5m]))
```

**Common root causes**:

- Status-update loop: the operator detects spec drift → patches →
  reconciles → detects the same drift again. Suspect this if
  `metadata.generation` does not change but reconciles re-enter.
- A flapping external dependency: cert-manager `Certificate` toggling
  between `Ready` and `NotReady` causes operator re-runs every cycle.

**Fix**: `kubectl logs ... --follow` to trace the reconcile reason.
Resolve the drift root cause — usually a mismatch between the
admission webhook's mutating defaulter and the reconciler's desired
state.

### 3.2 Data-plane latency increase (slow log)

**Symptom**: client p95 latency climbs from ~1 ms to ~100 ms.

**Likely causes (outside the operator's scope but still
diagnosable)**:

| Cause | Diagnostic |
|---|---|
| AOF rewrite in progress | `valkey-cli INFO persistence \| grep aof_rewrite_in_progress` |
| RDB snapshot in progress | `valkey-cli INFO persistence \| grep rdb_bgsave_in_progress` |
| Big key present (BIGKEY) | `valkey-cli --bigkeys` — a single 100 MB+ key exposes slow O(N) commands |
| Memory swap | `kubectl top pod <pod>` and `vmstat 1` on the node |

Tuning here is a **data-plane** decision; the operator does not
intervene directly. Drop into `runbook.md §6.3` and investigate via
`valkey-cli` directly.

## 4. Backup / restore failures

### 4.1 `ValkeyBackup phase=Failed`

```sh
kubectl describe vkb <name>     # Reason in Conditions
kubectl logs job/<backup-job-name>
```

**Common root causes**:

- `TargetRef` Secret credentials invalid → the S3 client returns
  `403 SignatureDoesNotMatch`. Check `status.reachable` on
  `kubectl describe vbt <target>`.
- Bucket does not exist → `404 NoSuchBucket`. Verify
  `spec.s3.bucket` on the `ValkeyBackupTarget`.
- PVC out of space → `no space left on device`. Estimate the RDB
  size as `used_memory_rss` from `INFO memory`.

### 4.2 `ValkeyRestore` hangs forever

ADR-B02 fails-fast on an RDB format mismatch (e.g. Redis 8.2.1 →
Valkey 9.0.4) and sets `Status.Phase=Failed`. Other hang patterns:

- Pod CrashLoopBackOff ≥ 5 times = the restore container keeps
  failing. Check logs for explicit reasons such as
  `Can't handle RDB format version`.
- `TargetRef` RDB download timeout (large object + slow link) —
  raise the Job's timeout spec.

## 5. General diagnostic cheat-sheet

```sh
# Every CR's phase at a glance
kubectl get vk,vc,vkb,vbt,vkr -A \
  -o custom-columns=NS:.metadata.namespace,KIND:.kind,NAME:.metadata.name,PHASE:.status.phase

# Last 5 minutes of reconcile reasons (operator log)
kubectl -n valkey-operator-system logs -l control-plane=controller-manager --since=5m \
  | jq -R 'fromjson? | select(.msg)' | head -50

# Conditions diff (previous vs current status)
kubectl get vk <name> -o jsonpath='{.status.conditions}' | jq

# Recent events
kubectl get events --field-selector involvedObject.name=<name> --sort-by='.lastTimestamp'
```

## 6. Last-resort procedures

- `runbook.md §6` for emergency procedures (force-restart the
  manager, abort a restore, drop into the data plane directly).
- `INC-0001` style cluster-fail scenarios → ADR-0039 self-heal
  recovers **most** automatically; if it does not, follow
  `runbook.md §2.3` manual recovery.

## 7. When a problem refuses to converge

If the same issue recurs three or more times, open
`docs/kb/incident/INC-NNNN-*.md` per the global
`incident-kb.md §2 trigger`. If a system change is required, follow
the formal path: RFC → ADR → code change.
