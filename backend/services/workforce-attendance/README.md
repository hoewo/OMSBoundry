# Workforce Attendance

问题 A → 可验证结果 B：考勤、请假和月中入离职目前只能靠工资行手工调账 → 形成一份带人员快照、规则 revision 和精确日单位的月度出勤摘要，由 Payroll 显式确认后折算固定工资。

本服务拥有出勤记录、请假记录、月度规则和月度摘要；People & Groups 继续拥有人与入离职事实，Payroll 继续拥有薪酬金额、工资行和最终发放。月度摘要只交付 `standard / employment eligible / leave deduction / absence deduction / payable` 的整数日单位，不直接写工资表。

当前切片实现日历日／显式排班工作日两种规则、月中入离职截断、部分天请假、带薪比例、旷工扣减和版本化摘要契约。HTTP、IAM、PostgreSQL、People 快照端口、Workbench 页面、D1 和恢复尚未实现，不能描述为已部署功能。
