# OA Party → Project D1 正式链与恢复实验

日期：2026-09-27  
结论：`supported-for-local-d1-api-chain`

## 验证问题

> 独立 Party Directory 与首个 Project 消费边界已在代码和真库中成立 → 将它们装入正式 `Nginx → Gateway → IAM → 目标应用` D1 链，验证精确授权、故障隔离、幂等恢复和联合备份还原是否同时成立。

本实验只评估本地 D1 API 链，不声明 Party 管理页面、Contract／Money 迁移、真实 Ubuntu 或公网入口已完成。

## 装配结果

- Party Directory 以独立进程、独立 migration、独立数据库账号和独立健康检查加入 D1。
- Suite Manifest 声明 Party 路由、运行单元和 Capability；因当前尚无可用管理页，本轮不把 Party 放入 Workbench 菜单。
- Project 通过注入的 Party API 端点访问 Party，不使用固定 IP、主机名或共享数据库。
- D1 pull-only 供应链现包含 12 个运行摘要引用；发布侧管理 4 个基础制品与 10 个应用制品。

## IAM 与最终授权证据

Project 读取 Party 时不持有长期静态 Token。它使用自身工作负载身份向 IAM 交换最长 60 秒、绑定当次请求、单 action 且精确到 `party:{id}` 的 Party 原生 Capability Credential。

运行验收同时证明：

- IAM 审计记录实际的 Project workload 凭证交换；
- Party Directory 复验 issuer、tenant、audience、action 和精确 Party scope，并作最终授权；
- Project 只保存选定客户的 Party ID、revision、kind 和 legal name 快照，Party 当前档案后续变化不改写 Project 历史；
- 浏览器、Gateway 和 Project 都不能自行扩大 Party 权限。

## 故障与恢复证据

在真实 D1 容器组合中停止 Party Directory 后：

- 新建含 Party 引用的 Project 失败关闭，没有部分写入；
- 不携客户 Party 的 Base Project 仍可创建；
- 已存 Project 的客户快照仍由 Project 自身数据库读取；
- Party 恢复后，新 `command_id` 成功一次，重放不产生第二个 Project。

破坏性验收结果：

```text
D1 running-stack acceptance passed: nginx-only exposure, one login, enabled-app read/write,
People roster snapshot, Party to Project exact-reference lifecycle and Evidence byte lifecycle,
fault isolation and recovery
```

## 联合备份与空目标还原

备份 ID：`20260927T025800Z`

联合备份同时包含 IAM、People、Party、Project、Business Engagement、Contract、Money、Evidence 及签名材料，其中 Party 与 Project 分别保持自身数据库恢复边界。

恢复前核对：

- Party：2 条；
- Project：4 条；
- 携 Party 快照的 Project：3 条。

在确认为本轮专用合成 D1 环境后，执行空目标还原。所有 migration 检查通过，恢复后数量仍为 `2 / 4 / 3`，随后的非破坏性 D1 验收再次通过，证明恢复不只保存表和行，还保存了可继续执行的入口、授权与业务引用。

## 本轮结论与限制

本轮支持以下结论：

1. Party Directory 可在 D1 中作为独立数据所有者运行；
2. Project 可通过 IAM 控制的精确版本化端口引用 Party，无需共享表、共享 ORM 或分布式双写；
3. Party 依赖失败、恢复与 D1 联合空目标还原不破坏 Project 已确认快照和幂等性；
4. IAM 在这条链中不是装饰性登录服务，而是工作负载身份、精确能力授权和审计的实际边界；Party 仍拥有最终授权。

仍未完成：

- Party 管理与 Project 客户选择页面；
- Contract／Money 本地 Party 目录的显式映射、切换、回退和历史保持迁移；
- 真实 Ubuntu 目标、HTTPS 公网入口、离机备份和公司环境验收；
- 完整双 Tenant 交叉矩阵及 C3～C8。

因此，该结果是 OA 完全体可行性验证的一条已支持子证据，不是整体 `supported` 结论。
