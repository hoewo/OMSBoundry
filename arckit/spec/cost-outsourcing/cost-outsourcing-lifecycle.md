# 成本与外委生命周期

## 元信息

- 路径：`spec/cost-outsourcing/cost-outsourcing-lifecycle.md`
- 所属模块：`cost-outsourcing`
- 状态：🟡 进行中；领域核心、正式入口、精确引用、人类工作区、正式 D1／恢复及 C6→Money 应付建议独立接受／拒绝／故障恢复已验证，完整双 Tenant 与真实 Ubuntu 待补
- 优先级：P0
- 依赖能力：IAM、Project Workspace、Party Directory、Evidence Service；Contract／Money／Billing 按所选动作显式接入
- 上层旅程：[OA 完全体 C6](../assembly/full-reference-oa-validation.md#c6成本与外委)

## 问题 A → 可验证结果 B

**问题 A**：参考 OA 使用宽财务记录承载成本、外委付款和对方票据，又允许原地编辑和级联删除，无法稳定解释发生原因、法律依据、义务、票据与实际付款分别由谁确认。

**可验证结果 B**：用户可以在独立应用登记并纠正直接成本和外委事项，冻结精确 Project／Party／Contract／Evidence 来源；付款只作为可确认建议交给 Money，票据和实际付款只读各自所有者，所有历史、权限、故障和恢复均可验证。

## 产品边界

第一版使用一个独立 Cost & Outsourcing 应用承载两个严格聚合：

- `Cost Entry`：直接成本发生的来源事实；
- `Outsourcing Engagement`：外委工作范围、受托方、登记金额、合同引用和评分。

两个聚合共享 Tenant／Project 上下文、IAM、Evidence 和交付边界，但不共享 nullable 宽记录、状态机或纠错命令。出现第二个稳定独立交付场景或生命周期反证时，可以兼容拆分应用；当前不提前制造两个服务。

应用不拥有 Project、Party、Contract、Money Obligation／Transaction、Billing Invoice Registration 或 Evidence File 的正式事实。

## Cost Entry

### 第一版字段

- 稳定 ID、Tenant、Project ID／revision 快照；
- 类别：`vehicle|accommodation|public_transport|other`；
- 发生日期、整数 `amount_minor`、ISO `currency`；
- `distance_meters`（仅 vehicle 可选）、说明；
- 零到多个 Evidence File ID／Version 与显示快照；
- `recorded|withdrawn`、revision、创建／修改 Actor 与时间。

`recorded` 只表示成本来源登记已经确认，不表示报销、会计入账或付款。纠正形成下一完整 revision；错录或取消形成 withdrawn revision。历史不得更新或删除。

## Outsourcing Engagement

### 第一版字段

- 稳定 ID、Tenant、Project ID／revision 快照；
- 受托方 Party ID／revision、kind／legal name 最小快照；
- 委托范围、登记金额、币种和备注；
- 可选 Contract／Settlement ID、revision、状态和稳定摘要；
- 合同／说明 Evidence 业务引用；
- `recorded|withdrawn`、revision 与完整历史；
- 零到一个当前 Assessment 及其追加历史。

自由文本受托方名称不能代替 Party ID。正式合同存在时只冻结 Contract Ledger 精确引用；上传合同扫描件不创建法律合同事实。纠正 Party、范围、金额或合同引用都追加 revision，不改写旧快照。

## 评分

第一版评分规则固定为 `reference-oa-five-point-v1`：整数 1～5、理由、评分 Actor、评分时间和所评 Engagement revision。更正评分必须追加新 Assessment revision；撤回 Engagement 不删除评分历史。评分不复制到 Money、Billing 或 Contract。

## 付款建议

应用可以从精确 Cost Entry／Engagement revision 生成 `Payable Suggestion`：

- 保存来源类型、ID／revision、Party 快照、整数金额、币种、用途、Command ID 和发起人；
- 当前状态区分 `proposed|accepted|rejected|withdrawn|target_unavailable`；
- Money 独立确认并返回 Obligation ID／revision；C6 只保存确认引用；
- Money 接收后的 `pending` 在 C6 显示为 `proposed`；Money 接受时只生成一个自身拥有的 payable Obligation，拒绝时不生成 Obligation；
- 同一来源 revision 同时只存在一条未撤回建议，已接受或拒绝的建议不能靠新建议覆盖；
- 建议失败或 Money 不可用时不得伪造义务、付款或余额；
- 首次提交遇到 Money 不可用时，C6 保留 `target_unavailable` 本地事实和完整输入；恢复后由用户显式同步，既有事实不回滚也不假称送达；
- 同一 Command ID 和请求摘要幂等，不同摘要冲突。

实际付款与核销仍由 Money Flow 拥有。对方票据仍由 Billing 拥有；C6 只按精确引用展示。

## Evidence 与故障

- 成功命令只保存经过 Evidence reference-read 验证的 File ID／Version；
- 上传过程可以先于业务命令，上传失败时页面保留未提交字段；
- 任一 Project／Party／Contract／Evidence 校验失败时不写 current、revision、command 或 audit；
- 已确认 Command 重放先返回本地结果，不重新依赖外部服务；
- 依赖暂不可用与业务引用无效分别返回可重试 503 和不可重试 422；
- 已冻结历史在依赖停机时仍可读，实时下钻显示 unavailable。

## 权限

稳定动作：

- `cost-entry.create|read|amount.read|correct|withdraw|evidence.manage|history.read`
- `outsourcing-engagement.create|read|amount.read|correct|withdraw|assess|evidence.manage|history.read`
- `outsourcing-payable-suggestion.create|read|withdraw`

目标应用复验 Tenant、Project Scope、资源 Scope、当前状态和 expected revision；金额与 Evidence 独立授权，响应服务端收敛 `allowed_actions`。客户岗位只存在于 Customer Overlay，身份管理员无隐式业务权限。

## 最小组合

基础 Cost／Engagement 登记组合为 IAM／模板 + Project + Party + Evidence + Cost & Outsourcing。Contract 只在关联法律依据时加入；Money 只在提出／查看义务和付款时加入；Billing 只在关联对方票据时加入。

未选择本应用时，Workbench、Gateway、IAM Catalog、进程、migration 和备份清单都不出现 C6 项；既有 C1～C5 继续工作。

## 第一实现轮验收

验收编号与完整要求使用 [C6 责任边界审计](../../../docs/reports/oa-c6-cost-outsourcing-boundary-audit-20260928.md#c6-第一实现轮验收矩阵) 的 C6-A01～A18。领域核心、PostgreSQL 18、受保护版本化 HTTP、正式 IAM、Project／精确资源 Scope、金额／Evidence 分权、真实 Project／Party／Contract／Evidence adapter、人类工作区、独立进程、正式 D1、未选缺席和联合空目标恢复已经通过；Party 运行时专项又验证 Contract-linked Engagement 在显式 `cutover → rollback → recutover` 中按 canonical Directory Party 失败关闭或成功，并在恢复后保留映射、序列、ID、revision、快照与历史。C6→Money 跨进程验收使用独立进程和独立数据库验证精确来源、接受生成唯一 Obligation、拒绝零 Obligation、Money 停机保持 C6 事实、恢复同步、部分提交恢复和命令重放。Money 完整正式 D1 消费链、完整双 Tenant、付款／票据只读投影与真实 Ubuntu 继续作为独立证据轴。

## 不进入第一版

- 员工报销、工资、会计凭证和税务处理；
- 用户自定义成本类别、任意审批流和采购平台；
- 自动创建合同、义务、付款或票据；
- 按名称合并 Party 或删除其他应用事实；
- 无业务证据的 Project 完成规则、项目包成本分摊和专家费归属。

## 复审条件

出现以下证据时复审：Cost 或 Outsourcing 有第二个稳定独立交付场景；二者生命周期、权限或容量要求必须独立部署；真实客户提供审批／报销／采购状态样本；Money／Billing 无法用建议与精确引用满足用户结果；Project Completion 得到稳定的 C6 适用性和完成判据。
