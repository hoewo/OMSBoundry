# OA C5 D1 正式装配与恢复实验

日期：2026-09-28  
结论：`formal-d1-assembly-and-recovery-supported / business-cross-process-journey-pending`

## 问题与结果

**Approval & Audit Results 已有独立核心、IAM 和页面，但尚未证明能作为可选应用进入正式 D1 入口并随整套组合恢复 → 把它接入 Suite Manifest、Workbench、Nginx／Gateway／IAM、独立数据库、工作负载身份、镜像供应和联合备份恢复，并在隔离 D1 中执行启动、入口、未选缺席、销毁卷恢复和恢复后复验。**

## 验证组合

本轮使用独立 Compose project `omsd1c5` 和回环端口 `18150`，不触碰既有 Demo。选装：

- Identity & Access、Unified Workbench、Gateway、Nginx；
- Project Workspace、Contract Ledger、Evidence Service；
- Approval & Audit Results；
- Money、People、Party、Business Engagement、Billing、Production 均关闭。

Results 的直接依赖被显式固定为 Project、Contract 和 Evidence；任一依赖关闭时 D1 在创建容器前拒绝配置。它使用独立 `approval_audit_results` 数据库、独立 workload key volume、`approval-audit-results` audience 和 `approval-audit-result` Capability namespace。

## 执行证据

以下检查均实际运行并通过：

1. `go test ./...` 与 `go vet ./...`（`backend/services/approval-audit-results`）；
2. D1 image-mode、upgrade-selection、release-bundle、image-promotion 门禁；
3. `Dockerfile.approval-audit-results` 的真实本机镜像构建；
4. 启用与禁用 Results 的 Compose／Suite Manifest 渲染；
5. 隔离 D1 全栈启动，Nginx、Gateway、IAM、Project、Contract、Evidence、Results 全部 healthy；
6. `acceptance/d1-delivery/verify-running` 经 Nginx → Gateway → IAM 完成一次登录、应用授权、Results 页面打开、已选应用读写和未选应用 `404`；
7. `d1 check` 确认 IAM、Contract、Project、Evidence 和 Results schema 均 current；
8. `d1 backup` 生成并校验 `identity-access.dump`、`project-workspace.dump`、`approval-audit-results.dump`、Contract／Evidence ZIP、签名密钥、十项应用选择清单和 SHA-256；
9. 销毁该隔离 project 的容器、网络和卷后，`d1 restore` 从空目标恢复成功；
10. 恢复后再次运行同一正式入口验收并通过，Evidence 的原 File／Version／对象字节也被实际读取。

首次启动还发现并修正了一个真实装配缺陷：Results 的 audience 与原生 Capability namespace 不同，Suite Manifest 缺少显式 `capability_namespace` 时 Gateway 正确失败关闭。补充 `approval-audit-result` 后 Gateway 才允许启动，证明入口没有因新增应用放宽安全校验。

本机 Docker 默认地址池已被既有隔离项目占满，因此本轮通过 `D1_COMPOSE_OVERRIDE_FILE` 为测试栈指定未占用私网段。该覆盖只用于并行本地验收，不进入正式默认网络配置。

## 当前能声明什么

- Results 已是正式 D1 可选应用，不再是只能单独启动的开发模块；
- 启用时同时进入数据库、migration、IAM Catalog、Gateway 路由、Workbench、镜像、备份和恢复；
- 关闭时路由、菜单、容器和备份项均缺席；
- audience／Capability namespace、应用数据库和工作负载密钥仍保持独立所有权；
- 联合恢复已用真实销毁卷演练，不只是静态脚本检查。

## 仍不能声明什么

本轮通用 D1 验收只打开 Results 正式页面，没有在独立进程之间完成一条 `active Project → exact Contract revision → approval → audit → completion assessment → explicit confirmation` 业务记录。因此 C5 仍是部分通过，不得把“可装配、可恢复”误写成“完整业务旅程已通过”。后续必须补：

1. 真实跨进程批复、审计和 Project Completion Profile 旅程；
2. Results／Project 的撤权、账号停用、相邻 Project 和双 Tenant 正式入口矩阵；
3. 真实浏览器动作收敛；
4. 公司 Ubuntu 或个人公网试验田的同版本复验。

