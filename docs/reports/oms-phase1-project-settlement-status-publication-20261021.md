# OMS 一期项目结清状态公网发布验证（2026-10-21）

## 结论

OMS 项目列表已在 `https://oatest.antuinfo.cn/` 收敛为“应收状态／应付状态”，不再展示应收、未收、应付、未付金额。正常状态只显示“已完成／未完成”，既有超额收付或负余额继续显示“金额异常”。精确金额仍由项目详情及应收／应付专业模块展示。

本次只替换 Unified Workbench；Owner、API、数据库、schema、Grant 和业务数据均未改变。

## 发布制品

- 源码：`main@acce0301cc7ffaff9bae29d214574c7d0eeef07b`
- 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.20@sha256:f78176522965d41a6858ddf6bda7ea9f97332b0dca2b2fece9fe502760f0efc5`
- 平台：`linux/amd64`
- 静态资源版本：`20261021.1`
- 候选环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.20.https.env`
- 升级前备份：`/var/backups/omsbizfoundry/20261021T112852Z`

## 发布前证据

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js` 通过。
- Workbench Node 测试 235/235 通过。
- `GOWORK=off go test -count=1 ./...` 在 `frontend/oms-workbench` 通过。
- `git diff --check` 通过。
- v1.19 线上 D1 `check` 通过，17/17 长运行容器 healthy。
- v1.20 候选预检完成镜像准备并返回 Identity & Access 契约 `contracts-valid`。

## 发布动作

从 v1.19 环境文件复制 v1.20 候选，只替换 `D1_WORKBENCH_IMAGE`。完整备份成功后，仅重建 `omsbizfoundry-unified-workbench-1`，其他 16 个长运行组件没有被替换。

## 发布后证据

- Workbench 运行 v1.20 镜像并为 healthy。
- D1 全栈 `check` 通过；17/17 长运行容器 healthy。
- 公网 JavaScript 已包含 `projectMoneyStatusCell` 与“已完成／未完成／金额异常”，CSS 已包含 `oms-v1__project-status`；JavaScript SHA-256 为 `352b1287e0d4ffdede36634e6d8bf399c3d4871399a9e356e78d362146f8b63f`。
- 公网正式登录成功，HTML 明确引用静态资源 `20261021.1`；Gateway 对 Party、Project、Billing、Money、Contract 五个应用授权成功。
- Owner 只读复验：Party 7、Project 28、Billing 2、Money 3、Contract workspace 6。
- 本轮公网验收业务写入为 0。

## 边界

该发布只减少项目列表的信息量。应收、应付、回款和支付的精确金额、计算与异常事实仍来自既有 Owner；本轮没有建立新的结清事实，也没有改变金额精度、核销规则或任何历史数据。
