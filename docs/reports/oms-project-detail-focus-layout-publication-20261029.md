# OMS 项目详情重点化排版发布记录

日期：2026-10-29
范围：BF-286、BF-287
环境：`https://oatest.antuinfo.cn/`
状态：本地候选经用户确认，Workbench 公网发布与 D1 技术验收通过

## 结果

项目详情继续使用“概览／往来单位／合同／收付款”四个标签，并在详情内部收敛信息密度：内容区使用受控宽度，往来单位与合同改为紧凑的业务清单，去掉装饰性编号、角色说明和收／付圆形徽标。概览与收付款不再把四个金额做成等权大数字，而是把“待回款／待支付”作为第一视觉重点，已回／已付、应收／应付总额和票据状态降为进度与上下文。

Workbench 继续只读取并组合 Project、Party、Contract、Billing、Money 的既有投影；本次没有改变侧栏、项目列表、Owner、API、schema、Grant、数据所有权或业务事实。

## 本地验证

实际运行并通过：

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`；
- 项目详情、成员档案与 Payroll Owner 专项 Node 回归 16/16；
- `GOMAXPROCS=2 GOWORK=off go test -count=1 -p 1 ./internal/server`（`frontend/oms-workbench`）；
- 目标文件 `git diff --check`；
- 本地浏览器预览中由用户确认该版可以发布。

金额保持整数最小货币单位与 `BigInt` 精确计算；仅显示用进度百分比在有界缩放后转换为普通数字。空数据、已结清和超额异常分别显示明确状态，不把异常余额包装成正常完成。

## D1 发布

- Unified Workbench：`v0.1.0-test.oms-v1.57@sha256:73288153191402cd94c49764d7f2208277793b0ff2660d9987730f1758fa260f`；运行平台 `linux/amd64`。
- 候选环境从 v1.56 复制，去除 `D1_WORKBENCH_IMAGE` 后两份环境文件 SHA-256 相同，确认只改变 Workbench 镜像。
- 候选镜像在目标机按不可变摘要拉取；IAM 契约预检返回 `contracts-valid`。
- 升级前联合备份：`/var/backups/omsbizfoundry/20261029T132726Z`；`checksums.sha256` 全项通过。
- D1 `upgrade` 和随后独立 `check` 均返回 exit 0；19/19 长期容器全部 `healthy`。
- 运行中的 Workbench 镜像和候选摘要完全一致，环境文件为 `/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.57.https.env`。

## 公网核对

- HTTPS 根入口返回 303 到 `/workbench/?view=oms-v1`；未登录 Workbench 返回 200 登录页。
- 公网 JS、CSS 资源版本为 `20261029.2`，并包含 `flowProgressPercent` 与 `oms-v1__flow-focus` 新布局实现。
- 本轮没有使用用户凭据执行登录，也没有提交任何业务命令；公网验收为入口、不可变资源和运行态只读核对。登录后的产品视觉以用户已确认的同源码本地候选为准。

## 回滚

如需回滚，恢复上一环境文件中的 v1.56 Workbench 摘要并按 D1 运维入口重新装配；业务事实未变。若需要完整数据级恢复，使用本次联合备份 `20261029T132726Z`。
