# Project Initiation Info Change v1

问题 A → 可验证结果 B：旧待立项项目缺经营负责人、已存生产分工后在完成抽屉无法继续 → 同一抽屉可以明确补经营分工、保留或更换生产分工，再完成同一个项目，失败后安全重试。

关联 BF-090、BF-091。只修立项阻断，不扩展经营、生产或资质／费率字段。

## 不变量

- 新增 `POST /api/v1/projects/{id}/initiation-info-changes`，只支持 draft／pending_completion；旧 active-only Basic Info Change、PUT、Assignment 和命令 hash 不变。
- 复用显式核心 profile 和最多两项 owner set／clear 结构；初始界面仅缺经营分工的旧记录显示补齐入口，不能自动选择人员；普通派发仍要求经营负责人。
- profile 使用既有 update 能力，每类分工使用对应 manage 或通用 assignment.manage；任一项越权整条命令零写入；Tenant／Project Scope 继续由目标复验，无新能力或 Grant。
- 同一 Project 事务原子保存资料、结束旧分工、追加新人、一次 revision、历史和回执。People 仅提供版本化人员校验，不写其库；缺省分工不变，选择原人不重写快照，未来分工不被隐式结束。
- 新命令使用 `project-initiation-info-change-v1` 独立 hash／历史标记；相同命令重放不依赖 People 在线；stale revision、编号冲突和引用失败零部分保存。
- 原完成抽屉生产选择空表示未选择，不隐式清除既有分工；回填当前负责人，即使目录暂不可用也保留只读快照。刷新不自动提交；结果未知时沿用同一命令，重新读取已成功事实后不重复添加分工。
- 资料和分工保存后仍显式调用现有 complete-initiation，保留 business-owner-v1 门槛与 pending 历史，不把保存资料当成已立项。部分成功明确显示并可重新读取同一项目。
- 不改 schema／既有记录、IAM、入口链、Evidence、Suite、Deployment Profiles 或其他业务所有权。

## 验收

复现旧项目缺经营负责人＋已存生产负责人；补经营、保留原人、更换生产、生产可空、新派发正常、多类型、包；保存失败零部分写、成功后完成失败、丢响应原命令重试、刷新后继续、no-op、旧历史、错租户／范围、分权、撤权、People 不可用、并发、active／cancelled 拒绝。单元、真 PostgreSQL、HTTP、独立启动和浏览器分别记证；公网只用非真实夹具，用户原项目只读。
