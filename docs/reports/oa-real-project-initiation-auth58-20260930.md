# Auth58：参考 OA 首个真实立项切片（2026-09-30）

## 目标与边界

用户澄清部署目标为现有公司 ECS **Auth58**，不是另一台服务器。本批只把参考 OA 工作台的“新建待立项项目”接到真实 Project Workspace，保留已运行的主组合、恢复演练组合及其他业务服务。公网域名／HTTPS、完整 OA 与其他静态业务页面不在本批验收范围。

## 发布与回滚

- 发布前主组合 17/17 healthy；Workbench 为 `v0.1.0-test.72@sha256:69b59db42fe2a678e74942786856cc20d1208cdb92bb20157d3a944a62004ea7`。
- 本地串行 `GOMAXPROCS=2 GOWORK=off go test -p 1 ./...`、`go vet ./...`、嵌入 JavaScript 语法与变更空白检查通过。首个 Linux/AMD64 工作镜像为 `v0.1.0-test.73@sha256:5e0a05830cb9441134807ed3bf5cceed81f2205f3a2790ff6caa48dccb746204`；浏览器实测后把“当前入口是公网 HTTPS”和“个人待办”两处误导性文案纠正，最终镜像为 `v0.1.0-test.74@sha256:61ff89daceb8d7313a994acf9388dbf0d1ecb1e4fbebad5f0652b55abeffcc96`。两版均由 Auth58 按摘要拉取。
- 两次均只运行 `docker compose -p omsbizfoundry ... up -d --no-deps unified-workbench`。v73 切换前环境保存为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui73-real-project.env`；v74 切换前环境保存为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui74-copy-correction.env`。最终 Workbench 容器 `937c89db1f87` healthy，主组合 17/17 healthy，其他 16 个主组合容器 ID 均未改变；两版切换后 `d1 check` 均退出码 0。恢复演练组合未触碰。
- 回滚到 v73 或 v72 时使用对应切换前环境文件恢复 `D1_WORKBENCH_IMAGE`，仅重建主组合 Workbench；本轮未实际触发回滚，不能宣称回滚演练通过。

## 实测链路

1. Auth58 回环 Nginx 入口的 IAM 登录返回 `303`，Workbench 返回 `200`；Gateway 为 Project 签发能力返回 `303`；Project 列表返回 `200`，账号拥有 `project-workspace.create` 与层级管理能力。
2. 使用同一真实入口创建可识别的测试项目 `OA-VERIFY-20260930-01`，Project API 返回 `201`、ID `9fef5bc3-bf40-4478-8c80-414b8edb5b17`、`revision 1`、`draft`；随后以独立 GET 和 Project 自有 PostgreSQL 表复读到同一 ID／编号／revision／状态。
3. 真人浏览器通过 IAM 登录，在参考 OA“项目立项”看到真实待立项列表。抽屉铺满视口，内层可滚动；点击“创建真实待立项项目”提交 `OA-VERIFY-20260930-UI` 后，列表从 10 条刷新到 11 条，显示新记录。管理员“架构透镜”显示 Workbench、Gateway、IAM、Project 成功步骤与 Project ID `42faa38f-42fc-421a-903a-45344e88cef1`。Project 自有 PostgreSQL 表再次读到该 ID、编号、`revision 1`、`draft`。从列表“进入真实 Project”能打开该 ID 的 Project 工作区并看到同一摘要和修订。v74 浏览器复核确认透镜不再冒称当前回环入口已完成公网 HTTPS；立项页明确写成“当前 IAM 授权范围”。
4. 对同一正式入口提交匿名 `POST /project/api/v1/projects` 和携带伪造 `Authorization`、`X-OMS-Subject`、`X-OMS-Administrator` 的请求，均返回 `401`；Project 自有数据库中测试编号 `OA-VERIFY-NOAUTH-20260930` 的行数为 `0`。此证据只证明无会话／伪造 Header 不可越权，不替代已登录相邻角色与撤权测试。

两条 `OA-VERIFY-*` 是**保留在 Auth58 的测试数据**，不是客户业务项目；本轮没有删除或取消它们。

## 尚未通过的门禁

- 当前入口是 Auth58 的回环 HTTP 经 SSH 隧道访问；不能把它称为公网 HTTPS 验收。
- 已完成管理员正向路径和匿名／伪造 Header 拒绝；相邻角色拒绝、Grant 撤销、IAM／Project 故障时零写入、恢复重试以及本轮备份恢复未执行。
- OA 项目管理、合同、金额、统计等页面仍有界面验证数据和未接正式写动作。本批只证明**首个立项切片**可从参考 UI 写入 Project 自有事实，不改变完整 OA 验证报告的 `insufficient-evidence` 结论。
