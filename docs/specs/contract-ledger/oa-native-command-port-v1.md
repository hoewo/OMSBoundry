# 合同原生操作端口 v1

2026-10-04 补充：下文“不改数据库”只指原领域 JSON 操作端口本身。新增参考 OA 客户草稿扩展另需 Contract 自有 schema 20，见[扩展规格](./reference-oa-contract-drafts-v1.md)；原表单已候选接线，但未部署、未完成正式确认／附件或 B04～B06。下文 2026-10-03 的未接表单和 schema 19 启动证据保留历史范围。

问题 A → 可验证结果 B：原 OA 合同工作面只有只读 Project Link，不能在同项目保存合同 → 原组件调用 Contract 自有的版本化 JSON 命令，草稿、正式 revision、协议和结算可原生复读，Project 只在保存后关联返回的稳定 ID。

此端口是 BF-084／BF-096 的兼容实现步骤，不新增事实所有者或通用 RPC。它复用现有 Contract Use Cases、金额十进制字符串、expected version、command ID 和可信 Actor；不改数据库，不迁移或复制原共表后台。正式操作必须复验 Contract audience／Tenant／原生 capability 和精确 Contract Scope；创建只接受 Tenant Scope。只读参考模板不能调用写端口，客户端 Actor、Project ID 和身份 Header 不作授权输入。

原生边界：GET `/api/v1/contracts/{id}/workspace`；POST `/api/v1/contracts`、`/{id}`、`/{id}/cancel`、`/{id}/activate`、`/{id}/changes`、`/{id}/settlements`，草稿主体使用 `/{id}/parties` 和 `/{id}/parties/{link}/remove`。每种路径严格解码自己的请求类型；返回 `schema_version=1`、source application／tenant 和类型化回执。写入成功不要求后续 GET 一定成功；未知回复只能保留原命令继续，409 不自动覆盖。状态和前置条件由原领域用例最终检查，成功命令重放不因新的状态预读而被挡住。

约束：view 才能读取整个工作面；draft.manage 仅草稿维护；lifecycle.manage 才能激活、追加协议或结算。无 cookie 身份、模板 token 或 local 管理员旁路。错误不暴露凭据／SQL；业务拒绝沿用 404／409／422，认证依赖失败为 503。GET 零写，协议与 confirmed Settlement 不可覆盖。现有本地 Party 标识仍是实现事实，不能把此端口描述为已完成 Party 统一迁移。

不负责：两条 OA 经营链的类型化上下文、行业费率／分项费用、定案可编辑到不可变结算的更正映射、独立 Evidence 引用、原 ContractForm 接线及完整 B04～B06。这些必须与原表单一起落实，按[原操作映射](../assembly/oa-contract-original-operation-map.md)推进；裸 API 不能关闭合同工作面。当前反向授权、精确金额、Actor、幂等冲突与原语义已在聚焦真库测试验证，候选另按显式 ops migrate → server verify 完成独立空库启动／重启和匿名拒绝。未部署、未接原表单，不记为合同业务页通过；失败安装夹具及分层限制保留于[本批报告](../../reports/oa-complete-operation-progress-20261003.md)。
