# OMS 多项报销单行起步与布局收敛本地验证（BF-340）

日期：2026-11-17（GMT+8）  
状态：本地实现与回归通过；后续已随 Suite v1.106 发布，见[发布记录](./oms-reimbursement-single-initial-line-layout-v106-publication-20261117.md)

## 可验证结果

- 新建报销从单项切到多项时默认仍只有 1 条明细，不再自动生成第二条；第二条及后续明细只由“添加一项”产生。
- 多项模式可用 1 条明细保存草稿；选择“保存并提交”时，Workbench 与 Internal Finance Requests 都要求至少 2 条。
- 每条明细收敛为一条连续工作流：明细标题在上，名称／金额／日期同一行，发票方式与当前状态在下；删除左右双区、竖分隔和重复“关联发票”标题。
- 已持久化额外明细的可逆切换、每条明细一张发票、Evidence、精确金额、重复占用和审计边界保持不变。

## 已运行验证

- `node --check internal/server/oms_phase1_finance_owner.js`：通过。
- `node --test internal/server/oms_phase1_finance_owner.test.mjs`：10/10 通过。
- `node --test internal/server/*.test.mjs`：310/310 通过。
- Unified Workbench：`go test ./...`、`go vet ./...` 通过。
- Internal Finance Requests：`go test ./...`、`go vet ./...` 通过。
- 一次性 PostgreSQL 18.6：`TEST_DATABASE_URL=... go test -count=1 -tags=integration ./internal/finance` 通过；临时容器已删除。
- `git diff --check`：通过。

## 后续发布结果

- Finance Owner 与 Workbench v1.106 镜像、发布包、升级前备份、正式 D1 check、21/21 容器健康和公网资源摘要均通过。
- 浏览器刷新后既有登录会话已过期；本轮未代用户输入登录凭据，未执行线上报销写入或登录后视觉验收。
