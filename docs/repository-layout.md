# OMSBoundry 仓库结构

## 目的

这份仓库把可交付的 OMS 前端和既有小系统放在同一个可追溯基线中，同时保持前后端分离、应用数据所有权和独立部署能力。目录变化不代表把多个小系统合成一个后端，也不改变公网装配链 `Nginx → Gateway → IAM → Workbench／Owner`。

## 顶层目录

| 目录 | 职责 | 不负责 |
| --- | --- | --- |
| `frontend/oms-workbench` | OMS 一期界面、旧 OA 工作台、静态资源和页面装配服务 | 不拥有项目、合同、票据、资金、主体或文件事实 |
| `backend/platform/gateway` | 统一路由、限流、可信身份上下文和入口换证 | 不签发业务身份，不替 Owner 最终授权 |
| `backend/platform/identity-access` | 租户、账号、登录、会话、Grant 和 Capability Credential | 不拥有人员名册和业务事实 |
| `backend/services/*` | 各自独立的业务事实、数据库、migration、HTTP API 和运维命令 | 不直接读写其他服务数据库 |
| `contracts` | 版本化能力、访问、Evidence、建议和只读投影契约 | 不保存运行时业务数据 |
| `packages` | 已有多个稳定使用点的共享机制包 | 不承载客户专用流程或通用业务平台 |
| `deployments` | D1、演示、本地 OA、云部署和发布包 | 不拥有业务事实 |
| `acceptance` | 跨应用、权限、恢复和发布包验收 | 不作为生产运行时 |
| `docs`、`arckit` | 决策、规格、边界、报告和设计材料 | 不替代可执行验证 |

## 后端服务

`backend/services/` 当前包含：

- `party-directory`、`project-workspace`、`contract-ledger`、`billing-invoice`、`money-flow-ledger`、`evidence-service`
- `people-groups`、`business-engagement`、`design-operations`、`approval-audit-results`
- `cost-outsourcing`、`bonus-distribution`

OMS 一期当前主要组合前六个业务事实 Owner；其余服务保留在同一代码基线中，但不会仅为了“看起来完整”被强制接入 OMS 一期。

## 本地验证

```bash
go work sync
make fmt-check
make test
make vet
acceptance/architecture-boundary-static-audit/self-test
acceptance/architecture-boundary-static-audit/verify

npm --prefix frontend/oms-workbench/frontend ci --ignore-scripts --no-audit --no-fund
npm --prefix frontend/oms-workbench/frontend run check
npm --prefix frontend/oms-workbench/frontend test
npm --prefix frontend/oms-workbench/frontend run build
node --test frontend/oms-workbench/internal/server/*.test.mjs
```

发布和容器验收继续使用 `deployments/d1/`、`deployments/local-oa/` 与根 `Makefile` 中的目标。环境变量、运行数据库、备份、缓存、`node_modules` 和构建产物不进入 Git。

## 修改约束

1. 页面只通过版本化 API 和 Gateway／IAM 契约访问 Owner。
2. 新业务事实进入明确的 `backend/services/<owner>`，不能写入 Workbench、Gateway 或共享表。
3. 跨服务只使用版本化 API、快照或事件，并保留幂等、失败恢复和验收。
4. 金额继续使用整数最小货币单位或精确十进制表示，不使用二进制浮点。
5. 客户差异优先放入 Suite Manifest、配置、策略、类型化扩展或 Customer Overlay，不长期 fork 公共内核。
