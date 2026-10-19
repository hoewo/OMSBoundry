# Design Operations 技术方案

> 兼容定位（2026-09-27）：本文描述当前已实现的 `design-v1` 技术基线。目标 Production Operations 必须从该单一事实源 additive 演进，旧 API／Capability 只代理 `design-v1`；多类型迁移、canonical API 和反双写门禁见 [Production Operations 多类型边界决策](../production-operations/decision.md)。

## 元信息

- 路径：`tech/design-operations/solution.md`
- 技术领域：`design-operations`
- 状态：✅ 已采用
- 负责人：BizFoundry
- 支持功能：`spec/design-operations/design-production-delivery.md`
- 关联模型：`tech/_shared/models/DesignWorkItem.yaml`、`DesignStage.yaml`、`DesignDeliverable.yaml`、`DesignHistoryEvent.yaml`
- 关联 API：`tech/_shared/contracts/design-*.yaml`
- 变更历史：
  - 2026-09-25：固定 `design-v1` 服务边界、阶段投影、引用校验、事务和失败恢复。

## 方案概述

Design Operations 是 Project Workspace 的类型化业务扩展，而不是 Project 子模块或通用流程引擎。它以独立 Go 服务和独立 PostgreSQL 数据库拥有设计 Work Item、Stage、Deliverable Reference 和历史；Project 提供项目身份与 Assignment，Evidence 提供文件版本，两者都只通过版本化 HTTP reference-read 契约读取。

本地 Demo 直接访问独立 HTTP 入口并使用 v1 模板 IAM；完整组合改由 Nginx／Gateway 路由并切换正式 IAM Provider。两种模式共享领域规则、API、migration、数据库和验收。

## 进程与依赖

```text
Browser / acceptance script
  → Design Operations HTTP API
    → v1 Identity Provider port
    → Design application service
      → Design domain and design-v1 definition
      → Design PostgreSQL repository
      → Project Reference Validator (Project + Assignment)
      → Evidence Reference Validator (File Version)
```

强依赖只有本应用 PostgreSQL 与 v1 Identity Provider。Project／Evidence 只在创建、设置负责人、登记成果和完成 Stage 等需要新事实的命令中调用；普通 Work Item 和历史读取不依赖外部服务在线。

## 数据表与约束

| 表 | 作用 | 关键约束 |
| --- | --- | --- |
| `design_work_items` | Work Item 当前投影、Project／owner 引用、revision | tenant 隔离；同 Project／type 最多一个非取消记录 |
| `design_stages` | 四个 Stage 当前投影 | tenant＋work_item＋stage_key 唯一；sequence 固定 |
| `design_deliverables` | Evidence File Version 的追加引用 | 同 Stage／requirement 最多一个 active；旧引用不改写目标 |
| `design_commands` | command ID、请求摘要和结果 | tenant＋command ID 唯一且禁止更新／删除 |
| `design_history_events` | 不可变领域历史 | tenant＋work_item＋sequence 唯一且禁止更新／删除 |

数据库不保存外部服务地址，不建立跨库外键，不包含 Project 展示字段、人员名册或文件物理字段。

## 类型定义

`design-v1` 在代码中实现为不可变定义：四个 stage key、顺序、可不适用集合和必交 requirement key。数据库只实例化定义结果并保存 `type_definition_version`；没有通用 JSON 条件解释器、数据库脚本或管理员流程编辑器。

将来新增类型必须满足：

1. 使用新 type key／definition version；
2. 有独立产品规格、状态规则和验收矩阵；
3. 不修改 Project、People、Contract、Money 或 Evidence 的核心模型；
4. 旧 Work Item 始终按创建时 definition version 解释。

## 命令事务

所有写命令遵守：

1. 验证 credential 的 issuer、tenant、audience、action、scope 和时间；
2. 规范化请求并在 tenant 内检查 `command_id`；
3. 按命令需要在线读取 Project／Assignment／Evidence reference；
4. 开启本地事务，获取 command advisory lock 并锁定 Work Item；
5. 校验 `expected_revision`、当前状态、stage frontier 和 `design-v1` 不变量；
6. 原子写当前投影、追加引用、命令结果和一个 History Event；
7. 提交并返回新 revision。

外部验证在本地事务前完成，保存验证时 source revision。来源随后变化不会回写历史；下一次需要新事实的命令会重新验证。相同 command 成功重放先返回已提交结果，不要求上游继续在线。

## 阶段推进与回退

- 领域层计算唯一 frontier，客户端不能直接提交 Work Item 或 Stage 状态。
- `complete` 重新验证 owner 和 active 必交 Evidence Version；成功后推进 frontier。
- `set-not-applicable` 只接受定义允许的 Stage 和非空原因。
- `restore-applicability`／`reopen` 把目标 Stage 设为 ready，并把后续 completed Stage 设为 pending；后续 not-applicable 保留。
- 回退不删除 Deliverable；再次完成时重新验证现有 active 引用，也允许用户先追加替代版本。
- 最后 Stage 完成后只更新 Design Work Item 为 completed，不调用 Project 写 API。

## API 与错误语义

| API | Capability | 结果 |
| --- | --- | --- |
| `POST /api/v1/design-work-items` | `design-work-item.create` | 创建 design-v1 Work Item |
| `POST /{id}/owner` | `design-work-item.owner.manage` | 设置 active production_owner 引用 |
| `POST /{id}/stages/{key}/applicability` | `design-work-item.stage.manage` | 设不适用或恢复 |
| `POST /{id}/stages/{key}/deliverables` | `design-work-item.deliverable.manage` | 登记或替换 Evidence 引用 |
| `POST /{id}/stages/{key}/complete` | `design-work-item.stage.manage` | 完成当前 Stage |
| `POST /{id}/stages/{key}/reopen` | `design-work-item.stage.manage` | 显式回退 |
| `POST /{id}/cancel` | `design-work-item.cancel` | 取消 Work Item |
| `GET /{id}` | `design-work-item.read` | 读取自有事实和引用状态 |
| `GET /{id}/history` | `design-work-item.history.read` | 分页读取不可变历史 |
| `GET /api/v1/references/design-work-items/{id}` | `design-work-item.reference.read` | 向 Project 提供最小稳定引用投影 |

通用错误：400 格式错误；401 未认证；403 capability／scope 拒绝；404 不存在且不可见；409 revision、active 唯一、幂等冲突或取消后写入；422 Project／Assignment／Evidence 引用无效、Stage 顺序或完成缺口；503 上游不可用。响应包含稳定 code、request ID 和 retryable，不泄露凭据或数据库细节。

## 鉴权与引用端点

- 服务启动时发布稳定 Capability Manifest；本地模板不能关闭授权。
- HTTP 层删除客户端伪造身份 Header，只使用 Provider 验证结果。
- scope 可以覆盖 tenant、Project 或 Work Item；Project Assignment 是业务前置，不等于 IAM 授权。
- Project reference-read 使用 `GET /api/v1/references/projects/{project_id}`；Assignment 使用 `GET /api/v1/references/projects/{project_id}/assignments/{assignment_id}`，只返回 tenant、稳定 ID、revision、状态、类型／角色和显示快照。
- Evidence reference-read 使用 `GET /v1/references/files/{file_id}/versions/{version}`，只返回 tenant、File ID、Version、revision、status、摘要和非权威显示名，不返回对象键、STS 或签名 URL；兼容 Project Resource Link 的 File 级读取固定指向 v1 的第一个不可变版本。
- Design reference-read 只返回 Work Item ID、Project ID、type key、status、revision、当前 stage 和完成缺口；不返回完整历史或文件技术字段。

## 部署与可移植性

第一实现复用 Project Workspace 的工程形状：独立 Go module、migration、PostgreSQL database/user、`.env.example`、Dockerfile／Compose、健康检查、备份恢复和带校验和的 Linux release。Project、Evidence 与 IAM 地址全部配置注入，业务代码不得绑定 `localhost`、Docker 服务名、ECS IP 或客户域名。

Design Operations 在本地业务 Demo 中可以直接发布开发端口；D1／公司 Ubuntu 总装只能由 Nginx 对外发布。迁移部署方式不得改变 Work Item ID、revision、Stage、Deliverable Reference 或历史。

## 明确延期

- 第二生产类型、多类型并行和用户自定义类型；
- 批复审计、成本、外委、奖金、审批、通知、通用任务；
- 直接更新 Project 完成状态或跨应用分布式事务；
- CAD 预览、格式转换和在线编辑；
- 事件总线、Redis、搜索引擎和多节点 HA。
