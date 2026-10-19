# 讨论过程索引

本目录保留 OMSBizFoundry 的讨论过程。每轮文档记录当时的问题、依据、候选设计和开放项；后续开发的强制约束统一登记在[开发决策总表](../decisions.md)，整体自然语言解释以 [`../foundation.md`](../foundation.md) 为准。

| 轮次 | 主题 | 解决的主要问题 |
| --- | --- | --- |
| [第 1 轮](./round-01-contract-income-expense-foundation.md) | 总体业务与模块拆分 | 从参考项目中提取稳定业务主线，分开合同、收支和审批 |
| [第 2 轮](./round-02-contract-kernel.md) | 合同内核 | 定义主体快照、不可变文书、revision、合同关系、变更和结算 |
| [第 3 轮](./round-03-money-flow-kernel.md) | 经营收支内核 | 定义义务、票据、实际资金、多对多核销和统计口径 |
| [第 4 轮](./round-04-composable-foundation-blueprint.md) | 可组合小系统蓝图 | 定义数据所有权、公共端口、定制分层和演进方式 |
| [第 5 轮](./round-05-small-system-productization.md) | 小系统产品化与耦合约束 | 明确不做通用平台，要求小系统独立闭环、默认可用和可选组合 |
| [第 6 轮](./round-06-development-plan.md) | 开发计划与里程碑顺序 | 确定工程基线、两个小系统、连接器和交付验证的推进顺序 |
| [第 7 轮](./round-07-minimum-cause-principle.md) | 最小根因与总计划校正 | 用一句好懂的 A→B 约束项目、应用、板块、功能和技术机制 |
| [第 8 轮](./round-08-mandatory-iam-and-permission-decoupling.md) | 强制 IAM 契约与权限解耦 | 要求所有受保护访问经过同形 IAM 上下文，并用稳定业务能力隔离客户角色与页面差异 |
| [第 9 轮](./round-09-application-portfolio-and-single-node-architecture.md) | 应用组合与单节点架构 | 确定五个应用、三类装配组件、数据所有权和单台 ECS 最低拓扑 |
| [第 10 轮](./round-10-complete-suite-and-financial-source-boundaries.md) | 完整组合、人员管理与收支来源边界 | 将 Gateway 纳入正式组合，增加人员目录，明确登录注册、应用开关以及报销／奖金与台账的边界 |
| [第 11 轮](./round-11-people-groups-and-configurable-capabilities.md) | 人员编组与可配置能力 | 将施工队抽象为以 Group 为核心的 People & Groups，拆分成员、角色、能力画像与具体项目／财务事实 |
| [第 12 轮](./round-12-gateway-iam-and-nebula-auth-fit.md) | Gateway、IAM 与参考认证工程适配 | 明确登录注册归 IAM、Gateway 负责统一接入和基础验签，并评估 NebulaAuth 可借鉴与不可直接采用的部分 |
| [第 13 轮](./round-13-permission-chain-decoupling.md) | IAM、业务事实与小应用权限链解耦 | 将权限从强制调用链改为 Capability Manifest、业务事实快照、短期能力凭证和 allowed_actions 四份版本化契约 |
| [第 14 轮](./round-14-fixed-entry-chain-and-oss-evidence-service.md) | 固定入口链与 OSS Evidence Service | 确认 Nginx → Gateway → IAM／业务应用，并把 OSS 上传下载从业务代码收敛为独立 Evidence Service |
| [第 15 轮](./round-15-deployment-profiles-not-single-node-limit.md) | 单台 ECS 是最低档，不是架构上限 | 将应用开关与节点放置分开，确认一台、两台、三台和高可用多节点部署档 |
| [第 16 轮](./round-16-party-directory-and-dependency-boundary.md) | Party Directory 与独立交付依赖边界 | 用两个台账和参考 OA 证据确认 Party 独立应用，并把“独立”校正为显式、可验证依赖而非零依赖 |
| [第 17 轮](./round-17-base-overlay-and-full-oa-feasibility.md) | 稳定 Base、客户覆盖层与 OA 完全体可行性 | 把“尽量只做加法”收敛为可证伪的产品线方向，并区分 OA 完全体的架构可行性、当前覆盖和跨客户证据 |
| [第 18 轮](./round-18-oms-phase1-industry-research-scope.md) | OMS 一期行业调研范围与结构缺口 | 固定一期只做资质单位对外经营，并用官方行业资料识别履约、结算、质保、协作和工资链的候选缺口 |

## 使用方式

1. 开发前先读 `decisions.md`，继续讨论前再读 `foundation.md`，不必重复阅读所有历史轮次。
2. 需要理解某项结论为什么形成时，再回到对应轮次。
3. 新讨论形成或改变稳定共识时，新增轮次并同步更新 `decisions.md`、`foundation.md` 及受影响的投影文档。
4. 未进入基线的内容仍是候选意见，不作为工程实现承诺。
