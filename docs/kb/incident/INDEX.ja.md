# Incident Knowledge Base — INDEX (日本語)

> English: [INDEX.md](INDEX.md) — canonical / 正本

本ディレクトリは valkey-operator の運用 incident を blameless postmortem-lite
形式で保存する。グローバル標準: `~/Documents/ai-dev/standards/incident-kb.md`。

| ID | 題目 | Severity | Detected | Resolved |
|---|---|---|---|---|
| [INC-0001](INC-0001-cluster-fail-bootstrap-skip.md) | ValkeyCluster が cluster_state:fail のまま bootstrap が再実行されない | SEV-2 | 2026-05-09 14:27 KST | 2026-05-10 09:18 KST |

## 起草ガイド

- 形式: グローバル `standards/incident-kb.md §3` (Postmortem-lite)。
- トリガ: 運用障害 / セキュリティ事案 / 30 分以上デバッグした非自明なバグ / パターン発見 (3 回再発)。
- Blameless 文化: *システム* のどこが失敗を許容したかを問う。Action Items は *システム変更* を優先する。
- KB 鮮度: 30 日間未更新の INC が 30% を超えたら通知する (グローバル §6)。
