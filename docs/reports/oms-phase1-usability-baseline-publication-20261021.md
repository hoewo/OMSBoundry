# OMS 一期易用性基线公网发布验证（2026-10-21）

## 结论

2026-10-20 确认并进入 `main@99ead97` 的 OMS 易用性改动已发布到 `https://oatest.antuinfo.cn/`。本次只替换 Unified Workbench；独立的 `codex/oms-interaction-patterns` 试验分支没有发布，Owner、API、数据库、schema、Grant 和业务数据均未改变。

## 发布制品

- 源码：`main@99ead9715cfd405bc39d0dd5d86cd0c468b13fad`
- 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.19@sha256:6268306eadb7183ddae1da1229e623f5fab3d6fb0c033244b289d24a4629155c`
- 平台：`linux/amd64`
- 静态资源版本：`20261020.5`
- 候选环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.19.https.env`
- 升级前备份：`/var/backups/omsbizfoundry/20261021T111655Z`

## 发布前证据

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js` 通过。
- Workbench Node 测试 235/235 通过。
- `GOWORK=off go test -count=1 ./...` 在 `frontend/oms-workbench` 通过。
- `git diff --check` 通过。
- v1.18 线上基线 D1 `check` 通过，17 个长运行容器均为 healthy。
- v1.19 候选镜像摘要和 `linux/amd64` 平台校验通过；候选预检完成镜像准备并返回 `identity-access` 契约 `contracts-valid`。

## 发布动作

从 v1.18 环境文件复制 v1.19 候选，只替换 `D1_WORKBENCH_IMAGE`。完整备份成功后，仅重建 `omsbizfoundry-unified-workbench-1`，没有重启其他 16 个长运行组件。

## 发布后证据

- Workbench 容器运行镜像与候选摘要一致，状态为 running／healthy。
- D1 全栈 `check` 通过；17/17 长运行容器 healthy，无非健康容器。
- 公网根入口返回成功，静态资源版本为 `20261020.5`。
- 正式 IAM 登录成功；Gateway 对 Party、Project、Billing、Money、Contract 五个应用的授权成功。
- Owner 只读复验：Party 7、Project 28、Billing 2、Money 3、Contract workspace 6。
- 本轮公网验收业务写入为 0。

## 边界

该证据证明 `main@99ead97` 的 Workbench 易用性基线已经上线，不证明独立交互试验分支已合入或发布，也不扩大 OMS 一期的业务范围和事实所有权。
