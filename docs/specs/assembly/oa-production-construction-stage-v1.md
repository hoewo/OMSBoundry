# 原 OA 施工图阶段资料 v1

状态：私有原生候选已通过；未发布 Main，不代表变更／竣工、完整 Production 或完整 OA 已完成。关联 BF-113、[阶段资料集合 v1](./oa-production-stage-materials-v1.md)、[初步设计 v1](./oa-production-preliminary-stage-v1.md)、[原阶段逐动作对照](./oa-production-stage-original-operation-map.md)与[实际报告](../../reports/oa-construction-stage-native-validation-20261006.md)。

问题 A → 可验证结果 B：已经含方案和初步设计的 `reference-oa-stage-set-v1@3` 不能为了接施工图而原位改变历史语义，页面可见也不能代替运行时装配 → 保留 v2、v3 不变，用同一 Production Work Item 的显式、可审计、幂等 3→4 升级增加施工图，并由原 OA 施工图组件完成真实 Evidence 文件操作、复读、撤回、撤权与恢复。

## 原操作与复用界面

直接复用原 OA `ConstructionStageFileManagement` 和 `ConstructionStageEditModal`。源码核对确认它与初步设计属于同一交互类：同一项目生产页内的施工图 Tab、800px 抽屉、多份主文件、可选多份校审单、可选评分（`0` 有效）、取消、刷新回读、预览、下载和逐份移出。只替换原单体数据调用为 Production／Evidence 端口，不重写 UI，也不把原覆盖式文件更新带进新事实模型。

首次浏览器验收发现 Tab 与宿主均已渲染，但 Workbench 运行时只派发 `scheme`／`preliminary`，导致内容为空。修复为 `scheme`／`preliminary`／`construction` 统一派发到有界 Production Stage Workspace，并增加专门回归测试；这说明“组件存在”和“Tab 可点击”均不能作为原生场景通过证据。

## 所有权与依赖

| 事实 | 唯一所有者 | 本切片行为 |
| --- | --- | --- |
| Work Item、definition version、Stage、资料引用、评分、撤回及历史 | Production Operations | 新增显式 v4 与 3→4 升级事实 |
| File ID／Version、名称、摘要和字节 | Evidence Service | 浏览器完成上传，Production 写入前精确复验 |
| Project 身份、状态、类型和生产分工 | Project Workspace | 只提供版本化引用校验，不接收施工图事实 |
| 人员档案 | People & Groups | 只读引用，不接收阶段资料 |
| 账号、Capability Credential 与 Grant | IAM | 继续签发现有 Production／Evidence 原生能力 |
| 页面状态与续办草稿 | Unified Workbench | 仅短期装配，不形成业务真相 |

没有新增应用、Audience、Capability 或 Workload Policy 边。跨系统关系仍是正式入口和 IAM 鉴权、Production 写入时向 Evidence 精确校验、Workbench 只读装配。Project／People 的 canonical 摘要在验收前后不变。

## 兼容版本与升级

- v2 永久表示“仅方案”，v3 永久表示“方案＋初步设计”，v4 表示“方案＋初步设计＋施工图”。
- 新建显式阶段集合使用当前 v4；旧 v2／v3 行不批量迁移，读取尚未包含的阶段是已知未纳入且零写。
- 第一次真实施工图保存才执行 3→4；若从 v2 直接进入施工图，客户端必须持久化并顺序执行 2→3、3→4 两个独立命令，禁止跨版本跳跃。
- 每一步升级都要求 command ID 和 expected revision，在 Production 单库事务内追加升级行、新 Stage、Work Item revision、history 和 command receipt；Work Item ID 与全部前序 Stage／资料不变。
- 升级成功而 Evidence 上传或资料登记失败时，已确认版本不回滚；页面只续未完成子命令，不重复升级或上传已经确认的字节。
- migration `000005_construction_stage_definition.sql` 只扩充允许版本和 3→4 守卫，不改写旧事实；降级、跳级和缺少审计行的直接 UPDATE 均被数据库拒绝。

## 边界判定

方案、初设、施工图是第三个同类阶段场景，均复用同一 Production-owned 集合、Evidence 端口、IAM 动作和 Workbench 适配器，且没有新增跨系统边。由此可以确认：

> “阶段资料集合”子边界在同类多文件／校审／评分阶段内达到可重复边界；Production Operations 整体仍只是候选边界。

这不是把“没有吞成大系统”当成果，而是证明了一个更具体的演变规则：同类阶段通过不可变 definition version 与显式顺序升级扩展，外部 Owner 和 API 边不动。下一项“变更／竣工”具有两套列表、620px 上传抽屉、无校审／评分、文件类型差异和不同完成映射，必须作为异类场景重新证伪，不能继续复制 StageFilePanel。

## 实际证据与未关闭项

已实际运行：Production Go 全包、Workbench 171 项 Node、TypeScript check、生产 build、Workbench runtime 29 项、15 应用静态边界审计；私有 Nginx → Gateway → 正式 IAM → Project／People／Production／Evidence → 远端临时 PostgreSQL 18 的 10 场景浏览器链。五所有者逻辑空目标恢复前后全表摘要一致，5 份 Evidence 字节逐项一致，原 UI 继续，页面错误为零。本机 Docker 未启动、线上 Main 未修改、夹具已停止。

仍未关闭：Main／公网匹配发布、正式 D1 archive／keys／旧会话恢复、全角色矩阵、完整双 Tenant、多文件 Project 消费兼容、变更／竣工、试验、报告、生产总进度和第二客户升级。下一切片必须先固定原 OA 变更／竣工的两类事实及完成规则，再决定复用哪些端口。
