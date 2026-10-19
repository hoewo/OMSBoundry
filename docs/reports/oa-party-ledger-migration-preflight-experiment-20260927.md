# Contract／Money Party 迁移预检实验

日期：2026-09-27  
状态：预检通过；映射、切换和回滚验收尚未开始

## 1. 问题与结果

问题 A：Contract Ledger 与 Money Flow Ledger 已有各自本地 Party、业务引用和历史快照，若直接按名称合并、改写旧 ID 或让两个目录同时可写，会破坏既有合同、义务、资金和审计的解释力。  
可验证结果 B：两个台账都能从自己的数据库生成确定性、只读的迁移清单，逐条给出来源 Party、稳定摘要、业务引用计数和待人工确认的 `create_directory_party`／`map_existing_directory_party` 决策，同时明确拒绝自动合并和自动切换。

## 2. 不能破坏的约束

- 本地 Party ID、revision、当前资料、既有业务快照和审计不被修改。
- 预检只读，不创建 Party Directory 记录，不保存映射，不改变运行时读取来源。
- 名称或登记标识相同只能形成复核信号，不能自动证明同一主体。
- 每条来源记录都必须由人明确选择“新建”或“映射到已确认 Party”。
- 未完成映射、目标 Party 精确验证、切换审计和回滚验收前，`ready_for_cutover` 固定为 `false`。
- 不通过运行时双写维持两个当前真相。

## 3. 实现

新增两个只读运维入口：

```bash
contract-ledger-ops party-migration-preflight --tenant-id <tenant-id>
money-flow-ledger-ops party-migration-preflight --tenant-id <tenant-id>
```

Contract 报告格式为 `omsbizfoundry.contract-party-migration-preflight/v1`，统计草稿主体、正式 revision 主体和结算主体引用。Money 报告格式为 `omsbizfoundry.money-flow-party-migration-preflight/v1`，统计义务、义务 revision、资金流水和内部转账引用。每条 Party 使用规范化 JSON 计算 SHA-256，以便后续映射命令确认自己审查的仍是同一份来源事实。

两份报告都固定输出：

- `automatic_merge_allowed=false`；
- 每条记录 `decision_status=unresolved`；
- 允许的人工决策只有 `create_directory_party` 和 `map_existing_directory_party`；
- 重复登记标识形成 manual-review signal；
- 结构性无效数据形成 blocker；
- 预检成功也只表示 `ready_for_review=true`，不会把 `ready_for_cutover` 提升为 `true`。

## 4. 执行证据

代码与静态检查：

```bash
go test ./backend/services/contract-ledger/...
go vet ./backend/services/contract-ledger/...
go test ./backend/services/money-flow-ledger/...
go vet ./backend/services/money-flow-ledger/...
sh -n acceptance/d1-delivery/verify-running
git diff --check
```

以上检查均通过。D1 镜像重建后执行：

```bash
deployments/d1/ops/d1 check
D1_ACCEPTANCE_DISRUPTIVE=no acceptance/d1-delivery/verify-running
```

实际 D1 数据结果：

| 来源 | Party 数 | 被业务引用 | 未引用 | 待人工决策 | 重复登记标识组 | blocker | 可切换 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Contract | 5 | 0 | 5 | 5 | 0 | 0 | 否 |
| Money Flow | 5 | 0 | 5 | 5 | 0 | 0 | 否 |

两份报告均为 `ready_for_review=true`、`automatic_merge_allowed=false`、`ready_for_cutover=false`，每条摘要为 64 位十六进制 SHA-256。扩展后的 D1 非破坏性验收通过 Nginx 单入口、一次登录、已启用应用读写、Contract／Money Party 迁移预检、People 名册、Party→Project 精确引用和 Evidence 字节生命周期。

当前 D1 样本没有被业务引用的本地 Party，因此它证明了真实数据库读取、确定性清单、D1 运维接线与失败关闭状态；“已有引用时计数和历史保持”的语义由单元测试覆盖，仍需要下一轮构造有合同／义务／资金引用的数据并完成真实映射与切换验收。

## 5. 结论

预检切片通过。它证明迁移可以从各数据所有者自己的数据库出发形成可审查、可重复的输入，而不需要共享 Party 表、跨库读取或自动猜测主体身份。

这不等于 Contract／Money 已迁移。尚未实现和验证：

1. 经人工审查的兼容映射账本；
2. 映射目标的 Party ID／revision 精确验证；
3. 映射、审计和来源摘要的幂等持久化；
4. 显式切换状态与切换后的新引用路径；
5. 含既有合同、义务、资金和结算引用的数据保持；
6. Party 不可用时的运行时失败关闭；
7. 切换后回滚不重新制造本地当前真相；
8. 完整双 Tenant 交叉矩阵。

## 6. 下一项可证伪工作

下一切片实现“已审查映射账本”，但仍不切换运行时：映射命令必须绑定来源应用、来源 Party ID、来源摘要、迁移批次和目标 Party ID／revision，由 Party Directory 精确验证目标后，在来源应用自己的数据库中幂等保存兼容映射与审计。使用包含已被合同／义务／资金引用 Party 的测试数据证明旧 ID、快照和历史完全不变；任何摘要漂移、stale revision、跨租户、inactive 目标、重复冲突或 Party 不可用都必须零写入失败。
