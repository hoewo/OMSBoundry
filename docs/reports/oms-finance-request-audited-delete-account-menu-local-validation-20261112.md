# OMS 财务申请审计删除与账号菜单收起本地验证

日期：2026-11-12

事项：BF-333、BF-334

状态：本地实现与验证通过；后续已发布为 v1.101

## A → B

问题 A：报销与新增付款申请缺少统一、可审计的删除能力；账号目录行操作菜单打开后点击页面其他位置不会收起。

可验证结果 B：授权人员可对尚未发生实际或未知付款的两类申请填写原因后软删除，默认列表隐藏且审计可回看；账号行菜单点击外部、切换其他菜单或按 Escape 收起。

## 不变量

- Finance Owner 依据独立 Capability、Tenant、expected revision、状态和付款事实最终授权，Workbench 只呈现 `allowed_actions`。
- 可删除状态为 `draft`、`withdrawn` 和零付款／零核销／零超额／零未知结果的 `payable`；实际付款、核销、超额或未知结果一律阻断。
- 删除保留申请、全部 revision、Evidence 精确引用、发票指纹、命令与删除 Actor、Credential、时间和必填原因；不级联写入 Money、Evidence、Billing。
- 已删除申请从默认列表隐藏，并从有效重复发票占用判断中排除；原指纹仍留作审计。
- 账号菜单改动只处理 Workbench 瞬时 UI 状态，不改变 IAM 或 People 事实。

## 实现

- 增加 `internal-finance-requests.request.delete` privileged Capability；内部 DELETE 接受本人兼容删除或独立租户级删除，后者强制 expected revision。
- Finance 列表增加受独立能力保护的 `include_deleted=true` 审计读取和页面级 `view_deleted` 动作；删除元数据显示原因、Actor、Credential 与时间。
- Workbench 为报销和付款申请展示同一“删除”动作，原因无默认值；默认列表继续排除删除和普通归档记录，“已删除”筛选仅授权可见。
- 重复发票查询保留指纹表，通过关联申请的 `deleted_at` 排除已删除占用。
- 账号目录统一关闭已展开的行操作 `<details>`：打开其他菜单、点击外部或 Escape 时收起，菜单内部操作先关闭再执行原命令。

## 实际验证

- `backend/services/internal-finance-requests`：`go test -count=1 ./...`、`go vet ./...` 通过。
- 一次性 PostgreSQL 18：`go test -tags=integration -count=1 ./internal/finance` 通过；覆盖他人付款草稿删除、报销提交后撤回删除、已付款阻断、删除后同发票重新提交。
- `backend/platform/identity-access`：Capability 专项 `go test -count=1 ./internal/capability`、`go vet ./internal/capability` 通过。
- `frontend/oms-workbench`：`go test -count=1 ./...`、`go vet ./...`、JavaScript `node --check` 和 Node 全量 307/307 通过。
- `acceptance/p9-full-composition/test-preflight` 与 `deployments/d1/ops/test-p9-composition` 通过。
- `git diff --check` 通过。

## 未外推

- 本报告不表示 `oatest` 已更新；发布包、目标机、公网资源和真实 Chrome 尚未验证。
- 本地测试使用隔离数据；没有删除线上财务申请，没有改写任何 Money、Evidence、Billing 或账号数据。
- 新 Capability 不因账号是管理员而自动获得；部署时必须对目标测试操作主体显式授予并保留授权审计。

## 后续发布

该本地基线随后以 v1.101 发布；IAM 通过版本化、可审计的管理员权限档案把独立 Capability 映射给目标组织的在用管理员，而非由 Finance 按角色名放行。目标机、公网资源、真实 Chrome 和未写业务数据的发布证据见[发布记录](./oms-finance-request-audited-delete-account-menu-v101-publication-20261112.md)。
