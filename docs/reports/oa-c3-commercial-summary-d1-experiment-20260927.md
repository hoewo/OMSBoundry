# OA C3 项目经营链路 D1 实验报告

日期：2026-09-27  
结论：`local-d1-supported`；C3 经营链路只读组合的本地 D1 架构假设获得直接证据，OA 完整体仍为 `insufficient-evidence`

## 1. 本轮验证的问题

合同、应收／回款和开票分别由 Contract Ledger、Money Flow Ledger 与 Billing / Invoice 拥有。项目经营人员仍需要在同一 Project 上理解这些事实，但组合能力不能通过共享表、复制领域模型或把金额重新写入 Project 来实现。

本轮验证：Project Workspace 能否只根据自己拥有的 Resource Link，使用最小权限工作负载凭证并行读取三个来源应用的类型化投影，在 Nginx → Gateway → IAM → Project 正式入口中返回可解释、可下钻、允许局部失败的 C3 经营分段；同时验证人类金额读取权限、撤权、账号停用、依赖故障和性能边界。

## 2. 被测边界

调用链：

```text
Browser/API client
  → Nginx
  → Gateway
  → IAM 签发 Project 人类 Capability Credential
  → Project Workspace /commercial-summary
      → IAM 工作负载凭证交换（单 audience、单 action、单资源）
      → Contract Ledger reference
      → Money Flow obligation / transaction reference
      → Billing invoice-registration reference
  ← 按 Resource Link 顺序返回四个只读 segment
```

本轮没有把合同、义务、资金或票据字段写入 Project 数据库，也没有建立跨库查询、共享 ORM、跨来源自动合计或服务凭证下发浏览器。组合结果只存在于单次响应。

## 3. 环境与入口

- Compose project：`omsbizfoundry`
- 公共入口：`http://127.0.0.1:18102`
- 数据设施：PostgreSQL 18.6，同一实例中的应用独立数据库／账号
- 应用：Nginx、Gateway、IAM、Workbench、Project、Contract、Money、Billing、Party、People、Evidence
- 验收程序：`acceptance/d1-c3-commercial-summary/verify-running`
- 业务 API：`GET /project/api/v1/projects/{project_id}/commercial-summary`
- 人类读取能力：`project-workspace.commercial-summary.read`

## 4. 逐步执行与证据

### 4.1 建立来源事实

验收程序从正式入口登录首位管理员，并分别取得 Party、Contract、Money、Project 与 Billing 的应用凭证。随后通过各自公开命令建立：

- Project：`b81c3dcf-d931-460a-ae2d-2a582dabd62f`，最终 revision 6；
- Contract：`593542e2-d356-49ce-a120-aad7cbfb6f18`，当前有效金额 `100000 CNY`；
- receivable Obligation：`5c3a11c0-ad5c-4c41-937e-faf190fbfc2e`，有效／未清 `60000 CNY`；
- receipt Transaction：`89efc64c-a78b-4ae1-8c82-ffbab1b9ace5`，有效／未核销 `25000 CNY`；
- Invoice Registration：`55e6f78c-f054-478c-ac1b-1e5a201aecb7`，`3000000` minor units，即 `30000 CNY`。

Billing 只接受 Project `pending_completion|active`，因此验收先执行完成立项命令。该命令因缺少负责人返回预期的 `422`，但 Project 按已定义状态机进入 `pending_completion / revision 2`；随后 Billing 对同一 Project 引用验证成功。这证明验收使用了真实业务状态，而不是为组合测试绕过目标不变量。

### 4.2 建立 Project Resource Link

使用连续 Expected Revision 把四项来源连接到 Project：

1. `contract-ledger / contract / primary-contract`；
2. `money-flow-ledger / obligation / receivable`；
3. `money-flow-ledger / transaction / receipt`；
4. `billing-invoice / invoice_registration / invoice-registration`。

Project revision 从 2 推进到 6。每次连接前，Project 都向 IAM 请求目标 audience、`*.reference.read` 单 action 和当前资源 ID 的精确 Scope；IAM 审计存在对应签发记录，目标应用再次复验租户、类型和资源 ID。

### 4.3 类型化组合结果

基线响应返回四个 `available` segment，顺序与 Resource Link 一致。Contract、Obligation、Transaction 和 Invoice Registration 各自只出现本领域字段；金额分别保持十进制定点字符串或 Billing minor units。Project 没有产生跨来源合计。

基线请求总耗时 37 ms；四个来源分段耗时分别为 33、13、33、1 ms，证明请求并行而不是简单串行等待。`linked_revision=source_revision=1`；若以后不同，状态会转为 `revision_changed`，不会静默覆盖 Link 的历史观察。

### 4.4 人类权限矩阵

参考 OA Overlay `1.1.0` 通过 IAM 创建三类一次性测试 Subject 并只授予版本化角色动作：

| 角色 | Project read | commercial-summary.read | 结果 |
| --- | --- | --- | --- |
| 总经理 | 允许 | 允许 | 200，四段可读 |
| 财务 | 允许 | 允许 | 200，四段可读 |
| 生产经理 | 允许 | 拒绝 | Project 详情 200，但经营汇总 403 |

因此普通 `project-workspace.read` 不会隐式泄露经营金额；目标 Project 应用而非页面或 Gateway 执行最终拒绝。

### 4.5 撤权与账号停用

对财务 Subject 依次执行：

1. 撤销 `project-workspace.commercial-summary.read`；
2. 旧的未过期应用凭证在有界残余窗口内仍返回 200；
3. 重新授权应用后，新凭证访问返回 403；
4. 恢复 Grant 并重新授权后返回 200；
5. 停用账号后，旧应用凭证仍只在原 TTL／Scope 内有效，IAM Session 返回 401；
6. 再次申请应用凭证返回 401，Gateway 清除对应 Capability Cookie。

这组结果与离线复验和有界撤权残余窗口设计一致；没有通过逐请求回查 IAM 假装即时撤权。

### 4.6 性能采样

对同一 Project 连续读取 30 次：

| 指标 | p50 | p95 | 当前门槛 |
| --- | ---: | ---: | ---: |
| 并行组合总耗时 | 13 ms | 220 ms | ≤ 800 ms |
| 同批分段耗时串行等价 | 19 ms | 362 ms | 仅作对照 |

全部 30 次返回 200 且四段可用。该结果只描述本机 D1 样本，不是容量或生产 SLA 承诺。

### 4.7 故障注入与恢复

| 注入 | 故障时结果 | 恢复后结果 | 写入影响 |
| --- | --- | --- | --- |
| 停止 Contract | Contract segment=`unavailable`，其余可用 | 四段恢复 `available` | Project revision 不变 |
| 停止 Money | Obligation／Transaction=`unavailable`，Contract／Billing 可用 | 四段恢复 `available` | Project revision 不变 |
| 停止 Billing | Invoice segment=`unavailable`，Contract／Money 可用 | 四段恢复 `available` | Project revision 不变 |
| 停止 IAM，并重建 Project 清空凭证缓存 | 四段均 `unavailable`，汇总本身仍 200 | IAM 恢复后四段 `available` | 不产生组合写入 |

恢复不需要重启 Project（IAM 场景中只在故障开始前重建一次以强制清空缓存），也不需要修改 Resource Link 或重放业务命令。

### 4.8 HTTP 证据汇总

- 断言总数：100；状态不匹配：0；
- 方法分布：GET 54、POST 46；
- 状态分布：200×62、201×7、204×11、303×15、401×2、403×2、422×1；
- 单请求观测：最小 0.003294 s，最大 0.930490 s，平均 0.114343 s。

## 5. 实验中发现并修复的问题

### 5.1 D1 内部端口配置错误

Project 的 D1 环境最初把 Contract／Money／Billing 配成 `8080 / 8080 / 8092`，实际端口是 `8081 / 8083 / 8091`。表现为 IAM 已成功签发精确 Contract 凭证，但 Project Link 命令返回 `503 reference_unavailable`。直接在容器网络复验同一 JWT 和 Contract reference 返回 200 后定位到编排端口，并修正 `deployments/d1/compose.yaml`。

### 5.2 运行容器与 Suite Manifest 不一致

三个应用容器均在运行，但 `.runtime/suite-manifest.json` 仍是先前最小组合，Money／Billing 标为 disabled。Gateway 重启后按清单返回 404。重新使用当前应用开关渲染 Manifest，并重启 Gateway／Workbench 后恢复。结论是升级／恢复验收必须同时比较“容器集合”和“Manifest enabled 集合”，不能只看 `docker ps`。

### 5.3 验收证据统计误报

TSV 汇总最初使用默认空白分隔，带空格的请求标签造成 100 条成功状态被误计为差异。统计器改为显式制表符分隔后，完整故障矩阵重跑为 100／0。该问题不影响 HTTP 断言本身，但证明验收报告生成也必须可复核。

## 6. 架构判定

本轮支持以下结论：

- C3 的一体化用户阅读体验可以建立在应用自有事实和版本化引用上，不要求共享数据库；
- 精确工作负载凭证使跨应用调用的权限成本可控，没有把 Project 变成万能服务账号；
- 并行、局部降级和未知不等于零避免了“连接器一定造成长时间卡顿”的直接反证；
- 人类 `commercial-summary.read` 与普通 Project read 分离，证明 IAM 在 OA 金额链中承担实质角色，而不是只负责登录；
- Project 适合作为“当前 Project 已连接事实”的窄组合上下文，但不应演化为任意跨域查询平台或经营事实所有者。

该结果是“尽量只做加法”的一项正证据：Contract、Money 和 Billing 均未因 C3 修改数据所有权，Project 通过新增端口、Resource Link 解释和页面分段得到新能力。

## 7. 尚未通过与下一步

- macOS 自动化通道在本轮最终点击时仍报告系统锁定；页面资源和脚本已自动化验证，但“从 Workbench 点击 Project → 经营链路 → 四张卡片 → 来源下钻”的实际浏览器点击仍需补证；
- 未执行完整双 Tenant 交叉矩阵；
- 未在公司 Ubuntu 或 ICP 生效后的个人试验田复演；
- 当前 C3 只证明合同、确认义务、资金流水和开票登记的只读并列，不证明 Settlement、应开未开、自动建议、自动核销、税务生命周期或跨来源财务计算；
- 未验证来源 revision 改变后由用户确认重建 Link 的完整交互；
- C4～C8 仍未完成，不能把本轮外推为 OA 完整体通过。

下一步先补实际浏览器点击和 C3 双 Tenant／revision-changed 人类恢复；随后回到阶段主线选择 C4 多类型生产的最小纵向闭环。真实 Ubuntu 环境可用后，复用同一镜像与验收程序验证公网 HTTPS、备份恢复和继续写入。

