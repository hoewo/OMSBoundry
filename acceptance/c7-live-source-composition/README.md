# C7 真实来源独立进程组合

该夹具启动真实的 IAM、Project Workspace、People & Groups、Contract Ledger、Money Flow Ledger、Bonus Distribution 与 PostgreSQL 18。六个应用使用独立数据库；初始化 SQL 只建立每个来源应用自己的验收前置事实，运行期的 Bonus 只能通过正式 IAM 工作负载凭证和版本化 HTTP 引用读取来源。

它验证经营奖金和生产奖金各自从真实 Contract Settlement、Money Receipt Set、People Person 与 Project Assignment 形成草稿并发布；同时验证新加入的 Person／Receipt Set 精确 capability、Project 对 Settlement 资源连接的持久化约束，以及各应用没有共享业务表。

执行：

```sh
acceptance/c7-live-source-composition/run-isolated
```

需要 Docker、Go、curl、jq 与 openssl。该夹具不包含人类页面、D1、备份恢复或向 Money 提交应付建议。
