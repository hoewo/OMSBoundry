# OA C6 成本与外委责任边界审计（2026-09-28）

## 判定

- 审计问题：生产成本、外委事项、外委合同、付款、对方票据和评分应该由一个宽财务表拥有，还是由窄域来源应用与 Contract／Money／Billing 分工。
- 责任边界判定：建立独立 **Cost & Outsourcing（成本与外委）** 应用，在一个可独立交付边界内使用两个互不混写的聚合：`Cost Entry` 与 `Outsourcing Engagement`。它拥有“为什么发生”和当前业务登记；Contract、Money Flow、Billing、Party Directory、Evidence 继续拥有各自正式事实。
- 本报告原始判定：`boundary-supported / implementation-not-started`；后续已完成领域核心、受保护后端、人类工作区、正式 D1、Party 切换和 C6→Money Payable Suggestion 真实双进程旅程。当前为 `real-cross-process-payable-suggestion-supported / formal-d1-complete-dual-tenant-real-ubuntu-pending`，详见 [C6→Money 应付建议实验](./oa-c6-payable-suggestion-experiment-20260928.md)。
- OA 完整体阶段判定：`insufficient-evidence`。

## 问题 A → 可验证结果 B

**问题 A**：参考 OA 把成本、奖金、甲方付款、我方开票、专家费、外委付款和供应方票据塞进同一 `FinancialRecord`，又把外委事项保存在另一张可原地编辑和物理删除的表中。它能完成页面操作，却无法稳定回答“成本登记是否等于已付款”“外委金额是否等于合同金额”“删除外委为什么可以删掉付款历史”。

**可验证结果 B**：固定一个能被实现证伪的 C6 边界和 C6-A01～A18 验收矩阵；下一实现轮可以只实现来源事实、精确引用、纠错与建议交接，而不复制宽财务表、不直接写其他应用数据库，也不假称已经完成现实审批或付款。

## 审计方法

1. 对照参考 OA 的需求、规格、前端服务、后端模型、路由、服务和权限目录；
2. 把页面位置、单体外键和“财务记录”名称与真实事实所有权分开；
3. 分别追踪费用发生、外委关系、法律合同、应付义务、实际付款、对方票据、评分和文件；
4. 查找金额精度、revision、纠错、撤回、幂等、权限、故障和历史行为；
5. 比较一个窄域双聚合应用、两个应用、Production 内置和 Money 宽表四种方案；
6. 对证据不足的审批、报销、会计和项目完成规则保持未知。

## 直接证据

| 证据 | 观察 | 对目标边界的影响 |
| --- | --- | --- |
| 参考 OA `需求梳理.md` 与 `arckit/spec/production/cost.md` | 成本最小结果是打车、住宿、公共交通的时间／里程／金额和发票 | 支持独立 `Cost Entry`；它描述发生原因，不等于现金付款 |
| `arckit/spec/production/commission.md` | 外委包含个人／单位、评分、合同、支付和索票 | 一个页面串联多个责任域，但没有证明这些事实应共表 |
| `backend/internal/models/financial_record.go` | 一个可变模型混合成本、奖金、收付款、开票、专家费、外委支付和对方票据；金额为 `float64` | 不能复制到新架构；类型字段不是稳定业务边界，金额也不满足精确表示 |
| `backend/internal/models/external_commission.go` | 外委事项另有受托方、联系人、内容、金额、评分和合同文件，但无 revision／状态／历史 | 外委本身确有独立生命周期；单体当前模型不能作为目标契约 |
| `external_commission_service.go` 创建支付／票据 | 服务直接创建同库 `FinancialRecord`，并复制供应方名称、类型和评分 | 付款和票据不应成为外委应用的第二真相；供应方应冻结精确 Party 快照 |
| 同服务 `DeleteCommission` | 删除外委会删除关联财务记录；历史兼容路径甚至按同项目受托方名称删除无显式关联记录 | 这是严重反证：业务撤回不能物理删除法律、义务、票据或资金历史，名称不能作为身份 |
| 同服务 `GetSummary` | 支付总额直接汇总项目下 `commission_payment` | 读模型可聚合，但正式付款只能来自 Money；未知或不可用不能当成零 |
| `frontend/src/services/production.ts` | 成本只是对宽 `financial` 接口的筛选和字段转换，编辑／删除原记录 | 页面适配不证明数据所有权；目标必须提供类型化 API、revision 和追加纠错 |
| `docs/PERMISSION_CATALOG.md` | 生产经理只读成本／外委，生产负责人或财务承担不同写职责，金额必须独立判权 | C6 需要 Project Scope、动作拆分和金额／Evidence 字段级能力，不能只用“可访问项目” |
| `docs/arckit-tech-model-api-comparison-2026-03-24.md` | 参考实现长期处于旧模型、新前端、独立专家费、宽财务记录并存的混合态 | 不能把“最新表结构”误认成业务共识；应保留冲突并用窄域不变量收口 |
| 现有 BizFoundry Contract／Money／Billing／Party／Evidence | 已分别拥有合同、义务与实际资金、票据登记、主体当前档案和文件技术事实 | C6 只能用版本化端口交接或引用，不能扩大成本应用为财务总账 |

## 方案比较

| 方案 | 优点 | 主要风险 | 判定 |
| --- | --- | --- | --- |
| 一个 Cost & Outsourcing 应用、两个聚合 | 同一 Project 生产经营上下文下可形成最小产品；成本与外委仍有各自模型、命令和历史 | 需要防止“同应用”退化为一张宽表 | 第一版采用 |
| Cost 与 Outsourcing 两个应用 | 所有权最细 | 目前只有一个参考场景，立即拆分会增加装配与跨应用成本，缺少第二稳定使用点 | 暂不采用；出现独立交付或生命周期反证时复审 |
| 放入 Production Operations | 页面路径接近参考 OA | 污染 C4 的 Work Item／Stage／Deliverable 内核，引入金额、Party、Contract 与 Money 依赖 | 不采用 |
| 放入 Money Flow | 可直接看应付和付款 | Money 只应确认义务、实际资金和核销，不能拥有“为什么发生”、外委范围与评分 | 不采用 |
| 通用 FinancialRecord／费用平台 | 初期表少、页面快 | 类型分支、可变字段、粗权限和级联删除会长期耦合 C3、C6、C7 | 明确拒绝 |

## 已接受所有权

Cost & Outsourcing 拥有：

- 项目下直接成本发生的业务登记、类别、日期、精确金额、可选里程、说明、Evidence 业务引用、revision、纠正／撤回和历史；
- 外委事项的工作范围、受托方精确 Party 快照、预算／约定登记金额、可选精确 Contract 引用、状态、revision 和历史；
- 外委履约评分及其规则版本、评分人、时间、理由和历史；
- 向 Money Flow 提出的幂等“应付义务建议”及目标确认结果；
- 对 Billing 票据登记、Money 义务／付款和 Contract 的精确只读关联，不复制其当前事实。

它不拥有：

- Project 身份、Assignment 或项目完成状态；
- Party 当前档案；
- 法律合同、补充、终止或结算；
- 已确认应付义务、实际付款、核销和余额；
- 对方发票的正式登记生命周期；
- 文件字节、签名 URL 或存储清理；
- 员工报销、会计凭证、工资、奖金、任意审批流或采购平台。

## 第一版业务语义

### Cost Entry

- 第一版固定 `vehicle`、`accommodation`、`public_transport`、`other` 四类；客户显示术语可以 Overlay，但不能在线增加任意行为类型。
- 金额使用整数最小货币单位与 ISO 币种；车辆里程若填写使用整数米，不使用二进制浮点数。
- `recorded` 表示来源域确认“该成本发生登记存在”，不表示已报销、已入账或已付款。
- 纠正追加完整 revision；错录／取消使用 `withdrawn`，禁止物理删除。
- Evidence 上传失败不得丢失已经填写的表单；成功业务命令引用经过验证的精确 File ID／Version。

### Outsourcing Engagement

- 受托方使用 Party ID／revision，保存 `kind + legal_name` 最小快照；自由文本名称不能代替身份。
- 应用拥有外委工作范围和业务登记金额；正式法律关系若存在，必须引用 Contract Ledger 的精确 Contract／Settlement revision，不能上传一个文件后冒充合同事实。
- 第一版只使用 `recorded|withdrawn` 登记状态，不凭现有证据发明审批、履约、验收或采购状态机。
- 评分是追加式 Assessment，固定使用 `reference-oa-five-point-v1` 的 1～5 整数分及理由；评分后纠错保留历史，不把评分复制到付款或票据记录。
- 外委撤回不能删除 Contract、Money、Billing 或 Evidence 中的任何事实。

### 义务建议与目标确认

- Cost & Outsourcing 可以根据 Cost Entry 或 Engagement 提出应付义务建议，但建议不等于义务；
- 建议冻结来源 ID／revision、Party 快照、金额、币种、用途和请求人，使用稳定 Command ID；
- Money Flow 独立确认、拒绝或幂等返回目标 Obligation，并拥有后续付款与核销；
- 目标不可用时建议保持可重试状态，不能在本地伪造“已付款”；
- 实际付款、对方票据和未清余额只能通过 Money／Billing 的版本化最小投影读取。

## 权限边界

第一版稳定动作至少拆分为：

- `cost-entry.create|read|amount.read|correct|withdraw|evidence.manage|history.read`
- `outsourcing-engagement.create|read|amount.read|correct|withdraw|assess|evidence.manage|history.read`
- `outsourcing-payable-suggestion.create|read|withdraw`

参考 OA Overlay 可把生产负责人、财务、生产经理和总经理映射到这些动作；目标应用不认识岗位名称。金额和 Evidence 分别授权，身份超级管理员无隐式业务权限。所有写命令同时复验 Tenant、精确 Project Scope、当前状态和 expected revision，并由目标应用返回 `allowed_actions`。

## C6 第一实现轮验收矩阵

| ID | 场景 | 必须证明 |
| --- | --- | --- |
| C6-A01 | 独立所有权 | 独立数据库；Production、Project、Contract、Money、Billing 不新增 C6 可写表 |
| C6-A02 | 双聚合隔离 | Cost Entry 与 Engagement 使用不同模型／命令／历史，不退化为 nullable 宽表 |
| C6-A03 | 精确金额 | 整数最小货币单位、币种、非负／溢出检查；无 `float64` 金额 |
| C6-A04 | Cost 生命周期 | 登记、纠正、撤回追加 revision，历史不可变且无物理删除 |
| C6-A05 | Party 来源 | 精确 Party revision、同 Tenant／active／类型复验和最小快照冻结 |
| C6-A06 | Contract 来源 | 可选精确 Contract revision；错类型、stale、停机失败关闭且零部分写入 |
| C6-A07 | Evidence | 精确 File Version、多附件、故障零写入、恢复后 Command 只成功一次 |
| C6-A08 | 外委纠错 | 范围、金额、Party 或 Contract 改变形成新 revision，不改写旧快照 |
| C6-A09 | 评分 | 版本化 1～5 规则、理由、评分人和追加纠错；不复制到付款／票据 |
| C6-A10 | 义务建议 | **已通过真实双进程当前范围**：精确来源 revision／digest、目标接受／拒绝、接受唯一 Obligation、拒绝零 Obligation、停机／恢复、幂等和部分失败恢复；完整正式 D1 connector、双 Tenant 和跨进程撤回待补 |
| C6-A11 | 付款读取 | 实际付款／余额只读 Money 投影，来源不可用显示 unavailable，不显示 0 |
| C6-A12 | 票据读取 | 对方票据只读 Billing 投影，C6 不保存票据第二真相 |
| C6-A13 | 权限 | 允许角色、相邻角色、Project B、错 Tenant／Audience／Scope、金额和 Evidence 字段矩阵 |
| C6-A14 | 状态拒绝 | withdrawn 资源拒绝纠正外的非法动作；拒绝时零写入 |
| C6-A15 | 最小组合 | IAM／模板 + Project + Party + Evidence + C6 可完成成本与外委登记；Contract／Money／Billing 仅在对应动作时显式依赖 |
| C6-A16 | 未选缺席 | 未选 C6 时 C1～C5 组合无空菜单、路由、Capability 或备份项 |
| C6-A17 | 装配恢复 | Suite Manifest、Workbench、D1 入口、联合空目标恢复保持 ID／revision／摘要 |
| C6-A18 | 回归反耦合 | Level A／B、C1～C5 回归；无共享表、跨库 ORM、同步双写、名称关联删除或宽财务表 |

## Base／Overlay 变化账

| 层 | C6 第一版归属 |
| --- | --- |
| Base | 两个聚合、精确金额、revision、纠正／撤回、引用、建议交接和权限内核 |
| 版本化策略 | 成本类别集、评分规则、可提建议的状态及 Project 完成规则（确认后再装入） |
| Customer Overlay | 岗位到 Capability、显示术语、是否启用 Cost／Outsourcing、必填项 |
| Suite Manifest | 是否安装 C6 及其直接依赖和 Workbench 入口 |
| 连接器／投影 | C6→Money 义务建议；Contract／Money／Billing 只读精确投影 |
| 客户专项 | 旧宽财务表迁移、客户报表或一次性字段映射，不进入 Base |

## 明确未知

- 成本登记、员工报销、财务入账和实际付款之间的正式责任与审批链；
- 专家费应属于 Business Engagement、C6，还是单独费用申请来源；
- 外委事项在何时成为法律合同，以及无合同外委是否允许；
- 外委付款建议的审批人、撤销、部分确认和重提规则；
- 对方票据的现实红冲／作废／退票生命周期；
- 评分时点、可见范围、申诉和多次评分规则；
- 成本／外委何时能被 Project Completion Profile 判定为 satisfied 或 not applicable；
- 项目包成本归属、分摊和回退规则。

这些未知不阻止来源事实内核，但阻止我们宣称 C6 全业务完成，也阻止把 C6 条件加入完整 OA Project 完成确认。

## 下一实现顺序

1. 模型、命令与 PostgreSQL 不变量；
2. Project／Party／Evidence 精确引用和原生 IAM；
3. Cost 与 Engagement 人类页面及权限矩阵；
4. Contract 可选引用、评分和历史；
5. Money 义务建议与目标确认（真实双进程当前范围已完成；正式 D1 connector 待补）；
6. Money／Billing 只读投影；
7. D1 装配、未选缺席、故障和空目标恢复（C6 本体已完成；connector 待补）；
8. 获得真实业务证据后再决定 Project Completion 规则。

任何实现若需要共享表、宽 `FinancialRecord`、名称匹配、物理删除、客户端合计、C6 直接创建付款／票据或用户可编辑任意流程，应立即停止并复审边界。

## 本轮未执行（边界审计当时状态）

该边界审计轮没有新增 C6 应用代码、数据库、API、Capability、页面或 D1 服务，当时总进度保持 73%。后续各轮已经依次完成核心／真库、受保护后端、工作区／独立进程、正式 D1、Party 切换和应付建议真实双进程旅程；C7 边界审计后当前总进度为 90%。仍不能据此宣称完整 C6 或 OA 通过，剩余边界见本报告开头和应付建议实验限制清单。
