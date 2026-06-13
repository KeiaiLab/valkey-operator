# Security Policy

To report a vulnerability: **do not open a public issue.** Use
[GitHub Security Advisories](https://github.com/KeiaiLab/valkey-operator/security/advisories/new)
or email `security@keiailab.com`.

## Supported Versions

The latest minor release receives security fixes. Older versions are best-effort.

## Scope

- Operator controller — RBAC, ServiceAccount token mounting, webhook validation.
- Generated Valkey workloads — PodSecurityContext, NetworkPolicy, TLS.
- Helm chart defaults — least-privilege RBAC, `automountServiceAccountToken`.
