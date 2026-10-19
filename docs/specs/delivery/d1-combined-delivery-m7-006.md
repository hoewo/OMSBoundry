# M7-006：D1 组合交付验收

日期：2026-09-21
状态：已验证

> 历史范围说明：本文件冻结 M7-006 当时的 IAM＋Workbench＋Contract＋Money 验收。M8 已把 People & Groups 纳入当前 D1；M10-003 又把 Evidence、第五套独立数据库、可选本地数据面和 `evidence-service.zip` 纳入当前 D1；2026-09-26 Project Workspace 再加入第六套应用数据库、Resource Scope、镜像和联合恢复边界，均不改写本文件的历史结论。新增证据见 [M8 规格](../people-groups/m8-minimal-vertical-slice.md)、[M10-003 Evidence D1 装配](../evidence/m10-003-d1-integration.md)、[OA 第一阶段权限实验](../../reports/oa-stage-1-authorization-experiment-20260926.md)与 [D1 当前运行手册](../../operations/d1-compact.md)。

## 1. 最小根因

> IAM、入口和两个业务应用分别可运行，但还不能作为一套产品重复交付 → 用同一份 D1 编排完成空环境安装、单次登录双应用、可逆开关、隔离故障、升级前备份和整机恢复。

本项只收口当前已经实现的 IAM、Unified Workbench、Contract Ledger 与 Money Flow Ledger，不把尚未实现的 People & Groups、Project Workspace 或独立 Evidence Service 伪装进交付范围。

## 2. 交付结构

`deployments/d1/compose.yaml` 固定以下运行边界：

- 只有 Nginx 发布宿主机端口；Gateway、Workbench、IAM、业务应用和 PostgreSQL 只加入内部 Docker 网络；
- 一个 PostgreSQL 18 实例初始化 `identity_access`、`contract_ledger`、`money_flow` 三个数据库及三个独立登录账号；
- 每个应用仍使用自己的 migration、数据库凭证、备份格式和恢复校验，不增加跨库查询或共享表；
- IAM 私钥、导出的公钥集合、Contract Evidence 和 PostgreSQL 数据使用不同持久卷；
- `ENABLE_CONTRACT`、`ENABLE_MONEY` 生成同一份 Suite Manifest，并共同决定 Gateway 路由、Workbench 菜单和目标应用进程；关闭应用不删除数据库或持久卷；
- IAM、Gateway 与两个业务应用使用 M7 已验证的 Ed25519 单 Audience 能力凭证；Gateway 不拥有账号或授权事实，目标应用继续作最终授权；
- D1 不要求 Redis、消息队列、服务注册中心、Kubernetes、SLB 或第二台服务器。

生产边缘必须使用 HTTPS、真实域名和 `COOKIE_SECURE=true`。仓库默认配置只把 HTTP 绑定到 `127.0.0.1`，用于本机验收，不能作为公网生产配置。

## 3. 操作契约

统一入口为 `deployments/d1/ops/d1`：

| 命令 | 结果 |
| --- | --- |
| `up`／`apply` | 校验环境、渲染 Suite Manifest、构建镜像、执行 migration、启动启用服务并等待健康 |
| `status` | 显示当前部署状态 |
| `check` | 分别执行 IAM、Contract、Money 的 current-schema 门禁 |
| `backup` | 在线生成 IAM dump＋签名私钥、Contract 数据库＋Evidence ZIP、Money ZIP 和整组 SHA-256 清单 |
| `upgrade` | 先生成联合备份，再重新构建、执行幂等 migration、重建服务并复核 schema |
| `restore ID` | 在显式确认后删除目标 Compose 项目的卷，从校验通过的备份恢复到全新空目标并启动复核 |
| `down` | 停止组合但保留数据卷 |

首位管理员初次安装后的业务能力授予通过 IAM API 执行，入口是 `deployments/d1/ops/grant-bootstrap-access`；它不直写 IAM 数据库，也不会把密码输出到日志。

## 4. 联合备份边界

一次 D1 备份 ID 对应同一个受限目录，至少包括：

- `identity-access.dump`：IAM PostgreSQL custom dump，排除可失效的 `suite_sessions` 数据；
- `signing-keys.json`：恢复既有凭证验证链所需的 IAM 私钥材料，权限固定为 0600；
- `contract-ledger.zip`：Contract 已有的 PostgreSQL＋available Evidence 联合备份；
- `money-flow-ledger.zip`：Money Flow 已有的 PostgreSQL 全量备份；
- `checksums.sha256`：上述所有交付物的外层 SHA-256。

该目录同时包含密码摘要、业务事实和签名私钥，是敏感运维资产，必须复制到 ECS 之外的受控存储并加密、限制访问。联合备份没有跨三个数据库的全局事务，因此不声称全局同一时刻快照；每个应用只保证自己的已验证一致性边界。跨应用连接器尚未存在，也没有需要伪造的跨库原子提交。

恢复只接受全新的 Compose 数据卷。IAM Session 不恢复，旧 Cookie 必须失效；账号、授权、签名链和两个业务应用事实恢复后可以重新登录继续使用。

## 5. 验收证据

2026-09-21 在独立 Compose 项目 `omsd1m7006`、宿主机回环端口 `18100` 上使用实际镜像和 PostgreSQL 18.6 完成：

1. 从空卷构建五个 Go 镜像，自动创建三个数据库／账号并顺序完成 IAM `000001`、Contract `000017`、Money `000016` migration；全部健康；
2. Compose 解析结果只有 Nginx 发布宿主端口；Nginx → Gateway → IAM／Workbench／两应用实际可达；
3. 首位管理员通过一次 IAM 登录分别取得 Contract 与 Money 单 Audience 能力凭证，两应用页面均返回 200，并各完成一次 Party 写入，均返回 303；
4. 停止 Contract 时 Contract 路由返回 503，Money 仍返回 200；恢复 Contract 后继续可用；
5. 将 `ENABLE_MONEY=false` 后，Money 容器停止、路由返回 404、Workbench 菜单消失，Contract 保持 200；重新启用后 Money 返回 200，原数据卷未删除；
6. 联合备份成功生成四项敏感资产和外层摘要；随后删除该验收项目的全部数据卷，从备份恢复三个空数据库、IAM 私钥和 Contract Evidence；
7. 恢复前 Cookie 对 IAM Session 返回 401；原管理员重新登录成功，两个应用原有 Party 均可读取，Contract 恢复后继续写入返回 303；
8. `upgrade` 先自动生成新备份，再重建镜像、重放幂等 migration、复核三个 schema；恢复后新增事实仍存在；
9. `acceptance/d1-delivery/verify-running` 再次自动验证仅 Nginx 暴露、一次登录、两个启用应用读写和单侧故障隔离。

验收使用回环 HTTP 以避免伪造生产证书；HTTPS 生产终止、ECS 安全组、离机备份落点和资源规格仍属于部署现场责任。D1 是单点最低档，不是高可用。

## 6. 不负责范围

- 不证明 D2、D3 或 D4 已部署；它们仍只有已接受的放置约束；
- 不证明 M7-006 当时尚未包含的 People、Project 或独立 Evidence Service；其当前完成事实分别由后续 M8、M10-003 与 OA 第一阶段权限实验证明；
- 不把当前 Contract 内嵌 Evidence 误称为目标独立文件服务；
- 不提供全局跨应用事务或跨应用数据库恢复点；
- 不替代真实 ECS 的域名、证书、系统防火墙、磁盘容量、监控告警、RPO／RTO 和异地备份演练。

操作者步骤见 [D1 Compact 运行手册](../../operations/d1-compact.md)，持续验证入口见 [验证入口](../../testing.md)。
