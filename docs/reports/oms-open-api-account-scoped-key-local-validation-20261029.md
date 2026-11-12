# OMS 开放 API 账号凭证本地验证

日期：2026-10-29
状态：本地通过，未部署

## 问题与结果

问题：API Key 创建把 IAM 用户必须已关联 People Person 当成统一前置条件，导致一个已经能正常登录、属于当前组织且持有开放 API Grant 的账号仍无法创建 Key；同时 People 的正常 `person_not_found` 被误报为可重试服务故障。

结果：Key 改为 Tenant＋Subject 账号凭证。创建、列表、撤销、`/me`、settings 和 Evidence 上传不解析 Person；只有 `open-api.reimbursement.*` 在调用时解析当前 active Person。缺少关联返回 `403 PERSON_MAPPING_REQUIRED`，People 真故障仍返回可重试认证依赖错误。

## 实现边界

- IAM 继续独占 Key、一次 Secret、不可逆 verifier、期限、上限、撤销、Grant 复验与调用审计。
- People & Groups 继续独占 Person 及可选 Subject 关联；成员管理提供当前修复入口。
- Gateway 只交换短期 Capability，不保存账号或 Person。Evidence Credential 不需要 Person；Internal Finance Credential 包含调用时解析的当前 Person 引用。
- migration `000011_account_scoped_api_keys.sql` 将历史 `person_id／person_revision` 和 usage `person_id` 改为可空；旧数据不重写，新 Key 不再冻结 Person。
- 个人设置或邀请时关联可以以后增加，但本轮不引入跨 Owner 邀请事务，也不把它们变成登录或 Key 创建条件。

## 已执行验证

```text
cd backend/platform/identity-access && go test ./...
cd backend/platform/identity-access && go vet ./...
cd backend/platform/gateway && go test ./...
cd backend/platform/gateway && go vet ./...
cd backend/services/people-groups && go test ./...
cd backend/services/people-groups && go vet ./...
cd frontend/oms-workbench && go test ./...
cd frontend/oms-workbench && go vet ./...
node frontend/oms-workbench/internal/server/oms_phase1_open_api.test.mjs
node frontend/oms-workbench/internal/server/oms_phase1_member_profile.test.mjs
jq empty backend/platform/gateway/internal/gateway/openapi.v1.json
acceptance/p9-full-composition/test-c5-api-keys
```

结果：全部通过。Open API 静态回归 11 项，成员档案回归 4 项；P9 C5 离线连续验收覆盖账号 Key 首次创建、恢复复用、双租户隔离、`/me` 的 Tenant／Subject／Key 绑定、通用调用无 Person、Secret 不进入证据和证据防篡改。

另启动一次性 `postgres:18.6-alpine`，从空库重放全部 Identity migration，并执行：

```text
go test -count=1 -tags=integration ./internal/apikey ./internal/httpserver
```

结果：两个包通过；临时数据库容器在测试结束后删除。集成覆盖账号 Key 创建不调用 People、通用调用不依赖 Person、报销缺关联 403、关联后短期 Owner Credential、旧 Key 兼容、审计可空 Person、上限／撤销／撤权、migration 11 与完整 IAM HTTP 生命周期。

## 未做事项

- 未构建或发布 D1 镜像。
- 未执行公网登录、真实 Key 创建或报销业务写入。
- 未改变现有账号、成员、Grant、Key 或业务数据。
