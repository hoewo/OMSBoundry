# OMS1 2026-10-25 迭代实施拆解

状态：实施中。BF-184～BF-237 已形成无产品待确认项的需求基线；P0～P7 已通过本地门禁。P8 的源码／构建及本地真实 Chromium 跨 origin 宿主门禁已通过，见[P8 源码报告](../../reports/oms-v1-iteration-p8-feedback-v2-source-validation-20261026.md)，但真实项目 115 通知、运行凭证、双账号／双租户和外部跨域浏览器旅程尚未验收，故 P8 未完成。此前阶段报告保持在 `docs/reports/`；本地通过不表示 D1 或公网已经发布新版本。

问题 A → 可验证结果 B：本轮同时涉及租户、两个新业务 Owner、开放 API、跨系统资金闭环和外部 Feedback SDK，若按页面并行堆功能会重新形成浏览器事实和隐式耦合 → 按所有权与依赖顺序交付可独立验证的纵向切片，每阶段都有迁移、授权、故障恢复和运行证据。

## 1. 实施不变量

- `Internal Finance Requests`、`Payroll`、People & Groups、IAM、Evidence、Money 各自拥有自己的数据库和事实，禁止共享表、跨库写或复用领域模型。
- Workbench 只组合页面和调用版本化 API，不保存报销、工资、反馈、租户或 Key 业务事实。
- 所有金额使用整数最小货币单位，比例使用整数精度；已确认事实通过 revision、调整、撤回或冲销演进，不静默覆盖。
- Tenant、Subject、Person 与 Capability 只来自可信 IAM／Gateway 上下文，浏览器不能通过参数或 Header 自报身份范围。
- 新增服务从第一天同时具备 PostgreSQL migration、健康检查、Capability Manifest、备份恢复和 D1 装配，不先做仅浏览器可用的假后端。
- 每阶段只在其验收矩阵完整通过后进入下一阶段；局部单元测试不能替代双租户、跨服务、恢复和真人浏览器证据。

## 2. 当前工程差距

| 能力 | 当前证据 | 缺口 |
| --- | --- | --- |
| 租户与账号 | P1 已证明同进程 Tenant A／B 的创建、登录、授权、资源隔离、撤权和恢复；BF-254 已修复 Gateway／Workbench 把 Manifest 默认 Tenant 误当白名单的问题；BF-255 已提供绑定 C0 的同 Origin 双租户运行驱动 | 驱动尚未在新 D1／公网执行；首版明确不提供同账号租户切换 |
| 成员 | P3 已有成员生命周期、任职 revision、Payroll 档案挂载及 Workbench 两 Owner 管理旅程；P4 已冻结工资成员快照；BF-256 已提供绑定 C1 双 Tenant Person 的 C2 薪酬档案运行驱动 | C2 尚未在新 D1／公网执行；待全组合恢复与连续旅程 |
| 财务申请 | P7 已完成 Evidence 实时复验、员工本人开放 API、重复识别、草稿保留与 Finance→Money 闭环；BF-258 已提供绑定 C0～C3 的双 Tenant 付款申请 C4 驱动 | C4 尚未在真实 D1 执行，且只验证无附件显式确认、无需审批和未支付；Evidence 报销、实际付款与恢复仍待连续旅程 |
| 工资 | P4 已有工资期间／行快照、计算、核对、调整、发放及 Money 连接；BF-257 已提供绑定 C0～C2 的双 Tenant 工资周期 C3 驱动 | C3 尚未在真实 D1 执行，且只验证零比例、待核对、未支付；实际发放、Money、恢复／导出仍待公网连续旅程 |
| 开放平台 | P7 已开放 profile／settings、Evidence 发票上传和本人报销命令／查询，并同步 OpenAPI 与中文说明；BF-259 已提供绑定 C0～C4 的双 Tenant 员工 Key C5 驱动 | C5 尚未在真实 D1 执行且只验证 Key、profile／settings 与本人审计；发票报销业务写和恢复留待后续连续阶段，其他业务 API 不在本轮 |
| 发票报销闭环 | P7 已用真实 TLS、IAM、Gateway、三个 Owner 和四库完成上传→提交→付款→本人 paid 查询及故障恢复 | 商业杀毒／OCR、自动代付、审批和公网连续旅程不在 P7 |
| Feedback V2 | P8 已完成正式 Capability、Gateway 配置适配、主 OMS1 桌面／移动入口、iframe bridge、未读角标、Arckit 诊断 allowlist、OMS Origin 模板及本地真实 Chromium 跨 origin 宿主门禁 | 项目 115 服务端通知开关、有效运行凭证、双账号／双租户、附件／回复／单条已读和外部端到端浏览器仍待验 |

## 3. 依赖顺序

```text
P0 基线冻结与需求闭合
  ↓
P1 IAM 多租户与 Subject↔Person 基础
  ↓
P2 Internal Finance Requests / Payroll 两个独立 Owner 骨架
  ├─→ P3 成员＋薪酬档案纵向切片
  ├─→ P4 工资生成、核对与发放切片
  └─→ P5 报销／付款申请与财务确认切片
             ↓
P6 员工 API Key 与 /open-api/v1/
             ↓
P7 发票上传→报销→付款→Money→本人查询闭环
             ↓
P8 Feedback V2 正式装配
             ↓
P9 D1 全组合、恢复与连续业务验收
```

Feedback V2 不依赖报销闭环的数据事实，但放在其后可避免在 IAM、多租户和主侧栏仍变化时重复接入。若后续确需提前，只能在 P1 的可信身份和正式 `feedback.use` 能力完成后并行，不得绕过门禁。

## 4. 分阶段交付

### P0：冻结可追溯基线并关闭需求

交付：

- 将 BF-184～最终 Feedback 决策形成一份无待确认产品项的迭代规格；
- 保存当前分支、commit、工作区差异、现有本地测试和 D1 已发布版本清单；
- 建立 BF 决策到服务、migration、API、页面、测试和运行证据的追踪矩阵；
- 明确测试数据库、端口、镜像名、Suite Manifest application ID 和 Capability 命名，避免实施中临时命名。

验收门：`git diff --check`、文档链接／决策索引校验、现有受影响模块测试通过；不存在有效 Key、密码或生产业务数据进入 Git。

### P1：账号、租户与成员身份基础

覆盖：BF-184，以及 BF-203、BF-204、BF-216～BF-220、BF-235～BF-236 所依赖的身份范围。

交付：

- IAM 平台管理端创建 Tenant 与首个管理员；租户管理员邀请、激活、停用账号并分配版本化能力；
- 同一域名登录后由会话确定唯一 Tenant，首版不提供租户选择／切换；
- IAM Subject 与 People Person 的租户内唯一映射，保存稳定 ID、状态和 revision；
- 为 Internal Finance、Payroll、Open API 和 Feedback 发布 Capability Manifest；
- 对现有单租户数据执行显式 Tenant 绑定迁移，保留 ID、revision、引用和审计。

验收门：同进程 Tenant A／B 正反例覆盖登录、搜索、分页、导出、附件、稳定 ID 猜测、撤权、停用和备份恢复；跨租户请求全部拒绝且目标数据无变化。

### P2：两个新 Owner 的最小可运行骨架

状态：2026-10-25 本地通过；未部署 D1／公网。完整证据见[P2 验证报告](../../reports/oms-v1-iteration-p2-owner-skeleton-validation-20261025.md)。

交付：

- 新增 `backend/services/internal-finance-requests`：独立 Go module、PostgreSQL schema／migration、健康检查、版本化 HTTP、Capability Manifest、审计、幂等命令表、revision 模型、备份恢复脚本；
- 新增 `backend/services/payroll`：同等交付骨架；
- 为两者增加 D1 数据库、镜像、Compose、Nginx／Gateway 静态路由、Suite Manifest、部署配置和恢复清单；
- Owner 端统一验证可信 Actor、Tenant、Capability、资源归属、expected revision 与幂等键；禁止 Workbench 直连数据库。

验收门：两个服务可单独启动／迁移／读写／备份／恢复；无 IAM 凭证、错误 Tenant、错误 Capability、重复或冲突命令均有稳定响应；架构静态审计不出现跨应用数据库／模型依赖。

### P3：成员与薪酬档案（已通过）

覆盖：BF-187、BF-188、BF-190、BF-211～BF-215。

交付：

- People & Groups 维护基础成员、任职周期和离职／停用 revision；
- Payroll 维护按 Person ID 挂载的 Compensation Profile、工资账户、地区比例方案和生效月份；
- 标准比例方案保存权威来源、地区、基数上下限、个人／单位比例、舍入和不可变 revision；自定义方案从标准版本复制，不改平台标准；
- Workbench 成员页面组合两类 Owner，分别授权和提交，不形成跨库事务假象。

验收门：新增／离职／重新入职、薪酬生效月、账号缺失、账户历史、隐私遮罩、撤权和双租户隔离全部通过；People 变化不改写 Payroll 历史，Payroll 变化不改写 People。

### P4：工资表、核对与实际发放（已通过）

状态：2026-10-25 本地通过；未部署 D1／公网。完整证据见[P4 验证报告](../../reports/oms-v1-iteration-p4-payroll-execution-validation-20261025.md)。

覆盖：BF-186、BF-189、BF-204～BF-215。

交付：

- 常规工资表月度唯一、成员范围、薪酬快照、五险一金计算、外部个税快照、绩效／调整录入和导入预览；
- 待核对、无需审批、待发放、部分发放、已发放、无需发放、待扣回、作废／替代及调整批次状态机；
- 同步新成员／薪酬档案仅在无任何成功／未知付款时追加 revision；
- 逐人／批量发放作为独立 Money Payment Delivery 编排，部分失败不回滚成功项。

验收门：整数计算和舍入 golden tests、同月并发创建、幂等重放、结果未知恢复、零／负实发、超额、账户缺失、部分发放、作废／替代、补发／扣回和本人已发工资可见性全部通过。

### P5：财务申请与付款确认

状态：2026-10-25 本地通过；未部署 D1／公网。完整证据见[P5 验证报告](../../reports/oms-v1-iteration-p5-finance-payment-validation-20261025.md)。

覆盖：BF-185、BF-191～BF-203、BF-223。

交付：

- 报销／付款申请草稿、明细、类别字典、收款人／Party 或一次性收款快照、费用归属、Evidence 引用和 revision；
- 分类型审批策略并冻结 `not_required` 快照；撤回修订、附件规则、敏感账户门禁与审计；
- 财务逐笔／批量确认实际付款，保存 Money payment／allocation 稳定引用，投影待支付／部分支付／已支付／超额异常；
- Workbench 完成成员本人、财务全量、敏感读取／导出和付款确认的独立能力界面。

验收门：申请人／收款人／操作人分离、附件与总额、整单核销、分次／超额付款、批量部分成功、撤回边界、敏感信息、跨租户和恢复全部通过。

### P6：员工 API Key 与开放 API

状态：2026-10-25 本地通过；未部署 D1／公网。完整证据见[P6 验证报告](../../reports/oms-v1-iteration-p6-open-api-validation-20261025.md)。P6 只建立凭证、入口和审计底座；下列报销业务写命令的幂等与恢复由 P7 实现。

覆盖：BF-216～BF-233。

交付：

- 登录 Workbench 中的 Key 申请／管理页；Key 绑定 Tenant、Subject、Person 与当前能力，最多 5 个，有效期 30／90／180／365 天且默认 180；
- secret 只显示一次，服务端保存不可逆 verifier；创建、撤销、轮换、到期和调用审计进入 IAM／凭证域；
- Gateway 增加 `/open-api/v1/` allowlist、Key 验证、账号／Grant 复验、每 Key／Tenant 限流、请求 ID 和统一错误；
- OpenAPI、中文说明、脱敏 `curl` 示例、Key 使用记录与 Webhook“暂未开放”占位；
- 所有写命令强制 `Idempotency-Key`，未知结果通过原键或业务 ID 恢复。

验收门：secret 不落日志／数据库明文／示例，撤销和停用立即拒绝新调用；限流、过期、轮换、同键同内容／异内容、错误清洗、分页／ETag、本人资源和双租户测试通过。

### P7：员工发票报销闭环

状态：2026-10-26 本地通过；未部署 D1／公网。完整证据见[P7 验证报告](../../reports/oms-v1-iteration-p7-invoice-reimbursement-validation-20261026.md)。

覆盖：BF-221～BF-234。

交付：

- `/open-api/v1/` 提供 Evidence 上传、草稿、明细、附件、提交、撤回修订、本人列表／详情；
- 发票格式／容量／安全扫描、强重复阻断、疑似重复人工核对、未引用文件保留与清理；
- 提交冻结并进入无需审批待支付，授权财务人员确认后由 Money 建立唯一实际付款／核销；
- Internal Finance Requests 保存 Money 引用并让员工本人 API 查询到已支付。

验收门：用真实 HTTPS、IAM、Gateway、三个 Owner 和 PostgreSQL 完成一张真实测试发票闭环；同时注入 Evidence／Finance／Money 故障，证明原幂等键恢复、无重复资金；撤权、越权、跨租户、重复和不安全文件负例全部通过。

### P8：Feedback SDK V2

状态：2026-10-26 源码／构建及本地真实 Chromium 跨 origin 宿主门禁通过，外部运行验收待执行；未部署 D1／公网。证据见[P8 源码报告](../../reports/oms-v1-iteration-p8-feedback-v2-source-validation-20261026.md)。

覆盖：BF-235～BF-237。

交付：

- 正式 `feedback.use` Capability 和主 OMS1 侧栏“产品反馈”入口；
- 部署配置注入 SDK URL、Gateway URL、Project ID 115、通知开关和项目专用 Key；Key 不进入 Git／日志／URL；
- 服务端以版本化密钥算法从 Tenant＋Subject 派生稳定脱敏 `customUserId`；
- 单一跨域 iframe、精确 postMessage 校验、提交／状态切换、关闭、账号切换清理和未读角标；
- Arckit SDK 发布配置加入 OMS origin，Feedback 服务为项目 115 开启通知；
- 预留从 `apiKey` 直连迁移 15 分钟 Session 的配置边界，不改变用户历史。

验收门：两个账号／两个租户的反馈与未读隔离、附件、开发者回复、单条已读、错误 Key／Project／origin、服务不可用降级、Key 轮换和退出登录清理全部通过；OMS 其他页面不因 Feedback 故障变慢或不可用。

### P9：D1 全组合与交付证据

交付：

- D1 增加两个服务、两个数据库、路由、Grant bootstrap、备份恢复与镜像清单；
- 本地与 D1 使用同一 API、migration 和镜像，配置只决定端点和放置；
- 连续完成 Tenant 创建、成员、薪酬、工资、财务申请、开放 API 报销闭环和 Feedback；
- C0／C1／C2／C3／C4／C5 运行驱动依次绑定固定证据摘要；C2 覆盖成员稳定引用到薪酬档案，C3 用明确非法律依据的六项零比例 fixture 验证工资周期、快照、无需审批与未支付边界，C4 以付款申请验证 Person 快照、无附件显式确认、无需审批与租户隔离，C5 验证默认 180 天员工 Key、证据外一次性 Secret、自助脱敏列表、身份绑定、Webhook 占位与本人调用审计；C3 不冒充真实缴费规则或发薪，C4 不放松报销 Evidence 规则或冒充实际付款，C5 不把 Key 建立冒充发票报销业务写；
- 生成逐项证据报告、回滚点、镜像 digest、容器健康、数据库恢复和浏览器 console 记录。
- 新联合备份在停写窗口内生成无秘密数据库事实指纹；空目标恢复必须在业务服务启动前与原 checkpoint 严格一致，随后仍要完成 Evidence 字节和关键业务读取／续写。

验收门：所有声明容器健康只是前置；必须在恢复前后重复关键业务旅程并核对 Tenant、ID、revision、金额、文件、Money 引用、Key 非秘密元数据、反馈身份映射和审计。未取得真实环境证据不得宣称整轮完成。

## 5. 测试与提交策略

- 每个阶段先写契约／状态机／授权负例，再写实现；Owner 单元测试、PostgreSQL 集成测试、Gateway 契约测试、Workbench DOM／浏览器测试和 D1 运行测试分层执行。
- 一个阶段一个可回滚提交组；migration 与代码同提交，部署清单和恢复脚本同阶段完成，不把不可运行的半个服务合入下一阶段。
- 新增全局 `make` 验收目标只组合已经存在且可独立运行的检查；禁止用跳过、mock 成功或仅字符串搜索替代真实数据库／跨服务验证。
- 每阶段完成后更新 `docs/testing.md`、`docs/progress.md` 和对应报告；未实际执行的命令保留为待办，不写“通过”。

## 6. 开工条件

以下条件同时满足才从 P0 进入 P1：

1. Feedback 自动诊断上下文已由 BF-237 明确；
2. 本轮迭代规格不存在第一版产品行为待确认项；
3. 用户已授权需求整体完成后按拆解实施；
4. 当前工作区文档变更提交为可追溯基线，且没有有效凭证进入 Git；
5. 现有基线测试通过或把已存在失败准确登记为非本轮引入。
