# Billing / Invoice

Billing / Invoice 独立登记“我方已经开具的票据”。当前实现包含领域核心、独立 PostgreSQL migration／Repository、版本化 Capability Manifest、目标应用 IAM Credential 复验、可信身份 Header 清洗、权限收敛响应视图、使用工作负载身份访问 Party／Project／Contract／Settlement／Money Obligation／Evidence 的类型化 HTTP reference resolver，以及创建、列表、详情、纠正、登记无效、Evidence 管理、历史和工作负载只读引用的完整生命周期 HTTP。Linux AMD64 非 root 镜像、pull-only release、Suite Manifest、Nginx→Gateway→IAM 正式入口、D1 故障恢复和联合空目标还原已经验证；人类页面与 C3 只读组合尚未实现。

当前核心支持：

- `recorded` 与 `registration_invalidated` 两种系统登记状态；
- 精确 `amount_minor + currency`，且服务启动时必须提供非空允许币种集合；当前参考切片只启用 `CNY`；
- 开票方／受票方 Party 快照、类型化来源快照和 Evidence File Version 引用；
- 创建、资料纠正、Evidence 替换、登记无效、当前详情、历史和只读引用；
- Command ID 幂等、Actor／负载冲突、Expected Revision、依赖失败零部分写入；
- 当前投影、不可变 revision、command result 和 audit 在一个数据库事务提交。
- Ed25519 Credential 严格复验 issuer、audience、tenant、TTL、Action 和 Tenant／Project／精确登记 Scope；
- 身份管理员不继承业务动作，模板人类凭证也不含工作负载专用 `billing-invoice.reference.read`；
- 服务端金额视图在无 `billing-invoice.amount.read` 时返回 `amount_visible=false` 与 `null` 金额／币种，历史 revision 同样收敛。
- Billing 工作负载只可向各来源交换单 action、单精确资源 Scope 的短期 Credential；动态凭证首次返回 401 时只失效并重取该资源一次；
- resolver 严格复验 Tenant、资源类型、ID、revision、来源状态与非空摘要；业务引用不存在／过期归类为无效输入，IAM、网络、拒绝、畸形响应和服务异常归类为依赖不可用；
- 外部依赖失败不落本地事实；使用同一 Command ID 恢复重试只提交一次，成功重放先命中本地 command result，不再访问外部依赖。
- 全部第一版 API Port 由目标应用原生 IAM／模板 Provider 保护；Project Scope 创建要求每个 Project 来源都在凭证 Scope 内，列表要求匹配 `project_id`，来源纠正不得换挂相邻 Project；
- `billing-invoice.reference.read` 只接受 IAM 工作负载的单 action、单精确 `invoice_registration:{id}` Scope；
- 列表以不透明 keyset cursor 稳定分页，只汇总当前 `recorded` 投影；没有金额动作时条目和汇总均返回 `null`；
- 真实 PostgreSQL 集成用例组合 Ed25519 IAM Credential、类型化来源服务与 Store，覆盖相邻角色、Project B、跨 Tenant、错误来源状态、503 零写入、恢复重试和金额／历史收敛。

本地启动前复制并填写 `.env.example` 中的配置；模板模式要求五个依赖各自的引用 Token，IAM 模式禁止静态引用 Token，并使用 Billing 独立工作负载密钥动态交换精确 Credential。加载环境后运行：

```bash
go run ./cmd/server
```

普通验证：

```bash
go test ./...
go test -race ./...
go vet ./...
```

PostgreSQL 18 真库验证：

```bash
TEST_DATABASE_URL='postgres://billing_invoice:***@127.0.0.1:5432/billing_invoice?sslmode=disable' \
  make -C ../.. billing-invoice-core-acceptance
```

受保护 HTTP 组合验证：

```bash
TEST_DATABASE_URL='postgres://billing_invoice:***@127.0.0.1:5432/billing_invoice?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/httpserver ./internal/invoice
```

这些验收会在给定数据库应用 Billing migration 并运行生命周期、幂等、并发、依赖失败、跨 Tenant、不可变保护，以及最小 HTTP 权限／恢复矩阵。只应使用可清空的测试数据库。正式 D1 入口与联合恢复使用 `acceptance/d1-delivery/verify-running` 和 `deployments/d1/ops/d1 backup|restore` 验证；结果与边界见 [C3 Billing D1 交付与恢复实验](../../../docs/reports/oa-c3-billing-d1-delivery-recovery-experiment-20260927.md)。
