# OMS 一期 `/mvp-review` 工作台交互公网发布验证（2026-10-22）

## 结论

BF-167／BF-168 已作为 OMS 一期后续界面基线发布到 `https://oatest.antuinfo.cn/`。当前工作台采用统一的侧栏、面包屑、紧凑标题、摘要、单行表格、排序、分页、就地筛选和条件导出；桌面项目详情不覆盖主菜单。项目经营、应收回款和应付支付可同时展开，分页条数控件的标签与箭头保持在同一边框内。

本次只替换 Unified Workbench；Owner、API、数据库、schema、Grant、精确金额和业务数据均未改变。

## 发布制品

- 源码：`main@32d959761a94ae1e80d8bf33212d3d0b780642ab`
- 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.21@sha256:6dcba5542c68b0c7970c80cee4282fc3560ece7efab197a87ac0e9088c65e423`
- 平台：`linux/amd64`
- 静态资源版本：`20261022.1`
- 候选环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.21.https.env`
- 升级前备份：`/var/backups/omsbizfoundry/20261022T021316Z`

## 发布前证据

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js` 通过。
- Workbench Node 测试 235/235 通过。
- `GOWORK=off go test -count=1 ./...` 与 `go vet` 在 `frontend/oms-workbench` 通过。
- `git diff --check` 通过。
- 本地桌面浏览器确认三个业务组可同时保持展开；分页触发器显示“20 条/页”，箭头右边界位于触发器边界以内。
- v1.20 线上 D1 `check` 通过，17/17 长运行容器 healthy。
- v1.21 候选镜像按不可变摘要拉取，平台为 `linux/amd64`；升级预检返回 Identity & Access `contracts-valid`。

## 发布动作

从 v1.20 环境复制 v1.21 候选，只替换 `D1_WORKBENCH_IMAGE`。完整备份成功后，仅使用现有 D1 Compose 装配重建 `omsbizfoundry-unified-workbench-1`；容器 ID 对比确认其余 16 个长运行组件没有被替换。

## 发布后证据

- Workbench 运行 v1.21 不可变镜像，状态为 running／healthy。
- D1 全栈 `check` 通过；17/17 长运行容器 healthy。
- 公网根入口仍以 303 进入 `/workbench/?view=oms-v1`。
- 公网正式登录成功，HTML 包含中性 `data-operating-party-id` 和静态资源 `20261022.1`；页面 SHA-256 为 `a9fd8b254d716d1deeeecc86b869eb39721f84d044a549f3ff73dc7239064e0b`。
- Gateway 对 Party、Project、Billing、Money、Contract 五个应用授权成功。
- Owner 只读复验：Party 7、Project 28、Billing 2、Money 3、Contract workspace 6。
- 本轮公网验收业务写入为 0。

## 访问入口纠正

公网域名当前解析到 `47.98.156.166`，部署账号是 `ecs-user`，SSH 使用 22 端口和本机专用 `omsbizfoundry_auth58_ed25519` 密钥。先前 `39.96.183.252:2222` 是过期的本机 SSH 配置目标；本次部署没有依赖阿里云 AccessKey，也没有降低安全组范围。

## 边界

该发布确认的是 Workbench 交互、不可变制品和既有 Owner 只读装配。它没有新增业务字段、事实 Owner、自动审批、期限／逾期规则或内部成本，也没有执行任何业务写入。公网自动浏览器的重页面 DOM 快照未形成额外视觉证据，因此侧栏与分页像素级证据继续引用同一源码／镜像的本地桌面验收；线上身份、HTML、静态资源、Gateway、Owner 和运行态证据已经闭合。
