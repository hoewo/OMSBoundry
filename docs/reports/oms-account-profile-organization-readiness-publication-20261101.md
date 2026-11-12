# OMS 账号资料编辑与组织就绪恢复公网发布记录

- 日期：2026-11-01
- 决策：BF-302
- 状态：已发布并完成只读公网复验
- 公网入口：`https://oatest.antuinfo.cn/`

## 问题 A → 结果 B

组织管理员无法修正账号／员工姓名等资料，且登录或切换后组织信息迟迟未返回时页面没有明确恢复动作 → 管理员在账号目录中分别维护 IAM 账号姓名与 People 员工姓名／部门／岗位；组织上下文短暂失败自动重试，耗尽后失败关闭、清除旧投影并明确提示重新加载或刷新。

## 边界

- IAM Subject `display_name` 只在当前 Tenant 内更新，要求 expected revision、管理员权限、同租户目标和不可变审计；不修改登录标识、密码、其他组织 Subject 或 People Person。
- People Person 的姓名、部门和岗位继续通过既有 `person.profile` 版本化端口更新；不修改 IAM、管理员身份或历史工资快照。
- Workbench 只组合两个 Owner 的投影并分别发命令，不伪造跨库原子更新。
- Session Tenant、Tenant 详情和组织清单当前项必须一致；自动重试三次仍失败时清空浏览器旧投影、禁用组织切换，并提供“重新加载组织信息／刷新页面”。

## 发布制品

- Identity Access：`v0.1.0-test.oms-v1.70@sha256:9611e9c9dde92969991d2b304851bcc49630b883715e6e254db300f9c094eed0`
- Unified Workbench：`v0.1.0-test.oms-v1.71@sha256:0ad1d1d9581fc5a673552ead9ed8a08fe17120f5def10e89afc98255caf9f171`
- 平台：`linux/amd64`
- D1 环境：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.71.https.env`
- 最终升级前恢复点：`/var/backups/omsbizfoundry/20261101T021911Z`

第一次候选环境从 v1.69 派生，只替换 `D1_IDENTITY_IMAGE` 与 `D1_WORKBENCH_IMAGE`；删除这两行后的配置 SHA-256 均为 `ce3b15b9584cf96fe6b0f353e466b77c6e05a575674068c5e0efb93a0d20ba73`。首次 v1.70 升级创建恢复点 `/var/backups/omsbizfoundry/20261101T021107Z`。随后发现 HTML 仍引用旧的 Identity JS 不可变缓存参数，故只构建 Workbench v1.71，把资源版本更新为 `20261101.1`；v1.70 与 v1.71 环境排除 Workbench 镜像行后的配置 SHA-256 均为 `6a31e74e8b56a70d7c018a5b708b36cf159a453f0fa320c97257116ebeb81ddd`，未改变其他服务或配置。

## 发布与验证

- 发布前 Identity／People／Workbench Go 回归、Identity `go vet`、前端定向 Node 16/16、JavaScript 语法及 `git diff --check` 通过。
- 临时 PostgreSQL 18 集成覆盖同租户改名、当前 Session 复读、过期 revision 冲突、跨租户拒绝和单条 IAM 审计；临时数据库已移除。
- 候选 IAM 契约预检返回 `contracts-valid`。
- v1.70 与缓存修正后的 v1.71 两次 D1 `upgrade` 均退出 0；Identity schema 保持 `000012_multi_organization_accounts.sql`，没有新增 migration。
- 恢复点 `checksums.sha256` 的 19 个条目全部为 `OK`。
- 使用同一候选环境独立执行 D1 `check` 退出 0；Compose 项目 19/19 长运行容器 healthy，无 unhealthy。
- 最终运行中 Identity 与 Workbench image ID 精确等于上述 Registry 摘要。

## 公网复验

- `/` 返回 303 到 `/workbench/?view=oms-v1`，工作台返回 200。
- 公网 `oms-phase1-identity.js?v=20261101.1` 与本地候选 SHA-256 同为 `51c7be4b327f2f9de03e074c393f0c522aeb6592912fc3a9dfc78763f702f0d1`；运行中 Workbench 二进制也包含该版本化引用。资源包含“编辑账号姓名”“编辑成员资料”“组织信息未就绪”、三次有界重试和两个恢复动作，旧的不可变缓存不会继续命中新实现。
- 新增 `/auth/api/v1/admin/subjects/{subjectID}/profile` 在未登录调用时返回 401，证明公网 Nginx → Gateway → Identity 新路由可达且受鉴权保护。
- 本轮没有在线上执行真实改名、成员资料编辑、权限调整或业务写入；写路径的权限、revision、审计和租户隔离由发布前真实 PostgreSQL 18 集成覆盖。

## 未改变事项

本次不修改登录账号、密码、Person 关联、Capability、Grant、工资历史、其他业务 Owner、schema 或业务数据，也不把 IAM Subject 与 People Person 合并为一个事实对象。
