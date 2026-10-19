# OA C3 Billing 最小受保护 HTTP 实验（2026-09-27）

## 判定

- 问题 A：领域服务、IAM verifier、PostgreSQL Store 和来源 resolver 即使分别通过，也不能证明真实 HTTP 请求不会越过 Project／Tenant Scope、泄露历史金额，或在来源故障时产生部分写入。
- 可验证结果 B：创建、详情和历史在一条真实 HTTP 链中组合 Billing 原生 IAM Credential、目标端最终授权、类型化来源读取和独立 PostgreSQL，并用正负矩阵证明失败关闭与恢复幂等。
- 本轮判定：`billing-protected-http-minimum-compatible`。
- C3 判定：仍为 `insufficient-evidence`。列表、纠正、登记无效、Evidence 管理、正式 IAM／Policy 签发、Nginx→Gateway、页面、D1、恢复和 C3 组合尚未验收。

## 1. 本轮范围

实现的受保护 endpoint：

- `POST /api/v1/invoice-registrations`：要求 `billing-invoice.record`；
- `GET /api/v1/invoice-registrations/{id}`：要求 `billing-invoice.read`；
- `GET /api/v1/invoice-registrations/{id}/history`：要求 `billing-invoice.history.read`；
- `GET /health/live` 与 `GET /health/ready`：只用于进程健康和数据库就绪。

HTTP 层使用严格 JSON、1 MiB 请求上限、安全响应 Header 和请求 ID。所有客户端提供的 Actor、Tenant、allowed actions 与 Scope Header 在鉴权前删除；Actor 只从通过复验的 Credential 派生。

## 2. 最终授权和字段收敛

1. Billing 原生 IAM Provider 复验 Ed25519 签名、Issuer、Audience、Tenant、有效期、Action、Scope 和 Credential ID。
2. Tenant Scope 可以创建；Project Scope 创建必须至少包含一个 Project 来源，并且请求中的每个 Project 来源都落在 Credential Scope 内。仅有相邻 Project 或精确登记 Scope 不能创建。
3. 详情和历史先按 Credential Tenant 从 Billing 自有数据库读取，再按精确登记或 Project Scope 最终授权。
4. `billing-invoice.amount.read` 独立于普通读取。缺少它时，当前与每条历史 revision 都返回 `amount_visible=false`，`amount_minor` 和 `currency` 为 `null`；有权限时返回精确 minor units。
5. `REFERENCE_INVALID` 映射为 422；`REFERENCE_SERVICE_UNAVAILABLE` 映射为可重试 503；两者不被混成伪造的 404 或 500。

实现复核时额外关闭了多 Project 输入的越权缝隙：不能凭 Project A 的 Scope，借由请求同时写入 Project B 来源。

## 3. 真实组合验证

集成测试启动两个真实 HTTP Server：一个模拟类型化 Party／Project 事实所有者端口，一个承载 Billing endpoint。Billing 使用真实 `auth.IAMProvider`、临时生成的 Ed25519 Key Set、真实 `invoice.Service`、`references.HTTPResolver`、`PostgresStore` 和 migration。PostgreSQL 18 运行于一次性容器和隔离 schema。

同一测试顺序证明：

- 来源 503 时创建返回 503，registrations、revisions、commands、audits 均为 0；
- 来源恢复后用相同 Command ID 创建成功，四类事实各为 1；
- 来源再次停机后成功 Command 重放仍返回原登记，四类事实保持各 1，证明重放先于外部访问；
- 无金额能力的详情和历史均无金额，有能力时详情返回 `12345 CNY`；
- 相邻读取角色不能创建，Project B Scope 不能为 Project A 创建；
- 有效签名但错误 Tenant 的 Credential 被拒绝；
- Project 状态不在白名单时返回 422，数据库计数不变；
- resolver 确实请求 `project-workspace.read + project:{id}` 的单精确资源 Credential。

## 4. 实际命令与结果

```bash
go test ./backend/services/billing-invoice/...
go test -tags=integration ./backend/services/billing-invoice/internal/httpserver

TEST_DATABASE_URL='postgres://billing_http:***@127.0.0.1:55440/billing_http_test?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./backend/services/billing-invoice/internal/httpserver \
  ./backend/services/billing-invoice/internal/invoice
```

以上均通过。无数据库环境时 integration 包可编译并明确跳过；给定一次性 PostgreSQL 后 HTTP 与领域真库 race 套件均实际通过。验证结束后临时容器已停止并删除。

## 5. 没有被本轮证明的内容

- 列表、纠正、登记无效、Evidence 管理和对外 reference endpoint；
- 正式 IAM／Policy 进程的登录、Overlay／Assignment 签发、账号停用和撤权；
- Nginx→Gateway 路由、Cookie／Session、浏览器和 Workbench；
- 可执行 Billing 进程、镜像、Suite Manifest、D1、备份和空目标恢复；
- C3 开票、应收、已收、余额连续页面和跨应用下钻。

## 6. 下一可执行缺口

补齐 Billing 剩余生命周期 HTTP：列表、纠正、登记无效、Evidence 管理和只读 reference；把配置、IAM／模板 Provider、PostgreSQL 与来源 endpoint 装入可运行进程；完成 Capability、Project／登记 Scope、状态、Expected Revision、金额／文件分权和错误恢复矩阵。该门禁通过后再进入 Gateway／D1，避免同时调试领域、权限、进程和部署四层变量。
