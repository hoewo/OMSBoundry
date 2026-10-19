# 原 OA 专业人员到奖金的连续工作面验证

2026-10-04 续轮。有限私有候选通过：正常 Workbench 原人员配置与奖金表单，经真实 Nginx→Gateway→正式 IAM→独立 Project／People／Bonus 完成连续点击、保存、重载及留史撤回。20 个场景、零页面脚本错误。随后另通过[所选 D1 联合冷恢复 14 场景](./oa-professional-selected-d1-restore-20261004.md)，不是全 Suite／Main／公网恢复。未发布 Main、未新增 Main Grant，完整 D01／生产／OA 与全 Suite 恢复仍未验收。

## 可见结果与范围

问题 A→结果 B：真实专业分工不能被旧 Assignment 引用发现，原奖金表单无法选同人的不同专业 → 在同一项目先配置专业名单，再在原奖金抽屉选到独立分工，服务端整批计算／登记，刷新和撤回保留原专业与金额历史。

原仓库只读。直接复用原 Manager、专业／审定抽屉、BonusAllocation／Form／List／Statistics；人员和奖金 CSS 逐字一致。仅替换类型化数据／权限端口、精确金额、追加历史及故障续办，不重新设计业务布局。60 个已登记原来源文件摘要复核一致。前端 skill 用于检查实际可见层；没有依它重画原 OA。

已实际查看本批[正常人员配置](../../acceptance/oa-initiation-repair/evidence/production-bonus-native-20261004/workbench-personnel.png)、[原奖金登记抽屉](../../acceptance/oa-initiation-repair/evidence/production-bonus-native-20261004/workbench-professional-bonus-form.png)、[保存后名单](../../acceptance/oa-initiation-repair/evidence/production-bonus-native-20261004/workbench-professional-bonus-saved.png)、[两版撤回历史](../../acceptance/oa-initiation-repair/evidence/production-bonus-native-20261004/workbench-professional-bonus-history.png)及[续办存储故障提示](../../acceptance/oa-initiation-repair/evidence/production-bonus-native-20261004/workbench-personnel-storage-unavailable.png)。这些是私有验收项目，不是公网 C／Test101 或原 OA 现场。

## 不变的边界

BF-105：Project 默认关闭的 D01 profile 发布 schema 1、独立 `reference_oa_production_assignment` 来源。单项可读取 ended 历史；活动发现有界，超过上限拒绝，不截断伪装完整。引用要求精确 Project 的原生 personnel.read，不继承旧 read。People 当前 Person 另以原生受保护端口复验；新来源不是旧 Base Assignment。

Bonus 只有显式 `reference-oa-bonus-v3` 接纳 `assignment_source=reference-oa-production-personnel-v1`。逐项检查 Tenant、Project 当前版本／active、来源 ID／lifecycle revision／active、People ID／版本／active 及六分工的专业完整性，冻结最小快照及来源摘要；一次命令整批零部分写入。专业四角色要求专业，审定／概算不要求。同人多分工按来源＋分工 ID 区分，不按 Person 去重。

原奖金表单只检查原成员 active，不因“参与不适用”移除其候选。该标志不结束任何事实；ended 分工不可新登记，已有奖金快照不随源改变。旧 v2 的无来源 JSON、命令 hash、两侧旧角色、Pool 和 Money 建议约束保持，Base 角色没有新增客户枚举。Bonus migration 3→4 只改变其自有类型化消费约束；不跨库写 Project／People，不制造实际付款。

正常 Workbench 明确配置两种 profile 后才打开人员入口，服务和逐项目选装／IAM Grant 仍独立。不选装默认关闭；隐藏面板不读数据。目录用 Tenant 能力，人员用 Project 能力，相关授权及请求串行持有 cookie scope，快速切到奖金时先等待人员请求，再复读 Project 当前 revision，不能使用旧版本或混用目录凭证。浏览器不能保存续办意图时显示可重试故障，未启用写入，不显示空名单。

## 实际运行

| 层 | 实际通过 | 限制 |
| --- | --- | --- |
| Project 普通 Go | 120 顶层／369 含子测试 | 不等于真库全部通过 |
| Project 全量 integration＋PG18 | 163 顶层／418 含子测试，零实际 FAIL／SKIP | 不等于多客户全角色矩阵 |
| Bonus 普通 Go | 22 顶层／76 含子测试 | 不等于旧核心全业务公网验收 |
| Bonus 全量 integration＋PG18及 race | 各 32 顶层／93 含子测试，零实际 FAIL／SKIP | 不证明任意压力／HA |
| Workbench／Gateway Go、四应用 vet、TypeScript | 实际通过 | 尚无匹配镜像发布闭包 |
| 前端／装配 Node | 347／347，零 skip | 包含 24 条 D1 profile／备份选择门禁，不是运行 D1 Suite |
| D1 脚本语法／升级选择 mock | 实际通过 | 证明备份取旧运行 profile、错配恢复在删卷前拒绝，不是实际联合恢复 |
| 原组件独立原生＋正常私有入口 | 20／20，零页面错误 | 前 12 条含私有代理，后 8 条使用真实 Nginx／Gateway／正常 Workbench |

[PG18／race／前端回执](../../acceptance/oa-initiation-repair/evidence/production-bonus-native-20261004/)仅整理实际运行结果；Go `[no test files]` 包级 skip 单列，不算测试 skip。正式七个 Go 进程分别独立启动，Nginx 是任务自有 loopback 容器；IAM 使用真实服务签名，原生 API 建立全部来源事实，没有 SQL 预灌人员名单。浏览器使用本批原生登录所得会话，并未测试正常登录表单、HTTPS、公网或完整 D1 Compose。

### 20 条连续场景

[浏览器逐条回执](../../acceptance/oa-initiation-repair/evidence/production-bonus-native-20261004/browser.json)保存真实操作、限定 scope 和命令，不包含 Token／Cookie。

1. 五应用原生 API 建项、选装、名册及精确 Grant；Project active。
2. 原专业／奖金工作面可见，打开零写。
3. 原专业和审定抽屉保存六分工，同一 Person 再加第二专业，共七活动事实。
4. 独立类型化引用发现／单项复验，读取零写。
5. 同人的桥梁／道路两条原候选，基数 1.01 元，各 50%，服务端 half-up 各 0.51 元，来源摘要不变。
6. reload 保留来源标签和金额，零新增命令。
7. 参与不适用仍保留 active 分工及奖金候选。
8. 奖金真实提交后丢回复；Bonus 停机，PG18 custom dump→新空库 restore，快照／命令／审计相同；reload 继续确切原命令，只重放、不重复提交。
9. 结束来源后新登记 422、整批零写，旧奖金标签保持。
10. 恢复后的原确认弹窗撤回，原历史抽屉可见两版。
11. 正式撤回金额能力，重新签发后金额脱敏／登记按钮消失、零写。
12. 原生工作负载精确 action／scope、People digest 和旧 Pool／Money 建议保持。
13. 正常 Nginx→Gateway→Workbench 总览：隐藏 D01／B12 零读、零写。
14. 正常原“人员配置”添加桥梁设计／复核，原子保存一次；Tenant 目录与 Project 人员分权。
15. 同一工作台切奖金，复读新 Project revision；基数 4.04 元、25% 得 1.01 元，typed source 原生登记。
16. 深链接 reload 保留原专业／金额，零新增命令。
17. 原撤回确认／历史抽屉正常点击，两版保存，不跨所有者写。
18. 正式撤回人员管理能力，新凭证只读可见、管理按钮消失，零写。
19. 故意延迟人员读取后立即切奖金：Project 复读等待原 native scope lease，没有凭证覆盖，零写。
20. 浏览器 sessionStorage 故障：正常工作面显示错误和“重新读取”，没有未处理 Promise 或业务写。

最终隔离 Bonus 为 4 条 head、6 revision、5 成功命令、6 审计，Pool／Money 建议均零；Project 专业分工 9 条（6 active／3 ended）、7 Overlay 事件、13 全应用命令回执、2 个目录版本。两条 Base Assignment 为原生创建的负责人，不是专业事实双写。读取、失败、重放和取消不计新增事实；来源写只有明确人员动作。

## 兼容、恢复和失败

真实 schema 3→4 连跑两次，旧手工记录／撤回历史／命令／审计 canonical 保持；v3 服务在上游离线时可重放旧 v2 命令且 hash 不变，再写六种新角色、继续撤回旧对象。新来源快照不可变、同 ID 不同来源 namespace 防重分别验。此次浏览器空目标恢复只恢复 Bonus 自有数据库；此前 Project 单库恢复证据另存。二者不能拼作全 Suite 联合恢复或跨客户长期升级成功。

首两轮失败是原字段名和带 icon 按钮定位错误。随后确有 UI 故障：未知回复期间表单把 pending 当作无权，并丢失 Cancel；已修为明确保存中／未知状态和“关闭表单（保留待续命令）”。后来两次驱动在请求尚未完成时读取了上一笔 command；改为等待真实 pending＋非 working＋错误／续办完成，原请求 body 没有放宽。截图另等待正常控件可操作，避免抽屉进场动画尚未完成就截图。五份失败和早期 12／18 场景回执均保留，不算最终 20 场景。

本批同时修正两个适配器：请求前即持久化 uncertain 意图；只有首次、确切已拒绝的 4xx 可解除待办。已经未知的命令重试遇到后续撤权，或成功提交后最终复读失败，仍保留原 command／body，不丢失恢复资格。

D1 备份选择另加两项公开 profile：从实际运行 Project／Bonus 容器读取，不能把候选 v3 环境误记为旧 v2 检查点。外层校验和包括该文件；恢复前闭合值、唯一声明和版本一致性都在删卷前检查。旧备份缺 profile 只允许两项均关闭的恢复配置，不自动推断已启用版本。24 条纯门禁加真实脚本 Docker mock 已运行；随后所选六库的实际 D1 联合备份／空目标恢复及原页面继续通过，限制另见恢复报告。完整 Suite 和历史主站升级未因此通过。

## 复演与剩余门槛

需要本机 Chrome、真实 PG18 任务容器（精确 ID／label／loopback 校验），不得指向 Main。原生驱动只写私有随机库：

```sh
node acceptance/oa-initiation-repair/verify-production-bonus-native-browser.mjs \
  "$(mktemp -d /tmp/oa-production-personnel-native.XXXXXX)"
```

正常 bundle SHA `8dfcbaf1f98bc3d828557d78715526bf8675a37409cc90f2b694c5eba0ec44ea`，原来源／候选代码和 CSS 摘要见[provenance](../../acceptance/oa-initiation-repair/evidence/production-bonus-native-20261004/candidate-provenance.json)。证据目录不复制备份、密钥、会话或服务二进制。

剩余：精确 Main 新能力授权、Project12／Bonus4／Workbench／Gateway及 workload policy 的匹配镜像发布；完整启用组合与历史主站旧新版本恢复闭包（本批所选六库冷恢复已通过）；同一公网验收项目真实点击；原站现场、多账号／跨租户矩阵；D01 其他清除组合及适用性原工作面；B11 来源奖池；其他生产阶段／批复审计／外委、资料统计及全站连续旅程。Cost 的既有批准不扩展到 Billing／Bonus／D01。完整 OA 和 Architecture Baseline 整体验证继续 `insufficient-evidence`，不能用这两页签关闭全站清单。
