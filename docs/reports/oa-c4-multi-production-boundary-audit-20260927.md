# OA C4 多生产类型责任边界审计（2026-09-27）

## 判定

- 审计问题：设计、咨询、勘察、测绘应共享一个受控生产内核、分别成为独立应用、进入 Project，还是由通用工作流平台承载。
- 责任边界判定：采用一个独立的 **Production Operations** 数据所有者，以代码拥有、版本化、逐类型验收的 Type Definition 承载共同生产生命周期；现有 Design Operations 是 `design-v1` 的第一实现，必须兼容演进而不是另建平行可写事实。
- 第一条反证切片：`consultation-v1`，固定为“方案阶段（可明确不适用）→ 编制报告阶段（不可跳过）”。
- 当前判定：`partially-supported`；C4-A01～A14 当前本地轮次已通过，含正式 Suite Manifest／Workbench、完整未选缺席、Nginx→Gateway→IAM 主链、依赖故障恢复及联合空目标恢复；Production 专项撤权／账号停用、双 Tenant、真实浏览器与真实 Ubuntu 尚未通过。
- OA 完整体阶段判定：`insufficient-evidence`。

## 问题 A → 可验证结果 B

**问题 A**：参考 OA 把多种生产类型放在同一单体、同一数据库和同一项目页面中；若按表位置复制，会把共享数据库误当领域内核，若按页面栏目拆应用，又会复制负责人、阶段、成果、纠错、幂等和历史。

**可验证结果 B**：从已确认需求、运行源码和现有 `design-v1` 三组证据中识别真正稳定的共同不变量，选择信息增益最高的第二类型，并固定一组能证明“加类型而不改 Project／Evidence／IAM 核心、不复制应用、不造万能工作流”的实施与验收门槛。

## 假设

- H1：一个严格 Production Operations 内核 + 版本化 Type Definition。（采用）
- H2：每个生产类型一个独立应用。（当前不采用；若类型出现独立事实与生命周期可复审）
- H3：生产类型与阶段由 Project Workspace 拥有。（不采用）
- H4：建立用户可编辑的通用流程／低代码平台。（明确拒绝）

反证 H1 的条件不是“第二类型字段更多”，而是该类型无法用共同的 Work Item、负责人引用、顺序 Stage、适用性、Evidence 成果、revision、纠错和历史解释，或者它的关键事实必须独立迁移、独立完成且不能与其他类型共享同一一致性边界。

## 审计方法

1. 读取参考 OA 的确认需求，提取类型、阶段顺序、适用性、多类型并行和页面并集规则；
2. 读取参考 OA 的模型、服务和页面，区分“业务证据”与“单体实现位置”；
3. 读取现有 Design Operations 的模型、migration、API 和权限，识别可复用不变量与设计专用硬编码；
4. 比较四种所有权方案，使用数据所有权、独立闭环、变更扩散、失败隔离、迁移和权限作为判据；
5. 选择与 `design-v1` 形状差异足够、证据又足够明确的第二类型；
6. 为下一实现轮固定兼容迁移、负例、最小组合和回归门槛。

## 直接证据

| 证据 | 观察 | 对边界的影响 |
| --- | --- | --- |
| `docs/OA_2026_REQUIREMENTS_CONFIRMED.md:124-131` | 类型默认含设计、咨询、勘察、测绘，支持多选和后续扩展；不能用四个布尔字段或逗号串 | Project 只需拥有所选类型键；生产定义不应成为 Project 宽表字段 |
| 同文件 `167-182` | 四种类型有不同固定阶段；多类型分别显示、分别完成、互不覆盖 | 类型共享生产抽象，但每个 Work Item 必须有独立 frontier、revision 和完成结果 |
| 同文件 `198-207` | 页面按已选类型取栏目并集，同名栏目只显示一次 | 页面并集是组合交互，不是合并可写事实的理由 |
| `backend/internal/oa2026/service.go:647-651` | 参考代码把四种类型映射到不同阶段定义 | 支持“受控类型定义”，不支持复制四套应用 |
| 同文件 `663-728` | 相同顺序推进算法按类型分别计算 | 负责人、阶段推进和完成投影存在稳定共同内核 |
| `backend/internal/oa2026/models.go:69-110` | 适用性、完成证据和文件均带 `project_type_key + stage_key` | 类型必须成为事实身份的一部分；单一项目级布尔值不够 |
| `frontend/.../ProductionInfo.tsx:186-205,637-695` | 设计、勘察试验和报告栏目按类型组合；咨询／勘察／测绘共用“报告”视图 | 同名页面可以复用，但类型事实仍应独立 |
| `backend/services/design-operations/internal/design/model.go:12-43` | 当前领域模型已有 Work Item、Stage Definition、适用性与类型版本，但只允许 `design-v1` | 现有实现是可演进的第一类型，不是完整多类型内核 |
| `backend/services/design-operations/internal/database/migrations/000001_design_operations.sql:1-40` | 数据库约束把 type、stage 和 sequence 写死为设计 | 不能只加 UI 选项；需要兼容 migration 与真库升级证据 |
| `backend/services/project-workspace/internal/projects/postgres_repository.go:380-409` | 当前完成立项硬编码要求 `design` | C4 必须修正为“至少一个已支持生产类型”，同时不让 Project 拥有阶段规则 |
| 参考 OA `production/approval.md`、`cost.md`、`commission.md`、`bonus.md` | 批复审计、成本、外委、奖金横跨生产类型并有独立金额／责任规则 | 它们属于 C5～C7，不应为了 C4 塞进生产类型定义 |

## 方案比较

| 方案 | 共同不变量复用 | 类型差异隔离 | 主要风险 | 判定 |
| --- | --- | --- | --- | --- |
| 严格 Production Operations + Type Definition | 负责人、Stage、成果、revision、纠错、历史只实现一次 | 每个定义固定 key、版本、阶段、适用性和成果要求 | 可能滑向通用工作流，需硬门禁 | 采用 |
| 每类型独立应用 | 数据可完全隔离 | 最强 | 复制稳定生命周期、权限、部署和恢复；跨类型页面更复杂 | 当前证据下过度拆分 |
| Project Workspace 内置 | 进程少 | 弱 | Project 吸收行业阶段、文件语义和客户差异 | 不采用 |
| 通用流程／低代码平台 | 表面灵活 | 配置驱动 | 无业务不变量、难迁移、难授权、难解释历史 | 明确拒绝 |

## 目标责任边界

Production Operations 拥有：

- `Production Work Item`、所用 Type Definition key/version、独立状态和 revision；
- 每个 Work Item 的顺序 Stage 实例、frontier、适用性、完成／重开历史；
- 生产负责人 Assignment 引用；
- Evidence File ID／Version 的成果业务引用、替换链和最小显示快照；
- 命令幂等、不可变历史、当前允许动作和最小 reference-read。

它只引用 Project、active `production_owner` Assignment、Evidence File Version 和 IAM Capability Credential。它不拥有 Project 身份、人员、文件字节、批复审计金额、成本、外委、奖金、合同或资金。

Project Workspace 只拥有已选 `project_type_keys`，并验证至少一个当前受支持类型后完成立项；它不复制 Stage Definition。完整项目是否完成属于后续跨域只读投影，不在 C4 首切片中由 Production Operations 反向写 Project。

## 严格 Type Definition，不是万能工作流

每个定义必须随代码／契约发布并满足：

1. 稳定 `type_key` 和不可变 definition version；
2. 固定、有序的 stage key；
3. 每个 Stage 的可不适用规则和必需成果槽位；
4. 独立产品规格、权限矩阵和验收用例；
5. 既有 Work Item 永远按创建时版本解释；
6. 未安装、未知或未被 Project 选择的类型失败关闭。

第一版禁止管理员在线新增阶段、任意 JSON 条件、脚本、表达式、跨应用写动作和客户专用数据库列。若客户差异只是术语、顺序展示或角色映射，进入 Overlay；若稳定业务语义不同，增加新版本定义或复审独立应用边界。

## 第一条实现切片：`consultation-v1`

选择咨询而不是测绘，是因为两者当前形状相同，先做一个即可验证内核；选择咨询而不是勘察，是因为勘察的“试验阶段”可能产生尚未确认的结构化试验事实，当前直接实现会掩盖边界未知。

| 顺序 | Stage key | 含义 | 可设不适用 | 必需成果 |
| --- | --- | --- | --- | --- |
| 10 | `scheme` | 方案阶段 | 是 | `primary-output`（仅适用时） |
| 20 | `report` | 编制报告阶段 | 否 | `primary-output` |

`consultation-v1` 与 `design-v1` 在同一 Project 中必须形成两个独立 Work Item；完成、重开、成果替换、负责人变化和故障只影响目标 Work Item。报告页面可以组合展示，但不得把两条 Stage 状态合并为一个可写记录。

## 兼容演进门槛

- `design-v1` 的 ID、revision、Stage、成果引用、历史和 reference-read 在升级前后完全一致；
- 现有设计 API／Capability 在兼容期只代理 `design-v1`，不能借旧权限操作咨询；
- 新的通用 API／Capability 使用生产领域名，目标应用仍作最终授权；
- 旧物理表名可以在兼容 migration 中暂留，但只允许一个数据所有者和一份当前事实；禁止同步双写两套表；
- 相同 Project 可各有一个未取消 `design-v1` 与 `consultation-v1`，同类型重复仍冲突；
- 未选咨询的组合不显示、不创建、不要求咨询事实；卸下咨询 Overlay 不破坏设计 Work Item。

## 下一实现轮验收矩阵

| ID | 场景 | 必须证明 |
| --- | --- | --- |
| C4-A01 | 定义注册表 | design／consultation 固定定义可读，未知类型拒绝 |
| C4-A02 | 兼容升级 | 既有 `design-v1` 真库数据逐字段、revision、历史和摘要不变 |
| C4-A03 | 咨询创建 | active、含 consultation 的 Project 创建两阶段 Work Item |
| C4-A04 | 类型门禁 | 未选择 consultation、未知定义、错误 revision 零写入 |
| C4-A05 | 同项目双类型 | design 与 consultation frontier、revision、历史互不覆盖 |
| C4-A06 | 负责人 | 只接受同 Project active `production_owner`；结束后新写失败 |
| C4-A07 | 咨询适用性 | scheme 明确跳过、恢复后回退 report 完成投影且历史保留 |
| C4-A08 | 报告成果 | Evidence 精确引用、替换追加、故障零部分写、恢复单次成功 |
| C4-A09 | 顺序与纠错 | 不能越过 scheme；report 完成、重开和再次完成可追溯 |
| C4-A10 | 权限 | 生产责任角色允许；经营角色、Project B、错 Tenant／Audience／Scope 拒绝 |
| C4-A11 | 最小组合 | IAM／模板 + Project + Evidence + Production 可独立完成咨询闭环 |
| C4-A12 | 未选应用缺席 | design-only 与 consultation-only 均不依赖另一类型的页面或数据 |
| C4-A13 | 回归 | J3-A01～A20、Level A Design、当前 Level B J1～J4 全部保持 |
| C4-A14 | 反万能平台 | 无在线流程编辑、任意条件解释器、客户列、共享表或跨库 ORM |

## Base／Overlay 变化账

| 层 | C4 判定 |
| --- | --- |
| Base | Work Item／Stage／成果／负责人／revision／纠错／历史共同内核，以及版本化定义注册机制 |
| 类型化扩展 | `design-v1`、`consultation-v1` 的固定阶段与成果规则 |
| Customer Overlay | 启用哪些类型、页面术语／顺序、客户角色到稳定 Capability 的映射 |
| Suite Manifest | 是否安装 Production、Project、Evidence、Workbench 等应用 |
| 连接器／投影 | Project 页面组合多类型状态；不得反向拥有或改写 Production 事实 |
| 客户专项 | 一次性旧 OA 数据迁移或专属报表；不能进入公共状态机 |

## 第一实现轮：定义注册与兼容创建（2026-09-27）

### 本轮问题与范围

本轮只检验一个问题：在不改变旧 Design API 语义、不复制数据所有者的前提下，能否让同一领域内核按不可变定义创建 `design-v1` 和 `consultation-v1`，并让未知／未选类型失败且零写入。

本轮没有把负责人、成果、完整阶段推进、canonical Production API／Capability、D1 或页面混入同一实验；这些仍需独立轮次，避免某一层通过掩盖另一层失败。

### 实际变更

- 新增代码拥有的版本化 Type Definition 注册表；读取返回副本，运行时不能改写 Base 定义；
- `design-v1@1` 保留原四阶段，`consultation-v1@1` 固定为 `scheme@10 → report@20`；
- 领域 Service 创建时规范化并校验 `type_key`，读取 Project 的 `project_type_keys` 后再进入本地事务；
- PostgreSQL 创建逻辑按定义实例化 Stage，同 Project 不同类型可并存，同类型 active 唯一约束继续生效；
- 增量 migration 只放宽受控 type／stage 约束，不复制或重命名事实表；
- 旧 `/api/v1/design-work-items` 创建固定注入 `design-v1`，列表和资源写入口隐藏非设计 Work Item，旧 Capability 不能借此操作咨询。

### 可复核步骤与结果

```bash
go test ./backend/services/design-operations/...
go vet ./backend/services/design-operations/...
go build ./backend/services/design-operations/...

docker compose -f backend/services/design-operations/compose.yaml up -d postgres
TEST_DATABASE_URL='postgres://design_operations:design_operations_dev@127.0.0.1:55438/design_operations?sslmode=disable' \
  go test -p=1 -count=1 -tags=integration ./backend/services/design-operations/...
```

实际结果：普通测试、`go vet`、`go build` 和 PostgreSQL 18.6 串行集成测试全部通过。一次并行 package 运行在同时创建多个隔离测试库时触发测试辅助器 20 秒环境超时；迁移单包随即通过，随后 `-p=1` 完整套件通过，因此它被记录为测试基础设施拥塞而不是业务断言失败。集成测试从只应用 `000001` 的旧库插入真实 `design-v1` Work Item 与四条 Stage，再执行 `000002`；升级后 type、definition version、Project source revision、Work Item revision、current stage 和 Stage 数量保持不变。另一组真库测试证明：

- 同一 Project 成功保存一个 `design-v1` 和一个 `consultation-v1`；
- 咨询只实例化 `scheme=ready`、`report=pending` 两阶段；
- 第二个 active 咨询发生明确冲突；
- 未注册 `customer-defined-v1` 和 Project 未选择 consultation 均拒绝；
- 两次拒绝后 Work Item／Command 计数仍分别为 `2/2`，没有部分写入。

### C4-A01～A14 当前状态

| 验收项 | 状态 | 本轮证据／剩余缺口 |
| --- | --- | --- |
| C4-A01 定义注册表 | 通过 | 两个定义、版本、顺序、可跳过规则、未知 key/version 和不可变副本均有单测 |
| C4-A02 兼容升级 | 通过 | 旧 Work Item、Stage、Deliverable、Command、History 逐项和确定性 fixture digest 在真库升级前后不变 |
| C4-A03 咨询创建 | 通过 | active 且选择 consultation 的 Project 成功创建两阶段 Work Item |
| C4-A04 类型门禁 | 通过 | 未选类型、未知定义和错误 Project revision 均拒绝且 Work Item／Command 零新增 |
| C4-A05 同项目双类型 | 通过 | 同 Project 的设计保持 revision 2／scheme ready，咨询独立推进至 revision 12；两边历史分别为 2／12 条 |
| C4-A06 负责人 | 通过 | 跨 Project、错误角色和已结束负责人均失败关闭；恢复有效引用后相同命令只成功一次 |
| C4-A07 咨询适用性 | 通过 | scheme 可跳过；完成 report 后恢复 scheme 会回退 report 投影并保留既有成果与历史 |
| C4-A08 报告成果 | 通过 | Evidence 不可用零写入；恢复后同命令成功；替换保留 superseded 记录，当前成果唯一 |
| C4-A09 顺序与纠错 | 通过 | report 不能越过 scheme 决策；完成、重开、再次完成及较早阶段恢复均可追溯 |
| C4-A10 canonical 权限 | 正式入口主矩阵通过 | `production-operations` audience、`production-work-item.*` Capability、精确 Project／Work Item Scope、OA C4 Overlay 与 Project Assignment Policy 已通过契约、IAM、HTTP、真库及 D1 Nginx→Gateway→IAM 验证；生产负责人允许、经营负责人和相邻 Project 拒绝，IAM 故障零写入／恢复重试通过。Production 专项撤权／账号停用仍作补证 |
| C4-A11 最小组合 | 通过 | 本地模板与正式 D1 均只启动 People、Project、Evidence、Production 及各自数据库；consultation-only Project 完成报告闭环，Contract／Money 未运行 |
| C4-A12 未选缺席 | 通过 | design-only／consultation-only 本地数据和 API 双向缺席；隔离 D1 的 Suite Manifest、Workbench 与路由又证明未选 Contract／Money／Party／Business Engagement／Billing 缺席；联合空目标恢复后 canonical 摘要、成果和历史不变 |
| C4-A13 回归 | 通过 | `make check` 全门禁通过；六应用 Level A 从 fresh-state 到 replay／cleanup 一次性全部通过；当前 Level B J1～J4 连续两次稳定，Contract／Money／People／Evidence／Design 逐项停机时 Project revision 9 与 Assignment 快照保持，恢复后六应用全部健康 |
| C4-A14 反万能平台 | 本轮通过 | 没有动态流程编辑器、客户列、共享表、第二数据库或双写 |

### 本轮科学结论

第一条反证没有推翻单一 Production 内核：新增咨询类型只增加一个受控定义和兼容约束，没有要求复制服务、Project 表、Evidence 模型或 IAM 身份模型。但这只证明“类型可以被安全装入并创建”，不能外推为咨询完整闭环或 OA C4 已完成。下一轮必须让两个类型在同一 Project 内分别推进并比较 revision／history，优先寻找共享状态机是否因阶段差异而泄漏。

## 第二实现轮：双类型生命周期与失败恢复（2026-09-27）

### 本轮问题与反证目标

本轮检验共享生命周期会不会把咨询的两阶段规则泄漏到设计工作项，或者因负责人、Evidence、跳过和返工而产生跨类型 revision／历史覆盖。反证条件为任一咨询命令改变设计 Work Item、`report` 能在 scheme 未决时完成、不可用外部引用留下本地 Command／History、恢复适用性丢失既有报告，或成果替换覆盖旧引用。

### 实现收缩

- 写命令在成功重放检查之后、调用外部引用之前，按 Work Item 保存的 `type_key + definition_version` 校验目标 Stage 和 requirement；已成功重放仍不重新依赖外部服务；
- `CompleteStage` 从该版本 Type Definition 读取必需成果槽位，不再把 `primary-output` 作为散落的完成规则；
- applicability 在领域入口同时校验类型归属和 `skippable`，数据库事务继续作最终状态与 revision 防线；
- 没有增加第二服务、第二数据库、共享表、跨库 ORM、动态流程配置或双写。

### 可复核步骤

```bash
go test ./backend/services/design-operations/...
go vet ./backend/services/design-operations/...
go build ./backend/services/design-operations/...

TEST_DATABASE_URL='postgres://design_operations:design_operations_dev@127.0.0.1:55438/design_operations?sslmode=disable' \
  go test -p=1 -count=1 -tags=integration ./backend/services/design-operations/...
```

以上四项在 PostgreSQL 18.6 上全部通过。新增真库场景在同一 Project 创建设计和咨询后分别设置负责人：跨 Project Assignment、错误角色、已结束负责人、错误类型 Stage、不可跳过 report、提前完成 report 和不可用 Evidence 均被拒绝；失败后 revision、Command 和 History 不增加，引用恢复后原 Command ID 只成功一次。

咨询 Work Item 随后完成 `scheme not-applicable → report 成果 v1 → 成果 v2 替换 → report complete → reopen → complete → restore scheme → scheme 成果/完成 → report 再完成`，最终 revision 为 12、历史恰为 12 条。设计 Work Item 在整个过程中仍为 revision 2、`scheme=ready`、零成果、两条历史。旧库 fixture 同时包含 Work Item、四 Stage、Deliverable、Command 与 History；`000002` 前后确定性 digest 完全一致，且历史不可变 trigger 仍拒绝更新。

### 本轮科学结论

第二轮没有发现“共享生产内核必然导致跨类型污染”的反证：当前差异仍可被严格 Type Definition 表达，状态锁、revision、成果替换和历史都以 Work Item 为隔离单位。这支持继续采用单一 Production Operations，而不支持把结果外推到 IAM、安装裁剪、D1 或勘察结构化事实；下一高风险点转为旧 Design Capability 与新 Production Capability 能否真正隔离。

## 第三实现轮：canonical Production 权限隔离（2026-09-27）

### 本轮问题与反证目标

本轮检验兼容期的旧 Design 权限是否会成为跨类型后门，以及“生产负责人”是否被错误固化为目标应用内的客户岗位。反证条件为：旧 `design-work-item.*` 能操作 consultation、错误 audience／资源类型被接受、经营负责人自动取得生产写权限、相邻 Project／Work Item 可见，或 canonical API 把 `consultation-v1` 降格为无版本别名。

### 实现收缩

- 同一进程同时暴露旧 `/api/v1/design-work-items` 与 canonical `/api/v1/production-work-items`；旧入口固定 `design-v1` 并隐藏其他类型，canonical 创建必须显式给出已注册 `type_key`；
- 正式 IAM Provider 分离 `design-operations` 与 `production-operations` audience，后者只接受 `production-work-item.*` 和 `project`／`production-work-item` Scope；旧 action、旧 audience 和旧资源类型均失败关闭；
- Capability Manifest 以显式 `capability_namespace` 表达 audience 与资源导向 action namespace 的差异；这是向后兼容的可选契约字段，既有应用缺省仍使用 application ID；
- 参考 OA C4 Overlay 只把站点生产经理和 OA 业务管理员映射为完整生产能力，总经理只读，经营经理、财务和身份超级管理员没有生产动作；Project 级 `production_owner` 由独立 Assignment Policy 产生精确 Project Scope；
- 目标应用继续复验 action 与 Project／Work Item Scope，并返回精确 `consultation-v1` 的 canonical Reference；没有引入客户角色表、共享数据库或逐请求业务应用转发。

### 可复核步骤与结果

```bash
go test ./acceptance/access-contract \
  ./backend/platform/identity-access/internal/policy \
  ./backend/services/design-operations/...

go vet ./backend/services/design-operations/... \
  ./backend/platform/identity-access/internal/policy \
  ./acceptance/access-contract
go build ./backend/services/design-operations/...
go test -race ./backend/services/design-operations/... \
  ./backend/platform/identity-access/internal/policy \
  ./acceptance/access-contract

TEST_DATABASE_URL='postgres://design_operations:design_operations_dev@127.0.0.1:55438/design_operations?sslmode=disable' \
  go test -p=1 -race -count=1 -tags=integration ./backend/services/design-operations/...
```

以上全部通过。直接证据包括：

- Production IAM 接受正确 audience／action／Project Scope，拒绝 Design audience、Design action 和 `design-work-item` Scope；
- 同一 HTTP Server 的旧列表只返回 design，canonical 列表返回 design 与 consultation；旧能力访问 canonical 路由为 403，精确 Work Item Scope 允许本项并拒绝相邻项；
- 真库 canonical API 完成 consultation 创建、负责人、scheme 不适用、report 成果、完成和精确版本引用，最终 revision 5；同一 Work Item 从旧 Design 路由读取为 404；
- Project Assignment Policy 只让 active、当前 revision、同 Tenant／Project 的 `production_owner` 获得八项生产能力；经营负责人、相邻 Project、错 Tenant、stale 或 ended Assignment 均返回授权拒绝；
- Capability Manifest、Overlay 和策略示例均通过严格未知字段与跨字段契约测试。

### 本轮科学结论

没有观察到旧 Design 权限穿透 canonical Production 边界，也没有把客户岗位或 Project Assignment 复制到 Production 数据库。C4-A10 的本地目标端、Policy 与真库纵向证据通过，支持“稳定 Base Capability + Customer Overlay + 版本化业务事实 Policy”的解耦方式。它仍不等于正式入口通过：Nginx→Gateway→IAM 签发→Production Operations、撤权刷新、账号停用、IAM／Policy 故障和浏览器行为必须在 C4 D1 组合中取得直接证据。

## 第四实现轮：consultation-only 最小组合（2026-09-27）

### 本轮问题与反证目标

本轮只检验 C4-A11：不启动 Contract／Money，也不选择 `design` 时，People、Project、Evidence 与同一 Production 数据所有者能否从空库完成咨询交付，并在重复命令后保持相同 ID、revision 和持久状态。反证条件为 Project 把 `design` 写死成 active 前提、Compose 隐式启动未选业务应用、canonical 咨询数据泄漏到旧 Design 入口、重放产生新事实，或任一跨应用引用绕过公开 API。

### 实现与边界修正

- 真正发现并修复一个边界错误：Project `CompleteInitiation` 原先硬编码检查 `design`；现在只要求至少一个规范化项目类型。Production 创建时仍按自身不可变定义校验 `consultation-v1` 是否注册、Project 是否选择 `consultation` 及 Project revision，因此未知或未选类型仍失败关闭；
- 新增 `consultation-delivery` 驱动：建立咨询生产组、经营负责人、consultation-only Project、报告 Evidence 与 canonical Production Work Item，执行 `scheme not-applicable → report deliverable → report complete`，最终 revision 5、历史 5 条；
- 旧 `/api/v1/design-work-items/{id}` 返回 404，旧列表为空；canonical 列表只含该 `consultation-v1`；Contract／Money 服务未启动；
- Docker 本机默认地址池已耗尽，验收没有执行全局 network prune，而是给审计专用 Compose 覆盖显式分配 `10.252.20.0/24`～`10.252.26.0/24`；正常组合网络不变，审计结束仅删除自身临时项目和卷。

### 可复核步骤与结果

```bash
GOWORK=off go test ./acceptance/reference-oa-demo/...
GOWORK=off go test ./backend/services/project-workspace/...

TEST_DATABASE_URL='postgres://project_workspace:***@127.0.0.1:55499/project_workspace?sslmode=disable' \
  GOWORK=off go test -count=1 -tags=integration \
  ./backend/services/project-workspace/internal/projects

./deployments/local-oa/ops/local-oa composition-audit-consultation
```

全部通过。真库 Project 测试证明仅含 `consultation` 的 Project 在两个有效 Assignment 后从 draft 到 active revision 4。最小组合从空卷运行两次，均得到同一 Project、Evidence 和 Work Item；最终持久状态 SHA-256 为 `27ee80bd9f48058179da2973bd27382bdcece850a79bcbe9632241a2ea0f9777`。随后重跑 design-only 组合并增加 canonical 列表断言，两次结果只含 `design-v1`，状态 SHA-256 为 `f7baedf1acba990dbb42b95255fe7c877e2c63dae23b25f55e80fb4f8d8076a0`。本轮关闭 C4-A11，并把 C4-A12 提升为“双向本地数据／API 缺席通过、正式装配仍待验”；模板鉴权不冒充 Nginx→Gateway→IAM 正式入口，且尚未证明发布包、备份恢复或真实 Ubuntu。

## 第五实现轮：聚合回归与验证装配漂移修复（2026-09-28）

### 本轮问题与反证目标

本轮关闭 C4-A13：最近的 Project 类型中立化、consultation 增量和最小组合不得破坏六个独立 Demo、既有 Design J3 或完整参考 OA J1～J4。反证条件为任一应用不能从空状态运行／重放／清理，Design 纠错历史丢失，完整组合不能幂等复跑，或单个依赖停机改写 Project／Assignment 事实。

### 实际发现与修复

- People 独立 Demo 的浏览器断言仍期待已废弃英文 `IAM linked`，而实际模板已经是“已关联 IAM”；断言改为当前真实页面文案，人员与 IAM Subject 数据检查保持不变；
- Project 独立 Demo 的 Docker build context 仍停留在应用子目录，无法复制共享 `packages/workload-identity`；改为仓库根 context，并同步 `backend/services/project-workspace/Dockerfile` 路径；
- Project reference stub 仍返回 C3 之前的最小 Contract 引用，不能满足当前版本化公开契约；补齐 `schema_version`、`source_application`、摘要、币种和当前有效金额，不放松生产 validator；
- Docker 本机默认地址池耗尽；给 Project／Design 审计 Compose 分配独立 `10.253.30.0/24`～`10.253.35.0/24`，没有执行全局 network prune；
- 完整 OA 演示在保留数据库上暴露旧固定命令号与新 Project 类型载荷冲突；将依赖的 Project／Design 命令集升级为第二版命名空间，并把 Project 业务编号升级为 `OA-DEMO-2026-002`；
- 新 Project Assignment 不再写死 People revision 1/2，而是在建立引用前读取当前 Person／Group revision，并断言该次实际快照在后续 People 变化后保持不变。旧冲突保护、目标应用最终校验和不可变快照均未绕过。

### 可复核步骤与结果

```bash
make check
./deployments/demos/ops/level-a audit --confirm-destroy-demo-data
./deployments/demos/ops/level-a status

./deployments/local-oa/ops/local-oa up
./deployments/local-oa/ops/local-oa demo
./deployments/local-oa/ops/local-oa demo
./deployments/local-oa/ops/local-oa failure-isolation
./deployments/local-oa/ops/local-oa verify
./deployments/local-oa/ops/local-oa status
```

全部通过。Level A 六应用均完成 `fresh-state → run → replay → cleanup`，Design Work Item 两次均到 `cancelled / revision 21 / history 21`，Project 两次均到 `cancelled / revision 9 / history 9`。Level B 使用 Project `426b18f8-ec92-4e97-8519-215dc64adf7f`，两次均完成 J1～J4、Project revision 9、五条稳定跨应用引用与两条不可变 Assignment 快照；状态摘要 SHA-256 为 `631f231a797dfc8c2b76c94b9d238fda10355c934278205f615b6e78792e26d9`。Contract、Money、People、Evidence、Design 逐个停机时 Project 均保持 active revision 9，引用按来源局部降级，恢复后六应用全部 healthy。C4-A13 因此关闭；这些本地模板／直连证据不替代正式 Nginx→Gateway→IAM、D1、浏览器、恢复和真实 Ubuntu 证据。

## 第六实现轮：正式最小组合、未选缺席与恢复（2026-09-28）

本轮用隔离 D1 只装入 People、Project、Evidence、Production，以及不可关闭的 IAM、Gateway、Workbench、Nginx。49 个正式入口断言、0 个状态不匹配，证明 Workbench 只显示 Production 和已选依赖，Contract／Money／Party／Business Engagement／Billing 页面及路由缺席；生产负责人取得精确 Project Scope，经营负责人和相邻 Project 被拒绝。Evidence 与 IAM 分别停机时命令返回 503 且零部分写入，恢复后同一 Command 只提交一次。

随后联合备份 IAM 签名材料、People、Project、Evidence 与 Production，销毁隔离项目专属卷并恢复。咨询 Work Item 的 canonical 状态摘要恢复前后同为 `592c166db157ee72840f198b3496cab5a8171d86daca08df4259b66645db4e7b`，revision 5、一个成果、5 条历史不变；设计 Work Item 和 IAM 故障恢复创建的相邻 Project Work Item 也保持。恢复演练发现并修复 Docker edge-network 端点释放竞争：只在确认项目已无容器时容忍复用残留网络，不执行全局清理。

Production 首次把 application audience 与 action namespace 分离，暴露了旧契约用 audience 前缀校验动作的错误。Capability Manifest／Suite Manifest 现可显式声明 `capability_namespace=production-work-item`；Gateway、Workbench、IAM Catalog 使用该声明，而 Grant／Policy／Workload 仍必须命中目标 Capability Catalog 的精确动作。本轮因此关闭 C4-A12，并把 C4-A10 提升为正式入口主矩阵通过；完整证据见[正式最小组合与恢复实验](./oa-c4-formal-minimum-composition-experiment-20260928.md)。

## 未知与限制

- 勘察试验是否只有文件成果，还是需要样点、指标、设备、复核等结构化事实，尚未确认；因此不把 `survey-v1` 冒充已设计完成。
- 测绘当前与咨询同为“方案→报告”，但没有第二份独立业务证据证明成果要求完全相同。
- 设计前三阶段的校审单和评分在参考旧规格中是必填，而当前 `design-v1` 只固定 `primary-output`；这是独立的设计类型增量，不用咨询切片顺手改写。
- C5 批复审计、C6 成本外委、C7 奖金不是 Production Type Definition 的字段；它们仍需各自责任边界审计。
- 六轮实现已运行代码、migration、完整咨询生命周期、canonical Production 权限／Policy、最小组合、PostgreSQL 18.6、完整 Level A／B 聚合回归、隔离 D1 正式入口、未选缺席和联合空目标恢复；尚未完成 Production 专项撤权／账号停用、双 Tenant、真实浏览器与真实 Ubuntu，不得把 `partially-supported` 写成 C4 或 OA 整体通过。

## 下一轮可执行入口

下一轮优先补 Production 专项 Grant 撤销、Assignment 结束、账号停用、真实浏览器和双 Tenant，再在可用公司／个人 Ubuntu 环境复演同一 pull-only 发布物；C4-A13 聚合回归继续作为每轮必须保持的基线。任何需要共享表、双写、任意工作流或破坏 `design-v1` 历史的实现都应立即停止并把边界判为失败，而不是继续补丁化。
