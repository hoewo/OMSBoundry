# OMS 已确认发票识别记录审计删除本地验证

日期：2026-11-12  
事项：BF-332  
状态：本地实现与验证通过，尚未发布

## A → B

问题 A：发票识别列表中的已确认记录没有删除入口，重复或误确认记录只能长期保留在默认列表。  
可验证结果 B：待确认和已确认记录都可在填写必填原因后追加为 `discarded`；默认列表不再展示，但识别候选、不可变确认、Evidence 引用、全部 revision、操作人、凭证、时间、原因和审计仍可追溯。

## 不变量

- 删除使用既有 `invoice-recognition.discard` Capability、版本化 API、Command ID 与 expected revision，不扩大 Grant。
- `confirmed → discarded` 是识别 Proposal 当前投影的追加式生命周期变化，不更新或删除 Confirmation，也不物理删除 Evidence。
- 已消费确认快照的报销、付款申请和 Billing 正式票据属于各自 Owner，不随识别记录删除而级联撤销。
- 删除原因不得预填且必须非空；Actor 来自可信身份上下文，删除时间和原因写入 revision 与审计。
- 默认列表隐藏 `discarded`，在“已删除”筛选中仍可查看原因、操作人和时间。

## 实现

- Invoice Recognition Service 与 PostgreSQL Repository 同时允许 `needs_review`、`confirmed` 两种状态进入 `discarded`，避免仅前端放开或仅服务层放开。
- Workbench 为已确认行增加“删除”，弹窗明确提示跨 Owner 不级联影响，并要求操作人主动填写删除原因。
- 已删除详情增加原因、操作人和时间；静态资源版本更新为 `oms-phase1-workforce.js?v=20261112.1`。

## 实际验证

- `backend/services/invoice-recognition`：`go test ./...`、`go vet ./...` 通过。
- 一次性 PostgreSQL 18：`TEST_DATABASE_URL=... go test -count=1 -tags=integration ./internal/recognition` 通过；确认后删除仍保留 1 条 Confirmation，并形成 3 段 Proposal revision 与 3 条审计事件。
- `frontend/oms-workbench`：Node 306/306、`go test ./...`、`go vet ./...` 通过。
- `acceptance/p9-full-composition/test-preflight`、`deployments/d1/ops/test-p9-composition`、`git diff --check` 通过。

## 未外推

- 本报告不表示公网已更新；验证完成时 `oatest` 仍为 v1.99。
- 未在公网删除任何真实记录，也未验证报销、付款或 Billing 的业务撤销；本变更只处理识别域列表与历史。
