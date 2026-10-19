# Approval & Audit Results

这个应用独立拥有“审批结果”和“审计结果”两类已确认业务事实。当前已经实现领域核心、PostgreSQL 生命周期、版本化受保护 HTTP 入口、原生 IAM 最终授权、Project／Contract／Evidence 精确引用适配器和按 Project 使用的人类工作区；Project Workspace 已实现 `reference-oa-c5-v1` Completion Profile 的只读评估与显式检查点确认，真实跨进程组合与 D1 装配仍未开始。

## 当前已实现边界

C5 资料来源切片：当前报告引用已有事实 → 发布 Result 当前 revision 的只读 File ID／Version／slot 投影；使用既有 read 能力，不附带金额、不调用上游、不改变 Completion Reference 或历史。源码／单元、真 PG18 与真实 IAM8 的原生能力／精确 Scope 检查已通过，Main 尚未部署、Project 消费及完整 C8 未通过。见 [规格](../../../docs/specs/assembly/results-material-reference-v1.md)与[实测](../../../docs/reports/auth58-results-material-source-validation-20261001.md)。

- 一个租户、项目和结果类型只有一条当前投影，审批和审计分别拥有独立记录。
- 金额按 `reference-oa-v1` 固定拆为设计费、勘察费和咨询费，全部使用最小货币单位整数并由服务端计算合计。
- 审批结果必须引用 Contract Ledger 的精确合同或结算 revision。
- 审计结果必须引用本应用内审批结果的精确历史 revision，不会重读当前合同或当前审批投影。
- 更正、撤回和撤回后的重新登记只追加 revision；历史、命令回放和审计事件不可修改或删除。
- Project、Contract 和 Evidence 引用在写入前解析为精确快照；任一依赖失败时不产生局部事实。
- 命令 ID、预期 revision、事务锁和租户隔离共同约束幂等与并发。
- Project reference-read 必须同时证明项目 revision 与活动资源链接；Contract／Settlement reference-read 提供来源类别、币种和精确金额，Evidence reference-read 复验 File ID／Version／SHA256。
- 目标应用复验 IAM 的 tenant、audience、原生 Capability 和 Project／Result Scope，并在读取响应中按 `approval-audit-result.amount.read` 服务端脱敏金额。
- `reference-oa-v1` 当前仅接受一条已链接 Contract／Settlement，把来源类别映射到设计费、勘察费或咨询费的一项，其余两项为零；多合同聚合尚未证明，不会在本轮猜测实现。
- 人类工作区按 Project 展示当前结果，允许具备相应 `allowed_actions` 的用户登记、纠正、撤回和读取历史；页面不自行推导权限或信任浏览器传入的来源事实。
- Project Completion Profile 只读检查 Results 当前事实且 GET 零写入；显式确认冻结 Project expected revision 与精确 Results 来源 revision，只追加 Project revision、确认记录和历史。来源随后变化时旧确认显示 drift，Project 状态仍保持 active，不能把 C5 检查点冒充完整 OA 完成。

## 尚未实现与未证明

- 统一工作台界面、Gateway/Nginx 路由、D1 镜像与恢复验收。
- 真实独立进程之间的端到端联调和多合同／多结算来源聚合。

## 验证

```sh
go test ./backend/services/approval-audit-results/...
TEST_DATABASE_URL='postgres://...' make approval-audit-results-core-acceptance
TEST_DATABASE_URL='postgres://...' make approval-audit-results-protected-http-acceptance
cd backend/services/project-workspace && go test ./...
```

`TEST_DATABASE_URL` 必须指向可丢弃的 PostgreSQL 18 数据库；测试会创建并删除独立 schema。受保护 HTTP 验收覆盖 401／403／409／422／503、金额脱敏、精确工作负载凭证、依赖失败零写入及恢复后同 Command 单次生效。Project 的真库测试另覆盖 Completion GET 零写入、显式确认、幂等回放、stale／依赖失败关闭和来源变化 drift。

运行服务时必须显式配置 `PROJECT_BASE_URL`、`CONTRACT_BASE_URL`、`EVIDENCE_BASE_URL`。本地模板模式使用独立的 24 字节以上访问令牌；IAM 模式需要 IAM issuer／公钥、Results 工作负载私钥和服务凭证交换端点，静态引用令牌在 IAM 模式下会被拒绝。默认监听 `:8095`。
