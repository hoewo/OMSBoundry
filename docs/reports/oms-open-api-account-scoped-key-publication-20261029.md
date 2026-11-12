# OMS 开放 API 账号凭证公网发布记录
日期：2026-10-29
状态：公网 D1 已发布并通过运行检查

## 发布结果

API Key 已从“必须先关联员工的员工凭证”改为 Tenant＋Subject 组织账号凭证。正常登录、属于当前组织且持有开放 API Grant 的账号可以创建、列出和撤销自己的 Key；`/me`、settings 与 Evidence 上传不再解析 People Person。只有本人报销等明确依赖员工身份的动作才在调用时解析当前 active Person，缺少关联稳定返回 `403 PERSON_MAPPING_REQUIRED`。

本次只替换 Identity Access、Gateway 与 Unified Workbench；其他 16 个长运行组件继续使用发布前固定 digest。未改变 Tenant、账号、Grant、成员、既有 Key 或业务事实。

## 发布镜像

- Identity Access：`v0.1.0-test.oms-v1.60@sha256:4a4137048e22f24f7c630d838d954055a1bd13554f8b86d9ba14537832eaff3b`
- Gateway：`v0.1.0-test.oms-v1.60@sha256:e099f14e82f925d26d3caf135eed7ab75586c8cec6390c501262d8f70496f835`
- Unified Workbench：`v0.1.0-test.oms-v1.60@sha256:f24be392afec5b7e1e3b8d1405bd4eb61537e8b18dc6e23b218cb8eeb4f96a4a`

三个 OCI index 均实际包含 `linux/amd64` 镜像和 provenance attestation；线上容器的 `Config.Image` 与运行 image ID 均精确匹配上述 digest。

## 升级与恢复

- 候选环境由 `v1.59` 环境文件复制，只变更 `D1_IDENTITY_IMAGE`、`D1_GATEWAY_IMAGE`、`D1_WORKBENCH_IMAGE` 三项。
- `preflight-upgrade` 通过，候选 IAM capability／policy contract 解析结果为 `contracts-valid`。
- 升级前自动备份：`/var/backups/omsbizfoundry/20261029T153931Z`。
- D1 upgrade 成功；Identity 报告 `000011_account_scoped_api_keys.sql` 为 current。
- 升级后独立执行 D1 `check` 通过；19/19 正式长运行容器均为 healthy。
- 数据库只读复核：`api_keys.person_id`、`api_keys.person_revision`、`open_api_usage_records.person_id` 均为 nullable，旧记录不被重写。

## 公网复验

- `https://oatest.antuinfo.cn/open-api/v1/openapi.json` 返回 `200`，标题为 `OMS1 Organization Account API`。
- `/me` 文档只返回 Tenant、Subject 与 Key 引用；全部报销动作声明 `PersonMappingRequired` 403。
- `https://oatest.antuinfo.cn/workbench/assets/oms-phase1-open-api.js?v=20261029.1` 返回 `200`，SHA-256 为 `f1583528d2a846414587c8feb7926136351ae991f26184f9f84aea0f868b5d36`，与本地发布源完全一致。
- 正式登录端点返回有效 Subject Session。部署验证未创建测试 API Key，也未执行报销或其他业务写入；账号级 Key 创建与缺 Person 报销拒绝已由发布前 PostgreSQL 18.6 集成测试覆盖。

## 边界

- Key 仍服从当前账号状态、Grant、期限、数量上限、不可逆撤销、HTTPS、Gateway 限流和 Owner 最终授权。
- Person 关联仍由 People & Groups 拥有；成员管理保留可选账号关联入口。
- 本次发布不实现邀请时自动关联、个人设置关联或新的通用身份平台。
