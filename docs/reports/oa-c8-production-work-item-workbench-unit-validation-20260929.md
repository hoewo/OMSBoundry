# OA C8 Production WorkItemProjection → Unified Workbench 低资源单元验证

日期：2026-09-29  
判定：`c8-single-source-personal-work-unit-supported / gateway-browser-d1-pending`

## 问题 A → 可验证结果 B

**A**：参考 OA 可以直接从共享 Project／User 表推导首页待办；BizFoundry 若复制这种方式，会让 Workbench 拥有来源状态、接受浏览器提交的用户身份，或迫使多个应用共享任务表。

**B**：一个现有事实所有者发布绑定当前 IAM Subject 的 `WorkItemProjection v1`；Workbench 只从 Suite Manifest 发现来源、读取、校验、分组和深链导航。Assignment／Grant 失效、相邻 Subject／Scope、来源故障和无可执行动作均失败关闭，工作台不写来源状态。

## 本批实现

- Production Operations 新增 `production-work-item.work-items.read` 与 `GET /api/v1/work-items`；
- 来源只选择未完成／未取消、active direct-person `production_owner` Assignment 属于 Credential Subject、且 Credential Scope 覆盖目标资源的 Work Item；
- `allowed_actions` 由目标应用结合当前 Credential 和状态计算。没有 stage／deliverable／cancel 任一可执行动作时不发布可执行条目；
- ended／stale Assignment 与相邻 Subject 被排除，Project Assignment 引用服务不可用返回 503；
- 投影包含稳定 source、Work Item／Project／Scope、reason、state、action、resource revision、observed time、相对来源 deep link 和 allowed actions；
- Suite Manifest v1 新增可选 `work_item_projection` 声明，Gateway、Workbench、JSON Schema 和 Access Contract 使用同一严格字段；
- Workbench 页面只对 enabled 声明来源读取：最多四路并发、每来源 3.5 秒超时，校验 schema／source／Subject，组内排序和去重；
- Gateway 仍持有短期 Credential，浏览器脚本只通过来源路径限定的 HttpOnly Cookie 使用它，不读取 Token、不提交 Subject；
- Production 深链接收 `work_item_id`，进入后重新读取 current 详情、revision 和 `allowed_actions`。

## 串行验证

本批未调用 Docker、未启动数据库或常驻应用，只按下列顺序运行一个模块：

```bash
cd backend/services/design-operations
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../unified-workbench
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../gateway
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../../acceptance/access-contract
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
```

四组均通过。另以 `node --check` 分别检查 Workbench 与 Production 嵌入 JavaScript，`sh -n` 检查 D1 Manifest 渲染脚本，`git diff --check` 通过。

直接覆盖：

- 当前 Subject、相邻 Subject、active／ended／stale Assignment；
- 精确 Project Scope 与相邻 Project 排除；
- 独立 `work-items.read` 缺失为 403；
- 可执行写动作撤销后响应成功但 `items: []`，不沿用旧动作；
- Assignment 引用服务不可用为 503，而不是空；
- Manifest 非法平台来源、schema／path／action 失败关闭；
- Workbench 不存在 Subject 输入，并校验响应 Subject；
- 单来源超时／拒绝／契约错误只改变该来源状态；
- Production deep link 重新读取目标详情。

## 为什么不用 Workbench workload 冒充个人

服务身份只证明“Workbench 服务可以调用某端口”，不能证明“某个人当前仍拥有某业务动作”。让 workload token 携带页面传入的 Subject 会绕过人的 Grant／停用状态。第一版因此复用现有 Gateway／IAM Session 签发链：IAM 仍决定人的 audience、action 和 Scope，来源应用再复验 Assignment 与当前状态，Workbench 既不签发也不代理写命令。

若未来需要服务端聚合，必须先新增受信 Policy Provider 的显式 on-behalf-of 契约和撤权验证；不能把本批浏览器聚合静默改成服务身份代理。

## 未覆盖与下一批

本批没有运行真实 IAM、Gateway、Production、PostgreSQL、浏览器或 D1，因此不证明：

- Gateway→IAM→Production 的真实 Cookie 路径、Grant 撤销窗口和账号停用；
- 正式 D1 的选装／未选缺席、来源停机局部恢复、备份恢复；
- 第二工作项来源与多来源真实并发；
- Group Assignment 经 People Membership／Policy 解析为个人；
- 双 Tenant、真实 Ubuntu／HTTPS 或性能目标。

下一批仍按低资源原则只选择一项：真实浏览器／正式入口最小链，或第二来源局部失败；不得同时启动完整 D1 和多个尚未稳定的来源。
