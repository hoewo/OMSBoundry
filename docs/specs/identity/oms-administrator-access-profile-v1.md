# OMS 管理员默认全权限画像 v1

日期：2026-10-27  
状态：已确认，本地实现与门禁通过，未发布 D1／公网

## 问题与结果

问题 A：组织用户已经显示为“管理员”，但没有同步获得 Party、Project 等已安装业务应用的正式 Capability，导致新增主体和创建项目被拒绝。  
结果 B：每名启用中的组织管理员都通过显式、可审计、可恢复的 IAM 权限画像获得当前 Tenant 已安装能力目录的完整 tenant-scope Grant；系统管理员和普通用户不继承该画像。

## 稳定规则

1. 产品语义是“管理员默认拥有本组织当前已安装功能的全部业务权限”。技术实现仍使用逐 Audience／Action 的正式 Grant，不使用通配符，也不在目标应用中增加 `if administrator then allow` 绕过。
2. 画像稳定 ID 为 `oms-administrator-full-access-v1`。IAM 启动、管理员登录、管理员提升及 Capability Manifest 更新后的重启都会执行幂等目录对齐。
3. 画像只展开 IAM 实际装载的 Capability Catalog；未安装、未启用或未发布 Manifest 的能力不能被凭空授予。
4. 系统管理员是平台身份，只管理组织与平台账号，不获得组织业务 Capability。
5. 管理员降级时只撤销来源为 `administrator_profile` 的 Grant；人工授予的 Grant 保留。管理员画像遇到既有人工 Grant 时不覆盖其来源。
6. 管理员全权限仍受 Tenant、资源类型、业务状态、`expected_revision`、历史不可变和目标应用 `allowed_actions` 的最终约束。
7. 管理员画像对齐失败时 IAM 启动或管理员登录失败关闭；已写入的单项 Grant 可由同一幂等对齐重试恢复。

## 与 People 职位的关系

People Person 的“总经理”“财务主管”等职位是业务任职事实，不是 IAM 用户级别，也不直接成为 Capability。后续职位目录、任职历史和职位到权限模板的显式映射另行实现；本 P0 不把职位文本、部门或 Group Membership 自动转换为 Grant。

## 验收

- 存量启用管理员在 IAM 启动后得到当前 Catalog 全部 tenant Grant；普通用户与系统管理员零新增业务 Grant。
- 管理员能取得 `party-directory.create`、`project-workspace.create` 等 Credential，并通过既有目标应用校验执行合法命令。
- 管理员降级只撤销画像来源 Grant，人工 Grant 保持；再次提升可恢复完整画像。
- 新增 Manifest 后重启重新对齐缺项，不重复制造有效 Grant 或业务事实。
- 所有新增、恢复和撤销都有不可变 IAM 审计；跨 Tenant、错误 Audience／Action 和目标业务状态仍拒绝。

本地验证记录见 [`../../reports/oms-administrator-default-access-profile-validation-20261028.md`](../../reports/oms-administrator-default-access-profile-validation-20261028.md)。
