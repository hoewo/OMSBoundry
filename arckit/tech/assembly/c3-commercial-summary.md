# C3 项目经营链路只读组合技术方案

## 决策

C3 第一版由 Project Workspace 承载只读组合端点，因为 Project 已拥有业务上下文和类型化 Resource Link。该职责不创建通用 BFF：它只解释“这个 Project 已连接的经营事实”，不接受任意查询、不拥有来源数据，也不替 Workbench 组合业务事实。

## 调用链

```text
Browser → Nginx → Gateway → Project Workspace
  → active Project Resource Links
  → IAM service credential exchange（每个精确资源）
    → Contract reference v1.2
    → Money obligation / transaction reference v1.2
    → Billing invoice registration reference v1
  ← ordered Commercial Segments
```

浏览器持有 Project 人类凭证，其中读取 C3 还必须显式包含 `project-workspace.commercial-summary.read` 和当前 Project Scope；通用 `project-workspace.read` 不授予金额组合。Project 的 Ed25519 工作负载私钥只在 Project 容器内；IAM 根据版本化 Workload Policy 签发最长一分钟、单 action、精确资源 Scope 的目标凭证。目标应用继续复验 tenant、audience、action、resource type 和 resource ID。

## 运行模型

- API：`GET /api/v1/projects/{project_id}/commercial-summary`。
- 发现：只读取 Project 自有 `project_resource_links` 的 active 记录。
- 支持组合：Contract/contract、Money/obligation、Money/transaction、Billing/invoice_registration。
- 调度：保留 Link 顺序，最多八个 goroutine 并行；每个 HTTP Client 受 `REFERENCE_TIMEOUT` 限制。
- 降级：`ErrReferenceInvalid → invalid`；网络、IAM、5xx、401 重换失败或契约解码失败 → `unavailable`；source revision 不同 → `revision_changed`。
- 生命周期：响应构造后立即丢弃来源投影，不落 Project PostgreSQL、不写事件、不缓存为来源真相。
- 深链：响应只给 Gateway 内部稳定路径；页面先用当前 IAM Session 为目标应用重新授权，再导航。

## 契约与精确表示

- Contract 金额沿用十进制定点字符串，`null`／省略表示未知。
- Money 金额沿用十进制定点字符串；组合层不转为二进制浮点计算。
- Billing 金额沿用 `amount_minor` 整数；页面只做显示格式化。
- 组合层不比较、净额或累计这些不同语义金额。
- `linked_revision` 解释 Link 建立时看到的来源，`source_revision` 解释本次读取，不相等时必须显式呈现。

## 配置与迁移

- Project 增加 `BILLING_BASE_URL`／template-only `BILLING_ACCESS_TOKEN`。
- 正式 IAM 模式禁止 People、Party、Contract、Money、Evidence、Design、Billing 的全部静态下游 Token；配置任何下游 URL 时必须完整配置工作负载身份。
- D1 为 Project 注入 Contract、Money、Billing 内网地址，不对公网暴露它们的新端口。
- migration `000004_billing_invoice_resource_links.sql` 只扩展 Project Link 枚举；没有跨库外键。
- Workload Policy 只追加 Contract `contract`、Money `obligation|transaction`、Billing `invoice_registration` 三组精确权限。

## 测试证据边界

2026-09-27 的隔离 D1 已通过 Nginx → Gateway → IAM → Project 正式入口建立真实 Contract、Obligation、Transaction 与 Invoice Registration Resource Link，并取得 100 个 HTTP 断言、0 个状态差异。总经理／财务允许、生产经理拒绝；撤权后的旧凭证残余窗口、新凭证 403、恢复授权、账号停用 401 均符合契约。Contract、Money、Billing 和 IAM 逐项停机时组合只把相应 segment 标为 `unavailable`，恢复后重新读取，Project revision 不变。30 次读取的并行总耗时 p50/p95 为 13/220 ms，低于 800 ms 本地门槛。

本轮同时修复 D1 Project 的三个错误内部端口，并确认 Suite Manifest enabled 集合必须与运行容器集合一致。页面资源与脚本已验证，但 macOS 自动化通道在最终点击时仍报告锁屏；双 Tenant、真实 Ubuntu、来源 revision 变化的人类恢复和自动业务动作不在本结论内。直接证据见 [OA C3 项目经营链路 D1 实验报告](../../../docs/reports/oa-c3-commercial-summary-d1-experiment-20260927.md)。当前状态为 `local-d1-supported`，不是 OA 完整体通过。
