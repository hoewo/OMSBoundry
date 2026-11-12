# Feedback V3 与项目创建解耦发布验证（BF-325／BF-326，2026-11-11）

> 2026-11-11 纠错：BF-328 证明 SDK 3.0.2 存在配置事件早于 React 监听时回落 V2 产品面的竞态，因此本报告中“V3 产品面已完成”的结论不充分。正式 `sdk-v3` 已由 3.0.4 修复并完成可见产品面验收；Project／Contract 解耦结论不受影响。[纠错报告](./oms-feedback-v3-product-surface-race-repair-20261111.md)

## 问题与结果

问题 A：OMS 仍使用 Feedback V2 产品表面，无法消费 V3 的智能客服与反馈契约；新组织的 Contract Ledger Party runtime 或本方经营主体尚未配置时，“新建项目”把合同就绪错误当成 Project 创建前提，导致截图中的“合同功能尚未准备完成，请联系管理员”和禁用保存。

结果 B：Feedback SDK V3.0.2 已发布并提升到 'v3-stable'／'sdk-v3'，OMS 入口改为“帮助与反馈”，只接受声明支持 V3 的 ready 事件。项目创建现在只受 Project Workspace 创建权限和外部甲方约束；合同未就绪时，表单不再采集应收金额，而是先保存项目并说明可在项目“合同”页补录。合同补录继续使用 Contract Ledger 与 Project resource-link 的版本化命令和全部就绪门禁。

## 产品与数据边界

- Feedback V3 是 SDK 产品契约，不是新的 HTTP API；Gateway 和 Feedback 后端继续使用既有 V2 端口。
- Workbench 仍须验证固定 SDK origin、iframe window、V3 supported contract 和匹配的 configured request ID；iframe 加载本身不等于配置成功。
- Feedback API Key 的浏览器可见风险没有因 V3 消失，BF-235 的服务端短期 Session 仍是下一安全收敛目标。
- Project Workspace 独占项目事实。Contract Ledger 不可用时，Workbench 不把应收金额暂存在 Project、浏览器或其他 Owner。
- 后续“补录甲方合同”重新校验 Contract 可用性、合同创建权限、Party Directory authority、本方经营主体、外部甲方状态和项目合同关联权限；任一不满足继续失败关闭。
- 当前活跃新组织没有 Contract／Money Party runtime cutover，也没有 Tenant→Operating Party 绑定。本次没有自动推断或写入这些配置，只修复它们不应阻断 Project 创建的问题。

## 实现

- 新建项目将 'canCreateProject' 与 'contractAssemblyReady' 分离。Contract 未就绪时只提交 Project 命令，成功后停止后续 Contract 调用。
- 应收金额字段只在 Contract 能够拥有该事实时显示；否则显示延迟补录说明。
- 项目合同页在没有 'primary-contract' 时提供“补录甲方合同”，使用稳定 command ID 创建合同、连接甲方与本方主体，再建立 Project resource link。
- Feedback 宿主切换到 SDK V3 stable URL，ready 载荷必须包含 'supported_contracts: ["v3"]'；界面文案收敛为“帮助与反馈／智能客服”。

## 本地与跨域验证

- Unified Workbench：主／Feedback JavaScript 语法检查通过；Feedback 专项 31 项、Project 创建专项 11 项、全量 Node 303/303 通过；全部 Go test 与 go vet 通过。
- P8 真实跨域 Playwright 三个场景分别通过 24、5、5 项断言，覆盖宿主配置、失败关闭和重开行为。
- Feedback SDK：23 个测试文件 141/141，发布脚本 8/8，V3 production build 通过。
- SDK 3.0.2 artifact SHA-256：'bc65d29aeda16cc7a701aff93292c3ced5d773dbf443b3468b82137710f94e44'；JavaScript：'4b7396c6effd917ec4256b545587d787a90e724fdfc4622b62dd08cd7dc8912c'；CSS：'efd052452f484e6190479cd7d3b3d30c74cd03fe5bad176d29b53ac025c068a3'。
- 'v3-stable' 的 minimum／recommended version 均为 3.0.2，'sdk-v3' 指向 '/sdk-releases/3.0.2'。

## 公网发布

- Suite：'v0.1.0-test.oms-v1.97'。
- Unified Workbench：'v0.1.0-test.oms-v1.97@sha256:baf47772830c8d825c258fabb1544d21541e6d09825402049d330b92fe67f8df'；Linux AMD64 manifest 为 'sha256:318128ed97d26424826fa7790847dcd3214964ee5357336e626011cdbbf0ea6a'。
- D1 发布包 SHA-256：'c6acc8ad50f66dd1cf830f8a7df6c27b1ce8cc54ddad44903e3dd560fb6ba3f9'。
- 切换前完整备份：'20261111T133415Z'，'checksums.sha256'、数据库指纹与一致性文件存在并通过。
- 候选 IAM 契约预检、发布后独立 D1 check 和 21/21 长运行容器 healthy 通过。
- 运行容器的 'FEEDBACK_SDK_URL' 为 'https://feedback.feitianchengzi.com/sdk-v3/index.html?embed=web'。
- 公网主 JS SHA-256 '9eb904ef5b85bc2df2cef9a9ccf5a7ceab5665334366367164e13f40cb68028c'、Feedback JS SHA-256 '6333285eb13ea5cf7c79a28bc08489c381d1607188be9d0ba3673323e6421af6'，均与本地逐字节一致。
- 发布前后 Project Workspace 数据均为租户 '077886c1-f083-4afd-b22a-53de299aeae1' 1 行、'6539cd13-7190-40f1-8bd5-57f9e7c9b968' 1 行；没有新增项目、合同或反馈。

共享浏览器控制在公网只读复验时连续连接超时，因此本报告不把真人登录页面检查描述为通过。已通过的证据是本地全量回归、真实跨域 Feedback 浏览器测试、D1 独立检查、公网资源逐字节摘要和业务项目行数不变。当前组织可在页面自行创建项目；若要立即补录合同，仍需管理员显式完成该 Tenant 的 Contract Party runtime 与 Operating Party 装配。
