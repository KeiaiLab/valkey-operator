# Valkey StatefulSet → ValkeyCluster 迁移指南

> English: [README.md](README.md) — canonical / 正本

> 引入 ValkeyCluster CR 时,将既有的 StatefulSet 部署 *无服务中断* 平迁过去的流程目录。

## 文档 (执行顺序)

| 文档 | 场景 |
|---|---|
| [zero-downtime.zh.md](zero-downtime.zh.md) | 常规迁移 (5 步,< 30 分钟,0 秒停机) |
| [secondary-promote.zh.md](secondary-promote.zh.md) | 将 Replica 提升为 Primary 后下线旧 primary (数据一致性优先) |
| [rollback.zh.md](rollback.zh.md) | 迁移途中发现客户端异常,立即回退 |

## Pre-flight checklist

- [ ] ValkeyBackup CR 行为正常 (operator 安装完成后)
- [ ] cert-manager Certificate Ready (operator webhook TLS)
- [ ] 既有 StatefulSet 的 PVC `accessModes: [ReadWriteOnce]`
- [ ] 应用工作负载的 connection retry 策略已启用
- [ ] PrometheusAlert `ValkeyClusterDegraded` 告警通道已确认

## SLO 概览

| 场景 | 停机时间 | 总耗时 | 数据缺口 |
|---|---|---|---|
| zero-downtime | 0 秒 | < 30 分钟 | 0 |
| secondary-promote | 0 秒 (read-only 窗口 5 秒) | < 15 分钟 | 0 |
| rollback | < 5 分钟 | < 10 分钟 | < 30 秒 |

## Refs

- [ROADMAP.md](../ROADMAP.md)
- [ADR-0015](../kb/adr/0015-valkeyrestore-init-container-pattern.md) (restore via init container)
- [ADR-0016](../kb/adr/0016-valkeybackuptarget-crd-external-storage.md) (ValkeyBackupTarget S3 abstraction)
