# OMS 多项报销单行起步与布局收敛 v1.106 发布记录

日期：2026-11-17（GMT+8）

事项：BF-340

状态：已发布；无业务写入验收

## 发布结果

- 公网入口：`https://oatest.antuinfo.cn/`
- Suite：`v0.1.0-test.oms-v1.106`
- 源码提交：`a40816c04de4f0e0643b6582d8ac9c51dc27dd2c`
- Internal Finance Requests 镜像摘要：`sha256:89b00e96d4e4902048297e40e1055e8bc9bcb7ef2b6e78900c96f447520a0a4f`
- Unified Workbench 镜像摘要：`sha256:9d6a3fd8559c3c2d5e996b18f26453c578f1c98ee8ab6fbdaaa8ffb543c25a22`
- 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.106_linux_amd64.tar.gz`
- 发布包 SHA-256：`8a924386e47506daf55f4a3a01375fd767690333a73f8dc94f1c36bc691ba032`
- 升级前备份：`/var/backups/omsbizfoundry/20261117T111224Z`

## 验证

- Finance／Workbench Go test 与 vet、Workbench Node 310/310、JavaScript 语法及 PostgreSQL 18.6 集成通过。
- P9 preflight、D1 composition、发布包外层校验和包内 93 项校验、候选 IAM 契约预检通过。
- 升级前备份的 21 项 checksum 复验通过；正式 D1 `check` 通过，21/21 长运行容器 running，0 unhealthy。
- 运行中的 Finance Owner 与 Workbench 均为 v1.106 候选镜像。
- 公网 `oms-phase1-finance-owner.js` SHA-256 为 `013b5447555f9ff0581c7485db98cb315d04845477b59adf1ee982173d8dba80`，`oms-phase1.css` 为 `2627fd3492b8e2b2ee077776d3598cf76a796033748efa21f6fc4b5f5fec5e09`，均与本地逐字节一致；公网根入口 303 到 Workbench，Workbench 返回 200。

## 升级说明

- 标准 `upgrade` 在备份完成后再次遇到既有 Cost Outsourcing 一次性 workload-key 端点占用旧默认网络；没有删除数据卷。
- 使用同一 v1.106 候选执行 D1 `up` 后收敛，并重新独立运行 D1 `check`。

## 未外推

- 浏览器刷新后既有登录会话已过期并到达登录页；没有代用户输入登录凭据，因此不宣称完成登录后的真实视觉或写旅程。
- 本次没有创建、修改、提交或删除报销申请，没有上传发票。
- 本次没有新增 schema migration，也没有改变 Invoice Recognition、Evidence、Billing、Money 或其他 Owner 的事实所有权。
