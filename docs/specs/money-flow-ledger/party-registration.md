# Money Flow Ledger：轻量主体登记与列表

里程碑：M3  
功能编号：MF-M3-002  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 首位管理员能够在本系统登记组织或个人主体，并从 PostgreSQL 列表中看到结果。该目录是 Money Flow 自己的默认 Party 实现，为后续债权方、债务方和义务确认时的主体快照提供本地来源；它不读取、复用或同步 Contract Ledger 的 Party。

## 业务事实

- Party 表示 Money Flow 当前可选择的业务主体资料，不是登录 Local User；
- `organization` 表示组织，`individual` 表示个人；
- 法定名称或姓名必填，去除首尾空白后长度为 1 至 200 个字符；
- 登记标识可选，去除首尾空白后最长 100 个字符；空值持久化为 NULL；
- 新建 Party 的 revision 固定为 1；
- revision 1 代表当前目录资料版本，不是义务中的历史快照；后续资料变更不得反向修改既有义务快照；
- 登记标识本轮不声明全局唯一，不猜测不同国家、个人或组织的标识规则；
- Party ID、Command ID 和 Audit ID 都是不同的 UUID，不用名称或登记标识充当身份；
- 创建 Actor 只能来自 Money Flow 已验证的本地管理员会话。

## 用户流程

```text
管理员登录
  → 打开 Money Flow 主体列表
  → 页面生成一次性 Command ID 与 CSRF token
  → 填写类型、法定名称／姓名、可选登记标识
  → 服务端标准化并验证
  → Party、创建命令和审计在同一事务提交
  → 返回列表并显示 revision 1
```

## 幂等与并发

- 每次表单意图携带一个 UUID Command ID；
- 幂等负载由标准化后的 kind、legal name、registration identifier 和可信 Actor 身份计算 SHA-256；
- 相同 Command ID 与相同负载重放返回原 Party，不新增 Party、command 或审计；
- 相同 Command ID 改变任何业务意图或 Actor，返回显式幂等冲突；
- PostgreSQL 事务按 Command ID 取得 advisory transaction lock，使并发相同命令只形成一个结果；
- 不通过名称或登记标识做“相似主体”自动合并，避免误把不同主体静默合并。

## 权限、审计与数据边界

- GET／POST `/parties` 都必须先通过 Money Flow 本地会话；
- 本轮只有 administrator 可以登记；非管理员即使拥有有效会话也得到 `403`；
- 请求中的 `actor_kind`、`actor_id`、`actor_display_name` 等字段全部忽略，Actor 只由 Principal 构造；
- `party_created` 审计保存 Command ID、可信 Actor、Party revision 和标准化事实摘要；
- Party、创建命令和审计必须原子提交；审计失败不能留下 Party 或 Command；
- 创建命令和审计禁止 UPDATE／DELETE；Party 本轮没有编辑或删除入口；
- Money Flow 只写自己的数据库，不访问 Contract Ledger 数据库、Cookie、用户或 HTTP 接口。

## 验收标准

1. administrator 可以登记 organization 和 individual，并在稳定排序列表中看到 revision 1。
2. 非法类型、空名称、过长名称、过长登记标识和非法 Command ID 返回字段级错误且不写数据库。
3. 输入首尾空白在计算幂等负载和持久化之前清理；可选空登记标识读取为空字符串。
4. 相同 Command ID 和相同标准化负载串行或并发重放只形成一个 Party、一个 command 和一条审计。
5. 相同 Command ID 改变负载或 Actor 返回冲突，不返回旧结果冒充成功。
6. 故意令审计写入失败时 Party、command 和审计全部回滚。
7. HTTP POST 要求登录、administrator 和有效 CSRF；客户端 Actor 字段不能覆盖真实用户。
8. 列表来自 PostgreSQL；数据库错误不伪装为空列表或创建成功。
9. migration 从 `000002` 升级到 `000003` 并可重复执行，表、索引、约束和不可变触发器存在。
10. 单元、竞态、真实 PostgreSQL、真实 HTTP 和独立启动验证通过；Contract Ledger 停止时仍能完成登记与列表。

## 明确不做

- Party 编辑、停用、删除、合并、去重建议或批量导入；
- 登记标识格式库、真实性核验或全局唯一约束；
- Local User 与 Party 绑定；
- Contract Ledger Party 同步、共享 ID、跨库外键或自动映射；
- 义务、债权方／债务方快照、结算对象、第三方代收付；
- 客户、供应商、员工等客户专用主体类型和行业字段。
