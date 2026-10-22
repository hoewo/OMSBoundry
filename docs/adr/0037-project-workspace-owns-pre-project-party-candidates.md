# ADR-0037：Project Workspace 拥有建项前合作主体候选

状态：已接受  
日期：2026-10-22  
关联：BF-177

## 问题

OMS 要求先在甲方管理或施工方管理中维护主体，再在后续项目中选择。Party Directory 只有中性主体档案；Project Workspace 只有项目已经建立后的 `client_party` 快照与 `contractor-party` 关系。因此仅凭已发生项目关系投影列表，会让新建施工方在刷新后消失；把所有 Party 同时展示到两页，又会让甲方错误进入施工方页面。

## 决定

Project Workspace 新增租户级、追加式 `project-party-candidate`：

- 键为 Tenant、Party ID 和 `client | contractor`；
- 保存登记时 Party revision、command、Actor 和时间；
- 只表示该主体可进入后续项目的对应选择器；
- 同一 Party 可分别登记两个候选角色；
- 记录不可更新或删除，后续若需要撤销必须另行定义追加式操作语义。

Party Directory 仍只拥有主体档案、稳定 ID 与 revision。具体项目仍由 Project 的 `client_party` 快照和 `contractor-party` Resource Link 表达。Workbench 先向 Party Directory 创建主体，再以独立幂等命令向 Project Workspace 登记候选；第二段失败时保留第一段并以原命令续办。

API 使用 `GET /api/v1/project-party-candidates` 与 `POST /api/v1/project-party-candidates`。读取复用租户级 `project-workspace.read`，登记复用租户级 `project-workspace.create`，不新增 Grant。

## 不变量

- 不给 Party 增加甲方、施工方、客户或供应商全局类型；
- 不把候选解释成已经发生的项目关系；
- 不复制 Party 名称或当前档案，不跨库读取／写入；
- Workbench 不在 localStorage、sessionStorage 或内存之外保存候选事实；
- 业务关系建立后的关联计数仍只来自 Project 实际关系；
- 每条写入必须校验同租户、启用且 revision 精确匹配的 Party 引用。

## 结果

用户在施工方管理新建主体后，候选登记成功即刷新并显示在施工方列表，也可用于关联项目；甲方不会因此混入施工方。该变化增加 Project Workspace migration 与版本化端口，因此必须与 Workbench 联合发布、迁移和恢复验证，不能只替换前端。

## 未包含

- 候选停用、撤销或审批；
- Party 主档删除策略；
- 已发生项目关系的自动反向登记；
- 新 Capability、客户专用角色或通用 CRM／供应商平台。
