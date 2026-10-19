# ADR-0027：经营汇总是按本方主体与币种隔离的 current 投影

日期：2026-09-20  
状态：接受

## 背景

Money Flow 已拥有可以重建当前 Obligation outstanding、Cash current effective 和 Cash unallocated 的不可变事实，但用户仍需分别打开义务与资金列表才能判断当前应收、应付、实际收付和未分配资金。M4 计划要求汇总能够下钻，同时不能建立另一套可写余额、混合币种，或把经营台账冒充会计和银行系统。

退款、坏账和票据均因真实语义不足而延期。当前汇总只能诚实反映已经实现的事实，不能为追求一个看似完整的数字擅自补入尚不存在的净额。

## 决策

- 新增只读 Current Operating Summary，不保存汇总表或可写金额；
- 每个汇总分组使用 `ledger owner Party ID + currency`，显示当前 Party 名称但不以名称合并身份；
- 第一版固定六项指标：receivable outstanding、payable outstanding、receipt effective、payment effective、receipt unallocated、payment unallocated；
- Obligation 指标复用 `effective - net allocated - remitted`；Cash 指标复用 Cash Reversal 后的 current effective 与 `effective - net allocated`；
- 每项只统计正贡献事实并提供事实数量和类型化下钻；下钻总额必须由页面返回的同一批贡献事实精确求和；
- 汇总与每次下钻分别使用 Money Flow PostgreSQL 的只读 `REPEATABLE READ` 事务，并显示各自生成时间；
- 金额按六位小数定标整数校验与求和，禁止浮点数；负余额、超精度或非法存量金额导致整个请求失败；
- Internal Transfer、草稿、退款、坏账、票据、Contract、银行余额、会计收入和跨币种换算全部排除。

详细规格见 [当前经营收支汇总与逐项下钻](../specs/money-flow-ledger/current-operating-summary.md)。

## 结果

优点：

- 用户可以从当前总量直接下钻到已验证事实，不需要维护第二套余额；
- 业务主体和币种边界显式，避免不同账簿或币种相加；
- 汇总公式与现有详情页一致，Allocation Reversal、Remission 和 Cash Reversal 自动反映；
- 延期能力不会以隐含假设污染当前数字。

代价：

- 当前页面不是历史报表、会计报表或银行余额；
- 退款实现前，actual 指标只能明确称为“当前有效实际收付款”，不能称为退款后的净现金；
- 汇总与稍后点击的下钻可能因期间形成新事实而不同，页面必须显示各自生成时间。

## 否决方案

### 保存可编辑汇总字段

否决。它会形成第二真相源，且难以与追加事实、Reversal 和 Remission 保持原子一致。

### 把所有主体或币种合成一个总数

否决。账簿所有者决定应收／应付方向，币种之间没有已定义汇率；直接相加没有稳定业务意义。

### 使用普通浮点数在页面求和

否决。金额必须与数据库精确事实一致，不能让展示层引入舍入差异。

### 把 Internal Transfer 或延期退款计入实际收付

否决。Internal Transfer 非经营收付；退款目前没有完整事实模型，隐式扣除会制造不可追溯数字。
