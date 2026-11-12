# OMS Open API 现场测试 CLI 验证记录

- 日期：2026-10-30
- 事项：BF-291
- 结论：本地 CLI、公网安全冒烟与公网真实报销提交链均通过；当前账号显式关联既有测试成员后，CLI 已完成 Evidence 上传、报销创建、提交、详情与本人列表回读，线上财务管理同步显示一条 ¥1.00 的“待支付”测试报销。

## 目标

为开放 API 提供可重复、不会把 Secret 固化到仓库或证据中的测试入口，并明确区分“Key 无效”和“仅员工业务缺 Person 关联”。

## 实现

- 新增 `acceptance/open-api-live/oms-open-api-test`。
- `check` 只读验证 Key、profile、settings 和员工业务准入。
- 默认 `smoke` 在只读检查后创建 Evidence 上传会话并立即以 `client_shutdown` 终止；不上传字节，不创建报销。
- `draft`／`submit` 才进行真实 Evidence 与报销写入，分别要求显式业务写确认；`submit` 还要求独立提交确认。
- API Key 与完整收款账号只接受受限文件或进程环境变量，不接受命令行明文；认证配置、签名 URL、完整账号和请求正文只进入权限为 0700 的临时目录并在退出时清理。
- 修正本地 OpenAPI 3.1 契约：上传会话终止请求必须包含 `reason`，枚举为 `user_cancelled`、`transfer_failed`、`client_shutdown`。运行实现原本已经校验该字段，本次只补齐规范及契约测试。

## 实际验证

本地短生命周期回环服务验证：

```sh
sh -n acceptance/open-api-live/oms-open-api-test
node acceptance/open-api-live/oms-open-api-test.test.mjs
(cd backend/platform/gateway && GOWORK=off go test ./internal/gateway)
```

结果：

- CLI shell 语法通过；
- 回环测试覆盖缺 Person 映射、安全 abort、完整 upload／draft／submit、写入确认门禁，以及输出不泄露 Key、完整账号和签名 URL；
- Gateway 内部测试通过，并锁定 abort `reason` 请求契约。

公网 `https://oatest.antuinfo.cn/open-api/v1` 使用用户现场提供、仅保存在当前进程内的 Key 执行默认 `smoke`：

- Key context：200；
- `/me`：200；
- `/settings`：200；
- `/reimbursements?limit=1`：403 `PERSON_MAPPING_REQUIRED`，与 BF-290 的按业务 Person 校验一致；
- 创建 Evidence 上传会话：201；
- 携带 `{"reason":"client_shutdown"}` 终止会话：204；
- CLI 最终结果：`pass`，`credential=valid`，`person_mapping=missing`，`business_write=false`，`evidence_session=created_and_aborted`。

随后按用户要求继续完整业务链：

- 通过正式 People & Groups API 将当前账号显式关联到既有测试成员“员工2”；关联结果 revision 2；
- 再运行 `check --expect-person active`，结果为 `credential=valid`、`person_mapping=active`、`business_write=false`；
- 使用显式的 `--confirm-business-write --confirm-submit` 运行 `submit`，生成一份最小测试 PDF 并完成 Evidence create → upload → complete，最终 Evidence 状态为 `available`；
- 创建标题为“开放 API CLI 测试报销”的 ¥1.00 报销，提交后状态为 `payable`；
- CLI 随后通过报销详情和本人列表回读确认该记录存在，最终结果为 `pass`、`business_write=true`；
- 重新加载公网 Workbench 后，财务管理显示 1 条对应记录：申请人／收款方“员工2”、金额 ¥1.00、状态“待支付”、附件 1 份；详情与 CLI 结果一致；
- 未点击“确认付款”，没有登记任何实际资金支付。

固定报告不记录 API Key、Authorization、Key ID、Subject／Tenant ID、签名上传 URL、上传会话 ID、完整账号或请求正文。

## 边界

- 本次只证明已申请 Key 对当前组织的报销纵向链能够完成：组织账号识别、按业务 Person 门禁、Evidence 上传、报销创建／提交、本人回读和 Workbench 投影一致。
- 账号仅在确实使用员工报销业务时关联 Person；Key 的申请与通用组织级 API 不以前置 Person 关联为条件。
- 留下一条真实测试数据“开放 API CLI 测试报销”（¥1.00，待支付）。测试未登记付款；如需撤回或删除，应另行按正式业务规则操作。
- 本次没有部署新 CLI 或修订后的 OpenAPI 规范；公网业务服务本身未改动。
- API Key 仍是有效持久凭证；本轮只在受限临时文件中使用，测试结束后应按需要在开放平台撤销或轮换。
