# OMS 管理员默认全权限画像本地验证

日期：2026-10-28  
范围：BF-279 / `oms-administrator-full-access-v1`  
状态：本地实现与门禁通过，未发布 D1／公网

## 问题与结果

组织用户已经显示为“管理员”，但 IAM 没有自动授予 Party、Project 等已安装应用的业务 Capability，因而“新增甲方档案”等合法操作被拒绝。本次把管理员语义落实为 IAM 内的锁定权限画像：按实际装载 Capability Catalog 逐 Audience／Action 创建 tenant-scope Grant，不使用通配符，也不让目标应用按管理员字段绕过授权。

## 实现事实

- IAM schema 10 为 tenant Grant 增加 `source_kind`，区分 `manual` 与 `administrator_profile`。
- IAM 启动、管理员登录和管理员提升都会幂等对齐 `oms-administrator-full-access-v1`；目录新增动作可在重启后补齐。
- 管理员降级在同一事务内只撤销 `administrator_profile` 来源 Grant；人工 Grant 保留。人工授予与画像已有同一动作时，来源转为 `manual`，避免随后降级误删。
- 系统管理员、普通用户和其他 Tenant 不继承管理员画像；系统管理员仍不能签发组织业务 Credential。
- Workbench 新增账号说明已改为管理员默认获得本组织当前已安装功能的全部业务权限，静态资源版本更新为 `20261028.1`。
- People 职位、部门和任职事实没有被写入 IAM，也没有直接成为授权条件。

## 实际验证

以下检查均实际运行通过：

1. `backend/platform/identity-access`: `go test ./...`。
2. `backend/platform/identity-access`: `go vet ./...`。
3. 临时 PostgreSQL 18：`TEST_DATABASE_URL=... go test -tags=integration ./...`。
   - fresh migration 和重复 migration 均通过，当前 schema 为 `000010_administrator_profile_grants.sql`；
   - 管理员首次登录后的有效画像 Grant 数与测试中实际装载 Catalog 的 Action 总数一致；
   - 管理员签发的 Party Credential 包含 `party-directory.create`；
   - 普通用户提升管理员后完整补齐画像并使旧 Session 失效；
   - 同一 Action 改为人工授权后再降级，画像 Grant 为 0，人工 Grant 保留 1 条；
   - 既有双 Tenant 隔离、系统管理员零业务凭证及目标 Audience／Scope 负例继续通过。
4. `frontend/oms-workbench`: `go test ./...`。
5. `frontend/oms-workbench`: `node --check internal/server/oms_phase1_identity.js`。
6. `frontend/oms-workbench`: `node --test internal/server/oms_phase1_dialog.test.mjs`。
7. 仓库：`git diff --check`。

测试使用的临时容器 `oms-admin-profile-test-20261028` 已在命令退出时删除；本轮未保留额外长期服务。

## 未包含

- 未部署到 `oatest.antuinfo.cn`，线上截图中的缺权状态不会在本地代码变更后自动消失。
- 未实现 People 职位目录、任职历史、职位权限模板或职位到 Capability 的映射。
- 未改变目标业务应用的 Tenant、资源、状态、revision、历史和 `allowed_actions` 最终授权。
