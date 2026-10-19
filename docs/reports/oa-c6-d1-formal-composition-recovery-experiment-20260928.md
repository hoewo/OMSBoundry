# OA C6 正式 D1 装配、故障与恢复实验

日期：2026-09-28  
结论：`formal-d1-assembly-and-recovery-supported / contract-party-runtime-cutover-pending`

## 1. 要回答的问题

本轮只验证一个收缩后的问题：已经在本地独立进程中成立的 Cost & Outsourcing，能否作为正式 D1 可选应用，使用自己的镜像、数据库、migration 和工作负载身份，经 Nginx → Gateway → IAM 入口完成业务，并在未选装和空目标恢复时保持边界。

本轮不实现 Payable Suggestion，不实现 C7 奖金，不把 C6 纳入 Project 完成条件，也不以本地 D1 结果冒充真实 Ubuntu 或公网验收。

## 2. 被验证的装配

选择集为 People & Groups、Party Directory、Project Workspace、Contract Ledger、Evidence Service 和 Cost & Outsourcing；Money Flow、Business Engagement、Billing、Production、Approval & Audit Results 与 C7 不启动。

正式请求链为：

```text
浏览器／验收驱动
  → Nginx
  → Gateway
  → IAM Session 与 Project Scope
  → Cost & Outsourcing 最终授权
  → IAM 工作负载凭证交换
  → Project／Party／Contract／Evidence 精确引用端口
```

C6 使用独立 PostgreSQL database、账号、migration、镜像、运行进程和 Workload Key。Evidence Service 是没有业务工作台的支撑服务：它必须被选择并可路由，但不出现在 Workbench 菜单。

## 3. 首次验证步骤

### 3.1 静态交付门禁

1. 构建 Cost & Outsourcing 的 Linux AMD64、非 root 镜像；
2. 检查 Compose、pull-only Compose、Suite Manifest、环境模板、数据库 provisioning、release／publish／promote、backup／restore／upgrade 均包含 C6；
3. 检查镜像总数为 16，独立数据库密码总数为 13；
4. 检查 Shell 语法、Compose 渲染、Go 格式与相关应用回归。

### 3.2 正式入口与业务准备

1. 启动隔离 Compose project `omsd1c6`，公网测试入口只绑定 `127.0.0.1:18170`；
2. 通过正式 IAM 登录，分别授权 People、Party、Project、Contract、Evidence 和 C6；
3. 验证 Suite Manifest 选择集、Workbench 菜单、C6 页面、浏览器资产和 Gateway 相对路径；
4. 建立两名 People 负责人、一个 active Project、一个 Party Directory 服务方和一份 Evidence 文件；
5. 在旧 Contract Ledger 建立本地主体、激活合同并把合同精确链接到 Project。

### 3.3 故障、幂等与权限

1. 停止 Evidence，使用固定 Command ID 登记 Cost，预期 503；确认 Cost 列表仍为空；
2. 恢复 Evidence，重放同一业务意图，预期只产生一条 revision 1 Cost；再次重放返回同一事实；
3. 停止 Party，登记 Outsourcing Engagement，预期 503；确认 Engagement 列表仍为空；
4. 恢复 Party，登记 Engagement 和 5 分 Assessment；
5. 使用旧 Contract 和独立 Directory Party 组合登记 Engagement，要求失败关闭，不允许按名称或位置隐式替换 Party ID；
6. 使用 observer 身份验证 Project Scope、最小 read、金额脱敏、动态 `allowed_actions`、无 create 403 和错误 Project 403；
7. 查询 IAM 审计，要求四次交换分别精确命中 Project、Party、Contract 和 Evidence 的 audience、action、resource type 与 resource ID。

### 3.4 联合备份与空目标恢复

1. 创建正式联合备份 `20260928T024459Z`；
2. 只销毁隔离 `omsd1c6` 的容器、网络和数据卷；
3. 对空目标执行正式 restore；
4. 重新登录和授权，读取恢复后的 Cost、Engagement 与 Assessment；
5. 比较恢复前后 canonical SHA-256、稳定 ID、revision、Project／Party／Evidence 快照和历史；
6. 恢复后追加 Cost correction，要求 revision 1 → 2、历史为 `recorded → corrected`。

### 3.5 未选装与重新选装

1. 把 `ENABLE_COST_OUTSOURCING` 设为 false 并应用选择集；
2. 确认 C6 runtime 缺席、Suite Manifest 为 disabled、Workbench 无入口、Gateway route 为 404；
3. 创建备份 `20260928T031009Z`，确认 selection 为 false 且没有 `cost-outsourcing.dump`；
4. 重新启用 C6；正式 Gateway 先返回授权跳转，完成授权后页面返回 200，原数据库卷没有被取消选装删除。

说明：未选装阶段的顺序断言已全部通过。验收脚本最初把重新选装后的未授权 303 错当成 200，修正为先授权再要求 200；完整脚本随后一次 Compose 重建等待被人工中止，重新启动完成后已单独实际复验“授权 303 → 路由 200”。因此本报告把这项记为分段直接证据，不伪称修订后的整段脚本又完整跑了一遍。

## 4. 结果

- 首次正式业务旅程：57 个 HTTP 断言，0 个状态不匹配；
- Cost ID：`9a855e88-3e6a-4be9-9de6-0d49a58fbb4d`；
- Engagement ID：`fa085720-8b38-4b7e-9d8f-07609a97e503`；
- Project ID：`7c695924-1fd9-4818-82f0-1a198fde7486`；
- Party ID：`e876c1b9-4165-466c-9df3-1fb93b7e014f`；
- Contract ID：`4c093dfb-1377-4e67-9e41-9f34a2433c58`；
- Evidence File ID：`10cf0c06-7f91-409b-b437-659d5f73f1d6`；
- Evidence／Party 停机均返回 503 且零部分写入，恢复后同一意图只成功一次；
- observer 金额字段被服务端脱敏，写动作和错误 Project 均被 403 拒绝；
- IAM 审计精确记录四个目标应用的单 audience、单 action 和单资源凭证；
- 旧 Contract 本地主体与 Party Directory 主体不一致时返回 422 `REFERENCE_INVALID`；
- 恢复前后 Cost、Engagement、Assessment canonical SHA-256 分别保持：
  - `f2dee9ffe391ca857e7ef923785efc7ffd7287bd1a63cd54a93487f6e385388e`
  - `8aa514c5a00c0bb5898e38b8ebe0fc51ef8db6681619aeb8249ce16821cf92f3`
  - `fbaf91f4b8c8f552ce19dae7a6cee51e48acfb6dcb508d14d66d0ec481219e6d`
- 恢复后继续写入成功，Cost revision 从 1 变为 2，旧 revision 保留；
- 相关 Cost、Gateway、IAM、Project、Party、Evidence 与 Contract Go 回归全部通过。

## 5. 边界结论

### 5.1 已支持

C6 已是 D1 的正式可选业务应用，而不是只在开发机能运行的模块。它有独立数据所有权、正式入口、IAM 最终授权、精确工作负载引用、故障失败关闭、选装边界和联合恢复证据。Money、Billing、Production、Results 与 C7 的缺席没有阻止最小 Cost／Engagement 组合成立，说明“独立应用允许显式直接依赖”这一解耦定义在本切片成立。

### 5.2 发现的真实边界

旧 Contract Ledger 仍拥有本地 Party ID；C6 使用 Party Directory ID。迁移预检当前仍是 `ready_for_cutover=false`，运行时切换和回滚没有完成。两者不能因为名称相同就视为同一主体，也不能由 C6 私自维护第二套映射。因此：

- 不带 Contract 的 C6 Engagement 已成立；
- 带旧 Contract 的 C6 Engagement 在主体 ID 未对齐时必须失败关闭；
- “Contract＋Party Directory＋C6 完整外协链”仍未成立；
- 下一轮应优先完成来源应用自有的 Party 运行时切换／映射／回滚证据，再重跑带 Contract 的 C6 旅程。

这不是解耦失败，而是边界正在正确阻止隐式耦合；但它也是阶段一必须关闭的集成缺口。

### 5.3 仍未证明

- C6 Payable Suggestion → Money 独立确认 Obligation；
- Billing 投影与票据业务；
- C6 是否进入 Project Completion Profile；
- 完整双 Tenant、C7／C8 和整套 OA 端到端旅程；
- 真实公司 Ubuntu／公网入口、正式 clean-HEAD 制品和第二客户适配。

## 6. 可复演入口

```sh
acceptance/d1-c6-cost-outsourcing/run-isolated full

go test ./backend/services/cost-outsourcing/... ./backend/platform/gateway/... ./backend/platform/identity-access/... \
  ./backend/services/project-workspace/... ./backend/services/party-directory/... \
  ./backend/services/evidence-service/... ./backend/services/contract-ledger/...
```

运行夹具会删除并重建自己的 `omsd1c6` 数据卷，不能把 `D1_PROJECT_NAME` 指向共享或生产环境。
