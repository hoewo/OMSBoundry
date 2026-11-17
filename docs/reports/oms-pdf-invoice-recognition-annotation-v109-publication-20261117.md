# OMS PDF 发票识别与标注恢复 v1.109 发布记录

日期：2026-11-17（GMT+8）

事项：BF-341

状态：已发布；既有 PDF 只读验收通过

## 发布结果

- 公网入口：`https://oatest.antuinfo.cn/`
- 最终 Suite 制品：`v0.1.0-test.oms-v1.109`
- 识别实现提交：`0a0e17de95642240daa4a4dd8d08a9a59ba5fe1c`
- 缓存键修复提交：`403389f08253f1f235fca42fa4902297f660e472`
- Invoice Recognition 镜像：`sha256:f3fcff7caf7eccf1551432ddb9738ca041454e8d9f574a29daf012462ed16d4b`
- Unified Workbench 镜像：`sha256:466e1c080dcc3c9f528a67f68f733e3923f27b56bffd945a4a9d17e60737d84e`
- 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.109_linux_amd64.tar.gz`
- 发布包 SHA-256：`8724d4a70bca1e25fdc746e676020f51f04927259c9c6b0338eff176affc76c0`
- 识别服务升级前备份：`/var/backups/omsbizfoundry/20261117T131340Z`
- 最终 Workbench 升级前备份：`/var/backups/omsbizfoundry/20261117T132623Z`

## 验证

- Invoice Recognition Go test／vet、Workbench Go test／vet、JavaScript 语法、Node 310/310、P9 preflight 与 D1 composition 通过。
- 两个应用镜像均为 Registry 锁定的 `linux/amd64`；发布包外层 SHA-256 与包内 checksum 集通过。
- 最终正式 D1 `check` 通过，21/21 长运行容器 running 且 healthy；运行中 Invoice Recognition 与 Workbench Image ID 分别等于上述摘要。
- 公网 `oms-phase1.js` SHA-256 为 `d021d2bcba262e9cb5756270a64aa8260d5426f544e57b5bf84c9061f01dc179`，`oms-phase1-workforce.js` 为 `ad151252aa1fa265ee82074becde4f4430d0ccdde0b2c242c535a37b9be39282`，与本地一致。
- 真实 Chrome 刷新并打开截图对应既有 PDF 后，左侧为统一图片查看器而非浏览器 PDF `iframe`，显示缩放工具和 9 个可交互标注：发票号码、开票日期、购销双方及税号、不含税金额、税额、价税合计。
- 线上复验只读取现有 Proposal，没有上传、确认、删除或修改发票，没有创建 Finance 事实。

## 发布过程说明

- v1.108 首次发布后，真实 Chrome 发现查询键 `20261117.2` 曾被先前候选使用，浏览器命中旧缓存并把 PNG 当 JSON 解码；后端审阅图请求本身返回 200、233630 字节。
- Workbench 随后把两个相关不可变资源键提升到 `20261117.3` 并发布 v1.109。刷新后浏览器重新获取资源，既有 PDF 的 9 项标注实际可见。
- 首次标准 `upgrade` 在备份后命中既有 Cost／Outsourcing 一次性 workload-key 容器占用旧默认网络；未删除数据卷，使用同一候选执行 `d1 up` 后收敛。

## 边界

- 既有 Proposal 候选保持不可变；本轮只恢复其标注显示，不静默清洗历史名称。
- 左右分栏和中文 OCR 空格清理只作用于新上传生成的新 Proposal。本轮没有为验收重复上传同一业务发票，因此不宣称取得公网新样本准确率证据。
- 仍只识别 PDF 首页，不新增多页选择、OFD 或跨版式准确率承诺。
