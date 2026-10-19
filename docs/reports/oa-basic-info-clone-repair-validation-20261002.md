# 原 OA 基本信息与负责人修复验证

日期：2026-10-02  
状态：核心资料与两类分工候选分层验证通过；P02 全项和 Main 未完成  
关联：BF-087、[原使用流程](../specs/assembly/oa-original-usage-workflow-baseline.md)

## 结论

已按原 OA 把核心资料、经营负责人和生产负责人放进同一个“编辑基本信息”抽屉，分别控制编辑权限，一次保存。去除先前自创的单独“补充生产负责人”按钮；生产负责人仍可在立项后调整。**候选尚未部署 Auth58 Main，完整 OA 不因此通过。**

问题 A → 结果 B：同一基本信息编辑里改资料和换负责人会部分成功、入口偏离原站 → 一次 Project 原生命令原子保存，保留原分工历史，并可安全重试。

只读核对原提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302` 的 `BasicInfoTab.tsx`、`useProjectPermissions.ts` 和对应 profile 提交：单一编辑抽屉、760px 宽度、基本资料与两类分配权限分开、负责人可不设置或清除。原工作树仍干净。没有运行原站浏览器或证明原站全部行为；资质、费率、甲方及候选人员筛选没有伪装成已复制。

## 实现边界

[Basic Info Change v1](../../arckit/spec/project-workspace/basic-info-change.md) 只改变 active Project 自有核心字段和显式 set／clear 的两类分工。每个被提交字段分别要求既有能力；错误角色、越权、People 不可用或版本无效在写入前拒绝。数据库按 Project revision 锁定，编号冲突、future 分工或并发使整个事务回滚。旧 PUT、Profile Correction、Assignment 和 schema 10 不变，没有新增能力、客户岗位或服务。

旧 Assignment 仅结束有效期，身份、source revision 和 display snapshot 不覆盖；新增分工单独建记录。实际变化只增加一次 Project revision／历史，no-op 只留命令回执，重放不重新校验 People。Client Party 快照、父子关系、Resource Link、外部已确认事实保持不变；不写 People／IAM。默认详情只显示当前分工；`include_inactive=true` 查询历史，不把结束记录当现任。

UI 只给有相应权限的字段启用编辑；People 不可用时不隐式清空负责人。失联复用同一命令，409 保留输入并要求重新加载；成功保存但复读失败禁止再次提交。前端技能帮助忠实核对原操作和真实点击／滚动，文档技能同步仓库决策和分层证据，未另创产品设计。

## 实际验证

| 层级 | 已运行结果 | 限制 |
| --- | --- | --- |
| 本地无容器 | Project／Workbench 普通 Go 测试与 vet、Node 24 项、JS 语法通过 | 不是正式身份／持久化链 |
| 私有 PG18 `j` | Project 104／HTTP 66 条 PASS 记录（含子测试），普通项目和包原子资料＋换人、清除、no-op、历史、幂等、stale、跨 Tenant、编号冲突整体回滚、并发和 future 分工保护通过 | 原组件测试 Provider，不是公网组合 |
| 独立启动 `k` | 自有空库迁移 schema 10、非 root 只读进程；五个匿名原生路由 401；重启事实不变；Main 不变 | 测试 JWKS，不签正式身份 |
| 追加 HTTP 真库 `m` | 修正当前／历史断言后 66 条含子测试 PASS；普通项目及包一次提交 core＋双 clear、当前视图零分工／历史保留、People 503 零写入、恢复与离线重放通过 | Template IAM 与 HTTP People 端口；不是正式公网用户 |
| 合成浏览器 | 普通项目与未编号包保存／刷新、清除、必填拒绝、no-op、三类分权和 1280×720 抽屉滚动可达通过 | 明示替身，不是 Gateway／IAM／PG 一次联合成功 |

普通项目从 revision 3 变 4（核心与双负责人），再次未改保存保持 4，清除生产负责人变 5；无效空名称未发送写请求。包一次保存由 3 变 4，编号仍为空且两子项上下文保留。四次命令计数包含一次 no-op，其余旧 update／assignment／correction／dispatch／complete 均为 0。三类受限编辑另起独立探针，各只提交一次；生产／经营分配权不解锁 core，core 权不解锁任一负责人。负责人仍从当前 active People 中选，原候选来源与离职人员展示尚未等效验证。后续查实际 SQL 纠正本报告原“岗位候选筛选”误述：当前原函数不按经营／生产岗位筛选，包含离职人员，详见[B02 对照](../specs/assembly/oa-business-personnel-clone-gap.md)；旧运行结果不变。

抽屉初始保存按钮 y=799 不在 720px 视口；实际鼠标滚动后 y=666、center hit 为按钮，正常点击成功。不是靠强制点击证明可用。截图仅工具内检查，未宣称文件归档或所有尺寸通过。控制台无 error。

首轮 missing owner operation 返回 500，已显式修正为 400，HTTP 负例重测通过。受限浏览器首轮夹具把 active detail 权限写死为全权，零写入结束后修正并重跑；不计为分权通过。追加 HTTP 真库 `l` 默认 GET 只返回当前分工，却错误断言应含结束历史而失败；诊断保留，改为分别检查默认当前视图及 `include_inactive` 历史视图，`m` 修正后复跑全部通过。最终浏览器夹具也对齐该原生契约，另复测更换后当前人及清除后不显示旧人（两次命令、revision 5）。

结果与摘要：[真库／启动](../../acceptance/oa-initiation-repair/evidence/basic-info-results-20261002.json)、[合成浏览器](../../acceptance/oa-initiation-repair/browser-basic-info-synthetic-state-20261002.json)。全权、production-only、business-only、profile-only 和最终当前视图探针分别正常结束（258.42、46.30、47.90、63.64、71.73 秒），不能把不同探针接成一次正式用户旅程。

## 资源和未完成事项

没有本地 Docker；ECS 测试串行，runner＋PG 上限 0.45 CPU／448 MiB，独立 server＋PG 上限 0.4 CPU／352 MiB，无端口发布。`j`／`k`／`l`／`m` 的 Main 13 库摘要／17 容器 ID 未变，诊断保留；全部探针与 PG stopped／无 OOM，只有失败 `l` 为 exit 1，其余 exit 0；restore20 未重开。临时浏览器与本地探针关闭，不发布镜像或改正式 Grant。

其余 P02 字段、原站候选规则、P01 经营／生产进度、项目包调整、经营、四种生产、文件、资料、台账和统计仍逐项接通。正式兼容发布、联合备份／空目标恢复及真实公网浏览器是独立门禁。详细复验入口见[验收说明](../../acceptance/oa-initiation-repair/README.md)。
