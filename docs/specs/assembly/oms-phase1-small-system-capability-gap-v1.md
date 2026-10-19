# OMS 一期小系统能力映射与缺口矩阵 v1

日期：2026-10-11  
状态：BF-145 实现门禁；BF-150 已完成一期六 Owner 公网纵向验收  
关联：[OMS 一期 MVP 产品设计 v1](./oms-phase1-mvp-product-design-v1.md)、[小系统边界演变账](./small-system-boundary-evolution-ledger.md)

问题 A → 可验证结果 B：OMS 一期页面已经明确，但页面名不等于后端事实边界 → 在开发前把每项产品能力映射到唯一事实 Owner、现有版本化端口、局部兼容扩展或停止条件，避免为赶页面另建共享 OMS 数据库、跨库写或第二套金额真相。

## 1. 总结结论

一期产品可以继续在现有小系统上开发，不需要全盘推翻 Project、Party、Contract、Billing、Money、Evidence、IAM、Gateway 或 Workbench。现有能力已经覆盖身份登录、项目身份、主体当前档案、合同承诺、发票事实、实际资金、附件和精确金额这些核心事实。

但“页面全貌存在”与“真实业务闭环已经可用”必须分开：

- `/oms-v1/` 入口、信息架构、响应式页面、列表／抽屉和 Owner 适配器属于装配层局部扩展；Party 主体创建、Project `contractor-party` 关系、Billing 双方向票据、Money 实际收付款和 Evidence 引用均沿各自 Owner 原生命令接线，不需要 OMS 业务表；Billing 写入由显式 `OMS_QUALIFICATION_PARTY_ID`、Owner `allowed_actions` 和精确 Party／Project revision 共同失败关闭，正式 IAM 与公网实写已由 BF-150 验证；
- Money 已在原 Owner 内提供适合 OMS 的跨项目 JSON 流水列表，并在 reference projection 显式标注 Party 快照角色；Project 以精确 Party revision 建立 `contractor-party` Resource Link，在受授权项目目录返回当前有效关系，并通过 `commercial-summary` 瞬时组合。上述均不落 Project 表，属于 B 级兼容扩展。Party 当前档案仍只承诺一期已确认字段；上下游 Contract 金额编排已完成公网纵向验证；
- Contract、Money 的历史本地主体已在完整备份后逐项审计、显式映射并切换为 Party Directory runtime；重复执行无变化，Workbench 不按名称拼主体，也没有 OMS 主体表；
- Money 现有义务要求 `due_on`，OMS 一期明确没有固定期限，因此不得填写假日期来复用义务／逾期模型。应收、应付总额先由 Contract 承诺投影，实际收付由 Money 持有。

当前边界影响判定：**整体不是超级大改，可以按既有 Owner 组合实现；主体运行时统一和无固定期限义务模型是两个必须单独评审的显著边界事项。**

## 2. 分级口径

| 等级 | 含义 | 本轮处理 |
| --- | --- | --- |
| A 直接复用 | Owner、公开契约与产品语义基本一致 | 接入并做真实读写验收 |
| B 局部兼容扩展 | Owner 不变，只增加版本化端口、字段、投影、Overlay 或 Workbench 适配 | 可实施，但必须单测、集成和恢复验证 |
| C 显著边界变更 | 涉及事实迁移、强制依赖切换或既有契约大面积重解释 | 先记账并单独评审，不随页面隐式实施 |
| X 禁止方案 | 形成共享表、跨库写、双写、第二套金额真相或虚构业务事实 | 不实施 |

## 3. 产品能力到事实 Owner 的映射

| 产品能力 | 唯一事实 Owner | 当前端口／事实 | 判定 | 一期实现约束 |
| --- | --- | --- | --- | --- |
| 登录、会话、能力授权 | IAM | 既有登录、Session、Capability Credential | A | `/oms-v1/` 不使用 `preview=oa`，未登录进入 IAM；目标应用继续最终授权 |
| 公网入口 | Nginx、Gateway | Nginx 统一边缘代理；Gateway 静态路由和可信身份 Header | B | Gateway 新增 `/oms-v1/`，303 到受保护 Workbench；旧 `/oms/` 保留 |
| 四入口与页面编排 | Unified Workbench | 已有应用注册表、换证与受保护页面 | B | Workbench 只保存瞬时 UI 状态和可恢复命令意图，不保存项目、金额、主体或附件事实 |
| 项目名称、开启日期、甲方引用、revision | Project Workspace | `GET/POST /api/v1/projects`、`PUT /api/v1/projects/{id}`、详情／history | A+B | Workbench 已接基本信息编辑，提交 exact revision 并原样保留 Owner 项目号／类型；产品“项目开启日期”映射 `planned_start_date`，内部项目号由有界编排稳定生成且不作为主要页面字段 |
| 项目甲方当前档案 | Party Directory | `GET/POST/PUT /api/v1/parties`、status、history | A | Project 冻结发生时 Party ID／revision；当前档案仍归 Party；Workbench 已接 `POST /api/v1/parties`，只创建主体档案，不把新主体自动认定为甲方／施工方 |
| 甲方／施工方联系人、电话、银行、地址、备注 | Party Directory | 当前仅 kind、legal_name、registration_identifier、status、revision | B | 一期首个闭环只承诺名称和现有字段；补充档案必须在 Party 内做版本化兼容扩展，不能塞入 Workbench |
| 甲方主合同、补充附件、应收约定总额 | Contract Ledger＋Evidence | Contract workspace、变更／材料；Evidence File／Version | A+B | Contract 持有承诺和精确金额，Evidence 持有字节；Project 只保存 Contract Link；“项目编辑”是编排入口，不代表金额归 Project |
| 施工方关系、施工协议、每施工方应付总额 | Contract Ledger＋Project Link＋Party | Project `contractor-party` Link／目录投影与 Workbench 单关系写入已实现；Contract Parties／总额与 Evidence 已存在 | A+B | Workbench 通过 Project 原生 `resource-links` 命令写稳定 Party ID／revision；每个“项目＋施工方”仍须映射一份下游合同承诺，不能只按主体名称猜测 |
| 应收／应付汇总 | Contract＋Billing＋Money 的只读组合 | Project commercial summary 组合 Contract 承诺、Money 精确资金、显式 Party 角色和 Link 时间 | B（BF-150 已验） | 上下游 Contract 总额和实际收付独立保存并按项目组合；任一 Owner 不可用时显示 unavailable，不显示 0，不把开票自动生成应收 |
| 我方向甲方开票 | Billing Invoice | invoice registration、correction、invalidation、evidence、history；Workbench 新增表单已接 `POST /api/v1/invoice-registrations` | A | 仅在部署显式配置资质单位稳定 Party ID、主体可读且启用、Owner 返回 `record` 时开放；issuer／recipient 冻结 Party revision，以 Project source ref 区分项目，金额在浏览器转为精确整数分后提交 |
| 施工方向我方开票 | Billing Invoice | 同一 Invoice 模型；Workbench 使用项目 `contractor-party` Link 限定施工方 | A+B（BF-150 已验） | 用 issuer／recipient 方向识别收票，不新建“收票表”；未关联／停用施工方不可选择；正式 IAM 与公网实写已通过 |
| 实际回款 | Money Flow | cash transaction create/get/reversal、精确金额字符串；`GET /api/v1/cash-transactions` | A+B（列表已实现） | Money 独占实际资金；列表受 IAM、Tenant Scope 和金额能力收敛，不读取 Money 内部表 |
| 实际支付 | Money Flow | 同一 Cash Transaction 模型和 JSON 列表 | A+B（列表已实现） | 以交易方向和主体快照区分支付；错录通过反向记录／现有 reversal 语义纠正，不物理删除 |
| 无固定期限应收／应付 | 暂无合适的正式 Owner 契约 | Money obligation 当前要求 `due_on` | C | 一期不得造假期限；先由 Contract 承诺＋实际资金投影余额。后续另设计可选期限义务，不原位放宽既有账龄语义 |
| 发票／资金附件 | Evidence Service＋业务引用 Owner | File／Version、短期访问地址；Billing evidence refs | A+B | 字节只归 Evidence；Money 若要持凭证引用需增加 Money 本域版本化引用，不把 File JSON 塞 Workbench |
| 金额修改、作废与审计 | 各事实 Owner | revision、correction／invalidation／reversal／history | A+B | UI 可简化原因，但不能绕过 Owner 的 expected revision 和审计；未知回复重放同 command |
| 图表、余额和经营总览 | 各 Owner 只读投影＋Workbench 瞬时组合 | Money metric、Project commercial summary 等 | B | 统计口径必须由 Owner 投影或有版本的装配契约给出；Workbench 不持久化汇总 |
| 分页、筛选、Excel 导出 | 各 Owner 列表／Workbench 瞬时导出 | Workbench 使用 Owner 原生 cursor／offset 连续读取完整授权集合，再做 20／50／100 条 UI 分页和紧凑筛选；六类列表把当前筛选后的完整已授权集合导出为 Excel 兼容 CSV，金额两位小数且不受 UI 分页限制 | B（BF-150 已验） | 205 条 offset、201 条 cursor、重复页／游标失败关闭单测通过；公网真实筛选集合通过，本地开票筛选 CSV 已核对。第一版没有要求原生 `.xlsx`，不得把 CSV 描述为 `.xlsx` |

## 4. 四条业务链的装配方式

### 4.1 新建项目

1. Workbench 从 Party Directory 读取活动甲方；
2. 向 Project Workspace 创建项目，冻结甲方 ID／revision；
3. 向 Contract Ledger 创建甲方主合同，合同总额即应收约定总额；
4. 向 Project 写入该 Contract 的 Resource Link；
5. 用户选择施工方时，为每个施工方建立独立下游合同并链接到 Project；
6. 附件先写 Evidence，再把 File／Version 交给 Contract 保存业务含义。

这是多 Owner 的可恢复编排，不是跨库事务。每一步使用稳定 command ID；前一步成功、后一步失败时保留已确认事实，只续未完成段，不重建项目或合同。

### 4.2 开票与收票

1. 必须先选项目；
2. 开票自动使用项目甲方作为 recipient，我方为 issuer；
3. 收票只能选择项目已关联施工方，施工方为 issuer，我方为 recipient；
4. Billing 冻结主体 revision、Project source ref、业务日期、金额、备注和 Evidence 引用；
5. 发票累计独立统计，不自动产生应收、应付或资金事实。

### 4.3 回款与支付

1. 回款必须选择项目并自动带出甲方；支付还必须选择项目施工方；
2. Money 保存 actual Cash Transaction、发生日期、方向、精确金额和来源引用；
3. OMS 汇总读取有效交易，不复制交易；
4. 录错关联不能在页面层换项目，应使用 Money 的可审计纠错语义后重新登记。

### 4.4 总览与项目详情

总览先读取 Owner 提供的有限聚合，再按 Project ID 和稳定资源引用组合。项目详情展示 Project 自身资料、甲方／施工方合同承诺和四类事实的汇总，不在详情复制明细列表。来源超时、无权或版本不兼容分别显示“暂不可用／无权限／需要升级”，不能冒充金额为零。

## 5. 当前显著边界事项

### 5.1 主体目录运行时切换

Project、Billing、Contract、Money 均已在 OMS 一期正式 D1 使用 Party Directory 稳定主体引用。Contract、Money 的历史本地主体仍作为迁移前事实保留，但运行时已通过受验收的显式映射切到 Directory，不再作为当前主数据来源。

本轮禁止：

- Workbench 通过名称查找并猜测 Contract／Money 本地 Party；
- 创建一套 OMS 专用甲方／施工方表；
- 每次编辑同时双写 Party Directory、Contract 本地 Party 和 Money 本地 Party；
- 对历史事实静默替换 Party ID。

BF-150 已完成上述切换：升级前形成完整远端备份 `/var/backups/omsbizfoundry/20261012T175912Z`；脚本分页读取全部活动 Directory 主体，按登记标识、受限历史夹具后缀或唯一精确名称形成映射，歧义失败关闭，其余主体显式新建；Contract 4 条、Money 5 条本地主体均被映射，两域切到 `directory`，第二次执行均为 `changed:false`。历史业务快照保持不变，回滚边界为备份与应用 runtime 状态，而不是名称猜测或双写。

### 5.2 无固定期限义务

现有 Money obligation 的 `due_on` 参与账龄与逾期语义，不能为了 OMS 页面传入项目开启日、当前日或任意远期日。后续兼容方案应显式支持“未设置期限”，并同时更新 API、数据库约束、统计、导出、恢复和旧数据解释；这不是本轮页面实现可以顺手修改的字段。

### 5.3 项目与合同的编排一致性

产品把“项目编辑”作为用户操作入口，但应收／应付金额仍归 Contract。Workbench 对 Project、Contract 和 Link 的操作是持久子命令序列。若业务要求用户看到一个保存按钮，应显示各段状态并支持续办；不得把 Contract 金额复制到 Project 表以获得表面原子性。

## 6. 本轮允许实施的最小纵向闭环

按以下顺序推进，任何阶段均不得用虚构后端结果冒充通过：

1. 新增 `/oms-v1/`，验证 Nginx→Gateway→IAM→Workbench；
2. 建立四入口、响应式导航、总览骨架、统一列表／筛选／分页／抽屉交互；
3. 接入 Party Directory 和 Project 的真实读取；主体档案创建写 Party，项目基本信息编辑写 Project 原生 `PUT`，项目关联施工方写 Project 自有 `contractor-party` Link，完成真实项目列表／详情且不复制主体主档；
4. Billing 双方向登记与公网真实写入已通过；资质单位继续由 `OMS_QUALIFICATION_PARTY_ID` 显式注入，不按名称猜方向；
5. Money 回款／支付写入与复读、Project commercial summary、Owner 完整分页／筛选已通过；
6. 主体 runtime cutover、项目创建和 800 万／268 万上下游合同编排已通过；
7. 六份附件的 Evidence 上传、Owner 引用、短期授权下载和字节哈希已通过；经营图表与 CSV 继续只组合授权后的 Owner 结果。

页面在能力尚未接通时显示明确空态“此能力尚未接通”，不能使用看似真实的演示金额。`preview=oa` 仍只属于旧参考 Demo，不进入一期验收路由。

## 7. IAM 能力映射初稿

“查看／维护”是产品层两档能力，不是一个能绕过各 Owner 的万能角色。IAM Grant 最终应展开为目标应用原生 Capability：

| 产品档 | 需要的原生能力族 |
| --- | --- |
| 查看 | `project-workspace.read`、`project-workspace.commercial-summary.read`、`party-directory.read`、`contract-ledger.view`、`billing-invoice.read`、`billing-invoice.amount.read`、`money-flow-ledger.view`、`money-flow-ledger.amount.read`、适用的 Evidence 读取能力 |
| 维护 | 查看能力＋Project create/update/resource-link、Party create/update/status、Contract draft/lifecycle/evidence、Billing record/correct/invalidate/evidence、Money cash.record/cash.reverse、Evidence upload/complete 等原生能力 |

Workbench 只向 Gateway／IAM 请求当前动作所需的最小原生凭证；Gateway 删除客户端伪造身份 Header，目标应用结合 Tenant、Scope、资源状态和 expected revision 作最终授权。

## 8. 验收证据要求

1. 路由、静态资源和 API 全部通过公网 Nginx／Gateway；
2. 未登录、只读账号、维护账号、撤权后四种访问行为有实际证据；
3. Project、Party、Contract、Billing、Money、Evidence 各自数据库保持唯一写入；
4. 任一 Owner 停机时页面显示来源不可用，其他事实仍可读，Workbench 没有复制值兜底；
5. 跨 Owner 命令丢失回复后使用同 command 续办，无重复项目、合同、发票、资金或文件；
6. 金额使用整数最小货币单位或精确十进制，列表、详情、图表下钻和 Excel 对账一致；
7. 空目标恢复后 ID、revision、金额、主体快照、文件引用和审计历史保持；
8. 只有实际运行过的范围才能登记为通过；本文件的完成结论以 BF-150 报告中的公网证据为限。
