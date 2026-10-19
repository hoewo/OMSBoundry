# ADR-0010：合同核心条款更正追加新 Revision

日期：2026-09-19  
状态：Accepted

## 背景

Contract Ledger 已把激活、补充协议、终止、结算、关系、关闭和文件证据保存为不可变事实。不可变能够阻止静默篡改，却不能假设操作者永远不会录错。现有规格因此多次保留“受控纠错”入口，尤其要求 terminated 和 closed 合同仍可追溯纠错，但尚未定义如何修正当前合同核心条款。

直接修改旧 Revision 会破坏历史引用；只追加一条自由文本备注又不能给出新的可用当前快照。把错误数据当作一份新的补充协议则会混淆“真实商业变更”和“系统记录更正”。

## 决策

1. 技术性更正使用独立 `correction_instrument` 和命令，不复用补充协议命令。
2. 更正只针对提交时的 current revision，并连续形成 Revision N+1；旧 Revision 永久保留。
3. 第一版允许完整重述标题、分类、当前有效金额、合同签署日期和合同生效日期；编号、币种与 Party Snapshot 原样复制。
4. active、terminated、closed 都可更正，生命周期状态保持不变。命令同时消费 Expected Status 与 Expected Revision。
5. Correction Instrument 的日期和顺序表达更正记录进入账簿的确定位置；Revision 中被更正的合同日期仍作为独立快照字段保存。
6. 只有 administrator 拥有新增的 `business.correct` 权限；editor 和 viewer 只读。
7. 更正与其他合同写入共用 Contract 行锁，并与新 Revision、快照、命令和审计原子提交。
8. 列表、搜索和详情中的正式合同字段统一从 current revision 读取，Contract 根表只保留草稿／稳定身份数据，不形成第二真相源。

## 原因

- 新 Revision 能给后续补充、终止、结算、关系和关闭提供明确的当前输入，同时保留所有旧引用。
- 独立 Instrument 类型让审计能够区分商业变更与数据纠错。
- 只更正当前 Revision 避免插入过去顺序后偷偷重算整条法律时间线。
- Expected Status 防止关闭不推进 revision 时产生不可见竞态；允许 closed 更正又不会把合同重新开放给普通写入。
- 管理员专属权限与不可变审计降低高风险操作被普通编辑者误用的概率。

## 后果

- closed 合同可能出现“Closure 引用 revision N，而当前为纠错 revision N+1”。这是有意保留的历史：关闭当时确实基于 N，之后管理员追加了纠错；系统不得把 Closure 偷换为 N+1。
- 已有 Settlement、Relation 和 Evidence 不会自动漂移到新 Revision。需要时由各自后续的类型化纠错能力处理。
- 编号、币种和 Party Snapshot 错误暂时不能通过本功能修正。它们需要业务身份冲突、跨币种事实和主体身份方面的单独规则。
- 导出格式必须同时包含旧 Revision、更正文书、新 Revision 和旧事实引用，不能只导出当前投影。

## 被否决方案

### 直接更新旧 Revision

会使 Settlement、Relation、Closure 和 Evidence 的历史依据在没有新事实的情况下发生变化，违反不可变账簿边界。

### 把更正伪装成普通补充协议

补充协议表示双方真实改变条款，更正表示系统记录与已确认事实不一致。二者权限、审计解释和后续统计不同，不能只靠自由文本区分。

### 只记录自由文本更正备注

备注无法形成新的结构化当前快照，后续业务仍会继续消费错误字段。

### 建设万能纠错表和 JSON Patch

万能补丁无法稳定执行字段校验、引用保护和状态规则，也会绕过每类业务事实应有的反向语义。
