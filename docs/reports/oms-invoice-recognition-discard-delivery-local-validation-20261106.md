# 发票识别候选删除与交付本地验证（BF-312）

## 结论

本地候选已完成“待确认候选审计删除 → 默认隐藏／历史回看 → 已确认结果导出 → 显式登记 Billing 并关联原 Evidence”的代码闭环。普通 Go／Node 回归及临时 PostgreSQL 18 上的 migration、删除重放、状态冲突与不可变历史集成通过。本报告只记录本地证据；后续 D1／公网结果见[发布验证](./oms-invoice-recognition-discard-delivery-publication-20261106.md)。

## 事实与不变量

- 只有 `needs_review` 可删除；命令要求独立 Command ID、expected revision 和 1～500 字原因。
- 删除追加 `discarded` revision、命令响应和审计，保存 Actor／时间／原因；不删除 Evidence 或历史。
- `confirmed` 不允许删除；正式发票作废仍归 Billing。
- CSV／JSON 只导出已确认结果；JSON 带 Proposal revision、Evidence 精确版本、Provider／模型、警告和重复线索。
- 正式关联必须由用户选择进项／销项、项目和往来单位；Workbench 调用 Billing 正式命令，并把 Proposal 来源快照及原 Evidence 交给 Billing。
- Billing 成功、Evidence 关联失败时不回滚或重复创建，界面返回已创建记录和续办提示。

## 已运行验证

```text
node --check oms_phase1_workforce.js                         PASS
node --check oms_phase1.js                                   PASS
node --test oms_phase1_workforce.test.mjs                    PASS (4/4)
go test ./...  packages/invoice-recognition-contract         PASS
go test ./...  backend/services/invoice-recognition           PASS
go test ./...  frontend/oms-workbench                         PASS
go test -tags=integration ./internal/recognition              PASS (PostgreSQL 18)
```

最终复验还包括 `go vet`、Access Contract 回归和相关测试重跑；其结果以本轮交付记录为准。

相关模块 `go vet` 已通过。Access Contract 全量测试执行到旧 OA 权限目录用例时仍存在仓库既有差异：测试固定期望 49 条、当前目录为 48 条；本次未修改该目录，也未用调整旧 OA 规则掩盖失败。其余已执行的 schema／manifest 用例通过。

## 发布时仍保留的后续门禁

1. 浏览器真实验证删除、筛选、CSV／JSON 下载、进／销项登记、Evidence 回读和部分失败续办。
2. 在 Billing Owner 内增加类型化识别来源／消费回执和发票代码＋号码最终唯一约束，再考虑向识别页反投影“已使用”状态。
