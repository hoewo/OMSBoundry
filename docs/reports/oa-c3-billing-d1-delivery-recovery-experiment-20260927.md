# OA C3 Billing D1 交付与恢复实验

日期：2026-09-27  
结论：`billing-d1-formal-entry-recovery-compatible`；C3 完整体仍为 `insufficient-evidence`

## 问题 A → 可验证结果 B

Billing 只有本地进程证据，尚不能证明它能作为与其他应用等价的正式装配成员交付 → 以同一 Billing 发布物完成 Linux AMD64 非 root 镜像、D1 Suite Manifest、Nginx→Gateway→IAM 正式入口、精确工作负载凭证、故障恢复和联合空目标还原，并保持独立数据库与事实所有权。

## 本次范围

- 将 `billing-invoice` 接入 D1 build／pull、镜像发布／提升、独立数据库、migration、工作负载密钥、Gateway `/billing` 路由和 Suite Manifest；
- Billing 作为当前 API-only 应用启用，暂不伪造 Workbench 业务页面；
- 通过正式 IAM 用户 Credential 执行登记、读取、纠正、Evidence 关联、登记无效、历史和 Project Scope 列表；
- 逐个引用使用 Billing 工作负载身份向 Party、Project 和 Evidence 交换单 action、单精确资源 Credential；
- Party 停机时验证 Billing 新写入 `503`、零登记／零命令，恢复后以原 Command ID 单次提交；
- 将 Billing dump 与启用选择纳入 D1 联合备份，销毁 Compose 数据卷后恢复并继续写入。

不在本次范围：Billing 人类页面、C3 开票／应收／回款只读组合、完整角色／撤权／账号停用矩阵、Billing-only 最小组合、双 Tenant、真实 Ubuntu／公网环境。

## 实际证据

### 1. 可移植发布

- `deployments/d1/docker/Dockerfile.billing` 实际构建 `linux/amd64` 镜像；
- 镜像元数据为 `linux/amd64 user=postgres`，容器内实际 UID／GID 为 `70/70`；
- D1 release、publish、promote 和 pull-only 门禁均包含 `D1_BILLING_IMAGE`，目标 ECS 不需要现场编译源码；
- 应用只加入内部网络，不发布宿主端口，只有 Nginx 暴露统一入口。

### 2. 正式入口与权限

运行中的隔离 D1 从 Nginx 统一入口完成：Suite Session 登录；Gateway 删除并重新派生可信身份上下文；IAM 为 `billing-invoice` 签发原生用户 Credential；Billing 最终复验 Tenant、Audience、Action、Project／登记 Scope、状态和 Expected Revision；Billing 再以自身工作负载身份分别读取精确 Party、Project 与 Evidence，IAM 审计包含预期 audience、action 和单资源 Scope。

验收完成登记 revision 1、纠正 revision 2、Evidence revision 3、登记无效 revision 4，并验证历史 `[4,3,2,1]`、无效登记不进入有效金额汇总、Project Scope 无过滤列表拒绝、当前 Project 允许、相邻 Project 拒绝。来源 Project 先按自身规则从 `draft` 进入 `pending_completion` revision 2；Billing 正确拒绝草稿并只冻结可用的精确 revision。

### 3. 故障与恢复

启用 `D1_ACCEPTANCE_DISRUPTIVE=yes` 后，停止 Party Directory：新 Billing 登记返回 `503`；`invoice_registrations` 与 `invoice_registration_commands` 均无部分写入；Party 恢复后以原 Command ID 重试成功且仅提交一次；既有 Contract、Money、People、Project 与 Evidence 隔离／恢复回归同时通过。

### 4. 联合备份与空目标还原

备份 `20260927T114829Z` 包含 `billing-invoice.dump`、IAM 签名材料、其余启用应用数据、`suite-selection.env` 和 `checksums.sha256`；八个应用开关均参与选择校验，所有文件 SHA-256 通过。

销毁隔离 Compose 项目的全部数据卷后，在新空卷恢复。Billing 恢复前后摘要完全一致：

```text
current_count|current_revision_sum|revision_count|audit_count|projection_digest
3|9|9|9|27e791a0dd194d69a0dcea9caa459f3a
```

恢复后所有启用服务健康、Billing schema 为 `000001_billing_invoice.sql`，完整正式入口验收再次通过并产生新的业务写入。

## 实际运行的门禁

```bash
go test ./backend/services/billing-invoice/... ./acceptance/access-contract/... ./packages/workload-identity/...
go vet ./backend/services/billing-invoice/... ./acceptance/access-contract/... ./packages/workload-identity/...
sh deployments/d1/ops/test-image-mode
sh deployments/d1/ops/test-release-bundle
sh deployments/d1/ops/test-upgrade-selection
sh deployments/d1/ops/test-ingress-mode
deployments/d1/ops/publish-images self-test
deployments/d1/ops/promote-images self-test
acceptance/d1-delivery/verify-running
D1_ACCEPTANCE_DISRUPTIVE=yes acceptance/d1-delivery/verify-running
deployments/d1/ops/d1 backup
D1_CONFIRM_EMPTY_RESTORE=yes deployments/d1/ops/d1 restore 20260927T114829Z
docker buildx build --platform linux/amd64 --load -f deployments/d1/docker/Dockerfile.billing .
```

以上均实际通过。恢复后的正式入口验收也再次通过。

## 判定与下一缺口

本次关闭 Billing 的可移植交付、正式 IAM／Gateway 入口、D1 故障恢复和联合空目标还原缺口；不再把这些列为未实现。C3 总目标仍不能判定通过，因为尚缺：

1. 人类可操作 Billing 页面与 Workbench 入口；
2. Contract／Money／Billing 分段聚合的 C3 开票—应收—回款只读视图与降级语义；
3. 业务角色、撤权、账号停用、双 Tenant 和 Billing-only 最小组合矩阵；
4. 公司 Ubuntu 或个人试验田的真实环境证据。
