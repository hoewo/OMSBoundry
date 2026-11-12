# 财务申请进入待支付与幂等重放发布验证（BF-322，2026-11-09）

## 问题与结果

财务申请列表原来的“提交申请”实际表示把草稿变为 `payable`，不是执行付款。该表述已改为“进入待支付”，确认框同时明确说明不会执行付款，实际付款仍是后续独立动作。

线上记录 `4·14·12` 的首次命令已经成功提交为 `payable / revision 2`，但浏览器重放同一命令时，Internal Finance Requests 在幂等记录查询前先执行了旧 revision 的服务层预检，于是把已经成功的结果误报为 `finance request revision conflict`。修复后，相同 command ID 与相同请求内容先由 Store 命中既有命令结果并原样重放；不同命令或不同内容仍保持 revision／状态冲突，不放宽并发保护。

Workbench 还会在收到 revision conflict 时精确重读当前申请：若 Owner 已显示 `payable`、`partially_paid`、`paid` 或 `paid_with_overpayment`，则按已成功处理并刷新列表；否则显示中文提示，要求用户重新确认，不再直接暴露内部英文错误。

## 本地验证

- Internal Finance Requests：`go test ./...`、`go vet ./...` 通过。
- 新增服务层单元测试覆盖“当前 revision 已前进时仍委托 Store 做幂等重放”，并断言不会重复执行票据校验。
- 新增 PostgreSQL 集成场景，覆盖同一 command ID 在首次提交成功后的重复提交返回同一 `payable` 结果。
- Unified Workbench：`go test ./...`、`go vet ./...` 通过。
- Workbench Node 回归共 300 项通过；JavaScript 语法检查通过。

## 不可变制品

- Internal Finance Requests：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/internal-finance-requests:v0.1.0-test.oms-v1.93@sha256:47e6c136a2d40162743a70a85fd80bc896320ff9cc42dfa87290fe0c0e054201`
- Unified Workbench：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.93@sha256:113e01520b7374470d32ec3d8325fcfb9d6626f50bbfde98c9f80be114f63bb3`
- D1 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.93_linux_amd64.tar.gz`
- 发布包 SHA-256：`d1ca13d1879e3fb595bca01656fc8899aac7f44ca684b5bf4e109c3ed60df3dd`
- 公网财务脚本 SHA-256：`c0dde54140cf5ab5dd3b8b92364d701db795b6449d80f56216683a6d9ced4559`

## 发布与运行证据

- 候选发布包校验和与候选 IAM 契约预检通过。
- 升级前联合备份：`20261109T111018Z`。
- Compose 清理旧网络时遇到活动端点竞态；数据卷未删除，随后用同一 v1.93 候选和同一受限环境文件执行幂等 `d1 up`，迁移与服务正常恢复。未回退旧二进制，也未执行数据恢复。
- 独立 `d1 check` 通过；Internal Finance Requests schema 保持 `000005_optional_invoice_code.sql`，没有新增 migration。
- 21 个长期容器全部为 healthy；运行中的 Finance 与 Workbench 镜像摘要均与目标摘要一致。
- `https://oatest.antuinfo.cn/workbench/assets/oms-phase1-finance-owner.js?v=20261109.1` 返回 `200`，线上与本地 SHA-256 完全一致，并包含“确认进入待支付”和中文 revision 冲突恢复提示。
- 发布后只读复核 `4·14·12` 仍为 `payable / revision 2`，更新时间保持 `2026-11-09 10:52:14.244286+00`；部署验证没有再次写入该申请。

## 边界

本次不合并“进入待支付”和“确认实际支付”，不放宽报销凭证门禁、付款申请确认、Money 支付写入、Owner 最终授权或 revision 并发保护。没有新增 schema、Capability、Grant 或业务事实。
