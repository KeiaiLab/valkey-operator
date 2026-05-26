<p align="center">
  <b>English</b> |
  <a href="i18n/ko/ADOPTERS.md">한국어</a> |
  <a href="i18n/ja/ADOPTERS.md">日本語</a> |
  <a href="i18n/zh/ADOPTERS.md">中文</a>
</p>

# Adopters of valkey-operator

> 한국어 버전: [ADOPTERS.ko.md](i18n/ko/ADOPTERS.md)

This document is the **public** list of organizations and projects
that run or evaluate `keiailab/valkey-operator`. Self-registration is
welcome — open a PR adding a row.

## Production users

Organizations running `valkey-operator` in production with a
production-grade SLA.

| User | Component | Usage pattern | First version | Current version | Listed |
|---|---|---|---|---|---|
| **Internal production cluster** ([keiailab](https://github.com/keiailab)) | Valkey 9.0.4 (Standalone + sharded Cluster 3×1) | Cache and pub/sub layer for an internal production workload. 6-pod ValkeyCluster, `cluster_state=ok`, ServiceMonitor + alert-rules.yaml + PodSecurity restricted. | v1.0.0 | v1.0.3 | 2026-05-07 |

## Evaluators

Proof-of-concept, evaluation, and external Redis-cluster migration
candidates.

| User | Stage | Notes |
|---|---|---|
| _Self-registration welcome_ | — | Open a PR to add a row. Note the Redis 8.2 → Valkey 9.0 RDB compatibility limit described in the ValkeyRestore docs. |

## How to add yourself

Open a PR that appends a row to one of the tables above:

```markdown
| **<organization or project>** ([profile](<URL>)) | <component + topology> | <usage pattern> | <first version> | <current version> | <YYYY-MM-DD> |
```

If you would rather be listed anonymously, reach out via the security
contact in [SECURITY.md](../.github/SECURITY.md) and a maintainer will register
an organization-anonymized row on your behalf.

## CNCF Sandbox reference

This list also serves as the public reference for the CNCF graduation
criterion "≥ 1 public adopter."

## Migrating from an external Redis-cluster chart

If you operate an external `redis-cluster` Helm chart (Redis 7.x / 8.x) and are
evaluating Valkey, see `ROADMAP.md` → **Phase B (RDB compatibility
and alternative migration paths)**. Some Redis 8.2.x RDB files cannot
be restored directly into Valkey 9.0.4; `ValkeyRestore` fails fast in
that case so operators never wait indefinitely on a silent error.
