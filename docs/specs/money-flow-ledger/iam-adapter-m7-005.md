# M7-005 Money Flow Ledger IAM 迁移

日期：2026-09-21  
状态：implemented and verified

## 1. 最小根因

> Money Flow 仍有自己的登录边界 → 用户登录 IAM 一次即可进入 Contract 与 Money Flow，而两个应用仍不共享账号表或业务数据库。

## 2. 身份与能力边界

Money Flow 使用 `IDENTITY_PROVIDER=local|iam` 选择同形身份提供者：

- local 模式继续拥有本地管理员、密码和 Session，供独立安装与测试；
- iam 模式只接受 `oms_cap_money_flow_ledger` 中 Audience 为 `money-flow-ledger` 的 Ed25519 短期凭证；
- 应用离线复验 issuer、Tenant、Subject、Audience、有效期、Key、Tenant Scope、动作命名空间和 `view` 入口能力；
- iam 模式不注册本地登录与注销路由，不保存 IAM 密码、Session 或角色，也不查询 IAM 数据库；
- Contract Credential 在 Money Flow 返回 401，Money Flow Credential 在 Contract 返回 401。

## 3. 稳定 Capability

| Capability | Money Flow 最终执行边界 |
| --- | --- |
| `money-flow-ledger.view` | 查看主体、义务、资金、分配、预测、经营汇总和历史 |
| `money-flow-ledger.party.register` | 登记 Money Flow 自有主体 |
| `money-flow-ledger.obligation.manage` | 建立、确认、调额、减免、关闭义务并维护现金预期 |
| `money-flow-ledger.cash.record` | 登记实际收款或付款 |
| `money-flow-ledger.cash.reverse` | 受控冲销错误资金事实 |
| `money-flow-ledger.allocation.manage` | 分配／核销资金与义务并受控反向核销 |
| `money-flow-ledger.transfer.record` | 登记内部账户转账 |
| `money-flow-ledger.export` | 导出本方主体可校验账簿 |

所有写路由由服务端检查具体 Capability；页面只渲染对应动作的表单。Capability 通过不跳过方向、金额、币种、revision、余额、幂等、冲销顺序和不可变审计等领域规则。

## 4. 历史身份

- IAM 写入审计使用 `actor_kind=iam_subject`、Subject UUID 和显示名快照；
- 既有 `local_user` 审计、密码摘要与 Session 历史不改写；
- `local_user_iam_subject_links` 只接受操作者显式确认的 UUID 映射和理由；
- 映射由 PostgreSQL Trigger 禁止 UPDATE／DELETE，`money-flow-ledger-ops link-iam-subject` 是运维入口；
- 不按用户名或显示名自动合并历史身份。

## 5. 统一入口与故障行为

页面通过 `PUBLIC_BASE_PATH=/money` 生成统一入口链接。Gateway 为 Contract 和 Money Flow 分别设置 Route Prefix 限定的 HttpOnly Cookie；一个应用凭证不会发送给另一路径，也不能被另一个应用接受。

Gateway 注销允许 IAM 双提交 CSRF，或在目标应用表单无法读取 IAM CSRF 值时使用“已有 IAM CSRF Cookie＋严格同源 Origin”校验；跨源 Origin 被拒绝。注销清除 Suite Session 及两张应用凭证。

- Contract 完全停止时，Money Flow 页面仍返回 200、已写数据仍可读；Contract 路由明确返回 503；
- Money Flow 停止不影响 Contract 的对称证据已在 M7-004 取得；
- 直接访问 Money Flow 内部端口且没有有效凭证返回 401；
- local 模式无需 IAM、Gateway 或 Contract 即可构建、测试和启动。

## 6. 已验证结果

- 单元测试覆盖签名验证、错误 Audience、Capability 映射、IAM Subject Actor、IAM 配置、本地 bootstrap 禁止、缺凭证 401、只读写入 403、前缀链接和按动作隐藏表单；
- PostgreSQL 18.6 migration 测试从历史版本升级到 `000016_local_user_iam_subject_links.sql`，验证显式映射不可变；全量历史 migration 测试已更新并通过；
- 实际 Money Flow 开发库从 `000015` 显式升级到 `000016`，随后以 IAM／`verify` 模式启动；
- 同一个 IAM Suite Session 分别换取 Contract 和 Money Flow 单 Audience Credential，两个应用均经 Nginx／Gateway 返回 200；
- 实际创建“`M7 IAM 收支验收主体-*`”并回读，数据库审计记录真实 IAM Subject UUID 和 `actor_kind=iam_subject`；
- 交叉投递两张凭证均返回 401；Contract 完全停止时 Money Flow 继续返回 200，Contract 路由返回 503；
- 从 Money Flow 页面提交注销时，同源 Origin 校验通过，IAM Session 随即返回 401，Suite Session 与两张应用 Cookie 均被清除；重新登录成功。

M7-005 不建立 Contract → Money Flow 连接器，也不共享身份表。后续 M7-006 已只收口当前组件的 D1 组合交付，没有改变该边界。
