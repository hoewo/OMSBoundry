# Money Flow Ledger：多条单笔 Allocation 组合闭环

里程碑：M3  
功能编号：MF-M3-007  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员继续使用 MF-M3-006 已验证的单笔 Allocation 命令，完成一笔资金分配给多条义务、多笔资金共同核销一条义务，以及部分余额保留的组合闭环。管理员可以从任一实际资金或义务详情下钻到与它直接相关的全部 Allocation，并逐条返回另一端事实。

本功能不引入批量写协议，也不增加第二套余额或汇总事实。组合关系只由多条既有不可变 Allocation 构成。

## 业务事实与状态变化

- 每次提交仍然只新增一条 Cash Transaction 与一条 Obligation current revision 之间的 Allocation；
- 同一 Cash Transaction 可以通过不同 Command ID 形成指向多条兼容 Obligation 的 Allocation；
- 同一 Obligation 可以通过不同 Command ID 接受来自多笔兼容 Cash Transaction 的 Allocation；
- 同一对 Cash Transaction／Obligation 也允许形成多条 Allocation，历史不能合并或覆盖；
- 每条 Allocation 的幂等、兼容性、Expected Obligation Revision、精确金额、两侧不超额、可信 Actor、原子审计和不可变约束继续完全沿用 MF-M3-006；
- 成功写入只追加 Allocation、命令和审计，不更新资金金额、义务 revision、义务原始金额或可写余额字段；
- 任一侧余额归零都不自动改变 Cash Transaction 的 `recorded` 状态，也不自动把 Obligation 从 `open` 推进到其他状态；
- 详情历史是现有 Allocation 事实的只读投影，不产生审计或任何新业务事实。

## 唯一组合与守恒口径

代表性闭环固定使用下列事实证明多对多成立：

```text
资金 A 120 ──40──> 义务 X 100
         └──80──> 义务 Y  80

资金 B  80 ──60──> 义务 X 100
```

完成后三条 Allocation 合计 `180`：

- 资金 A：allocated `120`，unallocated `0`；
- 资金 B：allocated `60`，unallocated `20`；
- 义务 X：allocated `100`，outstanding `0`；
- 义务 Y：allocated `80`，outstanding `0`；
- 资金侧已分配总额、义务侧已分配总额与 Allocation 总额都必须等于 `180`；
- 两条义务即使 outstanding 为 `0` 仍保持 `open`。

以上三次写入必须是三个独立单笔命令。任何一次失败只回滚自身，不形成跨命令事务或“部分成功的批次”概念。

## 下钻历史

### 实际资金详情

- 显示该 Cash Transaction 的原始金额、allocated 与 unallocated；
- 显示引用该资金的全部 Allocation，不混入其他资金的记录；
- 每行至少显示 Allocation、目标义务及其 revision、金额、人工分配依据、操作人和生效时间；
- Allocation 与义务均可继续进入各自详情。

### 义务详情

- 对 `open` 义务显示原始金额、allocated 与 outstanding；
- 显示引用该 Obligation 的全部 Allocation，不混入其他义务的记录；
- 每行至少显示 Allocation、来源资金、金额、人工分配依据、操作人和生效时间；
- Allocation 与实际资金均可继续进入各自详情；
- draft 义务显示空分配历史，不把它加入余额。

两类历史固定按 `(applied_at DESC, allocation_id DESC)` 排序。相同时间下仍有确定顺序；读取失败返回明确服务错误，不能伪装为“尚无分配”。当前 M3 验收数据量不引入分页；出现真实规模证据后再单独定义绑定父事实与筛选条件的 keyset 游标。

## 应用与数据边界

- Allocation 领域服务提供按 Cash Transaction ID 和按 Obligation ID 的类型化历史查询；无效 UUID 在进入 repository 前拒绝；
- PostgreSQL 查询继续从 `allocations`、`cash_transactions` 和 `obligation_revisions` 读取不可变快照，复用 `000007` 已有父 ID／时间索引；
- 本功能不新增 migration、表、触发器、状态、余额列或缓存；
- HTTP 详情必须先确认父事实存在，再读取其 Allocation 历史；父事实不存在返回 404，历史读取失败返回 500；
- 页面和查询仍要求 Money Flow 本地 administrator，会话、Cookie、CSRF 和 Actor 不与 Contract Ledger 共用。

## 明确不做

- 一次请求提交多条 Allocation；
- 自动选择、先进先出或按到期日分配；
- Allocation reversal、义务调整、资金冲销、退款、跨币种或第三方代收付；
- 因余额归零自动关闭义务；
- 账户余额、账龄、预测、票据或会计凭证；
- Contract Ledger API、数据库、领域对象或连接器。

当前功能没有跨系统业务流动，因此不制造虚假的系统联调。独立验收必须在 Contract Ledger 进程停止时完成；将来出现真实 Contract → Money Flow 流动时，另立版本化契约、幂等、失败恢复和端到端联调事项。

## 验收标准

1. 使用三次独立单笔命令完成上述 A／B／X／Y 组合，三条 Allocation、三条命令和三条审计原子对应；
2. 一对多与多对一均成功，四个父事实的投影余额和总额守恒精确成立，不经过二进制浮点数；
3. 资金 A 详情只显示指向 X／Y 的两条记录，资金 B 详情只显示指向 X 的一条记录；
4. 义务 X 详情只显示来自 A／B 的两条记录，义务 Y 详情只显示来自 A 的一条记录；
5. 同一父事实下历史按时间、Allocation ID 确定倒序，另一父事实的 Allocation 不泄漏；
6. 每条历史可以进入 Allocation 详情和对应另一端详情，金额、revision、依据、Actor 与时间一致；
7. 无效父 ID 不调用 repository；父事实 404、历史数据库失败 500，均不渲染成空历史；
8. 既有幂等、超额、兼容性、并发和不可变测试继续通过；本功能不改变 schema version；
9. 隔离空库 HTTP 流程完成组合写入、双方详情下钻和数据库守恒核对；
10. Contract Ledger 停止时 Money Flow 仍可完成 migration、ready、登录、三次单笔分配和全部下钻；
11. `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过。
