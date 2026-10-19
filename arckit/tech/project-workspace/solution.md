# Project Workspace 技术方案

## 元信息

- 路径：`tech/project-workspace/solution.md`
- 技术领域：`project-workspace`
- 状态：✅ 已采用
- 负责人：BizFoundry
- 支持功能：`spec/project-workspace/project-initiation.md`
- 关联模型：`tech/_shared/models/Project.yaml`、`ProjectPackageConversion.yaml`、`ProjectAssignment.yaml`、`ProjectResourceLink.yaml`、`ProjectHistoryEvent.yaml`
- 关联 API：`tech/_shared/contracts/project-*.yaml`
- 变更历史：
  - 2026-09-25：固定 J1 的服务边界、事务、鉴权、外部引用和失败恢复。
  - 2026-09-25：实现独立运行时并通过 PostgreSQL 18、HTTP、引用故障、两种 IAM Provider 和 Docker 冒烟验证。
  - 2026-09-25：补齐 ops、PostgreSQL custom archive 备份恢复和带校验和的 Linux release，并在新空库完成恢复核对。
  - 2026-09-26：正式 D1 的 People 引用改用工作负载身份动态交换一分钟精确资源凭证，并通过无重启密钥轮换、IAM／People 故障与恢复验收。
  - 2026-09-26：Assignment 写入增加三项稳定角色能力和目标 role key 二次校验，C1 多角色／多 Project／撤权／状态矩阵通过。
  - 2026-09-26：增加两级 Project Package、attach／wrap 转换、不可变转换记录与安全撤销；数据库和服务共同拒绝第三层，转换不搬运其他应用事实。
  - 2026-09-27：列表／详情输出状态收敛的 `allowed_actions` 和 active Package Conversion；客户页面接入显式 IAM 权限刷新，Gateway 重新授权失败时清除旧应用凭证。

## 方案概述

Project Workspace 是独立 Go 服务和数据所有者。它只实现 Project、最多两级的 Project Package 关系、可撤销转换记录、Project Assignment、类型化 Resource Link、幂等命令和不可变历史；每个写动作在本应用 PostgreSQL 事务内完成。People、Contract、Money Flow、Evidence 和设计运营扩展通过版本化 HTTP 契约提供引用验证或当前投影，Project 不读取其数据库，也不把其业务事实缓存成第二真相源。

本地 Demo 使用 Project 的独立 HTTP 入口和 v1 模板 IAM Provider；正式组合继续使用 Nginx → Gateway → Project，并由 Project 再次验证 Capability Credential。两种入口共享同一应用层、领域规则、数据库、migration 和测试。

## 进程与依赖

```text
Browser / acceptance script
  → Project HTTP API
    → v1 Identity Provider port
      → local template provider | formal IAM provider
    → Project application service
      → Project domain
      → Project PostgreSQL repository
      → External Reference Validator ports
        → People v1 | Contract v1 | Money v1 | Evidence v1 | Design Operations v1
```

强制依赖只有本应用 PostgreSQL 与一个 v1 Identity Provider。外部 Reference Validator 只在创建 Assignment 或 Resource Link 的命令中按目标类型调用；普通 Project 读取和历史查询不依赖外部应用在线。

## 数据所有权与表

| 表 | 作用 | 可变性 |
| --- | --- | --- |
| `projects` | 当前 Project／Package 身份、两级父子关系、基础资料、可选客户 Party 创建快照、状态和 revision | 只由类型化命令更新；取消后冻结；Package 不得带客户快照 |
| `project_assignments` | Person／Group 项目角色及有效期 | 旧记录不改写；结束只写 `ended_at` 和结束 revision |
| `project_resource_links` | 外部类型化引用 | 旧记录不删除；解除只写 `unlinked_at` |
| `project_commands` | command ID、规范化请求摘要、结果引用 | 不更新、不删除 |
| `project_history_events` | 每个成功写命令的事件 | 不更新、不删除 |
| `project_package_conversions` | attach／wrap 的来源位置、双 revision、状态和撤销原因 | 创建后只允许从 active 追加撤销结果；不删除 |

每张业务表都包含 `tenant_id`。数据库不建立跨库外键，不保存外部服务连接名以外的部署地址。非空项目编号使用标准化值的租户级条件唯一索引；Package 可以暂不编号，普通／子 Project 必须编号。`node_type`、自引用外键、父节点类型触发器和服务层规则共同保证 Package 顶层且最多两级；负责人唯一性使用 active 条件唯一索引。

## 命令事务

所有命令遵守同一顺序：

1. 验证 v1 IAM Context 的 issuer、tenant、audience、action、scope 和时间；
2. 标准化请求并检查 `command_id`；同 ID 同摘要返回原结果，同 ID 不同摘要返回冲突；
3. 若命令含外部引用，在进入本地事务前通过对应公开 API 验证 tenant、resource type、ID 与 revision；创建 Project 的重放检查先于 Party 调用；不可用时失败，不创建待猜测记录；
4. 开启数据库事务并锁定 Project；
5. 校验 `expected_revision`、Project 状态和领域不变量；
6. 写当前投影、不可变命令结果和一个 History Event；
7. 原子提交并返回新 revision。

外部引用验证和本地提交之间可能发生来源变化，因此 Link／Assignment 保存验证时的 source revision。后续读取目标当前投影时仍以目标应用为真相；如果来源 revision 已变化，界面显示“引用已更新”而不是静默覆写 Project 历史。

创建普通 Project 时可选提交 `client_party.party_id + source_revision`。Project Workspace 通过 Party Directory 的 `GET /api/v1/references/parties/{id}` 校验精确 ID、tenant、active、revision、kind 和 legal name，并只保存 `{party_id, source_revision, kind, legal_name}`。这份快照解释立项发生时选择了谁，不是 Party 当前档案；Party 改名、停用或继续产生 revision 不回写 Project。首个切片不把登记标识、联系方式或搜索目录复制进 Project，也不提供重选／解除命令。

## API 与错误语义

第一版 API 前缀固定为 `/api/v1/projects`：

| API | Capability | 结果 |
| --- | --- | --- |
| `POST /projects` | `project-workspace.create` | 创建 draft Project |
| `PUT /projects/{id}` | `project-workspace.update` | 完整替换可编辑基础资料 |
| `POST /projects/{id}/assignments` | 通用 `assignment.manage` 或匹配目标 role key 的三项 `assignment.*.manage` 之一 | `assign` 或 `end`；专用能力不能跨角色 |
| `POST /projects/{id}/resource-links` | `project-workspace.resource-link.manage` | `link` 或 `unlink` |
| `POST /project-packages` | `project-workspace.hierarchy.manage` | 创建顶层 Package；编号可选 |
| `POST /projects/{id}/package-conversions` | `project-workspace.hierarchy.manage` | `attach_existing` 或 `wrap_as_package`，原 Project ID 不变 |
| `POST /projects/{id}/package-conversions/{conversion_id}/rollback` | `project-workspace.hierarchy.manage` | 满足安全条件时恢复原位置；否则 409 且零部分写入 |
| `POST /projects/{id}/complete-initiation` | `project-workspace.initiation.complete` | 激活或返回补全缺口 |
| `POST /projects/{id}/cancel` | `project-workspace.cancel` | 取消且保留全部事实 |
| `GET /projects/{id}` | `project-workspace.read` | 只返回 Project 自有事实和稳定引用 |
| `GET /projects/{id}/commercial-summary` | `project-workspace.read` | 并行读取 Contract／Money／Billing 的即时分段；不持久化、不跨来源加总 |
| `GET /projects/{id}/history` | `project-workspace.history.read` | 按 revision、event sequence 分页 |

通用错误：未认证 401；能力或 scope 不足 403；不存在且不可见统一返回 404；格式错误 400；业务前置不满足 422；revision、编号、active 负责人或幂等冲突 409；外部引用服务不可用 503。失败响应包含稳定 code、request ID 和可重试标记，不把数据库或上游秘密返回客户端。

列表与详情响应同时返回 `allowed_actions`。列表值来自当前已验证 Credential；详情值再按 Project／Package 类型、状态、scope 和 active Conversion 收敛。它只控制页面呈现与用户恢复路径，不取代写端点对 Credential、tenant、scope、目标 role key、revision 和状态机的最终检查。详情仅在存在 active attach／wrap 时返回 `active_package_conversion`，使撤销命令使用服务端事实而不是浏览器猜测 conversion ID。

## 鉴权边界

- Capability Manifest 是 Project 自己发布的稳定动作目录。
- 本地模板 Provider 只用于开发和验收，必须验证与正式 Provider 相同的上下文 Schema、audience、action、tenant 和 scope。
- HTTP 层先删除任何非受信来源注入的身份 Header；应用层只消费已验证 Context。
- Gateway 的基础验签不能替代 Project 的最终授权。
- 正式页面的“刷新权限”固定调用 Gateway `/gateway/authorize/project-workspace`，由 Gateway 使用 HttpOnly IAM Session 申请新 Credential；浏览器不读取 Capability Token。IAM 拒绝重新签发时 Gateway 清除 Project 路径下旧 Capability Cookie，页面进入只读恢复态，避免把最长五分钟残余凭证误显示成当前授权。
- 所有 Capability ID 使用 `project-workspace.*` 命名空间，与正式 IAM audience 保持一致；IAM 不接纳 `project.*` 别名。
- `project-workspace.read` 只能读取租户级或明确包含 Project ID 的 scope；Assignment 不自动产生系统权限，授权仍由 IAM／Policy Provider 签发。
- `project-workspace.hierarchy.manage` 必须使用 tenant scope：单个命令会同时改变原 Project 与目标／新建 Package，不能让只覆盖一个资源 ID 的凭证越权改变另一个节点。该能力只授予显式 OA 业务管理员，不由身份超级管理员或项目负责人隐式获得。
- Assignment 写入的专用能力固定为 `assignment.business-owner.manage`、`assignment.production-owner.manage` 和 `assignment.participant.manage`。`assign` 从请求 role key 选择所需动作；`end` 必须先由 Project 读取既有 Assignment 的 role key 再判断，客户端不能通过省略或伪造角色绕过。
- Customer Overlay 可以把客户站点角色映射为这些稳定能力，但 Project 不接收、保存或判断 `business_manager`、`production_manager` 等客户岗位名。通用 `assignment.manage` 只在显式业务管理员范围使用，不是 IAM 超级管理员的默认能力。
- 凭证 action 与 scope 通过后，Project 仍按自身状态、`expected_revision`、负责人唯一性和引用有效性作最终判断；因此 `active` 状态下被领域规则禁止的资料修改继续返回 `409`。
- Project→Party 读取在正式 IAM 模式使用 Project Workspace 工作负载身份交换 audience=`party-directory`、action=`party-directory.reference.read`、resource=`party:{id}` 的短期凭证；客户端令牌、用户 Cookie 和长期 ACR／阿里云密钥不得传给 Party。
- Project 客户页读取候选时另行使用当前人类 IAM Session，通过 Gateway 为 `party-directory` 签发用户凭证；这只允许浏览器发现 active 候选。创建端点不信任候选的名称、类型或状态，只接受 Party ID／revision，并继续执行上一条工作负载精确复验。两条授权链不能合并，也不能把 Party Capability Cookie转发成服务间凭证。

## 外部引用策略

Assignment 第一切片只接受 People 的 `person` 或 `group` 引用。请求不直接提交姓名或成员数组；验证成功后只保存 source、kind、ID、revision 和用于历史解释的非权威 display snapshot。People 不可用时新建／结束 Assignment 命令失败关闭，但现有 Project 和 Assignment 仍可读。

Resource Link 使用固定目标组合 `(target_service, resource_type, resource_id, relation_key)`；允许的 service/type 由版本化配置清单校验。清单现包含 Billing `invoice_registration`。Project 只保存引用及验证 revision，不保存合同金额、Money 余额、开票金额、Evidence 对象信息或设计阶段状态。解除 Link 不调用目标删除接口。

Project 完成立项只要求至少一个规范化 `project_type_key`，不复制 Production Operations 的 Type Definition 注册表或阶段规则。Production 创建 Work Item 时按自身代码拥有的定义校验类型是否注册、Project 是否选择该业务方向及定义版本；因此 `consultation-only` Project 可以独立进入 active，而未知生产定义仍在事实所有者入口失败关闭。

稳定引用读取端点固定为：Contract `GET /api/v1/references/contracts/{id}`、Money Obligation `GET /api/v1/references/obligations/{id}`、Money Transaction `GET /api/v1/references/transactions/{id}`、Billing `GET /api/v1/references/invoice-registrations/{id}`、Evidence File `GET /v1/references/files/{id}`、Design Work Item `GET /api/v1/references/design-work-items/{id}`。本地组合可以使用 tenant-scoped Bearer 模板凭证；正式组合全部使用目标 audience 的 IAM Capability Credential。C3 版本化投影追加来源自有的必要金额／余额字段，但不暴露完整领域模型。

`commercial-summary` 只遍历 active Contract／Money／Billing Link，最多八路并行读取并保留 Link 顺序。一个来源的不存在、契约无效、IAM／网络／服务故障或 revision 变化只标记该 Segment；其他成功 Segment 继续返回。结果不写 Project 表、不缓存、不产生 Project revision，也不自动加总语义不同的合同、义务、现金与开票金额。详细方案见 `tech/assembly/c3-commercial-summary.md`。

Party 不进入通用 Resource Link 清单：客户主体是 Project 立项语义中的类型化关系，并需要保存选择时快照；把它降为任意 `relation_key` 会丢失这一不变量。`PARTY_BASE_URL` 只有在创建请求实际包含 `client_party` 时才成为运行依赖；未配置或不可用时只拒绝该写命令，既有 Project 及未选择 Party 的新 Project 仍可读写。

Project 根页面的新建对话框始终保留“无客户主体”选项。正式 Suite 中按需调用 Party 列表，选项只携带 ID 与 revision；Party 无权、为空或故障时保留已填写 Project 字段并允许 Base 创建。详情显示 Project 本地保存的客户名称与来源 revision，不按页面读取重新拼接 Party 当前事实。首个切片没有改绑／解绑 API 或控件，避免在业务语义未知时把基础资料更新变成跨域历史覆盖。

正式 D1 的全部下游引用都不配置长期 `*_ACCESS_TOKEN`。Project 用独立 Ed25519 工作负载私钥为 People、Party、Contract、Money、Billing 等每个精确资源请求签发一次性、请求摘要绑定的 Assertion，向 IAM `POST /api/v1/service-credentials` 交换最长一分钟、单 action、精确资源 Scope 的 Capability Credential；凭证接近过期时自动刷新，目标返回 `401` 时失效并最多重换一次。私钥只在 Project，IAM 只持公钥和版本化 Workload Policy；active／retiring 重叠轮换不重启 Project。IAM 或某来源不可用时，新增写命令继续失败关闭；C3 读组合只降级对应 Segment。机器身份契约见 `tech/_shared/contracts/service-credential-exchange.yaml`。

隔离 D1 的 `acceptance/d1-c1-authorization/verify-running` 以参考 OA C1 Overlay、两个 Project、多个 Package 和九类 Subject 执行 152 个真实入口断言并全部匹配。目标应用拒绝相邻经理跨角色管理、跨 Project、伪造 Header、状态绕过、旧双 revision、第三层和不安全撤销；Assignment 结束后旧事实不能取得新凭证，已签凭证只按最长五分钟 TTL 留存。Project Package 结构治理只授予 OA 业务管理员的 tenant-scoped `project-workspace.hierarchy.manage`，没有进入项目资源角色。页面以服务端 `allowed_actions` 呈现动作；撤销结构能力后新动作集合收敛，重新授权失败时 Gateway 清除旧应用凭证。该证据新增 Project Workspace 自有 migration，但没有共享角色表、跨应用 ORM 或客户条件分支。

## 历史与纠错

### C5 Completion Profile 检查点

Project Workspace 以 `reference-oa-c5-v1` 读取 Approval & Audit Results 的项目级最小引用集合。评估响应逐项区分 `satisfied`、`pending`、`not_applicable` 与 `unavailable`，并比较最近确认的精确来源集合；GET 不写 Project 或来源数据库。显式确认使用独立 `completion.read`／`completion.confirm` Capability、Project Scope、expected revision 与 result ID／kind／revision／digest，只追加 Project confirmation、history、command 和新 revision，Project 仍为 `active`。来源纠正或撤回只产生 drift；成功命令回放先于外部调用，不能因 Results 暂时不可用而失去原幂等结果；新的确认遇到 `unavailable` 必须返回可重试的 503，而不是业务条件未满足的 422。该检查点不代表完整 OA 或整个 Project 完成。

- Project 当前资料通过 `expected_revision` 的完整替换命令修正；旧值留在 History Event。
- Assignment 通过“结束旧记录＋新建记录”更换，不更新旧主体或角色。
- Resource Link 通过 unlink 结束后再 link，不把旧目标 ID 改写成新 ID。
- Project 取消是终态，首个切片不提供恢复；以后若有真实需求，必须新增显式 revision 和事件，不能改数据库状态。
- `attach_existing` 和 `wrap_as_package` 只改变 Project Workspace 自己拥有的父子关系；原 Project ID、编号、Assignment、Resource Link 和外部事实均保持原位。
- 每次转换保存独立 conversion ID、原父节点、原排序和创建时双 revision。attach 可恢复原位置；自动外层 Package 只有仍未被继续使用时可撤销，撤销后外层 Package 进入 `cancelled` 而不物理删除。
- 自动外层 Package 已增加子项、Assignment、Resource Link、资料或状态变化时返回 `package_rollback_unsafe`；用户必须先明确处理新增事实，服务不猜测级联删除。
- 历史 payload 只包含 Project 自有字段和外部稳定引用，不复制外部完整响应。

## 部署与可移植性

第一实现必须具备独立 Go module、独立 migration、独立 PostgreSQL database/user、独立 `.env.example`、Dockerfile／Compose、健康检查、备份恢复和 release 入口。所有地址由配置注入；业务代码不得硬编码 `localhost`、ECS IP、Docker 服务名或客户域名。

本地 Demo 可以直接发布 Project HTTP 端口；D1／公司 Ubuntu 组合只能由 Nginx 对外发布，Gateway 路由到内部 Project。部署档改变不改变 Project ID、revision、命令、历史、API 或数据库所有权。

## 明确延期

- 设计项目阶段、成果要求和完成投影；
- 通用任务／审批／通知；
- 合同或结算自动生成 Money Flow 义务；
- 项目包合同金额向子项目分配、预算、开票、奖金、成本与外委；
- 搜索引擎、事件总线、Redis、服务注册和多节点 HA。

这些延期项不能以 JSON 扩展字段或数据库直连提前塞入 J1。
