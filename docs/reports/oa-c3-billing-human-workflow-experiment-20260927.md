# OA C3 Billing 人类工作流与正式入口实验（2026-09-27）

## 1. 本轮要回答的问题

**A：Billing 已有完整 API 与 D1 装配，但普通用户还不能从统一工作台完成可理解的开票登记工作。**

**B：在不共享 Party、Project、Money、Contract 或 Evidence 数据库的前提下，用户可以经 `Nginx → Gateway → IAM → Billing` 完成登记、查询、纠错、票据文件引用、历史和登记无效；权限变化、并发冲突和上游故障都有明确且可恢复的结果。**

本轮不实现 C3 的 Contract／Money／Billing 跨应用只读组合，也不宣称 OA 完整体已经通过。

## 2. 不能破坏的边界

- Billing 只拥有“已开票登记”、金额、当时的 Party／来源快照、Evidence 引用和不可变 revision 历史。
- Party、Project、Contract、Money、Evidence 仍由各自应用拥有并写入；Billing 只能通过版本化、精确 Scope 的端口读取。
- IAM 签发短期 Capability Credential；Billing 根据 Tenant、Scope、当前状态和自身 Capability Manifest 最终授权。
- 金额不可见时，当前详情、列表汇总与历史 snapshot 均由服务端脱敏；前端隐藏不构成安全边界。
- 纠错、文件引用变化与登记无效必须追加 revision，不覆盖旧事实。
- `409` 必须重新读取最新 revision，不自动重放旧编辑；`503` 必须保留本次 Command ID 供原命令安全重试。

## 3. 实现范围

新增 Billing 人类工作区：

- 列表、状态／Project 筛选、有效金额汇总和服务端 `allowed_actions`；
- 新建已开票登记，显式输入开票／收票 Party revision 和来源 revision；
- 详情、来源快照、Party 快照、Evidence 引用和不可变历史；
- 显式纠错、完整 Evidence 引用集合替换和“系统登记无效”；
- `401/403` 退出编辑并回到权限恢复路径，`409` 载入最新 revision，`422` 展示引用错误，`503` 保留输入和原 Command ID；
- Gateway 权限刷新，金额无权时隐藏金额字段且不提交金额修改；
- Suite Manifest 启用 Billing 时，Unified Workbench 显示 Billing 入口。

实现过程中发现并修复一个安全缺口：仅有 `billing-invoice.correct`、没有 `billing-invoice.amount.read` 的调用者原本可以盲写 `amount_minor`／`currency`。目标应用现在直接拒绝该请求，避免“看不见但能改”的字段级越权。

## 4. 验证步骤与结果

| 层次 | 实际执行 | 结果 |
| --- | --- | --- |
| 单元与 HTTP | `go test ./backend/services/billing-invoice/... ./frontend/oms-workbench/...` | 通过；包含公共页面／脚本、受保护 API、盲写金额拒绝与有权金额纠错 |
| 交互渲染 | 本机 headless Chrome 以 1440×1050 渲染 `arckit/interaction/billing-workspace/default.html` | 通过；左侧筛选／汇总／记录和右侧状态／事实／动作／页签层级可读，无重叠或截断；正式登录态浏览器点击矩阵仍单列待验 |
| 静态检查 | `go vet ./backend/services/billing-invoice/... ./frontend/oms-workbench/...`、`node --check`、`sh -n` | 通过 |
| 发布／入口回归 | D1 image、release bundle、upgrade selection、ingress mode tests | 全部通过 |
| D1 健康 | `D1_ENV_FILE=deployments/d1/.env D1_PROJECT_NAME=omsbizfoundry deployments/d1/ops/d1 check` | Billing schema `000001_billing_invoice.sql` 与全部启用服务健康 |
| 正式入口 | `acceptance/d1-delivery/verify-running` | 通过；页面、脚本、Workbench 入口与 Billing 生命周期均经 Nginx 路由验证 |
| 权限变化 | 撤销／恢复 `billing-invoice.amount.read` 后经 Gateway 刷新 | 通过；当前详情和全部历史金额收敛，盲写金额返回 `403`，恢复后金额重新可见 |
| 并发 | 使用旧 `expected_revision` 再提交纠错 | 返回 `409`，当前 revision 与金额不变 |
| 故障隔离 | `D1_ACCEPTANCE_DISRUPTIVE=yes acceptance/d1-delivery/verify-running` | 通过；Party 停机时新登记失败关闭且零写入，恢复后新命令单次提交；组合既有故障回归也通过 |

非破坏验收和破坏性验收都在同一 `omsbizfoundry` D1 组合、同一 Nginx 公网边缘模型和同一 Suite Manifest 下执行，没有从宿主机直连 Billing 端口绕过 Gateway／IAM。

## 5. 本轮结论

Billing 已从 API-only 成为可经正式组合入口使用的人类工作区。它证明了一个小应用可以在保持自身数据库、原生最终授权和精确外部引用的情况下装入统一 OA 组合，并能对撤权、并发和依赖故障给出可恢复行为。

这项证据只关闭“Billing 人类工作流与正式 D1 入口”缺口。以下内容仍然开放：

- C3 开票／应收义务／回款的分段只读组合与跨应用延迟体验；
- 完整业务角色、资源范围、状态、账号停用和双 Tenant 交叉矩阵；
- Billing-only 最小交付组合；
- 公司 Ubuntu 或公网真实环境；
- 浏览器尺寸／辅助技术的完整人工矩阵。

## 6. 下一验证切片

下一步应实现 C3 分段只读组合：由组合读模型分别读取 Billing、Money Obligation／Cash 和必要的 Contract／Project 摘要，不跨库联表、不把票据与应收或回款强行一对一绑定，并记录每段来源、revision、局部失败和延迟。该切片要直接回答连接器／API 解耦是否会造成不可接受的用户等待，而不是先做新的公共平台。
