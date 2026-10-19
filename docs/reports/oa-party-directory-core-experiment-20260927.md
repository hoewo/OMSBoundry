# OA Party Directory 核心运行时实验报告

日期：2026-09-27

结论：**第一条独立核心运行时通过；Party Directory 完整交付、消费者迁移与 OA 总装尚未通过。**

后续状态：同日下一轮已通过独立 release、非 root 镜像与空目标恢复；见 [Party Directory 交付恢复实验](./oa-party-directory-delivery-recovery-experiment-20260927.md)。本报告仍保留为“核心轮”当时的证据边界。

## 1. 本轮要回答的问题

参考 OA 中的甲方、资质单位、合作方等主体不能继续由 Contract、Money、Project 各自复制一套可变目录，否则新增应用会不断复制 Party ID、revision、去重、权限和迁移规则。需要验证：在不共享数据库、不导入其他应用领域模型、不改写既有业务快照的前提下，独立 Party Directory 是否能完成最小主体生命周期，并由 IAM 契约保护。

本轮只验证 Party 自身核心，不把以下事项混入结论：管理页面、可执行 release、非 root 容器、备份恢复、D1 正式入口、Contract／Money 迁移和完整 OA 组合。

## 2. 固定边界

- Party Directory 拥有当前主体档案、稳定 Party ID 和不可变 revision；
- 消费应用拥有主体在本业务中的角色与确认时快照；
- Party 不拥有账号、人员名册、合同、义务、项目角色或客户专用字段；
- 相同名称／登记标识不自动合并；
- 每个应用写自己的数据库，跨应用只走版本化 API；
- IAM 签发目标应用原生 Capability，Party 对 Tenant、Audience、Action、Scope、状态与 Expected Revision 作最终判断。

## 3. 实现切片

新增 `backend/services/party-directory` 独立 Go 模块与 PostgreSQL migration，包含：

1. `parties` 当前投影；
2. `party_revisions` 完整不可变快照；
3. `party_commands` 命令幂等结果；
4. `party_audit_events` 追加审计；
5. 数据库 deferred constraint trigger，保证事务提交时当前投影等于最新 revision；
6. 数据库不可变 trigger，拒绝修改／删除 revision、命令和审计；
7. organization／individual、active／inactive、字段规范化与稳定 HMAC 分页游标；
8. Tenant + Command ID advisory lock、Expected Revision 和失败无部分写入；
9. 创建、搜索、详情、更正、状态、历史和精确引用 HTTP API；
10. 正式 IAM Ed25519 Provider 与同形本地 Template Provider；
11. 配置失败关闭、健康检查和自动／只校验 migration 模式。

## 4. 验证步骤与结果

### 4.1 机器契约

Party 模型、七个 API 契约和 Capability Manifest 均通过 YAML／JSON 解析及 access-contract 自动化。该步骤证明版本化输入可被机器读取，不单独证明运行时。

### 4.2 根工作区与独立模块

实际通过：

```bash
go test ./backend/services/party-directory/...
go vet ./backend/services/party-directory/...
go build ./backend/services/party-directory/...

cd backend/services/party-directory
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

结果证明 Party 没有依赖根 `go.work` 才能编译的隐藏源码耦合。

### 4.3 PostgreSQL 18 真库与 race

实际通过：

```bash
TEST_DATABASE_URL='postgres://测试用户:测试密码@127.0.0.1:测试端口/测试数据库?sslmode=disable' \
go test -tags=integration -race ./backend/services/party-directory/internal/party
```

覆盖结果：

- 创建 revision 1 与相同命令精确重放；
- 相同 Command ID 改载荷冲突；
- 更正 revision 2；
- stale Expected Revision 与无有效变化不产生部分写；
- 停用 revision 3 后拒绝新引用；
- history 返回 3、2、1；
- 默认列表只返回 active，`status=all` 可解释停用记录；
- 跨租户按不存在处理；
- 直接篡改历史被数据库拒绝；
- 只改 current、不追加匹配 revision 被数据库拒绝；
- 失败命令不残留成功命令、revision 或审计；
- 8 个并发相同创建全部返回同一 ID，数据库只产生一个 Party 和一个 revision。

首次真库运行暴露 deferred consistency trigger 在不同表触发记录上直接引用 `NEW.party_id` 的错误。修复为从触发记录 JSON 中安全选择 `id`／`party_id` 后，全部真库与 race 用例通过。这是有效实验发现，而不是跳过失败。

### 4.4 IAM 与 HTTP

自动化覆盖：

- 无凭证、错误 issuer、audience、tenant、action、scope、过期及未来凭证拒绝；
- 客户端伪造身份 Header 在认证前删除；
- 人类列表要求 Tenant Scope；
- 工作负载 reference 只接受 `party-directory.reference.read` 和一个精确 `party:{id}` Scope；
- Party 自身返回并执行 `allowed_actions`；
- revision、幂等、无效变化与依赖错误分别映射为 409、422 和 503。

### 4.5 独立进程旅程

应用以独立端口 `127.0.0.1:18190`、Template Provider 和隔离 PostgreSQL schema 启动，实际完成：

```text
ready
→ create revision 1
→ 相同命令 replay revision 1
→ update revision 2
→ stale update 409
→ inactive revision 3
→ status=all + query 搜索
→ history revision 3 / 2 / 1
```

测试进程随后关闭，隔离 schema 删除，没有遗留测试业务数据。

原计划用 `compose.yaml` 拉起单独 PostgreSQL，Docker 在创建网络前报告预定义地址池耗尽。为区分产品缺陷和宿主环境问题，实验改用已经运行的 PostgreSQL 并以随机 schema 隔离。它证明了独立进程与数据库生命周期，但**不证明 Party 容器、release 或 Compose 交付已经通过**。

## 5. 当前可以声明什么

可以声明：

- Party Directory 的最小领域边界和数据不变量可以在独立应用内实现；
- 不需要共享 Contract／Money 表或复制它们的 ORM；
- revision、幂等、并发和数据库防篡改可以形成双层保护；
- 正式 IAM Provider 与本地同形 Provider 可以替换实现而不关闭鉴权；
- 精确 Party 引用可作为后续消费者保存本地快照的契约入口。

不能声明：

- Party 已经是可交付产品；
- 备份／空目标恢复、非 root 镜像、D1 正式入口或页面体验已经通过；
- Contract、Money 或 Project 已经从本地 Party 目录迁移；
- Party 停机时消费者既有快照与新命令的组合故障行为已经验证；
- OA 完整体已经完成。

## 6. 下一轮最小因果切片

先完成 Party 自身可执行 release、非 root 容器、备份／空目标恢复和恢复后继续写入，使“核心可运行”提升为“可独立交付与恢复”。然后选择一个消费者完成精确引用、确认时快照、Party 更名／停用不改历史和 Party 停机失败关闭；最后再进入 D1 正式入口与现有双目录迁移。这个顺序可以把交付问题、跨应用契约问题和历史迁移问题分开归因。
