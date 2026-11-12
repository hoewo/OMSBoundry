# 发票识别候选删除与交付发布验证（2026-11-06）

## 结论

BF-312 已部署到公网 D1。待确认识别候选现在支持带原因的追加式删除并默认隐藏；已确认结果支持 CSV／结构化 JSON 导出，也可由用户明确选择进项或销项、项目和往来单位后登记 Billing，并关联原 Evidence File ID／Version。识别域不自动入账，已确认候选不能在识别域删除。

公网 Invoice Recognition／Unified Workbench 均为 `v0.1.0-test.oms-v1.82`。本次正式升级完成了升级前备份、候选契约预检、PostgreSQL migration、独立 D1 检查与公网资源校验；没有执行真实候选删除、确认或 Billing 登记，不把静态发布验证写成业务写旅程通过。

## 发布制品

- Invoice Recognition：`v0.1.0-test.oms-v1.82@sha256:3b4619faf17e7dbe3b36ef2e0760e6ef56f19c4895b0a1d76e2617f75b996993`
- Unified Workbench：`v0.1.0-test.oms-v1.82@sha256:dd002c14684faa5dcbeb77204947d0a5b1c85d4d3270d7e53b00f8b01d845ec3`
- 发布包：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.82_linux_amd64.tar.gz`
- 发布包 SHA-256：`9c7cdbe93096124a6da7afaf7aac00cff944c7113fa402d4ee376cfd1c3d65ba`

除上述两个镜像外，其余正式服务沿用 v1.81 环境固定的原摘要；D1 升级按同一组合重建，不改变 Owner 数据所有权。

## 升级与恢复证据

- 目标机候选 `preflight-upgrade` 通过，候选 IAM 能解析全部 Capability Manifest；`invoice-recognition.discard` 已进入运行契约。
- 正式升级前恢复点：`/var/backups/omsbizfoundry/20261106T142017Z`。
- `checksums.sha256` 中 21 项备份制品全部通过。
- 正式 `upgrade` 返回 0；Invoice Recognition schema 为 `000002_discard_proposals.sql`。
- 升级后再次独立执行 `d1 check` 返回 0。
- Compose 正式项目共有 21 个长运行服务，全部为 healthy；Invoice Recognition 与 Workbench 运行镜像摘要和候选一致。

## 公网与权限复核

- 公网 Workbench 资源可读取：`oms-phase1.css?v=20261106.5`、`oms-phase1-workforce.js?v=20261106.5`、`oms-phase1.js?v=20261106.3`。
- 线上脚本包含 `data-invoice-discard`、`invoice-recognition.delivery/v1`、`oms-v1:use-recognized-invoice`、`manual_confirmation` 和 `invoice_document`。
- 匿名调用 `POST /invoice-recognition/v1/api/v1/proposals/{id}/discard` 返回 401，路由存在且未绕过 IAM。
- 当前 computer use 的本地浏览器控制在 Work 设置中被禁用，因此本次没有把登录态页面点击或真实写命令列为通过。

## 本地门禁

- Invoice Recognition Contract、Owner 与 Workbench 的 `go test`／`go vet` 通过。
- Workbench JavaScript 语法与专项 Node 4/4 通过。
- PostgreSQL 18 migration、discard 幂等／冲突、confirm 不可变历史集成通过。
- Access Contract 的相关 schema／manifest 用例通过；全量套件仍有与本次无关的既有 OA 权限目录数量差异（测试期望 49、当前 48），本次没有修改旧 OA 目录掩盖该失败。

## 明确未完成

- 登录态“删除 → 已删除筛选 → CSV／JSON 下载 → 进／销项登记 → Billing／Evidence 回读”仍须以专门测试数据执行。
- Billing 尚无原生类型化识别来源、消费回执反投影和发票代码＋号码最终唯一约束。
- OFD 仍需可替换 Provider，PDF 仍只处理首页；多版式脱敏样本准确率门禁尚未建立。
