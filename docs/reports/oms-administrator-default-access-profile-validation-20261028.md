# OMS 管理员默认全权限画像验证与发布记录

日期：2026-10-28  
范围：BF-279 / `oms-administrator-full-access-v1`  
状态：本地门禁、D1 公网发布与管理员浏览器动作复验通过

## 问题与结果

组织用户已经显示为“管理员”，但 IAM 没有自动授予 Party、Project 等已安装应用的业务 Capability，因而“新增甲方档案”等合法操作被拒绝。本次把管理员语义落实为 IAM 内的锁定权限画像：按实际装载 Capability Catalog 逐 Audience／Action 创建 tenant-scope Grant，不使用通配符，也不让目标应用按管理员字段绕过授权。

## 实现事实

- IAM schema 10 为 tenant Grant 增加 `source_kind`，区分 `manual` 与 `administrator_profile`。
- IAM 启动、管理员登录和管理员提升都会幂等对齐 `oms-administrator-full-access-v1`；目录新增动作可在重启后补齐。
- 管理员降级在同一事务内只撤销 `administrator_profile` 来源 Grant；人工 Grant 保留。人工授予与画像已有同一动作时，来源转为 `manual`，避免随后降级误删。
- 系统管理员、普通用户和其他 Tenant 不继承管理员画像；系统管理员仍不能签发组织业务 Credential。
- Workbench 新增账号说明已改为管理员默认获得本组织当前已安装功能的全部业务权限，静态资源版本更新为 `20261028.1`。
- People 职位、部门和任职事实没有被写入 IAM，也没有直接成为授权条件。
- 公网复验进一步发现部分 OMS Owner 仍运行早于多租户改造的旧镜像：即使 IAM 已为新 Tenant 管理员签发正确 Credential，旧 Owner 仍按早期固定 Tenant 拒绝。Party、Contract、Money、Billing、Evidence 与 Project 已升级到当前多租户校验实现；目标应用仍验证签名、Audience、Tenant、Action、Scope 和资源不变量，没有增加管理员旁路。

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

## D1 公网发布

2026-10-28 已发布到 `https://oatest.antuinfo.cn/`：

- Identity Access 使用 `v0.1.0-test.oms-v1.44`，Unified Workbench 使用 `v0.1.0-test.oms-v1.45`；组织管理员画像已为目标 Tenant 管理员形成 176 条当前 Catalog 精确 Grant。
- Party Directory 使用诊断及多租户修订镜像 `v0.1.0-test.oms-v1.46@sha256:678995d36416a2138063f8e509d10c7a34b23e8185dc6d2c1abc7ccbc049109a`。
- Contract、Money、Billing、Evidence 使用 `v0.1.0-test.oms-v1.47`，镜像摘要依次为 `sha256:45c2d1d680ddde01ab7551c4c5c1e3279af88a370f26ea212c0be3a906548ba5`、`sha256:cee1f1ec483524e578958041308f8fa2efb4e6a2ed63fb3fcc1414c0acb61778`、`sha256:280f9d243b7689eaaf25071a941599b0fd072acaecca2c4e7ff0712eed69b4af`、`sha256:cb3cdbe78355644a05c525aeeba311c697ce40d9f5b00fb01fe97560bcaedbf2`。
- Project Workspace 使用 `v0.1.0-test.oms-v1.48@sha256:34937774961f4b4efabe9bf5668ed3b72c3363fb5d9312fe5ac259e966fd4464`；镜像内 `server`、`ops`、`workload-keyctl` 与本地 Linux AMD64 构建摘要一致。该升级补齐动态 Tenant Credential 校验，未改变 Project schema（仍为 `000014_project_party_candidates.sql`）。
- 升级前联合备份位于 `/var/backups/omsbizfoundry/20261027T163251Z`；Evidence migration 3／4 和 Money migration 25／26 已执行，Billing／Contract 已处于当前 schema。迁移没有清空或伪造业务数据。
- 2026-10-28 再次发布前生成并校验联合备份 `20261028T110509Z` 与 `20261028T110851Z`。第一次切换暴露出旧 `v44` 发布编排与新 Contract／People 镜像的运行契约不一致，迁移任务失败关闭；没有回滚或改写数据。随后生成并通过 bundle 门禁的完整 D1 `v0.1.0-test.oms-v1.48` 发布包，以同版本 Compose 重新装配 Contract／People、内部财务、工资及其余服务。
- 正式 Compose project 保持 `omsbizfoundry`，误生成的隔离测试 project 容器、网络与卷已删除；最终为 19 个正式容器运行且 0 unhealthy、0 隔离残留。
- 发布后的独立 D1 `check` 返回 exit 0；公网 Workbench 未登录入口返回 HTTP 200。
- 使用用户已登录的真实管理员浏览器会话刷新公网页面后，进入“项目经营 → 甲方管理 → 新增甲方”：旧提示“当前账号没有项目候选主体维护权限”已消失，“保存主体”按钮可用。复验只打开并取消弹窗，没有提交测试甲方或其他业务事实。
- Party 鉴权只增加不含 Token 的拒绝原因日志，便于发现 IAM／Owner 契约漂移；未记录 Credential 内容。

本轮没有掌握用户账号密码，也没有替用户提交真实甲方、施工方或项目。已验证管理员可进入新增工作面并获得合法保存动作；真实业务字段校验与新建事实仍由后续实际录入验收。

## 未包含

- 未实现 People 职位目录、任职历史、职位权限模板或职位到 Capability 的映射。
- 未改变目标业务应用的 Tenant、资源、状态、revision、历史和 `allowed_actions` 最终授权。
