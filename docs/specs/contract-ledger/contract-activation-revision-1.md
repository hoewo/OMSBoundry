# Contract Ledger：合同激活、签约主体快照与 revision 1

里程碑：M1  
功能编号：CL-M1-005  
状态：已接受

## 目标

用户能够把已完成签署准备的合同草稿激活。激活必须在一个数据库事务中冻结原始合同条款、签约主体资料和合同角色，形成不可变 revision 1，同时记录可追溯的激活命令与审计事实。

激活后的历史真相来自 revision 和主体快照，不再来自可变化的 Party 当前资料或草稿关联。

## 激活前置条件

- 合同状态必须是 `draft`；
- Expected Draft Version 必须等于当前 `draft_version`；
- 签署日期必填；
- 生效日期必填；
- 至少存在两个不同 Party ID 的有效签约主体；
- 每个有效关联继续满足合同内“Party + 不区分大小写角色”唯一；
- 合同总额可以暂未确定，但币种仍然必填；
- 当前功能不要求文件、审批证明、电子签章或 Money Flow 存在。

同一主体承担多个角色只计算为一个法律主体，不能满足“至少两个不同签约主体”的前置条件。

## 原子激活结果

一次成功激活必须原子完成：

1. 保存激活命令；
2. 复制当前合同基本字段为 revision 1 条款快照；
3. 按确定顺序复制全部有效签约主体关联为 revision 1 Party Snapshot；
4. 把合同状态改为 `active`；
5. 把 `current_revision` 设置为 1；
6. 保存 `contract_activated` 审计事实。

任一步失败都必须整体回滚，不能留下“状态已激活但快照不完整”等半成品。

激活消费当前 `draft_version` 作为并发前提，但不再增加草稿版本；激活后的并发控制进入 `current_revision` 体系。

## revision 1 条款快照

revision 1 冻结：

- Contract ID；
- revision 编号 1；
- 激活 Command ID；
- 激活所消费的 Draft Version；
- 编号命名空间与合同编号；
- 标题与通用分类；
- 币种与可选精确总额；
- 签署日期与生效日期；
- 激活时间；
- 激活 Actor Ref。

数据库必须拒绝对 revision 快照执行 `UPDATE` 或 `DELETE`。发现错误时只能在后续功能中追加更正或替代事实。

## 签约主体快照

每个有效草稿关联冻结：

- 确定的显示顺序；
- 来源 Draft Party Link ID；
- 来源 Party ID；
- 来源 Party revision；
- 主体类型；
- 当时法定名称或姓名；
- 当时登记标识；
- 本合同中的角色。

数据库必须拒绝修改或删除快照。激活后 Party 当前资料即使变化，也不能改变 revision 1 的主体名称、登记标识、revision 或角色。

## 激活命令与幂等

输入：

- Command ID；
- Contract ID；
- Expected Draft Version；
- 由可信应用上下文提供的 Actor Ref。

规则：

- 相同 Command ID、Contract ID、Expected Draft Version 和 Actor Ref 重放时返回原 revision 1，不重复激活或写审计；
- 相同 Command ID 改变任一负载时返回幂等冲突；
- 不同 Command ID 对已经激活或取消的合同执行激活时返回状态冲突；
- 幂等重放优先于状态检查；
- 一个合同最多存在一条成功激活命令和一个 revision 1。

## 最小激活审计

激活审计至少保存：

- Audit ID；
- Contract ID；
- 事件类型 `contract_activated`；
- Command ID；
- Actor Kind 与 Actor ID；
- 发生时间；
- Source Draft Version；
- Resulting Revision。

审计记录在数据库层不可更新、不可删除。

本功能最初验收时使用配置的 `local:<operator-id>` 作为过渡审计来源。`CL-M1-006` 已将当前运行模式替换为登录会话构造的 `local_user:<user-id>`，并把激活纳入通用不可变写入审计；迁移前已保存的 Actor 仍按原值保留。

## 页面行为

- 草稿详情在满足与不满足前置条件时都显示明确说明；
- 激活表单带 CSRF、Command ID 和 Expected Draft Version；
- 激活成功后详情显示 active、revision 1、激活时间、Actor 和冻结主体快照；
- active 合同不再显示编辑、取消、添加或移除主体表单；
- 页面读取 revision Party Snapshot，不使用 Party 当前名称替代历史快照。

## 验收标准

- 满足前置条件的草稿能原子激活并形成 revision 1；
- 缺少签署日期、生效日期或两个不同主体时明确拒绝且不产生任何激活事实；
- 过期 Expected Draft Version 被拒绝；
- 激活命令可安全幂等重放；
- 同 Command ID 改变负载、不同命令重复激活均被拒绝；
- revision 1 精确复制金额、日期和基本字段；
- 主体快照精确复制当时 Party revision、名称、登记标识与角色；
- 激活后修改 Party 当前资料不会改变历史快照；
- 应用命令不能编辑、取消或增删 active 合同数据；
- 数据库直接 `UPDATE` 或 `DELETE` revision、Party Snapshot、激活审计时被拒绝；
- 一个事务失败时状态、revision、快照和审计全部不落库；
- 页面具备 CSRF 和请求大小限制；
- 不启动 Money Flow 也能完成全部验收；
- 激活不会创建应收、应付、发票、收付款或核销记录。

## 本功能不包含

- 登录、密码、会话和多用户权限；
- 签署代表；
- 文件证据、审批证明或电子签章；
- revision 2 及后续合同变更；
- 恢复草稿或撤销激活；
- 合同事件 outbox 和外部发布；
- 自动创建 Money Flow 义务或任何跨系统写入。
