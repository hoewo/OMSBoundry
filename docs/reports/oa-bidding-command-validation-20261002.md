# OA 招投标方式和适用性验证

日期：2026-10-02。关联 BF-093／BF-094。当前状态：`.11`／k 已部署；同一 G、同一承接来源的三种方式 → 不适用 → 恢复适用 → 刷新，经真实公网 Chrome 点击及原生复读通过。最终 Project revision 4／单一 Link、BE revision 7，刷新零新增 revision／历史。文件、多附件、原站现场等效及完整 OA 仍未验收。下方失败是不可覆盖的历史证据。

问题 A → 可验证结果 B：当前项目的招投标栏目无法操作 → 用户在同一项目选择原 OA 三种承接方式、设置不适用或恢复，Business Engagement 确认事实，Project 只保存引用，刷新复读同一来源。

## 边界和实际改动

本轮小系统代码改动为 Business Engagement 自有列表的兼容 `project_id` 过滤，并在限制数量前过滤 Tenant／Project。原无过滤查询、独立数据库、schema、三种方式、材料规则、revision、历史和原生能力不变。Workbench 新增原 OA 表单接线、分段继续及准确附件状态；`.11` 另补旧部署缺失的 Project→BE 配置端点，Project 镜像、其他 env、IAM 和 schema 不变。不是共享数据库、统一业务事务或把业务事实搬进工作台。

原 OA 的 `BusinessInfoTab`／`BiddingMethodControl` 源码作为当前操作依据：选择即保存，比选／竞争性谈判／招投标，不要求填写其他小系统 ID。原站现场等效仍待取得。招投标文件上传、查看、下载和移除不在本切片通过范围，页面明确待接通；没有伪造材料或冒充 `ready`／中标／签约。

## 已运行检查

- 107 项 Node 检查通过、零 skip：30 项本切片加 77 项既有检查。覆盖读零写、三种方式、来源复用、不适用／恢复、四段响应丢失原命令重放、关联失败不重复创建、409 显式重读、缺权、错项目／租户、切项目清空旧内容及保存后回读失败不再写入。
- Business Engagement 和 Workbench 全量 Go 单元检查、vet、JavaScript／Python 语法检查通过。HTTP 测试覆盖原查询兼容、新精确查询、当前／相邻 Project／原生 Engagement scope、缺能力与错误参数拒绝。
- Auth58 私有 PG18 的经营承接测试组实际通过 10 项、零 skip，包含新精确 Project 在 limit 前过滤／Tenant 隔离／重复读取不追加历史以及既有真实生命周期、幂等和并发测试；Main 快照不变。临时测试库由原测试 helper 创建并清理，runner 和 PG 已停止。
- 独立新 BE 二进制、空自有数据库、正式 IAM 配置实际启动及重启通过；5 个匿名原生读写入口均 401、事实不变，Main 快照不变。新增资源上限共 0.4 CPU／352 MiB，未启动本地 Docker。

第一轮独立启动驱动错误地只接受 bind mount，而现有公开钥／工作负载钥由只读 named volume 挂载；门禁在创建服务容器前拒绝。失败证据保留，随后只把这两份既有配置以只读方式注入独立服务，未挂主业务数据或备份目录；第二轮通过。该修正属于验收驱动，不是产品鉴权变化。

## 交付和剩余工作

候选版本 `.8` 的兼容闭包为 Business Engagement 读 API＋Workbench，同一 API／migration／数据所有权；Project、IAM、Gateway、Suite 及其余应用不在替换范围。`.7` 已完成 ACR 字节核对但未部署：部署前发现已有 undetermined 草稿需先按原生适用性动作推进，随后才取得原生 method.select；修正并补两项检查后用新不可变版本 `.8`，没有覆盖 `.7` 标签或更改能力目录。制品为显式测试发布，`official_release=false`，不声称 dirty workspace 是 clean-HEAD 正式版本。

`.8` 的两镜像 build／push／pull、镜像内程序字节和 prepare 通过。SSH 中断时只能确认六库输出；此历史观察保留在[连接中断观察](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/connection-interruption-observation.json)。用户登录阿里云后经既有 Workbench Shell 只读核对原回执，确认[13 库空目标恢复终态](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/restore.json)、文件／IAM key 字节、schema 10 和旧事实保持均通过，私有 PG 已停，没有重跑或覆盖恢复库。随后 apply [17/17 healthy、其他 15 容器不变](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/rollout.json)，[离机归档 SHA 核对](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/offhost-backup.json)通过。没有重启 ECS、修改安全组或新增访问权限。

公网 native Chrome 实际完成 G 的派发、编号／类型、留空可后补生产负责人及同 OA 详情；G 为 active revision 3、零承接来源／引用，既有 Project、用户101 和 Grant 保持。经营→招投标控件仍禁用，不能记为方式切片通过。诊断发现 Project 原生返回 `project-workspace.resource-link.manage`，Workbench 却检查不存在的 `resource.manage`；旧测试夹具复制了相同错误。将夹具先对齐真实 Capability Manifest／Go 常量后，32 项中 16 项失败，证明此前单元检查不足。失败及根因保留在[能力接线反例](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/capability-mismatch-observation.json)。

`.9` 只修 Workbench 两处能力检查，不改变 IAM、Grant、Project、BE 二进制或 schema；32 项 B03＋77 项既有 Node 共 109 项通过、零 skip，Workbench Go test／vet 和 Python／JS 语法通过。新增五项纯续验门禁检查确保不能换项目掩盖失败、改变用户／Grant／其他事实或顺手改其他 runtime。按独立 `OA_PUBLIC_RUN=i` 联合备份／恢复门禁发布新不可变标签；续验必须保持同一 G，逐阶段真实点击三种方式→不适用→恢复→刷新，并以原生 revision／历史确认，不通过 API 准备业务事实。

`.9` 首次本地制品准备与异步编译重叠，清单未包含尚未生成的 server；此目录未上传或发布。制品准备现先验证必需 server／Dockerfile，缺少即拒绝生成清单；新增两项纯检查通过。编译完成后另建新目录，清单及服务器上传包摘要相符，ACR 仅发布一个 Workbench 镜像并复核镜像内程序字节；不覆盖首轮目录或旧标签。

## 附件下一切片的已发现差距

`.9` 公网比选点击真实失败：Project 仍 active revision 3，BE 原生创建已成功，来源 `231f2977-79d4-4b36-a178-f19a0d8d3119` 为 undetermined revision 1。原生命令回执只有 engagement／materials／requirements／missing_requirements，`allowed_actions` 来自独立 authenticated GET；测试夹具曾错误地在 POST 中返回权限。对齐真实 POST 后三项方式检查均失败。`.10` 不改 BE：验证回执身份后 GET 同一原生详情取得下一步动作；回读失败保留原 command ID／载荷，并发 revision 变化停止，要求显式重读。37 项 B03＋77 项既有 Node 共 114 项通过、零 skip，Workbench Go／vet、8 项纯续验门禁、2 项制品完整性和语法检查通过。首次纯门禁命令误在应用目录运行，未发现测试；切回仓库根目录后实际通过。新批 `OA_PUBLIC_RUN=j` 只替换 Workbench，不覆盖 i 的失败证据、不换 G 或另建来源。

`.10` 已完成 ACR 单镜像字节核对、13 库／两文件树空目标恢复、apply、17/17 healthy、其他 16 容器 ID 不变；[离机备份](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/j/offhost-backup.json)摘要相等。首次 j 验收脚本误抄来源 ID 为 `1231f297-…`，baseline 失败；保留原结果，以独立 checker v2 目录修正为 i 不可变回执中的真实 ID，新增纯门禁防止再次抄错。v2 baseline／completed 通过，不覆盖旧失败。

`.10` 原生 Chrome 刷新后选择比选，实际报“外部引用服务暂不可用”，未完成方式保存；[点击与诊断观察](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/j/browser-observation.json)保留。只读检查确认 Project 容器没有 `BUSINESS_ENGAGEMENT_BASE_URL`，既有 IAM workload policy 已允许 `business-engagement.material-reference.read`；这是旧 D1 装配缺失端点，不是缺授权或业务所有权冲突。`.11`／`OA_PUBLIC_RUN=k` 仅补当前标准 D1 中已声明的内部端点，复用原 Project 镜像／schema／密钥，Project 因配置变化重建；另外发布 Workbench 附件状态文字修正。新增两项 Node 反例先失败，再修正：可选 requirement 的 satisfied 不能当成文件已登记。39＋77 共 116 项 Node、13 项纯续验／装配门禁通过；公网、部署及最终三方式链须另验。

## `.11` 实际终态与验收驱动反例

本批 ACR 仅发布 Workbench，不可变 digest `sha256:c6b133063cd9800dd0270423fc3d133c56081208fe7de26a9cdd21ed2cd13645`；源码包、镜像程序和公网三份 runtime 字节匹配。13 库／两文件树冷备份与空目标恢复通过；Project schema 10，12 业务库和 IAM 静态事实保持。私有 PG 和 restore20 均停止。离机私有备份为 222258 bytes、SHA `513e6ecb6b34a3b97c5b9ca1a2705f9053125750a9d933d34a6aa0fa5d1570ff`，已核对且不进入仓库。

apply 已重建 Project／Workbench 后，驱动因 Docker inspect 的 Mounts 数组排列变化报错；没有重新 apply。8 次同容器只读观察显示顺序变化、完整挂载规范排序后摘要相同。独立 postflight 按旧声明配置核对原镜像、仅一项 env 增量、显式挂载的类型／源／名称／RW、既有镜像匿名卷的创建时间，以及其他 15 容器 ID 和旧事实；随后当前 wrapper check 通过、17/17 healthy。部署前完整原生 Mounts 快照没有保存，因此不声称直接前后快照比较通过；[原失败](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/k/backup/rollout-failure.json)、[顺序观察](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/k/mount-order-observation.json)及[独立 postflight 的范围／限制](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/k/postflight.json)均保留。后续驱动已保存私有前态、按完整挂载内容排序比较，17 项纯门禁通过；新驱动不冒称已在本次重新部署验证。

公网[逐次点击观察](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/k/browser-observation.json)和[最终原生复读](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/k/bidding-browser/final.json)确认：复用 `231f2977-79d4-4b36-a178-f19a0d8d3119`，BE revision 3／4／5／6／7 分别对应比选／谈判／招投标／不适用／恢复；Project 只在首次关联增至 revision 4，后续保持。刷新历史相等；所有旧 Project、用户101、Grant 和其他十业务库保持，没有假材料或 ready。116 项 Node、17 项纯门禁和两项制品完整性检查通过；BE／Workbench Go、真实 PG 和独立启动的先前证据按上节保留，不混为本次重新运行。19 份显式选择的安全 JSON 已离机核对 SHA 后保存；不包含 runtime 环境、密钥或数据库备份。

当前运维必须使用 `/opt/omsbizfoundry/acceptance/oa-public-release-20261002k/d1-active`，旧 a wrapper 缺本次端点，不能再作为当前 up 入口。用户调整 SSH 安全组后现有密钥连接成功；本批没有代改安全组、扩权、重启 ECS 或新增凭证。制品仍是 dirty workspace 的显式测试发布，不是 clean-HEAD 正式交付。

## 多附件仍需能力演进

最终复读后另做架构透镜只读点击：[观察](../../acceptance/oa-initiation-repair/evidence/bidding-20261002/k/architecture-lens-gap.json)显示招投标页仍展示固定立项步骤，没有 BE 调用链。它不影响已经确认的方式事实，但不符合“解释当前操作背后的架构”的目标，必须在本页接线时纠正；不能把现有透镜称为承接架构学习验收通过。

原 OA `BiddingFileList.tsx` 对每种类型保留文件 ID 数组，逐份显示、查看、下载及删除；当前已部署 BE `business_engagement_materials_active_key` 则约束每种 requirement 仅一条有效引用，`put` 会取代旧引用。所以只把现有 put/remove 接到按钮不能实现原 OA 多附件。该能力需先确定兼容契约和迁移验收，保持材料集合由 BE 拥有、字节与不可变 File／Version 由 Evidence 拥有；连接器不能另存附件真相，也不能物理删除仍被其他业务引用的文件。此处为源码差距记录，不是已部署或已验收的多附件方案。

下一项是 B03 Evidence 附件全链；B02 经营参与人、完整立项字段、合同／支付／开票／成本／奖金、分类型生产、资料统计、角色反向矩阵和完整恢复交付继续逐项关闭。方法／适用性小切片不能代表 B03 或整个 OA 完成，整体结论仍为 `insufficient-evidence`。
