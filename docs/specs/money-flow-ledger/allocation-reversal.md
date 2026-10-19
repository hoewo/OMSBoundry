# Money Flow Ledger：Allocation 完整反向核销

里程碑：M4  
功能编号：MF-M4-001  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够纠正一条错误的 Allocation：追加一条引用原 Allocation 的不可变 Reversal，完整释放这条 Allocation 占用的资金可分配额和义务余额。原 Allocation 不更新、不删除；列表、Allocation 详情、资金详情和义务详情都能同时看见原分配与反向事实，当前余额由两类事实净额重建。

本功能只处理一条 Allocation 的一次完整反向核销。若正确金额小于原金额，操作顺序是“完整反向原 Allocation，再新建一条正确 Allocation”，不在原事实上做部分修改。

## 核心建模决策

- Reversal 是对一条已经存在的 Allocation 的纠错事实，不是负数 Allocation、Cash Transaction、Obligation Adjustment、退款或会计凭证；
- 一条 Allocation 最多有一条 Reversal；Reversal 金额、币种、Cash Transaction、Obligation 和 Obligation revision 必须精确复制原 Allocation，用户不能提交或覆盖；
- 原 Allocation 永久保留。没有 Reversal 时派生状态为 `active`，存在 Reversal 时派生状态为 `reversed`；状态不是可写字段；
- 当前 allocated amount 等于 Allocation 正金额之和减去 Reversal 金额之和；unallocated／outstanding 继续由根事实金额减去 allocated amount；
- 反向后释放的资金和义务容量可以再次形成新的 Allocation，但原 Allocation 与 Reversal 继续出现在历史中；
- 本轮不要求 Obligation 仍处于原 revision 或重新校验原分配的方向／主体／币种。Reversal 纠正的是已经发生的关系，只依赖原 Allocation 的冻结事实；
- Reversal 与新的 Allocation 在同一 Cash Transaction／Obligation 上固定按“资金、义务”顺序加锁，避免并发释放和重新分配破坏净额不变量。

详细依据见 [ADR-0017](../../adr/0017-allocation-reversal-is-an-immutable-full-offset.md)。

## 业务事实

- Reversal ID、Command ID 和 Audit ID 是三个不同 UUID；
- 反向原因必填，去除首尾空白后长度为 1 至 200 个字符；它说明为什么纠正，不替代 Evidence 或外部审批；
- 来源类型固定为 `manual`；
- `reversed_at` 由服务端可信 UTC 时钟生成，客户端不能提交或覆盖；
- Reversal 冻结原 Allocation 的 Cash Transaction ID、Obligation ID、Obligation revision、精确金额和币种；
- 反向金额严格等于原 Allocation 的正金额，数据库继续使用 `numeric(24,6)`，领域和页面不经过二进制浮点数；
- 新 Reversal 立即生效且不可更新或删除；本轮没有草稿、撤回 Reversal、Reversal 的 Reversal 或物理清理。

## 唯一余额模型

```text
cash allocated amount
  = SUM(Allocation.amount) - SUM(AllocationReversal.amount)

cash unallocated amount
  = CashTransaction.amount - cash allocated amount

obligation allocated amount
  = SUM(Allocation.amount) - SUM(AllocationReversal.amount)

obligation outstanding amount
  = current Obligation original amount - obligation allocated amount
```

- 每条 Reversal 与原 Allocation 等额且每条 Allocation 最多反向一次，因此净 allocated amount 不会因为 Reversal 小于 0；
- 新 Allocation 的应用层余额校验和数据库插入触发器必须使用相同净额公式，不能只修改查询页面；
- 列表与详情中的历史保留原金额；派生余额只对有效净额求和；
- 本轮只提供当前投影，Reversal 时间戳为后续 `as_of` 历史投影保留事实基础，但不提前实现历史时点查询。

## 用户流程

```text
管理员登录
  → 打开 Allocation 详情
  → 核对原资金、义务、金额和当前 active 状态
  → 填写反向原因并确认
  → 服务端从会话构造可信 Actor
  → PostgreSQL 先解决 Command 重放
  → 固定锁定原 Cash Transaction、Obligation 和 Allocation
  → 校验原 Allocation 存在且尚未反向
  → Reversal、命令和审计在同一事务提交
  → 返回 Allocation 详情，状态派生为 reversed
  → 资金和义务余额恢复，历史仍保留原分配与反向信息
```

## 幂等、并发与数据库保护

- 每次反向意图携带 UUID Command ID；
- 幂等负载由标准化后的 Allocation ID、反向原因、来源类型和可信 Actor 身份计算 SHA-256；
- 相同 Command ID 与相同负载串行或并发重放返回原 Reversal，不重复释放余额或写审计；
- 相同 Command ID 改变 Allocation、原因或 Actor 返回显式幂等冲突；
- 不同 Command ID 串行或并发反向同一 Allocation 时恰好一个成功，另一个得到包含既有 Reversal ID 的显式状态冲突；
- Reversal repository 先读取不可变 Allocation 定位两端，再固定锁定 Cash Transaction、Obligation 和 Allocation；Apply repository 继续使用相同前两把锁；
- PostgreSQL Reversal 插入触发器按同一锁序，校验复制的两端、revision、金额与币种完全等于原 Allocation，并拒绝第二条 Reversal；
- Allocation 插入触发器升级为净额计算，直接 SQL 也不能把已反向金额继续当成占用余额；
- Reversal、创建命令和审计原子提交；缺失 Allocation、重复反向、数据库或审计失败均不留下部分事实；
- Allocation、Reversal、两类命令和审计全部不可 UPDATE／DELETE。

## 权限、审计与错误边界

- GET `/allocations/{id}` 继续显示详情；POST `/allocations/{id}/reverse` 只允许 Money Flow 本地 administrator；
- POST 必须通过 CSRF；Actor 只来自登录 Principal，客户端提交的金额、币种、两端、revision、状态、Actor 和时间全部无效；
- `allocation_reversed` 审计保存 Command ID、可信 Actor、Reversal ID、原 Allocation、Cash Transaction、Obligation、revision、精确金额、币种和原因；
- Allocation ID 非法或事实不存在返回 404；验证错误返回 422；Command 冲突或已经反向返回 409；数据库失败返回 500；
- 详情读取失败不能把已反向 Allocation 渲染为 active，也不能丢失 Reversal；
- 本功能只访问 Money Flow 数据库，不访问 Contract Ledger。

## 明确不做

- 部分 Reversal、负数 Allocation、编辑或删除 Allocation；
- 撤回 Reversal、Reversal 的 Reversal、链式冲销；
- Cash reversal、退款、Cash Transaction 更正或 Internal Transfer 冲销；
- Obligation adjustment、减免、坏账、关闭或 revision 2；
- 自动重建正确 Allocation、批量反向或自动匹配；
- 票据、账龄、`as_of` 查询、统计报表、导入导出；
- 文件 Evidence、审批、会计总账、跨币种或 Contract 连接器。

## 验收标准

1. administrator 可以从 active Allocation 详情追加完整 Reversal；原 Allocation 保留，详情和各历史页面显示 `reversed`、原因、可信 Actor 与时间；
2. Reversal 的两端、revision、金额和币种只能从原 Allocation 复制，客户端伪造字段无效；
3. 反向前后的 Cash allocated／unallocated 和 Obligation allocated／outstanding 使用精确净额恢复，根事实金额不变；
4. 反向释放的容量可以被一条新 Allocation 使用，余额仍不超额；原 Allocation 与 Reversal 保留在历史中；
5. 一条 Allocation 最多完整反向一次；不同 Command 串行或并发尝试时恰好一个成功；
6. 相同 Command ID／负载串行或并发重放只形成一条 Reversal、一条 Command 和一条审计；改变负载或 Actor 返回冲突；
7. 无效或缺失 Allocation、空或过长原因、重复反向、审计失败和数据库失败均不留下半成品；
8. Reversal、Command、原 Allocation 和审计不可更新或删除；直接 SQL 复制错误的两端、revision、金额或币种被数据库拒绝；
9. Allocation Apply 的应用层与数据库超额校验都改用净额；反向与新分配并发时最终成功事实满足两侧余额范围；
10. Allocation 全局列表、资金历史和义务历史继续保留确定排序，并能区分 active／reversed；详情读取错误不伪装为 active；
11. HTTP 覆盖登录、administrator、CSRF、可信 Actor、成功、重放、冲突、404、422 和数据库失败；
12. migration 从 `000008` 升至 `000009` 并可重放，既有 Allocation 初始均为 active，原 Party、Obligation、Cash Transaction、Internal Transfer 和余额保持不变；
13. 隔离空库真实 HTTP 流程完成 bootstrap、Party、Obligation、Cash Transaction、Allocation、Reversal、余额恢复和重新分配；
14. Contract Ledger 停止时上述 Money Flow 流程和 ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器联调；
15. `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过。
