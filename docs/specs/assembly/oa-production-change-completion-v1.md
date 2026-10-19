# 原 OA 变更／竣工原生切片 v1

状态：私有原生候选已通过；Main／公网／正式 D1 与完整 OA 未通过。关联[原生产阶段逐动作对照](./oa-production-stage-original-operation-map.md)、[阶段资料集合](./oa-production-stage-materials-v1.md)、[小系统边界演变账](./small-system-boundary-evolution-ledger.md)与[实际验证报告](../../reports/oa-change-completion-native-validation-20261006.md)。

问题 A → 可验证结果 B：原 OA 的“变更竣工”栏目目前只有占位界面，无法维护真实资料 → 在同一 Production Work Item 内复用原 `ChangeFileList`、`CompletionFileList`、`AuxiliaryFilePanel` 和两套 620px 上传抽屉，完成变更／竣工多文件保存、复读、预览下载、替换、移出留史和失败续办；Evidence 仍独占字节，Project／People 零写入。

## 原 OA 事实

- 一个栏目内按顺序显示“变更洽商”和“竣工验收”两套文件列表，不跳转另一个系统。
- 两套列表都支持多文件上传、预览、下载、单文件替换和确认删除；没有校审单和评分。
- 变更允许 PDF、Word、Excel、图片和 CAD；竣工在此基础上还允许 ZIP／RAR。
- 变更文件类型为 `variation_order`，阶段键为 `change`；竣工文件类型为 `completion_report`，阶段键为 `completion`。
- 设计项目生产进度中的 `construction_support` 只以有效竣工主文件为完成依据；变更资料可选，不阻塞竣工和项目完成。
- 原实现的替换会原位更新文件引用，删除会软删业务记录；目标必须保留用户可见效果，但不能覆盖已确认历史或删除 Evidence 字节。

## 目标边界

- Production 继续独占阶段类别、资料业务引用、替换／移出历史和完成投影。
- Evidence 独占 File／Version、字节、上传会话和短期访问地址；Production 只保存经验证的稳定引用和显示名快照。
- `reference-oa-stage-set-v1@5` 在不可变 v4 之后同时增加：
  - `change`，sequence 40，可选，不构成完成前沿；
  - `completion`，sequence 50，必需，其有效主文件投影为原 OA `construction_support` 完成。
- 旧 v2／v3／v4 含义不变。已有 v4 第一次真实变更或竣工写入才显式 4→5；旧 v2／v3 只能顺序升级，禁止跳级和批量改写历史行。
- v5 不增加 Audience、Capability、Workload Policy 或跨库事务，复用现有四项 stage-material 能力与 Production → Evidence 精确校验。
- change／completion 只允许主文件；服务端必须拒绝校审单和评分。扩展名规则属于 Production 业务规则，由服务端基于可信 Evidence 显示名再次校验，不能只依赖浏览器 `accept`。

## 替换与失败恢复

“替换”不是 UPDATE：先用原命令上传并登记新 File／Version，再以独立、持久的结束命令移出旧引用。中途失败时：

1. Evidence 已完成但 Production 未登记：保留 File，继续同一登记命令；
2. 新引用已登记但旧引用未结束：两者暂时都可见，继续同一结束命令，不删除新引用；
3. 任一成功回复丢失：重放同一 command／idempotency key 并最终回读；
4. 页面重载：只保存有界身份、摘要和回执，不保存凭证或文件字节；尚未上传的文件要求用户重新选择并核对摘要；
5. 移出只结束 Production 引用，Evidence 字节和历史保持。

## 验收门槛

1. 原双列表、原标题／说明／按钮、620px 抽屉、小视口和预览展开保持；UI 组件直接复用，不重新设计同名表单。
2. v4 读取 change／completion 为“已知但未纳入”且零写；第一次真实写恰好一次 4→5，同一 Work Item 和前三阶段事实保持。
3. 变更为空时，只有竣工主文件仍可使 v5 Work Item 完成；增加或移出变更不改变竣工完成，移出最后竣工文件会重新打开 completion 前沿。
4. change／completion 的 review sheet 或 score 请求、错误扩展名、错 Tenant／Project／Stage、撤权和并发 revision 全部失败关闭且零部分业务写入。
5. 上传、替换、移出、回复丢失和重载均按原命令续办；预览下载读取真实 Evidence 字节。
6. PostgreSQL 18 兼容升级、Nginx→Gateway→正式 IAM→Production／Evidence、五所有者逻辑空目标恢复后同 ID／revision／历史继续。

未关闭：Main／公网匹配发布、正式 D1 archive／keys／旧会话、全角色／双 Tenant、Project 进度消费者、试验／报告及完整 OA。

## 2026-10-06 实际结果

最终全新夹具 11／11 场景通过：原双列表／620px 抽屉、取消零写、一次 v4→v5、变更多文件、竣工独立完成、真实字节预览下载、不可变替换、移出／重加、正式 IAM 撤权、五所有者逻辑空目标恢复与零页面错误均实际运行。最终为 1 Work Item、5 Stage、5 资料引用、2 ending、8 command receipt、1 upgrade、0 score；Project／People 摘要保持，Contract 未加入依赖图。

验收先后抓到两项真实实现缺陷：migration 未同步扩大数据库 stage key／sequence 约束，以及代码阶段注册表未识别 v5 动态定义。两项修复并增加回归后，才从全新夹具重新计通过。由此只说明 Production 阶段资料子边界承接了首个异类场景；Production 整体仍为候选，跨客户稳定没有证明。
