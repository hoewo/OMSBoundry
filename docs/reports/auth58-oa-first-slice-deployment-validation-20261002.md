# Auth58 原 OA 修复切片部署和有限验证

日期：2026-10-02  
版本：`v0.1.0-oa.20261002.1`，Linux/amd64 开发测试制品，`official_release=false`  
入口：[公网 OA 工作台](https://oatest.antuinfo.cn/workbench/)  
本批次结论：BF-084～BF-089 的已实现切片部署 Main；专用新夹具的立项、后补生产负责人、基本资料及甲方保存取得同一次公网浏览器＋真实 IAM／Gateway／Project／PostgreSQL 证据。**.1 当时用户“测试项目101”无法立项，完整立项／OA 未验收通过。**

后续 `.2`（BF-091）已经实际修复该类旧待立项恢复阻断，真实公网补齐／更换、完成同一项目、进入同 OA 详情和刷新复读通过；用户原记录未代选或提交。见[恢复修复报告](./oa-initiation-recovery-repair-validation-20261002.md)。下文 .1 的反例、失败和有限证据保留其发生时范围，不是当前版本状态；完整表单及 OA 仍未验收。

## 用户反例和结论修正

本报告此前的“首条修复链通过”只能覆盖下文那一个新夹具的正常路径，不能证明旧草稿、已有负责人、分段失败后重试或完整立项表单可用。用户实际操作反例有效，不能让用户重新创建干净项目绕过。

2026-10-02 11:30（北京时间）只读检查 `OA-2026-101`：同一 Project 停在 `pending_completion` revision 6，咨询＋勘察；只有 revision 5 创建的生产负责人，没有经营负责人。历史 revision 2、4、6 均记录 `missing=[business_owner]`。当前完成抽屉没有补齐经营负责人的可用操作；重新打开又清空生产选择，选择不同于已有分工的人员时前端拒绝更换。这是缺失恢复路径，不是用户操作错误。

原 OA 源码明确在派发时选择“项目经营负责人”，完成抽屉可选“项目生产负责人”；后者有管理权限才显示且无必填规则，基本信息中可后补／调整。不能仅凭模糊记忆改字段，也不能把“保护历史”变成用户无法完成原操作的借口。原完成表单的费率、甲方、资质可录入，当前禁用占位亦未等效。

本次读取经过公网 Nginx／Gateway／IAM，详情／历史与 Project 前后事实摘要核对；无业务或 Grant 写入，未改用户记录、未部署。正常身份会话／签发／审计仍产生。见[脱敏只读证据](../../acceptance/oa-initiation-repair/evidence/user-initiation-counterexample-readonly-20261002.json)。没有复现截图中那次选择人员的点击，不虚报浏览器通过。

按 BF-090 暂停新 B02 功能，先执行[原 OA 分步清单](../specs/assembly/oa-original-usage-workflow-baseline.md)：普通项目立项及旧数据、已有分工、部分保存、刷新／重试全部关闭后才推进经营和生产。此前反例检查是拆解和诊断，当时故障未修复；后续恢复实现见上方 .2 报告。

## 本次解决的问题

问题 A → 可验证结果 B：候选修复停留在源码和隔离验证，公网仍跳进独立 Project 页面 → 将已验证切片实际发布到 Auth58，使普通用户在同一 OA 页面完成派发、立项、详情和后补。

不重新设计原 OA 流程，不复制旧共享表。Project 仍拥有项目及项目专属联系上下文，People 拥有人员，Party 主档未变；Nginx → Gateway → IAM／原生目标应用链不变。没有新增小系统。

## 实际部署范围

| 项目 | 实际结果 |
| --- | --- |
| Project | 新 server／ops／workload-keyctl；schema 7 → 10 前向迁移 |
| Workbench | 新统一 OA 目录、详情、抽屉和甲方表单；保留已有镜像非 root、CA 等运行环境 |
| IAM | 保留 IAM8 二进制；显式选 Project Capability Manifest 0.6 与 Assignment Policy 1.2，旧策略文件不覆盖 |
| 账号 | 只给既有全权测试管理员补 `profile.correct`、`counterparty-context.read`、`counterparty-context.manage` 三项 Project 原生 Grant；Overlay 1.3 未批量应用到其他账号 |
| 其他组件 | 其余 14 个长期容器 ID 不变；Gateway、Suite Manifest 和其他业务镜像未替换 |

两镜像已推入用户自己的 ACR，按不可变 digest 拉取并校验镜像内四个二进制的字节摘要：

- Project：`sha256:88d9549b575016a79e108bdeb5a501ee0f82e071cb7b030f0242451ccb491ac4`
- Workbench：`sha256:6971dc78288ca9fc88c2a184fdb047738245df5d7ba4bae87b42337a8a65d16d`
- 源码归档：`93c1aecdd953f290a1e1a7d2c1a6d8b67dd24feca048e11ff0e72d658dffc3a7`

工作树不是 clean HEAD，所以 commit 明确记为 `unknown`；不能称为 official release。完整二进制和契约摘要见[机器证据](../../acceptance/oa-initiation-repair/evidence/public-release-results-20261002.json)。

## 实际验收

公网 Chrome 原生点击创建专用虚拟夹具 `OA 公网立项复核 20261002A（验收夹具）`，Project ID `b4283968-99d5-40c0-9d1f-3802d3d2c9fb`，编号 `OA-VERIFY-20261002-A`：

1. 默认项目管理 → 项目立项 → 只填名称、经营负责人，创建待立项成功。
2. 补编号和设计类型，生产负责人保持空；完成后直接进入 `/workbench/?project_id=…&tab=basic`，没有打开独立 Project 系统。
3. 同一基本信息抽屉补生产负责人并修改概况，真实保存成功。
4. 经营信息 → 甲方信息，保存虚拟名称／联系人／电话；刷新仍在同项目经营页，原值复读成功。
5. 甲方名称前缀出现幽灵补全，Tab 接受并移到联系人；随后取消，未提交第二次甲方写入。
6. 返回项目管理列表并重新打开同一详情成功。

独立只读复核确认：activation 在 revision 3，选用 `business-owner-v1`；生产负责人创建 revision 晚于 activation；最终 revision 5。原有 Project 七张业务表记录保持原摘要，其他 11 个数据库全表事实与 sequence 值保持备份前摘要。复核 GET 自身零 Project 写入。IAM 正常产生登录、能力签发、审计及上述三项显式 Grant，不能把 IAM 库也描述成完全无变化。

浏览器截图实际内联检查，**没有归档截图包**。工具异步读取曾遇到过期 AX 元素，刷新状态确认抽屉已打开；未据此重复创建夹具。测试结束后 Mac 自动锁定，未继续验证透镜重新打开。本轮只计管理员健康控制路径，不代替全角色、撤权、包／子项或原站真人全量验收。

## 备份、迁移和恢复

按 ADR-0035 先解析新 IAM 契约，再短暂停写 13 个有状态应用，联合备份全部 13 库、Evidence 文件及 IAM key 文件。已有事实在 Main 前向迁移前后保持不变；迁移幂等复演通过。

只启动一个有上限的私有 PostgreSQL：13 份 dump 逐个还原到新空数据库，表摘要和 sequence 相等；Evidence／IAM 文件还原到空目录后字节摘要相等；恢复出的 Project 7 → 10、check 和重复 migration 通过。私有 PG 随后停止。**这证明数据／文件空目标恢复，不证明完整 Suite 在恢复目标上的运行复演。**

备份根：`/var/backups/omsbizfoundry/oa-public-20261002a`。离机副本：`/Users/crispydog/Documents/aliyun/zhangfeng-antuiinfo/oa-public-backup-20261002.6Cq7Fz/oa-public-20261002a-backup.tar.gz`，目录 0700、归档 0600，207644 bytes；SHA-256 `50767f466ae52c73bea32cd4634936eb0b9b94d69b7a7601f481671d664fbee5` 两侧一致。没有把密码、环境文件正文、key、dump 放进仓库证据。

最终 D1 check 和 17/17 healthy 通过。全程没有启动本地 Docker，也没有重开 restore20；未删除任何业务数据或卷。

当前 Main 使用显式 Compose override。后续运维使用 ECS 上的入口：

```sh
sudo /opt/omsbizfoundry/acceptance/oa-public-release-20261002a/d1-active check
```

不要绕过该 wrapper 用旧 D1 配置重建 IAM，否则会丢失本次契约选择。新 schema 已应用，**禁止直接启动旧 Project 二进制回滚**；需要数据回退时先停写，将联合备份恢复到经过校验的新空目标，再完整切换入口，不覆盖当前库。

## 失败记录与未完成范围

发布脚本首次因第二次写结果时使用 O_EXCL 失败；修正元数据原子替换，并核对相同 source label 后恢复发布，最终两个 ACR digest 的二进制摘要均通过。公网只读 checker 首次把外部 JS 中的入口断言放在 HTML 上检查而失败；按实际资源分开读取后重跑通过。首次已授三项 Grant，第二次 `new_grants=[]` 只表示不重复授予，不表示本轮没有权限变更。

内置浏览器导航两次超时，改用 native Chrome 完成以上真实点击；接口成功没有冒充浏览器成功。本轮再次串行运行 Project／Workbench 普通 Go 测试、Node 32 项（零 fail／skip）、JS 语法、脚本语法和四程序交叉编译。普通 Go 命令不代替前期 opt-in 真库结果。

仍待完成：经营参与人 B02、资质／费率及其余字段、进度投影、其他经营栏目和分类型生产在同 OA 的完整操作接线、资料／统计、正式角色和撤权浏览器矩阵、项目包公网点击、完整恢复运行、clean-HEAD 正式交付。C4～C6／Evidence 候选 `test.50` 全闭包和原连续 C8 未由本轮完成；Evidence Main 仍是 Local Provider，OSS 文件运行接入及成熟前端 OSS 分离也是独立待办。

旧“测试项目101”缺少经营负责人，.1 未擅自补人或改历史记录；该批新夹具有限成功不关闭当时立项阻断。后续 .2 已修并验证同类旧状态恢复，仍不改变完整表单或总报告的 `insufficient-evidence`。以[原 OA 使用流程基线](../specs/assembly/oa-original-usage-workflow-baseline.md)逐动作接通，不能再按原工程 98% 推断产品可用率。
