# OA C5 人类工作区与 Project Completion Profile 实验（2026-09-28）

## 1. 结论

本轮结论为：**human-workspace-and-project-checkpoint-supported / cross-process-composition-not-tested**。

Approval & Audit Results 已有可理解、可操作的人类工作区；Project Workspace 已实现 `reference-oa-c5-v1` 的纯读取评估与显式检查点确认。该结果把 C5 从 API-only 推进到 C5-A10／A11 当前本地切片，但没有证明真实独立进程最小组合、Suite Manifest、Workbench、Nginx／Gateway／IAM 正式入口、联合恢复或完整 OA Project 完成，因此 C5 和 OA 整体仍不能判定通过。

## 2. 问题 A → 可验证结果 B

**问题 A**：若页面只显示 CRUD、Project 在 GET 时写“完成”，或直接复制 Results 状态，用户无法理解批复／审计工作流，Project 又会成为来源事实的第二真相；依赖故障、来源纠正和权限变化还可能被误判为完成。

**可验证结果 B**：Results 页面按 Project 读取当前事实、历史和服务端允许动作；Project GET 只解释两项 requirement，并区分 `satisfied／pending／unavailable`。只有显式命令才能追加 Project 自有 C5 检查点，且必须冻结 Project expected revision 与精确 Results 来源；来源以后变化只显示 drift，不改写历史，也不把 Project 状态改成“完整 OA 已完成”。

## 3. 实现范围

- Results 增加项目范围列表、项目级最小 reference-read 和人类工作区；
- Project 增加 `completion.read`／`completion.confirm`、Results 工作负载引用、只读评估、显式确认、不可变确认表与历史事件；
- `reference-oa-c5-v1` 固定要求当前 `approval` 和 `audit` 均为 `recorded`；
- 确认冻结 result ID、kind、revision、digest，只追加 Project revision，Project 仍为 `active`；
- 成功 Command 幂等回放先于 Results 外部调用；
- 页面动作依据 API `allowed_actions`，不是岗位名或前端隐藏；
- 新增产品规格、技术契约、数据模型和两张页面交互线框。

本轮不包含真实跨进程组合、D1 装配、备份恢复、C6／C7 requirement、多来源金额聚合、两级在线审批或签章。

## 4. 科学验证步骤

### 4.1 普通测试：规则、权限和契约适配

```text
cd backend/services/project-workspace && go test ./...
cd backend/services/approval-audit-results && go test ./...
```

结果：通过。覆盖：

1. 两类结果满足时评估 `satisfied`，缺失时 `pending`，Results 故障时 `unavailable`；
2. 最近确认来源与当前 ID／revision／digest 不同时 `drifted=true`；
3. stale expected sources 和 stale Project revision 失败关闭；
4. 成功确认回放在 Results 故障时仍返回原结果，且不再次读取依赖；
5. Results 页面包含使用说明、登记、纠正、撤回和历史，Project 页面明确“C5 检查点不等于完整 OA”；
6. 无 `completion.confirm` 能力时 POST 被目标应用拒绝，页面响应只返回读取动作；
7. Project→Results 使用 `approval-audit-results` audience、`approval-audit-result.read` action 和精确 `project:{id}` Scope；错 Tenant／畸形引用失败关闭。

### 4.2 PostgreSQL 18.6：Project 追加写入与 Results 生命周期

```text
TEST_DATABASE_URL='postgres://project_workspace:***@127.0.0.1:55437/project_workspace?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/projects ./internal/database

TEST_DATABASE_URL='postgres://project_workspace:***@127.0.0.1:55437/project_workspace?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/results ./internal/httpserver
```

结果：通过。Project 真库证明确认只新增一条 confirmation、一条 history、一条 command 并把 Project revision 从 4 推进到 5；Project 仍为 `active`，相同 Command 只返回同一 confirmation，数据库拒绝确认记录 UPDATE／DELETE，stale Project revision 返回冲突。Results 真库与受保护 HTTP 继续通过两类当前结果、项目范围列表、最小引用、精确 Scope、金额脱敏、依赖故障零写入和恢复重放。

### 4.3 文档与静态契约检查

新增 YAML 以解析器读取通过；两个 interaction 页面只引用共享灰度 `wireframe-style.css`，分别覆盖不少于四种状态。Capability Manifest、Role Overlay、Workload Policy 与 Suite Manifest 相关 JSON 均通过 JSON 解析。

## 5. 反耦合判定

| 观察项 | 结果 |
| --- | --- |
| Results 与 Project 是否共享表／ORM | 否；两个独立 module 和 Store，通过版本化 HTTP reference-read 协作 |
| Project GET 是否写业务状态 | 否；评估只返回可重建投影 |
| 显式确认是否写 Results | 否；只追加 Project 自有 confirmation／history／command／revision |
| Results 后续更正是否篡改历史确认 | 否；当前评估显示 drift，历史来源快照不变 |
| 依赖不可用是否冒充 pending／complete | 否；明确 `unavailable`，不产生 Project 写入 |
| 页面隐藏是否充当授权 | 否；目标应用复验原生 Capability 与 Project Scope |
| 是否借 C5 宣称完整 OA 完成 | 否；Project 保持 active，页面和契约显式限定为 C5 checkpoint |

## 6. 当前缺口与下一收缩

1. 用真实 Results、Project、Contract、Evidence 独立进程验证最小组合和故障恢复；
2. 把 Results、路由、Capability、Workload Policy、Workbench 和数据库加入可选 Suite Manifest，验证未选缺席；
3. 完成 D1 正式 Nginx→Gateway→IAM→目标应用入口、联合备份和空目标恢复；
4. 在上述证据通过前，不进入 C6／C7 实现，也不把当前检查点提升为完整 OA Project completion；
5. 多来源金额、`not_applicable` 的真实业务规则、撤回后已确认检查点的处置继续保持未知，等待真实业务证据。

## 7. 对总目标的影响

本轮关闭“C5 没有人类入口”和“Project 完成协作只有设计、没有实现”的两个高价值缺口，但 C5 的真实装配与恢复尚未开始。OA 完整体粗略进度由 **65% 调整为 67%**；剩余约 **33%** 主要集中在 C5 真实组合／D1／恢复、C6～C8、完整双 Tenant、Contract／Money Party 切换、真实 Ubuntu／HTTPS 和最终阶段报告。
