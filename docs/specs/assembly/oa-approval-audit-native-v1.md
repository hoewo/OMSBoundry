# OA 批复／审计原生闭环 v1

状态：私有原生闭环已验证（2026-10-06）；Main／完整 OA 未验收

## 问题与可验证结果

参考 OA 的“批复审计”页面已经可见，但现有装配把金额和文件引用始终发送为空，纠正会丢失既有引用，审计来源类型也不符合 Results 契约。要把“可看的双列页面”变成“可完成、可复读、可恢复的批复→审计业务闭环”，同时不得把 Project、Contract、Evidence 或 IAM 的事实吞进 Workbench。

可验证结果：在一个精确 Project Scope 内，用户可沿用原 OA 双列界面完成批复登记、审计登记、金额引用或人工覆盖、多个报告文件、追加纠正、撤回和历史复读；刷新、重新登录和恢复后仍由各所有者重建相同事实，隐藏页面不请求数据。

## 原 OA 事实与稳定映射

| 原 OA 表面语义 | 新组合中的所有者与实现 |
|---|---|
| 批复／审计双列、报告文件、三项金额与合计 | Workbench 复用原 OA 布局；Results 返回当前投影 |
| 批复引用当前合同／结算金额 | Results 冻结 Contract Ledger 精确资源 ID + revision + 金额快照 |
| 审计引用当前有效批复 | Results 冻结自身 approval 的精确 ID + revision；不再次读取 Contract |
| 手工调整金额 | Results 使用整数分并要求非空覆盖原因；页面不以浮点数重算事实 |
| 多个报告文件 | Evidence 独占 File ID、Version、字节与访问；Results 只冻结引用、slot、摘要和显示名 |
| 编辑已有记录 | 追加 correction revision，不覆盖历史 |
| 删除记录 | 业务上使用 withdraw revision；文件不物理删除 |
| 查看历史 | Results 自有 revision 历史；Workbench 只读呈现 |

原 OA 的自由备注当前不进入本闭环：Results v1 尚未拥有通用备注字段，不能把备注偷塞进确认人、覆盖原因或 Project。是否作为 Results 的兼容新增字段，必须在真实使用证据出现后单独演进。

## 不变量

1. Project 只提供当前项目身份、active 状态、revision 和资源连接；不拥有批复／审计金额或报告。
2. Contract Ledger 只提供指定 contract／settlement revision 的来源快照；Results 不读其数据库。
3. Results 独占批复／审计当前事实、金额快照、来源快照、Evidence 引用和不可变历史。
4. Evidence 独占文件字节、版本、上传会话和短期访问地址；移出 Result 引用不删除文件。
5. IAM 只签发 `approval-audit-results` audience 的原生 capability credential；Results 仍结合 Project／Result scope、状态和 `allowed_actions` 最终授权。
6. Workbench 只保存可恢复的命令意图和已完成但未登记的 Evidence 版本；不保存业务真相。
7. 金额使用精确整数分；金额与冻结来源不同时必须填写覆盖原因。
8. 审计来源必须是 `approval-audit-results / approval-result / approval ID / exact revision`。
9. 读取不得产生写入；隐藏 Tab 不读取；重复点击或未知回复必须复用原 command ID。

## 失败与恢复

- Evidence 上传完成、Results 命令尚未确认：保留待登记文件版本和原命令；重新进入后可继续，不重复上传。
- Results 写成功但响应未知：重试同一 command ID，由所有者幂等回放。
- revision 冲突：停止自动重试，要求重新读取和人工核对，不覆盖新事实。
- IAM 失权：已有获权事实仍可按 read/amount.read 脱敏复读；写按钮立即由 owner-native `allowed_actions` 关闭。
- Project／Contract／Evidence 不可用：Results 失败关闭，不落部分结果。

## 验收

1. 原 OA 双列页面不再显示样例金额或样例文件。
2. 批复从精确 Contract／Settlement revision 登记并复读三项金额。
3. 审计从精确 approval revision 登记，且不调用外部 Contract source resolver。
4. 人工金额差异无原因被拒绝，有原因成功且合计精确。
5. 多文件上传、预览、下载、移出引用成功；Evidence 字节未被 Results 或 Workbench 复制。
6. 纠正保留原来源、金额和文件，除非用户明确修改；revision 只增一。
7. 撤回、重新登记和历史按原语义可见。
8. 去掉写能力后所有写按钮不可用，零写入；amount.read 缺失时金额脱敏。
9. 一次页面读取没有轮询风暴或跨系统逐请求授权链。
10. D1 备份恢复后 Project／Contract／Results／Evidence 各自摘要和引用一致。

实际私有验收进一步覆盖了审计撤回后同 aggregate 重登记、空 Evidence 集合稳定输出、回执丢失同 command 重放、页面缓存请求上界、六所有者逻辑空目标恢复和正式 IAM 写能力撤销。最终 11／11，详见[验证报告](../../reports/oa-approval-audit-native-validation-20261006.md)。本批没有执行 Project Completion Checkpoint，不以 Results 页面闭环冒充项目完成联动已通过。

## 明确不负责

- 不把客户岗位名写进 Results 权限条件。
- 不由 Results 推断合同是否中标、付款或项目完成。
- 不在本闭环引入 Bonus、Money 或 Production 写依赖。
- 不借此建立通用表单、通用流程或万能附件服务。
