# Contract／Money Party 运行时切换与 C6 复验

日期：2026-09-28  
结论：`referenced-runtime-cutover-supported / complete-dual-tenant-pending`

## 1. 问题与验证结果

问题：Contract 与 Money 仍保存本地 Party，单纯建立 `local Party → Directory Party` 映射并不能证明运行时已经安全采用统一主体；旧 Contract Party ID 与 C6 使用的 Directory Party ID 不一致时，合同关联的外协登记会以 422 失败关闭。

结果：两个台账现在都具有来源应用自有、追加不可变的 Party 运行时转换账。切换前重新验证同一批次的全部目标 Party 精确 revision，并要求每个已被业务事实引用的本地 Party 都存在显式映射；切换后新的引用投影使用 Directory Party ID／revision，本地 Party 写入口永久冻结。回滚只恢复本地兼容读取，不恢复本地写权，也不删除映射或 Directory 权威已经建立的事实。

## 2. 实现边界

- Contract migration `000019_party_runtime_transitions.sql` 与 Money migration `000018_party_runtime_transitions.sql` 保存租户级 `cutover／rollback` 序列、批次、审核人、原因、映射数、引用数和请求摘要；数据库 Trigger 禁止更新和删除。
- 两个应用的 `party-runtime-transition` 运维命令只接受显式 Tenant、Command、Action 和 Migration Batch；同 Command 同摘要重放，不同摘要冲突。
- `cutover` 在事务中锁定当前转换状态和映射批次，重新调用 Party Directory 精确 reference，随后再次核对批次摘要，防止验证与提交之间映射漂移。
- 任何被 Contract revision／签约主体或 Money obligation／cash transaction 引用的本地 Party 缺少映射时，切换失败；不按名称、登记标识或位置猜测。
- `rollback` 只进入 `local_compatibility` 读模式；Directory 权威一旦建立，本地 Party POST 始终返回 409，避免出现两个可写当前真相。
- Contract 的工作负载 reference 在正式合同上读取当前 revision 的不可变 Party Snapshot，而不是已经失去权威的草稿 Party 关系；草稿仍读取 active draft Party。

## 3. 直接证据

### 3.1 Contract 与 Money 真库

两个应用的 PostgreSQL integration 均使用真实被业务事实引用的 Party，完成：

1. 来源自有映射；
2. `cutover` 后 Directory authority；
3. `rollback` 后只读兼容；
4. 本地写权限不恢复；
5. 转换记录防篡改。

随后两个应用分别执行 `go test ./...`，全部通过。

### 3.2 C6 正式 D1 串行复验

命令：

```text
COMPOSE_PROGRESS=plain acceptance/d1-c6-cost-outsourcing/run-isolated full
```

从空隔离环境取得以下结果：

- 62 个正式入口 HTTP 断言，状态不匹配 0；
- Contract 先用两个本地 Party 建立真实合同；Party Directory 建立对应 owner／provider；
- 切换前 Contract-linked Engagement 返回 422；
- 两条显式映射完成后，`cutover` sequence 1，Engagement 返回 201；
- `rollback` sequence 2 后同类 Engagement 再次返回 422；
- 回滚期间本地 Party 写入返回 409；
- `recutover` sequence 3 后恢复 Directory 运行态；
- 最终保留两条 Engagement，其中一条引用 Contract；
- Evidence／Party 停机均返回 503 且零写入，恢复后原业务意图成功；
- Project Scope、最小 read、金额脱敏、无 create 和越 Scope 拒绝均通过。

运行证据位于 `deployments/d1/.runtime/c6-d1-evidence/`：

- `contract-party-map.json`：2 条来源映射；
- `contract-party-cutover.json`：sequence 1，mapped=2，referenced=2；
- `contract-party-rollback.json`：sequence 2；
- `contract-party-recutover.json`：sequence 3；
- `state.json`：`party_runtime_mode=directory`、`party_runtime_sequence=3`、`engagement_count=2`；
- `result.md`：62／62 入口结果；
- `workload-audits.tsv`：精确工作负载交换审计。

### 3.3 联合备份、空目标恢复与选装

联合备份 `20260928T052302Z` 后销毁隔离容器、网络和卷，从空目标恢复：

- Contract schema 恢复到 `000019_party_runtime_transitions.sql`；
- Party Directory、Evidence 与 C6 事实恢复；
- Contract Party runtime 保持 `cutover|<batch>|3|2`；
- 显式映射、切换序列、业务 ID、revision、快照和历史保持；
- 恢复后 Cost correction 从 revision 1 推进到 2。

随后取消 C6 选装，验证 runtime／Workbench／Gateway route／backup dump 均缺席；重新选装后路由恢复，原数据库卷未被删除。

## 4. 发现并修复的反例

第一次切换已经成功，但 C6 仍返回 422。直接读取 Contract 工作负载 reference 后发现，正式合同响应没有 Party IDs。根因是 handler 对 active Contract 仍读取草稿关系，而签约后权威主体已经冻结在当前 Contract revision 的 Party Snapshots。修复后 reference 只为当前 revision 输出 snapshot 的 `source_party_id`，并增加 active／draft 两类单元测试；重新构建正确的 `contract-migrate` 镜像后，直接响应与完整 D1 均通过。

这一反例说明：统一主体并不意味着改写历史快照；运行时当前引用可以解析为 Directory ID，但法律／业务发生时的名称与身份快照仍由来源应用按 revision 保留。

## 5. 结论与限制

本轮关闭“有引用数据的显式切换／回滚”和“Contract-linked C6 使用 canonical Directory Party”缺口。它直接支持来源应用数据所有权、无共享表、精确 revision、失败关闭、可逆兼容读取和恢复可解释性。

仍未证明：

- 完整双 Tenant D1 交叉矩阵；
- Money 作为完整 D1 消费链中的切换／回滚（Money 真库运行时证据已通过）；
- C6 Payable Suggestion 到 Money 的独立接受／拒绝与故障恢复；
- C7～C8、真实 Ubuntu／公网入口及 OA 最终报告。

因此不能把本轮描述为 OA 完整体或跨客户架构已经验证完成。
