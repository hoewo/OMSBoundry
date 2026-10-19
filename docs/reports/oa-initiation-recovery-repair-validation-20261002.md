# OA 立项故障修复与验证

日期：2026-10-02  
关联决策：BF-090、BF-091  
入口：[Auth58 公网工作台](https://oatest.antuinfo.cn/workbench/)

结论：**本次旧待立项恢复／提交阻断已修复并部署 Auth58 Main**。真实公网 Chrome 明确补齐经营负责人并更换已有生产负责人，完成同一 Project、进入同一 OA 详情、刷新复读通过；原分工结束历史和全部原有 Project 行保持。用户“测试项目101”只读、未代选或提交。完整 OA、完整立项表单和原站现场复核仍未验收。

## 修复范围

问题 A → 可验证结果 B：旧待立项项目缺经营分工、已有生产分工时无法继续 → 在原完成抽屉明确补齐缺失经营负责人，回填或按权更换生产负责人，保存后完成同一 Project 并进入同一 OA 详情。

Project 新增 `POST /api/v1/projects/{id}/initiation-info-changes`，只处理 draft／pending_completion。资料使用既有 update，经营／生产分工分别使用既有管理能力；资料、结束旧分工、新分工、revision、历史及回执同事务保存。旧 active-only basic-info-changes、schema 10、IAM 能力和授权不变。历史种类和命令摘要与 active 编辑隔离，失败事务不留下部分调整。

前端保留原 OA 抽屉和顺序；仅旧数据缺经营分工时显示补齐选择，不自动选人。已有生产负责人回填，空值不清除；更换留原分工快照和结束历史。提供真实“重新加载资料”按钮，未知网络结果重试使用原命令编号和 revision。

不直接修补用户“测试项目101”，不修改人员名册或外部业务事实。费率、甲方、资质在完成表单的禁用占位未由本修复解决，不能因此关闭 I01～I04 全项或完整仿 OA。

## 已运行检查

- Project 与 Workbench 全部 Go 包单元测试通过；立项运行脚本 Node 29 项通过，包括缺经营负责人零写、原子补齐及更换、相同／空负责人保持、保存及激活响应丢失重试。
- Auth58 私有 PostgreSQL 18 串行执行 Project 114、数据库 3、HTTP 86 项（含子测试），全部通过；新增旧项目恢复用例验证事务回滚、旧分工历史、同人不改、revision、租户、状态和跨接口 command 冲突。Main 事实／容器未改变，演练容器结束即停止。
- 新 Project 二进制以独立空数据库启动和重启，schema 10 不变、事实保持，9 个匿名受保护原生路由均拒绝；私有 PG＋进程合计上限 0.4 CPU／352 MiB。该证明不等于全 Suite 独立旅程。

## 公网发布和真实点击

开发测试版本 `v0.1.0-oa.20261002.2`，Linux/amd64，`official_release=false`、dirty source commit unknown。Source SHA-256：`be4ae399624784f880d34e1ac37cd872cbc44c3ac20ad1cc6e2bf91ef4d5cb1c`。

ACR 已发布并逐字节复核二进制：Project digest `0ebc8db34163de414d43009e075399201b2625cf53c71987f32738abca80bd1f`；Workbench digest `75528a75629b3662959bc685da644717481edc3811469c16ae2037603a45b198`。只替换 Project、Workbench，其他 15 个容器 ID 不变；schema 10、IAM 已有 Manifest／Policy／授权及挂载路径均不变，未授新 Grant。D1 check 与 17/17 healthy 通过。

真实 native Chrome 使用既有登录会话，在公网新开一个验证标签；用户原标签保留。先只读打开“测试项目101”的完成抽屉：缺经营分工时显示明确补齐选择，咨询／勘察及已有生产负责人正确回填；点击“重新加载资料”值保持，随后取消，没有选人或保存。

专用非真实夹具 `OA-RECOVERY-20261002-B`，Project ID `5db2eb3f-7125-4190-bbb0-59ce0beab79d`，由 API 设置为“缺经营负责人、已有生产分工、待补全 revision 3”；该准备步骤不是浏览器证据。随后真实点击：

1. 在原完成抽屉明确选择缺失的经营负责人，并选择另一 Person 更换生产负责人；测试目录存在同名标签，具体不同 ID 由原生 API 复核。
2. 点“完成立项并进入详情”，抽屉关闭，进入 `/workbench/?project_id=…&tab=basic`，仍是同一 OA 项目，未跳独立 Project 产品。
3. 刷新 Chrome，项目类型、资料和两类负责人仍在同一详情。只读复核 active revision 5；revision 4 原子保存补齐／更换，旧生产分工结束、快照保留；revision 5 激活，只出现一条新命令历史和一次 activation。

夹具以外的全部既有 Project 表记录及用户项目／分工／历史摘要相等；“测试项目101”仍为原 revision 6，等待用户明确选择经营负责人后提交，不用新建干净项目绕过。正常 IAM 登录、换票和审计可产生记录，不声称 IAM 库完全不变。本次只覆盖该管理员恢复路径；全角色撤权、原站真人、完整 I01～I04 和全部屏幕尺寸未验收。截图仅内联检查，无归档截图包。

## 备份与恢复

发布前串行联合备份 13 库、Evidence 文件和 IAM key；13 份 dump 逐个恢复到新空数据库，表事实／sequence 相等，文件及 key 字节相等；schema 10 不变，重复 migration 和已有 Project 事实保持通过。只运行一个有上限的私有 PG，完成即停止。**这是数据／文件空目标恢复，不是完整 Suite 恢复后的运行旅程。**

ECS 备份 `/var/backups/omsbizfoundry/oa-public-20261002b`；离机副本 `/Users/crispydog/Documents/aliyun/zhangfeng-antuiinfo/oa-recovery-backup-20261002.jRGK8L/oa-public-20261002b-backup.tar.gz`，211761 bytes、目录 0700／归档 0600。两侧 SHA-256：`c115b4ce647ac679edfcfcb65f679cba55f960d20278f0a98724ebbdeabb91f9`；备份私有内容未放入仓库。全程不启动本地 Docker、不重开 restore20、不删业务数据或卷。

运维仍使用 ECS `/opt/omsbizfoundry/acceptance/oa-public-release-20261002a/d1-active`，它加载的 override 与本次契约挂载相同，读取已更新的镜像 env。不得绕过它用旧 D1 默认配置重建 IAM。

## 失败记录和证据

首次公网复读误把默认详情的“当前分工”当成包含历史，assignment 数量断言失败。修正验收脚本为显式 `include_inactive=true` 后，只读重跑通过，没有重做提交或放宽应用校验。部署脚本旧 stdout 的“其他 14 个”是旧批次固定文案，本次以 rollout JSON 的 **15 个**为准。

- [私有 PostgreSQL／独立启动](../../acceptance/oa-initiation-repair/evidence/recovery-20261002/private-validation.json)
- [发布、空目标恢复、部署和原生复读](../../acceptance/oa-initiation-repair/evidence/recovery-20261002/deployment-readback.json)
- [真实浏览器观察及首次复读失败](../../acceptance/oa-initiation-repair/evidence/recovery-20261002/browser-observation.json)
- [用户记录部署前只读](../../acceptance/oa-initiation-repair/evidence/recovery-20261002/user-project-before.json)
- [用户记录部署后只读](../../acceptance/oa-initiation-repair/evidence/recovery-20261002/user-project-after-readonly.json)

下一步仍按[原 OA 逐步清单](../specs/assembly/oa-original-usage-workflow-baseline.md)补完整立项字段和权限矩阵，再逐栏目接通。此故障关闭不改变总报告 `insufficient-evidence`，不表示经营／生产、资料统计、Evidence OSS 或标准前端 OSS 分离已经完成。
