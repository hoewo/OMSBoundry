# Business Engagement 架构决策

状态：✅ 已采用（后端纵向闭环、正式入口、工作负载身份、客户工作区和本地 D1 恢复已实现，浏览器故障专项与后续连接器待完成）

日期：2026-09-27

## 问题与可验证结果

参考 OA 把承接适用性和方式放在兼容模块、把材料数组挂在 Project 关联模型、把专家费写进宽财务记录，能够短期完成功能，但无法保持 Project、Contract、Money 与 Evidence 的事实边界 → 设立独立 Business Engagement 应用，专门拥有合同形成前的经营承接生命周期、材料业务含义和历史。

## 候选方案

### H1：独立 Business Engagement 应用（采用）

优点：承接事实可在 Contract 不存在、未启用或失败时独立成立；能独立表达适用性、方式、材料缺口、revision 和纠错；Project 保持薄容器；Contract 只从明确建议建立自己的法律事实；专家费原因与实际付款分离。

成本：增加一个应用、数据库、Capability Manifest、Suite Manifest 依赖、Project／Evidence 引用契约和后续 Contract／Money 连接器，需要独立交付与故障验收。

### H2：Project Workspace 类型化扩展（不采用为事实所有者）

Project 类型化扩展适合 Project 自身的类型、责任和关系，但承接方式、材料要求和纠错不是 Project 身份事实。把经营承接写入 Project 会让客户经营流程进入通用 Project，后续增加合同前多轮承接、失败结果或客户材料策略时继续扩大 Project 内核。

Project 仍可通过 Resource Link 和只读状态投影展示 Engagement，不拥有其可写数据。

### H3：Contract Ledger 的前置状态（不采用）

承接阶段发生在合同存在之前，可以不适用、取消、失败或长期没有合同。Contract 不应为了表达“尚无合同”而拥有投标材料、适用性或经营方式；否则独立 Contract 产品会携带并不需要的 OA 前置流程。

Contract 只在目标应用明确确认建议后创建合同草稿，并独立执行合同编号、主体、金额、revision、激活、补充、终止与结算规则。

## 直接证据

参考仓库 `/Users/crispydog/Workspace/Hoewo/WebProjects/RoadSystemOA/road-design-oa-system` 的只读审计显示：

- `docs/OA_2026_REQUIREMENTS_CONFIRMED.md:143-159` 把承接方式、阶段不适用、按方式变化的材料要求和自动完成定义为独立经营阶段；
- 同文件 `92-96` 要求“不适用”可恢复，证明它不是页面隐藏或空值；
- `backend/internal/oa2026/models.go:51-87` 分别保存阶段适用性和承接方式，已经出现不同事实；
- `backend/internal/oa2026/service.go:470-543` 以方式和材料计算承接完成，未依赖合同存在；
- 同文件 `689-703` 才把承接完成、合同完成和支付完成组合为经营进度，说明进度是聚合结果而非单一表事实；
- `backend/internal/models/bidding_info.go:9-28` 把三类文件数组直接挂在 Project 关联模型，是当前实现位置，不是目标所有权的证据；
- `backend/internal/oa2026/service.go:841-879` 直接跨读 Contract 和 FinancialRecord 计算完成，是遗留单体聚合方式，违反目标应用间禁止共享表的边界；
- 参考代码和确认需求没有找到未中标、流标、承接终止、多轮投标或自动创建合同的明确状态／行为，因此这些语义保持未知。

## 数据与端口边界

Business Engagement 目标态独占以下可写事实：

- Engagement 当前投影、revision 和状态；
- Project 稳定引用及确认时快照；
- 阶段适用性、承接方式、材料策略版本和材料业务引用；
- Command 幂等、不可变 History Event；
- 对下游建议的 delivery／confirmation 状态。

应用通过端口访问：

- Project reference-read：校验 tenant、Project ID、状态和 revision；
- Party reference-read：当前不是默认依赖；只有后续证据确认与 Project 客户不同的独立主体角色时，才校验 Party ID／revision 并保存该角色的最小业务快照；
- Evidence file-version-reference-read：校验不可变 File ID／Version；
- IAM Capability Credential：验证 Subject、Tenant、Audience、Action、Scope、TTL 和策略／事实 revision；
- Contract／Money suggestion port：提交幂等建议并记录目标确认，不直写目标数据库。

Business Engagement 不返回或代理 Evidence 字节，不读取 Project、Party、Contract、Money 或 IAM 内部表，不导入其 ORM／领域对象。项目客户关系通过 Project 语境解释，不在 Engagement 内重复保存；若未来增加独立 Party 角色，跨应用快照也只保存解释该业务历史所需的最小字段。

## 状态和一致性

2026-10-02 BF-095：材料集合在同一所有者内兼容演进，不增加应用或改变协作边界。候选新增 append／remove_one、有效 kind＋File／Version 去重／100 项上限和追加 migration；旧 put／remove 与不可变命令保持。单份移除只结束引用，Evidence 仍拥有字节／生命周期。Project v1 已允许同 relation_key 的多个不同 File／Version，不需共享表或跨库事务。旧二进制不能直接回滚新集合数据库；具体门禁及未部署状态见[规格](../../../docs/specs/assembly/oa-bidding-material-collection-slice.md)。

第一版只采用 `draft`、`collecting_evidence`、`ready`、`not_applicable`、`cancelled`。`ready` 表示材料规则满足，不表示已经签约、开票或收款。

- 所有写命令要求 `command_id` 和 `expected_revision`；
- 当前投影、History Event 和 Command 结果同事务提交；
- 相同命令重放返回原结果，同 ID 不同内容拒绝；
- 方式改变后按新策略重算缺口，旧材料引用不物理删除，历史仍能解释；
- 恢复适用后重新计算，不沿用“完成”作为不可逆事实；
- 下游建议使用 Outbox／可重试投递，目标确认独立；网络超时不能被当作目标失败或成功；
- 撤销／纠错通过新 revision 和事件完成，不更新外部应用事实。

## 权限边界

Business Engagement 发布自己的稳定 Capability，不识别参考 OA 岗位、菜单或按钮。Customer Overlay 把站点角色映射为候选 Capability；Project Assignment Policy 只提供资源责任事实和精确 Scope；IAM／Policy 组合后签发 `aud=business-engagement` 的短期凭证；应用结合当前状态和 revision 作最终授权。

Gateway 只负责入口 Token 边界、Header 清洗、静态路由和基础限流。Nginx 只负责 HTTPS 边缘。身份超级管理员默认无业务动作，OA 业务管理员也必须通过显式 Overlay Grant 才能获得动作。前端 `allowed_actions` 只改善体验，服务端每个命令必须复验。

第一版 Capability Manifest 只发布已实现的 create、read、适用性、方式、Evidence、取消和历史七个动作；下游建议保留为 P1 契约候选，在对应接口和幂等确认语义实现前不进入可签发 Manifest。IAM Catalog 会加载并校验该 Manifest，目标应用 IAM Provider 只接受 `aud=business-engagement` 及其中的原生 action，明确拒绝参考 OA 的粗粒度 `manage_business_info`。

已实现的 API 边界为 `GET/POST /api/v1/business-engagements`、读取单条、设置适用性、选择方式、登记／移除材料、取消和读取历史。入口适配器删除外来身份 Header，验证 action 后再按 tenant、Project 或 Business Engagement scope 限制资源；Service 与 PostgreSQL Store 继续执行引用校验、状态机、expected revision、幂等和不可变历史。Project／Evidence HTTP Port 同时支持模板环境静态凭证与正式环境精确资源工作负载凭证。可执行 server 在正式 IAM 模式中禁止 `PROJECT_ACCESS_TOKEN`／`EVIDENCE_ACCESS_TOKEN`，使用独立 Ed25519 工作负载身份按每个稳定 ID 向 IAM 交换 `project-workspace.read + project:{id}` 或 `evidence-service.view + file:{id}`，并在目标 `401` 后只失效该资源的缓存凭证、最多重换一次。Project 与 Evidence 继续复验目标 audience、action 和资源 ID；Evidence 上传仍只允许 tenant scope，不能借只读 File 凭证上传或覆盖对象。

## 组合和部署

Business Engagement 是 Suite Manifest 中可选的业务应用：

- 独立产品最小组合允许 IAM／模板 Provider、Project、Evidence 和 Business Engagement；
- Party、Contract、Money 与 Billing 根据目标旅程显式启用，不构成承接材料闭环的隐藏启动前提；
- D1 使用独立数据库／账号／migration，内部端点配置注入；D2～D4 复用同一镜像、API 和数据边界；
- Workbench 现在发布 Business Engagement 入口；经营承接工作区通过公开 API 读取列表、详情、材料和历史，并只根据服务端 `allowed_actions` 显示动作。Project、Engagement、Contract 和 Payment 的后续连续界面仍只能只读聚合，不保存可写经营进度；
- 备份恢复、单应用停机和重新启用不得改变 Project、Evidence 或下游应用身份。

当前 D1 已实现该装配：`ENABLE_BUSINESS_ENGAGEMENT=true` 显式要求 Project 与 Evidence，同步生成 `/business` Gateway 路由、启动独立 migration／server／workload-key 生命周期，并把 Business Engagement database 纳入联合备份和空目标恢复命令；应用只加入内部 `app`／`data` 网络，不发布宿主端口。构建、pull-only、promotion 和 release bundle 清单已纳入同一不可变镜像键。隔离 D1 已从 Nginx 单入口完成健康、角色、Scope、撤权和工作负载凭证运行验收，又以联合备份、销毁全部专属卷、恢复原 ID／revision／历史／引用和恢复后继续写入完成本地恢复门禁；pull-only 外部 Ubuntu、离机副本和真实公网仍是后续门禁。

## Base 与 Customer Overlay

Base 第一版只包含三个已有证据的方式、三类材料 key、适用性区别、revision／历史／幂等和建议边界。客户差异按以下顺序承载：

1. 显示术语与启用选择：配置；
2. 材料清单、必需条件和判断规则：版本化策略；
3. 行业特有的独立生命周期：类型化扩展；
4. 页面顺序、角色映射和所选应用：Customer Overlay／Suite Manifest；
5. 一次性迁移或报表：隔离客户专项。

客户规则不得直接覆盖 Base 状态语义、历史、鉴权或数据所有权。重复规则只有在两个稳定场景中语义一致后才提升进 Base。

## 反证与复审条件

出现以下任一直接证据，应重新评估独立应用边界：

- 经营承接在多个真实场景中始终只是 Project 创建时不可变的单字段，且不存在独立纠错、材料或失败状态；
- Contract 在所有场景中必然先于承接材料存在，并且法律上必须拥有全部前置事实；
- 独立应用无法形成不依赖 Contract／Money 的主要业务闭环；
- 实现只能依靠共享表、跨应用 ORM、同步双写或客户核心分支才能完成。

当前证据与上述反证相反，因此采用独立应用。第一条代码、migration、可执行进程、目标应用 IAM 凭证验证、精确 Project／File 工作负载交换、PostgreSQL 18 真库闭环、正式 D1 的 Customer Overlay／多目标 Assignment Policy／Nginx→Gateway→IAM 角色矩阵，以及 Workbench 客户工作区和浏览器撤权刷新已通过；C1 在同一多 Policy IAM 上仍通过 152 条回归。Business Engagement 的联合备份、销毁隔离卷后的空目标恢复、原事实保持和继续写入也已直接通过；95 条扩展断言又证明 409 无写入、Evidence 503 无部分写入及依赖恢复后以新命令单次成功。跨租户、账号停用、完整 IAM／Policy 故障、真实浏览器最终故障点击、下游建议和真实环境仍未通过，因此不能给出 C2 或整体架构完成结论。

## 未决实现问题

- 是否允许第一版在 Party Directory 未实现前保存“名称 + 无 Party Ref”的临时业务快照；
- 明确承接结果、未中标／流标／终止／重开和多轮语义；
- `ready` 后由谁、以什么证据确认可提出 Contract 建议；
- 专家费是 Business Engagement 的费用建议、独立报销来源，还是其他应用的责任；
- 客户材料策略的 Schema、版本升级和既有记录重算规则；
- 是否需要多 Engagement／Project，参考 OA 当前只证明一对一。
