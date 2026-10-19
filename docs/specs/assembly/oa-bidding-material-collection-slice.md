# OA 招投标多附件兼容切片

2026-10-03 BF-096 已实施：直接复用原 OA `BiddingFileList`、承接方式控件、`FilePreviewDrawer`、确认弹窗及样式，最小适配替换业务／权限端口；React 局部挂入 Workbench，不继续重写仿品。`.20261003.1` 真实公网上传、原组件预览／下载、单份移除、Project 当前资料读取、刷新零写及丢回复后 reload／原命令继续通过。Main 仍用 local 文件 Provider；更多文件／权限／撤权、OSS 及整个 B03／OA 未通过。见[组件与业务实测](../../reports/oa-reused-bidding-ui-validation-20261003.md)。

日期：2026-10-02，验证更新于 2026-10-03。BF-095；状态：真 PG18、旧库兼容、独立恢复和启动通过，现已部署 BE 000002／Workbench 两项。见[此前候选实测和反例](../../reports/oa-bidding-material-collection-validation-20261003.md)；下方候选待接线仅表示夜间历史阶段，不是当前状态。

问题 A → 可验证结果 B：原 OA 每类材料可以连续上传多份并单独移除，而当前 BE 的 put 会取代同类旧引用 → 同一承接记录可追加多个明确 File／Version，单份移除不影响其余引用，历史、旧命令及 Project 资料投影保持可解释。

## 所有者、兼容和约束

- Business Engagement Base 拥有材料集合和业务引用；Evidence 独占文件字节、不可变版本、上传完成校验和授权访问。Workbench 只接线，不保存材料事实、OSS 密钥或签名 URL。
- 在既有 v1 materials 路由增加 `append`／`remove_one`，复用原生 evidence.manage、tenant／scope、状态和 expected revision。旧 `put` 仍整类替换，旧 `remove` 仍整类移除；旧客户不得被静默改成追加。新页面只用新明确操作。
- `remove_one` 必须给本 Engagement／同 requirement 的当前 material_reference_id；不能以 File ID 猜目标，不可移除其他租户／承接记录的引用。它只追加结束 revision，绝不调用 Evidence 物理删除。
- 新字段 material_reference_id 为空时不参与旧 JSON hash，保持旧命令 hash／回执幂等；历史 event 和命令回执不重写。
- 同一 requirement 的同 File／Version 不能重复有效；不同 File 或版本可并存。每 Engagement 最多 100 条当前引用，以保持既有 Project material-reference v1 的已声明消费上限；拒绝越界必须零部分写入。
- 当前 requirement 只要有至少一条有效引用就满足，不变更三方式／ready 的含义；不适用和 cancelled 仍禁止材料写入。方式切换不删除旧材料。
- requirements 增加可选 material_references 当前数组，保留 legacy material_reference 为最新 created revision 的代表；全量 materials 仍包含历史。Project v1 本已允许同 relation_key 多个不同 File／Version，无需改 Project schema／owner。
- 只追加 migration 000002：改有效唯一索引和允许的新历史事件，不修改 000001 或迁移前任何业务行。升级后禁止旧二进制直接回滚（其 schema 门禁及单份假设不兼容）；回退需原二进制＋升级前数据库联合备份，不能删除新引用来凑旧约束。

## 验收与不负责

先运行单元、HTTP 契约、真实 PG18 的多份追加／单份移除／旧 put-remove／重复与越界拒绝／并发／幂等／错租户及 migration 前后旧事实保持，再做独立启动和空目标恢复／继续写入。只有后端这些门禁通过后才进入页面文件闭环，并单独验证浏览器文件选择、上传完成、来源登记、查看／下载／移除、刷新、权限、未知回复及孤立上传的可解释恢复。

此切片不改材料规则、IAM Manifest／Grant、文件 Provider、合同／资金、项目资料所有权或建通用附件平台；不以 API 准备的数据冒充真实浏览器上传，不关闭 B03、OSS、完整 Suite 恢复或完整 OA。

2026-10-03 页面实现：追加须经独立 Evidence v1 精确版本复读；SDK 完成后先保存 File／Version 身份，读取失败不当空列表，不自动重新上传。未知材料回执保留原命令，409 不覆盖；Legacy 单份响应不收集合移除命令。按 Tenant／Subject／Project 隔离的 sessionStorage 暂态最多 100 项、24 小时，只有原命令／File／Version／明确意图，没有凭证或授权 URL；存储受阻降为内存，不绕过 IAM。原组件接线新增 16 项，Workbench Node 合计 172 项；实际 reload／原命令继续见 public-h。不把暂态当事实库或把当前 local CSP 当 OSS 验收。

依 BF-079 的轻量透镜随新 Workbench 发布，12 项 Node 门禁通过，按 Tenant／Subject 隔离、无敏感值、迟到回复保护；存储失败不影响业务。它只辅助架构学习，不新增平台、业务库、权限或服务器审计责任；Evidence step 被 key 白名单遗漏的缺口另记，完整文件架构解释链尚未验收。
