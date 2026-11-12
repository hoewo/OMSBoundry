# OMS 内部管理操作人与批量发放入口发布记录（2026-10-28）

## 结论

成员管理和工资管理的操作人显示错误，以及批量“确认已发放”无解释禁用，已完成修复并发布。

## 根因

- 成员页丢弃了 People 已保存的审计主体，直接写死“Owner 服务”。
- 工资页保留了 Payroll 的 `created_by_subject_id`，但只显示 UUID 前 8 位，没有把当前可信 Subject 映射为服务端已验证的 IAM 显示名。
- 单条“发放”允许缺账户记录进入后再提示，批量按钮却把 `payment_account_missing` 直接作为禁用条件，因此相同事实出现两套交互。

数据库只读核对确认当前两名员工的最新 Person revision 均由 Subject `15f6c50c-629b-4a6b-946f-7577aee7a33b` 操作，审计显示名为“戴鹏飞”。

## 修复

- People Person 列表在不增加 schema 的前提下，从相同 Person／revision 的不可变 `person_revisions` 只读返回 `last_actor_subject_id` 和 `last_actor_display_name`。
- 成员页优先展示 Owner 返回的审计姓名；仅在旧数据缺姓名且 Subject 等于当前会话时使用服务端渲染的当前 IAM 姓名。
- 工资页在工资表创建 Subject 等于当前登录 Subject 时显示当前 IAM 姓名；其他主体仍显示有界账号标识，避免猜测姓名。
- 批量按钮对待发放／部分发放记录启用。点击后若存在缺工资账户的成员，显示成员姓名以及“成员管理补账户 → 工资详情同步工资账户”的路径，并在发起任何发放命令前停止。

## 验证与发布

- `node --check`：两个修改脚本通过。
- 相关 Node 回归：10/10 通过。
- `go test ./backend/services/people-groups/... ./frontend/oms-workbench/internal/server`：通过。
- `go vet ./backend/services/people-groups/... ./frontend/oms-workbench/internal/server`：通过。
- `git diff --check`：通过。
- 发布前候选预检通过，联合备份：`20261028T125445Z`。
- People & Groups：`v0.1.0-test.oms-v1.52@sha256:6276c50cb243d015c1b6f9464ba4430835aa94f8cac88d7dd6b507b79b6d7ede`。
- Unified Workbench：`v0.1.0-test.oms-v1.52@sha256:caa06d4bd1933b3b23c7ee8a89eb4902bb8c253d6a9486173d9d67775612373c`。
- D1 全栈检查通过，19/19 长运行容器 healthy。
- 公网 `oms-phase1-internal.js?v=20261028.2` 与 `oms-phase1-payroll-owner.js?v=20261028.3` 的 SHA-256 分别与本地文件完全一致。
- 数据库审计复读仍为“戴鹏飞”；没有 migration、审计回填、工资账户补写或实际发放。

## 边界

People 仍拥有人员与 revision 审计，Payroll 仍拥有工资表、账户快照与发放命令；Workbench 只组合显示。工资账号对建薪酬档案和建工资表仍是可选项，但确认实际发放前仍是必需项。本次没有放宽 Payroll 最终校验，也没有代用户确认工资已发放。
