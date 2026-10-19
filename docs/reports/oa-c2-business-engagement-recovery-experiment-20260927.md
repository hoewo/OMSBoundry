# OA C2 Business Engagement 联合恢复实验

日期：2026-09-27  
结论：**本地 D1 恢复子命题通过；OA 整体仍为 `insufficient-evidence`**

## 1. 要回答的问题

Business Engagement 虽然已经进入 D1 编排和备份脚本，但“脚本里有一行 `pg_dump`”不能证明它能随 Nginx、Gateway、IAM、Project、Evidence 一起恢复并继续工作。本实验验证：

> 已有权限、Project、Evidence 与 Business Engagement 事实 → 联合备份、销毁隔离栈全部数据卷并恢复 → 原 ID、revision、历史、引用和 IAM 签名材料保持，旧会话失效，恢复后的正式入口可以继续写入。

本实验不验证真实 Ubuntu、ACR pull-only、离机备份、公网 HTTPS、跨租户或浏览器 409／503 恢复。

## 2. 隔离与前置条件

- Compose project：`omsbizfoundry-c2`；
- 唯一宿主入口：`http://127.0.0.1:18101`；
- 选择集：IAM、Workbench、People & Groups、Project Workspace、Evidence Service、Business Engagement、Gateway、Nginx、PostgreSQL；
- Contract 与 Money 未启用，证明它们不是经营承接材料闭环的隐藏前提；
- 本机 Docker 地址池已满，使用独立覆盖文件固定 `10.250.9.0/24`～`10.250.12.0/24`；恢复只删除 `omsbizfoundry-c2` 的卷；
- 环境文件和覆盖文件均位于临时目录，报告不记录密码、Cookie、Token 或私钥。

为保证恢复使用与原栈相同的网络定义，D1 运维入口新增 `D1_COMPOSE_OVERRIDE_FILE`。它只接受存在的绝对路径，并按 `主 Compose → pull/HTTPS 覆盖 → 环境覆盖` 的顺序装配；未设置时行为不变。

## 3. 可复验步骤

以下命令中的环境文件必须由操作者自己准备，不能提交秘密：

```bash
export D1_ENV_FILE=/absolute/path/d1-c2.env
export D1_PROJECT_NAME=omsbizfoundry-c2
export D1_COMPOSE_OVERRIDE_FILE=/absolute/path/d1-c2-networks.yaml

deployments/d1/ops/d1 check
deployments/d1/ops/d1 backup

# 先核对返回的备份目录、checksums.sha256 和全部产物，再执行破坏性恢复。
D1_CONFIRM_EMPTY_RESTORE=yes \
  deployments/d1/ops/d1 restore 20260926T191755Z

D1_PUBLIC_URL=http://127.0.0.1:18101 \
  acceptance/d1-c2-business-engagement-authorization/verify-running
```

恢复前后另以只读 SQL 核对各应用 public 表计数，并逐条比较 Business Engagement 的 `id / project_id / status / revision / project_source_revision` 与 History `event_sequence / event_type`。IAM 签名密钥只比较 SHA-256，不输出内容。

## 4. 恢复前基线

| 数据所有者 | 关键事实 |
| --- | --- |
| IAM | 39 Subject、102 Capability Grant、333 Audit Entry、73 Suite Session、4 Workload Assertion Nonce |
| People & Groups | 20 Person、20 Person Revision、20 Audit Entry |
| Project Workspace | 18 Project、15 Assignment、56 History Event、51 Command、3 Package Conversion |
| Evidence Service | 6 File、6 File Version、12 Command、12 Audit Entry、6 Upload Session |
| Business Engagement | 6 Engagement、30 Command、30 History Event、6 Material |

6 条 Engagement 均为 `cancelled / revision 5 / project_source_revision 4`，每条都有从 `engagement_created` 到 `engagement_cancelled` 的 5 条顺序历史。IAM 签名密钥文件摘要为 `9938d7e6…9378`。

## 5. 备份与空目标恢复

UTC 备份 ID 为 `20260926T191755Z`。目录包含：

- `identity-access.dump`
- `signing-keys.json`
- `people-groups.dump`
- `project-workspace.dump`
- `evidence-service.zip`
- `business-engagement.dump`
- `checksums.sha256`

全部文件权限为 `0600`；逐项 `shasum -a 256 -c checksums.sha256` 均为 `OK`。Evidence 冷备份报告 `files=6 / versions=6 / objects=6`。

恢复命令实际停止并删除 C2 容器，删除其 PostgreSQL、IAM key、Evidence object 和两个工作负载私钥卷，重新创建固定的四张隔离网络与空卷，然后依次恢复 IAM dump／签名密钥、People、Project、Evidence 和 Business Engagement，重建工作负载密钥并启动所有启用服务。最终 `d1 check` 返回 IAM migration `000004`、People `000001`、Project `000002`、Evidence `000001` 和 Business Engagement `000001` current，全部服务健康。

## 6. 恢复后一致性与继续写入

恢复后除 Session 外，表计数与恢复前逐项一致；6 个 Engagement 的 ID、Project ID、状态、revision、来源 revision 和 30 条历史逐条一致；Evidence 仍为 6 File／6 Version，Project 仍为 18 Project／15 Assignment，IAM 仍为 102 Grant／333 Audit Entry，签名密钥 SHA-256 未变化。

Suite Session 从 73 变为 0 是明确的安全边界：备份故意排除 `suite_sessions`，恢复后必须重新登录，不能复活旧浏览器会话。

随后从 Nginx 唯一入口重新执行 C2 验收：

- 89 个 HTTP 断言；
- 0 个状态不匹配；
- 最大本地请求时间 0.333473 秒；
- Nginx → Gateway → IAM／Policy → Business Engagement、Project／File 精确工作负载凭证、撤权、状态最终授权和 IAM 审计全部通过。

该次验收在恢复库中新增一条完整业务记录，使 Business Engagement 从 `6 / 30 / 30 / 6` 增长到 `7 / 35 / 35 / 7`（Engagement／History／Command／Material），新记录为 `cancelled / revision 5`。因此恢复目标不只是可读归档，而是可继续执行业务命令的系统。

## 7. 架构判定

本实验支持以下子命题：

1. Business Engagement 保持自己的 PostgreSQL 所有权，同时能进入 D1 联合恢复；无需共享 Project、Evidence 或 IAM 表；
2. IAM 在恢复中是关键安全事实所有者：账号、Grant、审计和签名密钥保持，浏览器 Session 不保持；Gateway 和 Nginx 不拥有这些事实；
3. Project ID／revision 与 Evidence File ID／Version 作为稳定契约引用，在各自数据所有者独立恢复后仍能被 Business Engagement 解释；
4. Contract／Money 缺席时经营承接仍能恢复并继续写入，符合最小组合边界；
5. Compose 网络覆盖属于 Deployment Instance 差异，不进入业务 Base、Suite Manifest 或秘密管理。

它没有证明：离机副本可用、同一 ACR 摘要在真实 Ubuntu 恢复、公网 HTTPS、节点故障、高可用、跨租户、账号停用、409／503 浏览器交互恢复、下游 Contract／Money 建议。上述任一缺失都禁止把 OA 整体提升为 `supported`。

## 8. 下一缺口

下一轮只验证 Business Engagement 页面在业务冲突 `409` 和 Project／Evidence／IAM 暂不可用 `503` 时是否保留上下文、解释原因并可在依赖恢复后继续；不再重复把备份成功当作恢复证据。
