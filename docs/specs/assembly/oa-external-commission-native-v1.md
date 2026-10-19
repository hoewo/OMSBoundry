# 原 OA 对外委托原生闭环 v1

状态：2026-10-06 已接受边界；主路径私有真实入口 10／10 已通过，扩展权限／停机矩阵仍待补。  
适用范围：统一工作台“生产信息 → 对外委托”原列表、统计、抽屉、合同文件、评分和支付展示。  
关联决策：BF-118。  

问题 A → 可验证结果 B：原 OA 的对外委托页面把供应商文本、联系人、合同文件、评分和可删除重建的支付宽记录放在一个页面动作里，而目标组合必须保持 Cost、Party、Contract、Evidence 与 Money 各自所有权 → 用户仍能在同一原界面完成委托登记、纠正、文件预览、评分和应付流转，但每个长期事实只写入唯一 Owner，局部成功可续办，既有事实不因另一 Owner 失败或页面重载而丢失。

## 1. 原 OA 实际动作

只读审计原实现确认以下可见动作：

1. 列表显示受托方、联系人、委托内容、金额、合同文件、评分和支付记录；
2. 顶部统计委托总数、有效委托总金额和平均评分；
3. 抽屉新增或编辑联系人、内容、金额、合同附件、备注和多条支付日期／金额；
4. 合同文件可预览、下载和从表单移出；
5. 委托可删除；
6. 编辑保存时，原实现删除该委托全部旧 `FinancialRecord` 后按页面数组重新创建；旧数据缺少委托 ID 时还会按供应商名称兜底关联。

第 6 项是参考现状，不是目标语义。它会把“页面编辑”变成跨域删除重建，无法可靠表达已确认应付、实付、核销和历史，也可能误关联同名供应商。

## 2. 唯一事实所有者

| 页面语义 | 唯一 Owner | 保存内容 | 明确不拥有 |
| --- | --- | --- | --- |
| 委托事项 | Cost & Outsourcing | Project 快照、Provider 快照、发生时联系人快照、范围、精确金额、合同精确引用、Evidence 引用、备注、状态、revision、历史 | Party 当前主档、合同法律事实、文件字节、应付确认、实付 |
| 受托方当前档案 | Party Directory | Party 稳定 ID、当前名称／类型／状态／revision | 本次委托联系人、金额、合同、付款 |
| 法律合同 | Contract Ledger | 合同／变更／结算及其 revision | 页面附件字节、委托状态、实付 |
| 合同附件字节 | Evidence Service | File ID、Version、摘要、对象字节与短期访问 | 委托业务含义、合同法律状态 |
| 评分 | Cost & Outsourcing | 追加式 Assessment、规则键、分值、原因、engagement revision | Party 全局信用分、任意覆盖旧评分 |
| 应付款建议 | Cost & Outsourcing | 来源委托、Provider、金额、到期日、用途、revision、状态和向 Money 交付历史 | Money 义务或实付事实 |
| 应付义务／实付／核销 | Money Flow Ledger | 独立确认后的 Obligation、Cash Transaction、Allocation 与余额 | 委托正文、Party 主档、合同字节 |
| 页面装配 | Unified Workbench | 最长会话期的待续子命令、已上传 File Version 和短期读取缓存 | 任何长期业务事实或跨来源合计真相 |

## 3. Cost 小系统的有意进化

### 3.1 发生时联系人快照

`Outsourcing Engagement` 增加可选 `contact_name` 与 `contact_phone`。它们表达“这次委托联系谁”，随 engagement revision 追加保存；不是 Party Directory 的当前联系人主档，不反向同步 Party，也不建立通用联系人管理。

该变化属于 Cost Base 的类型化可选字段，而不是只为一个客户存在的任意 JSON：

- 新旧客户端兼容：字段可选，旧快照缺失时读为空；
- 新写入统一 trim，姓名最多 100 个 Unicode 字符，电话最多 50 个 Unicode 字符；
- 纠正必须产生新 engagement revision，历史快照保留旧联系人；
- 列表和历史的公共 HTTP 视图返回相同字段；
- PostgreSQL 当前以版本化 JSON snapshot 保存聚合，故无需新增业务列，但导出、备份和恢复必须原样保持字段。

### 3.2 评分与合同文件

- 评分复用 Cost 已有 append-only Assessment；页面平均分只从每条委托最新有效 Assessment 计算，不写第二份平均分；
- 合同业务引用可同时包含精确 Contract revision 与 Evidence `outsourcing_contract` 槽。两者含义不同：Contract 是法律事实，Evidence 是文件字节；没有正式 Contract 时允许只保存附件，但页面必须显示“仅附件，未连接法律合同”，不能冒充已签合同；
- 从表单移出文件只在下一 engagement revision 中移除业务引用，不删除 Evidence 字节。

### 3.3 支付拆分

原页面“支付记录”在 v1 拆成两个清晰阶段：

1. Cost 基于 engagement 提出 Payable Suggestion；
2. Money 独立接受／拒绝，并在接受后拥有义务；实际现金支付与核销仍由 Money 的原生命令完成。

页面可以并列显示建议状态、Money 目标引用和已存在的实际支付投影，但不能把 suggestion 标成“已支付”，不能按供应商名称猜关联，不能在编辑委托时删除或重建 Money 事实。

## 4. 依赖方向与时机

| 发起方 → 目标 | 时机 | 类型 | 失败语义 |
| --- | --- | --- | --- |
| Cost → Project | 新建委托 | 写入时精确引用校验 | Project 不可用则零 Cost 写入；同 command 可重放 |
| Cost → Party | 新建／纠正委托 | 写入时精确引用校验并冻结快照 | Party 不可用时已有委托仍可读，新写失败关闭 |
| Cost → Contract | 带正式合同时的新建／纠正 | 写入时精确引用校验并冻结快照 | 不可用时不得把未经校验引用保存成正式合同；允许用户明确改为仅附件 |
| Cost → Evidence | 带文件的新建／纠正 | 逐 File／Version 精确校验 | 已上传字节保留，续办 engagement 子命令，不重复上传 |
| Cost → Money | 交付应付建议 | 版本化建议交付 | Cost 保留 proposed／target_unavailable；Money 恢复后 sync，不回滚委托 |
| Workbench → Owners | 页面读写 | 原生 Capability Credential 与只读组合 | 每个来源独立 unavailable／forbidden；不显示为空记录或零金额 |

普通列表复读不得同步回查 Project、Party、Contract 或 Evidence；Cost 已冻结足以解释委托的快照。合同预览／下载只在用户点击文件时请求 Evidence。Party 目录只在打开可编辑抽屉或有创建／纠正能力时读取。

## 5. 一个按钮的可恢复子命令

新增／纠正保存最多包含以下有序子命令，每段都有稳定身份：

1. 新文件上传完成并得到 File／Version；
2. Cost engagement 新建或纠正；
3. 可选 Assessment 追加；
4. 可选 Payable Suggestion 新建；
5. 可选 suggestion 向 Money 同步。

任一后段失败时保留前段已确认事实；会话存储只保存待续命令 ID、已确认 File／Version 和必要输入摘要。未知回复必须以原 command ID 查询／重放，不能生成新命令重复登记。发生 revision 冲突时失败关闭并要求刷新核对，不自动覆盖。

## 6. 原界面复用要求

- 继续使用原 `external-commission__*` 样式层级、三项统计、列表行、880px 抽屉、合同文件预览和支付区；不重新发明另一套卡片页面；
- 首次加载先显示原结构骨架；同 Project 返回 Tab 在 30 秒 fresh 窗口内新增 Cost 读取为 0，显式刷新才重读；
- 列表可在 Party／Contract／Evidence／Money 暂时不可用时显示 Cost 已拥有的委托快照；
- 没有金额能力时总金额和单行金额均显示“未授权”，不能显示 0；没有历史／评分／建议能力时保留布局并明确禁用原因；
- “撤回”替代物理删除；历史、附件引用和 Money 事实仍可审计。

## 7. 验收矩阵

关闭 v1 至少需要从全新夹具实际运行：

1. 原组件结构、空态、骨架、统计和抽屉可见；
2. Party 选择、联系人、精确金额和备注写入后从 Cost 复读；
3. 合同文件直传 Evidence，预览／下载字节一致；移出仅结束引用，字节仍在；
4. 可选 Contract 精确 revision 冻结，Contract 后续变化不改写历史；
5. 评分追加，旧评分历史保留，平均分来自最新 Assessment；
6. suggestion 创建、Money 独立接受／拒绝，页面不把未支付义务冒充支付；
7. 纠正、撤回、历史和同 command 重放；服务端成功但回复丢失不重复记录；
8. Party／Contract／Evidence／Money 分别停机的零错误写入与恢复续办；
9. 正式 IAM 撤权、跨 Tenant／Project 拒绝及金额脱敏；
10. 返回缓存、并发合并、显式刷新请求上界和迟到响应隔离；
11. Cost、Party、Contract、Evidence、Money、Project 逻辑空目标恢复后 ID／revision／摘要／字节保持；
12. Project／Production／Contract 边界摘要验证：Production 请求和写入为 0；Project 除新建边界精确校验外零写；Contract 仅在显式选择正式合同时精确读且零写。

## 8. 明确延后与不负责

- v1 不建设 Party 通用联系人目录；
- v1 不在 Cost 内实现现金支付、核销或余额；
- v1 不把页面 payment 数组迁成可删除重建的 Money 宽记录；
- v1 不建设任意表单／任意审批／任意工作流 DSL；
- 若 Money 当前缺少从已接受 suggestion 到真实 Cash／Allocation 的完整原界面操作，页面必须标“义务已确认，实付未接通”，不得伪造；
- Main／公网、正式 D1、双 Tenant、全角色和第二客户必须分别留下运行证据后才能提升整体判定。

## 9. 边界判定

本规格只接受 Cost 边界的一次有意演进：委托发生时联系人快照。Project 不增加字段或状态；Production 不拥有对外委托；Contract 只提供精确法律事实；Party、Evidence、Money 的所有权不变。即使全部 v1 验收通过，也只能把“对外委托组合”记为当前 OA 切片下的候选／可重复证据，不能宣称跨客户稳定。

## 10. 2026-10-06 实际证据

私有 Nginx → Gateway → 正式 IAM／Policy → 八事实 Owner／PostgreSQL 18 → Chrome 从全新夹具实际通过 10／10：原 UI、回执丢失同 command 重放、Party／联系人／Contract／Evidence／金额冻结、真实文件预览下载、评分与纠正、Cost 建议 → Money 独立接受 → 显式同步、历史／撤回、返回缓存零读／显式刷新一读、八库空目标恢复、IAM 撤权只读、精确单资源工作负载 Scope 和零页面错误。

该证据覆盖验收矩阵的主路径及恢复边界，但没有把 Money 拒绝、真实 Cash／Allocation、各依赖逐一停机、跨 Tenant／相邻 Project、金额脱敏和全角色全部跑进同一浏览器，因此状态不是“完整矩阵关闭”。运行范围、三重点系统边界和限制见[验证报告](../../reports/oa-external-commission-native-validation-20261006.md)。
