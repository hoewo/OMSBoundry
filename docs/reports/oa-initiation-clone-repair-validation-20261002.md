# 原 OA 立项操作修复验证

日期：2026-10-02  
状态：首条操作链候选实现通过组件回归及合成浏览器；完整仿制仍在实施，Auth58 Main 未更新  
关联：BF-084、[原 OA 使用流程基线](../specs/assembly/oa-original-usage-workflow-baseline.md)、[验收入口](../../acceptance/oa-initiation-repair/README.md)

后续同日更新：active 项目的核心基本资料编辑已在 BF-085 接通并获得分层证据；本报告的“基本信息编辑待实施”保留为首片段当时状态，最新范围和仍未关闭字段见[后续修复报告](./oa-profile-clone-repair-validation-20261002.md)。Main 仍未更新。

## 结论

已把立项第一段改为原 OA 的操作顺序：只填名称和经营负责人派发，自动打开补全；普通项目在完成前填正式编号及类型，项目包编号选填；不强制此时填写生产负责人；完成进入同一工作台的同一项目基本详情，生产负责人可后补。抽屉改为全视口及内部滚动，避免被页面容器高度裁切。

这是候选代码和隔离验证结果。公网当前仍是旧版本，完整原 OA 的字段、经营、生产、包管理、文件、统计和权限操作尚未全部接通。I01、I03、I04 不能写成全量验收完成，原站真人对照也未取得。

## 实现及边界

| 操作 | 候选实现 | 限制 |
| --- | --- | --- |
| 派发普通项目或包 | Project `InitiationDispatch v1` 在一个事务写 draft、经营 Assignment、命令及历史；使用同 Tenant 有效 Person revision | 需要创建／层级和经营分工原生能力；不是 Workbench 写业务表 |
| 补全并完成 | 保存核心资料后，显式选择闭合 `business-owner-v1` 门槛；保留旧默认 `dual-owner-v1` 和省略 profile 的命令 hash | 生产后补不绕过 scope、状态、版本、完整性或 complete 能力 |
| 进入同项目详情 | Workbench 读取 Project 与 Assignment，URL 保留 Project ID；不跳独立 Project 产品 | 当前真实详情只有核心基本字段；经营／生产标签仍待接线 |
| 后补生产负责人 | 追加原生 Assignment 并复读 | 负责人更换尚未接 UI，不覆盖已有分工历史 |
| 布局 | 抽屉移到 BODY，100dvh，头部与可滚动内容分离 | 仅实测两种 720px 高度视口；全站响应式和逐屏对照未完成 |

000010 只放宽 draft／pending 的编号及类型数据库约束，active 仍须满足正式资料约束；没有改写旧项目或引入新服务、共享表。People、IAM、Gateway、Evidence 和各事实所有者边界不变。

## 实际运行的检查

本机 Project 与 Workbench 分别串行 `GOMAXPROCS=2 GOWORK=off go test -p 1 ./...`、`go vet -p 1 ./...` 通过；Workbench Node 9 项行为测试、独立运行时与嵌入主 JavaScript 语法检查通过。未启动本地 Docker；本地缺 PostgreSQL 的 integration 测试编译／跳过不算真库通过。

ECS 使用独立私有 PG18，无公布端口，单测试 runner＋PG 最高 0.45 CPU／448 MiB。Project 分组 90、migration 分组 3、HTTP 分组 46 条 PASS 日志记录分别通过，计数包含子测试。HTTP 真库补了普通项目／包派发、上游不可用时成功重放、补资料、无生产负责人完成及后补后的同 ID／revision 检查；component 权限测试补创建与分工组合、profile 不绕 scope／能力、跨租户和无效 Person 零写入。

最初两个批次失败保留：第一批旧恢复测试写死 schema 9，第二批旧 HTTP 夹具缺正式 Contract reference v1 字段。分别修正测试预期和夹具后重跑，没有放宽正式 reference 校验。`b` 整批为 false，`c` 仅 HTTP 为 true，不把分组结果拼成一次 clean run。三个批次 Main 13 库 canonical 摘要和 17 容器 ID 均不变；探针停止，restore20 没有重开。原始远端目录及二进制摘要见验收入口与 [结果文件](../../acceptance/oa-initiation-repair/evidence/postgres-results-20261002.json)。

实际浏览器使用同一候选 HTML／CSS／JS，但 Gateway、People、Project 是明确合成端口。完成普通项目与包共两次派发、两次资料更新、两次完成及一次生产分工；刷新保持同 ID，返回列表显示已立项项目。1280×720、1440×720 下 panel 高 720，内容 client 645／scroll 879，mask 父为 BODY，提交按钮可滚动点击；浏览器没有 error／warning。状态见 [合成证据](../../acceptance/oa-initiation-repair/browser-synthetic-state-20261002.json)。这些不能替代正式 IAM、真实数据库持久化或公网链路。

## 尚未关闭

- I02 的默认“关于我的／全部”权限切换；P01 年份／时间线、筛选、分页、项目包语境和返回位置。
- 甲方、资质和管理费等字段的目标所有权及版本化写入；当前明确禁用，不假保存。生产负责人更换、基本信息编辑、四类型生产和多类型并集仍待实施。
- 命令未知结果的同页幂等续跑已有测试，但刷新后的 intent 恢复、撤权及完整失败浏览器矩阵未完成。
- 原 OA 其余动作按操作 ID 逐一实现与实测；原站占位或需求／源码冲突单列，不仿制成假功能。
- Auth58 候选固定、当前 Manifest／IAM 的实际兼容、联合备份离机及空目标恢复、成套升级、正式登录和公网浏览器尚未完成；旧 Project48 不能直接回读新 schema 10 冒充回退。

新 Project 二进制已实际独立启动：自己的空库自动迁移至 000010，IAM verifier 读取公开测试 JWKS，匿名原生列表和派发均 401；隔离服务重启后逐表事实摘要不变。服务＋PG 上限 0.4 CPU／352 MiB、无公网端口；结束均停止，Main 13 库及 17 容器 ID 不变。其私有测试库、非超级用户 owner、只用于该库的私有配置和日志保留用于诊断，未删除；没有创建正式账号、签发正式业务凭证或更新主站。见 [独立启动结果](../../acceptance/oa-initiation-repair/evidence/server-startup-results-20261002.json)。

独立启动通过不关闭上述业务和正式发布门禁。完整 OA 总结仍为 `insufficient-evidence`，不能用这条组件链重置总验收状态或估算为 98%。
