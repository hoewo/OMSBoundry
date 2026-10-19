# OA 完整体权限架构验证方案

## 目标

OA 的站点职位、项目角色、临时授权、数据范围和业务状态同时参与授权。如果只验证统一登录或全能力管理员，入口链虽然贯通，仍无法证明复杂 OA 权限能够在小应用边界内成立。

该方案验证以下结果：

> 多层 OA 权限容易被复制进 Gateway、Project 和各业务应用 → 以版本化身份、能力、业务事实和短期决策凭证形成可复验授权，在不共享数据库或客户角色代码的前提下完成资源级业务控制。

## 依据与冲突处理

验证输入包括：

- `docs/discussions/round-08-mandatory-iam-and-permission-decoupling.md`；
- `docs/discussions/round-13-permission-chain-decoupling.md`；
- `docs/specs/identity/access-contract-v1.md`；
- `docs/specs/identity/access-threat-model-v1.md`；
- 参考 OA 的 `OA_PERMISSION_MODEL.md`、`PERMISSION_CATALOG.md`、`PERMISSION_DESIGN_GUIDE.md` 和 `AI_PERMISSION_DESIGN_SUMMARY.md`；
- 参考 OA 的后端权限目录、服务和中间件实现。

参考 OA 文档用于建立角色、范围、字段和操作样本，不直接成为 Base 权限模型。来源冲突按以下顺序处理：

1. 当前确认需求和 Architecture Baseline 决定目标边界；
2. 新版权限目录和设计指南提供验收样本；
3. 旧运行代码只证明存在过的行为；
4. 冲突项形成正反例，不通过猜测合并。

身份超级管理员是否自动拥有业务全权是已知冲突样本。目标边界采用“身份治理权不自动产生业务权”，任何业务访问均要求显式 Capability 和范围。

## 信任链与责任

```text
Browser
  → Nginx: TLS、连接和请求边界
  → Gateway: 路由、Header 清洗、入口验签、限流
  → IAM / Policy: Subject、Tenant、授予、策略与签发
  → Target App: 凭证复验、资源范围、状态、revision、最终执行
```

Nginx 和 Gateway 不保存账号、角色或策略。IAM 不读取目标应用内部表，也不转发每个业务请求。Project、People & Groups 和 Party Directory 不签发目标应用权限。目标应用不解释客户职位名。

### 可信边缘与限流客户端键

D1 只把 Nginx 暴露为网络入口。Nginx 必须覆盖客户端提供的 `X-Forwarded-For`，并把与自己直接建立连接的 `$remote_addr` 作为唯一值转交；不得用可追加外来内容的代理链作为当前客户端事实。Gateway 默认忽略该 Header，只有在部署档显式设置可信代理模式、且 Gateway 不能被绕过直连时，才使用这个单值作为限流客户端键。

可信代理模式只接受一个语法有效的 IP。逗号链、空值或非法值一律退回 Gateway 的直接对端地址；这能防止客户端通过伪造 Header 逃逸限流，同时避免固定 Nginx 前置时把所有真实用户聚合成同一个 Nginx 容器地址。D1 Compose 固定启用该模式并保持 Gateway 无宿主机端口。若以后在 Nginx 前增加负载均衡器或其他代理，必须由新的可信边缘先规范化客户端地址并重新验证信任边界，不能直接放宽为信任任意转发链。

Auth58 的 `v0.1.0-test.5` 目标证据在正式默认 `120 RPM` 下，让两个不同容器地址各经同一 Nginx 发出 70 次请求，结果为 `70/70 + 70/70`；这直接证明两个客户端不再共用 Nginx 容器配额。普通 D1 交付验收也在默认限额下通过。C2 全矩阵自身是单地址、单分钟超过 120 次的合成负载，因此只在临时验收配置使用 `1200` 完成 110 个断言和完整故障注入，随后自动恢复正式 `120`；该验收例外不是运行时边界或生产容量结论。

## 授权判定元组

每次受保护动作使用以下可观测元组：

| 维度 | 事实 | 权威来源 |
| --- | --- | --- |
| 身份 | Subject、账号状态、认证时间 | IAM |
| 租户 | Tenant 与安装范围 | IAM、Suite Manifest |
| 目标 | Audience、Capability、资源类型 | 目标应用 Capability Manifest |
| 授予 | Subject 获得的稳定 Capability | IAM |
| 业务范围 | Project Assignment、组织范围、Resource Link、有效时点 | 版本化 Business Fact Snapshot |
| 策略 | 客户职位／项目角色到目标动作与资源的映射 | 版本化 Customer Overlay／Policy |
| 本地状态 | 资源归属、状态、revision、金额与领域不变量 | 目标应用 |

可信策略边界只在全部输入有效时签发单一 Audience、短期、有界的 Capability Credential。目标应用再次验证凭证和本地状态；凭证通过不等于命令必然成功。

## 参考身份样本

参考角色只存在于测试夹具和 Overlay 映射：

| 样本 | 全局能力样例 | 资源级能力样例 | 明确不自动获得 |
| --- | --- | --- | --- |
| identity-super-admin | 账号、邀请、授权治理 | 无 | 项目、合同、资金、文件、奖金业务能力 |
| oa-business-admin | 显式站点管理 Capability | 按策略授予 | 绕过目标应用状态和审计 |
| general-manager | 多业务域只读 | 项目只读 | 修改、上传、删除、分配、指定 |
| business-manager | 经营域全局查看、负责人配置 | 被授权的项目治理动作 | 所有项目经营执行写入 |
| production-manager | 生产域全局查看、生产负责人配置 | 被授权的项目治理动作 | 经营配置和所有项目生产执行写入 |
| project-business-owner | 无 | 单项目经营维护、文件和经营奖金 | 其他项目、生产奖金、身份治理 |
| project-production-owner | 无 | 单项目生产维护、文件和生产奖金 | 其他项目、经营奖金、身份治理 |
| business-member | 无 | 单项目日常经营动作 | 负责人指定、人员组织、奖金分配 |
| production-member | 无 | 单项目日常生产动作 | 负责人指定、人员组织、奖金分配 |
| finance | 明确财务动作 | 被授权项目的金额／票据动作 | 经营或生产资料治理 |

同一 Capability 在不同资源范围的结果必须不同；同一客户角色改名后，目标应用看到的 Audience、Action 和 Resource Scope 保持不变。

## 实验设计

### 控制组

- 有效 Subject、正确 Tenant、正确 Audience、正确 Capability、已授权资源和允许状态组成允许控制组；
- 现有全能力系统管理员只用于入口与业务健康控制，不作为 OA 权限通过证据；
- 本地模板 Provider 与正式 IAM 使用相同 Subject／Tenant／Capability／Scope 语义，分别证明独立运行和完整入口。

### 单变量反例

每个允许样例分别只改变一个变量：

| 变量 | 反例 | 预期 |
| --- | --- | --- |
| 身份 | 无 Session、账号停用、错误 Subject | `401` 或拒绝签发，不写业务事实 |
| Tenant | 凭证 Tenant 与目标安装不一致 | 在业务 handler 前拒绝 |
| Audience | Contract 凭证投向 Money 等 | `401`，不泄露资源存在性 |
| Capability | 只有 `view` 却调用写命令 | `403`，不产生部分写入 |
| Resource Scope | 同角色访问未分配项目或另一资源 | `403` 或过滤为空 |
| Business Fact | Assignment 已结束或来源 revision 失效 | 新授权失败关闭；既有凭证仅按 TTL 行为 |
| Local State | 已终止合同、关闭义务、revision 冲突 | 目标应用按领域错误拒绝 |
| Client Input | 伪造 `X-OMS-*`、Authorization 或按钮请求 | Gateway 清洗，目标应用仍复验 |
| Time／Signature | 过期、未来、超长 TTL、未知 `kid`、篡改签名 | `401` |
| CSRF／Origin | 缺失、错误或跨源写请求 | `403` |

### 多变量旅程

每条 C1～C8 旅程使用至少三名 Subject：预期执行者、相邻只读角色和无关角色。旅程记录登录、Credential 交换、页面 `allowed_actions`、直接 API 正反例、数据库状态差异和审计。高风险动作增加撤权后旧页面提交、并发 revision 和幂等重放。

## C1～C8 权限关注点

| 旅程 | 主要范围事实 | 高风险动作 | 必需反例 |
| --- | --- | --- | --- |
| C1 项目与责任 | Tenant、Project、Assignment | 建立项目、指定负责人、完成立项 | 只读经理修改；未指派人员完成立项 |
| C2 经营与合同 | Project Link、Contract | 承接确认、合同变更、终止、结算 | 生产角色改合同；终止后继续变更 |
| C3 开票与回款 | Project、Contract／Billing／Money 引用 | 开票、更正、资金登记与核销 | 非财务写票据；错误 Audience 核销 |
| C4 生产交付 | Project Assignment、生产类型 | 负责人变更、阶段完成、成果替换 | 经营角色推进生产；结束任职后写入 |
| C5 批复审计 | Project 与批复审计结果 | 创建、读取、金额读取、纠正、撤回、Evidence 管理、历史读取 | 无金额能力读取或提交显式金额；无 Evidence 能力改文件；相邻 Project／错 Tenant／Audience／Scope；缺来源时登记或完成；当前 D1 已直接验证金额脱敏、跨 Project、无 create 和 correct Grant 撤销／恢复，账号停用与双 Tenant 待补 |
| C6 成本外委 | Project、精确 Contract／Party／Evidence 引用 | `cost-entry.create/read/correct/invalidate`、`outsourcing-engagement.create/read/correct/cancel/score/suggest-payable`、金额与 Evidence 分离读取 | 普通成员确认或更改付款建议；无金额能力看到真实金额；结束 Assignment 后写入；相邻 Project／错 Tenant／Audience／Scope；依赖 503 时产生部分记录 |
| C7 奖金 | Project Assignment、来源快照 | 策略运行、分配、撤销 | 普通成员分配；经营角色分配生产奖金 |
| C8 资料统计 | Project、人员、文件与投影范围 | 下载、导出、个人待办 | 只有列表权下载；无金额权看到 `0` 或真实金额 |

C4 canonical 能力使用 `production-work-item.*` 并以 `aud=production-operations` 签发；旧 `design-work-item.*`／`aud=design-operations` 在兼容期只能代理 `design-v1`，绝不能把旧设计权限扩大为咨询权限。IAM／Policy Provider 可以根据 active `production_owner` 和 Project Scope 签发精确能力，但 Production Operations 仍须复验 tenant、audience、action、scope、type、resource、state 和 revision 并作最终授权。`consultation-v1` 必须包含生产责任角色允许、经营角色拒绝、相邻 Project、结束任职、错误 Tenant／Audience／Scope 和撤权后的正反例。

Audience 与 action namespace 分离：Production 在 Capability／Suite Manifest 中声明 `capability_namespace=production-work-item`。Gateway 用它校验入口动作；IAM Grant、Business Fact Policy 和 Workload Policy 必须继续验证动作存在于 `production-operations` 的 Capability Catalog，不能仅凭字符串前缀放行。未声明 namespace 的既有应用仍回退 application ID。

## 故障与撤销

| 场景 | 预期行为 |
| --- | --- |
| IAM 停止 | 新登录、续签和新授权失败；未过期凭证只在原 TTL 与 Scope 内工作 |
| Policy／业务事实来源停止 | 需要新事实的新授权失败；不扩大旧 Scope，不查询内部表补偿 |
| Gateway 停止 | 公网业务入口失败，数据库事实不变 |
| 单一目标应用停止 | 该路由 `503`，其他 Audience 和应用继续工作 |
| 账号停用／授权撤销 | Session 和后续签发立即受控；已签凭证残余窗口按契约记录 |
| Assignment 结束 | 新资源级授权失败；旧凭证行为受 TTL 限定并可审计 |
| 公钥或 Manifest 无效 | Gateway／目标应用不 ready，不降级为跳过验证 |

## 证据与指标

每个验证记录至少包含：

- Base、Overlay、Suite Manifest、Capability Manifest 和 Policy 版本；
- Subject、Tenant、Audience、Action、Resource／Scope、凭证 `jti` 或决策标识；
- Business Fact Snapshot 来源与 revision；
- HTTP 状态、业务错误、前后状态摘要和审计引用；
- Gateway 与目标应用的拒绝层级；
- 单次和批量场景延迟，区分授权签发与业务请求；
- 客户角色或权限变化涉及的 Base、Overlay、契约和 migration 变更数量。

日志和报告不保存密码、Session、JWT、Cookie、私钥或邀请／激活原文。

## 当前 D1 动作级基线

`acceptance/d1-authorization/verify-running` 在真实运行的 D1 组合中建立两个非管理员 Subject：合同只读用户只获得 `contract-ledger.view`，主体登记员获得 `contract-ledger.view` 与 `contract-ledger.party.register`。验收从 Nginx 单入口经过 Gateway 和 IAM 获取单 Audience 短期凭证，再由 Contract Ledger 复验并执行最终动作授权。

当前直接证据覆盖：

- 非管理员调用 IAM 授权管理端点返回 `403`，业务 Capability 不推导 IAM 身份治理权；
- 两份 Credential 只有各自显式 Grant、单一 Contract Audience、Tenant Scope 和不超过五分钟的 TTL，没有夹带相邻业务动作；
- 只读用户读取 Contract Party 成功，直接提交 Party 写请求返回 `403` 且没有产生记录；
- 主体登记员读取和登记 Party 成功；
- 两个 Subject 在没有 Money 授予时请求 Money Audience 均返回 `403`；
- 账号停用后 IAM Session 与新 Capability Credential 签发立即返回 `401`；
- 停用前已签发的 Contract Credential 在最长五分钟 TTL 内继续按原 Audience 与 Action 工作，残余窗口与故障／撤权契约一致。

该 D1 基线只证明 Tenant Scope 内的动作级最小授权，不证明完整 OA 权限。D1 bootstrap 管理员已经通过部署工具获得显式业务 Grant，本实验也不证明“身份超级管理员默认无业务权”。

正式组合还要求 Capability ID 属于目标 audience 命名空间。Project Workspace 固定使用 `project-workspace.*`，并通过 `project-workspace-capabilities.json` 发布十三项动作；其中三项角色专用 Assignment 管理动作把负责人配置分开，`project-workspace.hierarchy.manage` 专管 tenant-scoped Project Package 结构变化，不接纳旧的 `project.*` 别名。该约束由 Access Contract、IAM Catalog 和 Project 全包测试共同守卫，保证 IAM、Gateway 和目标应用使用同一种权限语言。

`acceptance/d1-resource-authorization/verify-running` 已把 Project Workspace 装入隔离 D1，并用两个真实 Project ID 完成 Nginx → Gateway → IAM → Project 入口闭环。非管理员 Subject 只获得 Project A 的 `read`／`update` Resource Grant：省略 scope 和请求 Project B 凭证均为 `403`，Project A 凭证只含单一 audience、两项 action、一个 Project ID 和不超过五分钟 TTL；A 的读取／修改成功，B 的读取／修改失败，拒绝修改后 B 的名称和 revision 保持不变。Project 页面及脚本也已在 `/project` 前缀下通过。联合备份销毁卷恢复后，资源专项和完整 D1 再次通过。详细步骤、矩阵和发现见[OA 第一阶段权限实验报告](../../../docs/reports/oa-stage-1-authorization-experiment-20260926.md)。

该证据证明的是显式直接 Resource Grant，不是业务事实策略。后续 Business Fact 实验已补齐最小策略闭环，C1 补充实验又补齐已实现项目责任范围的身份管理员反向隔离和多角色矩阵；跨租户、C1 剩余生命周期与 C2～C8 仍是阻断性缺口，阶段结论保持 `insufficient-evidence`。

## 当前 D1 Business Fact Policy 基线

`acceptance/d1-business-fact-authorization/verify-running` 在独立 D1 组合中建立一条最小、可证伪的 Project Assignment 策略，不把完整 OA 角色系统一次性塞入 IAM：

1. Project Workspace 拥有 Assignment 并发布带稳定 ID、source、revision、有效时点和结束状态的策略事实；
2. People & Groups 拥有 Person，并发布 Person revision、状态和 `iam_subject_id` 映射；
3. IAM 内的 `project-assignment-access@1.0.0` 使用精确 Project／Person scope 的一分钟服务凭证读取这两类事实，把稳定 Project 角色键按版本化配置映射为 Project Workspace actions；
4. Gateway 只把不透明事实引用交给 IAM，不读取事实、不解释角色、不扩展 scope；
5. Project Workspace 再次验证目标 audience、actions、精确 Project scope 和本地 revision／状态后执行命令。

本次运行直接证明：

- 非管理员 `business_owner` 只得到 Project A 的 `read/update`，Project B 读取被拒绝；
- IAM 决策记录 Policy ID／Version 和 Assignment ID／Revision，凭证只有一个 audience、一个 Project scope 且 TTL 不超过五分钟；
- 额外直接授予的 `history.read` 可以单项撤销，撤销没有停用账号，也没有删除策略产生的 `read/update`；
- Assignment 结束会推进事实 revision，旧 revision 的新凭证请求失败关闭；已经签发的凭证不被伪装成即时失效，只在既定最长五分钟窗口内继续工作；
- 策略签发和单项撤权均写入 IAM 不可变审计；IAM、Project 和 People 之间没有共享表或 ORM 模型。

## 当前服务身份与凭证生命周期基线

Project 创建 Assignment 时不再注入静态 People Token。正式 D1 使用版本化工作负载身份交换：

1. Project 只持有自己的 Ed25519 私钥，按精确 audience/action/resource 生成最长 60 秒、一次性且请求摘要绑定的 Assertion；
2. IAM 只持公钥集合，验证 Tenant、时间、请求摘要、PostgreSQL replay store 和版本化 Workload Policy；
3. IAM 只签发最长 60 秒、单 `people-groups.view`、精确 Person／Group Scope 的普通 Capability Credential；
4. People 再次复验并只允许对应单一资源，拒绝目录列表、catalog 和相邻资源；
5. Project 按资源缓存未接近过期的凭证，`401` 时失效并最多重换一次。

D1 的 private/public 两个卷只表达密钥放置，不改变协议。`workload-keyctl` 使用 active／retiring 重叠窗口轮换；Project 每次签名和 IAM 每次验签都重读文件，因此轮换前后两个 Key ID 均进入 IAM 审计，而 Project 容器 ID 不变。

隔离 D1 实际证明：没有 `PEOPLE_ACCESS_TOKEN`；自动交换、续期和轮换成功；停止 IAM 或 People 时，新 Assignment 返回 `503`、已有 Project 读取继续 `200`，恢复后无需重启 Project。稳定契约见 `tech/_shared/contracts/service-credential-exchange.yaml` 与 `contracts/access/v1/` 的 Workload schemas。

稳定 Assignment 策略当前覆盖 `business_owner`、`production_owner` 和 `participant` 三个 Project 角色键；站点角色使用独立版本化 Customer Overlay，不扩充 Project 角色键。工作负载交换只解决机器身份和最小凭证生命周期，不代替用户角色矩阵、Business Fact Policy 或目标应用最终授权。C4 隔离 D1 已证明 production_owner 精确 Project 允许、business_owner 与相邻 Project 拒绝、IAM 故障零写入／恢复重试，以及 Assignment 结束、Grant 撤销和账号停用后的新凭证拒绝与页面动作收敛。

## 当前 C1 多角色／多 Project 基线

`reference-oa-c1-role-overlay.json` 和 `customer-role-capability-overlay.schema.json` 固定了第一份可验证客户角色映射。Overlay 只能引用目标 Capability Manifest 中真实存在且属于同一 audience 的动作；未知或跨应用动作失败关闭。`identity_super_admin` 显式没有 Project 业务动作；`business_manager` 和 `production_manager` 分别只获得经营负责人、生产负责人 Assignment 管理动作。资源级项目负责人和参与者继续由 `project-assignment-access@1.0.0` 从版本化 Assignment／Person Fact 推导，不把站点职位冒充项目责任事实。

隔离 D1 `omsd1c1` 在 `127.0.0.1:18130` 上执行 `acceptance/d1-c1-authorization/verify-running`，责任权限基线最初得到 116 个 HTTP 断言；加入 Project Package 后为 134 个；继续加入客户页面资源、状态收敛 `allowed_actions`、权限刷新与撤权恢复后，同一正式入口得到 152 个 HTTP 断言、0 个状态不匹配，最长本地请求 0.263164 秒。矩阵覆盖：

- 身份超级管理员只有 IAM 治理能力时，Project 换证为 `403`；即使伪造身份 Header，Gateway 仍清洗并把 GET 返回 Workbench 授权入口；
- OA 业务管理员通过显式 tenant actions 创建 Project A／B、装配双负责人和参与者并完成立项；
- 经营／生产经理只可建立或结束匹配的负责人 Assignment，跨角色请求 `403` 且 revision 不变；
- 项目双负责人得到 Project A 的 `read/update`，参与者、总经理和财务只有读能力；Project B、错误 Fact／Scope 和写操作分别失败关闭；
- 正确 Capability 也不能越过 Project `active` 状态规则，目标应用返回 `409` 且名称、revision 不变；
- 结束 Assignment 后旧 revision 的新签发为 `403`，结束前凭证只在 `exp-iat <= 300s` 内继续；IAM 保存 Grant、Policy 签发和撤销审计。
- 总经理缺少结构管理能力时创建 Package 返回 `403`；业务管理员可以创建 Package、把既有 Project 归包或外包、幂等重放、按双 revision 拒绝并发冲突，并在结构未被后续使用时安全撤销；第三层嵌套和已被修改／引用的外包 Package 撤销失败关闭且不产生半写。
- Project 列表／详情返回当前 Credential、Scope 和本地状态共同收敛的 `allowed_actions`；客户页面据此展示 Project Package 动作，并通过 Gateway 使用当前 IAM Session 显式刷新权限。
- 撤销 `hierarchy.manage` 后，新凭证和页面数据不再包含结构动作，直接结构命令返回 `403`；撤销全部 Project 入口能力后重新授权失败，Gateway 清除旧应用 Capability Cookie，旧页面不得继续提交。

本轮 Base 变化包含三项稳定 Assignment Capability、一个结构管理 Capability、目标 role key 二次校验、通用 Overlay 契约、Project Workspace 自有的两级 Package 数据／转换边界，以及与服务端同源的页面动作投影和失效恢复；客户差异只在版本化映射。没有共享角色表、跨应用 ORM、Gateway 最终授权或客户核心分支。它支持 C1 项目责任权限、Project Package／父子转换／安全撤销和页面权限变化子命题，不证明跨租户、Package 金额分摊、组织／临时代理、字段／金额／文件动作或 C2～C8，阶段仍为 `insufficient-evidence`。

## 当前 C2 目标应用授权子矩阵

Business Engagement 第一条后端闭环已发布 `business-engagement-capabilities.json`，把创建、读取、适用性、方式、材料、取消和历史拆成当前实现的目标应用原生 action，不接受参考 OA 的粗粒度 `manage_business_info`；尚未实现的下游建议 action 不提前发布。入口适配器先删除所有外来身份 Header，再验证 `aud=business-engagement` 的 Ed25519 Credential、原生 action 和 tenant／Project／Business Engagement scope；Service 与 PostgreSQL Store 随后继续验证外部引用 revision、expected revision、当前状态和材料规则。

当前自动证据覆盖：无凭证 `401`、缺 action `403`、Project B scope `403`、错误 audience 拒绝、遗留粗 action 拒绝、伪造 Header 清洗、取消状态 `409`，以及允许的精确 Engagement scope 成功；拒绝路径不进入目标命令。PostgreSQL 18 真库又证明并发仅一个 revision 成功、命令幂等、不可变历史和取消后写入失败。IAM Catalog 和 Access Contract 同时加载该 Manifest，避免 IAM 与目标应用使用两套权限语言。

Business Engagement 现已有可执行 server 和严格分离的两种装配：本地 template 模式要求应用、Project 与 Evidence 三份显式测试凭证；正式 IAM 模式禁止这些静态下游 Token，使用独立 Ed25519 工作负载身份分别申请 `project-workspace.read + project:{id}` 与 `evidence-service.view + file:{id}`。交换断言绑定完整 audience／action／scope，请求按精确资源缓存，目标返回 `401` 时只失效该资源并最多重换一次。Workload Policy 只允许上述两组目标，没有给 Business Engagement 目录读取、上传文件或其他应用动作。

这次装配审计暴露并修复了一项目标服务不一致：Evidence 原先只接受 tenant scope，即使 IAM 能签发单 File scope，调用方也只能失败或索取过宽凭证。Evidence 现在允许 `evidence-service.view` 使用精确 `file:{id}` scope，并在列表、reference-read 与 access-grant 路由再次检查 File ID；`evidence-service.upload` 仍只允许 tenant scope。自动证据证明允许文件成功、相邻文件在进入用例前 `403`、列表过滤，以及资源级上传凭证被拒绝。

`reference-oa-c2@1.0.0` 和 `business-engagement-project-access@1.0.0` 已把该目标应用接入 Customer Overlay 与 Project Assignment Policy。IAM 的 Assignment Policy 现在按目标 audience 选择版本化映射：同一 Project／People Fact 可以为 Project Workspace 与 Business Engagement 产生不同最小动作，重复或未知 audience 失败关闭。经营负责人获得 Project 级创建、读取、适用性、方式、材料和历史但没有取消；生产负责人只读；经营经理通过显式 tenant Grant 获得当前全动作；总经理只读；财务和身份超级管理员没有隐式业务动作。

隔离 D1 `omsbizfoundry-c2` 最新已通过 `acceptance/d1-c2-business-engagement-authorization/verify-running`：95 个 `Nginx → Gateway → IAM／Policy → Business Engagement` HTTP 断言无状态不匹配。证据覆盖允许、相邻角色拒绝、Project B 拒绝、伪造 Header、动作撤销、Assignment 结束、新凭证拒绝、最长五分钟旧凭证残余窗口、取消状态 `409`、精确 Project／File 工作负载交换与 IAM 审计；页面断言覆盖 Workbench 入口、公开工作区静态资源、列表／详情 `allowed_actions`、撤权后动作收敛、取消态只读、Assignment 结束后的新入口拒绝、409／503 恢复控件、新 `command_id` 重试和返回 Project 路径。实际浏览器已经证明经营经理从 Workbench 换证进入，IAM 撤销业务动作后刷新立即移除，恢复 Grant 后动作重新出现。

最新故障注入又用旧 expected revision 得到 `409 REVISION_CONFLICT`，确认 revision 与历史均未变化；停止隔离栈 Evidence Service 后，材料命令得到 retryable `503 REFERENCE_SERVICE_UNAVAILABLE`，本地 revision 与活动材料保持，恢复 Evidence 后以新的 `command_id` 重试只新增一次 revision。页面脚本因此清除 409 待重放命令、保留 503 输入意图并要求显式重试；目标应用继续作最终授权和事务边界。详细证据见 [C2 权限与正式入口实验](../../../docs/reports/oa-c2-business-engagement-authorization-experiment-20260927.md)与[C2 权限和故障恢复实验](../../../docs/reports/oa-c2-business-engagement-fault-recovery-experiment-20260927.md)。

随后隔离 D1 完成联合备份、销毁该 Compose project 全部专属卷后的空目标恢复：IAM Grant／Audit／签名密钥、Project／Assignment、Evidence File／Version 和 Business Engagement 的 6 条主记录／30 条历史逐项保持，Suite Session 按设计归零；恢复后同一 89 条入口矩阵再次通过并新增完整 Engagement，使业务事实可继续写入。详细证据见 [C2 联合恢复实验](../../../docs/reports/oa-c2-business-engagement-recovery-experiment-20260927.md)。

该子矩阵仍未覆盖跨租户和 409／503 的真实浏览器最终点击；因此完成的是 C2 正式入口权限矩阵、正常权限变化页面恢复、本地数据恢复、账号停用、IAM／Policy 故障和自动化故障恢复，不等于 C2 完整 OA 权限或 OA 总权限轴已经通过。

## 当前 C4 Production 权限生命周期基线

Production Operations 列表与详情现在返回经过 Credential Scope 与目标资源状态共同收敛的 `allowed_actions`。页面只从该集合派生创建、负责人、成果、阶段、取消与历史动作；显式“刷新权限”经 Gateway 向 IAM 重新申请凭证，`401/403` 会清空动作、关闭提交态并显示恢复入口。Gateway 仍不代替 Production 的最终授权。

隔离 D1 的 86 条 HTTP 断言与真实浏览器直接证明：撤销 `production-work-item.stage.manage` 后，旧凭证只在最长 300 秒签名残余窗口内存在，刷新后页面与直接 API 同时失去该动作；恢复 Grant 后新凭证恢复动作；账号停用后 Suite Session、新授权与登录立即失效，页面退出提交态；重新启用后必须新建 Session 与凭证；Assignment 结束后新 Policy Credential 拒发。三种状态变化均写入 IAM 不可变审计。完整双 Tenant D1 矩阵与真实 Ubuntu 仍未被此基线替代。

## 判定

权限架构通过需要同时满足：

- 全部受保护端点拒绝缺失或无效可信上下文；
- 至少一个真实资源级策略样例贯通 Business Fact Snapshot、Credential 和目标应用最终授权；
- C1～C8 每条已实现旅程均有允许、相邻角色拒绝、跨资源拒绝和业务状态拒绝；
- 前端显隐与服务端授权同源，直接 API 不能绕过；
- 撤权、停用、故障和 TTL 行为有直接证据；
- 客户角色映射只改变 Overlay／策略，不修改目标应用能力、领域状态或数据库；
- 跨应用内部表读取、万能管理员旁路、客户角色硬编码和 Gateway 最终授权数量均为 `0`。

如果只有登录、全能力管理员或 Tenant Scope 证据，结果为 `insufficient-evidence`。任一跨租户成功、伪造身份成功、客户端绕过、目标应用跳过最终授权或权限变化要求共享表／客户核心分支，结果为 `not-supported`。
