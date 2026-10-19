# 原 OA 基本资料编辑修复验证

日期：2026-10-02

状态：P02 核心字段候选通过分层验证；P02 全项、完整仿制及公网验收未完成

关联：BF-085、[原 OA 使用流程基线](../specs/assembly/oa-original-usage-workflow-baseline.md)、[上一条立项切片](./oa-initiation-clone-repair-validation-20261002.md)

## 结论

已接通已立项项目的名称、编号、承接日期、概况和多类型编辑。普通项目和无编号项目包均实际点击保存、复读、刷新，保持同一项目 ID；生产负责人仍可后补。另修复架构透镜挡住抽屉保存按钮的问题。**这些是候选实现及隔离证据，Auth58 Main 未更新，完整 OA 仍为 `insufficient-evidence`。**

本切片解决“原 OA 已立项后可以编辑资料，当前 Project 却只允许草稿更新 → 同一项目详情可以真实保存自有基本资料，同时保留状态、分工、引用及历史”。它不关闭 P02 的负责人更换、资质、管理费和甲方等全部字段，也不证明经营／生产／统计可用。

## 原站依据与实现边界

只读参考仓库提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`；原工作树保持干净。核对 `BasicInfoTab.tsx` 编辑表单、`useProjectPermissions.ts`、后端 `permission_catalog.go` 和 `project_service.go`：基本资料编辑与两种负责人分配分别授权；默认项目管理员及经营负责人可编辑，生产负责人默认模板不含此能力。尚未取得原站浏览器真人复核。

新增 [Project Profile Correction v1](../../arckit/spec/project-workspace/profile-correction.md) 和原生 `project-workspace.profile.correct`。旧 PUT／update 保持 draft／pending_completion 门槛及旧命令 hash；新动作只处理 active 且当前租户、资源和能力均满足的 Project。更正追加 revision、原因和历史；无变化只记录回执。同命令重放、编号冲突、并发版本和取消后的新动作分别受控，不覆盖 Client Party 快照、Assignment、Resource Link、完成确认及外部事实。

工作台仍沿用原 OA 基本信息卡片及右抽屉，通过原生 API 保存。界面使用固定“修改项目基本资料”原因，不额外要求原站没有的填写步骤。409 保留输入并要求明确加载最新资料；未知结果重试原 command；保存已成功但复读失败只再读，不重复提交。前端技能用于检查现有抽屉和按钮可达性，没有重新设计操作体系。

可选 `reference-oa-c1-role-overlay-v1.2.json`／`project-assignment-policy-v1.1.json` 显式发布新授权，旧文件原样保留。新版本并未装入 Main，也没有自动修改当前 IAM Grant；身份管理员不因此获得业务能力，生产负责人和参与人不因分工得到更正能力。

## 实际验证

| 证据层 | 已运行结果 | 不能代替 |
| --- | --- | --- |
| 本地无容器 | Project／Workbench 普通 Go 测试与 vet；Workbench Node 13 项、运行时语法；IAM capability／policy 测试及新策略原生 parser、JSON 动作／版本匹配均通过 | 正式 IAM 运行时、客户权限全矩阵 |
| ECS 独立 PostgreSQL | `f` 同批 Project 97／HTTP 53 条 PASS 记录（包含子测试）；普通项目／包、更正历史、旧接口拒绝、跨租户、编号冲突、并发、无变化、取消后重放、Client 快照及分工／引用不变通过 | 公网浏览器业务链 |
| 独立服务启动 | `g` 新二进制非 root 在自有空库迁移到 schema 10；测试 JWKS verifier 启动，三个匿名原生端点 401，重启逐表事实不变 | 完整 Suite、正式用户、业务恢复验收 |
| 合成浏览器 | 使用候选 HTML／CSS／JS 实际点击普通项目和项目包派发、完成、编辑；空类型明确报错；两次更正，刷新仍为同 ID；项目包编号留空且生产负责人未设 | PostgreSQL 持久化、真实 Gateway／IAM／公网联合证据 |

真库首轮 `e` 未通过：新增测试把原子的 draft＋经营负责人派发误算成两条历史，预期六条而实际五条。修正测试计数后 `f` 全组通过，未放宽正式校验；远端失败日志保留。原始结果、路径和二进制摘要见 [真库及启动结果](../../acceptance/oa-initiation-repair/evidence/profile-correction-results-20261002.json)。

浏览器前两次发现保存正常点击实际打开架构透镜：抽屉 layer 40，透镜触发器 75；首个单类选择器又被后载 CSS 覆盖，不能记为成功。最终使用更明确的抽屉选择器使 mask 为 100，并修正 hidden 重新加载按钮被 `.ant-btn` 显示规则覆盖的问题。最终 1280×720 下 BODY portal、mask 高 720／top 0，内容 client 与 scroll 均 662；保存 top 639.375／bottom 671.375，中心 hit-test 为“保存”，普通点击确实产生更正。最终 opt-in 浏览器探针通过，344.67 秒；两次派发、两次旧更新、两次完成、两次更正、零生产分工。见 [最终合成状态及源码摘要](../../acceptance/oa-initiation-repair/browser-profile-synthetic-state-20261002.json)。工具内截图已检查，未另保存可交付截图文件；不假称截图归档或全站尺寸验收。

没有启动本地 Docker。ECS 测试 runner＋私有 PG 上限 0.45 CPU／448 MiB；独立 server＋PG 上限 0.4 CPU／352 MiB，无公布端口。两批探针、私有 PG 和合成浏览器均停止，临时页面关闭；restore20 未重开。Main 13 库 canonical 摘要、17 容器 ID 不变。诊断容器、文件及独立启动空库／专用配置保留；未读取或输出私有密钥，没有创建正式用户或升级 Main。

## 下一切片及未关闭门禁

1. P01 项目管理：默认列表、筛选／年份／时间线、包与子项目语境、返回和刷新状态，不能只保留 Project ID。
2. P02 剩余：两种负责人更换／清除、资质与管理费／甲方目标映射；保持分别授权、版本化历史和外部事实边界。
3. 经营、四类型生产、附件、资料、台账和统计逐操作接通；原站冲突／占位单列，不使用成功提示伪造保存。
4. 候选镜像、Manifest／新授权版本兼容、联合备份离机／空目标恢复和成套升级后，补正式登录及公网浏览器。新 schema 10 不以旧 Project48 直接启动当作回退证明。

当前没有将分层证据拼成一次正式用户端到端成功，也没有给出整站完成百分比。
