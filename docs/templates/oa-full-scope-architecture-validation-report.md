# OA 完整体架构可行性验证报告模板

> 使用说明：本文件是阶段最终报告的固定结构，不是通过结论。完成阶段后复制为 `docs/reports/oa-full-scope-architecture-validation-report.md`，删除提示语并填入直接证据。

## 1. 报告信息

| 项目 | 内容 |
| --- | --- |
| 报告版本 | `TBD` |
| 验证时间范围 | `TBD` |
| Base 版本／Git 提交 | `TBD` |
| Customer Overlay 版本 | `TBD` |
| Suite Manifest 版本 | `TBD` |
| Deployment Profile | `TBD` |
| 应用镜像摘要 | `TBD` |
| 验证环境 | 公司私有 Ubuntu／个人阿里云试验田／本地 |
| 证据目录 | `TBD` |

## 2. 执行结论

结论只能选择一项：

- `supported`：完整业务、架构边界、定制覆盖和至少一个真实环境均有直接证据；
- `conditionally-supported`：核心方案成立，但存在明确、有界且不会推翻主要结论的条件；
- `not-supported`：出现共享事实、核心 fork、不可恢复数据、不可接受耦合或其他架构反证；
- `insufficient-evidence`：关键旅程或真实环境证据不足，尚不能判断。

| 字段 | 内容 |
| --- | --- |
| 最终结论 | `TBD` |
| 一句话理由 | `TBD` |
| 适用范围 | `TBD` |
| 不适用范围 | `TBD` |
| 阻断项 | `TBD` |

## 3. 架构主张验证

| 主张 | 结果 | 直接证据 | 反证／限制 |
| --- | --- | --- | --- |
| 小应用可独立交付 | `TBD` | `TBD` | `TBD` |
| 最小组合能完成目标业务 | `TBD` | `TBD` | `TBD` |
| 完整 OA 可由当前边界重组 | `TBD` | `TBD` | `TBD` |
| 跨应用只使用版本化契约 | `TBD` | `TBD` | `TBD` |
| Stable Base + Customer Overlay 可承载本参考产品 | `TBD` | `TBD` | `TBD` |
| 客户定制没有污染公共内核 | `TBD` | `TBD` | `TBD` |
| OA 权限分层不把客户角色、项目事实或最终授权写入错误组件 | `TBD` | `TBD` | `TBD` |
| 同一发布物可迁移到真实 Ubuntu 环境 | `TBD` | `TBD` | `TBD` |

## 4. C1～C8 业务覆盖

| 旅程 | 状态 | 事实所有者 | 正常／纠错／恢复证据 | 未决事项 |
| --- | --- | --- | --- | --- |
| C1 项目与责任建立 | `TBD` | `TBD` | `TBD` | `TBD` |
| C2 经营承接与合同链 | `TBD` | `TBD` | `TBD` | `TBD` |
| C3 开票、应收与回款 | `TBD` | `TBD` | `TBD` | `TBD` |
| C4 多类型生产交付 | `TBD` | `TBD` | `TBD` | `TBD` |
| C5 批复、审计与项目完成 | `TBD` | `TBD` | `TBD` | `TBD` |
| C6 成本与外委 | `TBD` | `TBD` | `TBD` | `TBD` |
| C7 奖金计算与分配 | `TBD` | `TBD` | `TBD` | `TBD` |
| C8 资料、统计与个人工作 | `TBD` | `TBD` | `TBD` | `TBD` |

允许状态：`passed`、`failed`、`not-applicable-with-evidence`、`insufficient-evidence`。不得使用“页面已存在”代替业务通过。

## 5. 应用与最小组合

对每个被验证组合填写：

| 目标业务结果 | 主要应用 | 显式直接依赖 | 未选择应用 | 独立启动／迁移／恢复 | 故障隔离 |
| --- | --- | --- | --- | --- | --- |
| `TBD` | `TBD` | `TBD` | `TBD` | `TBD` | `TBD` |

说明是否存在隐藏依赖、共享表、跨库读取、共享 ORM 或只能在完整套件中工作的应用。

## 6. Base 与 Customer Overlay 变化账

| 变化 | 唯一归属层 | 原因 | 兼容范围 | migration／回退 | 验收证据 |
| --- | --- | --- | --- | --- | --- |
| `TBD` | Base／配置／策略／类型化扩展／连接器／客户装配／客户专项 | `TBD` | `TBD` | `TBD` | `TBD` |

汇总：

- Base 兼容修改数量：`TBD`
- 新增 Overlay 内容数量：`TBD`
- 客户专项数量：`TBD`
- 客户核心分支数量：必须为 `0`
- 共享业务表数量：必须为 `0`
- 现场核心补丁数量：必须为 `0`
- 无法回退或无法升级的 Overlay：必须为 `0`

## 7. 数据、契约与故障边界

记录：

- 每个应用的数据库、账号、migration 与备份；
- 跨应用契约版本、幂等键和失败恢复；
- revision、历史、冲销／反向和来源快照；
- 各来源逐项停机、恢复和陈旧快照行为；
- 任何跨应用延迟、批量读取或缓存的实际测量。

| 验证项 | 结果 | 证据 | 风险 |
| --- | --- | --- | --- |
| 数据所有权 | `TBD` | `TBD` | `TBD` |
| 契约兼容 | `TBD` | `TBD` | `TBD` |
| 幂等与重放 | `TBD` | `TBD` | `TBD` |
| 故障隔离 | `TBD` | `TBD` | `TBD` |
| 备份恢复 | `TBD` | `TBD` | `TBD` |
| 用户可感知性能 | `TBD` | `TBD` | `TBD` |

## 8. 权限架构专项验证

### 8.1 信任链责任

| 层级 | 应承担的责任 | 结果 | 直接证据 | 越界／限制 |
| --- | --- | --- | --- | --- |
| Nginx | HTTPS、请求边界和转发 | `TBD` | `TBD` | `TBD` |
| Gateway | 路由、Header 清洗、入口验签和限流 | `TBD` | `TBD` | `TBD` |
| IAM／Policy | Subject、Tenant、授予、业务事实求值和有界凭证签发 | `TBD` | `TBD` | `TBD` |
| 目标应用 | 凭证复验、资源范围、状态、revision 和最终授权 | `TBD` | `TBD` | `TBD` |

### 8.2 角色与资源矩阵

每个角色至少记录一个预期允许、相邻角色拒绝、跨项目拒绝、跨租户拒绝和状态拒绝。全能力管理员只作健康控制，不计为本表通过证据。

| 参考角色 | 业务旅程／动作 | 资源范围 | 允许证据 | 拒绝证据 | 撤权／故障证据 |
| --- | --- | --- | --- | --- | --- |
| identity-super-admin | `TBD` | `TBD` | `TBD` | 不自动拥有业务权：`TBD` | `TBD` |
| oa-business-admin | `TBD` | `TBD` | `TBD` | `TBD` | `TBD` |
| general-manager | `TBD` | 全局只读 | `TBD` | 写入拒绝：`TBD` | `TBD` |
| business／production-manager | `TBD` | 全局查看／负责人配置 | `TBD` | 项目执行写入拒绝：`TBD` | `TBD` |
| project-business／production-owner | `TBD` | 指定 Project | `TBD` | 其他 Project 拒绝：`TBD` | `TBD` |
| business／production-member | `TBD` | 指定 Project | `TBD` | 组织／奖金拒绝：`TBD` | `TBD` |
| finance／office | `TBD` | 明确财务／办公范围 | `TBD` | 经营生产治理拒绝：`TBD` | `TBD` |

### 8.3 攻击、撤权与客户端绕过

| 验证项 | 预期 | 结果 | 直接证据 |
| --- | --- | --- | --- |
| 无凭证／错误签名／过期 | 入口或目标应用拒绝，不写事实 | `TBD` | `TBD` |
| 错误 Tenant／Audience／Action／Scope | 在业务执行前拒绝 | `TBD` | `TBD` |
| 伪造 Authorization／`X-OMS-*` | Gateway 清洗且目标应用复验 | `TBD` | `TBD` |
| 隐藏按钮后直接调用 API | 服务端 `403` | `TBD` | `TBD` |
| 撤权后旧页面提交 | 服务端拒绝，前端退出编辑并刷新权限 | `TBD` | `TBD` |
| IAM／Policy 停止 | 旧凭证仅按 TTL，新增授权失败关闭 | `TBD` | `TBD` |
| 目标应用业务状态不允许 | 即使 Capability 有效仍拒绝 | `TBD` | `TBD` |

权限专项只有在至少一个真实资源级策略样例贯通 Business Fact Snapshot、Capability Credential 和目标应用最终授权，并且 C1～C8 已实现旅程具有多角色正反证据时才能标记 `passed`。只有登录、Tenant Scope 或全能力管理员证据时必须标记 `insufficient-evidence`。

## 9. 人类可用体验

至少记录首页、列表、记录工作区、完整 OA 旅程和错误恢复的浏览器证据。

| 页面／旅程 | 结果 | 浏览器证据 | 可理解性／恢复问题 |
| --- | --- | --- | --- |
| `TBD` | `TBD` | `TBD` | `TBD` |

## 10. 真实环境验证

| 验证项 | 结果 | 证据 |
| --- | --- | --- |
| 环境所有权与授权 | `TBD` | `TBD` |
| 镜像摘要与目标架构 | `TBD` | `TBD` |
| HTTPS 与入口链 | `TBD` | `TBD` |
| IAM 与目标应用授权 | `TBD` | `TBD` |
| OSS／Evidence | `TBD` | `TBD` |
| 应用开关与单侧故障 | `TBD` | `TBD` |
| 升级、离机备份和空目标恢复 | `TBD` | `TBD` |
| 恢复后继续写入 | `TBD` | `TBD` |

如果本节没有至少一套真实 Ubuntu 环境的通过证据，最终结论不得为 `supported`。

## 11. 发现与反证

### 11.1 支持当前架构的证据

- `TBD`

### 11.2 要求修正边界的证据

- `TBD`

### 11.3 明确失败信号

- `TBD`

## 12. 未决事项与限制

| 未决事项 | 影响 | 责任 | 恢复条件 | 是否阻断结论 |
| --- | --- | --- | --- | --- |
| `TBD` | `TBD` | agent／human／external | `TBD` | 是／否 |

## 13. 下一轮验证输入

下一轮参考应用保持待选。报告只记录选择条件：

- 具有与参考 OA 不同的业务结构或纠错方式；
- 能取得合法授权、脱敏且可追溯的事实；
- 至少包含一个能够挑战当前 Base／Overlay 边界的真实差异；
- 不只比较页面和字段，而能验证责任、状态、历史和交付；
- 有可执行的成功、失败和恢复样例。

候选及未选择理由：`TBD`。

## 14. 最终建议

明确写出：

1. 哪些边界继续保留；
2. 哪些边界必须修正；
3. 哪些内容进入 Base；
4. 哪些内容保留在 Overlay／客户专项；
5. 下一轮最需要证伪的假设；
6. 是否可以进入下一参考应用验证。
