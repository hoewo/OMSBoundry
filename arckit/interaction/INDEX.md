# 交互设计索引

## 使用说明
- ✅ 已完成 | 🟡 设计中 | ⚪ 待设计 | 🔴 已废弃
- 📐 线框图 | 📋 交互文档

## 交互设计地图

- demo-overview/ 独立 Demo 首页：用途说明、演示边界、推荐路径、摘要和帮助。✅
  - default.html 首页线框：已就绪、加载、空状态与错误恢复。✅ (132行)
  - interaction.md 首页交互：六应用投影、导航、反馈与恢复。✅ (111行)
- records-list/ 业务记录列表：领域查询、有限筛选、创建入口与列表恢复。✅
  - default.html 列表线框：内容、加载、空状态、权限变化与错误恢复。✅ (126行)
  - interaction.md 列表交互：领域列、搜索筛选、允许动作、权限刷新、导航和错误处理。✅ (116行)
- record-workspace/ 业务记录工作区：摘要、允许动作、关联事实、历史与安全提交。✅
  - default.html 工作区线框：已就绪、编辑确认、成功、权限变化和冲突恢复。✅ (127行)
  - interaction.md 工作区交互：状态变化、结果反馈、关联事实、权限刷新与并发恢复。✅ (111行)
- engagement-workspace/ 经营承接工作区：阶段适用性、承接方式、Evidence 引用、允许动作与历史。✅
  - default.html 页面线框：已就绪、建立、编辑、成功、权限变化和依赖失败六类状态。✅ (92行)
  - interaction.md 页面交互：Project／Assignment 上下文、服务端动作投影、刷新与错误恢复。✅ (126行)
- party-directory/ 主体目录工作区：主体查询、登记、更正、状态、历史和权限恢复。✅
  - default.html 页面线框：列表、登记、详情编辑、冲突和权限变化状态。✅ (72行)
  - interaction.md 页面交互：动态允许动作、expected revision、历史、冲突重载和失败恢复。✅ (106行)
- project-workspace/ Project 工作空间：建项主体快照、资源装配、项目资料、独立下载授权与故障恢复。✅
  - default.html 页面线框：Base 建项、主体选择、资料可用、来源变化和权限收敛状态。✅ (94行)
  - interaction.md 页面交互：Party 精确复验、Evidence metadata-only 组合、下载分权和局部失败恢复。✅ (136行)
- billing-workspace/ 开票登记工作区：登记、金额收敛、纠正、Evidence、历史和权限恢复。✅
  - default.html 页面线框：已就绪、空列表、编辑、金额收敛、冲突／依赖失败和权限变化状态。✅ (18行)
  - interaction.md 页面交互：动态允许动作、金额收敛、expected revision、原命令重试和正式入口。✅ (110行)
- c3-commercial-summary/ 项目经营链路：Contract、Money、Billing 实时分段、局部失败与来源下钻。✅
  - default.html 页面线框：已读取、revision 变化、部分失败和空状态。✅ (18行)
  - interaction.md 页面交互：按需并行加载、未知金额、重新授权下钻和权限恢复。✅ (37行)
- production-workspace/ 生产交付工作区：多类型工作项、阶段成果、允许动作和权限恢复。✅
  - default.html 页面线框：已就绪、建立、推进、成功、权限变化和冲突／依赖失败六类状态。✅ (9行)
  - interaction.md 页面交互：类型化生产、服务端动作投影、重新授权和错误恢复。✅ (117行)
- approval-audit-results-workspace/ 批复审计结果工作区：Project 范围登记、纠正、撤回、历史和权限收敛。✅
  - default.html 页面线框：已就绪、空结果、登记／纠正、权限变化和冲突／依赖失败。✅ (8行)
  - interaction.md 页面交互：精确来源、allowed_actions、expected revision、脱敏和恢复。✅ (50行)
- project-completion-checkpoint/ Project C5 完成检查点：纯读取 requirement、显式确认、漂移和权限恢复。✅
  - default.html 页面线框：待评估、满足、待办／不可用、已确认、漂移／冲突和权限变化。✅ (9行)
  - interaction.md 页面交互：零写入 GET、精确来源冻结、幂等和非全 OA 完成边界。✅ (38行)
- cost-outsourcing-workspace/ 成本与外协工作区：Project 范围的成本、外协、应付建议、评估、历史和权限收敛。✅
  - default.html 页面线框：成本／外协／应付建议三页签，以及建立、目标不可用、Money 接受／拒绝和重试状态。✅ (11行)
  - interaction.md 页面交互：金额与 Evidence 能力、精确来源、expected revision、目标端独立决定和失败恢复。✅ (98行)
- bonus-distribution-workspace/ 奖金分配工作区：经营／生产草稿、真实来源核对、发布、替代、撤回、历史、脱敏和失败恢复。✅
  - default.html 页面线框：入口、列表、空状态、草稿、发布、替代、依赖／冲突和权限恢复八类状态。✅ (131行)
  - interaction.md 页面交互：精确来源与参与人、动态动作、追加式纠错、零部分写入和正式入口。✅ (138行)
- unified-workbench-personal-work/ 统一工作台个人工作与经营指标：当前 Subject 的工作投影、按来源分区的指标、深链、脱敏、空／无权／局部失败与零代办／零跨来源计算边界。🟡
  - default.html 页面线框：指标精确值、金额脱敏、指标局部失败，以及个人工作加载、可执行、空、无权和局部不可用状态。✅
  - interaction.md 页面交互：Gateway 短期凭证、Subject／Tenant 绑定、Manifest 来源发现、有界并发、来源深链、指标脱敏和失败恢复。🟡（真实浏览器／D1 待验）
