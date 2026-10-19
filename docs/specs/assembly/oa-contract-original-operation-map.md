# 原 OA 两链合同操作映射

问题 A → 可验证结果 B：只读 Contract Link 与空合同页不能完成 OA 经营流程 → 同一项目复用原合同工作面，基础合同、协议、定案、文件和后续资金各自保存并可复读，失败只续办未完成步骤。

本映射来自原 OA `ContractAndAmendments.tsx`、`ContractForm.tsx`、`ContractAmendmentForm.tsx`、`ContractFileList.tsx` 及 `contractAmounts.ts`，参考提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`。原目录只读。2026-10-04 已匹配发布 Contract schema 22 与 Workbench `.5`；同一 C 的管理员有限公网链实际完成两链保存／刷新、原始合同确认、三类协议、阶段／最终结算及一个真实文件上传／预览／字节下载。历史区截图与原生边界收尾分别通过。最终更正、全角色／冲突、三类多材料公网链和完整连续旅程仍未关闭；本表不是整段等效声明。实际失败及复验见[公网报告](../../reports/oa-commercial-public-ui-validation-20261004.md)。Contract 原生端口另见[端口规格](../contract-ledger/oa-native-command-port-v1.md)，客户草稿见[扩展规格](../contract-ledger/reference-oa-contract-drafts-v1.md)。

## 原操作必须保留

| 原 OA 操作／字段 | 目标映射与当前事实 | 不允许的捷径 |
| --- | --- | --- |
| 甲方合同、京宏合同两列，分别新建／打开 | 两个明确客户链标识；Project 自有 Resource Link 的 relation key 区分，同一 Contract ID 不在无声明时合并到两链；原 780px 编辑、1360px 预览复用 | 把当前所有合同汇成一列，或以未知链默认甲方链 |
| 合同编号、签订日期 | Contract 原生编号 namespace／number／signed_on；草稿可保存，正式生效仍受原用例检查；日期使用日历日期字符串 | 用项目编号充当合同编号、自动用签订日期填生效日期 |
| 设计／勘察／咨询费，总金额自动计算 | 客户专用费用属于版本化 Reference OA Overlay；已发布 `stringMode`、整数分相加与 Contract 自有持久化端口，Base 仅拥有 `total_amount`。公网原始合同 1.01＋0.09＋0.90＝2.00，确认协议后有效额 2.02，原分项不被改写 | 放任浮点相加，或只存总额丢掉分项 |
| 合同费率百分比、可空与零 | 客户 Overlay 显式 nullable 整数基点，0% 不等于未提供；不改变 Base 金额或把费率当通用税率 | 使用 truthy 判断丢掉零，自动乘出付款义务 |
| 原始合同保存后开放协议及定案 | 保存 Contract 后保留返回 ID／draft version，再完成 Project Link；Link 当前已有 contract 草稿投影，revision 明确取 DraftVersion／正式 CurrentRevision，不需要假装草稿已激活 | Link 故障后重建合同，或将 UI 保存成功解释成正式激活 |
| 正式两方主体及生效条件 | Base Contract 使用既有自有 Party 及合同内角色快照；甲方／资质单位／公司上下文须显式选择或声明的配置映射，缺上下文先阻断正式激活；Party 统一迁移尚未完成 | 名称匹配自动创造主体，文本甲方自动成为 Party，假称全部已统一到 Party Directory |
| 补充协议：调整、修订、无金额变化 | Overlay 保留分项语义；Base `changes` 追加事实，金额分别是差额／目标总额减当前有效额／零。明确保存草稿和确认后的更正区别 | 修改原 confirmed Change、按签订日期重算确认历史，或将每种协议都当正向增额 |
| 结算随合同变化，或手工结算 | 前者是 Overlay 的可编辑意图／预览，不是 confirmed Settlement；明确定案后才冻结当前来源 revision 与金额、调用原生 settlement，后续更正追加 | GET 时写结算，把定案附件上传等同结算确认，静默覆盖旧结算 |
| 原合同文件、多份协议文件、定案文件，预览／下载／移出 | Evidence 保存 bytes／File ID／Version；Contract schema 22 已发布材料引用／追加历史。实际 IAM＋Evidence 三类材料／停机／应用库＋字节空目标恢复 m 属 private/local；公网另实际完成一个原合同文件上传、登记及字节精确下载，不能冒充三类材料全部公网通过；不能搬旧 fileService 到公共内核 | 共享 files 表、放 OSS SDK／长密钥进业务应用、失败时删除物理文件 |
| 删除合同或协议 | 原弹窗保留；草稿只能走原生 cancel，有理由／幂等／历史；正式合同／协议不能映射成物理删除，需显式撤回／反向事实能力及可验证状态 | HTML delete 直接调用数据库，或把 confirmed 历史当草稿覆盖 |
| 只读、刷新、取消、保存后关闭 | 原组件权限适配至 native allowed_actions／allowed_operations；全权和只读均实测，读取／取消零写，未知回复保留 scoped 原命令 | 一个 `canManageBusinessInfo` 放行所有动作，或拿按钮隐藏作为服务端权限 |

## 保存链及失败继续

先验证完整类型化输入 → 保存 Contract 的草稿／Overlay → 保留原生 ID 与回执 → 登记 Project 自有 Link → Evidence 上传并完成校验 → 登记 Contract 业务材料引用 → 原生复读并更新组件。材料也可以先由 Evidence 完成再登记，但两者的成功不能混报；次序须在实际实现中固定并记录。

每段只有一个稳定 command ID／expected version。首次 Contract 成功而 Project Link 失败，显示“合同已保存，项目关联待完成”，只重放原 Link；Evidence 已完成而材料引用失败，保留 File ID／Version，只续登记；回复未知时重放原命令，不重新创建或上传。关闭／重新打开原抽屉必须恢复已保存 ID 和 scoped 待办。409 不自动换版本，401／403 不降级到模板身份，关联读取故障不当空列表。

原 OA 的“先创建、再上传、再 update”和 catch 中清理文件不能照搬：首次创建成功但未上传新文件时原代码可能不保留 ID；物理清理会毁掉可恢复文件；已确认记录的 update 会违反不可变历史。视觉层复用，数据／权限端口和这些保存机制做必要最小修正，并登记来源差异。

## 本批已具备和必须先实现的缺口

已具备并发布：Contract JSON 端口、正式 IAM 精确 Scope、原领域草稿／激活／协议／结算、原生草稿引用投影，以及 Project 有界 relation key 的自有 Link。候选 API 真库与实际公网原表单分别记账。

2026-10-04 已补类型化 Overlay 分项／费率／两链持久化、草稿协议、正式主体显式选择、协议→Instrument 不可变确认关联、三类业务材料及原生阶段／最终结算。旧 schema 18→22、匹配发布与有限管理员同项目正式入口已验；阶段 1.01 与最终 2.02 冻结双方、来源正式版本 4 和确认日期，取消确认零新增，原合同与三协议历史保留。仍须最终结算后追加更正、全角色／冲突和更多原材料／失败场景，不能以一条管理员链关闭 B04～B06。扩展由事实所有者拥有，不建万能元数据服务；sessionStorage 只暂存 scoped 待办，不充当业务数据库。历史分层证据见[候选报告](../../reports/oa-contract-draft-ui-candidate-20261004.md)，当前正式入口见[公网报告](../../reports/oa-commercial-public-ui-validation-20261004.md)。

验收顺序：同一 C 两链分别新增 → 关闭／刷新复读字段 → 相邻链不能混用 → 草稿修改／取消 → 正式确认 → 三种协议 → 定案与更正 → 多材料与字节预览 → 故意丢回复、Link／Evidence 故障续办 → 只读及反向授权 → 旧事实保持、升级／恢复。任何段未接通仍标未完成，不能仅放出原表单壳关闭 B04～B06。
