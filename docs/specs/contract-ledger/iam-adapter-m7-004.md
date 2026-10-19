# M7-004 Contract Ledger IAM 试点迁移

日期：2026-09-20  
状态：implemented and verified

## 1. 最小根因

> Contract Ledger 只认识自己的本地账号 → 同一套合同能力既能由中心 IAM 凭证驱动，也能在独立交付时由本地模板账号驱动。

## 2. 身份提供者边界

Contract Ledger 通过 `IDENTITY_PROVIDER=local|iam` 选择身份提供者，但所有 HTTP 写入继续经过同一组服务端权限与领域校验：

- `local` 保留原有本地用户、密码、Session、管理员／编辑／只读角色，供独立安装和测试使用；
- `iam` 只接受 `oms_cap_contract_ledger` 中的 Ed25519 Capability Credential，不保存 IAM 密码、Session 或角色；
- IAM 模式严格验证 issuer、Tenant、`contract-ledger` audience、有效期、签名 Key、Tenant Scope 和动作命名空间；
- Contract 不在线查询 IAM，也不读取 IAM 数据库；active／retiring 公钥通过原子发布文件离线加载；
- IAM 模式不注册本地登录、密码和用户管理路由，不允许以本地管理员作为旁路。

## 3. 能力到业务动作的映射

| Capability | Contract 最终执行边界 |
| --- | --- |
| `contract-ledger.view` | 进入受保护页面，读取合同、主体、证据元数据和审计 |
| `contract-ledger.party.register` | 登记 Contract 自己的签约主体 |
| `contract-ledger.draft.manage` | 建立、编辑、取消草稿和维护草稿签约主体 |
| `contract-ledger.lifecycle.manage` | 激活、变更、终止、结算、关闭和建立合同关系 |
| `contract-ledger.correct` | 追加受控更正文书 |
| `contract-ledger.evidence.manage` | 新建证据及追加版本 |
| `contract-ledger.export` | 导出合同账簿 |
| `contract-ledger.restore` | 恢复合同账簿 |

Gateway 的 `entry_action` 只控制是否可以进入应用。Contract 针对每条 POST／特权 GET 再检查具体 Capability，页面也只显示对应 `allowed_actions`；即使客户端自行构造请求，服务端仍会返回 403。合同状态、revision、金额、幂等和审计规则不因 IAM 通过而跳过。

## 4. 历史身份与审计

- 新的中心身份写入审计时使用 `actor_kind=iam_subject`、IAM Subject UUID 和签发时的显示名；
- 既有 `local_user` 审计不改写，旧 Session 不转换成 IAM Session；
- `local_user_iam_subject_links` 只保存操作者明确确认的历史账号映射，记录 Tenant、IAM Subject、执行映射的 IAM Subject、原因和时间；
- 映射表由 PostgreSQL Trigger 禁止更新和删除，不能以用户名或显示名自动猜测身份；
- `contract-ledger-ops link-iam-subject` 是唯一运维写入口，并要求显式 UUID、理由和执行者。

## 5. 路径与故障行为

IAM 组合模式以 `PUBLIC_BASE_PATH=/contract` 生成页面链接和表单 action；Gateway 去掉该前缀再转发到 Contract。Contract 自己不依赖 Nginx 或 Gateway 的普通身份 Header，只相信可复验的签名凭证。

- 直接访问内部 Contract 端口且没有凭证返回 401；
- 错误 audience、action、Tenant、issuer、时间或签名失败关闭；
- IAM 临时不可用时，已签发且未过期的凭证仍可离线验证；新登录、新授权和续签失败；
- Money Flow 停机不影响 Contract 已授权读写；
- local 模式不需要 IAM、Gateway 或 Money Flow 即可独立运行。

## 6. 已验证结果

- 单元测试覆盖 Credential 签名验证、错误 audience、Capability 到内部权限映射、`iam_subject` Actor、IAM 配置、缺凭证 401、只读凭证写入 403、本地身份路由隐藏、前缀链接和按具体 Capability 显示表单；
- PostgreSQL migration 集成测试验证显式历史身份映射可写、重放安全且 UPDATE 被数据库以 `55000` 拒绝；
- 实际 Contract 开发库显式从 `000016` 升级到 `000017_local_user_iam_subject_links.sql`，随后以 IAM／`verify` 模式启动并读取 PostgreSQL 18.6；
- 实际链路 `Nginx → Gateway → IAM → Contract` 完成管理员登录、八项 Contract Capability 授予、单 Audience 凭证交换、主体创建和列表回读；
- 创建审计实际记录 `actor_kind=iam_subject`、IAM Subject UUID 和“系统管理员”，直接无凭证访问 Contract 返回 401；
- 暂停 Money Flow 进程期间，统一入口中的 Contract 仍返回 200 并读取刚写入的数据；恢复 Money Flow 后端口重新监听；
- Gateway 与 IAM 的真实凭证响应联调发现并修复了 `token_type`／`claims` 严格解析不一致，回归测试固定完整响应格式。

M7-004 不迁移 Money Flow，不建立 Contract → Money Flow 连接器，也不把 IAM Subject 复制成 Contract 本地用户。下一关是 M7-005。
