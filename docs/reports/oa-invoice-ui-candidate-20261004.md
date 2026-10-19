# 原 OA 开票候选接线与分层证据

2026-10-04。原双列表、三费用行、640px 抽屉／表单、追加纠正／历史、登记无效和文件引用已候选接线，未发布。完整范围仍是[OA 全站清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)，不是只有招投标或这三个经营页签。

最新重构建收尾见[整轮检查点](./oa-complete-ui-checkpoint-20261004.md)：bundle `f716e1cc6de47984272582de5055b1900c87941afe79f2259f611e22a1e0cfb6` 的 Contract ae／Cash j／Invoice f 共 51 场景、0 页面错误及 Workbench native c 两次独立资源启动通过；较早回执／摘要保留原版本，不混称新构建。所有组件检查仍是本地明确测试端口，未发布、不代表公网或完整 OA。

## 边界与实际改动

BF-100／[开票规格](../specs/assembly/oa-invoice-original-command-slice.md)：Billing 独占已开具票据登记、精确金额、双方发生时快照、来源、revision／命令／历史／审计和文件引用。Party Directory 独立拥有当前主体；Project 六种有界关系键归类两链／三费用；Evidence 独立拥有 File／Version、字节和访问；Workbench 无业务数据库。源目录只读，直接复用原 InvoiceRecordList、OurInvoiceForm 的布局、原样式和已有 AttachmentUploader／FilePreviewDrawer；原源摘要及必要差异记在 `frontend/oms-workbench/frontend/source-provenance.json`。

整数金额请求仍为 v1 int64 数字字面量，前端逐字段校验／编码、不经 Number 舍入；响应兼容增加 `amount_minor_decimal`，当前／历史所有金额表示按同一 amount.read 脱敏。发现创建仅检查 record、未检查 amount.read，已补最终服务端拒绝及反例；没有扩宽 Grant。列表不向缺金额权限的人展示 record 动作。明确选择 active Party ID／revision，不按甲方名称猜身份。

创建→保留 Invoice ID→Project Link→GET 各自确认；纠正显式原因／expected revision、追加不可变历史；已有 Link 的 source revision 保留发生时值，界面同时显示当前 revision。未知回复保留原命令／stage，回复不匹配原意图则停止并保留待办；409 不自动覆盖，403 不换身份。历史页重叠／重复 revision 拒绝，不当完整结果。

文件完成与业务引用分开：SDK 上传／complete 后持久保留 scoped File／Version hint，元数据故障或 owner 回复未知不重传；PUT evidence 保存 Billing 引用；移出不删除字节。已无效登记／切换项目不能开始 SDK 上传。Billing 最新候选复验工作负载只签发单 `metadata.read`／精确 File；遗留 view 改为更窄动作，匹配 policy 必须随正式发布闭包校验。

## 已运行的检查

| 层次 | 实际结果 | 限制 |
| --- | --- | --- |
| Billing Go test／vet，Workbench Go test／vet | 通过 | 普通 Go 包内数据库检查可能 skip；下面专项另计 |
| TypeScript check／build，全量 Node | 通过；最新 262 项零 skip，Invoice adapter 18 项；原工作面 selected-only 及 PUT 写回执追踪另验 | 逻辑故障为显式测试端口 |
| 原组件 e，与合同 ad／支付 i 同构建 | 15 场景、25 个业务命令、8 条登记、0 页面错误；六分类、大额精确 wire、追加纠正／冻结历史、创建／Link／纠正／附件／登记无效丢回复原命令续办、刷新／只读／脱敏零写；实际 SDK complete、预览／下载字节相等、移出 0 DELETE | 本地内存原生端口，不代表 PostgreSQL／实际 IAM issuer 或公网 |
| 私有 PG18 HTTP 专项 a | `TestProtectedHTTPWithNativeIAMPostgresAndTypedResolver` 实际通过、无 skip；自有数据库和清理的测试 schema，金额超 JS 安全整数、盲写拒绝、Scope／Tenant、历史脱敏／纠正／文件与幂等 | 真实 IAM verifier／测试 issuer；版本化来源与 Evidence 为 fixture |
| 独立启动 a | 空自有库 ops migrate→server verify、schema 1、八个匿名端点 401、重启事实保持；正式 IAM 配置／私有端口 | 不验证来源在线；该二进制先于 narrow metadata 调整，不能混报最新二进制 |
| 最新 c 真库专项 | 原 HTTP 专项及 `TestPostgresRemoveLastEvidencePreservesHistoryAndReplay` 两项实际运行、通过、无 skip；最后一个引用移出后复读为空数组，原命令重放与旧 revision 文件引用保持 | 正式 verifier／测试 issuer；不是公网或组件端到端 |
| 最新 c 实际 IAM＋Billing＋Evidence 组合 e | 三实际进程／自有 PG18、精确大额和盲写拒绝、实际文件直传完成／引用／重放／下载、四次单 metadata.read／精确 File 工作负载审计；匿名 401、相邻记录 403、Evidence 停机新命令 503 且零业务写；Billing 完整自有库＋Evidence 库和字节→空目标恢复，继续纠正／移出最后引用、历史和下载保持 | Project／Party 为明确 HTTP 参考夹具；Billing 自有库使用 PG18 dump／restore，Evidence 使用其实际 backup CLI；local 文件／私有网络，不是 OSS、Nginx 或完整 Suite 恢复 |

安全回执：[组件 e](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/components-e.json)、[真库 a](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/postgres-a.json)、[启动 a](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/startup-a.json)、[双列表](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/two-chain-invoices-e.png)、[纠正历史](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/append-correction-history-e.png)。截图已视觉检查。统一 bundle SHA `e9f67ef24d4d396d6c1df1d48aeecb63178d7cd4ba733c0591096962b6609861`。真库 a test SHA `98518438daa27be65b62db88673b29b91aa5bd5f29b33c8eb23622dd959c8b9b`，启动 server SHA `8704beb723e346620d6e31ba7a9ec27dbd4de832b78bf2236c1977b1af656b33`；ops SHA `536b092f8145101962f7c1961ca08708cfa4a79090cc82fe0426d88304d61077`。

## 失败及未完成项

浏览器 a 因原卡片不显示备注而定位失败；b／c 关系键 chain 索引误用 fee；d 的同文案 owner／附件错误提示定位歧义。修正测试为原列表记录顺序、明确 chain 和主面板实际失败 Alert，e 通过；不强制点、不改 DOM 状态，失败和已保存测试事实保留。普通单元早期旧 fixture 缺 amount.read 导致创建测试失败，已改为显式授权 fixture，不恢复盲写漏洞。

原 UI 一对一支付关联尚未实现，控件禁用且明确命名。文件先结构化登记后进详情补入，与原创建时一起排队上传仍有操作差异；预览复用独立原抽屉，未复用内联 1320px 扩展。税务开具／税务作废／红冲不属于 Billing 登记能力；不能把“登记无效”称作税务成功。Money 核销、Receipt Set、奖金及全角色撤权不在本通过结论内。

正式 issuer 组合失败 a～d 分开保留：a 在相邻记录未授权时已被 IAM 拒绝签发，改为只获相邻记录读／材料权后验证当前记录 403；b 将命令回执纳秒时间和真库复读微秒时间全量相等误作业务改变，改为逐字段相等、仅两时间字段允许 ≤1 微秒、原命令回执仍完全相等；c 的旧 Python 不能解析九位小数，改为严格 RFC3339／整数纳秒比较。d 已通过实际 metadata 单动作／精确 File 审计及停机零写、恢复启动，随后移出最后引用返回 500：PostgresStore 把空 slice 编码成 null，违反 array 约束。这是真实业务缺陷，已修成规范空数组，nil／[] 命令输入统一，追加单元及真库“最后引用移出／历史／复读／重放”反例；随后最新 c 二进制的 e 联合与真库 c 均独立通过，不能用旧 d 记通过。所有失败 Main 未变、私有 runner／PG 停止；[a](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/combo-a-failed.json)、[b](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/combo-b-failed.json)、[c](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/combo-c-failed.json)、[d](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/combo-d-failed.json)。

最新 c 的窄 metadata／空数组修复已由组合 e 及真库 c 补证；旧 d 的 500 不抹除，e 才是通过回执：[组合 e](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/combo-e.json)、[真库 c](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/postgres-c.json)。Billing c server SHA `e89efbbcb9f17c87989e223301f9eb72801218be0ac9438ed3b226ad1fc6a5cc`，HTTP test SHA `6943c38f307b65c2f8ae5b54e4f20c2b77cbcdc21d22935d733bd361769ab6fb`，Store test SHA `3f6a1c5d9233956fd7aa6b3b6641a26ed27641160f25b4f9ba74014526d90238`。该组合中的 Party／Project 是明确 HTTP 参考夹具，不能宣称这两个来源真实进程或完整 OA。正式匹配 IAM workload policy、旧发布兼容、入口／同项目公网链、全角色仍是门禁。Main、用户101、Grant 未改，恢复演练栈不重启；Evidence 仍 local，前端仍 ECS。

入口：`verify-invoice-candidate-ecs.py`、`verify-invoice-startup-ecs.py`、`verify-invoice-material-combo-ecs.py`；`node acceptance/oa-initiation-repair/verify-invoice-components-browser.mjs /tmp/oa-invoice-components.<新后缀>`。不收 env、日志凭据、dump 或 key 到 Git。
