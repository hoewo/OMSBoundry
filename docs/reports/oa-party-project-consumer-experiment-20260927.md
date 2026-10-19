# OA Party → Project 首个消费者边界实验

日期：2026-09-27  
结论：`supported-for-local-boundary`；D1、客户页面和旧目录迁移仍为 `insufficient-evidence`

## 1. 要回答的问题

Party Directory 已经能够独立运行、发布和恢复，但“其他应用通过连接器直接使用它的数据和模型”还没有实际消费者证据。本实验回答一个更窄的问题：

> 参考 OA 在项目立项时选择客户，究竟应由 Project Workspace、Business Engagement 还是 Party Directory 拥有；选定后，能否既复用 Party 当前档案，又不共享数据库或让历史被当前资料改写？

## 2. 参考事实与边界修正

对参考 OA 工程的只读审计显示：

- `clients` 是站点级客户主数据；
- `projects.client_id` 表达“这个项目选择了哪个客户”；
- 项目详情只拥有该关系及项目语境；
- Business Engagement 表达合同形成前的承接过程，不是默认客户关系所有者。

因此原先把首个 Party 消费者放入 Business Engagement 的候选被否定。稳定边界改为：

- Party Directory 拥有主体当前档案、稳定 ID、状态和 revision；
- Project Workspace 拥有项目选择客户这一业务关系，并冻结选择发生时的最小快照；
- Business Engagement 继续只引用 Project；只有以后出现与项目客户不同、且证据充分的独立主体角色时，才直接引用 Party；
- 未选择客户时 Project 仍能独立创建，参考 OA Overlay 可在未来页面／规则中把该可选能力收紧为必填。

## 3. 实验实现

普通 Project 创建请求新增可选：

```json
{
  "client_party": {
    "party_id": "<uuid>",
    "source_revision": 1
  }
}
```

Project Workspace 使用 Party Directory 的 `GET /api/v1/references/parties/{id}` 验证：

- `schema_version=1`；
- `source_application=party-directory`；
- tenant、Party ID 和请求完全一致；
- 状态为 `active`；
- revision 精确一致；
- kind 为 `organization` 或 `individual`；
- legal name 非空。

验证成功后只保存：

```text
party_id + source_revision + kind + legal_name
```

不复制登记标识、联系方式、搜索目录或 Party 完整对象。migration `000003_project_client_party.sql` 用 all-or-none 约束保护快照完整性，并禁止 Package 保存客户快照。

正式 IAM 模式使用 Project Workspace 工作负载身份请求：

```text
audience = party-directory
action = party-directory.reference.read
resource = party:<exact id>
```

客户端 Session、用户 Cookie、长期 Bearer Token 和云密钥都不进入该调用。

## 4. 验收结果

| 场景 | 结果 |
| --- | --- |
| active、同租户、精确 revision | 创建 revision 1 Project，返回并持久化最小快照，`project_created` 历史同值 |
| Party 后来改名并推进 revision | Project 详情和创建历史仍返回原名称／revision |
| 同 command 重放时 Party 已变化 | 返回同一 Project；Party 调用计数保持不变 |
| stale revision | HTTP 422；数据库没有对应 Project |
| Party 503 | HTTP 503、retryable；数据库没有对应 Project |
| 跨租户、inactive、错误 source | 契约测试全部拒绝为无效引用 |
| 未配置 Party、请求也未选择 Party | 原有 Project 创建不受影响 |
| 基础资料更新夹带 `client_party` | 明确拒绝；首个切片不静默重选客户 |
| 正式 IAM 凭证请求 | audience、action、resource type 和精确 ID 全部匹配 |

实际执行：

```bash
cd backend/services/project-workspace
GOWORK=off go test ./...
GOWORK=off go vet ./...

TEST_DATABASE_URL='postgres://project_workspace:***@127.0.0.1:55437/project_workspace?sslmode=disable' \
  GOWORK=off go test -tags=integration ./internal/projects ./internal/httpserver
```

以上检查均通过。集成测试使用随机创建并自动销毁的 PostgreSQL 18 数据库，没有修改当前 Demo／D1 业务数据库。

## 5. 结论

本实验支持以下主张：

1. Party 的当前数据模型可以被 Project 直接消费，不需要共享表、共享 ORM 或复制整个领域模型；
2. “直接可用”不是实时引用和历史快照二选一：创建时用当前精确 revision 验证，创建后由消费应用保存本业务发生时的最小快照；
3. Party 故障只阻断需要新验证的写命令，不破坏既有 Project，也不阻断未选择 Party 的 Project；
4. 参考 OA 的客户关系应放在 Project；把它放进 Business Engagement 会形成错误所有权；
5. 这是一项 Base 的向后兼容加法：旧 Project 和不需要客户主体的组合继续成立，参考 OA Overlay 可以在未来要求选择客户。

## 6. 本实验没有证明

- Party Directory 已进入 D1 的 Nginx → Gateway → IAM 正式入口；
- 客户选择／搜索页面已经可用；
- Project 客户重选、解除或主体合并后的业务语义；
- Contract Ledger／Money Flow Ledger 的本地 Party ID 已迁移；
- Party 停机时完整组合的页面降级和运维恢复；
- 一个参考 OA 足以证明第二真实客户也采用相同关系。

因此 Party Directory 的总状态从“零消费者”推进为“首个 Project 消费边界已验证”，但组合交付仍未完成。下一轮应在 D1／页面与 Contract／Money 迁移之间重新按 OA 主线价值选择一个最小缺口，不能把本轮结论扩大成全部消费者迁移成功。
