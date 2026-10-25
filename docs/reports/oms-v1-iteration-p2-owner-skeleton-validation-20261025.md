# OMS1 迭代 P2 两个 Owner 骨架验证报告

日期：2026-10-25
分支：`codex/company-internal-management`
状态：本地通过；未部署 D1 或公网

## 1. 目标与边界

问题 A → 可验证结果 B：公司内部管理此前只有 Workbench 浏览器候选，财务申请和工资事实没有服务端 Owner → 建立 `Internal Finance Requests` 与 `Payroll` 两个可独立迁移、运行、授权、读写、备份和恢复的最小 Owner，为后续 P3～P7 纵向业务切片提供稳定边界。

本阶段只实现两个草稿聚合：

- `Internal Finance Requests` 拥有报销／付款申请草稿的 ID、Tenant、申请账号、类型、标题、精确金额、状态和 revision；
- `Payroll` 拥有租户内月度工资批次草稿的 ID、期间、名称、状态、创建账号和 revision；
- 两者各自拥有 PostgreSQL 数据库、migration、命令幂等表、不可变 revision、审计事件和当前投影；
- Workbench、Gateway、IAM、People、Evidence 与 Money 不获得这两类业务事实的所有权。

P2 不实现成员薪酬档案、工资计算／发放、财务明细／附件／审批／付款确认、Money 建议、开放 API 或页面切换。这些仍分别属于 P3～P7。

## 2. 实现结果

新增两个独立 Go module：

- `backend/services/internal-finance-requests`
- `backend/services/payroll`

每个服务包含：

- `cmd/server` 与 `cmd/ops`；
- PostgreSQL 18 migration、schema 门禁和健康检查；
- IAM 签名凭证与显式本地模板身份提供者；
- 版本化 HTTP API、Gateway 去前缀兼容路由和精确引用读取；
- Tenant、Capability、资源范围、本人／租户读取、`expected_revision` 与命令幂等校验；
- 不可变 revision／command／audit 触发器和“当前投影必须等于最新 revision”延迟约束；
- Dockerfile、独立 Compose、环境样例、备份及空目标恢复入口。

正式装配路由为：

- `/internal-finance/v1/` → `internal-finance-requests:8092`
- `/payroll/v1/` → `payroll:8093`

D1 已增加两个可选 profile、独立数据库和口令、Suite Manifest 应用项、镜像覆盖、migration、健康检查、备份／恢复选择及发布包镜像清单。Capability Manifest 随发布包携带。服务默认不启用；本阶段没有执行 D1 升级或公网发布。

## 3. 已执行验证

以下验证均以本次代码实际运行：

- 两个 module 的 Go 单元测试、HTTP 授权测试和 `go vet`；
- IAM 模式使用 Ed25519 签名 JWT／JWK 验证 issuer、audience、Tenant、Capability 和资源范围；
- PostgreSQL 18.6 集成测试分别覆盖创建、同命令同内容重放、同命令异内容冲突、乐观 revision 冲突、跨 Tenant `404`、当前投影直接篡改拒绝及历史 revision 更新／删除拒绝；
- `acceptance/p2-owner-skeleton/run-isolated` 构建两个独立镜像并启动两个服务，在一次性 PostgreSQL 中完成写入、重放、冲突、revision 更新、分别备份、恢复到空数据库以及恢复后继续写入；
- D1 Compose 配置、Suite Manifest 路由与启用开关校验；
- `publish-images self-test`、`promote-images self-test`、`test-image-mode`、`test-ingress-mode`、`test-upgrade-selection`、`test-release-bundle`；
- 静态 import 审计确认两个新服务没有导入其他业务服务的内部模型或数据库代码；
- 有效 API Key／密码模式扫描无命中。

隔离验收最终输出：

```text
P2 owner skeleton acceptance passed: two independent images + write/replay/conflict + revision + backup/empty restore + continuation write
```

## 4. 结论与后续

P2 的最小 Owner 骨架本地通过：两个服务已经具备独立事实边界、可信身份入口、租户隔离、可追溯修改、幂等与恢复基础。该结论不表示财务申请或工资业务已经可供用户使用，也不表示 D1／公网已发布。

下一阶段进入 P3：由 People & Groups 维护成员／任职事实，由 Payroll 通过 Person ID 挂载独立的薪酬档案、工资账户、社保和公积金方案；两类 Owner 继续分别授权和提交，不形成共享表或跨库事务。
