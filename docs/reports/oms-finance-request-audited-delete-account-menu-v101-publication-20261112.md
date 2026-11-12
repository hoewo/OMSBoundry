# OMS 财务申请审计删除与账号菜单收起 v1.101 发布记录

日期：2026-11-12

事项：BF-333、BF-334

状态：已发布并完成无业务写入验收

## 发布结果

- 公网入口：`https://oatest.antuinfo.cn/`
- Suite／Workbench：`v0.1.0-test.oms-v1.101`
- 源码分支：`codex/company-internal-management`
- 构建源码提交：`2ff6ec27f21f91d50c8cc3b8868aa9101cc1b9e0`
- 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.101_linux_amd64.tar.gz`
- 发布包 SHA-256：`a20581790ae705ab412514bd17c4f6b1c8a962c1e921d9eeb0e07f1bd9da2c49`
- 升级前备份：`/var/backups/omsbizfoundry/20261112T153745Z`

## 运行镜像

- Internal Finance Requests：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/internal-finance-requests:v0.1.0-test.oms-v1.101@sha256:af330765caa177f3c331a31b5f23554abd893f1194d330ad7614051e7a3adc81`
- Unified Workbench：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.101@sha256:65ba5bd4993ad59b787aa02a4664d3ca0f7819a908fdc910a7417767013e9526`

目标机 `docker inspect` 显示两个容器均运行以上不可变摘要且为 `healthy`。正式 D1 检查退出码为 0，21/21 个长运行容器健康；公网根入口返回到 `/workbench/?view=oms-v1`，Gateway readiness 返回 `ready`。

标准 `d1 upgrade` 已先生成并校验完整备份，随后在既有 Cost Outsourcing workload-key 的默认网络端点处停止；未移除 volume。对同一已校验候选执行标准 `d1 up` 后完成装配，再执行独立 `d1 check` 通过。该过程未回滚、未换包、未改业务数据。

## Capability 与授权

Finance Owner 最终校验独立 `internal-finance-requests.request.delete`，Workbench 只根据 `allowed_actions` 展示入口。IAM 启动时由版本化、可审计的 `oms-administrator-full-access-v1` 权限档案为目标组织的在用管理员协调该 Grant；数据库只读核对确认戴鹏飞、曹华、黄祯鑫的活动组织成员关系均有该能力，来源为 `administrator_profile`。

系统管理员账号仍不持有业务 Grant。发布过程中尝试用系统管理员执行普通业务 Grant bootstrap 时 IAM 返回 403，符合身份边界且没有写入任何 Grant；Finance 也不按“管理员”角色名自行放行。

## 公网资源核对

- `oms-phase1-finance-owner.js?v=20261112.1`：`cf0d1aa4490edc1e0f60ca1d6625b82a07275e4bfbc071250c5ff38c7dde9e27`
- `oms-phase1-identity.js?v=20261112.1`：`0c4ccf5995de84fc11332706c2c8e134c09fe1cd60578e0960551d539322e740`

公网读取摘要与构建源文件一致。

## 真实 Chrome 只读验收

以目标组织管理员戴鹏飞的现有登录态完成以下操作：

1. 完整刷新至 v1.101，进入“账号与租户”。
2. 打开戴鹏飞行的 `•••` 菜单，确认三个原有操作可见。
3. 点击菜单外的账号搜索框，菜单内容立即从可访问树消失，证明外部点击收起生效。
4. 进入“公司内部管理 → 财务管理”。线上当前有两条报销：`4·14·12`（已撤回）和 `ddd`（草稿），两行均按 Owner `allowed_actions` 显示“删除”。
5. 未点击删除，未打开删除提交确认，也未创建、提交或修改申请。

线上当前没有付款申请样本，因此未伪造付款申请做页面验收。报销与付款申请共用同一 Workbench 动作和 Finance 删除端口；付款申请删除已由单元测试和隔离 PostgreSQL 18 集成测试覆盖，包括他人付款草稿删除、已付款阻断和幂等／revision 冲突。

## 已验证门禁

- Finance：`go test -count=1 ./...`、`go vet ./...`
- Finance PostgreSQL 18：`go test -tags=integration -count=1 ./internal/finance`
- Identity Capability：`go test -count=1 ./internal/capability`、`go vet ./internal/capability`
- Workbench：`go test -count=1 ./...`、`go vet ./...`、变更 JavaScript `node --check`
- Workbench Node：307/307
- `acceptance/p9-full-composition/test-preflight`
- `deployments/d1/ops/test-p9-composition`
- 正式发布包 checksum、候选 IAM 合约预检、目标机 D1 check、21/21 healthy、公网资源摘要和真实 Chrome 只读验收

## 未外推

- 本次没有删除任何线上报销或付款申请，也没有新增付款申请作为验收数据。
- “删除”仍是 Finance 审计软删除，不是物理删除；申请、revision、Evidence 引用、发票指纹与删除审计继续保留。
- 已付款、已核销、超额或付款结果未知的申请仍不能删除；必须沿调整、冲销或付款异常处理路径解决。
- 发布不改变 Money、Evidence、Billing、IAM 或 People 的事实所有权。
