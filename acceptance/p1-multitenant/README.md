# P1 多租户真实进程验收

问题 A → 可验证结果 B：IAM 与 People & Groups 的单元测试不能证明两个真实服务进程、工作负载换票和空目标恢复共同保持 Tenant 边界 → 本验收在一次性 PostgreSQL 18.6 容器中启动真实 IAM／People 二进制，执行双租户正反例、撤权／停用、备份恢复和恢复后续写。

运行：

```sh
./acceptance/p1-multitenant/run-isolated
```

脚本只创建名称带随机后缀的一次性容器和临时目录，不使用或停止 `oms-v1-integration-*`、D1 或开发数据库。失败时测试日志保存在系统临时目录，并由 `go test` 报告具体路径。

覆盖：

- 同一 IAM 进程中的 Tenant A／B 独立登录与能力签发；
- People 真实 HTTP 创建、列表和稳定 ID 猜测拒绝；
- Subject↔Person 只允许同租户稳定引用；
- 撤销写能力后新凭证立即收敛，停用账号后会话立即失效；
- IAM 与 People 分库 `pg_dump`，恢复到两个新空数据库；
- 恢复后 Tenant、稳定 ID、revision、Subject 快照保持，并可继续追加新 Person。

它不代替 Owner 的搜索、分页、导出和 Evidence 附件矩阵；这些仍由对应 Owner／D1 验收负责。
