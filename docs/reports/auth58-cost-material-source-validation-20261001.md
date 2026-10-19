# Auth58 Cost／Outsourcing 资料来源（2026-10-01）

结论：C6 两个只读资料端口的源码、真实 PostgreSQL 18 与真实 IAM8 检查通过。主站未更新；写入前 Project／Party／Evidence 引用是显式测试提供者，不能解释为完整跨进程 C6／C8 通过。

## 范围与实际证据

Cost Entry 与 Outsourcing Engagement 分别发布当前自有 snapshot 的 File ID／冻结 Version／slot、Project、状态、revision 和排序 items 的 SHA-256。沿用各自原生 read 和精确 Cost／Engagement、Project 或 Tenant Scope，不要求 amount.read；不含金额、Party、合同、评分、应付建议、Object Key 或下载地址。空是明确数组，撤回保留引用并明确状态；没有新服务、migration 或写命令。

- 全包 Go 测试／vet、两包 race 通过。本地 integration 编译／SKIP 不算真库证据。
- ECS `TestCostMaterialPostgresBothAggregatesCurrentHistoryAndZeroWrites` PASS、无 SKIP：两个聚合替换文件、保留旧 revision、撤回保留新版本；只读时十张自有表 canonical 相等，不能调用上游 resolver。
- ECS `TestCostMaterialHTTPPostgresNativeIAMBothAggregates` PASS、无 SKIP：真实 IAM8 单原生动作签发和 Cost 正式 verifier；接受三个 Scope，拒绝匿名、相邻资源、只有 amount.read、错 Tenant，以及 Cost read 调用 Engagement 和反向调用；响应 no-store，十表 GET 前后不变。
- 私有 internal 网络、零公开端口、总限额 0.55 CPU／416 MiB；结束 PG／runner 全部停止，未移除容器、卷、私有 IAM 数据或诊断。主站 13 库摘要／17 容器 ID 前后及停止后不变，restore20 保持停止。

入口：[专项说明](../../acceptance/d1-c6-cost-material-source/README.md)。ECS 原始目录 `/opt/omsbizfoundry/acceptance/cost-material-source-20261001/cost_material_source_auth58_20261001a/`；results／binaries 离机副本使用独立文件名保存。SHA-256：results `03e94a19fa5398c2ea7f246f242b0010e3313499e314dc5bcfd42ba0600567d0`，binaries `d59d670dd05403e85b595506ff1c2ae90be7813342405ac55db5440b2f5bdcb3`，test.log `54f99ecab8307c682a1da680fda2044fa304671a095aae5603aa65b0177f0591`。

## 仍未关闭

Project C5／C6 canonical Link／资料消费者、真实 Workload 跨进程、正式兼容发布、联合空目标恢复、原连续 C8 与真人浏览器仍待验。Main Evidence 仍 Local，标准 Web OSS 尚未分离；整体维持 `insufficient-evidence`。
