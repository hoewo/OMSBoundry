# OMS 报销模式切换与横向明细本地验证（BF-339）

日期：2026-11-16（GMT+8）

状态：本地实现与回归通过，尚未发布

## 问题与结果

多项模式会自动增加第二条空明细，旧逻辑又要求先把额外明细移除才能切回单项，导致刚切到多项就像被锁住；同时每条明细使用卡片套发票卡片，纵向高度和缩进过多。

修复后，新建或尚未保存的多项明细切回单项时，第 2 项及之后的明细只暂存在当前编辑会话；再次切回多项会原样恢复。已保存的额外明细继续失败关闭，避免模式开关静默改写 revision。桌面明细改为左侧基本字段、右侧发票工作区的扁平双列，移动端回落单列。

## 边界

- 暂存只存在于当前打开的 Workbench 编辑对象，不进入浏览器持久化、API 请求或 Finance Owner。
- 选择单项并保存时只提交当前第一条明细；暂存的未保存明细不会进入业务事实。
- 已持久化多项不能借模式切换删除，继续要求显式、可审计的 Owner 语义。
- 本批只修改 Workbench HTML／CSS／JavaScript 与资源版本，不改变 Finance API、schema、Capability、金额或发票所有权。

## 已运行验证

- `node --check internal/server/oms_phase1_finance_owner.js`：通过。
- `node --test internal/server/oms_phase1_finance_owner.test.mjs`：10/10 通过。
- `node --test internal/server/*.test.mjs`：310/310 通过。
- `go test ./...`：通过。
- `go vet ./...`：通过。
- `git diff --check`：通过。

## 发布前门禁

- 构建并摘要锁定新的 Unified Workbench 镜像。
- 复用 v1.104 其余镜像形成兼容闭包，执行发布包校验、升级前备份和独立 D1 check。
- 公网核对 JS／CSS 摘要，并用真实浏览器执行“单项 → 多项 → 单项 → 多项”无业务写入切换及桌面布局检查。
