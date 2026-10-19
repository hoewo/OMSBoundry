# OA C2 经营承接责任边界审计（2026-09-27）

## 判定

- 审计问题：C2 的经营承接应由独立应用、Project 扩展还是 Contract 前置状态拥有。
- 责任边界判定：`Business Engagement` 作为独立应用最符合当前证据。
- 本轮判定：`supported-as-target-boundary`；随后第一条后端纵向闭环已实现并通过真库与权限子矩阵，但完整 C2 尚未通过。
- OA 完整体阶段判定：`insufficient-evidence`。
- 下一实现切片：Business Engagement 第一版最小闭环；未中标／流标、多轮承接、自动建合同和专家费责任保持未知。

## 问题 A → 可验证结果 B

**问题 A**：参考 OA 已经能处理承接方式和投标材料，但代码把适用性、材料、项目、合同和资金放在同一数据库中；如果按表位置直接复制，会把遗留单体耦合误当业务边界。

**可验证结果 B**：用确认需求、模型、服务、路由和页面交叉验证事实生命周期，比较三种所有权方案，得到能被后续实现和真实场景证伪的责任边界，并明确不知道什么。

## 假设

- H1：经营承接有独立生命周期，应成为独立 Business Engagement 应用。
- H2：经营承接只是 Project 自身字段，应作为 Project 类型化扩展。
- H3：经营承接只是合同形成前的一段状态，应由 Contract Ledger 拥有。

反证条件不是“新应用开发较多”，而是业务事实没有独立修订／完成语义，或必须由 Project／Contract 自身不变量才能解释。反之，遗留表在同库或页面位于项目工作区不能单独证明所有权。

## 审计方法

1. 读取最新确认需求，找适用性、承接方式、材料、完成、恢复和下游顺序；
2. 读取现有模型和 migration，记录当前存储位置，但不把位置直接等同于业务责任；
3. 读取服务计算，确认哪些事实可以在合同不存在时成立；
4. 读取路由与权限，识别当前粗粒度权限和目标最小动作差异；
5. 搜索失败、流标、未中标、终止、多轮和自动建合同，缺失时保持未知；
6. 用 Project、Contract、Money、Evidence、Party 和 IAM 已接受边界逐项做反证检查。

## 直接证据

| 证据 | 观察 | 对边界的影响 |
| --- | --- | --- |
| `OA_2026_REQUIREMENTS_CONFIRMED.md:92-96` | 阶段不适用可以恢复，恢复后重新参加进度计算 | 这是可修订业务事实，不是页面开关 |
| 同文件 `143-159` | 方式三选一；不同方式决定材料；阶段不适用与材料不要求不同 | 有独立规则和完成条件 |
| 同文件 `161-165` | 合同和支付明确排在承接之后 | Contract 不应拥有合同前承接事实 |
| `backend/internal/oa2026/models.go:51-87` | 适用性与方式分别持久化 | 现有实现已承认两种不同事实 |
| `backend/internal/oa2026/service.go:470-543` | 仅凭适用性、方式和材料即可计算承接完成 | 承接闭环不依赖合同存在 |
| 同文件 `689-703` | 承接、合同、支付完成随后才被聚合为经营进度 | Project／Workbench 适合投影，不适合拥有全部来源 |
| 同文件 `841-879` | 单体服务直接跨读 Contract 和 FinancialRecord | 是迁移时必须消除的耦合，不是目标方案 |
| `backend/internal/models/bidding_info.go:9-28` | 三类 File ID 数组挂在 Project 关联模型 | 证明当前技术位置，不能证明 Project 所有权 |
| `backend/internal/router/router.go:153-165,301-335` | 同一 `manage_business_info` 覆盖方式、材料和专家费 | 目标权限需要按稳定业务动作拆分 |
| 全仓关键字搜索 | 未找到未中标、流标、承接结果、多轮或自动建合同的明确实现 | 这些规则不能被当作已确认 Base |

## 方案比较

| 方案 | 独立闭环 | 对既有边界的影响 | 未来定制风险 | 判定 |
| --- | --- | --- | --- | --- |
| 独立 Business Engagement | 可在无 Contract／Money 时完成适用性、方式、材料和历史 | 保持 Project 薄、Contract 法律事实、Money 资金事实 | 材料策略和行业类型可加法扩展 | 采用 |
| Project 类型化扩展 | 技术上可实现 | Project 将拥有经营流程、Evidence 含义和客户材料规则 | 客户差异持续扩大 Project 内核 | 不采用为事实所有者；只保留 Link／投影 |
| Contract 前置状态 | 合同不存在或承接不适用时语义别扭 | Contract 被迫拥有投标和失败前置事实 | 独立合同产品被 OA 流程污染 | 不采用 |

## 目标边界

Business Engagement 拥有 Engagement ID／revision、Project 引用、适用性、承接方式、材料策略版本、Evidence 业务引用、缺口／完成投影、命令幂等和历史。它可以向 Contract／Money 提出幂等建议，但目标应用独立确认。

Project 只拥有项目身份、Assignment、Package 和 Resource Link；Contract 只拥有合同法律事实；Money 只拥有已确认义务与实际资金；Evidence 只拥有文件技术事实；Party 拥有当前主体档案；IAM／Policy 签发有界能力，Business Engagement 最终授权。

第一版 `ready` 只表示材料条件满足，不等于中标或签约。整体经营进度是跨应用只读投影，不能通过跨库联表或可写总状态恢复遗留耦合。

## Base／Overlay 变化账

| 层 | 当前结论 |
| --- | --- |
| Base | 三种承接方式、阶段不适用、三类材料 key、方式到材料要求、revision／历史／幂等、下游建议边界 |
| 配置 | 页面术语、显示顺序、是否在工作台展示 |
| 版本化策略 | 客户材料清单、条件和判断；不能改变所有权 |
| 类型化扩展 | 经证据确认、具有独立生命周期的行业承接类型 |
| 连接器 | Engagement → Contract／Money 的幂等建议与确认 |
| Customer Overlay | 角色到 Capability、Suite Manifest、页面装配 |
| 客户专项 | 一次性历史数据迁移或特定报表 |

当前没有把客户名称、岗位或参考 OA 表结构写入 Base。跨客户有效性仍未证明；第二个真实场景可以推翻材料 Base 或状态假设。

## IAM 验证设计

实现不得沿用一个粗粒度 `manage_business_info` 覆盖全部写操作。候选动作至少区分创建、读取、适用性、方式、材料、建议、取消和历史。

第一版必须通过：

- 项目经营负责人对 Project A 允许、Project B 拒绝；
- 经营人员只能取得明确日常动作，高风险建议／取消默认拒绝；
- 生产负责人、财务和总经理只按 Overlay 获得明确读写范围；
- 身份超级管理员没有隐式经营业务权；
- 账号停用、Assignment 结束、Grant 撤销、策略／事实 revision 变化后的新凭证拒绝；
- 错误 issuer、tenant、audience、action、scope、签名、TTL 与伪造 Header 失败关闭；
- `ready`、`not_applicable`、`cancelled` 状态仍由目标应用限制动作；
- 401／403／409／503 不产生 Engagement、历史、材料引用或下游建议的部分写入。

## 未知和限制

- 参考 OA 没有明确表达未中标、流标、终止、重开或多轮承接；
- “中标材料齐备”与“经营确认承接成功”的法律／业务差异尚未确认；
- 专家费目前借用 FinancialRecord，不能据此判断未来由报销、费用来源还是 Engagement 自身拥有；
- 第一版是否允许无 Party Ref 的名称快照需和 Party 实现节奏共同确认；
- 当前只证明每 Project 一个 Engagement 的参考行为，不能宣称多轮或多记录不需要；
- 边界审计之后已实现 `backend/services/business-engagement` 的模型、migration、Service、HTTP API、原生 Capability Manifest、IAM／模板 Provider 与 Project／Evidence Port；该增量不改变本审计保留的业务未知。
- PostgreSQL 18 `-race` 验证已覆盖创建、幂等、冲突、并发 revision、材料补齐／替换／移除、`ready`、取消和不可变历史；HTTP／IAM 测试已覆盖无凭证、缺 action、Project B scope、错误 audience、遗留粗 action 和伪造 Header 拒绝。
- 尚无客户页面、完整角色／撤权矩阵、正式 Nginx→Gateway→IAM D1、备份恢复、下游建议或公司／个人真实环境证据，因此没有 C2 完整通过结论。

## 下一轮可执行入口

下一轮从已经通过的后端闭环继续收口：补恢复适用与引用故障用例，接入可执行 release／正式工作负载凭证，完成最小客户页面和更完整的 OA 角色／撤权矩阵。Contract／Money 建议仍作为其后的独立切片，不用假实现阻塞当前闭环。
