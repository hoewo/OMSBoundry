# OMS 报销模式切换与横向明细 v1.105 发布记录

日期：2026-11-16（GMT+8）

事项：BF-339

状态：已发布并完成无业务写入验收

## 发布结果

- 公网入口：`https://oatest.antuinfo.cn/`
- Suite／Workbench：`v0.1.0-test.oms-v1.105`
- 源码提交：`4dfbf99a97bbd074ef2cc78e880895f340c28ac7`
- Workbench 镜像摘要：`sha256:4e7be09a57c53cfe97bddaa8b57ae992aa306a65c13b6ba28d30e93aa6ceed3d`
- 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.105_linux_amd64.tar.gz`
- 发布包 SHA-256：`3771d18f14e3d3418ad4eabe03718aa8619a839df5f25fae947d51214dcb4569`
- 升级前备份：`/var/backups/omsbizfoundry/20261116T140053Z`
- ECS 验收目录：`/opt/omsbizfoundry/acceptance/oms-reimbursement-mode-switch-layout-v105-20261116`

## 验证

- Workbench JavaScript 语法、专项 10/10、Node 310/310、Go test／vet、P9 preflight 和 D1 composition 通过。
- v1.105 清单相较 v1.104 只替换 Workbench；发布包外层和包内 93 项 checksum、候选 IAM 契约预检通过。
- 升级前备份的 21 项 checksum 复验通过；正式 D1 `check` 通过，21/21 长运行容器 running，0 unhealthy。
- 运行中的 Workbench 镜像与候选摘要一致。
- 公网 `oms-phase1-finance-owner.js` SHA-256 为 `b02ee0b1e0026c171b28e7dee0bb202f91599d1d6b62c03bf36e8dd38a8f0548`，`oms-phase1.css` 为 `d8250ecc8cf7ddbf6ac9c37501cd1ffd1860f36c3071058ccb7b6a5d16fac144`，均与本地逐字节一致。
- 真实 Chrome 在 125% 缩放下显示左侧明细字段、右侧发票工作区的双列布局；第二项输入“切换保留测试”后执行单项 → 多项 → 单项 → 多项，第二项及文字恢复。验收最后点击取消，未保存申请。

## 升级说明

- 标准 `upgrade` 在备份完成后再次遇到既有 Cost Outsourcing 一次性 workload-key 端点占用旧默认网络；没有删除数据卷。
- 使用同一 v1.105 候选执行 D1 `up` 后收敛，并重新独立运行 D1 `check`。

## 未外推

- 本次没有创建、修改或提交报销事实，没有上传发票。
- 已持久化多项明细的阻断规则由源码和回归门禁覆盖；公网验收没有修改既有申请来制造该场景。
- 本次没有改变 Finance API、schema、Capability、金额算法、Invoice Recognition 或 Evidence 所有权。
