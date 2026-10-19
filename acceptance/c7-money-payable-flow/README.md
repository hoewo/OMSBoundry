# C7 → Money 应付建议跨进程验收

这个验收只启动一个临时 PostgreSQL 容器以及 Bonus Distribution、Money Flow Ledger 两个本地进程，用来证明：

- 已发布 Bonus Allocation 的精确快照可以提出版本化应付建议；
- Money 不可用时 C7 保存 `target_unavailable`，恢复后用原命令继续提交；
- Money 独立接受时只创建一条由 Money 拥有的应付义务，独立拒绝时不创建义务；
- People Person 必须先经过精确 revision/digest 到 Money Party 的人工复核映射；
- 命令重放不产生重复事实，历史与人工映射不可改写；
- 来源侧随后追加撤回事实时，Money 已确认义务不会被跨库级联删除。

运行：

```sh
acceptance/c7-money-payable-flow/run-isolated
```

脚本在构建完成后才启动 Docker；无论成功或失败都会终止两个进程并删除唯一的临时容器。
