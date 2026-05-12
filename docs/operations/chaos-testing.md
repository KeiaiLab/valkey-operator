# Chaos testing — valkey-operator

> 한국어 버전: [chaos-testing.ko.md](chaos-testing.ko.md)

How to run the ADR-0041 chaos-mesh-based 4-scenario chaos-engineering
end-to-end suite.

## Prerequisites

1. **Kind cluster** (or any Kubernetes) up:
   ```sh
   make setup-test-e2e   # or: kind create cluster --name valkey-e2e
   ```

2. **Deploy valkey-operator**:
   ```sh
   make docker-build IMG=ghcr.io/keiailab/valkey-operator:e2e-dev
   make deploy IMG=ghcr.io/keiailab/valkey-operator:e2e-dev
   ```

3. **Install chaos-mesh**:
   ```sh
   make chaos-mesh-install
   # Manual: kubectl apply -f https://mirrors.chaos-mesh.org/v2.7.2/chaos-mesh.yaml
   ```

4. **Target ValkeyCluster** (namespace `valkey-chaos-e2e`):
   ```yaml
   apiVersion: cache.keiailab.io/v1alpha1
   kind: ValkeyCluster
   metadata: { name: vc-chaos, namespace: valkey-chaos-e2e }
   spec:
     shards: 3
     replicasPerShard: 1
     autoFailover: true
     version: { version: "9.0.4" }
   ```

## Run

```sh
make chaos-e2e
# Or override the target namespace:
CHAOS_TEST_NAMESPACE=my-ns make chaos-e2e
```

## Scenarios (4)

| ID | Chaos type | Behaviour | Recovery check |
|---|---|---|---|
| 1 | PodChaos (pod-kill) | Random pod kill every 1 m for 5 m total | `cluster_state=ok` recovers within 5 m |
| 2 | NetworkChaos (partition) | 30 s master ↔ replica partition | Failover or recovery within 3 m |
| 3 | IOChaos (`ENOSPC` fault) | 60 s simulated 80 %-full disk | Cluster degraded but healthy within 3 m |
| 4 | IOChaos (latency) | 60 s of 100 ms I/O delay on a replica | Master unaffected (no failover) within 3 m |

Each scenario applies the chaos CR → time elapses → automatic
cleanup → verifies the cluster recovers to healthy. The
`BeforeSuite` `vc-chaos` CR is preserved across scenarios.

## Operational integration

- **Developer local**: recommended after any reconciler change
  (full e2e + chaos ≈ 30 min).
- **CI nightly**: ADR-0041 AI-005 (separate follow-up) — automated
  after the CI infra work lands.
- **Production debug**: chaos-mesh **never** runs directly in
  production. Staging / pre-prod only.

## Cleanup

```sh
make chaos-mesh-uninstall
kubectl delete namespace valkey-chaos-e2e
```

## Adding a scenario

- New chaos CRDs: adopt other `chaos-mesh.org/v1alpha1` kinds
  (`TimeChaos`, `DNSChaos`, `KernelChaos`, etc.).
- Pattern: add a new `var _ = Describe(...)` block in
  `test/chaos/scenarios_test.go` using the `makeChaos(kind, name, ns,
  spec)` helper.
- chaos-mesh CRD spec reference: <https://chaos-mesh.org/docs/>

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `chaos-mesh.org/v1alpha1: NoMatchError` | chaos-mesh CRDs missing — `make chaos-mesh-install` |
| `kubectl apply` permission denied | The chaos-mesh controller lacks **namespace permissions**. Check the `--local kind` install option. |
| Scenario times out | Cluster size / image pull is slow — re-run with `--timeout=30m` or longer. |
| Pod stuck `Terminating` | Finalizer needs to be cleared — `kubectl patch pod ... --type=merge -p '{"metadata":{"finalizers":[]}}'` |

## References

- ADR-0041 — chaos-mesh adoption rationale + candidate comparison.
- ADR-0040 §gap #4 — chaos engineering e2e.
- chaos-mesh: <https://chaos-mesh.org/>
- Makefile targets: `chaos-mesh-install`, `chaos-mesh-uninstall`,
  `chaos-e2e`.
