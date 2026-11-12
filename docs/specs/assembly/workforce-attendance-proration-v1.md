# 考勤、请假与月中入离职折算 v1

## 问题与结果

考勤、请假和月中入离职只能靠工资行手工调账 → 生成一份带人员快照、规则 revision 和精确日单位的月度出勤摘要，由 Payroll 显式消费并以整数最小货币单位折算固定工资。

## 事实与边界

- People & Groups 继续拥有 Person、在职状态、入职日、离职日及其 revision。
- Workforce Attendance 独立拥有考勤记录、请假记录、规则和月度出勤摘要。
- Payroll 继续唯一拥有固定工资、工资行、最终应发／实发金额和发放事实。
- Attendance 不保存工资金额，不直接写工资表；Payroll 不反向改写考勤、请假或入离职事实。
- 月度摘要是版本化输入建议，Payroll 必须验证 schema、人员快照、规则摘要和单位等式后再形成工资调整。

## 精确单位和规则

一个完整日使用 `10000` 个整数单位。摘要至少包含：

- 标准单位 `standard_units`；
- 入离职范围内可计薪单位 `employment_eligible_units`；
- 无薪请假扣减 `unpaid_leave_deduction_units`；
- 缺勤扣减 `absence_deduction_units`；
- 可计薪单位 `payable_units`。

必须满足：

`payable = employment eligible - unpaid leave - absence`

首版支持自然日和显式排班工作日两种基数。排班工作日必须传入当月明确日期，不允许公共核心偷偷假定周一至周五。请假支持部分日和整数基点的带薪比例；同一日期的请假与缺勤合计不得超过一个完整日。重复记录 ID 必须拒绝。

People 入离职日期只截取当月可计薪范围，不改变标准基数。Payroll 使用半入制和大整数中间值计算：

`prorated fixed salary = fixed salary × payable units ÷ standard units`

所有金额使用整数最小货币单位，禁止二进制浮点数。

## 当前完成与明确未完成

当前已完成版本化月度摘要契约、独立 PostgreSQL Repository／migration、HTTP／IAM Capability、People 多任职区间快照连接器、自然日／显式工作日政策、月中入离职截断、部分日请假、带薪比例、缺勤、重复记录拒绝、Workbench 规则／缺勤／请假／摘要页面、Payroll 摘要复验与整数半入折算、工资 revision、D1 装配和备份恢复。公网 Workforce Attendance／Payroll `v0.1.0-test.oms-v1.76` 与 Workbench `v0.1.0-test.oms-v1.77` 已运行。

系统不会在读取摘要时自动改工资；经办人必须在摘要行点击“应用到工资”，Payroll 才追加工资 revision。后续仍需按交付地区配置和验证权威节假日／调休／计薪政策 Customer Overlay，并以真实月结样本验收；公共核心不内置未经确认的法规口径。

[发布验证](../../reports/oms-invoice-recognition-attendance-proration-publication-20261106.md)
