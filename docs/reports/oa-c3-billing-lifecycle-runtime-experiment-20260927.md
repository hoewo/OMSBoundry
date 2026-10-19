# OA C3 Billing 完整本地生命周期与运行时实验（2026-09-27）

## 结论

- 结果：`billing-local-lifecycle-runtime-compatible`。
- Billing 已有可配置 `cmd/server`，把独立 PostgreSQL、IAM／模板 Provider、五类类型化依赖端点和允许币种装入同一进程。
- 创建、列表、详情、纠正、登记无效、Evidence 管理、历史和工作负载只读引用已经进入受保护 HTTP；真实 PostgreSQL 18 联调完成 revision 1→4 生命周期。
- C3 总判定仍为 `insufficient-evidence`。本轮没有运行真实 IAM／Policy 签发进程、Nginx→Gateway、页面、D1、备份恢复或 C3 Contract／Money／Billing 组合。

## A→B 与不变量

问题 A：领域核心和前三个 HTTP endpoint 已成立，但剩余生命周期仍只能通过内部 Go 调用，无法作为一个配置明确、权限失败关闭的独立应用继续进入交付验证。

可验证结果 B：同一个 Billing 进程通过版本化 HTTP 完成登记、查询、纠正、文件引用、登记无效和历史解释；列表金额、Project Scope、Expected Revision 与工作负载引用都由目标应用收敛，任何相邻资源或依赖失败不产生部分事实。

保持不变量：

- Billing 独占登记、revision、command 和 audit，不读取其他应用数据库；
- Project Scope 列表必须携带且匹配 `project_id`；纠正来源时所有 Project 引用均须在凭证 Scope 内；
- 无 `billing-invoice.amount.read` 时条目与汇总都返回 `null`，不是零或客户端遮盖；
- `billing-invoice.reference.read` 只接受 IAM 工作负载的单 action、单精确 `invoice_registration:{id}` Scope；
- Evidence 命令先验证精确 File Version，成功后才原子追加 Billing revision；
- `registration_invalidated` 只否定系统登记，并从有效金额汇总排除，不代表现实票据红冲或作废。

## 实现结果

- `GET /api/v1/invoice-registrations`：项目、Party、状态、日期和不透明 keyset cursor 筛选；只汇总当前 `recorded` 投影；金额动作独立收敛。
- `POST /api/v1/invoice-registrations/{id}/corrections`：先按当前登记最终授权，再校验来源变更 Scope、Expected Revision 和精确依赖 revision。
- `POST /api/v1/invoice-registrations/{id}/invalidation`：追加无效 revision，保留原事实和原因。
- `PUT /api/v1/invoice-registrations/{id}/evidence`：逐一验证同 Tenant 精确 File Version 后替换引用集合，不删除 Evidence 字节。
- `GET /api/v1/references/invoice-registrations/{id}`：返回 Billing 自有最小摘要，不代理其他应用事实。
- `cmd/server`：支持 `auto|verify` migration、模板／IAM 双 Provider、静态本地引用 Token／正式工作负载交换二选一、五个显式来源 URL、允许币种与超时配置；混用静态凭证和 IAM 时启动失败。

## 直接验证

```bash
go test ./backend/services/billing-invoice/...
go test -race ./backend/services/billing-invoice/...
go vet ./backend/services/billing-invoice/...
go test ./acceptance/access-contract ./packages/workload-identity/...
go vet ./acceptance/access-contract ./packages/workload-identity/...

TEST_DATABASE_URL='postgres://billing_http:***@127.0.0.1:55442/billing_http_test?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./backend/services/billing-invoice/internal/httpserver \
  ./backend/services/billing-invoice/internal/invoice
```

全部通过。临时 PostgreSQL 18.6 容器随后停止并删除。联合用例证明：

1. 来源 503 时 registrations／revisions／commands／audits 均为 0；恢复后同 Command 只创建一次。
2. Project Scope 无 `project_id` 的列表拒绝；匹配 Project 可见，相邻 Project 不可用于创建或来源纠正。
3. 无金额动作时当前、历史、列表条目与汇总均不泄漏金额；有权限时有效汇总为 12345。
4. 纠正、Evidence 和登记无效按 revision 1→2→3→4 追加，历史保留四条不可变快照。
5. 精确工作负载引用在 revision 3 返回 Billing 摘要；宽 Scope 或多 action 由 HTTP 单元矩阵拒绝。
6. 登记无效后记录仍可查询和审计，但有效金额汇总变为 0。

## 未覆盖与下一门禁

- 真实 IAM／Policy Grant、Project Assignment、撤权、账号停用和凭证残余窗口；
- 非 root 镜像、release、Suite Manifest、Gateway 静态路由和 Nginx 入口；
- 独立备份、空目标恢复、恢复后继续写入与联合恢复；
- Billing 人类页面和 C3 开票／应收／已收／余额只读组合；
- Billing-only 最小组合、D1 正式入口、完整双 Tenant 及现有 C1／C2 回归。

下一步应先完成 Billing 的可移植交付与 D1 装配／恢复，再进入正式 IAM 权限矩阵和 C3 页面，避免把本地生命周期通过误报为完整 OA C3 通过。
