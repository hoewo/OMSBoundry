# OMS1 迭代 P0 可追溯基线与 P1 开工记录

日期：2026-10-25
分支：`codex/company-internal-management`
需求基线提交：`abe170fc38b1c5aca67d5f0389da9e8ed406ff9a`
结论：P0 通过；P1 后续已完成本地验收，见 [`oms-v1-iteration-p1-multitenant-validation-20261025.md`](./oms-v1-iteration-p1-multitenant-validation-20261025.md)。本记录本身不是 D1 发布或公网验收结论。

问题 A → 可验证结果 B：BF-184～BF-237 同时改变身份、数据 Owner、开放 API 和外部反馈装配，若没有冻结命名、依赖和证据位置，后续提交会失去可审计边界 → 以不可变需求提交、追踪矩阵、保留命名和分层验证入口形成可重复的 P0 起点。

## 1. 基线边界

- BF-184～BF-237 的产品行为以 [`oms-v1-iteration-plan-20261025.md`](../specs/assembly/oms-v1-iteration-plan-20261025.md) 和 [`decisions.md`](../decisions.md) 为准；实现顺序以 [`oms-v1-iteration-implementation-plan-20261025.md`](../specs/assembly/oms-v1-iteration-implementation-plan-20261025.md) 为准。
- `abe170f` 是进入实现前的干净文档基线。P1 的 IAM、People、Capability Manifest 和 D1 候选变更位于其后的工作区，只有完成 P1 门禁并提交后才成为新的可运行基线。
- 当前公网 `oatest.antuinfo.cn` 没有因本地 P0／P1 工作发生发布；既有公网版本和本地候选必须分开描述。
- 未把 Feedback 项目 Key、密码、Cookie、会话、数据库备份或业务数据加入 Git。针对 `ak_` 加 64 位十六进制模式的工作区扫描结果为 0 个文件命中。

## 2. 保留命名

| 项目 | 保留值 | 说明 |
| --- | --- | --- |
| Suite application ID | `internal-finance-requests` | 报销／付款申请事实 Owner |
| Suite application ID | `payroll` | 薪酬档案、工资批次和发放投影事实 Owner |
| Suite application ID | `open-api` | Gateway 开放 API audience；不拥有报销事实 |
| Suite application ID | `feedback` | Feedback V2 入口 audience；反馈事实由外部 Feedback 平台拥有 |
| D1 内部端口 | `8092` | 预留给 Internal Finance Requests |
| D1 内部端口 | `8093` | 预留给 Payroll |
| PostgreSQL database / role | `internal_finance_requests` | 独立数据库和角色 |
| PostgreSQL database / role | `payroll` | 独立数据库和角色 |
| API 前缀 | `/internal-finance/v1/` | Workbench／Gateway 内部财务申请路由 |
| API 前缀 | `/payroll/v1/` | Workbench／Gateway 工资路由 |
| API 前缀 | `/open-api/v1/` | 员工 Key 调用的稳定公开契约 |
| Capability namespace | `internal-finance-requests.*` | 申请、敏感读取／导出、付款确认等独立能力 |
| Capability namespace | `payroll.*` | 档案、工资读取／编辑／发放／导出等独立能力 |
| Capability namespace | `open-api.*` | Key 管理、本人报销调用和脱敏审计 |
| Capability | `feedback.use` | 主 OMS1 产品反馈入口 |

端口只用于同一 Deployment Profile 内的服务发现，不允许进入领域事实或浏览器硬编码。应用端点继续由配置注入。

## 3. 决策追踪矩阵

| BF | 阶段 | Owner／组件 | migration／API | 页面 | 必需验收证据 | 当前状态 |
| --- | --- | --- | --- | --- | --- | --- |
| 184 | P1 | IAM、People & Groups、全部受保护 Owner | IAM Tenant／Subject revision；People Subject snapshot；可信 `tid` | 登录、账号与成员 | 同进程 A／B 登录、搜索／分页／导出／附件、稳定 ID 猜测、撤权／停用、空目标恢复 | 实施中；IAM＋People 主路径已有 PG18 证据，完整 Owner 矩阵未关闭 |
| 185 | P2/P5 | Internal Finance Requests、Evidence、Money | 独立 schema、`/internal-finance/v1/`、建议／实际付款引用 | 财务管理 | Owner 独立启动、幂等、状态机、Money 故障恢复 | 未开始 |
| 186 | P2/P4 | Payroll、Money | 独立 schema、工资批次与 Payment Delivery | 工资管理 | 部分成功、未知结果、实际发放确认 | 未开始 |
| 187～190 | P3/P4 | People、Payroll | Person 引用、Compensation Profile、方案／个税／任职快照 | 成员、薪酬档案、工资行 | 生效月、离职／重入、比例 golden test、历史不改写 | 未开始；P1 先建立可信 Subject↔Person 基础 |
| 191～203 | P5 | Internal Finance Requests、Money、Evidence、People／Party／Project／Contract 引用 | 多次付款、申请 revision、类别、明细、收款快照、费用归属 | 报销／付款申请、财务确认 | 权限拆分、附件、敏感读取、撤回、分次／超额、批量部分成功 | 未开始 |
| 204～215 | P3/P4 | Payroll、People、Money | 本人可见性、工资计算／同步、账户 revision、补发／扣回 | 工资详情、批量操作 | 精确金额、零／负／超额、缺账户、同步边界、本人隔离 | 未开始 |
| 216～220 | P6 | IAM／Gateway、Open API | Key 生命周期、当前 Grant 复验、限流 | API Key 管理 | secret 只显示一次、最多 5 个、30／90／180／365 天、撤权即时生效 | 未开始；Capability Manifest 已先登记 |
| 221～233 | P6/P7 | Gateway、Open API、Internal Finance、Evidence | 幂等、重复核对、上传会话、本人查询、错误模型、版本兼容 | Key 使用记录、Webhook 占位 | HTTPS、分页／ETag、故障恢复、文件安全、秘密扫描 | 未开始 |
| 234 | P7 | Internal Finance、Evidence、Money、IAM／Gateway | 一张真实发票端到端 | 员工提交＋财务确认＋本人查询 | 真实 HTTPS 和三个 Owner，故障注入后无重复资金 | 未开始 |
| 235～237 | P8 | Workbench、Feedback SDK V2、IAM | 部署注入、稳定脱敏用户 ID、严格诊断 allowlist | 产品反馈、我的反馈、未读角标 | 双账号／双租户、origin／Key／Project 负例、降级和轮换 | 未开始；`feedback.use` Manifest 已先登记，秘密未入库 |

## 4. P0 门禁和 P1 已取得证据

本次实际执行并通过：

- `git diff --check`；
- IAM 与 People 普通 `go test ./...`；
- 受 P1 租户透传影响的 Go modules 单元测试和 `go vet`；
- Capability Manifest 目录加载测试，含 Identity、Internal Finance、Payroll、Open API 和 Feedback；
- D1 Compose 配置解析；
- `Dockerfile.identity` 构建，镜像中 `/app/server`、`/app/ops`、`/app/keyctl`、`/app/tenantctl` 均可执行；
- `Dockerfile.people` 构建；
- PostgreSQL 18.6 全新数据库上的 IAM 双租户生命周期集成测试；
- PostgreSQL 18.6 上 People `000001 → 000002` 旧 Subject 关联迁移，确认标记为 `legacy_unverified` 且历史不可变触发器恢复。

以下是 P0 结束时登记的 P1 待办；其后已由 P1 报告关闭本地门禁，保留此处作为时序记录：

- 全部受保护 Owner 的同进程双租户正反例清单，尤其搜索、分页、导出、附件和精确稳定 ID；
- IAM＋People 双租户空目标备份恢复及恢复后继续写入；
- D1 候选启动后的真实 Gateway／IAM／People 纵向请求；
- 现有单租户事实的 Tenant 绑定审计报告和异常数据门禁；
- P1 提交与对应镜像 digest。

## 5. 回滚与秘密边界

- P1 migration 均为前向 schema 变更；应用 migration 后不得用 P0 二进制覆盖运行库。回滚使用 P0 备份恢复到空数据库，而不是反向改表。
- `tenantctl` 只接受密码文件路径；生产操作不得把首个管理员密码放入命令行参数、日志或 JSON 回执。
- Feedback Key 只允许部署环境注入。P0／P1 文档、示例、测试和镜像层不包含真实值。
