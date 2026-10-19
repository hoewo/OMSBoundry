# Project Counterparty Name Suggestions v1

问题 A → 可验证结果 B：原 OA 的甲方名称有历史文字补全，候选只有普通输入框 → 输入时显示无下拉的幽灵文字，Tab 明确接受后继续焦点移动，不自动修改名称或主体身份。

原依据：`826e0fc65aa09947fc4f64f62a2644fa5d11a302` 的 `ProjectPartyNameInput.tsx`、CSS、`oa2026.SuggestNames`。原查询混合历史项目 profile 与关联主档；目标不照搬跨库查询或无范围全局披露。

- 新只读 GET `/api/v1/counterparty-context/name-suggestions?q=...&limit=10`，schema `project-counterparty-name-suggestions-v1`，只返回有界名称数组，不返回项目 ID、联系人、电话或 Party ID。
- 只使用 Project 自有甲方历史快照和创建时已验证主体的最小名称快照；不读 Party 内表、外部主档或浏览器业务缓存，不新建事实／服务／迁移。
- 必须有既有 `counterparty-context.read`；Tenant／Project Resource Scope 来自可信凭证，不接受 URL/Header 改写。子项权限不隐含父项／兄弟项，Tenant 不交叉。
- 默认 10、最多 50，关键词最多 200 字；空查询零名称，字面不区分大小写匹配，`%`／`_` 不是通配符。SQL 单条只读一致快照，不产生幂等回执或 revision。
- 前端只在聚焦且非空时查询，无跨用户／权限长期缓存。前缀匹配优先，其次包含匹配；排除与当前文字完全相等的候选，但仍可提示其他包含此文字的名称（与原控件相同）；首尾空白不提示。失效／失焦／切项目的迟到结果不能改变当前提示。
- 普通 Tab 才采用文字，Shift+Tab 不采用，不阻止浏览器原焦点移动；响应不能自动填值。查询失败可继续手工填写，不制造主档匹配或新的写命令。
- 覆盖作用域／Tenant／旧名称、只返回名称、有界／字面查询、GET 零写、HTTP 授权／参数、JS 补全与真实浏览器 Tab。正式 IAM、Main 及 B01 全部权限仍是独立门禁。
