# 原 OA 初步设计阶段资料 v1

状态：私有原生候选已通过；未发布 Main，不代表施工图、变更竣工、完整 Production 或完整 OA 已完成。关联 BF-112、[阶段资料集合 v1](./oa-production-stage-materials-v1.md)、[原阶段逐动作对照](./oa-production-stage-original-operation-map.md)与[实际报告](../../reports/oa-preliminary-stage-native-validation-20261006.md)。

后续 BF-113 已以不可变 v4 和显式 3→4 增加施工图；本文件继续准确描述 v2→v3 初设切片，不把当时 v3“新建最新版本”的实现状态冒充当前最新版本。当前新建阶段集合使用 v4，详见[施工图规格](./oa-production-construction-stage-v1.md)。

问题 A → 可验证结果 B：已经登记的 `reference-oa-stage-set-v1@2` 只包含方案，不能为了接初步设计而原位改变历史语义 → 保留 v2 不变，用同一 Production Work Item 的显式、可审计、幂等 2→3 升级增加初步设计，再由原 OA 初设组件完成真实 Evidence 文件操作、复读、撤回、撤权与恢复。

## 原操作与复用界面

直接复用原 OA：

- `frontend/src/components/production/PreliminaryStageFileManagement.tsx`
- `frontend/src/components/production/PreliminaryStageEditModal.tsx`

可见操作保持为同一项目生产页中的初步设计 Tab、800px 抽屉、多份初设主文件、可选多份校审单、可选评分（`0` 有效）、取消、刷新回读、预览、下载和逐份移出。界面复用不等于复用原共享数据库或覆盖式文件更新；目标实现继续使用 Production 追加历史和 Evidence 不可变 File／Version。

## 所有权与依赖

| 事实 | 唯一所有者 | 本切片行为 |
| --- | --- | --- |
| Work Item、definition version、Stage、资料引用、评分、撤回及历史 | Production Operations | 新增显式 v3 与 2→3 升级事实 |
| File ID／Version、名称、摘要和字节 | Evidence Service | 浏览器完成上传，Production 写入前精确复验 |
| Project 身份、状态、类型和生产分工 | Project Workspace | 只提供版本化引用校验，不接收初设事实 |
| 人员档案 | People & Groups | 只读引用，不接收阶段资料 |
| 账号、Capability Credential 与 Grant | IAM | 继续签发现有 Production／Evidence 原生能力 |
| 页面状态与续办草稿 | Unified Workbench | 仅短期装配，不形成业务真相 |

没有新增应用、Audience、Capability 或 Workload Policy 边。仍是浏览器／Workbench → Gateway → IAM → Production，以及 Production 写入时 → Evidence 的精确引用校验；Project／People 的事实摘要在验收前后保持不变。

## 兼容版本与升级

- v2 永久表示“仅方案”；历史 Work Item、命令摘要和 completed 含义不变。
- v3 表示“方案＋初步设计”，两阶段都由同一个 Work Item 拥有。
- BF-112 验收时新建显式阶段集合使用 v3；BF-113 后当前新建使用 v4，历史 v3 仍按本文件语义读取。既有 v2 读取初步设计时返回“已知但尚未纳入当前定义”，读取零写入。
- 只有用户第一次实际保存初步设计时，前端先提交持久化的升级子命令，再提交资料登记子命令；不在打开 Tab、刷新或后台读取时自动迁移。
- 本切片的 `POST /api/v1/production-work-items/{id}/stage-material-definition-upgrade` 只执行精确 2→3；当前端点另支持 BF-113 的精确 3→4，仍要求既有 stage-material read／register 边界、expected revision 和 command ID，并禁止跳级。
- 升级在 Production 单库事务中追加 `production_definition_upgrades`、初设 Stage、Work Item revision、history 和 command receipt；保持 Work Item ID、方案 Stage 和全部旧资料不变。
- 同一命令重放返回原回执；不允许 3→2、跨版本跳跃或用后续版本改写 v2。

`000004_preliminary_stage_definition.sql` 只扩充允许版本及追加升级事实和约束，不批量更新旧行。升级成功而后续 Evidence 上传或资料登记失败时，v3 是已经确认的 Production 事实；页面保存原阶段操作和子命令，恢复后只继续未完成段，不回滚升级、不重复上传已完成字节，也不制造第二次升级。

## 边界判定

方案和初设已经用同一个“多主文件＋可选校审＋可选评分”阶段资料端口完成第二个同类场景，且未改变数据所有权或增加依赖边。因此：

> Production 的“阶段资料集合”子边界获得第一份同类可重复证据；整个 Production Operations 仍只属于候选边界。

不能把子边界提升解释为整套 Production 已稳定。施工图可能有不同表单、阶段完成规则或成果结构；变更竣工、试验、报告也已经确认不是简单复制方案／初设表单。若下一阶段需要任意流程 DSL、第二套可写资料表、跨库事务或把文件字节放入 Production，则当前边界被证伪，必须先重新立决策。

## 实际证据与未关闭项

已实际运行：Production Go 全包、Workbench 169 项 Node、TypeScript check、生产 build、15 应用静态边界审计，以及私有 Nginx → Gateway →正式 IAM → Project／People／Production／Evidence → 远端临时 PostgreSQL 18 的 9 场景浏览器链；五所有者逻辑空目标恢复后原 UI 继续，四份 Evidence 字节摘要保持，零页面错误。

仍未关闭：Main／公网匹配发布、正式 D1 脚本和 keys／旧会话恢复、全角色矩阵、完整双 Tenant、变更／竣工及其后阶段、多文件 Project 资料消费兼容和完整 OA。施工图已在 BF-113 作为后续独立切片完成私有候选，未扩大本报告当时范围。
