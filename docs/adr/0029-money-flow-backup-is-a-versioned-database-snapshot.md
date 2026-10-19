# ADR-0029：Money Flow 全量备份是版本化数据库快照

日期：2026-09-20  
状态：Accepted

## 背景

`MF-M4-012` 的本方主体 CSV 归档解决业务数据移交与离线核对，刻意排除了身份、幂等命令、其他本方账簿和数据库内部结构，因此不能恢复一个可继续运行的 Money Flow 实例。

Money Flow 当前全部持久事实都在自己的 PostgreSQL 中，没有 Contract Ledger 那样的外部 Evidence 字节目录。灾难恢复仍需要处理在线一致性、旧会话失效、空目标门禁、客户端兼容、失败回滚和恢复后证明，不能只写一句“运行 pg_dump”。

## 决策

1. 使用 `omsbizfoundry.money-flow-ledger/full-backup-v1` 单 ZIP，固定包含严格 manifest 与 PostgreSQL custom dump。
2. 用 PostgreSQL 导出 snapshot 同时约束 `pg_dump`、schema version 和所有 public base table 的逐表计数；`local_sessions` 不计入来源计数并排除 table data。
3. 备份保留本地用户密码摘要、锁定状态、幂等命令、payload hash、完整审计和所有业务事实，因此归档按高敏感运维资产处理。
4. 恢复只允许真正空的目标数据库，拒绝覆盖、合并和自动清空；`pg_restore` 使用单事务和遇错退出。
5. 恢复后必须核对 schema、完整非 session 表集合与逐表计数，并证明 session 为 0；已提交后的最终核验异常要求隔离和重建目标。
6. 来源、目标、`pg_dump` 和 `pg_restore` 使用相同 PostgreSQL major；跨 major 升级另行设计。
7. v1 只覆盖 Money Flow 自有 PostgreSQL，不读取 Contract Ledger，也不形成跨系统一致快照。

## 原因

- 导出 snapshot 让在线 dump 与外部核验元数据对应同一 MVCC 时点，不需要停服或锁住业务写入。
- 完整逐表计数比只检查 Party／Obligation／Cash 三个聚合更能发现漏表、命令链或审计丢失。
- custom dump 保留触发器、约束、函数和内部幂等状态，恢复后仍可执行既有业务规则。
- 排除 session 可以保留账号而不让备份前 Cookie 在新环境继续有效。
- 空目标和单事务恢复将失败保持在可理解、可重试的边界内，也避免未经授权的破坏性覆盖。

## 后果

- 归档暴露所有账簿和密码摘要，不能作为普通 CSV 发送或放入 Web 下载目录；v1 不内置加密。
- 在线备份会持有长时间只读 snapshot；生产部署必须依据数据量设置容量、超时、监控和执行窗口。
- PostgreSQL custom dump 是内部恢复资产，不承诺跨任意版本或作为第三方导入格式。
- 恢复工具不会替运维平台安排周期、远端复制、保留期限或自动切换；这些必须通过真实 RPO／RTO 决定。
- 将来若 Money Flow 引入外部文件存储，必须升级格式并重新证明数据库与文件的一致恢复，不能静默沿用 v1。

## 被否决方案

### 用 `MF-M4-012` CSV 反向导入

CSV 有意排除身份、命令、其他 owner 和数据库约束；反向导入会丢失运行时不变量并把业务交换格式误作灾备格式。

### 只保存一份裸 `pg_dump`

没有严格归档版本、摘要、snapshot 计数和恢复后核验，无法区分文件损坏、连接错库、漏表与真正可恢复的备份。

### 备份 `local_sessions`

会让恢复前 Cookie 在新环境继续有效，扩大归档泄露和灾难切换后的身份风险。

### 自动清空非空数据库再恢复

这是破坏性覆盖，不属于安全的第一版灾难恢复。明确创建新目标能够让源、失败目标和恢复结果同时保留，便于核验与回退。

### 与 Contract Ledger 做联合备份

当前没有需要原子恢复的跨系统业务事实，联合 snapshot 会破坏两个应用的独立交付边界，并引入尚无事实依据的分布式一致性协议。

详细规格见 [Money Flow 可验证全量备份与空环境恢复](../specs/money-flow-ledger/full-database-backup-restore.md)。
