# OMS 账号与成员关联闭环本地验证（2026-10-30）

## 目标

账号属于组织并能通过 IAM 认证，即可登录、接受邀请、创建 API Key 和使用组织级功能；只有报销等确实依赖员工身份的动作才要求关联 People Person。关联必须显式选择，不能按姓名自动匹配。

## 实现

- 成员新增与编辑保留可选登录账号；待激活账号也可预先关联，空值表示不关联或显式解绑。
- 账号邀请可选现有未关联在职成员。Workbench 先创建 IAM 邀请，再调用 People 版本化命令；People 失败时仍展示一次性激活链接，并标记部分完成及续办入口。
- “成员管理”增加“关联当前账号”。管理员只能从当前 Tenant 的未关联在职成员中明确选择；普通用户不能自行认领成员，页面提示联系管理员。
- 员工 API 缺关联返回 `403 PERSON_MAPPING_REQUIRED`，同时返回 `member_account_link`、`GET` 和 `/workbench/?view=oms-v1#member-account-link`。OpenAPI 与 CLI 公开同一恢复契约，CLI 输出 `repair_path`。
- API Key、Evidence 等组织账号能力继续不要求 Person；IAM 与 People 数据所有权、唯一关联、revision、同租户校验不变。

## 验证

以下检查已实际通过：

```text
node --check（4 个变更后的 Workbench 脚本）
sh -n acceptance/open-api-live/oms-open-api-test
成员／邀请／Open API 定向 Node 回归：通过
Open API CLI 回环链路回归：通过
Gateway go test ./internal/gateway：通过
People & Groups go test ./...：通过
Unified Workbench go test ./internal/server：通过
```

额外执行全部 Workbench `*.test.mjs` 时，本次相关用例均通过；套件仍包含与本次无关的既有失败，包括 Feedback 旧断言、Finance Owner 旧资源版本／字段断言，以及缺少部署脚本函数环境的 profile 用例，因此不把全量 Node 套件描述为通过。

本报告只记录本地验证；没有构建镜像、修改线上数据或部署公网。
