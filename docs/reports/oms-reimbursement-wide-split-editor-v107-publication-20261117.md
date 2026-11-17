# OMS 报销宽窗双区编辑 v1.107 发布记录

日期：2026-11-17（GMT+8）

事项：BF-341

状态：已发布；无业务写入验收

## 发布结果

- 公网入口：`https://oatest.antuinfo.cn/`
- Suite：`v0.1.0-test.oms-v1.107`
- 源码提交：`72b31a91de17c65ada9a6a1782afe6b183a26d61`
- Unified Workbench 镜像摘要：`sha256:a0cdfc634c23579b958232a87aaf129602301eb2bf28ce6a43b43aa8c3e44311`
- Internal Finance Requests 沿用 v1.106：`sha256:89b00e96d4e4902048297e40e1055e8bc9bcb7ef2b6e78900c96f447520a0a4f`
- 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.107_linux_amd64.tar.gz`
- 发布包 SHA-256：`41db5d9978af8a9ccde585ec57f437734ed5b7518af8cbb3dbc11f27bf336329`
- 升级前备份：`/var/backups/omsbizfoundry/20261117T115753Z`

## 验证

- JavaScript 语法、专项 10/10、Workbench Node 310/310、Go test／vet、P9 preflight 与 D1 composition 通过。
- 发布包外层校验和、包内 93 项校验、候选 IAM 契约预检通过。
- 升级前备份 `checksums.sha256` 的 21 项复验通过；正式 D1 `check` 通过，21/21 长运行容器 running，0 unhealthy。
- 运行中的 Workbench 为 v1.107 候选摘要；公网根入口 303 到 Workbench，Workbench 返回 200。
- 公网 `oms-phase1.css` SHA-256 为 `3b6159abbdd434eefa5e41419c12199a22424d98b3d8971375ebe76a260df534`，主脚本为 `b1549bb171238120c4a737e2f8da4a3a7e239ecdcb1d20fccd9a6254c6f9ffa3`，财务脚本为 `01b3ee2b9a808dd4d73797582e2cd9ca5a116364fe3cf944422d07aa6c3ac3ab`，均与本地逐字节一致。

## 升级说明

- 标准 `upgrade` 在备份完成后遇到既有 Cost Outsourcing 一次性 workload-key 端点占用旧默认网络；没有删除数据卷。
- 使用同一 v1.107 候选执行 D1 `up` 后收敛，并重新独立运行 D1 `check`。

## 未外推

- 本次没有创建、修改、提交或删除报销申请，没有上传发票。
- 本次没有 schema migration，也没有改变 Finance、Invoice Recognition、Evidence 或其他 Owner 的数据所有权。
