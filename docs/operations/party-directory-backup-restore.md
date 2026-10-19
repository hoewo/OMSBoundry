# Party Directory 备份与恢复

## 备份格式

`party-directory-backup create` 产生不可覆盖的 ZIP，固定包含 `manifest.json` 与 `database.dump`。清单记录格式版本、生成时间、数据库名、PostgreSQL 版本、schema migration、每张业务表的行数、dump 大小和 SHA-256。dump 与清单来自同一 PostgreSQL 导出快照，因此在线写入不会造成表计数与 dump 的观察时点分裂。

```bash
party-directory-backup create \
  --database-url "$DATABASE_URL" \
  --output /secure-backups/party-directory-20260927.zip
```

备份完成后必须把 ZIP 复制到故障域之外，限制为管理员可读，并定期在隔离环境做真实恢复。ZIP 不是加密容器；如备份包含敏感登记信息，应再使用组织批准的静态加密和密钥管理。

## 空目标恢复

恢复只接受完全空的 PostgreSQL 数据库，并要求备份、目标服务器和 `pg_restore` 的 PostgreSQL 主版本一致：

```bash
party-directory-backup restore \
  --database-url "$EMPTY_TARGET_DATABASE_URL" \
  --input /secure-backups/party-directory-20260927.zip
party-directory-ops check
```

工具在恢复前校验 ZIP 路径、条目类型、清单字段和 dump 大小；恢复时使用单事务、禁止 owner/privilege 迁移；恢复后再次比较 schema 版本、表集合和逐表行数。任何失败都应隔离并重建目标空库后重试，不能把失败目标直接接入流量。

## 恢复验收

- 相同 Party ID、租户、当前 revision、状态和登记字段可读；
- revision 历史、命令幂等记录和审计事件数量完全一致；
- 第二次向同一目标恢复被明确拒绝；
- 恢复库能在 `expected_revision` 约束下继续产生下一 revision；
- Gateway/IAM 接入时仍由新环境凭证授权，备份不携带账号密码或 IAM 私钥。
