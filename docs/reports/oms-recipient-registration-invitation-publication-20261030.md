# 受邀人自助注册邀请公网发布记录

日期：2026-10-30
状态：已发布并完成只读公网复验

## 问题 A → 结果 B

管理员必须代填别人的姓名和手机号／邮箱，容易录错并混淆账号所有权 → 管理员只确定组织内用户级别并生成一次性链接，受邀人打开链接后自行选择手机号或邮箱、填写本人姓名和密码，完成后即可登录。

## 发布范围

- Identity Access 新增无个人账号邀请创建、邀请预览和受邀人注册／既有账号验证加入。
- Unified Workbench 管理端改为“创建邀请链接”，只选择普通用户或管理员；登录页承接邀请预览、手机号／邮箱选择、姓名、密码和完成状态。
- 新账号在同一事务内完成账号与当前 Tenant Subject 注册；既有账号必须验证原密码，只加入当前组织且不改密。
- Person 关联不进入 IAM 邀请；注册完成后仍由本组织管理员通过 People 版本化端口显式处理。
- Gateway 继续使用既有通用 `/auth` 代理，因此本次没有变更 Gateway 镜像、路由所有权或业务授权边界。

## 镜像与环境

- Identity Access：`v0.1.0-test.oms-v1.66@sha256:dec6e12a620e64e564a3bd97414249d88536cbe3c93fd249278465ba766e034e`
- Unified Workbench：`v0.1.0-test.oms-v1.66@sha256:645443cbe865a7accd42737b4562d88cc6dc1a2eafb8671fe643e2cf781f834a`
- D1 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.66.https.env`
- 旧环境与候选环境除上述两个镜像引用外内容摘要一致。

## 备份与升级

- 正式 `upgrade` 在任何容器替换前生成联合备份：`/var/backups/omsbizfoundry/20261030T142928Z`。
- `checksums.sha256` 独立复验为 `19/19 OK`。
- 发布后 D1 正式 `check` 独立退出 `0`，全部 schema、PostgreSQL、Evidence 存储和服务检查通过。
- Compose 项目 `omsbizfoundry` 长运行容器为 `19/19 healthy`。
- Identity 与 Workbench 容器的实际 image ID 分别等于上述不可变 digest，且环境文件标签均指向 v1.66。

## 公网复验

- `https://oatest.antuinfo.cn/` 返回 `303`，`/workbench/?view=oms-v1` 返回 `200`。
- 无效邀请 Token 携带有效 CSRF 调用 `/auth/api/v1/invitations/preview` 返回 `401`，证明公网 Nginx → Gateway → IAM 新路由可达且无效凭证不会泄露邀请信息。
- 管理端资源含“创建邀请链接”“邀请链接已创建”和 `/api/v1/admin/invitations`；受邀人资源含手机号／邮箱选择、`registration_invitation`、`/api/v1/invitations/preview` 与 `/api/v1/invitations/register`。
- 公网 Identity JS、Login JS、Login CSS 的 SHA-256 与本地候选逐项一致；v1.65 的旧资源号分别为 Identity `20261030.4`、Login CSS `20261022.6`、Login JS `20261026.2`，本次引用已提升为 `20261030.5`、`20261030.1`、`20261030.1`，不会命中旧 immutable 缓存。

## 数据边界

本次复验没有创建真实邀请、账号或成员关联，也没有修改 Grant、People、Owner 或业务数据。一次完整的真实收件人注册会产生 IAM 身份事实，应由管理员在线上按实际需要发出邀请后由收件人本人完成，不用测试脚本伪造。

## 关联材料

- [规格](../specs/identity/oms-v1-account-onboarding-v1.md)
- [本地验证](./oms-recipient-registration-invitation-local-validation-20261030.md)
