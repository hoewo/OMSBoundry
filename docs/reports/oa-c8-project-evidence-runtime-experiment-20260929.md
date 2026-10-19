# OA C8 Project → Evidence 最小运行组合实验

日期：2026-09-29  
结论：`c8-direct-material-runtime-supported / human-page-and-d1-pending`

## 问题 A → 可验证结果 B

**问题 A**：C8 的 Evidence 分权和 Project 资料组合此前只有单元证据，无法证明真实 IAM 工作负载换票、PostgreSQL 数据、独立进程故障和恢复能够在不共享表、不扩大下载权的条件下工作。

**可验证结果 B**：在最多一个临时 PostgreSQL 容器内，为 IAM、Project、Evidence 建立三个独立数据库，并把三个服务作为本地独立进程运行；Project 使用自己的 Ed25519 工作负载身份向真实 IAM 换取 `evidence-service.metadata.read + file:{id}` 短期凭证，组合 direct Evidence Link，拒绝相邻 Project Scope，在 Evidence 停机时只降级资料分段，恢复后重新可用，最终清理回 0 个容器。

## 运行边界

- 验收入口：`acceptance/c8-material-composition/run-isolated`；
- 只启动一个 `postgres:18.6` 容器；IAM、Evidence、Project 使用本地二进制顺序启动；
- 没有启动 Nginx、Gateway、Workbench、完整 D1 或其他业务应用；
- 一个容器内的 `identity`、`project`、`evidence` 是三个独立数据库，不存在跨库查询；
- 夹具只向各事实所有者自己的数据库写入前置事实，运行期组合只走 IAM 和版本化 HTTP；
- 脚本拒绝在已有运行容器时启动，并在退出时要求容器数回到 0。

## 直接证据

1. Project workload policy 只有 `evidence-service.metadata.read`，资源类型只有 `file`；
2. IAM 审计记录 1 次 `project-workspace → evidence-service.metadata.read → file:c800...0010` 精确换票，download 换票为 0；
3. Project 资料响应包含两个 direct Link 分段：当前 revision 为 `available`，冻结 revision 2 对当前 Evidence version 1 为 `revision_changed`；
4. 用只授权相邻 Project 的人类凭证访问目标 Project 返回 `403 scope_denied`；
5. 返回包含文件名、版本、SHA-256、媒体类型、来源状态与关系键，不包含 `bucket` 或 `object_key`；
6. Evidence 进程停止后，两个分段都返回 `unavailable/source_unavailable`，Project API 本身保持 200 并保留逐段故障；Evidence 以同库重启后当前资料恢复为 `available`；
7. HTTP 断言依次为 `200、403、200、200`，容器计数为 `0 → 1 → 0`。

原始运行证据位于 `deployments/d1/.runtime/c8-material-composition-evidence/`：

- `http-evidence.tsv`；
- `workload-audit.tsv`；
- `materials.json`、`materials-outage.json`、`materials-recovered.json`；
- `resource-usage.tsv`、`processes.tsv` 和三个服务日志。

## 结论边界

本轮支持的是 C8 的第一条 direct Project → Evidence 资料运行链，以及其真实 IAM、Scope、局部失败和恢复语义。它没有证明：

- 人类建立／浏览 Link 和独立下载按钮；
- 业务应用资源间接引用的 Evidence 集合；
- `MetricProjection v1`、`WorkItemProjection v1` 或 Workbench 聚合；
- Nginx → Gateway → IAM 正式入口、Suite Manifest 未选缺席、D1 备份恢复、双 Tenant 或真实 Ubuntu。

因此 C8 不能判定整体完成；下一条业务增量应在“资料人类页面、一个来源的间接 Evidence 引用、一个工作项来源”之间重新按用户价值和不确定性选择，仍按低资源小批次推进。
