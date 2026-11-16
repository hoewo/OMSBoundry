# OMS 多发票报销 v1.104 发布记录

日期：2026-11-16（GMT+8）

事项：BF-338

状态：已发布并完成无业务写入验收

## 发布结果

- 公网入口：`https://oatest.antuinfo.cn/`
- Suite：`v0.1.0-test.oms-v1.104`
- 源码提交：`733634e2e5f0812dcde347e8068aa141e70dc5f6`
- Internal Finance Requests 镜像摘要：`sha256:7cc0674269b39fba516c7657fe4dd2364fe97c83b7ebcb6dec570c8135bc6782`
- Unified Workbench 镜像摘要：`sha256:3ac85b851497af3ce1210c5df34a7d78125b3af488a312679c4b532db89052fd`
- 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.104_linux_amd64.tar.gz`
- 发布包 SHA-256：`d70ef1f0251ce63ec33180ff76c6e66592efed4d7b3d548c31fc17f89258f6ee`
- 升级前备份：`/var/backups/omsbizfoundry/20261116T133714Z`
- ECS 验收记录：`/opt/omsbizfoundry/acceptance/oms-reimbursement-multi-invoice-v104-20261116`

## 验证

- Finance Go test／vet、Workbench Go test／vet、Workbench Node 309/309 和 JavaScript 语法检查通过。
- 临时 PostgreSQL 18.6 实例实际运行 Finance／HTTP integration 并通过；实例随后停止并自动移除。
- P9 preflight、D1 composition、发布包外层校验和包内 93 项校验、候选 IAM 契约预检通过。
- 升级前联合备份包含 21 项 checksum 且复验通过；正式 D1 `check` 通过，21/21 长运行容器 running，0 unhealthy。
- 运行中的 Internal Finance Requests 与 Workbench 镜像摘要分别与上述候选完全一致。
- 公网 `oms-phase1-finance-owner.js` SHA-256 为 `7b48f89263f0176ba2a97f8121b8c22141236637535296c890fe178557b96fdc`，`oms-phase1.css` 为 `aac17b0874eb6df252e870874b8fe928c477ab5c44e60cee317f196ce3a2c139`，均与本地逐字节一致。
- 公网根入口返回 303 到 `/workbench/?view=oms-v1`，Workbench 返回 200。

## 升级说明

- 标准 `upgrade` 在完成备份后遇到既有的一次性 Cost Outsourcing workload-key 端点占用旧默认网络；没有删除数据卷。
- 随后使用同一 v1.104 候选执行 D1 `up` 完成收敛，并独立执行 D1 `check`；最终运行镜像、数据库 schema 和健康状态均通过。

## 未外推

- 发布验收没有创建、修改、提交或删除报销申请，也没有上传发票；用户可使用自己的测试数据验证单项／多项交互。
- 本次没有新增 schema migration，没有改变 Invoice Recognition、Evidence、Billing 或 Money 的事实所有权。
- 静态资源与运行健康通过不等于真实 Chrome 的单项、多项、强重复或旧草稿写旅程已经执行。
