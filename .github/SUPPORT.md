<p align="center">
  <b>English</b> |
  <a href="../docs/i18n/ko/SUPPORT.md">한국어</a> |
  <a href="../docs/i18n/ja/SUPPORT.md">日本語</a> |
  <a href="../docs/i18n/zh/SUPPORT.md">中文</a>
</p>

# Support

Thanks for using `valkey-operator`. This page explains where to get
help.

## Decide what you need

| Situation | Where to go |
|---|---|
| **You think you found a security vulnerability.** | **Do not open a public issue.** Use [SECURITY.md](SECURITY.md) — GitHub Security Advisory or `security@keiailab.com` (PGP signed). |
| You have an "is this supposed to work like X?" or "how do I configure Y?" question. | [GitHub Discussions](https://github.com/keiailab/valkey-operator/discussions). Searchable and indexed by future operators. |
| You found a bug — something behaves differently from the docs. | [Open an issue](https://github.com/keiailab/valkey-operator/issues/new/choose) using the **Bug report** template. |
| You want a feature added or behaviour changed. | [Open an issue](https://github.com/keiailab/valkey-operator/issues/new/choose) using the **Feature request** template. Check [ROADMAP.md](../docs/ROADMAP.md) first to see if it's already planned. |
| You have a "this should be in the FAQ" question. | [Open an issue](https://github.com/keiailab/valkey-operator/issues/new/choose) using the **Question** template. |
| You're hitting a Prometheus alert and need the MTTR procedure. | [`docs/operations/runbook.md`](../docs/operations/runbook.md) §9 (every alert's `runbook_url` annotation points there). |
| You're seeing odd behaviour but no alert. | [`docs/operations/troubleshooting.md`](../docs/operations/troubleshooting.md) — symptom → cause → diagnostic → remediation flowchart. |
| You want to contribute code or docs. | [CONTRIBUTING.md](CONTRIBUTING.md). |

## Before opening an issue, please

1. Search [existing issues](https://github.com/keiailab/valkey-operator/issues?q=is%3Aissue) and [Discussions](https://github.com/keiailab/valkey-operator/discussions) — your question may already be answered.
2. Try the [troubleshooting flowchart](../docs/operations/troubleshooting.md).
3. Have the following ready in your report:
   - `valkey-operator` version (`kubectl get deploy -n valkey-operator-system -o jsonpath='{.items[0].spec.template.spec.containers[0].image}'`)
   - Kubernetes version (`kubectl version`)
   - Helm chart version (`helm list -A | grep valkey-operator`)
   - The smallest reproduction you can produce
   - The output of `kubectl describe <Valkey|ValkeyCluster> <name>`

## Response expectations

This is an open-source project maintained on best-effort time.
[GOVERNANCE.md](GOVERNANCE.md) describes the decision-making and
review process. We typically respond on issues within a few business
days; security reports are handled per the SLA in
[SECURITY.md](SECURITY.md) (initial ack within 72 h, severity triage
within 7 days).

If you need a paid support relationship or a hard SLA, reach out via
`security@keiailab.com` and we can discuss options.

## Commercial vendors

`valkey-operator` does not endorse a paid support vendor today. If
this changes, an entry will be added here with the vendor's terms
and the upstream feature it supports.

## Code of Conduct

Every channel above is governed by the
[Code of Conduct](CODE_OF_CONDUCT.md). Please read it before
participating.

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
