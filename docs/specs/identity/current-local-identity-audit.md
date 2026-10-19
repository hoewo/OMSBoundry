# M7-001 现有本地身份实现审计

日期：2026-09-20  
状态：current code fact；作为 IAM 迁移输入

## 1. 审计目的

两个台账已经各自拥有可用本地登录，但不能因为字段相似就把任一实现直接提升为公共 IAM。本审计固定当前运行时事实、共同能力、差异和迁移约束，为 IAM Context v1 与历史 Actor 迁移提供依据。

## 2. 共同事实

Contract Ledger 与 Money Flow Ledger 当前都具备：

- 独立 `local_users` 与 `local_sessions` 表；
- 小写规范化用户名与数据库唯一约束；
- 12～72 字节密码、BCrypt 摘要和不存在用户的 dummy hash 比较；
- 连续 5 次失败后锁定 15 分钟；
- 32 字节随机 Session Token，数据库只保存 SHA-256；
- Session TTL 默认 12 小时，账号停用或 generation 变化后失效；
- `HttpOnly`、生产可设 `Secure`、`SameSite=Lax` 的应用专属 Session Cookie；
- 双提交 CSRF Cookie／表单值检查，CSRF Cookie 使用 `SameSite=Strict`；
- 服务端从会话解析 Principal，并把可信 Actor 写入不可变审计；
- bootstrap 只允许空用户库建立首位管理员，不会覆盖既有账号。

## 3. 差异

| 事实 | Contract Ledger | Money Flow Ledger | IAM 影响 |
| --- | --- | --- | --- |
| 角色 | administrator、editor、viewer | 只有 administrator | 不能把任一应用角色直接当成企业岗位或 IAM 通用角色 |
| 权限 | business.write／correct／export／restore、identity.manage | 所有业务路由只认 administrator | 必须从实际业务动作建立应用自有 Capability Manifest |
| 用户管理 | 管理员可建用户、改角色／状态、重置密码；用户可改密码 | 只有 bootstrap、登录和注销 | 账号生命周期统一进入 IAM，不能继续由 Contract 代管 |
| Cookie | `contract_ledger_session`、`contract_ledger_csrf` | `money_flow_ledger_session`、`money_flow_ledger_csrf` | 中心模式新增 Suite Session 和短期应用凭证；旧 Cookie 不复用 |
| Actor | `local_user` + 本应用 User UUID | `local_user` + 本应用 User UUID | 同一人可能有两个不同 UUID，历史审计不能按用户名强行合并 |
| Evidence | Contract 已有嵌入式 Evidence 操作 | 当前无文件能力 | Contract Capability 需要单独表达 Evidence 写入动作 |

## 4. 当前风险与不能直接复用的部分

- 两个数据库中的相同用户名不证明是同一个自然人；显示名也不能作为合并键；
- 两个 BCrypt hash 使用独立 salt，即使密码相同也不能靠摘要相等判断账号相同；
- Money Flow 的 administrator 不能自动映射为 Contract administrator；
- Contract 的 `identity.manage` 是本地交付功能，不是业务应用在中心模式下继续拥有账号的理由；
- 旧 Session 只对本应用有效，不能交换成 IAM Session 或 Capability Credential；
- `actor_kind=local_user` 与本地 User UUID 已写入不可变审计，不能替换成新 IAM Subject；
- 当前应用数据没有 tenant 列，M7 只能支持“一个安装实例绑定一个 Tenant ID”，不能宣称行级多租户。

## 5. 迁移规则

1. 中心模式启用前，运维显式建立一个安装 Tenant，并把同一个 Tenant ID 配置给 IAM、Gateway 和两个应用。
2. IAM 首位管理员在 IAM 空库重新 bootstrap；不从任一业务库静默推断企业管理员。
3. 既有本地账号以“待邀请／待激活”身份导入 IAM，不复制密码摘要；用户在 IAM 重新建立密码。
4. 同名账号来自两个应用时必须由操作者显式确认是否关联同一 IAM Subject；未确认时保持两个候选，不自动合并。
5. 每个业务应用新增只读历史关联，记录 `local_user_id → tenant_id + iam_subject_id`；关联用于解释历史，不修改旧审计行。
6. 中心模式的新操作写入 `actor_kind=iam_subject`、IAM Subject UUID 和当时显示名；旧操作继续保留 `actor_kind=local_user`。
7. 切换中心模式时撤销本应用全部旧 Session；本地密码保留用于受控回退，但中心模式不读取它。
8. 回退到本地模板 Provider 后用户必须重新登录；IAM Session 或 Capability Credential 不能被本地 Provider 接受。
9. 关联冲突、未知本地用户、重复 Subject 或 Tenant 不一致全部失败关闭，不用用户名临时兜底。

## 6. Capability 提取结果

正式 v1 清单位于：

- [`contracts/access/v1/examples/contract-ledger-capabilities.json`](../../../contracts/access/v1/examples/contract-ledger-capabilities.json)；
- [`contracts/access/v1/examples/money-flow-ledger-capabilities.json`](../../../contracts/access/v1/examples/money-flow-ledger-capabilities.json)。

Capability 使用用户可理解的业务动作，不沿用 `business.write`、`administrator`、菜单或按钮名称。IAM 可以把角色授予转换为这些动作，但目标应用只认识自己发布的 Capability。

## 7. 审计结论

两套本地实现证明了密码、Session、CSRF、可信 Actor 和独立交付的最低机制，但它们是两个不同身份域。IAM 迁移必须采用“新 IAM Subject + 显式历史关联 + 新旧 Actor 并存”，不能合库、改写审计或按用户名自动合并。
