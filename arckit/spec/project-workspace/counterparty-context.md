# Project Counterparty Context v1

问题 A → 可验证结果 B：原 OA 在项目经营页行内录入甲方名称与联系人，候选界面尚不能保存 → 在同一项目上下文保存、读取和更正项目专属对方联系资料，不要求跳到主体产品、不创建或修改主体主档。

## 原操作与所有权

参考 `road-design-oa-system` HEAD `826e0fc65aa09947fc4f64f62a2644fa5d11a302` 的 `BusinessInfoTab.tsx` 437～482、640～744：必填名称，可选联系人与电话，行内编辑、取消或保存；保存后仍在甲方工作面。原界面明确说明“名称和联系人直接保存到当前项目，不会新建或修改甲方主数据”。

Project 拥有本项目的 `client` 业务上下文，不是 Party 当前档案，不含登记标识、稳定主体 ID 或主体合并判断。`client_party` 仍是创建时经精确 Party 引用验证的不可变身份快照；本命令不改绑／解绑它。不按名称匹配，不将联系资料用于 IAM、Contract 或 Money 的正式主体引用。两者不一致时界面分别说明；后续合同仍须独立、显式验证主体。

## 原生约束

- GET／POST `/api/v1/projects/{id}/counterparty-context`；固定 v1 `role=client`，只承载名称、项目联系人与电话，不扩为通用字段平台。
- GET 需要 `project-workspace.counterparty-context.read`，写需要独立 `.manage`，均检查 Tenant／Project Scope。无金额能力也可录入；基本资料更正权不隐含此权。
- 写仅 active Project，含 command、expected Project revision、原因和完整联系资料。名称必填 200 字，联系人 80 字，电话 50 字并遵循原 OA 数字、空白、加号、横线及括号格式；可选空值明确清除。
- 每次变化追加版本化 `project-counterparty-context-v1` 快照于 Project 自有不可变历史；这是本事实的权威记录，不是浏览器缓存。利用已有 tenant/project/revision 索引按最新快照投影，Project revision 与历史、命令回执同事务提交。旧快照不可覆盖或删除，已有整库备份范围保持。
- 相同内容只记录回执，不制造 revision；成功重放命中原结果，stale、取消状态、无权、跨租户和事务失败不写部分事实。无外部依赖或主数据双写。
- 普通 Project GET／Reference 不新增联系人；History 有读取历史权但无此资料读取权时隐藏此事件内容，只保留事件身份／revision 等非内容元数据。联系人不会因普通项目读取或历史读取而泄露。
- Capability Catalog 兼容增加两个原生能力；客户仅显式选择新 Overlay 1.3／Assignment Policy 1.2 才授予。旧 Overlay、Policy、Grant 不自动变更，身份管理员不自动获得业务能力。

## 页面及验收

同一项目“经营信息→甲方信息”，行内三字段、必填／电话校验、取消零写入、保存并回读，按服务端 allowed_actions 显示／禁用。读失败不是“未录入”；提交结果不确定保留命令，保存成功回读失败只重读；409 保留输入并要求显式加载最新资料。

单元、HTTP 权限矩阵、真实 PG 原子／幂等／并发／不可变／隔离、独立启动和合成浏览器分层记账；不把这些拼成正式 Nginx／IAM／公网全链已通过。B01 之外的合同、参与人、招投标、款项、开票、成本、奖金及正式部署未由此关闭。
