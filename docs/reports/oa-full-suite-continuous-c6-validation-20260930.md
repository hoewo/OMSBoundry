# OA 完整 Suite 连续 C6 验证报告

日期：2026-09-30  
状态：`passed`  
连续门禁：`C0～C6 = 7/9`  
适用范围：参考 OA 完整体第一阶段、同一 Tenant／同一 Project 连续旅程中的成本、外协与应付建议切片

## 1. 验证问题

问题 A：C1～C5 已形成项目、主体、合同、文件、经营、生产和批复事实，但仍不能证明 Cost & Outsourcing 能在不复制这些事实、不共享表、不越权创建资金义务的前提下完成成本／外协闭环。  
可验证结果 B：在同一 Tenant、同一 Project 和同一组既有引用上，Cost & Outsourcing 只拥有 Cost、Engagement、Assessment 和 Payable Suggestion；Money 独立接受建议并拥有 Obligation；依赖停机、过期 revision、错误 Project Scope 和重放均失败关闭或单次恢复。

本轮不验证工资、报销、发票、自动付款、通用工作流、第二 Tenant、跨进程撤回或真人浏览器体验。

## 2. 环境与基线

- 目标：公司 Ubuntu ECS，隔离 Compose project `omsbizfoundry-restore20`；
- 入口：仅回环地址 `http://127.0.0.1:18120`，链路为 Nginx → Gateway → IAM／目标应用；
- 长期容器：17 个，验证前后均为 `17/17 healthy`；
- Tenant：`6539cd13-7190-40f1-8bd5-57f9e7c9b968`；
- Project：`7a3a222d-ebfa-45d3-a6c9-ab44e6a1651b`，C5 输入与 C6 输出均为 revision 11、`active`；
- 复用 C1 客户 Party、C2 供应商 Party、Contract、award Evidence 和 C1 业务 Actor；
- 接受发布包：`v0.1.0-test.38`，归档与内部 checksum 均已核验；
- 修复镜像：
  - Cost & Outsourcing `sha256:878200e8a594c98b79610f115b3c3191f2c989989bae5f38539d56fa4a0c3ee5`；
  - Money Flow `sha256:5262b567a0885245d32a2f65640213798a41141805c8dbcab13f02aa6c9039af`。

正式证据目录：

```text
/opt/omsbizfoundry/validation-evidence/reference-oa-continuous-v1/c6
```

## 3. 科学验证步骤

验证器：`acceptance/d1-full-oa-journey/verify-c6`。

1. 对 C1、C2、C4、C5 状态摘要和同一 Tenant／Project 进行前置校验，先保存七个数据所有者的计数基线；
2. 为既有 C1 Actor 补齐 C6 的最小能力集合，不使用万能管理员完成业务命令；
3. 停止 Evidence，只提交同一个 Cost Command，要求 HTTP 依赖失败且 Cost 表零写入；恢复 Evidence 后重放同一 Command，只允许形成一条 Cost；
4. 对 Cost 追加 revision 2 correction，再以 stale revision 提交并验证零写入；
5. 停止 Party Directory，提交同一个 Engagement Command，要求零写入；恢复后同命令只形成一条 Engagement；
6. 使用相邻 Project Credential 提交 Assessment，要求拒绝；再以正确 Project 创建 Assessment，并拒绝 stale revision；
7. 创建一个版本化 Payable Suggestion。停止 Money 时 C6 只能保存 `target_unavailable`，不得伪造 Money 接受；恢复后同一建议 ID 重试，Money 独立接受并创建一条 Obligation，C6 再同步决定；
8. 读取 Cost、Engagement、Assessment、Suggestion、Money Intake、Money Obligation 和历史，核对精确所有权增量、工作负载凭证、HTTP 状态和 17/17 health；
9. 在已经 `passed` 的状态再次执行同一个验证器，要求全部 ID、revision、摘要和所有权增量保持不变。

## 4. 首轮发现的真实缺陷

### 4.1 连接器请求了错误的能力名称

Cost 与 Bonus 的 Money HTTP Client 原请求 `payable-suggestion.submit/read/withdraw`，但版本化 Capability Manifest 和 Workload Policy 的稳定动作是 `money-flow-ledger.payable-suggestion.submit/read/withdraw`。IAM 因动作不在工作负载白名单而拒绝签发，Cost 正确保留 `target_unavailable`，Money 零写入。

修复：两个连接器统一请求目标应用原生、完整命名空间动作；Cost 与 Bonus 的低并发单元测试均通过。

### 4.2 Money 验签器漏掉精确 Payable Suggestion Scope

修复 4.1 后，IAM 已连续签发两份精确 `payable_suggestion:{id}` 凭证，但 Money 的 IAM Provider 只接受普通 reference 与 receipt-set 的 resource scope，仍返回 401。该状态证明问题位于目标应用验签边界，而不是签发、网络或数据库。

修复：Money IAM Provider 仅允许以下三个动作绑定一个精确 `payable_suggestion` UUID：

- `money-flow-ledger.payable-suggestion.submit`；
- `money-flow-ledger.payable-suggestion.read`；
- `money-flow-ledger.payable-suggestion.withdraw`。

错误资源类型仍失败关闭；目标 IAM 单元测试通过。两个修复分别构建为单一 Linux AMD64 镜像，远端也只替换对应服务，没有同时重建完整栈。

## 5. 结果

首次正式通过共 24 个 HTTP 断言，完成态重跑 15 个断言，错误数均为 0。

最终事实：

- Cost `aa653d2e-0787-41dd-8162-508774ae0dad`：revision 2，纠正为追加 revision；
- Engagement `23193958-0856-4ed3-a97c-849db808e17e`：revision 1；
- Assessment `0806a79c-d8e2-4b28-a569-4b8d5475ea3e`：revision 1；
- Payable Suggestion `a341b2b3-774d-4daf-ae72-e11d79a3190e`：revision 3，目标 revision 2，`accepted`；
- Money Intake：revision 2；
- Money Obligation `a246f8ac-17cc-4eaf-95aa-95b83ab5fa85`：revision 1、`open/payable`，来源固定为上述 Engagement。

精确所有权增量：

| 所有者 | 新增或推进 | 必须保持不变 |
| --- | --- | --- |
| IAM | 14 个 C6 Grant | 不拥有业务事实 |
| Cost & Outsourcing | 1 Cost／2 revisions；1 Engagement／1 revision；1 Assessment revision；1 Suggestion／3 revisions；7 commands／7 audits | 不创建 Party、Contract、Evidence、Project 或 Money Obligation |
| Money Flow | 1 Intake／2 revisions；1 Obligation／1 revision | Party 0、Party Mapping 0 |
| Project Workspace | 0 Project、0 Link、0 Command、0 History | Project 保持 revision 11 |
| Party Directory | 0 Party、0 revision | C1／C2 Party 原样复用 |
| Contract Ledger | 0 Contract、0 revision、0 Settlement | C2 Contract／Settlement 原样复用 |
| Evidence Service | 0 File、0 Version | C2 award Evidence 原样复用 |

工作负载审计证明 Cost 分别使用精确 Project、两个 Party、Contract、File 和 Payable Suggestion scope；没有 tenant-wide 工作负载业务凭证。

## 6. 边界判定

本轮对 C6 的判定为 `supported`：

- Cost／Engagement 可以复用稳定引用和不可变快照，不需要复制 Party／Project／Contract／Evidence 领域模型；
- Payable Suggestion 是跨应用请求而非 Money 事实，Money 保留最终接受权和 Obligation 所有权；
- 连接器失败只推进来源应用自己的恢复状态，不制造半笔 Money 事实；
- 修复是版本化契约对齐和目标验签白名单补全，没有共享数据库、跨库读取、万能服务或 OA 专用核心分支；
- 完成态重跑证明恢复路径与验收器可重入。

该结论只覆盖 C6。完整第一阶段仍须连续通过 C7、C8，并完成最终静态边界审计和总报告收口。

## 7. 可复核证据

- `state-after-c6.json`：首次通过状态和全部检查；
- `state-after-c6-completed-rerun.json`：完成态重跑；
- `ownership-before.json`、`ownership-after.json`、`ownership-delta.json`：数据所有权；
- `http-evidence.tsv`、`http-evidence-completed-rerun.tsv`：逐请求状态与耗时；
- `workload-audits.tsv`：精确服务凭证；
- `cost-*`、`engagement-*`、`assessment-*`、`suggestion-*`、`money-*`：业务事实与历史；
- `c6-summary.md`、`c6-summary-completed-rerun.md`：机器运行摘要。

下一步只从 `state-after-c6.json` 进入 C7；不得新建 Tenant 或 Project，也不得并行执行 C8。
