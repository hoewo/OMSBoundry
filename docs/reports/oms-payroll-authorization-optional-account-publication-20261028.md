# OMS Payroll 授权回跳与可选工资账号发布记录

日期：2026-10-28
范围：BF-282
状态：代码、本地门禁、D1 公网发布与只读浏览器验收通过；既有成员薪酬待用户显式续办

## 问题与根因

新增成员时 People & Groups 已返回 201 并建立 Person，Workbench 随后为 Payroll 强制换取 Capability。Gateway 成功签发后返回 303、设置 `/payroll/v1` 路径 Cookie，并导航到 `/payroll/v1/`；Payroll 是 API-only Owner，没有根页面，因此最终 GET 返回 404。Workbench 当时使用 `redirect: follow`，错误地以最终根页面响应判断授权失败，导致真正的 Compensation Profile PUT 从未发出。

线上取证显示 `2026-10-28T12:21:03Z` Person 创建成功，`12:21:04Z` Payroll authorize 返回 303，随后 `/payroll/v1/` 返回 404；当前 Tenant 的 Payroll Compensation Profile 与 Payment Profile 均为空。截图中的工资账号为空，Payment Profile 按设计未进入提交分支，实际失败是薪酬档案授权前置步骤。

## 修复

- Workbench Capability 授权改用 `redirect: manual`；明确 3xx 或浏览器 `opaqueredirect` 作为 Gateway 成功签发结果，非重定向 4xx／5xx 仍解析并失败关闭。
- 成员连续命令记录当前阶段；People 已保存后的错误精确显示成员状态、薪酬档案或工资账户档案。
- 工资账号继续可选；只有 `accountNumber` 非空时才验证账户名／开户行并调用 Payroll Payment Profile。
- 不自动重放此前失败的薪酬命令，不修改既有 Person，也不生成空账户档案。

## 验证

- `node --check`：主脚本和内部管理脚本通过。
- Workbench Go 全模块测试与 vet 通过，`git diff --check` 通过。
- 相关 Node 13 项回归通过，新增用例固定手动重定向接受规则、禁止跟随应用根页面、工资账号可选及精确阶段提示。
- 公网资源核对包含 `redirect:'manual'`、`if(!credentialRedirect(response)&&!response.ok)`、`if(accountNumber)`、薪酬档案和工资账户档案阶段。

## D1 发布

- 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.50@sha256:c990777ce60cc7cd94477de3eea4234ddc18ed06a37c0524a4eb445d5362edda`。
- 发布前备份：`/var/backups/omsbizfoundry/20261028T123049Z`。
- 候选 IAM 契约预检返回 `contracts-valid`；只对 Unified Workbench 执行 `--no-deps --force-recreate`。
- 发布后 D1 `check` exit 0，19/19 正式容器 healthy，Workbench 日志正常监听。

## 浏览器验收与剩余动作

用户已有 Chrome 会话强制刷新后，成员列表显示“员工1”且薪酬仍为“未配置”；编辑表单中“新工资账号”为空并明确提示“不填写表示不变”。验收随后取消表单，业务写入为 0。

修复不会猜测并重放此前表单内容。用户确认固定工资、社保基数和公积金基数后，可从“编辑成员”显式保存；账号留空时只建立 Compensation Profile，不建立 Payment Profile。
