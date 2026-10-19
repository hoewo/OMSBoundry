# 产品功能索引

🟢 已实现 | 🟡 开发中 | ⚪ 计划中 | 🔴 已废弃

<!-- 直接写文件名/目录名（不用链接语法），每级一句总结，文档后标注行数；依赖关系在 _map/RELATIONS.md -->
<!-- 示例:
- user-management/ 用户管理：账号认证、权限控制。🟢
  - authentication.md 用户认证：登录登出、会话管理。🟢 (325行)
-->

- assembly/ 产品装配：用独立小系统形成可验证的业务组合。🟢
  - demo-validation-2.md 小系统群 Demo Validation 2：六个独立 Demo、参考 OA 高保真组合、设计交付／商业台账可裁剪子集、首次实际定制化边界与可移植开发包验收。🟢 (205行)
  - demo-experience-baseline.md 独立 Demo 体验基线：六个小系统的人类可读入口、任务导航、状态反馈、视觉层级与浏览器验收。🟢 (136行)
  - customer-scenario-evidence.md 客户场景边界证据：区分参考、真实客户和假设证据，允许多个不同就绪度的参考产品并固定六层归属与失败关闭门槛。🟢 (134行)
  - reference-oa-demo.md 参考 OA Level B：J1-J4 高保真变更历史、五来源故障隔离与无源码可移植复演。🟢 (167行)
  - full-reference-oa-validation.md OA 完全体验证：十类业务关系、八条全业务旅程、多角色权限治理、C1～C7 当前边界／证据、阶段报告、真实环境门槛及通过条件。⚪ (240行)
  - c3-commercial-summary.md C3 项目经营链路：Contract、Money、Billing 类型化分段、局部降级、来源下钻和本地 D1 证据。🟡 (72行)
  - c8-material-statistics-personal-work.md C8 项目资料、统计与个人工作：Project 范围资料组合、来源自有指标／工作项、文件下载分权和局部故障语义。⚪ (145行)
- business-engagement/ 经营承接：独立拥有合同形成前的适用性、方式、材料业务引用和历史。⚪
  - engagement-lifecycle.md 经营承接生命周期：三种方式、材料规则、最小状态、权限边界、409／503 恢复和第一版验收。⚪ (177行)
- billing-invoice/ 开票登记：独立拥有已开票事实、金额、Party／来源快照、Evidence 引用和纠错历史；工作区与 C3 组合实现已进入 D1 验收。🟡
  - invoice-registration.md 已开票登记：系统登记与现实票据状态分离，人类工作区、C3 组合、细粒度权限、精确来源状态、故障边界和第一版验收。🟡 (231行)
- party-directory/ 主体目录：独立管理主体当前档案、revision、状态和消费应用引用。🟡
  - party-lifecycle.md 主体档案生命周期：登记、查询、更正、停用／恢复、历史、管理页、Project 精确选择，以及 Contract／Money 预检、来源映射、含引用数据切换／只读回滚与恢复边界。🟡 (178行)
- design-operations/ 设计生产与交付：用类型化扩展承接设计阶段、负责人和成果。🟢
  - design-production-delivery.md 设计生产与交付：design-v1 阶段、成果引用、回退、验收及 Production 兼容定位。🟢 (200行)
- production-operations/ 多类型生产交付：以一个严格生产事实所有者和版本化类型定义承接设计、咨询等生产类型，不建立万能流程平台。🟡
  - multi-type-production.md 多类型生产交付：design-v1 兼容基线、consultation-v1 第一增量、权限、失败边界和 C4 验收。🟡 (122行)
- approval-audit-results/ 批复审计结果：独立登记外部结果、精确金额、来源快照、Evidence 引用和纠错历史。🟡
  - result-registration.md 批复审计结果登记：领域核心／真库、受保护 HTTP／IAM、精确引用、人类工作区、Project C5 检查点、正式 D1 装配／恢复与 74 断言真实跨进程业务旅程已验证，完整双 Tenant 和真实浏览器待补。🟡 (152行)
- cost-outsourcing/ 成本与外协：双聚合、正式入口、选装恢复、Contract-linked Party 运行时与 C6→Money 应付建议接受／拒绝／恢复旅程已实现。🟡
  - cost-outsourcing-lifecycle.md 成本与外协生命周期：Cost Entry 与 Outsourcing Engagement 双聚合、精确金额、主体快照、Evidence 引用、追加纠错、正式 D1／恢复、Party 切换及版本化应付建议边界。🟡 (114行)
- bonus-distribution/ 奖金计算与分配：独立拥有经营／生产奖池、版本化策略快照和人员分配历史；正式 D1、未选缺席与联合恢复已验证，向 Money 的建议仍待实现。🟡
  - bonus-lifecycle.md 奖金生命周期：精确来源与人员快照、整数金额、正式 IAM、真实来源、人类工作区、D1 装配／恢复和 C7-A01～A18 验收。🟡 (172行)
- project-workspace/ 项目工作区：拥有最小项目身份、项目分工、外部资源引用和自身历史。🟢
  - project-initiation.md 项目立项：J1 状态、类型中立的立项门槛、两级 Project Package 转换／安全撤销、客户页面、Party D1 精确引用／选择、动态允许动作、不变量和 C1 正式入口证据。🟢 (319行)
