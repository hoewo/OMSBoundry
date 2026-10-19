# 参考 OA 高保真业务架构 Demo

## 元信息

- 路径：`spec/assembly/reference-oa-demo.md`
- 所属模块：`assembly`
- 状态：🟢 Level B 高保真本地业务架构 Demo、首个实际组合变化的定制化归层及当前高保真可移植开发包已通过；完整入口链、真实公司 Ubuntu 和跨客户适配待独立验收
- 优先级：P0
- 依赖能力：Project Workspace、设计项目运营扩展、People & Groups、Contract Ledger、Money Flow Ledger、Evidence Service
- 关联技术：`tech/delivery/d1-environment-promotion.md`
- 上层验收：[小系统群 Demo Validation 2](./demo-validation-2.md)；本规格是其 Level B 当前回归基线

## 功能目标

参考 OA Demo 使用可独立运行的小系统完成项目立项、经营合同与回款、设计生产交付、资料追溯四条业务旅程。Demo 验证业务事实的责任边界和系统组合能力，不以公网入口或统一登录是否完成作为业务架构成立的前提。

## 运行模式

### 本地业务验证模式

- 每个受保护的小系统使用版本化 IAM 契约的本地模板 Provider；不存在关闭鉴权的运行方式。
- 各小系统通过独立本地地址访问，不要求 Nginx、Gateway、中心 IAM 或公网域名参与。
- 各应用拥有独立数据库、migration、配置、会话、发布物和恢复边界。
- Demo 使用本地 Evidence Provider 保存演示文件；业务应用只保存 Evidence Reference 和业务含义。
- Demo 数据全部为人工构造的非真实客户数据。

### 完整组合验证模式

- 同一业务能力通过 `Nginx → Gateway → IAM → 目标应用` 访问。
- 完整组合复用本地业务验证已经通过的应用版本、API、migration 和数据边界。
- 入口组件不获得 Project、合同、资金、人员或文件业务事实的所有权。

## 核心旅程

### J1：项目立项与责任建立

**问题 A**：项目基础资料、负责人和参与组散落，无法形成可追溯的工作上下文。

**可验证结果 B**：用户建立一个设计项目，补全最小基础资料，为人员或 Group 建立带有效期的 Assignment，并能看到项目自身的变更历史。

**规则**：

- Project Workspace 拥有 Project、基础资料、Assignment、外部 Resource Link 和项目自身历史。
- Assignment 引用 People & Groups 的稳定 PersonRef、GroupRef 或 Roster Snapshot，不复制人员名册。
- 第一生产类型固定为“设计”；第二生产类型不属于本 Demo 的通过条件。
- Project Workspace 未运行时，People & Groups 仍能独立管理人员与组。

**验收**：

- 从空数据库创建项目草稿并完成立项。
- 为项目建立、变更和结束责任分配，历史记录保持可解释。
- 重复命令、过期 revision 和无权限命令得到确定拒绝。

### J2：经营承接、合同与回款

**问题 A**：项目、合同依据、应收义务和实际回款缺少可追溯连接，容易由一个系统越权代替另一个系统形成事实。

**可验证结果 B**：用户从项目关联正式合同，明确确认应收义务，登记实际收款并完成核销，项目只展示来自各事实所有者的当前结果。

**规则**：

- Contract Ledger 独占合同及其 revision、结算、关闭和合同证据。
- Money Flow Ledger 独占已确认义务、实际资金、核销、余额和纠错。
- Project Workspace 只保存类型化 Resource Link 和来源快照引用，不复制合同或资金事实。
- 合同或结算事实不会自动生成正式义务；责任人必须明确确认，Money Flow 才保存义务。
- 第一版不建立自动 Contract → Money Flow 连接器。

**验收**：

- 项目可以关联一个已生效合同并显示稳定引用。
- 人工确认前 Money Flow 不存在正式义务；确认后来源快照可追溯。
- 收款、部分核销、余额和错误纠正继续由 Money Flow 的既有规则控制。
- Contract 或 Money Flow 不可用时显示明确失败，不伪造项目侧成功。

### J3：设计生产组织与成果交付（已完成独立业务切片）

详细边界、状态机和可执行验收见 [Design Operations：设计生产与交付](../design-operations/design-production-delivery.md)。独立运行时、Project／Assignment／Evidence reference-read 和 J3-A01～J3-A20 已通过；J3 仍需在 J2／J4 操作者旅程中被组合验证，不能单独代表本 Demo 完成。

**问题 A**：设计生产阶段、负责人、成果和完成条件没有稳定责任位置。

**可验证结果 B**：用户在设计项目运营扩展中建立一条最小生产工作项，推进阶段，登记交付成果，并以明确条件完成该工作项。

**规则**：

- 设计项目运营扩展拥有设计类型、生产阶段、工作项、阶段状态和成果适用性。
- 通用 Project Workspace 不增加设计行业专用字段或万能阶段字段。
- 负责人引用 Project Assignment，不复制 People Membership。
- 成果文件由 Evidence Service 拥有，扩展只保存 File ID、Version 和项目内业务含义。
- 奖金、薪酬、外委、生产成本、通用审批和客户专用公式不属于本 Demo。

**验收**：

- 一个已立项设计项目能够创建、推进并完成一条生产工作项。
- 缺失负责人、必需成果或不满足状态前置条件时不能完成。
- Evidence 不可用时成果登记失败并可重试，不产生虚假完成状态。

### J4：项目资料与状态追溯

**问题 A**：项目相关合同、经营、生产和文件事实分散，无法解释项目当前状态来自哪里。

**可验证结果 B**：用户从项目视角查看四类资源引用及其来源、版本、状态和失败信息，并能回到事实所有者完成纠错。

**规则**：

- 项目视图只组合公开 API、不可变快照或稳定引用。
- 状态展示标明来源应用和来源 revision，不把缓存冒充最新事实。
- 目标应用负责纠错；Project Workspace 不直接修改目标应用事实。
- 某个目标应用停机时，其余资源仍可查看，失败项显示不可用或待重试。

**验收**：

- 项目视图能够追溯负责人、合同、义务／回款、生产工作项和成果文件。
- 任一引用均能识别事实所有者和稳定 ID／revision。
- 单个目标应用停止后，不影响其他小系统自身闭环。

## Demo 通过条件

1. J1、J2、J3、J4 在同一组非真实演示数据上连续完成。
2. 每项写操作只进入事实所有者数据库，不存在跨库写入、共享 ORM 或跨库外键。
3. 每个受保护应用使用正式 IAM 契约的本地模板 Provider，没有鉴权旁路。
4. Contract、Money Flow、People & Groups 和 Evidence 仍可脱离 Project Workspace 独立启动和使用。
5. 停止任一非必需应用时，其他应用不产生虚假成功或静默写入。
6. Demo 可以在本地重新创建，并能使用相同应用发布物迁移到公司私有 Ubuntu 环境。
7. 迁移到完整组合时只替换入口、IAM Provider 和环境配置，不重写业务模型。

## Level B 高保真覆盖矩阵

| 旅程 | 同一业务故事中的状态变化 | 可观察结果 | 责任边界证据 |
| --- | --- | --- | --- |
| J1 责任建立 | 两名人员加入设计组，发布 revision 4 名册；负责人资料、IAM Subject 和组内角色变化，第二名成员结束任职并停用，再发布 revision 8 名册 | Project 的经营／生产 Assignment 仍保留最初 Person revision 1 与 Group revision 2 的显示快照；People 当前名册只剩一名 active 负责人 | IAM Subject 只作为 People 外部引用；Project 不复制人员名册，也不把后续人员变化写回旧 Assignment |
| J2 合同与回款 | 合同补充、更正、终止、阶段／最终结算和关闭形成 revision 1～4；人工确认应收后调额为 125,000 CNY，两笔回款合计 70,000 CNY；错误核销被反向并替换，55,000 CNY 减免后关闭，另有误录收款被冲销 | Contract 为 `closed / revision 4`，Obligation 为 `closed / revision 2`；原核销、反向事实、两笔有效核销和误录资金冲销都保留稳定 ID | Contract 不自动写 Money；Money 的义务由责任人以 Contract revision 4 作为人工来源依据后才成立；Project 只保存两类稳定引用 |
| J3 设计生产 | 方案成果被新 Evidence 替换；初设先明确不适用，客户变化后恢复并补交；施工图完成后因审查意见重开，再顺序完成后续阶段 | Design Work Item 最终 `completed / revision 18`；18 条历史同时包含旧成果、新成果、适用性恢复和返工原因 | Design 拥有阶段和纠错历史；Evidence 拥有三个 File Version；Project 只保存 Design／Evidence 引用 |
| J4 追溯与隔离 | 五类 Resource Link 回挂 Project；Contract、Money、People、Evidence、Design 逐项真实停机并恢复 | Project 保持 `active / revision 9` 和 9 条自身历史；对应实时引用才变为 `unavailable`，其余引用与两个 Assignment 快照仍可读 | 无跨库读取、共享 ORM 或项目侧事实修正；People 停机时五个 Resource Link 仍可用，证明责任显示来自已确认快照而非同步人员链 |

## 未覆盖、拒绝与待确认

- **明确未覆盖**：开票生命周期、管理费／客户公式、通用审批、外委／生产成本来源流程、奖金／薪酬／考勤、公司级统计、回款提醒、项目包／子项目和第二生产类型。
- **明确拒绝**：为 Demo 自动生成 Contract → Money 正式义务、把目标应用事实复制进 Project、跨库读写、关闭鉴权、万能流程或 OA 专用核心分支。
- **仍待确认**：真实客户对 J1～J4 细节的认可、第二个独立场景对边界的反证、统一 UI／UX、公司 Ubuntu 完整入口和个人试验田公网复验。

## 实现与验证证据（2026-09-25 Level B 已通过）

- `deployments/local-oa` 用六个应用进程、六个独立 PostgreSQL 数据库和独立数据卷运行本 Demo；应用间只通过版本化 HTTP reference-read 通信。
- `acceptance/reference-oa-demo` 使用同一组非真实数据连续完成上述高保真 J1～J4，实际形成 People revision 4／8 名册快照、Contract `closed / revision 4`、Money Obligation `closed / revision 2`、Design `completed / revision 18` 和 Project `active / revision 9`。
- 操作者使用 `./deployments/local-oa/ops/local-oa demo` 执行旅程；固定 Command ID 和本地 `0600` 续跑状态使同一旅程可重复执行，不需要编辑数据库或关闭鉴权。
- `demo` 在完全删除本组合的容器卷与运行状态后一次通过，并立即再次通过；第二次复用相同 Project、Contract、Money、Evidence 和 Design 身份，证明幂等重放没有制造第二套业务事实。
- `failure-isolation` 依次真实停止 Contract、Money、People、Evidence 和 Design：对应不可用引用数分别为 1、2、0、1、1；每轮 Project 都保持 `active / revision 9`、两个 Assignment 快照可读，未停止服务继续 ready，停止项恢复 healthy 后才进入下一轮。
- 组合验证发现 Design reference-read 只有 `work_item_id`、不满足 Project 统一 `id` 契约的问题；实现已追加统一 `id` 并保留 `work_item_id` 兼容字段，相关 Design／Project 测试通过。
- `v0.1.0-demo.11` Linux/amd64 开发包从同一独立归档完成两层摘要、离线镜像装载、新秘密初始化、六个独立产品的 fresh-state／run／replay／cleanup、两类可裁剪子集稳定重放、两次同 ID 高保真旅程、五来源隔离、最终 12 容器健康检查和显式零残留清理；归档不包含源码、运行状态或秘密，并携带只读 Ubuntu 目标机预检。
- 以上证据完成本规格的 Level B 本地业务架构与可移植开发包通过条件，但不证明 Nginx、Gateway、中心 IAM、Workbench、公网 TLS、真实公司 Ubuntu、正式 clean-HEAD 发布或生产可用性。

## 明确不负责

- 公网域名、TLS、Nginx、Gateway 和中心 IAM 的部署验收；
- 第二生产类型和跨类型复用结论；
- 自动 Contract → Money Flow 连接器；
- 项目包／子项目转换、完整开票、奖金、薪酬、考勤、外委、生产成本和公司统计；
- 客户真实数据导入、生产容量、高可用和 D2～D4 部署档；
- 证明整个商业圈子的通用产品边界。

## 失败判据

- 为完成 Demo 必须把合同、资金、人员或文件事实复制进 Project Workspace；
- 设计行业字段进入通用 Project 内核；
- 本地 Demo 依赖关闭鉴权、共享数据库或万能管理员；
- 目标应用失败时 Project 记录虚假成功；
- 为单一参考 OA 修改现有小系统稳定内核或维护 OA 专用核心分支。
