# 原 OA 变更／竣工原生切片验证报告（2026-10-06）

结论：`private-native-passed / public-main-and-full-oa-not-accepted`。

问题 A → 可验证结果 B：原工作台的“变更竣工”只有可见壳，且前三个阶段的同形表单不能证明异类阶段也适用 → 直接复用原 OA 双列表和上传抽屉，在真实私有 Nginx→Gateway→正式 IAM→Production／Evidence→PostgreSQL 18 链完成变更与竣工资料闭环，并用五所有者空目标恢复证明 ID、revision、历史和字节保持。

## 实际运行范围

- 驱动：`acceptance/oa-initiation-repair/verify-change-completion-native-browser.mjs`；
- 最终命令：`OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-change-completion-native-browser.mjs`；
- 最终回执：`/tmp/oa-change-completion-suite.1e2zf2/change-completion-browser.json`；
- 时间：2026-10-06 12:02:12～12:06:40（Asia/Shanghai）；
- 运行位置：私有临时远端，临时 Nginx 与 PostgreSQL 18；本地 Docker 未启动；
- Main／公网未改变，夹具结束后已停止。

## 11 个实际通过场景

1. 既有 v4 的方案、初设、施工图定义在辅助阶段使用前保持；
2. 直接复用的变更／竣工面板读取 v4 时零业务写；
3. 变更上传取消零写；
4. 同一 Work Item 恰好一次 v4→v5，竣工可完成而可选变更不阻塞；
5. 变更多文件仅登记主文件，不产生校审或评分，并保持竣工资料；
6. 预览／下载返回真实 Evidence 字节且不写 Production；
7. 替换先追加新引用、再结束旧引用，旧 Evidence 字节仍在；
8. 移出最后一份竣工资料会重开 completion，重新登记后恢复完成，但不会把空的方案／初设／施工图伪造为完成；
9. Identity、People、Project、Evidence、Production 五库逻辑空目标恢复后 v5、两阶段、历史和原 UI 继续可用；
10. 正式 IAM 撤销辅助资料写权后仍可读，页面隐藏写动作，服务端返回 403 且零写；
11. Project／People 所有权与摘要不变，仅有一次显式升级，页面错误为零。

最终 Production 计数为：1 个 Work Item、5 个 Stage、5 个不可变资料引用、2 个 ending、8 个 command receipt、1 个 upgrade／upgrade event、0 个 score。Work Item 保持 `in-progress`：completion 已完成并不伪造前三个仍为空的必需阶段。

恢复校验覆盖五个独立数据库和 5 份本地 Evidence 字节；恢复前后每个数据库 canonical digest、每份字节 SHA-256 和签名密钥身份一致。

## 实现中真实发现的缺陷

| 轮次 | 现象 | 性质与修复 |
| --- | --- | --- |
| 1 | 取消按钮选择器不精确 | 验收驱动缺陷，尚未写业务数据；收窄到原抽屉按钮 |
| 2 | v4→v5 返回 500 | **实现缺陷**：migration 加了定义却未同步 Work Item／Stage 的 stage key 和 sequence 约束；将数据库约束作为定义版本同一原子 migration 修复 |
| 3 | completion 登记返回 400 | **实现缺陷**：`IsKnownStageKey` 仍是旧静态集合；改为同时咨询 code-owned stage-material 定义并加单元回归 |
| 4 | completion 完成后错误期待整个 Work Item completed | 验收语义错误；按原规则修正为仅 completion completed，空的前序必需阶段仍使 Work Item in-progress |
| 5 | 撤权页宽泛选择器误计一个写控件 | 验收驱动缺陷；改为业务写控件精确集合并等待稳定态 |
| 最终 | 全新夹具完整重跑 | 11／11 通过，不能用前轮局部结果拼接 |

这两项实现缺陷表明：增加 code-owned Stage 不是只改页面或插入定义行；应用注册表、数据库约束、迁移、投影和恢复必须作为同一兼容闭包。

## 三个重点小系统的边界结论

### Production Operations

- 新增不可变 `reference-oa-stage-set-v1@5`：`change` 为 sequence 40、可选；`completion` 为 sequence 50、必需；
- 两者只允许主文件，无校审、无评分，服务端依据可信 Evidence 显示名复验扩展名；
- 替换不是原位 UPDATE，而是“新引用登记＋旧引用结束”两个可重放子命令；
- 继续拥有 Stage、资料业务引用、ending 和完成投影，不保存文件字节；
- 没有新增 Audience、Capability、Workload Policy、事实所有者、共享表或跨库事务。

这证明同一 Production 边界能够承接一个**结构不同**的双列表场景，而不仅是复制方案表单；但只提升阶段资料子边界的异类复用证据，Production 整体仍是候选边界，未获得跨客户稳定结论。

### Project Workspace

Project 本轮零 schema、零 Capability、零 Link、零业务写，仍只提供稳定 `project_id`／revision 上下文。completion 完成不会反向写 Project 进度或完成状态；未来进度只能通过版本化只读投影或显式 Project 自有检查点消费，不能让 Production 直接更新 Project 表。

### Contract Ledger

Contract 未加入运行组合、未新增依赖边、未改 schema／Capability／事实。变更洽商是 Production 阶段资料，不冒充合同 Change；若以后某份变更资料形成法律合同变更，必须由显式连接器提交 Contract 原生命令并保留各自 ID／revision，不能共享“change”表或由名称相同推断同步。

## 跨系统依赖结论

- IAM：沿用四项 Production stage-material 原生能力；目标应用继续最终授权；
- Evidence：只通过 File／Version 精确引用和真实字节访问；移出 Production 引用不删除字节；
- Project／People：只提供已有上下文，本轮摘要保持；
- Workbench：只装配原组件和可恢复子命令，不持久化业务事实；
- Contract：零依赖；“变更”同名不构成业务耦合。

## 未关闭范围

Main／公网匹配发布、正式 D1 archive／keys／旧会话、双 Tenant、全角色、Project 进度消费者、勘察试验、咨询／勘察／测绘报告、批复审计、完整 Production 类型进度和完整 OA 均未通过。当前阶段总判断继续为 `insufficient-evidence`。

下一切片按原 OA 实际组件进入“试验／报告”只读梳理与纵向闭环；不得因为 v5 通过而把勘察专有试验或多类型报告硬塞进设计阶段集合。
