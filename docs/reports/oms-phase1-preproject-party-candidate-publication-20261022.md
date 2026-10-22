# OMS 一期建项前合作主体候选发布报告

日期：2026-10-22  
事项：BF-177  
源码基线：`main@6903934`

## 结论

“新增施工方成功但页面刷新后不可见”不是单纯的前端刷新遗漏。原流程只在 Party Directory 创建了中性主体，而施工方页面只认已经形成的项目关系；这与“先维护施工方档案、后建立项目”的产品流程冲突。

本次由 Project Workspace 正式拥有建项前候选语义，新增追加式 `project-party-candidate` 事实与版本化端口。Workbench 从甲方／施工方管理新建主体时，依次执行 Party 创建和候选登记两条独立幂等命令，登记成功后再关闭弹窗、刷新并回到当前列表。中途失败可以安全续办，不重复创建 Party，也不把页面状态当成业务事实。

该候选只表示某稳定 Party 可进入后续项目的甲方／施工方选择器，不是 Party 的永久分类，也不代表项目关系已经发生。Party Directory、Project Workspace 及具体项目快照的数据所有权保持不变。

## 发布制品与回滚点

- Project Workspace：`v0.1.0-test.oms-v1.28@sha256:c4d12d24cf9793feb84b83967143e86901799e6d6398157f9d4a01ae8b5bb5ee`
- Unified Workbench：`v0.1.0-test.oms-v1.28@sha256:bee4929c1c39b1f0c8fc7b7915e02ca992e9e63850909ec9055f176295d232b9`
- D1 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.28.https.env`
- 人工升级前完整备份：`/var/backups/omsbizfoundry/20261022T121219Z`
- D1 升级自动备份：`/var/backups/omsbizfoundry/20261022T121425Z`
- 公网入口：`https://oatest.antuinfo.cn/oms-v1/`

## 验证

本地实际通过：

- Project Workspace `go test ./...`、`go vet ./...` 与 integration-tag 编译；
- PostgreSQL 18 上 migration 和候选 repository 定向集成测试；
- Workbench `go test ./...`、`go vet ./...` 与主脚本语法检查；
- 主体角色投影回归 3/3；
- 前端 TypeScript check、206/206 测试与两个 Vite bundle build；
- 公网验收单测 2/2、Python 语法检查、架构静态审计与 `git diff --check`。

公网发布后实际通过：

- D1 preflight、升级与全栈 `check`；
- Project Workspace migration `000014_project_party_candidates.sql` 已应用；
- 17/17 个长运行容器 healthy；
- 正式 IAM 登录、Gateway 授权与 Project／Party／Billing／Money／Contract Owner 读取；
- 新的候选 GET／POST API 可用，重复命令保持幂等；
- 此前失败流程留下的中性 Party“施工方1”通过正式 Project 候选命令补登记，没有直接写数据库或复制主体；
- 最终复读为 Project 1、Party 5、Billing 0、Money 0、候选 1；
- 目录投影为甲方 `[甲方1, 甲方2, 甲方3]`、施工方 `[施工方1]`。

## 边界

本次没有新增 Party 全局角色字段、共享表、跨应用数据库写入或新的 IAM Grant。候选登记不会自动建立项目关系、合同、应收、应付或资金事实。现有项目关系仍是关联项目数和项目内业务角色的唯一来源。
