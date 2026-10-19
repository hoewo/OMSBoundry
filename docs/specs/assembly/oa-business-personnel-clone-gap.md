# B02 经营参与人：原操作与候选缺口

2026-10-03 更新：以下“未实现／仅三个职责”是当时审计。BF-097 已接受有界 `commercial_contributor`、独立管理能力和 schema 11，复用原列表／选择抽屉，移出结束历史而不是物理删记录。实际真库／恢复／启动证据与公网反例见[本批报告](../../reports/oa-complete-operation-progress-20261003.md)。当前 `.6` 已关闭人员配置路由、冗余续签及隐藏来源抢发，有限名单／添加／结束／重入／原命令恢复和终场三项零写导航通过；原岗位显示、离职候选、经营授权策略及父包传播仍开着，所以 B02 全项不关闭。

日期：2026-10-02  
状态：只读源码对照／下一切片验收输入；未实现、未运行原站，不是新增架构决策  
关联：BF-083／BF-084／BF-017， [全站操作基线](./oa-original-usage-workflow-baseline.md)

问题 A → 可验证结果 B：当前通用 participant 无法区分经营参与人与生产专业参与人 → 保留原经营页添加／移除、原分权与同项目导航，Project 事实可明确表达参与目的，IAM 只按版本化可信事实签发相应原生能力，不把公司岗位或所有参与人写成经营权限。

## 原来怎么用

依据原提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302` 的 `BusinessPersonnelList.tsx`／CSS、`BusinessInfoTab.tsx`、`useAvailableProjectUsers.ts`、`PersonnelSelect.tsx`、`project_member.go`、`project_member_service.go`、`permission_service.go`、`permission_catalog.go`。原仓库只读且仍干净。

| 动作／显示 | 当前原源码事实 | 不能擅自替代为 |
| --- | --- | --- |
| 进入栏目 | 同项目经营信息 → 经营参与人；当前 active `business_personnel`，不把负责人放进此列表 | 跳 People 管理页；列全部 participant |
| 添加入口 | `project.business.member.assign` 允许才显示“添加参与人”；520px 右侧抽屉 | 甲方资料编辑权自动拥有成员管理权 |
| 此时填写 | 只选一个已有人员；固定 `business_personnel`，加入日期由客户端当天生成、active=true；无角色／日期／专业必填框 | 创建账号；选负责人；要求先填生产专业 |
| 候选人员 | 单独候选 API，不依赖人员目录管理。原查询排除已删除以及在职禁用账号，但包含离职人员；按在职优先、姓名／账号排序；UI 排除当前已加入的经营人员 | 只按经营公司岗位筛选；把所有账号表完整暴露 |
| 提交结果 | 添加成功关闭抽屉并更新该项目成员列表；取消只关闭并重置 | 切到另一个应用工作面；自动完成其他业务 |
| 移除 | 有成员管理权才显示删除；确认人名／公司角色；原后端物理删除该 member，不删除 user | 删除 People、IAM 账号或已有奖金／合同事实 |
| 卡片显示 | 人名、账号、部门、项目角色、公司角色；同一人可以有多个公司角色，离职标注在 Personnel 控件显示 | 用 People reference_code 冒充登录账号；用 IAM 授权标签冒充组织职务 |
| 只读 | 无管理权仍能看当前项目成员；按钮显隐与后端权限分别检查 | 为了显示页面给所有人 assignment.manage |

原 UI 的重复人员预检与候选排除已存在，但 `CreateMember` 没有为 business_personnel 作同人唯一断言，模型组合索引也非 unique；不能以 UI 检查当作并发／重试安全证据。原删除是物理删除，目标仍须使用已接受的结束有效期及历史保留，保留用户“移出当前列表”的效果。

通用 `ProjectMemberForm` 包含专业、日期和角色选择，但它不是该经营栏目实际使用的表单；不能拿通用旧入口当作 B02 的操作真相。

## 权限的实质区别

原 `business_personnel` 项目模板可看基本资料、看／改经营资料、看／上传／下载／删除经营文件；它没有设置经营参与人、成本、金额或奖金能力。管理经营参与人和维护甲方资料是两种不同权限。文件仍必须拆为目标 Evidence 原生授权，不能一次性给整个文件库通用读写。

原 `participant` 是生产专业参与人，需要 discipline；不能据名字“参与人”映射成经营参与人。Project 当前仅有 business_owner／production_owner／participant 三个受约束角色，Policy 1.2 的 participant 只有 Project read。[当前 Policy](../../../contracts/access/v1/examples/project-assignment-policy-v1.2.json)不能直接把所有 participant 扩成经营资料可改。

原子项目会从父包增加经营角色权限，生产角色仍属子项目。当前 IAM Assignment Evaluator 明确要求 fact.project_id 等于凭证的精确 project Scope，不能把父包凭证直接当子项目凭证。若要保留原使用效果，须有明确版本化父子业务事实和 Policy，为具体子项签发目标原生能力；不靠目录 UI 隐式继承、扩大父 Bearer 或写子库副本。此处尚未实现。

## 必须先收窄的实现边界

1. 参与目的事实由 Project 拥有，People 仍拥有人／组织资料，IAM 仍拥有账号与签发；不建立新的成员小系统或共享 User 表。
2. 经营参与目的的表示方式必须先登记版本化类型／契约及兼容性；本对照不接受把原 `business_personnel` 客户岗位名直接塞进 Base，也不选择无限任意 role／字段平台。当前三个 Base role 和旧策略保持不变。
3. 能力分开：读取参与、管理经营参与、管理生产参与、资料 read／manage、文件 read／upload／delete 分别界定；目标应用仍按原生能力、资源归属与状态最终授权。新版本只能显式选择，旧 Grant 不自动扩权。
4. People 当前最小 Person 没有原账号／部门／公司岗位显示字段。补这些展示前先确定最小版本化资料来源；没有来源的字段保持明确缺口，不造示例人名或读 IAM 内表。人员候选必须有其自身可信读取范围。
5. 原离职人员仍可选不等于目标应激活或恢复其登录；目标目前只接受 active People 精确 revision。历史选人／归档人员标注与“当前能否执行”要分开验证，不能关闭版本／状态校验。

## 下一切片的通过条件

- 同经营栏目列表 → 添加抽屉 → 只选人 → 取消零写／添加关闭复读 → 确认移出 → 刷新仍同项目；520px 与小视口按钮可达。只读不出现管理按钮，错误不伪装空列表。
- 管理者、只读者、仅经营资料维护者、生产参与人、经营参与人、负责人分别验证；经营参与人不能自增成员、不能查看原未授的金额／成本／奖金；父包／子项／兄弟及跨 Tenant 分开。
- 重复、双击、失联重放、revision 冲突、People 停用／版本变更／503、移出后再加入、移出后旧凭证到期与新签发拒绝，服务端事实和历史正确。
- 名单 GET 零写，不改原负责人、Production 人员、People、IAM、奖金及外部事实；添加／结束在 Project 本事务追加历史和回执。
- 类型／旧 API／Policy／schema 的兼容、空目标恢复后重放与继续、限资源独立启动及正式 Gateway／IAM 浏览器，分别留证。

仅有源码对照，不填写 PASS；B02 和 B01 经营参与人权限缺口仍开着。完整全站及 Main 部署验收继续 `insufficient-evidence`。
