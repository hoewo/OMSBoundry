# 多组织登录账号与租户身份 V1
日期：2026-10-30
状态：BF-294 已确认、实现并发布；公网 Identity schema 12／v1.63

## 问题与结果

> 手机号／邮箱既是全局登录凭据，又被当成单一组织 Subject → 一个账号只能加入一个组织，创建第二个组织时报“已被使用”。

结果：密码、锁定和登录标识由全局 `login_accounts` 保存一次；每加入一个组织就建立该组织自己的 IAM Subject。账号可以加入多个组织，并在登录后切换；Subject、管理员级别、状态、Capability、Session、审计和 People 成员关联均按组织隔离。

## 稳定对象与不变量

1. `login_accounts` 是全局认证账号，规范化手机号／邮箱／兼容用户名全局唯一，只保存一套密码摘要、失败锁定和密码更新时间。
2. `subjects` 是账号在一个 Tenant 内的安全主体。`(tenant_id, account_id)` 唯一；同一账号在不同 Tenant 使用不同稳定 Subject ID。
3. 组织内的启用／停用、管理员级别、Capability Grant 和审计只作用于该组织 Subject；不得把另一个组织的权限复制或带入。
   Subject 显示名同样是组织内资料：管理员可带 expected revision 修改当前组织姓名，但不得改写全局登录标识或其他组织 Subject。
4. 新登录账号仍走 24 小时一次性激活；已激活账号加入其他组织时沿用原密码，不再生成激活链接。
5. 系统管理员账号不能加入业务组织，也不能获得组织业务 Capability。
6. 切换组织必须旋转 Suite Session；Gateway 同时清除旧组织的全部 Audience Capability Cookie，页面刷新后再按新 Subject／Tenant 申请权限。
7. 密码重置属于全局账号，更新所有组织 Subject 的兼容密码快照并使其既有 Session Generation 失效。

## 成员关联

- IAM Subject 与 People Person 仍是两个对象。
- People 的 `persons_iam_subject_unique (tenant_id, iam_subject_id)` 是正式约束：同一组织内，一个 Subject 最多关联一个 Person。
- 不同组织使用各自 Subject，因此同一登录账号可以在组织 A 关联 Person A，在组织 B 关联 Person B。
- 关联仍通过 People 的版本化端口验证当前 Tenant、Subject 状态和 revision；IAM 不保存 Person ID，也不按姓名自动匹配。
- 修改 IAM Subject 显示名不会同步 People Person；员工姓名、部门和岗位由 People `person.profile` 单独维护。

## HTTP 与交互

- `GET /api/v1/organizations`：列出当前账号的组织身份，只返回当前账号可见的组织。
- `POST /api/v1/session/organization`：以 `tenant_id` 选择已启用组织，原子撤销旧 Session 并签发新 Session。
- `POST /api/v1/admin/subjects/{subjectID}/profile`：管理员以 expected revision 修改当前 Tenant Subject 显示名并写 IAM 审计。
- 系统管理员创建组织或设置管理员时，已存在账号返回 `action=joined`；新账号返回 `action=invited` 和一次性激活凭据。
- BF-297 起，组织管理员只创建带级别的邀请链接，不预先输入账号。受邀人选择手机号／邮箱；若该标识已有全局账号，必须用原密码验证后复用账号并建立当前组织 Subject，原密码不变。成员关联在注册后另行显式处理。

## 不负责

- 不提供公开搜索或自行申请加入组织；组织身份仍由系统管理员或组织管理员建立。
- 不自动合并不同登录标识，不支持一个账号绑定多个手机号／邮箱。
- 不把 People 任职、职位、部门或 Group 变成登录账号字段。
- 本地完成不代表公网 schema 12、Identity、Gateway 或 Workbench 已发布。

## 验收

1. 已激活账号可成为第二个新组织的首位管理员，不产生第二套密码或激活链接。
2. 同一账号列出两个组织，并在切换后得到不同 Subject ID、正确 Tenant ID 和对应管理员级别。
3. 切换后旧 Session 失效，Gateway 清空旧 Tenant Capability Cookie；跨 Tenant Subject、Grant 和业务资源仍拒绝。
4. 新账号邀请／激活、密码重置、停用、最后管理员保护和系统管理员隔离保持通过。
5. 同一 Tenant 重复关联同一 IAM Subject 到第二个 Person 失败；不同 Tenant 的各自 Subject 可分别关联 Person。
6. schema 12 从 schema 11 迁移及空库迁移都通过 PostgreSQL 18 集成测试，migration 重放保持幂等。
7. Session Tenant、Tenant 详情和组织清单当前项不一致时，Workbench 有界重试后失败关闭，不显示旧组织，并提供手动重载与整页刷新。
