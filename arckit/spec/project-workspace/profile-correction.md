# Project 基本资料更正 v1

问题 A → 可验证结果 B：原 OA 已立项项目仍能编辑基本信息 → 用户在同一项目详情保存资料、更改后复读一致，项目身份、状态、分工及外部业务事实不变。

来源：原 `frontend/src/components/project/BasicInfoTab.tsx` 的编辑抽屉与 `canEditBasicInfo`；对应全站使用基线 P02。候选不等于公网或整站通过。

权限来源再核对原 `useProjectPermissions.ts`、`backend/internal/services/permission_catalog.go` 和 `project_service.go`：基本资料权限独立于两种负责人设置；原默认项目管理员及项目经营负责人有 basic update，生产负责人默认模板不含它。新增可选 Overlay 1.2／Assignment Policy 1.1 显式授予 OA 业务管理员、项目管理员或当前经营负责人更正能力；保留旧版本原文件，生产负责人／参与人不因分工自动获得新能力。部署须明确选择版本或精确 Grant，不暗改当前 IAM。

## 原生动作与边界

- 新增 `POST /api/v1/projects/{project_id}/profile-corrections`，显式能力 `project-workspace.profile.correct`，只在 active 且当前资源授权成立时可执行。
- 旧 `PUT`、`project-workspace.update` 继续只处理 draft／pending_completion，不自动扩大旧凭证权限。
- 请求含 UUID command、expected revision、更正原因，以及完整编号／名称／概况／计划日期／类型集合。普通项目编号必填，项目包编号可空；所有 active 节点至少一个类型。
- 有变化追加 Project revision 和 `project_details_replaced` 历史，记录 `correction_reason`、`previous_revision` 与当前资料。无变化只保存命令回执，不制造 revision／历史。
- 同命令同内容重放原回执，修改内容、旧 revision、编号冲突返回 409。取消后新命令拒绝；已成功命令仍可重放。
- ID、生命周期状态、父子关系、Client Party 冻结快照、Assignment、Resource Link、完成确认及已有来源快照不被更正。类型集合的更正不级联删除 Production 类型／成果事实；来源保留各自历史。
- 负责人调整使用已有独立 Assignment 能力，不由基本资料能力代办。管理费、资质主体、甲方重选尚未完成映射，不假装保存。

## 页面与验收

沿用原 OA 基本信息卡片和右侧编辑抽屉，不设计新的页面体系。抽屉显示名称、编号、承接日期、概况、项目类型；界面将“修改项目基本资料”作为原因提交，不增加原 OA 没有的更正说明填写步骤。成功回读同项目，失败保留输入和错误；未知网络结果重试同 command，409 要重新读取后确认，不自动覆盖并发修改。

验收覆盖普通项目／包、active／draft／cancelled、缺能力／越资源／跨租户、空类型、幂等与并发、编号冲突、无变化、历史原因、独立真库持久化以及真实浏览器抽屉／提交／刷新。授权角色由显式版本化 Overlay 或 Grant 选择，新能力不自动授予所有账号。
