# OMS 一期 Demo 全量业务数据归零报告

日期：2026-10-22  
决定：BF-170  
环境：`https://oatest.antuinfo.cn/`

## 目标与边界

当前环境仍是可反复走录入流程的 Demo。用户明确不保留旧演示与旧 OA 业务数据，因此本次将当前 Suite 的用户录入、业务事实和本地附件字节归零。

为保证清空后仍可登录并从头录入，以下系统基准不属于外部业务数据并予以保留：

- PostgreSQL schema、migration 记录和 Contract／Money 的应用技术元数据；
- IAM 管理员、登录、Session／签发所需配置与既有 Grant；
- D1 部署、镜像、密钥与 Gateway／Nginx 配置；
- Workbench 运行必需的本方经营主体 `95d3793f-7e0d-4ec9-acb8-9baf8bcb1039`。该主体不会作为外部甲方或施工方候选展示。

本次不新增产品内批量删除功能，不改变 Owner、API、schema、Capability、Grant、Suite Manifest 或数据所有权。

## 回滚点

执行前生成并复核完整备份：

`/var/backups/omsbizfoundry/20261022T023910Z`

`checksums.sha256` 覆盖 IAM、签名密钥、Suite 选择、十二个业务 Owner 及 Evidence／Contract 备份；15 项逐项校验均为 `OK`。

## 执行范围

在停止公网入口和业务服务后，清空以下 Owner 的用户录入／业务事实：

1. Contract Ledger
2. Money Flow Ledger
3. People & Groups
4. Party Directory（仅保留本方经营主体及其一组 revision／command／audit）
5. Project Workspace
6. Evidence Service
7. Business Engagement
8. Billing
9. Production Operations（数据库 `design_operations`）
10. Approval & Audit Results
11. Cost & Outsourcing
12. Bonus Distribution

同时删除 `omsbizfoundry_evidence-data` 与 `omsbizfoundry_contract-evidence` 中的业务对象字节。Party Directory 的不可变触发器按设计拒绝常规删除；维护事务在完整备份与停机窗口中使用 PostgreSQL `session_replication_role=replica`，仅删除本方主体以外的数据，事务结束即恢复正常触发器行为。

## 验证结果

- 十二个数据库逐表执行实际 `count(*)`：除 Party 基准的 `parties=1`、`party_revisions=1`、`party_commands=1`、`party_audit_events=1` 外，其余业务行均为 0；
- Evidence 本地对象文件：0；
- Contract 本地对象文件：0；
- D1 `check`：通过；
- 长期容器：17/17 healthy；
- 公网正式登录：通过；
- Gateway 授权：Party、Project、Billing、Money、Contract 五个 Audience 通过；
- 公网 Owner 只读：Project 0、Billing 0、Money 0、Party 1（仅本方主体）；
- Contract 业务表逐表总数：0。验收脚本中的 workspace `6` 是空上下文 JSON 的字段数量，不是六份合同；
- 公网验收业务写入：0；
- 当前公网页面 SHA-256：`4b7e3fe2c04f5c3a57228247a77e8231d3b879a7f621ae66535661e175044609`。

## 结论

`oatest` 当前已处于空业务状态，可以从新建外部甲方／施工方和项目开始重新走 OMS 一期录入流程。需要整体回滚时使用上述备份；不要把本次维护方式暴露为普通用户的删除入口。

## 后续纠正（BF-171）

上述首次核验遗漏了 Contract／Money workspace context 的 Party Directory authority：清理把两库的 runtime cutover 与 reviewed mapping 一并删除，虽然 schema、容器、列表读取全部健康，新建项目仍会因 `directory_authority_established=false` 被正确禁用。现已通过两个 Owner 的正式 migration／cutover 命令重建最小运行基准，并把 directory authority 加入公网强制验收。业务事实和附件仍为空；详细根因与证据见[BF-171 修复报告](./oms-phase1-party-runtime-reset-repair-20261022.md)。
