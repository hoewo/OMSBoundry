# Party Directory 架构决策

状态：✅ 已采用（独立核心与交付恢复已实现，D1／消费者迁移待验）

日期：2026-09-25

## 问题与可验证结果

Contract、Money Flow 和客户装配分别维护相同外部主体时，会形成重复档案、重复修订规则和不稳定的跨系统映射 → Party Directory 独立拥有主体当前档案，各业务应用通过版本化契约引用主体并保存业务发生时的不可变快照。

## 决策

Party Directory 是独立的数据所有者和可单独使用的领域应用。它可以作为客户选择的主体管理主要产品，也可以作为 Contract Ledger、Money Flow Ledger、Project Workspace 等其他领域应用的已声明依赖随交付包一起安装。“主要／依赖”是具体组合中的角色，不是 Party 或消费应用的永久等级。

“小应用可以独立安装、运行和交付”表示应用边界、数据所有权、升级与失败行为可以独立验证，不表示单个业务应用必须在没有其他应用依赖的情况下以单进程运行。依赖必须显式写入 Suite Manifest、版本化契约和交付验收，不得通过共享表、共享 ORM、隐式网络地址或人工同步形成。

当前 Contract Ledger 与 Money Flow Ledger 各自实现的轻量 Party 目录继续属于现有实现事实，直到独立 Party Directory、迁移和兼容验收完成。目标边界不把文档结论伪装成已经完成的数据迁移。

## 数据所有权

Party Directory 拥有主体的当前身份档案、稳定 Party ID、revision、名称与登记标识等跨业务一致事实。主体类型可以覆盖组织和个人；新增属性只有在 Party 自身闭环和真实场景中稳定后才进入内核。

消费应用继续拥有以下事实：

- 主体在合同、义务、资金、项目或其他业务记录中的角色；
- 业务确认时使用的 Party ID、Party revision 和不可变主体快照；
- 业务记录自身的状态、revision、权限、不变量和审计；
- 客户专用称谓、流程与只在单一业务上下文成立的属性。

Party 当前档案更新不能改写既有合同、义务、资金或项目历史。业务应用不能直接读写 Party 数据库，也不能导入 Party 的内部 ORM 或领域对象。

## 组合与界面

客户装配决定 Party 管理能力如何暴露：

- 需要主数据管理的客户可以直接使用 Party Directory 的管理界面；
- 只需要快速录入的客户可以隐藏管理入口，在业务表单中提供自由输入或自动补全；明确允许临时主体时，消费应用保存无 Party Ref 的本地业务快照，而不是暗中建立第二套当前目录；
- 隐藏入口不改变已登记 Party 的数据所有权，也不强迫所有业务记录必须先建立主数据；
- 一个只交付 Contract 或 Money Flow 业务结果的产品包可以同时携带 Party、IAM 和该结果需要的其他显式依赖。

Party 的搜索、读取和引用使用版本化 API／Port；主体变化的异步传播可以使用事件。通用连接器不得直接复制 Party 内部表，也不得覆盖业务应用已经冻结的历史快照。是否允许“仅保存名称、不建立 Party”的临时主体由消费应用规格明确，并且不能把这些快照反向宣称为 Party 当前档案。

## 与其他应用的边界

- IAM 拥有租户、账号、登录和能力授权；Party 不拥有密码、会话或授权结论。
- People & Groups 拥有内部 Person、Group、Membership 和名册快照；Party 表达参与外部业务的法律或交易主体，两者不因名称相似而合并。
- Contract 拥有签约角色和签约时快照。
- Money Flow 拥有账簿所有者、债权方、债务方、收付款方角色和形成时快照。
- Project 拥有项目中的甲方、资质单位或其他上下文角色，不把这些角色写入 Party 通用内核。

## 采用理由

Contract 与 Money Flow 已出现两个稳定使用点：两者拥有相同的 organization／individual、法定名称、登记标识、revision 和列表／登记能力，并分别冻结业务快照。参考 OA 又证明同一主体能力在客户界面中可以被淡化为历史名称自动补全，而其他客户仍可能需要完整管理界面。

继续复制应用内目录会让新增应用重复实现主体 ID、revision、审计、去重和迁移，并要求连接器长期维护多个主体 ID 的对应关系。独立 Party Directory 把这类可预见耦合收敛在公开契约中，同时保留业务角色和历史快照的本地所有权。

## 不采用的边界

- 不使用所有应用共同读写的一张 Party 表或跨库外键；
- 不把 Party 做成客户字段、万能字典或任意主数据平台；
- 不把 Party 管理界面强制暴露给所有客户；
- 不用 Party 当前资料替代业务记录的历史快照；
- 不因 Party 可以单独使用，就把 Contract、Money Flow 或 Project 的业务角色迁入 Party。

## 实现状态

第一版产品生命周期、当前投影与不可变 revision 模型、版本化 API、IAM Capability、精确工作负载引用、Suite Manifest 依赖、故障行为和本地目录迁移边界已经定义。`backend/services/party-directory` 已实现独立 Go／PostgreSQL 核心运行时，并通过字段规范化、命令幂等、并发创建、Expected Revision、更正、停用／恢复、稳定查询、不可变历史、跨租户拒绝、IAM Audience／Action／Scope 和精确引用的自动化与真库检查。独立 release 的版本／校验和／契约随包、非 root PostgreSQL 18 镜像、快照一致全量备份、空目标恢复、非空目标保护和恢复后继续写入也已通过真实容器验收。首个 Project 消费者已进入 D1 正式 API 链，并通过请求绑定的精确 Party Scope 凭证、Party 目标最终授权、依赖停机／恢复与联合空目标还原验收。管理页和 Project 客户选择已进入 Workbench：用户凭证只负责发现候选，Project workload credential 负责保存前精确复验；首个切片不提供改绑／解绑。Contract／Money 消费者迁移和完整双租户矩阵仍未实现；后续验收必须继续证明历史 Party ID、revision、快照与审计不被改写。
