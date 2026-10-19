# C6 → Money 应付建议跨进程验收

这项验收用两个独立 PostgreSQL 数据库和两个真实 HTTP 服务验证：

- C6 从精确外协 revision 提出应付建议；
- Money 独立接收、接受并拥有生成的应付义务；
- Money 独立拒绝时不生成 Obligation，C6 只同步拒绝决定；
- C6 只同步 Money 的决定和 Obligation ID/revision；
- Money 停机时 C6 已有事实不丢失，恢复后可显式重试；
- 相同命令重放不重复生成 Obligation；
- 两个应用没有共享表或分布式事务。

运行：

```sh
./acceptance/c6-money-payable-flow/run-isolated
```

证据默认写入 `deployments/d1/.runtime/c6-money-payable-evidence/`。
