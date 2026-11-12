# 内部付款申请发票录入发布验证（2026-11-08）

## 结论

BF-313 已部署到公网 D1。公司内部“新增付款申请”现在提供“暂不录入、直接录入、发票识别录入”三种方式；识别从申请草稿发起，人工确认后恢复草稿并回填金额、日期、销售方和发票号码，同时把原发票的精确 Evidence File ID／Version 交给 Internal Finance Requests 保存。发票识别列表不再显示面向外部 Billing 的“关联使用”，既有付款申请仍不开放补票或换票。

公网 Internal Finance Requests 与 Unified Workbench 均为 `v0.1.0-test.oms-v1.83`。本次升级完成了升级前备份、候选预检、正式升级、独立 D1 检查、运行镜像摘要和公网静态资源逐字节校验；没有在线创建付款申请、识别候选、Billing 发票或 Money 资金事实，因此不把静态发布验证写成真实业务写旅程通过。

## 发布制品

- Internal Finance Requests：`v0.1.0-test.oms-v1.83@sha256:32ac2be04cb47a5031c66ada834a41336c973dcf6960a4bfc205f2c0d44623aa`
- Unified Workbench：`v0.1.0-test.oms-v1.83@sha256:f216c3b296f199386a5144b769add3c2476d7afb432aebfcec937b4d301aed8b`
- 发布包：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.83_linux_amd64.tar.gz`
- 发布包 SHA-256：`a2191b4c5f3ef08050ef8ea7c795c7d82a6db7c9875d393d2375c4496b1475cb`

其余正式服务沿用 v1.82 环境固定的原镜像摘要；本次没有改变 Owner 数据所有权或数据库 schema。

## 升级与恢复证据

- 目标机 `preflight-upgrade` 通过，候选 IAM 返回 `contracts-valid`。
- 正式升级前恢复点：`/var/backups/omsbizfoundry/20261108T103211Z`。
- `checksums.sha256` 中 21 项备份制品全部通过。
- 正式 `upgrade` 返回 0；Internal Finance Requests schema 保持 `000004_draft_retention.sql`，Invoice Recognition schema 保持 `000002_discard_proposals.sql`。
- 升级后独立执行 `d1 check` 返回 0；所有报告的 schema 均为 current，没有 pending migration。
- Compose 正式组合共有 21 个长运行服务，全部 healthy；Finance 与 Workbench 的运行镜像及摘要与候选一致。

## 公网复核

- `https://oatest.antuinfo.cn/` 返回 303 并跳转 `/workbench/?view=oms-v1`，Workbench 返回 200。
- 公网 `oms-phase1.css?v=20261106.6` SHA-256 为 `3573ba1dd2c58d21c4b5c7cab428e293d7106af17a68388baee25b2f863d2f77`，与本地候选一致。
- 公网 `oms-phase1-finance-owner.js?v=20261106.1` SHA-256 为 `e374a71ccefa563668cca47cb794a5190b2c242f019a48d5ae8998e5b5da7f3f`，与本地候选一致。
- 公网 `oms-phase1-workforce.js?v=20261106.6` SHA-256 为 `5bd702af9fa7ca6aa5925686b1f0892bab89196de2249ffbc40b7f4e8b50a1eb`，与本地候选一致。
- 匿名读取 Internal Finance Requests 受保护端口返回 303 登录重定向，没有绕过 IAM。

## 已验证与仍待验证

- 本地 296 项 Workbench Node、Workbench 与 Finance 全部 Go test／vet，以及 PostgreSQL 18.6 付款发票跨类型重复集成均已通过；详见[本地验证](./oms-internal-payment-invoice-entry-local-validation-20261106.md)。
- 本次发布没有代填线上账号密码，也没有执行“新建付款申请 → 上传识别 → 人工确认 → 回填 → 保存 → 详情回读”的真实登录写旅程。
- 既有申请补票／换票、Billing 外部关联入口、会计入账、自动付款、PDF 多页和 OFD Provider 均不在本轮完成范围。
