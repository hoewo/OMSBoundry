# OMS 多项报销单行起步与布局收敛本地验证（BF-340）

日期：2026-11-17（GMT+8）  
状态：本地实现与回归通过；尚未登记为公网发布

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

## 尚未完成

- 发布候选镜像／发布包、升级前备份、D1 check、容器健康、公网静态资源摘要和真实 Chrome 视觉复验尚未在本记录中宣称通过。
