# Evidence Service 备份与恢复

## 一致性边界

Evidence 的数据库与本地对象目录共同组成完整事实。v1 采用简单、可证明的冷备份：先从 Gateway 摘除 Evidence 并停止全部 Evidence Server／cleanup 写入者，再显式传入 `--confirm-service-stopped`。该参数是操作员确认，不会尝试猜测进程状态。

OSS Provider 的备份包保存完整数据库和 Provider 标识，不复制 OSS 字节；生产 Bucket 必须启用独立的版本保护、生命周期和跨地域／离机复制。恢复后仍指向原 Bucket/Object Key，必须另行证明 OSS 对象可用。

Local／OSS 兼容模式使用 `--provider hybrid` 与 v2 格式 `omsbizfoundry.evidence-service/full-backup-v2`：完整数据库、本地字节、外部 OSS 引用及摘要随包保留；`ExternalObjects` 是引用清单，不是对象字节。恢复必须使用匹配的 hybrid 模式和空目标。旧 Local v1 先按 local 恢复，再启动兼容服务；不能把存有 OSS 引用的混合库退回旧 Local-only 二进制。Primary 只决定新会话位置，不能重写历史对象引用。详见 [切换规格](../specs/evidence/local-oss-transition.md)。

## 创建

```bash
evidence-backup create \
  --database-url "$DATABASE_URL" \
  --provider local \
  --storage-root /absolute/evidence/objects \
  --output /absolute/off-host/evidence-20260921.zip \
  --confirm-service-stopped
```

工具使用 PostgreSQL repeatable-read exported snapshot、custom-format `pg_dump`、每个本地对象的大小和 SHA-256 生成不可覆盖 ZIP。已确认 Version 的对象若缺失或摘要不符，备份失败且不发布目标文件。

## 恢复

恢复目标必须是空 PostgreSQL 18 数据库；本地对象根目录必须不存在：

```bash
evidence-backup restore \
  --database-url "$EMPTY_DATABASE_URL" \
  --provider local \
  --storage-root /absolute/new/evidence/objects \
  --input /absolute/off-host/evidence-20260921.zip
```

工具拒绝未知／重复／路径穿越 ZIP 条目，逐项校验大小和 SHA-256，以单事务 `pg_restore` 恢复，并核对 migration、表计数、Provider 和所有不可变 Version 对象。验证失败的恢复目标必须隔离并重新创建空数据库和空对象目录后再试，不能在半恢复目标上重跑。

## 恢复演练

至少每个发布版本和每季度执行一次：冷备份 → 新空库/新目录恢复 → `health/ready` → 受 IAM 保护的下载与 Range 下载 → SHA-256 对比。备份文件必须复制到 ECS 之外并验证访问控制和保留周期。

2026-10-01 Auth58 隔离演练已通过 mixed v2 的 12 个 Version 空库恢复与全部字节复读，并对单份 4,736 B 合成 OSS 对象完成离机复制、误删、恢复。正式 Bucket 全量字节保护／定期恢复尚未完成，不能据此宣称所有文件可从当前 ZIP 还原。直接证据见 [Auth58 报告](../reports/auth58-evidence-oss-adapter-validation-20261001.md)。
