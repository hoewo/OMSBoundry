# OA C4 正式最小组合与恢复实验（2026-09-28）

## 判定

- 实验结论：`c4-a12-supported-local-d1`。
- C4-A12 已在隔离 D1 中关闭：Suite Manifest 只装入 People、Project、Evidence、Production，Workbench 只呈现已选业务应用，未选 Contract、Money、Party、Business Engagement、Billing 的页面和路由均缺席。
- `Nginx → Gateway → IAM／Policy → Production Operations` 正式入口主链通过；Production 使用独立 audience、显式 Capability namespace、精确 Project Scope 和目标应用最终授权。
- 联合备份与空目标恢复后，Production Work Item、revision、成果、历史、Project／Evidence 引用及 IAM 签名材料保持一致。
- 本轮不证明真实浏览器最终点击、完整 Production 撤权／账号停用矩阵、双 Tenant 或真实 Ubuntu，因此 C4 与 OA 完整体仍保持 `partially-supported`／`insufficient-evidence`。

## 问题 A → 可验证结果 B

**问题 A**：本地模板组合虽然能完成 consultation，但不能证明正式 Suite Manifest、Workbench、Gateway、IAM、可选装缺席和恢复边界真实成立。

**可验证结果 B**：从空的隔离 D1 启动最小依赖组合，经唯一 Nginx 入口完成真实登录、资源级签发、咨询／设计业务写入、依赖故障恢复、未选路由拒绝，再备份、销毁专属卷、恢复并比较 canonical 状态摘要与历史。

## 实际组合

启用：

- Identity & Access、Gateway、Nginx、Unified Workbench；
- People & Groups；
- Project Workspace；
- Evidence Service；
- Production Operations。

禁用：Contract Ledger、Money Flow Ledger、Party Directory、Business Engagement、Billing / Invoice。

Production Operations 沿用现有 `backend/services/design-operations` 的唯一可写数据库和兼容表，不创建第二生产事实源。D1 使用独立数据库账号、migration、工作负载私钥卷、镜像、备份选择项和 Suite Manifest 条目。

## 正式入口与权限证据

隔离入口为本地 Nginx 端口，所有业务请求均经 Gateway：

1. 首管登录后，Workbench 显示 Production，未选应用没有导航；对应未选路由返回 404。
2. Project 建立经营负责人和生产负责人 Assignment，完成立项后 revision 为 4。
3. Production IAM Policy 只给 active `production_owner` 签发 `aud=production-operations`、七项 `production-work-item.*` 动作和单 Project Scope；经营负责人签发被拒绝。
4. Production 页面与浏览器脚本从 `/production/` 正式路由加载；`design-v1` 与 `consultation-v1` 都从 canonical API 创建。
5. 生产负责人只能读取获授权 Project；相邻 Project 的 Work Item 返回 403。
6. Evidence 停机时交付物命令返回 503，Work Item 保持 revision 2 且零成果；恢复后以同一 Command ID 重试，只产生一个成果并到 revision 3。
7. IAM 停机时新建命令返回 503；恢复后相同命令只创建一个 Work Item。

验收结果：**49 个 HTTP 断言、0 个状态不匹配**。咨询 Work Item 完成于 revision 5，历史恰为 5 条；设计 Work Item 在 Evidence 故障恢复后为 revision 3、一个成果。

## 联合备份与空目标恢复

联合备份 `20260927T181130Z` 包含：

- `identity-access.dump` 与 IAM `signing-keys.json`；
- `people-groups.dump`；
- `project-workspace.dump`；
- `evidence-service.zip`；
- `production-operations.dump`；
- Suite selection 与逐文件 SHA-256 清单。

恢复前保存咨询 Work Item 的 `{work_item, stages, deliverables}` canonical 摘要：

```text
592c166db157ee72840f198b3496cab5a8171d86daca08df4259b66645db4e7b
```

恢复命令销毁的只有隔离 Compose 项目专属容器和卷。恢复后重新登录、重新签发 Production Credential，并验证：

- consultation Work Item 仍为 completed／revision 5／一个成果；
- design Work Item 仍为 revision 3／一个成果；
- IAM 故障恢复创建的相邻 Project Work Item 仍为 revision 1；
- consultation 历史仍为 5 条；
- canonical 摘要与恢复前完全相同。

第一次恢复演练发现 Docker Desktop 在容器已删除后仍可能短暂保留 Gateway 的 edge-network endpoint，导致 `compose down` 返回失败。恢复脚本现进行有界重试；若仍失败，只在确认该 Compose 项目已无残留容器时复用旧网络，绝不在存在容器时继续，也不执行全局 network prune。修复后同一备份完成空目标恢复并通过状态复验。

## 契约修正

Production 的 audience 是 `production-operations`，动作命名空间是 `production-work-item.*`。两者不应被强制设为同一个字符串前缀。因此：

- Capability Manifest 与 Suite Manifest 新增可选 `capability_namespace`；缺省仍回退 application ID，保持旧应用兼容；
- Gateway、Workbench 和 IAM Capability Catalog 使用该字段校验入口动作与能力目录；
- IAM Grant／Policy／Workload 继续要求动作真实存在于目标 Capability Catalog，不再用 audience 字符串前缀冒充能力所有权；
- 数据库 migration 只移除错误的 audience-prefix check，保留 action 语法与目录存在性校验。

这项修正允许“稳定应用 audience”和“面向资源的能力命名空间”解耦，不允许跨应用动作、未知动作或客户端自报能力。

## 可复核入口

```bash
acceptance/d1-c4-production-authorization/run-isolated run

D1_ENV_FILE="$PWD/acceptance/d1-c4-production-authorization/d1.env.example" \
D1_PROJECT_NAME=omsd1c4 \
D1_COMPOSE_OVERRIDE_FILE="$PWD/acceptance/d1-c4-production-authorization/compose.override.yaml" \
  deployments/d1/ops/d1 backup

D1_CONFIRM_EMPTY_RESTORE=yes \
D1_ENV_FILE="$PWD/acceptance/d1-c4-production-authorization/d1.env.example" \
D1_PROJECT_NAME=omsd1c4 \
D1_COMPOSE_OVERRIDE_FILE="$PWD/acceptance/d1-c4-production-authorization/compose.override.yaml" \
  deployments/d1/ops/d1 restore <BACKUP_ID>

acceptance/d1-c4-production-authorization/run-isolated verify-restored
```

静态与回归门禁同时通过：Production、IAM、Gateway、Workbench、access-contract Go tests；D1 image mode、ingress mode、release bundle、upgrade selection、publish／promote self-tests；通用 `acceptance/d1-delivery/verify-running` 也在该最小组合通过。

## 剩余反证目标

- 通过真实浏览器完成 Workbench → Production → consultation 操作与撤权后的交互恢复；
- 补齐 Production 专项 Grant 撤销、Assignment 结束、账号停用及凭证残余窗口矩阵；
- 在真实 Ubuntu／公司或个人试验环境复演同一 pull-only 发布物与恢复；
- 用双 Tenant 证明 Workbench、Policy、Project、Evidence 和 Production 全链均无交叉可见；
- 在引入 `survey-v1`／`mapping-v1` 前先取得结构化事实证据，继续允许证据否定单一 Production 内核。
