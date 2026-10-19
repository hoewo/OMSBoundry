# OA C8 Money Flow MetricProjection v1 低资源单元验证

日期：2026-09-29  
判定：`c8-money-metric-source-unit-supported / consumer-and-d1-pending`

## 问题 A → 可验证结果 B

**A**：参考 OA 可以直接从共享财务表生成公司经营数字；若在组合层复制这套方式，会让工作台重新解释 Money 的金额口径，或把无权查看的金额伪装成 0。

**B**：Money Flow 作为事实所有者发布第一个 `MetricProjection v1` 来源，稳定声明指标身份、租户／本方主体范围、精确十进制金额、口径版本、观测时间、来源摘要和深链；指标结构与金额读取分别授权，无金额能力时值与摘要同时隐藏。

## 本批实现

- Money Flow 新增 `money-flow-ledger.statistics.read` 与 `money-flow-ledger.amount.read` 两个独立能力；
- 新增 `GET /api/v1/metrics/operating-summary`，仅从当前 Tenant 的既有 Operating Summary 事实生成投影；
- 第一版固定六个当前经营指标：应收未结、应付未结、有效收款、有效付款、未核销收款、未核销付款；
- 每项指标携带 `ledger_owner` Scope、Party ID、币种、`money-flow-operating-summary-v1` 口径、观测时间与相对深链；
- 有金额能力时返回来源已有的精确十进制字符串，并以指标事实计数参与 SHA-256 摘要；
- 只有指标结构能力时返回 `value: null`、`redacted: true`、`source_digest: null`，避免把真实金额伪装为 0，也避免从小值域摘要反推金额；
- 来源读取失败返回 503，不产生缓存或替代数字；响应标记 `Cache-Control: no-store`；
- 新增独立 `contracts/read-projections/v1/metric-projection.schema.json` 和发布示例，组合层不能通过该契约回写 Money。

## 串行验证

本批固定 `GOMAXPROCS=2`、`go test -p 1`、`nice -n 10`，未调用 Docker，未启动数据库、浏览器或常驻进程。

```bash
cd backend/services/money-flow-ledger
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./internal/identity ./internal/httpserver
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../../acceptance/access-contract
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
```

三组均通过。直接覆盖：

- 当前 Tenant 和 Ledger Owner 范围；
- 六个稳定 Metric Key 与确定性顺序；
- 有金额能力时精确值和摘要；
- 无金额能力时值与摘要同时为空且响应不泄漏测试金额；
- 缺少 `statistics.read` 返回 403 且不调用来源；
- Operating Summary 来源失败返回 503；
- Schema／示例严格字段、相同观测时间、相对深链、重复指标身份和 redacted 负例。

## 未覆盖与下一批

本批只证明一个来源应用可以安全发布类型化指标，不证明：

- Unified Workbench／Project 的指标消费者、来源分区页面或多来源局部失败；
- Gateway／IAM 的真实人类 Credential、Grant 撤销和账号停用；
- 正式 D1 的 Suite Manifest、选装／未选缺席、恢复或真实入口；
- 跨来源公式、跨币种合计、历史报表、导出或物化视图；
- 双 Tenant、真实 Ubuntu、HTTPS 和性能目标。

下一批仍只选一个低资源目标：把 Money 的 `MetricProjection v1` 作为可选来源装入 Manifest／Workbench 并用零容器测试验证来源分区、脱敏与局部失败；或者补一个既有链的真实入口，不同时启动完整 D1。
