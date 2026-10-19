# OA C8 Money 指标 → Workbench 消费者低资源单元验证

日期：2026-09-29

## A → B

**A**：Money Flow 已发布严格的 `MetricProjection v1`，但 Suite Manifest 与人类工作台尚未消费它，无法验证来源分区、金额脱敏和来源局部失败是否会在展示层继续成立。

**B**：Suite Manifest 以可选 `metric_projection` 声明来源路径和读取动作；Unified Workbench 只发现已启用来源，经 Gateway 为当前 Session 取得来源短期 Credential，严格校验来源、Tenant、Scope 和精确值／redacted 不变量，并按来源、Ledger Owner Party、Currency 分区展示，不生成跨来源总计。

## 实现结果

- Suite Manifest v1 新增可选 `metric_projection {schema_version,path,action}`；Gateway、Workbench 和发布契约均拒绝平台应用声明、未知版本、非 API 路径及跨 Capability Namespace 动作。
- 本地、发布示例与 D1 Manifest 渲染器为 Money Flow 声明 `/api/v1/metrics/operating-summary` 和 `money-flow-ledger.statistics.read`，未选装应用或未声明来源时不发起指标请求。
- Workbench 新增“经营指标”来源分区；每个来源独立授权、3.5 秒超时，最多四路有界读取。一个来源的 403、5xx、超时、非法 JSON、错 Tenant 或契约漂移只降级该来源。
- 有 `amount.read` 时按来源返回的十进制字符串原样显示；没有金额能力时只接受 `value=null`、`source_digest=null`、`redacted=true`，显示“金额已隐藏”，不显示 0，也不尝试计算。
- 条目按来源、Ledger Owner Party、Currency 与 Metric Key 排序去重，深链被限制在来源 Suite Route Prefix；工作台不保存指标、不复制公式、不跨来源合计。

## 低资源验证

以下命令严格串行执行，`GOMAXPROCS=2`、`go test -p 1`，未启动 Docker、PostgreSQL、浏览器或持续运行服务：

```bash
cd frontend/oms-workbench
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../gateway
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../../acceptance/access-contract
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
```

三组均通过。Workbench 嵌入 JavaScript 的 `node --check`、发布／本地 Manifest JSON 的 `jq empty`、D1 Manifest 实际渲染与字段断言，以及 `git diff --check` 同时通过。

## 当前判定

`c8-money-metric-single-consumer-unit-supported / gateway-browser-d1-pending`

本批只证明单一指标来源的声明、发现、严格展示契约、脱敏与局部失败语义。真实 Gateway→IAM→Money Cookie 链、真实浏览器、Grant 撤销时序、第二指标来源运行时隔离、正式 D1、恢复、双 Tenant、真实 Ubuntu 和跨来源公式均未验证；不能将本结果描述为完整统计体系或正式环境通过。
