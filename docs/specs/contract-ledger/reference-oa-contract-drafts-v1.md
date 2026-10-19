# 参考 OA 合同客户草稿扩展 v1

2026-10-04，候选，未部署。问题 A → 可验证结果 B：原 OA 两类合同需要保留分项费用、零费率和多份可编辑协议，而现有 Base 只有独立合同事实 → 在 Contract 自有库保存有界、版本化的客户草稿，复用原表单，并明确区分草稿预览与正式合同、变更和结算。

此扩展沿用 BF-003／BF-016／BF-017／BF-084／BF-093／BF-098，不改变数据所有权或固定入口链。它不是万能字段表；Project 仅拥有 Resource Link，Workbench 仅组织版本化操作，Evidence 继续拥有字节和技术访问。原始合同主体／日期／显式生效、协议正式确认、三类材料及阶段／最终结算已候选接线；最终更正／正式联合与发布仍未完成，不关闭 B04～B06。

## 类型与不变量

`referenceoa.Draft` 固定 schema_version 1、Contract ID、Tenant ID、Project ID、两链枚举、revision、设计／勘察／咨询费、nullable 整数基点、结算意图和最多 100 份协议草稿。两链仅为 `client_to_qualification` 与 `qualification_to_our_company`；创建后项目和链不得更换。金额为 CNY 十进制字符串、整数分运算，0% 与未提供不同，日历日期不转换为时区时间。协议仅有 adjustment、revision、no_amount_change 三类，取消保留 tombstone，不能复活或从历史快照移除已有 ID。

Contract migration `000020_reference_oa_contract_drafts.sql` 增加本库当前投影与不可变命令历史，引用本库 Contract，不建立跨库外键。保存需要原生 `contract-ledger.draft.manage`、Tenant 或精确 Contract Scope、可信 IAM Actor、command_id、expected_revision 和 expected_draft_version。核心必须为 CNY draft；分项总额必须与当前 Base 草稿金额精确相等。独立 Base 后续修改导致不一致时，客户端显式报错，不伪造分项。

同 command／同标准化意图返回原回执；同 command／不同意图冲突。已提交回执重放先于当前状态检查。schema 21 候选允许 active 合同追加／维护未确认协议意图，但必须同时消费 current Contract revision；原始费用／费率、项目／链和已确认协议全部冻结，已确认协议必须保持排序前缀。每次保存同事务更新本库投影、追加命令快照和审计；历史表 UPDATE／DELETE 被拒绝。草稿协议与结算意图不写入 confirmed Instrument、Change 或 Settlement，预览不是中标、签约、生效或付款。

## 端口与分段继续

- `GET /api/v1/contracts/workspace-context`：只返回原生权限元数据；Tenant 或已有 Contract 精确凭证可读取，只有 Tenant 的 draft.manage 才得到 draft.create。每份合同仍独立校验，不能由元数据扩大资源范围。
- `GET /api/v1/contracts/party-candidates`：仅 Tenant view 读取 Contract 当前本地主体目录，DTO 明确 directory_kind=contract_local；精确 Contract Scope 不扩为全目录权限。不是 Party Directory 已统一迁移的证明。
- `GET /api/v1/contracts/{id}/workspace`：原生合同和可选 `reference_oa_draft`；无扩展明确缺映射，源故障不是空列表。
- `POST /api/v1/contracts/{id}/reference-oa-draft`：保存完整有界客户草稿；Tenant、Contract、Actor 由服务端注入，客户端身份不能授权。

界面首次保存顺序为 Base draft → 客户扩展 → Project Link → 原生回读。跨应用没有原子事务；每段保留原 command 与版本，失去回复时显式继续原操作，不自动重新创建合同。Link 失败不取消已保存来源；保存成功而回读失败只续 GET。sessionStorage 仅保存按 Tenant／Subject／Project 隔离的未确认操作暂态，不是业务数据库。409 保持冲突提示；显式冲突核对界面尚未完成，不自动更新 expected version 覆盖。

草稿取消走原生 cancel，保留客户扩展、链接和历史；协议取消追加带 tombstone 的扩展 revision。正式合同不能按这套草稿操作覆盖或删除。

## 同抽屉的原始合同确认

问题 A → 可验证结果 B：基础记录已保存，但签约主体和生效条件缺失，不能正式使用 → 在原合同抽屉附加小型原生准备区，显式选择本目录主体、填写合同内自由文本角色和生效日期，再由 Contract 原用例确认。保留原双列和表单，不跳转独立小系统，不自动以项目甲方名称创造 Party，也不把签订日期代填为生效日期。

主体添加／移出、日期保存及 activate 分别使用原生 capability、expected_draft_version 和稳定 command ID。日期更新保留当前编号、币种、金额及签订日期；移出仅结束草稿关联；activate 冻结 revision 与主体快照。协议草稿和结算意图不会随原始合同激活而转为正式事实。原有不可变历史约束继续适用，active 表单不再按 draft 编辑。

未知回复保持原段，已写入但回读失败只续 GET；明确的单事务输入／权限拒绝释放待续输入，允许修正，409 则继续阻断自动覆盖。页面可用性与服务端最终授权分别验证。当前这一准备链只有组件测试端口和独立原生命令的分层证据，尚无真实组合公网端到端证据。

## 原组件与恢复限制

### 定案到正式结算（施工约束）

问题 A → 可验证结果 B：原表单结算金额只是意图，后续资金与奖金无可冻结来源 → 在原定案区内显式选择当前正式合同版本的确认主体，填写文书编号、标题、累计金额、确认日期和依据，再调用既有原生 `settlements` 命令。合同金额／协议预览、附件上传和表单保存均不得自动产生 Settlement。

以 `contract.settlement.record` 和可信精确 Contract 凭证控制操作；冻结当前 Contract revision、expected settlement sequence、精确金额、确认主体位置和稳定 command ID。已提交而回复未知时仅重放此命令；提交后读取故障仅续读取。GET 零写；最终结算后只读，不覆盖或删除历史。原工作面保留结算意图显示，并另列正式记录和来源版本，避免预览被误当确认。当前 Base 仅支持追加阶段／最终结算，最终结算后的更正用例尚不存在，不能以第二份最终结算或修改旧表冒充更正能力。本段不负责 Money 入账、来源建议或完整 B04～B06；要分别验原组件点击、服务端原生复读、拒绝与恢复。

### 协议意图显式确认（候选实现，分层验证）

问题 A → 可验证结果 B：协议只能留在草稿预览，原始合同确认后无法继续经营 → 保留原协议表单，允许在已生效合同追加客户协议意图，再显式填写生效日期，由 Contract 同库事务调用原生追加变更并登记意图到 Instrument 的稳定关联。

原始分项、费率及已确认协议冻结；未确认协议仍可编辑或取消。确认必须带扩展 revision、合同 revision、完整意图快照和稳定 command ID，按原明确排序确认第一份未确认协议，不能由日期或编号猜测对应正式记录。调整对应差额、修订对应重述目标金额、无金额变化对应 unchanged；服务端同时检查 Base 当前金额与已确认客户分项一致，独立来源变更未核对时拒绝自动衔接。确认和关联必须一个本库事务；回复未知重放原确认命令，不能产生第二份正式协议。此段不负责附件字节、最终定案更正或 Money 资金。

`POST /api/v1/contracts/{id}/reference-oa-amendments/{amendmentID}/confirm` 使用原生 lifecycle capability。migration 21 追加不可变确认关联；原生 Change 和关联在 Contract 同库事务内写入，原生规范及审计不绕过。实际专项 PG18 已通过三类型、原金额／零费率保持、冻结／错序／错快照拒绝、重放零重复和只读／相邻资源拒绝；组件 p 实际点击通过三类型确认与刷新零写。schema 21 仍仅候选，不修改主站 schema 18；空目标恢复、独立进程和真实组合公网各自记账，不拼成整体通过。

直接复用原 OA 双列合同列表、780px 抽屉、合同／协议表单和样式；更换数据与权限端口。必要修改包括精确字符串金额、nullable 零费率、未确认状态、保存后恢复权限、数字全选不夺回焦点、父表单回车不误提交。三类附件复用既有 SDK／原控件，通过[有界引用端口](./reference-oa-materials-v1.md)分段提交，不沿用保存失败后删除物理文件。原结算区显示独立正式记录，列表优先显示来源冻结的正式结算，意图不被覆盖。

旧 Ledger Archive v1 不包含客户扩展。候选 export 遇到该数据明确拒绝，防止静默遗漏；版本化单合同导出／恢复尚未实现。完整应用库 schema 21 的正式协议关联（backup-e）、schema 22 的材料引用和命令历史（backup-g）→空目标恢复、原命令重放／继续写入已实际通过；不包含外部 Evidence 字节。联合恢复、旧 schema 兼容闭包、正式发布与公网操作分别验证，不能用本地组件测试代替。

实际证据与未完成项见[候选报告](../../reports/oa-contract-draft-ui-candidate-20261004.md)，全站范围以[完整 OA 操作交付清单](../assembly/oa-complete-operation-delivery-ledger.md)为准。
