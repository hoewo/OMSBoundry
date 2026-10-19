# Contract／Money Party D1 人工确认映射实验（2026-09-27）

## 1. 本轮问题与结果

问题 A：人工确认映射已经在本地 PostgreSQL 中证明，但尚未证明 D1 中的 Contract／Money 能以各自工作负载身份通过正式 IAM 和 Party API 保存真实映射，也未证明依赖故障和篡改会失败关闭。

可验证结果 B：在当前单租户 D1 中，为两个台账分别装配独立工作负载密钥和最小 IAM Policy；从各自真实预检清单人工选择一条来源 Party，将两条来源映射到同一个已确认 Directory Party revision。首次写入、等价重放、冲突、stale revision、Party 停机／恢复、IAM 审计和数据库不可变保护全部取得直接证据，且运行时 Party 来源没有切换。

## 2. D1 装配边界

- Contract 工作负载 Subject：`66666666-6666-4666-8666-666666666666`。
- Money 工作负载 Subject：`77777777-7777-4777-8777-777777777778`。
- 两个 Subject 只允许向 `party-directory` audience 申请 `party-directory.reference.read`，资源类型只允许 `party`；没有写 Party、跨应用业务动作或通配业务权限。
- 两套 Ed25519 私钥使用不同的 Docker 私有卷，由独立初始化服务生成；台账只读挂载自身私钥。
- Contract／Money 只通过版本化 Party reference API 读取精确目标，不共享 Party 数据库、表、ORM 或内部对象。
- `deployments/d1/ops/d1 party-map contract|money INPUT_JSON` 是运维入口；审核 JSON 先由人类复核，再从标准输入复制到目标容器的临时文件执行。

## 3. 实际样本与人工决定

当前 Tenant 为 `77777777-7777-4777-8777-777777777777`。重新预检后 Contract 与 Money 各有 6 条来源 Party，均为 0 条业务引用、无 blocker；本轮只审核其中各 1 条，其余各 5 条继续保持未决。

审核人选择带唯一时间戳／随机后缀 `20260927T044134Z-f666087f` 的两条测试来源。两个应用中的名称前缀不同，因此决定不是名称自动相似匹配，而是对测试样本来源、唯一后缀和目标档案进行人工核对后的显式决定：

| 来源 | 本地 Party ID | 来源摘要 | 目标 Directory Party | 接受 revision |
| --- | --- | --- | --- | --- |
| Contract | `d04ecc91-062d-439b-9e78-eeb70f7a1cc8` | `6c795b…bc759` | `652194f9-9701-47ac-82c0-3b99bc992cd6` | 1 |
| Money | `4586be3d-cda0-4aec-8669-2a2e3fa526a4` | `0c5142…d27d` | `652194f9-9701-47ac-82c0-3b99bc992cd6` | 1 |

摘要在审核输入和数据库中保存完整 64 位值；本文只显示截断值，避免把报告当作可直接重放的命令载荷。

## 4. 执行证据

### 首次写入与重放

- Contract 首次执行：`recorded=1`、`replayed=0`、`ready_for_cutover=false`。
- Money 首次执行：`recorded=1`、`replayed=0`、`ready_for_cutover=false`。
- 两端重复执行同一审核文件：`recorded=0`、`replayed=1`；映射表计数仍分别为 1。
- 使用同一 Command ID 改变负载，两端均以“不可变命令冲突”拒绝，没有增加映射。

### 目标、依赖与数据库保护

- Money 使用新 Command ID 但把目标 revision 改为 2，被精确 reference 校验拒绝为无效目标；映射计数不变。
- 停止 Party Directory 后，Contract 使用新 Command ID 的映射失败为“Party Directory 不可用”；映射计数不变。恢复 Party 后健康检查通过。
- 直接执行 SQL `UPDATE party_migration_mappings` 被数据库不可变 Trigger 拒绝。
- 最终数据库中 Contract／Money 各只有 1 条映射，目标 ID 和接受 revision 均与审核文件一致。

### IAM 审计

- Contract 映射形成 1 条工作负载交换审计；audience、action 和精确 Party Scope 均正确。
- Money 形成 2 条工作负载交换审计：一次成功映射和一次 stale 目标验证；两条 audience、action 和 Scope 均正确。
- Party 停机失败发生在目标不可用阶段，不产生来源映射事实。

## 5. 回归与运行状态

- IAM、Contract、Money 和共享 workload identity 包的 `go test ./...` 通过。
- D1 Compose 配置校验通过，Contract schema 为 `000018_party_migration_mappings.sql`，Money schema 为 `000017_party_migration_mappings.sql`。
- 当前 D1 全部服务恢复 healthy，唯一公开入口仍是 Nginx；没有为 Party、IAM 或台账增加宿主端口。

## 6. 结论与限制

本轮证明 D1 中的人工决定能够以“来源应用自有映射 + 独立工作负载身份 + IAM 最小能力 + Party 精确 revision + 不可变数据库事实”安全落地。它也证明 Party 故障不会制造部分映射，重试不会重复写入，审核命令和映射记录不能静默修改。

本轮仍不能宣布两个台账已经迁移完成：

- 当前 12 条来源样本均无合同、义务、资金或结算引用，只映射了 1+1 条，仍有 5+5 条未决；
- 只验证了一个 Tenant，没有完整双 Tenant 交叉拒绝矩阵；
- `ready_for_cutover` 仍为 `false`，Contract／Money 运行时仍使用各自本地 Party 目录；
- 尚未验证含引用数据在切换后的 ID、历史快照、审计和恢复保持；
- 显式切换门禁、切换后的新事实写入、回滚策略和联合恢复仍须独立设计与验收。

因此下一切片不是扩大自动匹配，而是准备含引用数据、补齐双 Tenant 负例，并设计可审计、无双写的显式运行时切换与回滚门禁。
