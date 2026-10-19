# Party Directory 技术方案

状态：🟡 已采用；独立核心、可操作页面、release、非 root 镜像、D1 正式链、联合恢复，以及 Contract／Money 预检、来源自有映射、含引用数据运行时切换／只读回滚和 Contract-linked C6 恢复已实现；Money 完整 D1 消费链与完整双 Tenant 待完成

日期：2026-09-27

## 方案目标

Party Directory 使用独立 Go 服务和独立 PostgreSQL 数据库拥有当前主体档案、稳定 Party ID 与不可变 revision。消费应用只通过版本化 HTTP Port 读取精确主体引用，并在自己的事务中保存业务角色和主体快照。

## 运行边界

```text
Browser
  → Nginx
  → Gateway
  → IAM / Policy
  → Party Directory
      → party database

Consumer Application
  → workload assertion
  → IAM credential exchange
  → exact Party reference credential
  → Party Directory reference API
  → consumer-owned immutable snapshot
```

- Nginx 和 Gateway 不解释主体字段、状态或 revision。
- IAM 签发 Party Directory 原生 Capability Credential，不拥有主体档案。
- Party Directory 作 Tenant、Action、Scope、状态和 revision 的最终授权者。
- 消费应用不读取 Party 数据库，不导入 Party ORM／领域对象，不把 Party 当前档案缓存为第二个可写目录。
- 所有服务端点由配置注入；Party-only 和 D1～D4 使用同一镜像与契约。

## 数据模型

### 当前投影 `parties`

`parties` 保存每个 Tenant 下的稳定 Party ID、当前 kind、legal name、registration identifier、status、revision 与更新时间。当前投影可更新，但只能由同一事务追加下一 revision 后推进。

主要约束：

- `(tenant_id, id)` 唯一；
- kind 只允许 `organization`、`individual`；
- status 只允许 `active`、`inactive`；
- revision 从 1 开始单调递增；
- 名称和登记标识遵守规范化长度，但不声明登记标识唯一；
- 不提供物理删除业务命令。

### 不可变 `party_revisions`

`party_revisions` 以 `(tenant_id, party_id, revision)` 为主键，保存每次完整档案快照、事件类型、原因、Command ID、可信 Actor 与发生时间。数据库触发器拒绝 UPDATE／DELETE。

当前投影的字段、状态和 revision 必须与最新 revision 完全一致。创建、更正或状态命令在一个 PostgreSQL 事务内锁定 Party、校验 Expected Revision、追加 revision、推进当前投影并写审计。

### 命令与审计

每类写命令保存 Tenant、Command ID、负载摘要、Actor、Party ID、来源／结果 revision 和结果。Tenant 与 Command ID 构成幂等范围；同键同摘要返回原结果，同键不同摘要拒绝。

审计是追加事实并受数据库不可变保护。业务 revision 用于解释主体事实，技术审计用于解释谁以何种凭证执行了命令，两者不能互相替代。

## API Port

| 契约 | 用途 | Capability |
| --- | --- | --- |
| `party-create.yaml` | 登记 revision 1 | `party-directory.create` |
| `party-list.yaml` | 查询和稳定分页 | `party-directory.read` |
| `party-get.yaml` | 读取当前详情与动作 | `party-directory.read` |
| `party-update.yaml` | Expected Revision 更正 | `party-directory.update` |
| `party-status-change.yaml` | 停用／重新启用 | `party-directory.status.manage` |
| `party-history-list.yaml` | 读取不可变 revision | `party-directory.history.read` |
| `party-reference-get.yaml` | 消费应用精确读取 | `party-directory.reference.read` |

浏览器 API 可以使用 Tenant Scope 或精确 Party Scope；工作负载引用 API 只接受精确 `party:{id}` Scope。所有写入都要求单一 Party 或 Tenant 创建范围、Command ID 和服务端可信 Actor。

列表响应除 `items` 和 `next_cursor` 外还返回当前用户可用于页面呈现的 `allowed_actions`；第一版列表动作只包含 `party-directory.read` 与可选的 `party-directory.create`。详情继续按目标 Party Scope 与状态返回短动作 `read`、`update`、`status_manage`、`history_read`。这些字段只控制页面显示，写端点仍完整复验 Credential、Tenant、Scope、Expected Revision 和状态。

## 人类页面与消费者选择

- Party 根路径提供独立管理工作区，浏览器脚本只调用 Party 的公开 v1 API；不存在跨库读取、共享 ORM 或写入消费应用的逻辑。
- 正式页面通过 Gateway `/gateway/authorize/party-directory` 使用当前 IAM Session 换取 Party 用户凭证。HttpOnly Credential 不暴露给脚本；401／403 后页面退出编辑态。
- Project 新建页需要候选时，先取得 Party 用户凭证并读取 active 列表。该链只负责发现，不是 Project 创建的可信验证。
- Project 创建命令只携带 Party ID 与 source revision。Project 服务使用自身工作负载身份交换精确 `party:{id}` Credential，再调用 reference API 复验 Tenant、状态和 revision。
- Party 列表无权、为空或 503 时，Project 页面保留“不选择客户主体”，因此 Party 不是 Base Project 的隐式启动前提。
- Project 详情读取本地冻结快照，不在线调用 Party；首个切片没有客户改绑或解绑端点。

## 并发与错误语义

- 创建按 Tenant + Command ID 取得事务级互斥，不按名称自动锁定或合并。
- 更正和状态变化锁定当前 Party，并要求 Expected Revision 与当前 revision 相同。
- stale revision 返回 `409 PARTY_REVISION_CONFLICT` 和最新 Party 投影，不写 revision、命令成功或业务审计。
- 幂等键负载变化返回 `409 COMMAND_PAYLOAD_CONFLICT`。
- 无实际变化返回 `422 NO_EFFECTIVE_CHANGE`，不消耗 revision。
- 数据库或依赖故障返回 503，不把错误读取渲染为空列表，不返回虚假成功。

## IAM 与目标最终授权

Capability Manifest 的 audience 固定为 `party-directory`。用户操作使用最多 300 秒的单 Audience Credential；消费应用使用工作负载身份交换最多 60 秒、单 action、精确 Party Scope Credential。

Party Directory 复验 issuer、签名、tenant、audience、action、scope、iat、exp 与 credential id，并在本地判断：

- Tenant 是否匹配 Party；
- Expected Revision 是否仍为当前值；
- inactive Party 是否允许当前动作；
- 读取是否精确落在凭证资源范围内。

Gateway 清除客户端身份 Header。客户端提供的 Tenant、Actor 或 allowed actions 都不作为授权输入。

## 快照协作

Party reference response 是版本化传输模型，不是共享领域对象。消费应用确认业务事实时复制：

- Party ID；
- Party revision；
- kind；
- legal name；
- registration identifier；
- 本应用拥有的业务角色。

消费应用必须在自己的业务事务中保存快照。Party 更名、停用或服务不可用不会改变已确认快照。草稿页面可以重新读取当前档案，但不能在后台静默刷新已确认历史。

## 故障与恢复

- Party 不可用时，消费应用从自身数据库读取既有快照；需要建立新引用或显式刷新草稿的命令返回依赖不可用。
- 已获取但未过期的精确 Credential 不能绕过 Party 服务状态；目标 API 仍以本地 Party 状态作最终判断。
- 恢复后调用方使用新 Command ID 明确重试；Party Directory 和消费应用分别保持本地幂等，禁止分布式双写事务。
- Party 自身备份包含当前档案、revision、命令幂等和审计，不包含 IAM Grant、账号、Session 或密钥；IAM 作为独立数据所有者必须由部署组合单独备份。空目标恢复后 ID、revision、表集合与逐表行数保持一致，并能从当前 revision 继续写入。

## Suite Manifest 与部署

Party Directory 作为独立应用条目声明 image、migration、health、Gateway route、Capability Manifest 和数据库。Party-only 组合只包含入口、IAM、Workbench、Party 与数据库；消费组合通过启用 Party 条目、注入版本化 Party API 端点并加载两端契约／策略表达显式依赖，不使用固定主机名、IP 或 localhost。

禁用 Party 路由不会删除数据。已依赖 Party 的消费应用可以继续展示既有快照，但新引用命令必须显示依赖不可用，不能自动切回本地可写目录。

## 本地目录迁移

Contract 与 Money 的迁移使用各自拥有的兼容映射，不改变既有 Party ID 或历史快照：

1. 导出本地 Party 当前事实与稳定摘要；
2. 对每条来源记录显式选择新建 Party 或映射到已确认 Party；
3. 以来源应用、来源 Party ID 和迁移批次作为幂等依据；
4. 在消费应用保存 `local party → directory party + accepted directory revision` 映射和审计；
5. 切换点之前的新引用由本地目录负责，之后的新引用由 Party Directory 负责；
6. 不唯一或冲突映射进入人工清单，迁移不能按名称自动合并；
7. 已形成的合同、义务、资金和结算快照不回写新 Party ID。

迁移不是运行时双写。切换后的回滚保留 Party Directory 和已形成映射；不能通过重新启用本地写入制造两个当前真相。

2026-09-27 已完成第 1 步的可执行预检：Contract 与 Money 的 ops 二进制分别从本应用数据库生成版本化 JSON，固定来源 ID／revision、规范化事实 SHA-256、引用计数、重复登记标识信号和允许的人工决策类型。预检从不写库，始终禁止自动合并并保持 `ready_for_cutover=false`；首轮 D1 样本两侧各为 5 条，映射专项创建验证样本后各为 6 条，均为 0 条业务引用。

随后完成第 2～4 步的机制与 D1 首轮真实审核：两个来源应用各自新增只追加映射表和 `party-migration-map`，并装配独立 Ed25519 工作负载密钥。请求按 Tenant、批次、Command、来源摘要、目标 revision、审核人和理由做稳定摘要；同 Command 同摘要重放，不同摘要冲突。新决定先用来源应用工作负载私钥向 IAM 交换 `party-directory.reference.read`、精确 `party:{id}` Scope 的短期 Credential，并要求 reference 返回 schema 1、同 Tenant、active、精确 revision。之后来源事务用 `FOR UPDATE` 锁定本地 Party、重算预检摘要并追加目标最小快照；数据库 Trigger 禁止更新／删除。D1 两端各 1 条人工映射、重放／冲突／stale、Party 停机恢复、IAM 审计和直接 UPDATE 拒绝已通过。目标验证和来源写入不是分布式事务，因此目标变化由精确 accepted revision 保持可解释，切换前还必须重新执行门禁。

2026-09-28 完成第 5～7 步的运行时机制与首个正式组合实证：Contract migration `000019_party_runtime_transitions.sql`、Money migration `000018_party_runtime_transitions.sql` 追加租户级 `cutover／rollback` 转换账；`cutover` 在事务外逐条复验目标 Directory Party，在事务内再次锁定转换状态、映射批次与全部已引用来源 Party，批次摘要漂移、目标 stale 或任何引用缺映射都失败关闭。`rollback` 只进入 `local_compatibility` 读模式；`directory_authority_established` 一旦为真，本地 Party 写入口永久返回 409，映射和已形成快照不删除。两个台账含引用数据的 PostgreSQL integration 已通过；C6 D1 进一步验证 Contract 的 `cutover 1 → rollback 2 → recutover 3`、切换前／回滚后 422、Directory 模式下 Contract-linked Engagement 201，以及联合备份和空目标恢复保持映射、序列、ID、revision 与历史。正式 Contract reference 从当前 Contract revision 的不可变 Party Snapshots 派生 Directory IDs，草稿才读取 active draft Party。Money 的完整 D1 消费链和完整双 Tenant 矩阵仍待补证。

## 安全与隐私边界

- 日志和审计不记录 Credential、Session、Cookie 或私钥原文。
- registration identifier 按普通业务标识处理，但 API、日志和导出仍按最小权限暴露；证件敏感分级不在第一版内。
- Party 不保存密码、人员任职、联系人通信记录、银行账户、税务申报或证照原件。
- Tenant 是所有查询和约束的第一过滤条件；跨 Tenant ID 碰撞也不能读取。

## 实现验收

### 2026-09-27 已取得证据

- `backend/services/party-directory` 可以在根工作区和 `GOWORK=off` 独立模块两种模式下完成 test、vet 与 build；
- PostgreSQL 18 integration race 覆盖创建、更正、停用、历史、跨租户、并发幂等、失败无残留、revision 不可变和 current／latest 一致性；
- HTTP／IAM 自动化覆盖受保护路由、可信 Actor、身份 Header 清理、Audience／Action／Tenant／Scope 与精确 Party reference；
- 独立进程完成 `ready → create → replay → update → stale 409 → inactive → search → history` 的 API 旅程；
- 第一轮 Docker Compose 独立数据库启动曾被本机 Docker 地址池耗尽阻止；交付轮改用既有用户定义网络上的隔离 PostgreSQL 18 tmpfs 容器，未修改任何既有业务容器或数据库。
- `v0.1.0-acceptance.1` 开发验收包通过 SHA-256、版本／commit、固定三二进制、7 份 API 契约和 2 份模型随包检查；非 root 镜像同时携带 PostgreSQL 18 `pg_dump`／`pg_restore`。
- 真实容器旅程完成 Party revision 1 创建、revision 2 更正、revision 3 停用、同快照全量备份、空数据库恢复、向非空目标二次恢复拒绝、ID／当前事实／3 条历史保持，以及恢复后 revision 4 重新启用。
- Party 与 Project 已装入只由 Nginx 发布宿主端口的 D1；Project 使用工作负载身份向 IAM 交换单 action、精确 `party:{id}` Scope 凭证，Party 记录目标最终授权结果。
- 破坏性 D1 验收已证明 Party 停机时新引用无部分写入、既有 Project 快照仍可读、无 Party 的 Base Project 创建仍成功，Party 恢复后新命令只写一次。
- D1 联合备份 `20260927T025800Z` 在空目标恢复后保持 2 个 Party、4 个 Project 与 3 个 Party 快照关系，完整非破坏性验收再次通过。
- Party 管理页、列表 `allowed_actions`、Workbench 入口与 Project 可选客户选择已装入当前 D1；非破坏性和破坏性 `acceptance/d1-delivery/verify-running` 均通过页面资源、active 候选、精确快照、Base 可选性、故障失败关闭和恢复后单次重试检查。

核心证据见 `docs/reports/oa-party-directory-core-experiment-20260927.md`；交付恢复证据见 `docs/reports/oa-party-directory-delivery-recovery-experiment-20260927.md`；D1 正式链与联合恢复证据见 `docs/reports/oa-party-project-d1-chain-experiment-20260927.md`；人类管理与选择证据见 `docs/reports/oa-party-project-human-workflow-experiment-20260927.md`。

实现只有同时通过以下证据才从“目标方案”转为运行事实：

- 领域单元测试、竞态测试和 PostgreSQL migration／约束／触发器测试；
- API 正常、幂等、冲突、权限、跨 Tenant 和精确 Scope 契约测试；
- Party-only 独立启动、release、非 root 容器、备份与空目标恢复（已通过）；
- D1 正式 API 入口的精确工作负载授权、目标最终授权、故障隔离和联合恢复（已通过）；
- Project 消费应用的引用、快照不变、Party 停机与恢复测试（已通过）；
- Contract 与 Money 本地目录的确定性只读预检、来源自有映射和含引用数据运行时切换／回滚机制已通过单元、HTTP 与 PostgreSQL 18.6；Contract-linked C6 的正式 D1、故障恢复、联合备份／空目标恢复与选装复验也已通过。Money 的完整 D1 消费链和完整双 Tenant 矩阵仍待完成。
