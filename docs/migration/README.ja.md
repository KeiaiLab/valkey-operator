# Valkey StatefulSet → ValkeyCluster Migration

> English: [README.md](README.md) — canonical / 正本

> ValkeyCluster CR を導入する際に、既存の StatefulSet ベースのデプロイメントを *無停止で* 移行するための手順カタログ。

## 文書 (実行順)

| 文書 | シナリオ |
|---|---|
| [zero-downtime.md](zero-downtime.md) | 一般的な移行 (5 ステップ、30 分未満、ダウンタイム 0 秒) |
| [secondary-promote.md](secondary-promote.md) | Replica → Primary へ昇格させた後に既存 primary を廃止 (データ整合性を最優先) |
| [rollback.md](rollback.md) | 移行中に client 異常を検知した場合の即時切り戻し |

## Pre-flight checklist

- [ ] ValkeyBackup CR が正常に動作することを確認 (operator 導入後)
- [ ] cert-manager の Certificate が Ready (operator webhook TLS)
- [ ] 既存 StatefulSet の PVC が `accessModes: [ReadWriteOnce]`
- [ ] アプリケーション client の connection retry ポリシーが有効
- [ ] PrometheusAlert `ValkeyClusterDegraded` の通知チャネルを確認

## SLO サマリ

| シナリオ | ダウンタイム | 全体所要時間 | データギャップ |
|---|---|---|---|
| zero-downtime | 0 秒 | 30 分未満 | 0 |
| secondary-promote | 0 秒 (read-only window 5 秒) | 15 分未満 | 0 |
| rollback | 5 分未満 | 10 分未満 | 30 秒未満 |

## Refs

- [ROADMAP.md](../ROADMAP.md)
- [ADR-0015](../kb/adr/0015-valkeyrestore-init-container-pattern.md) (restore via init container)
- [ADR-0016](../kb/adr/0016-valkeybackuptarget-crd-external-storage.md) (ValkeyBackupTarget S3 abstraction)
