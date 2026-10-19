# ADR-0028：Money Flow 导出采用本方主体范围的可校验 CSV 归档

日期：2026-09-20  
状态：接受

## 背景

Money Flow 已形成 Party、Obligation revision、Adjustment、Remission、Closure、Cash Expectation、Cash Transaction、Reversal、Allocation 和 Internal Transfer 等事实。M4 要求数据能够移交和离线核对，但当前没有证据支持先建设报表平台或通用导入器。全库备份恢复已经作为独立运维边界实现，不替代 owner 业务数据移交。

Contract Ledger 可以用单个 Contract ID 形成天然聚合；Money Flow 的事实则由本方账簿主体贯穿。Cash Transaction 与 Obligation 的 Allocation 已强制要求同一本方主体，因此 `ledger owner Party ID` 是既不会切断核销关系、又不会带出其他账簿数据的最小完整边界。

简单导出当前经营汇总会丢失不可变历史并形成第二套真相；直接提供 PostgreSQL dump 又会暴露身份凭证、会话、幂等实现和内部结构。单张扁平 CSV 也无法无损表达一对多 revision、核销及反向关系。

## 决策

1. 第一版每次导出一个 `ledger owner Party ID` 的完整业务事实，不做全库或任意筛选导出。
2. 使用版本化 ZIP；当前 `owner-ledger-export-v2` 固定包含一个 `manifest.json` 和十三张关系明确的 CSV，其中 `obligation_cash_expectations.csv` 保存完整预计日期历史。已交付的 v1 固定为十二张 CSV 且不包含该事实；新增事实以新格式版本表达，不静默改变 v1。
3. CSV 使用固定列、UTF-8、RFC 4180、CRLF 和确定排序；精确金额始终是十进制文本，空表仍保留表头。
4. manifest 对每张 CSV 保存行数、原始字节数和 SHA-256，并记录 Party、schema version、编码与排除项；摘要不是签名或可信时间戳。
5. 用户文本原样保存。由于 CSV 软件可能把 `= + - @` 开头内容解释为公式，归档和页面要求按文本导入；系统不以添加前缀的方式静默改变原文。
6. 导出包含选定本方主体的全部现有业务事实、冻结快照、Expectation 历史和范围内审计；不保存 current 汇总、预计金额、账龄或余额等派生投影。
7. 当前 Party 目录只输出本方主体和本次事实引用的往来方；其他本方主体拥有的事实、身份凭证、会话和命令 payload hash 均排除。
8. repository 在一个只读 `REPEATABLE READ` 事务中读取全部内容，并拒绝任何跨本方主体 Allocation。
9. 浏览页面使用 GET，真正生成归档使用带 CSRF 的 POST；只允许 Money Flow 本地 administrator。
10. 服务先构造完整归档，再追加 `money_flow_ledger_exported` 审计，审计成功后才发送响应；当前审计不进入自己的快照。
11. 第一版是同步内存归档。流式输出、后台任务、大小上限或导出队列等待真实数据规模证据。

详细规格见 [本方主体可校验台账导出](../specs/money-flow-ledger/verifiable-owner-ledger-export.md)。

## 结果

优点：

- 每个归档内部的 Cash、Obligation、Allocation 和 Reversal 引用闭合，可以离线重建当前余额；
- 多 CSV 对分析工具友好，同时保留关系和历史，不把汇总变成新事实；
- Party ID 和账簿边界显式，避免同名主体合并或不同客户账簿数据混出；
- 每张表可独立校验，传输损坏或文件替换容易发现；
- 不依赖 Contract Ledger、共享文件系统或外部服务。

代价：

- 使用方需要按 ID 关联多张表，不能把归档误当作一张即开即用的经营报表；
- CSV 原值与电子表格公式防护不能同时靠静默改值满足，接收方必须按文本导入；
- 同步内存构造不适合未经验证的超大账簿；
- 导出格式一旦交付就需要版本兼容，破坏性字段变化必须发布新格式版本。

## 否决方案

### 导出当前经营汇总

否决。汇总可以从原事实重建，单独导出会丢失 revision、核销、反向和审计，且容易被当成第二套可维护余额。

### 一张宽 CSV

否决。Obligation、Revision、Allocation、Reversal 和 Cash 之间是一对多／多对多关系，宽表只能重复、截断或隐藏历史。

### 全库 PostgreSQL dump

否决。它属于灾难恢复，不是稳定产品契约，并会混入本地身份、会话和内部幂等结构。

### 与 Contract Ledger 联合导出

否决。当前没有真实跨系统业务流动，联合快照会制造不必要的运行依赖和分布式一致性问题。

### 同时实现导入或恢复

否决。冲突、幂等、版本迁移和外部 ID 归属尚未定义；先验证单向数据移交，再独立设计写入边界。
