# M8 People & Groups 最小纵向闭环规格

日期：2026-09-21  
状态：已实现并验证

## 1. 目标与范围

本规格实现 [M8 行动方案](../../m8-people-groups-action-plan.md) 的完整 v1 纵向闭环。应用 ID 与 IAM Audience 固定为 `people-groups`，统一组合路由为 `/people`，独立应用默认监听 `:8084`。

## 2. 领域不变量

1. 所有业务事实都归属一个 `tenant_id`，请求 Actor 的租户必须与资源租户一致；
2. Person、Group、Membership 和 Assignment 使用服务生成 UUID；配置 key 使用小写稳定标识；
3. Person 的可选 `reference_code` 与 `iam_subject_id` 在租户内唯一；IAM Subject 只是外部引用，没有跨库外键；
4. Group Type、Membership Role 与 Capability Definition 必须处于 active 才能用于新事实；停用不改变旧历史；
5. 同一 Person 在同一 Group 同时最多有一个 active Membership；ended 后可以建立新的 Membership 周期；
6. 角色变化只追加 Membership revision；结束日期不得早于开始日期，ended Membership 不能再变化；
7. Group 的人员或能力变化都在同一事务中递增 Group revision；过期 `expected_revision` 返回冲突；
8. archived Group 禁止更新、加入／调整成员、配置能力或发布新的当前快照；
9. 同一 Capability 在同一 Group 同时最多有一个 active Assignment；结束后可以重新声明；
10. Roster Snapshot 在 `(tenant_id, group_id, group_revision)` 上唯一，保存不可变原始 JSON 和 SHA-256；
11. 成功写入、领域 revision 和审计必须在同一 PostgreSQL 事务提交；审计及历史 revision 由数据库 Trigger 禁止更新和删除；
12. `Idempotency-Key` 在租户与操作范围内唯一；同命令同负载返回既有结果，不同负载拒绝。

## 3. 接口

读取：

- `GET /api/v1/catalog`
- `GET /api/v1/persons`
- `GET /api/v1/persons/{person_id}`
- `GET /api/v1/groups`
- `GET /api/v1/groups/{group_id}`
- `GET /api/v1/groups/{group_id}/snapshots/{group_revision}`

写入：

- `POST /api/v1/persons`
- `POST /api/v1/persons/{person_id}/profile`
- `POST /api/v1/persons/{person_id}/status`
- `POST /api/v1/persons/{person_id}/iam-subject`
- `POST /api/v1/definitions/{kind}`
- `POST /api/v1/definitions/{kind}/{key}`
- `POST /api/v1/groups`
- `POST /api/v1/groups/{group_id}/profile`
- `POST /api/v1/groups/{group_id}/archive`
- `POST /api/v1/groups/{group_id}/memberships`
- `POST /api/v1/groups/{group_id}/memberships/{membership_id}/role`
- `POST /api/v1/groups/{group_id}/memberships/{membership_id}/end`
- `POST /api/v1/groups/{group_id}/capabilities`
- `POST /api/v1/groups/{group_id}/capabilities/{assignment_id}/end`
- `POST /api/v1/groups/{group_id}/snapshots`

全部写入返回资源当前 revision；验证失败为 400，未认证为 401，能力不足为 403，不存在为 404，重复业务键、过期 revision、命令负载冲突和非法状态为 409。

## 4. 身份与授权

正式模式只接受 IAM 签发的 Ed25519 单 Audience 短期凭证，并复验 `issuer`、`tenant`、`audience=people-groups`、action、tenant scope、时间和 key 状态。本地模板模式需要显式配置租户、Subject、显示名和高熵访问令牌；访问令牌只存在配置与 HttpOnly Cookie，不进入业务表或审计 details。自动测试使用同一 `Principal` 端口。

任何页面隐藏都不代替服务端 Capability 判断。Membership Role 和 Group Capability 不参与 IAM 授权计算。

## 5. 快照格式

Roster Snapshot v1 固定包含：

- `schema_version`、`snapshot_id`、`source_application`、`tenant_id`、`generated_at`；
- Group 的 ID、revision、type key、name、purpose、status；
- 按 Membership ID 排序的当前 active member：Membership ID/revision/role/start date 与 PersonRef ID/revision/display name/reference code；
- 按 Assignment ID 排序的当前 active capability：Assignment ID/revision/key/note/start date；
- 发布 Actor 的类型、ID 与显示名。

摘要为上述紧凑 JSON 原始字节的 SHA-256 小写十六进制。快照只陈述来源事实，不签发 IAM 权限，不保证 Person、Group 或 Capability 在未来仍处于当前状态。

## 6. 数据与运行边界

- 独立 PostgreSQL 数据库、账号、migration 和备份恢复边界；
- 不引用其他应用 Go module、数据库表或 ORM；
- `backend/services/people-groups` 脱离根 `go.work` 后仍可测试与构建；
- `MIGRATION_MODE=auto` 供独立开发，`verify` 供 migration job 后的正式启动；
- `/health/live` 不访问数据库，`/health/ready` 同时校验连接与 current schema；
- D1 通过 Suite Manifest 开关进程、路由和菜单，关闭不删除数据库；
- D1 联合备份包含 People & Groups PostgreSQL dump，恢复仅允许全新空卷并继续沿用旧 Session 失效规则。

## 7. 验收矩阵

- 正常：配置三类定义、建立两人两组、一人加入多组、角色变化、退出／重入、能力配置、快照发布、归档；
- 历史：旧 Membership revision、旧 Assignment revision、旧 Group revision、旧审计和旧快照保持可读且不可改；
- 并发：相同 expected revision 只有一个命令成功，另一个稳定返回 conflict；
- 幂等：相同 key/负载不新增记录，相同 key/不同负载拒绝；
- 安全：无凭证、错误 action、tenant、audience、签名、scope、过期凭证均拒绝；
- 边界：IAM Subject 不经跨库查询；数据库中不存在密码、Session 或 IAM role；
- 装配：D1 单次登录访问 People、Contract、Money，关闭 People 后其余应用继续，重新启用数据仍在；
- 恢复：销毁 D1 卷后从联合备份恢复，快照摘要可复算且可继续新增 Person 与 Membership。

## 8. 实现与验证结果

- 独立应用位于 `backend/services/people-groups`，默认端口 `8084`，独立开发数据库默认宿主端口 `55435`；
- Capability Manifest 位于 `contracts/access/v1/examples/people-groups-capabilities.json`，IAM、Gateway、Workbench 与 D1 均从同一 Suite Manifest／能力目录装配；
- migration `000001_people_groups.sql` 建立租户隔离、当前事实、不可变 revision、不可变审计、幂等命令和 Roster Snapshot；数据库 Trigger 阻止历史更新与删除；
- 独立测试、vet、构建、真实 PostgreSQL 集成测试、正式 IAM 令牌测试、HTTP 授权负例和实际 D1 API／页面链路均通过；
- D1 已验证启用／禁用、路由与菜单同步、People 单侧停机不影响 Contract／Money、独立数据库 current schema、联合备份与销毁卷后的恢复；
- 恢复后的原 Person、Group、Membership 和 Snapshot 可读，旧快照保持原始字节与摘要，服务可继续接收新命令。

实际命令与操作边界见 [`docs/testing.md`](../../testing.md) 和 [`docs/operations/d1-compact.md`](../../operations/d1-compact.md)。
