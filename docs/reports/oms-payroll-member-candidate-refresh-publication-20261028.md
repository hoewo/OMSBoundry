# OMS 工资表候选成员刷新发布记录（2026-10-28）

## 结论

用户反馈的“已有 2 个员工但工资表仍提示没有候选成员”已修复并发布。根因不是员工数量、工资账号必填或 Payroll 未保存，而是工资执行脚本只在 Workbench 启动时读取一次 People 与 Payroll；员工2在页面启动后创建并成功写入薪酬档案，工资页面仍用旧的空投影判断候选。

## 事实核对

- 当前 Tenant 有两名在职成员：员工1、员工2。
- 员工2已有 `2026-10` Compensation Profile：固定工资 20,000 元、社保基数 1,200 元、公积金基数 1,000 元。
- 员工1没有 Compensation Profile，这是 BF-282 之前部分成功后未自动重放的既有状态。
- Payment Profile 为 0 条；工资账号按已接受规则保持可选，不构成建表条件。
- Gateway／Payroll 日志确认员工2的薪酬写入返回 200，说明 Owner 事实已落库。

## 修复

“新建工资表”和“同步成员”在计算候选前先重新读取工资表、缴费方案、People 成员和 Payroll 薪酬档案。候选规则保持为：成员 `active`、存在 Compensation Profile、且尚未进入目标工资表。Payment Profile 不参与候选筛选；缺账户的工资行由 `payment_account_missing` 表达，实际发放前仍须补齐账户并显式同步。

## 验证与发布

- `node --check frontend/oms-workbench/internal/server/oms_phase1_payroll_owner.js`：通过。
- `node --test` 相关 8 项回归：8/8 通过。
- `go test ./frontend/oms-workbench/internal/server`：通过。
- `go vet ./frontend/oms-workbench/internal/server`：通过。
- `git diff --check`：通过。
- 发布前候选预检通过，联合备份：`20261028T124216Z`。
- Unified Workbench：`v0.1.0-test.oms-v1.51@sha256:0f5e681c1074950882548ed8a375cdf2e855ef0025e0e9f5fae943af057cca7e`。
- D1 全栈检查通过，19/19 长运行容器 healthy。
- 真实 Chrome 刷新后打开工资管理，点击“新建工资表”：正常显示月度工资表表单，候选只有员工2并标注“已配置薪酬档案”；员工1正确排除。
- 验证后取消表单，没有创建工资表，也没有新增业务写入。

## 边界

本次只替换 Workbench，未修改 People、Payroll、Gateway、IAM、数据库 schema、Grant 或业务事实。员工1的薪酬档案仍需用户按真实金额显式补齐；系统不会根据员工2或历史输入推测、自动重放。工资账号仍是可选档案，但没有账户的工资行不能确认实际发放。
