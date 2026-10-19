# C6 Cost & Outsourcing 独立进程最小组合

该夹具用独立进程启动 IAM、Project、Party、Contract、Evidence、Cost & Outsourcing 和 PostgreSQL，验证：

- C6 人类工作区可打开，并由 `allowed_actions` 驱动动作；
- Cost Entry 与 Outsourcing Engagement 正常登记、精确来源快照和 Assessment；
- Evidence／Party 停机时零写入，恢复后成功；已成功 Command 在依赖停机时仍幂等重放；
- IAM 凭证刷新后金额脱敏和写动作收敛；只读凭证与错误 Project Scope 被拒绝；
- Money、Billing、应付款建议和 C7 不在最小组合中。

执行：

```sh
acceptance/c6-cost-outsourcing-composition/run-isolated
```

需要本机可用 Docker、Go、curl 和 jq；脚本只创建并清理自身命名的临时 PostgreSQL 容器和临时目录。该证据不是 D1 正式交付或真实 Ubuntu 结论。
