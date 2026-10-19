# M8 行动方案：People & Groups 最小闭环

日期：2026-09-21  
状态：已完成并验证

## 1. 最小根因

> 人员和团队关系散落、无法被其他应用稳定引用 → 独立管理 Person、Group 与 Membership，并发布可校验的名册快照。

这里的 `Membership` 只表示“某个 Person 在某个 Group 中，于什么时间以什么组内角色参与”。它不是 IAM 权限、劳动关系、项目任务或付款依据。

## 2. 本阶段完成后的可见结果

M8 完成时，用户能够从独立应用或统一工作台完成：

```text
配置 Group Type、Membership Role、Capability
  → 建立 Person
  → 建立 Group 并填写 Purpose
  → 加入成员并指定组内角色
  → 调整角色或结束 Membership，历史仍可读取
  → 给 Group 声明可检索 Capability
  → 发布带 revision 和 SHA-256 的不可变 Roster Snapshot
  → 归档 Group，既有历史与快照保持不变
```

## 3. 固定边界

- People & Groups 拥有 Person、Group、Membership、配置定义、组能力和 Roster Snapshot；
- IAM 拥有账号、密码、登录、会话和业务 Capability 授权；Person 只保存可选 IAM Subject UUID 引用；
- Membership Role 是组内分工，不自动授予任何系统权限；
- Group Capability 是可配置画像，不触发合同、项目、预算、工资或付款规则；
- 其他应用只使用版本化 JSON API、`PersonRef`、`GroupRef` 或 Roster Snapshot，不读取本应用数据库；
- Person 可以属于多个 Group；同一 Person 在同一 Group 同时最多有一条有效 Membership，但结束后允许重新加入；
- Group 归档后禁止继续改变当前名册或能力，但不得删除历史；
- 正式 IAM、显式本地模板和自动测试提供者产生同形可信 Actor，不能关闭鉴权；
- 全部写操作必须校验目标应用原生 Capability、租户、期望 revision、Command ID 和领域状态。

## 4. v1 数据与接口

### Person

最小字段为稳定 UUID、可选业务引用码、显示名、`active/inactive` 状态、可选 IAM Subject UUID 和 revision。不保存身份证号、银行卡、住址、劳动合同或完整 HR 档案。

### Group

最小字段为稳定 UUID、Group Type key、名称、Purpose、`active/archived` 状态和单调 revision。客户可以把某个 Group Type 显示为“施工队”，内核不增加施工专用字段。

### Membership

一条加入周期拥有独立 UUID、Person、Group、Membership Role key、开始日期、可选结束日期、状态和 revision。加入、角色变化和结束都追加不可变 revision；不直接覆盖历史行。

### 配置与组能力

`group_type`、`membership_role` 和 `capability` 使用租户内稳定 key。显示名、说明和启停可以通过 revision 演进；已经进入历史 revision 或 Roster Snapshot 的值不被当前配置改写。Group Capability 有自己的加入、说明调整和结束历史。

### Roster Snapshot

显式发布时冻结当前 Group、有效 Membership、PersonRef 和有效 Group Capability，按 Group revision 唯一保存原始 JSON 字节及 SHA-256。相同 revision 不重复发布；后续成员变化产生新 Group revision，不改写旧快照。

公开接口统一位于 `/api/v1`，使用 JSON、稳定错误码、`Idempotency-Key` 和 `expected_revision`。服务端页面只是同一应用服务的操作入口，不形成第二套业务规则。

## 5. 稳定业务 Capability

| Capability | 用途 |
| --- | --- |
| `people-groups.view` | 查看 Person、Group、Membership、配置和快照 |
| `people-groups.person.manage` | 建立或维护最小 Person 与 IAM Subject 引用 |
| `people-groups.configuration.manage` | 建立和维护 Group Type、Membership Role 与 Capability Definition |
| `people-groups.group.manage` | 建立、维护和归档 Group |
| `people-groups.membership.manage` | 加入、调整和结束 Membership |
| `people-groups.capability.manage` | 声明或结束 Group Capability |
| `people-groups.snapshot.publish` | 发布不可变 Roster Snapshot |

客户岗位、菜单、页面和按钮名称不得成为 Capability。

## 6. 实施顺序

| 顺序 | 事项 | 退出证据 |
| --- | --- | --- |
| M8-001 | 规格、Capability Manifest 与数据库不变量 | 已完成：Schema、Capability Manifest、migration 与数据库负例可执行 |
| M8-002 | Person、配置与 Group | 已完成：真实 PostgreSQL 已验证创建、唯一性、revision、租户和审计 |
| M8-003 | Membership 与 Group Capability 历史 | 已完成：多组成员、角色变化、退出、重新加入、并发冲突和归档拒绝通过 |
| M8-004 | Roster Snapshot 与公开 API／最小页面 | 已完成：旧快照不变、摘要可复算，API 与页面共用服务端授权 |
| M8-005 | IAM、Gateway、Workbench 与 D1 装配 | 已完成：单次登录、应用开关、单侧停机、独立数据库和联合备份恢复通过 |
| M8-006 | 全仓回归与交付审计 | 已完成：独立 `GOWORK=off` 测试／vet／构建、全仓检查和运行验收通过 |

## 7. 明确不做

- 项目 Assignment、排班、工时、考勤、现场管理；
- 工资、奖金、社保、报销、应付款和付款批次；
- 合同范围、预算与成本控制；
- 招聘、绩效、完整员工档案和劳动关系合规；
- 组织树、Group 嵌套、角色自动继承、能力等级和证书；
- 动态字段设计器、通用工作流或标签触发器；
- Project、Contract 或 Money Flow 的连接器实现。

## 8. 总退出门槛

- 独立空库能够完成完整可见结果，并在不启动 Contract、Money、Project 或 Evidence 时继续工作；
- 正式 IAM 和本地模板模式都不能绕过可信 Actor，错误 audience、action、tenant、签名和过期凭证失败关闭；
- Membership 与 Group Capability 的旧 revision、审计和已发布快照不可被更新或删除；
- Person 多组、同组单一当前 Membership、结束后重新加入和 Group 归档规则有真实 PostgreSQL 证据；
- D1 只公开 Nginx，People & Groups 使用独立数据库／账号／migration，关闭应用不删除数据；
- People & Groups 停止或禁用时，IAM、Contract 和 Money 仍能按既有边界工作；
- 联合备份在全新空卷恢复后，IAM 登录、原 Person／Group／Membership／快照和继续写入均通过；
- 代码和配置中不存在项目任务、合同范围、预算、工资、付款或客户施工专用字段。

## 9. 完成证据

- `backend/services/people-groups` 已形成可脱离根 workspace 构建的独立 Go 应用、独立 PostgreSQL migration、正式 IAM Provider、显式本地模板 Provider、JSON API 与服务端页面；
- 真实 PostgreSQL 集成测试覆盖一人多组、角色变化、结束后重入、组能力、乐观并发、幂等命令、历史不可变、Person revision 传播与旧快照字节不变；
- IAM 单元测试覆盖正确凭证、错误 audience 和缺少入口 action；HTTP 单元测试覆盖无凭证 `401`、缺少具体写 action `403` 与无业务凭证健康检查；
- D1 只经 Nginx 暴露，在一次 IAM 登录后完成 Person、Group、Membership、Capability 与 Roster Snapshot 写入；停止 People & Groups 时 Contract 与 Money 继续工作；
- `ENABLE_PEOPLE=false` 时进程停止、路由 `404`、Workbench 入口消失，重新启用后原数据库事实仍在；
- 联合备份包含 `people-groups.dump` 与外层 SHA-256；销毁 `omsd1m7006` 测试栈数据卷后恢复出 `3 Person / 3 Group / 3 Membership / 3 Snapshot`，四套数据库 current-schema 检查和继续写入通过；
- 恢复演练发现并修复了新建 IAM 公钥卷的属主初始化缺陷，当前由一次性 `public-keys-permissions` 任务先收紧目录属主，再以非 root 身份发布公钥。
