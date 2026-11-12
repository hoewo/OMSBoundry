# OMS 工资批量输入可见标签本地验证

日期：2026-11-12

事项：BF-335

状态：本地实现与验证通过，尚未发布

## A → B

问题 A：工资批量录入每位成员并排显示两个只有数值的输入框，用户无法直接判断哪个是绩效、哪个是个人所得税。

可验证结果 B：每个输入框在原位置显示独立字段名和单位，并能由成员姓名与字段名唯一识别；预览和确认写入逻辑保持不变。

## 实现

- 成员行从包住两个控件的单个 `<label>` 改为普通语义容器。
- 两个输入分别使用独立 `<label>`，可见文本为“当月绩效（元）”和“个人所得税（元）”。
- `aria-label` 分别包含成员姓名和完整字段名，重复行中的同名字段可被辅助技术区分。
- 桌面端输入保持双列；760px 以下先显示成员信息，再显示两个带标签输入。
- Workbench CSS 与 Payroll Owner JavaScript 静态资源版本更新为 `20261112.2`，避免浏览器继续使用旧资源。

## 不变量

- `performance_minor` 与 `tax_minor` 的表单名和整数分转换不变。
- Payroll `imports:preview`、`imports:commit`、expected revision 和服务端重算不变。
- 不在浏览器计算个人所得税，不改变 Payroll API、schema、Capability 或工资事实。

## 实际验证

- `node --check internal/server/oms_phase1_payroll_owner.js`：通过。
- `node --test internal/server/oms_phase1_payroll_owner.test.mjs`：9/9 通过，新增可见 label、成员字段可访问名称和窄屏布局断言。
- `node --test internal/server/*.test.mjs`：308/308 通过。
- `GOWORK=off go test ./...`：通过。
- `GOWORK=off go vet ./...`：通过。
- `acceptance/p9-full-composition/test-preflight`：通过。
- `deployments/d1/ops/test-p9-composition`：通过。
- `git diff --check`：通过。

## 未外推

- 尚未构建或部署公网候选，未登记 `oatest` 已更新。
- 本地验证没有生成工资预览、确认写入或修改任何工资数据。
