# ADR-0032：正式 Release 溯源必须绑定干净 Git HEAD

日期：2026-09-20  
状态：accepted

## 背景

Contract Ledger 与 Money Flow Ledger 的 release bundle 已经嵌入 version、commit 和 build time，但原构建脚本接受任意调用方提供的 `RELEASE_COMMIT`，也允许仓库没有 `HEAD` 或存在未提交内容。实际复核进一步发现 BizFoundry 当前 `main` 尚无任何提交，全部工程文件均未跟踪。

外部摘要只能证明归档收到后没有变化，不能证明它对应哪一份源代码。如果正式构建可以把任意标签写成 commit，运维记录、日志、备份演练与源码之间仍无法建立可信追溯。

## 决策

两个产品共用根级 release provenance resolver，并明确区分两种模式：

- `development` 是默认模式，保留本地开发和 clean-room 测试能力；调用方可以注入测试标识，也可以得到 `unknown`，但该产物不得冒充正式发布；
- `official` 只接受仓库根目录，必须存在可解析的 `HEAD`，且 tracked、staged 和 untracked 状态全部为空；
- 正式模式从 Git 读取完整 `HEAD`，调用方若同时提供 `RELEASE_COMMIT`，必须与完整 `HEAD` 完全一致；
- 正式模式必须显式提供无小数秒的 UTC `RELEASE_BUILT_AT`，不从开发机本地时间静默推断；
- 两个 build-release 脚本必须在创建输出目录和编译前通过同一门禁；
- 根级 `make check` 必须覆盖门禁的成功和失败路径。

## 结果

干净、有提交的临时 Git 仓库能够解析出真实完整 HEAD；无 HEAD、脏 tracked、staged／untracked 内容、错误 commit、缺构建时间和非仓库根路径都会失败。开发模式仍可生成带显式测试 identity 的实际 bundle。

门禁实现时，BizFoundry 仓库因为没有首个提交而被正式模式正确拒绝，没有通过伪造 commit 或构建脚本自动提交规避。随后以显式版本控制动作建立首个 Git 基线 `cdc7bc8349c8af4efda1b33d2d21f0498bffbffd`；同一干净 HEAD 构建的两个 official-mode 候选包均嵌入该完整 commit，并通过实际归档验收。正式发布、远端推送和制品发布仍是独立动作。

## 放弃的方案

- 信任流水线任意传入 `RELEASE_COMMIT`：无法证明标签对应实际源码；
- 所有本地构建都强制依赖 Git：会不必要地阻断源码归档、测试夹具和开发构建；
- 构建脚本自动提交工作区：越过版本控制责任并可能收录不应提交的文件；
- 只检查 tracked diff：会漏掉实际参与构建的未跟踪源码；
- 用 bundle SHA-256 替代源码 commit：只能验证产物完整性，不能建立源码来源。
