# Party Directory 主体档案生命周期

## 元信息

- 路径：`spec/party-directory/party-lifecycle.md`
- 所属模块：Party Directory
- 状态：🟡 独立运行时、可操作管理页面、首个消费者、D1 恢复及台账含引用数据运行时切换已实现；Money 完整 D1 消费链和双租户组合验收待完成
- 关联决策：BF-036
- 关联模型：`Party.yaml`、`PartyRevision.yaml`
- 关联 API：`party-create.yaml`、`party-list.yaml`、`party-get.yaml`、`party-update.yaml`、`party-status-change.yaml`、`party-history-list.yaml`、`party-reference-get.yaml`

## 问题与可验证结果

多个业务应用分别维护同一个外部主体，会重复实现主体身份、修改历史和跨系统映射，并使合同、义务与项目引用无法稳定解释 → Party Directory 以稳定 Party ID 和不可变 revision 管理当前主体档案，消费应用通过版本化引用读取当前资料，并在业务确认时冻结本应用拥有的主体快照。

该功能既支持把主体管理作为客户购买的主要业务结果，也支持把 Party Directory 作为 Contract、Money、Project 或 Business Engagement 的显式直接依赖。组合角色不改变主体档案的数据所有权。

## 第一版用户结果

具备相应 Capability 的用户能够：

1. 登记组织或个人主体；
2. 按名称、登记标识、类型和状态查找当前主体；
3. 查看稳定 Party ID、当前 revision 和完整修订历史；
4. 以期望 revision 更正当前名称或登记标识；
5. 停用不再允许形成新业务引用的主体，并在需要时重新启用；
6. 在消费应用中选择 Party，并确认该业务发生时的 Party revision 与主体快照。

Party Directory 不创建合同、义务、项目角色、登录账号或客户专用业务身份。

## 主体事实

### 稳定身份

- Party ID 是租户内稳定 UUID，不使用名称或登记标识充当身份。
- Party 类型只有 `organization` 和 `individual`。
- 法定名称或姓名去除首尾空白后长度为 1 至 200 个字符。
- 登记标识可空；非空值去除首尾空白后长度为 1 至 100 个字符。
- 登记标识不声明全局唯一，也不执行国家、证件种类或真实性核验。
- 相同名称或登记标识不触发自动合并；系统可以展示重复候选，但不能替用户决定两个主体相同。
- 客户、甲方、资质单位、供应商、施工方和收款人不是 Party 类型，而是消费应用拥有的上下文角色。

### revision 与当前投影

- 新主体从 revision 1 开始，状态为 `active`。
- 每次资料更正、停用或重新启用都追加恰好一条连续 revision，并原子更新当前投影。
- revision 保存当时的类型、名称、登记标识、状态、可信 Actor、命令标识、原因和发生时间。
- 历史 revision 不可更新或删除。
- 当前资料变化不修改消费应用已经冻结的合同、义务、资金、项目或经营承接快照。

## 登记主体

登记命令包含 Command ID、类型、名称和可选登记标识。服务端从 IAM Principal 取得 Tenant 与 Actor，不接受客户端覆盖。

- 相同 Tenant、Command ID 和标准化负载重放返回原 Party，不增加 revision 或审计。
- 相同 Command ID 改变业务负载或 Actor 时返回幂等冲突。
- 并发相同命令只形成一个 Party 和 revision 1。
- 同名主体可以被登记；响应不得暗示系统已完成去重或真实性确认。
- Party、revision、命令记录和审计必须在一个事务内提交。

## 更正当前资料

更正命令包含 Command ID、Expected Revision、完整替换后的类型、名称、登记标识和更正原因。

- Expected Revision 必须等于当前 revision；过期值返回冲突并返回最新 revision，不产生部分写入。
- 成功更正形成当前 revision + 1，旧 revision 保持可读。
- 类型变化属于显式更正，必须保存原因；不能用删除再新建绕开历史。
- 标准化后事实没有变化时不创建新 revision，并返回 `NO_EFFECTIVE_CHANGE`。
- 更正不会向消费应用推送覆盖命令，也不会修改既有业务快照。

## 停用与重新启用

状态命令包含 Command ID、Expected Revision、目标状态和原因。

- `active → inactive` 表示该 Party 不再允许形成新的正式业务引用。
- `inactive → active` 表示恢复新引用资格，不改变旧业务快照。
- 状态变化形成下一 revision；相同状态返回 `NO_EFFECTIVE_CHANGE`。
- `inactive` Party 仍可按 ID 读取、查看历史和解释既有业务记录。
- Party Directory 不删除主体。合并、拆分和误建主体处置不在第一版内，不能用停用冒充合并。

## 查询与引用

### 人类查询

- 列表默认只显示 `active`，可显式筛选 `inactive` 或全部状态。
- 可按主体类型和标准化后的名称／登记标识子串查询。
- 排序固定为名称标准化值、名称显示值、Party ID，分页游标绑定全部筛选条件。
- 列表和详情返回当前 revision、状态与服务端计算的 `allowed_actions`。
- 历史按 revision 倒序返回，并能够下钻每次变化的 Actor、原因和事实快照。

### 消费应用引用

- 消费应用使用独立工作负载身份换取 `party-directory.reference.read` 的精确 Party Scope 短期凭证。
- 引用读取返回 Party ID、Tenant、类型、名称、登记标识、状态和 revision，不返回 Party 数据库结构、审计内部字段或其他客户资料。
- 新业务确认必须要求 `active` Party，并保存返回的 Party ID、revision 和当时档案快照。
- 消费应用在草稿阶段可以重新读取当前 Party；正式确认后只读取本地快照解释历史。
- Party 暂时不可用时，既有正式业务快照保持可读；需要创建或刷新 Party 引用的新命令失败关闭，不能以陈旧缓存冒充新鲜确认。

## 权限与审计

第一版发布以下稳定 Capability：

- `party-directory.read`：查看和搜索当前档案；
- `party-directory.create`：登记主体；
- `party-directory.update`：更正当前资料；
- `party-directory.status.manage`：停用或重新启用；
- `party-directory.history.read`：查看不可变 revision 历史；
- `party-directory.reference.read`：供已声明消费应用读取精确引用。

IAM 管理账号、站点角色与 Capability Grant；Party Directory 复验 Tenant、Audience、Action、Scope 和时效，并结合 Party 当前状态与 Expected Revision 作最终授权。身份管理员不会因管理账号自动获得主体业务权限。

每次成功写入审计至少保存 Tenant、Subject、Party ID、Command ID、来源 revision、结果 revision、动作、原因、时间和 Credential ID。拒绝、幂等重放和无效变化不伪装成新的业务 revision。审计不得记录 Session、JWT、Cookie 或工作负载私钥。

## 页面与恢复行为

- Workbench 仅在 Suite Manifest 启用 Party Directory 且用户可读时显示入口。
- 列表页提供搜索、状态筛选、主体登记和记录入口；无写权限时不显示写入动作。
- 详情页区分当前档案与历史 revision，停用状态具有明确提示。
- `409` revision 冲突保留用户输入，显示最新事实并要求用户明确重试。
- `401`／`403` 使页面退出编辑状态并刷新 `allowed_actions`。
- 依赖或数据库不可用返回明确失败，不显示空列表或保存成功。

## 最小组合与独立性

Party Directory 的最小独立组合是 IAM／同形本地 Provider、Party Directory 和其独立 PostgreSQL。它可以不安装 Contract、Money、Project、People、Business Engagement 或 Evidence 而完成登记、查询、更正、状态和历史闭环。

作为其他应用依赖时，Suite Manifest 显式声明应用、版本和健康端点。消费应用不得共享 Party 表、跨库外键、ORM 类型或内部缓存写入；故障只影响需要新鲜 Party 事实的新命令，不损坏各应用既有事实。

## 本地目录迁移边界

- Contract 和 Money 当前本地 Party ID 与历史快照在迁移后保持原值和原义，不被新 Party ID 回写。
- 每个本地 Party 到 Party Directory 的对应关系必须由显式、可审计的迁移命令建立；名称或登记标识相同不能自动证明身份相同。
- 消费应用拥有本地 ID 与 Directory Party ID 的兼容映射，并在切换后为新业务事实保存 Directory Party Ref 与本地业务快照。
- 迁移期间禁止双写两个当前目录。切换前由本地目录负责新引用；切换后由 Party Directory 负责新引用。
- 未能唯一映射的记录保持本地历史可解释，并进入人工处理清单，不能被丢弃或猜测合并。

### 迁移预检证据（2026-09-27）

Contract 与 Money 现已各自提供只读 `party-migration-preflight`。预检从来源应用自己的数据库读取本地 Party，输出来源 ID、revision、规范化事实 SHA-256、各类业务引用计数、重复登记标识复核信号和逐条未决决策；报告固定 `automatic_merge_allowed=false`、`ready_for_cutover=false`。首轮预检两侧各发现 5 条 Party；后续 D1 映射专项创建验证样本后各为 6 条，全部为 0 条业务引用且无 blocker，证明了真实库清单与装配接线，不证明已有引用数据的切换。

### 人工确认映射证据（2026-09-27）

Contract 与 Money 分别拥有不可变映射账本和追加不可变的租户级运行时转换账。审核人只能提交预检中的本地 Party ID／revision 摘要、`create_directory_party` 或 `map_existing_directory_party` 决定、目标 Party ID／精确 revision 和理由。执行器先用工作负载身份读取目标 Party，再在来源事务内锁定本地 Party并重算摘要；stale、inactive、错租户、依赖不可用、Command 冲突或来源漂移都失败关闭。同 Command 同负载返回原结果，不重复写入。`cutover` 必须重新验证批次内全部目标精确 revision，并要求每个已被业务事实引用的本地 Party 都有显式映射；成功后运行时引用使用 Directory ID／revision，本地 Party 写入口永久冻结。`rollback` 只恢复本地兼容读取，不恢复本地写权，也不删除映射或既有转换事实。两个应用的含引用数据 PostgreSQL 用例均通过切换、只读回滚和防篡改；正式 C6 D1 又通过 Contract `cutover → rollback → recutover`、切换前后失败／成功门禁和联合空目标恢复。Money 完整 D1 消费链与完整双 Tenant 继续待补。

## 明确不负责

- 登录账号、密码、会话、Person、Group 或人员任职；
- 客户／供应商分类、合同角色、债权债务角色、项目角色或行业标签；
- 证照真实性核验、工商数据同步、地址簿、联系人和 CRM 活动；
- 主体合并、拆分、批量导入和全球登记标识规则；
- 用 Party 当前资料改写任何消费应用历史；
- 向 Contract、Money 或其他目标应用直接写入业务事实。

## 第一版验收

1. organization 和 individual 能在空库中登记并形成唯一 Party ID 与 revision 1。
2. 字段标准化、边界长度、非法类型和客户端 Actor 伪造均被服务端验证。
3. 登记、更正和状态命令的串行／并发重放保持幂等；负载冲突和 stale revision 无部分写入。
4. 更正、停用和重新启用形成连续不可变 revision；数据库直接修改历史被拒绝。
5. active／inactive 搜索、稳定分页、详情和历史返回确定结果。
6. 相邻角色拒绝、错误 Audience、Tenant、Action、Scope、过期凭证和伪造 Header 全部失败关闭。
7. 精确工作负载凭证只能读取一个 Party 引用；跨 Party 和跨 Tenant 读取被拒绝。
8. 一个消费应用确认业务事实后，即使 Party 更名或停用，原快照仍保持不变。
9. Party 停机时消费应用既有历史可读，新鲜引用命令失败；恢复后使用新命令只写一次。（已通过 D1 Party→Project 故障验收）
10. 独立启动、真库 migration、备份、空目标恢复、release 和非 root 容器验证通过。（已通过）
11. Party-only 最小组合不安装 Contract／Money 仍可完成主要闭环；带 Project 消费者的 D1 组合不共享数据库，无 Party 的 Base Project 命令不被依赖故障阻断。
12. 迁移预检能确定性保留本地 Party ID、revision、来源摘要和引用计数，并对每条记录保持人工决策；来源应用自有映射保存精确目标 revision、审核与幂等事实，对摘要漂移或无效目标失败关闭；显式运行时切换要求覆盖全部被引用主体，回滚只恢复兼容读取，本地写权不恢复。（预检、映射、含引用数据切换／回滚和 Contract-linked C6 D1 已通过；Money 完整 D1 与完整双 Tenant 待验）
13. Workbench 能进入可操作 Party 页面；列表、登记、更正、状态和历史动作按服务端 `allowed_actions` 呈现，409 保留输入并重载最新 revision，401／403 退出编辑态。

## 人类管理与 Project 选择证据（2026-09-27）

`backend/services/party-directory` 根路径现提供查询、类型／状态筛选、登记、详情、更正、停用／恢复和历史工作区；列表响应补充租户凭证下可用的 `allowed_actions`，详情动作仍由目标服务结合 Scope 和状态收敛。D1 Suite Manifest 不再隐藏 Party，统一工作台通过 Gateway／IAM 进入页面。`backend/services/project-workspace` 的新建表单可按需取得 Party 用户凭证并读取 active 候选，但只提交 ID 与 revision；Project 服务继续以自身工作负载身份精确复验并冻结快照。

普通 Go／HTTP 测试、两个浏览器脚本语法检查、D1 schema check、非破坏性完整验收和破坏性故障恢复验收均已通过。自动化证明 Party 页面与脚本经唯一 Nginx 入口可读、active 候选及列表动作可用、Project 页面包含可选主体选择、服务端保存精确快照；Party 停机时 Base Project 仍可创建、带主体的新命令失败关闭、既有快照可读，恢复后重试只写一次。当前 Mac 自动浏览器仍因锁屏无法执行最终肉眼点击，因此不能把本轮记录成像素级视觉回归；这不影响已取得的 HTTP、权限和故障语义证据。

## 第一版之外

主体合并／拆分、批量导入、外部工商核验、多地址／多联系人、证照到期、隐私分级、跨租户共享和异步变更订阅需要各自的真实业务规则与独立验收，不进入第一版通用内核。
