# OA 公网连续操作与导航修复

日期：2026-10-02  
关联约束：BF-084、BF-087、BF-090、BF-091、ADR-0035  
入口：[Auth58 OA 工作台](https://oatest.antuinfo.cn/workbench/)

本报告是 `.3` 历史批次快照。解锁后的第二次清除明确复现 CSRF 过期拒绝；随后 `.4` 已修复、沿用同一 C 完成清除／恢复及搜索、刷新、返回、Back／Forward，最终原生复读通过，见[续验报告](./oa-csrf-continuation-validation-20261002.md)。下文的未结束和失败保留其当时状态；完整 I01～I04／OA 仍未验收。

当时结论：**发现并修复了“当前立项抽屉刷新后跳回旧项目”的真实网页故障，已部署 Main `.3`。** 同一非真实项目的派发、必填拒绝、无生产负责人完成、取消编辑、后补负责人及无变化保存已取得公网点击与原生复读证据。负责人清除有一次未成功的尝试，原因未定；随后 Mac 锁屏阻断继续点击，整条连续验证未结束，不宣布 I01～I04 或完整 OA 通过。

## 故障与修改

问题 A → 可验证结果 B：从旧项目详情切到立项后，页面显示新项目但地址仍指向旧项目 → 立项列表、当前完成抽屉及详情各有明确导航状态，刷新、取消和恢复不串项目。

修复前真实 Chrome：夹具 C 的完成抽屉显示在前台，地址仍有夹具 B 的 ID；刷新打开 B 的已立项详情。没有丢失已保存事实，但未保存输入被刷新丢弃。这是导航状态错误，不是数据库写入失败。

Workbench 的 URL／history 增加明确 `view=initiation`，进入立项清除旧 Project／tab，打开抽屉写入当前 ID，取消清除 ID，恢复按 URL 返回相同工作面；旧草稿详情链接继续进入原完成抽屉。保留原 OA 布局和动作顺序，不另设计流程。Project、IAM、Gateway、业务契约、schema、Grant、数据所有权均未改动。

## 真实连续操作

夹具 C：`70edead7-9dc9-44a3-9b35-b9d41b83264e`，名称“OA 连续操作复测 20261002C（验收夹具）”，编号 `OA-VERIFY-20261002-C`。它由真实公网页面创建一次，使用既有虚拟 D1 Person；复读脚本不创建业务、不补授权。

| 操作 | 公网页面与原生复读 |
| --- | --- |
| 派发名称＋经营负责人 | 自动打开当前完成抽屉；draft revision 1，一个分工／一个历史事件 |
| 空编号、空类型提交 | 分别显示必填错误，保持 revision 1，不产生业务历史 |
| `.3` 抽屉刷新、取消、重新打开 | 刷新仍是 C；取消回立项列表并清除 ID；重新打开同一 C，无重复创建 |
| 正式编号＋设计／咨询，生产负责人空 | 完成立项进入同一 OA 详情；active revision 3，只有经营分工，一次 activation |
| 编辑取消、空类型保存拒绝 | 原资料、分工、历史和回执均保持 revision 3 |
| 基本资料与后补生产负责人一次保存 | 日期 2026-10-02、概况及两个负责人复读一致；revision 4、两个当前分工、四个历史事件 |
| 无变化保存 | Project／分工／历史仍为 revision 4；只增加一个命令回执，不增加业务 revision 或事件 |
| 清除生产负责人第一次尝试 | 未成功：仍有生产分工，revision 4／历史四条，回执从五到六；不记通过 |

最后一项选择空值后没有单独确认下拉框已取空值，不能据此判断是页面清除逻辑故障还是原生输入未生效。单元检查确认显式空值会发送 clear，未变化或 disabled 不发送 clear；这不是公网清除成功的替代证据。待解锁后先确认选中值再保存，继续清除／恢复历史与列表搜索、返回、Back／Forward。

用户“测试项目101”及此前全部 Project 行保持；旧 A／B 夹具冻结。锁屏后只读边界复核确认其资料／分工／历史摘要、Grant 和其他 11 个业务数据库相等。读取只产生正常 IAM 登录、换票、注销和审计，不声称 IAM 整库不变。

## 回归与交付

实际串行运行：Workbench `GOMAXPROCS=2 go test -p 1 ./...`、`go vet -p 1 ./...`；Node runtime 37＋directory 7 项，零失败／skip；Python 纯验收门禁 6 项通过，明确拒绝本次清除未生效、revision 对但当前分工不对等反例；JS syntax、四个 Python 运维／复读脚本编译及 `git diff --check` 通过。Go 的 opt-in 合成 browser probe 未在本批启动，真实点击使用 native Chrome；原 OA 仓库仍 clean，本批没有原站现场浏览器对照。

开发测试发布 `v0.1.0-oa.20261002.3`、Linux/amd64、`official_release=false`、dirty source commit unknown：

- Source SHA-256：`3f8e9b10ff731254c2dc1236cb216d21688f267c640fb26150409e6e2c5a0f95`。
- Workbench binary SHA-256：`6b226734de0e93f06f490396ade59e278db66ca41589223fe476b3bd786e7629`。
- Runtime JS SHA-256：`24f15d3af276c8cd7561cde0020c52f6a105d9b74ae02361fb1c0e7f099d5bcb`。
- ACR Workbench digest：`bfd416532c8c32ab925f994e764ff3c8a338c68ac11d06d093b3f6c1c9baa6e1`。

publication 字节复核、Workbench-only closure、D1 check、17/17 healthy 通过。只重建 Workbench，其他 16 容器 ID 不变；Project schema 10、IAM 契约／Policy／Grant、Suite Manifest 和其他业务镜像不变。复读核对运行 image ID 和公网 runtime 字节属于此次发布，不靠页面缓存推断版本。

首次 prepare 在 publication 尚未完成时被 `published.passed` 门禁拒绝，发生在备份、env 和容器变更前；等待 publication 完成后再执行才通过。该拒绝保留，不描述为一次无失败部署。

## 备份与负载边界

13 库、Evidence 文件及 IAM key 联合冷备份；13 个空目标数据库逐个还原的事实／sequence、文件与 key 字节相等。没有 schema migration，既有 Project 事实保持。**这不是完整 Suite 恢复后运行验收。** 私有 PG 已停止，restore20 无运行容器；没有启动本地 Docker。构建和验证分批限资源，没有同时启动多个验证栈。

ECS 备份：`/var/backups/omsbizfoundry/oa-public-20261002c`。离机副本：`/Users/crispydog/Documents/aliyun/zhangfeng-antuiinfo/oa-navigation-backup-20261002.RYhDap/oa-public-20261002c-backup.tar.gz`，214978 bytes，目录 0700／文件 0600，SHA-256 `eb82f2bb0e3efd2932a929fcd18031999391023d6b817f27822b6ff50052451e`。私有备份不进入仓库。

运维仍使用 ECS `/opt/omsbizfoundry/acceptance/oa-public-release-20261002a/d1-active`，保留现有 IAM override；不能用旧默认装配重建 IAM。

## 证据与下一步

- [真实点击及失败观察](../../acceptance/oa-initiation-repair/evidence/navigation-20261002/browser-observation.json)
- [分阶段原生复读、冻结数据和未结束状态](../../acceptance/oa-initiation-repair/evidence/navigation-20261002/native-checkpoints.json)
- [发布、备份、空目标恢复和检查](../../acceptance/oa-initiation-repair/evidence/navigation-20261002/deployment-readback.json)
- [只读复读脚本](../../acceptance/oa-initiation-repair/verify-public-continuous-browser-ecs.py)

早期复读脚本的 `passed` 只代表读取与冻结数据检查，不能证明清除动作成功。已保留原 `production-clear-attempt1.json`，新增每阶段 expected status／revision／当前分工断言，失败也保存观察后退出；汇总证据明确拆开读取通过与操作未通过，不修改旧证据冒充成功。最终阶段还要求清除／恢复历史、取消和 no-op 的精确增量；当前没有最终通过记录。

恢复浏览器后继续同一 C，不新建夹具逃避失败。完整立项费率／甲方／资质字段、项目包、人员筛选／撤权／部分成功矩阵、原站真人与后续经营／生产／资料统计仍按[分步清单](../specs/assembly/oa-original-usage-workflow-baseline.md)待办；Evidence OSS、标准前端 OSS、完整 C8／Suite 恢复和正式 clean-HEAD 交付也未关闭。总报告继续 `insufficient-evidence`。
