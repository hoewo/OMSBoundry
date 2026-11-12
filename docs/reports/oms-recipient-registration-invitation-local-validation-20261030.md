# 受邀人自助注册邀请本地验证

日期：2026-10-30
状态：本地候选通过；后续公网发布见[发布记录](./oms-recipient-registration-invitation-publication-20261030.md)

## 问题 A → 结果 B

管理员必须代填别人的姓名和手机号／邮箱，既容易录错，也把账号所有权放错了位置 → 管理员只决定组织内级别并生成一次性链接，受邀人打开链接后自行选择手机号或邮箱、填写姓名和密码，完成后即可登录。

## 已实现

- `POST /api/v1/admin/invitations` 在未提供登录标识时创建 24 小时一次性注册邀请，只固定 Tenant 和管理员级别。
- `POST /api/v1/invitations/preview` 只返回组织名称、加入级别和有效期；Token 在 JSON 请求体中传递。
- `POST /api/v1/invitations/register` 接受本人姓名、手机号／邮箱和密码。
- 新登录标识原子完成全局账号和 Tenant Subject 注册；已有标识必须验证原密码后只加入当前组织，原密码不变。
- 邀请完成前使用不可登录的内部占位 Subject，保留稳定 ID、列表可见性和重签能力；页面不把内部标识显示成真实账号。
- 管理端对话框只保留用户级别；注册页提供手机号／邮箱切换、密码确认和完成状态。
- Person 关联不进入 IAM 邀请，注册后继续由管理员通过 People 版本化端口显式维护。

## 不变量与不负责

- 没有有效邀请不能注册；未开放公开注册、搜索或申请加入组织。
- Token 一次有效、默认 24 小时、只以 fragment 进入浏览器，不放入查询串、日志文案或持久页面状态。
- 既有账号验证失败不创建 Subject、不修改密码；系统管理员账号不能接受业务组织邀请。
- 不发送短信／邮件，不伪造送达状态；不自动关联 Person，不按姓名匹配成员。
- 本次没有修改 schema、Capability、业务 Owner 或公网数据。

## 实际验证

通过：

- Identity & Access `go test ./...`
- Identity & Access `go vet ./...`
- Unified Workbench `go test ./internal/server`
- Unified Workbench `go vet ./...`
- `node --check oms_phase1_identity.js`
- `node --check oms_phase1_login.js`
- `node --test oms_phase1_identity_account_link.test.mjs oms_phase1_registration_invitation.test.mjs`：7/7
- 一次性 PostgreSQL 18.6：`TEST_DATABASE_URL=... go test -count=1 -tags=integration ./internal/httpserver`

真库覆盖新手机号注册、组织／级别预览、一次性消费、登录，以及既有邮箱账号使用原密码加入第二组织并保留原登录。临时 PostgreSQL 容器已停止并随 `--rm` 删除。

## 未完成

- 未构建或发布 Identity、Gateway、Workbench 公网镜像。
- 未执行 D1 `check`、联合备份／恢复或公网浏览器真实邀请。
- 未创建、修改或发送任何线上邀请、账号、成员关联或业务事实。
