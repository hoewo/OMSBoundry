# Contract／Money Party 人工确认映射实验（2026-09-27）

## 1. 本轮要回答的问题

问题 A：两个台账的本地 Party 已能生成迁移预检清单，但人工确认结果如果只留在表格或聊天里，无法可靠重放、审计或发现来源已经变化。

可验证结果 B：Contract 与 Money 分别拥有一份不可变映射账本；执行前用精确 IAM 工作负载凭证读取指定 Party revision，事务内再次锁定来源 Party 并核对预检摘要。任何目标不匹配、来源漂移、重复冲突或依赖不可用都失败关闭。本轮不切换运行时 Party 来源。

## 2. 实现边界

- Contract 与 Money 各自新增 `party_migration_mappings`，没有共享表、跨库外键或 Party ORM。
- 每条事实保存 Tenant、迁移批次、Command ID、本地 Party ID／revision／预检 SHA-256、人工决策、目标 Party ID／revision／最小快照、审核人、理由、验证时间和请求摘要。
- 数据库 Trigger 拒绝 UPDATE／DELETE；同 Tenant + Command ID 固定幂等，同迁移批次中的同一本地 Party 只能决定一次。
- `party-migration-map` 使用本应用工作负载私钥向 IAM 交换 `party-directory.reference.read`，Scope 精确到 `party:{target_party_id}`；不接受长期静态 Bearer Token 参数。
- `create_directory_party` 表示审核人已先在 Party Directory 建立目标；映射命令本身不跨应用创建 Party，避免分布式双写。
- 输出固定 `ready_for_cutover=false`。已有合同、义务、资金、结算和快照不被改写；运行时继续使用本地 Party。

## 3. 第一次执行步骤

1. 对来源应用运行 `party-migration-preflight --tenant-id <tenant_uuid>`，保存完整 JSON。
2. 人工逐条核对；若需新建，先在 Party Directory 登记并记下当前 ID／revision；若映射既有主体，确认它确实是同一法律或自然主体。
3. 形成审核文件，格式如下（UUID 和摘要必须来自真实预检／目标响应）：

```json
{
  "format": "omsbizfoundry.contract-party-mapping-review/v1",
  "tenant_id": "<tenant_uuid>",
  "migration_batch_id": "<batch_uuid>",
  "reviewed_by_subject_id": "<iam_subject_uuid>",
  "reviewed_by_display_name": "审核人显示名",
  "mappings": [
    {
      "command_id": "<command_uuid>",
      "local_party_id": "<preflight_local_party_id>",
      "local_snapshot_sha256": "<preflight_sha256>",
      "decision": "map_existing_directory_party",
      "target_party_id": "<directory_party_id>",
      "target_party_revision": 1,
      "reason": "人工核对依据"
    }
  ]
}
```

Money 使用同形格式，只把 `format` 改为 `omsbizfoundry.money-flow-party-mapping-review/v1`。

4. 运行环境必须注入 `DATABASE_URL`、`PARTY_BASE_URL`、`IAM_SERVICE_CREDENTIAL_URL`、`IAM_TENANT_ID`、`WORKLOAD_ID`、`WORKLOAD_SUBJECT_ID` 和只读挂载的 `WORKLOAD_KEY_FILE`。
5. 执行 `/app/ops party-migration-map --input /path/review.json`。程序先检查同 Command 重放，再逐目标交换精确短期凭证并读取 reference API，最后开启来源数据库事务、锁定本地 Party、重算摘要并追加映射。
6. 保存版本化 JSON 结果。`recorded` 表示本次新增，`replayed` 表示相同命令返回原事实；二者都不表示允许切换。
7. 重复执行同一审核文件应只增加 `replayed`；改变同一 Command 负载必须失败。修改来源 Party 后，用旧摘要执行新命令必须失败且映射数量不变。

## 4. 实际验证

### 普通测试

- Contract：`go test ./...` 通过。
- Money：`go test ./...` 通过。
- 两端 HTTP 测试覆盖正确目标、stale revision、Party 503、精确路径和 Authorization Header。
- 请求测试覆盖规范化、稳定摘要和同一批次重复本地 Party 拒绝。

### PostgreSQL 18.6 集成测试

在临时、空白 PostgreSQL 18.6 容器上分别运行：

```text
TEST_DATABASE_URL=<temporary-url> go test -tags=integration ./internal/database ./internal/partymigration
```

Contract 与 Money 均通过。用例实际验证 migration 从历史状态升级、映射首次追加、相同 Command 重放不再次访问目标、数据库 UPDATE 被不可变 Trigger 拒绝、来源 revision／名称变化后旧摘要失败关闭，以及失败事务不产生第二条映射。

### 镜像构建

`Dockerfile.ledger` 已显式携带共享的工作负载身份实现，Contract 与 Money 两个 Linux 镜像均构建成功，`ops` 可包含新命令。

## 5. 结论与限制

本轮证明“人工决定可以变成来源应用自有、不可变、可幂等重放且由目标精确事实约束的迁移证据”，进一步收缩了 Party 解耦边界：Party 拥有当前主体，台账拥有“我的历史本地主体对应哪个 Party revision”的兼容事实。

本报告固定的是机制实现时的边界。后续 D1 专项已经完成两个台账的独立工作负载接线、各 1 条真实人工映射、重放／冲突／stale、Party 停机恢复、IAM 精确 Scope 审计和数据库不可变保护；仍未完成含引用数据、双 Tenant、运行时切换或回滚。后续直接证据见 [Contract／Money Party D1 人工确认映射实验](./oa-party-ledger-d1-reviewed-mapping-experiment-20260927.md)。
