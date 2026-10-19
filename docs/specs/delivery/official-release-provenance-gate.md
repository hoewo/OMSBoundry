# 正式 Release 源代码溯源门禁

里程碑：M6  
功能编号：M6-008  
日期：2026-09-20  
状态：已实现并验证

## 1. 目标

确保 Contract Ledger 和 Money Flow Ledger 的正式 release identity 只能来自当前干净 Git worktree 的真实完整 HEAD，不能由调用方用任意字符串冒充 commit。开发与 clean-room 测试构建继续可用，但必须与正式发布语义分开。

## 2. 模式契约

### development

- 是默认模式；
- 不要求 Git 仓库或 HEAD；
- 可显式提供测试用 `RELEASE_COMMIT` 与 `RELEASE_BUILT_AT`；
- 未提供 commit 时使用 `unknown`；
- 产物可用于本地开发、自动化和交付候选验证，但不能作为正式可追溯发布。

### official

- `RELEASE_PROVENANCE_MODE=official` 显式启用；
- 参数必须是当前 Git worktree 根目录；
- 必须存在 `HEAD`；
- `git status --porcelain --untracked-files=all` 必须为空；
- commit 从 `git rev-parse --verify HEAD` 读取；
- 调用方提供 commit 时必须与完整 HEAD 完全一致；
- `RELEASE_BUILT_AT` 必须显式提供并符合 `YYYY-MM-DDTHH:MM:SSZ`；
- 任一门禁失败都发生在创建输出目录与编译之前。

## 3. 验收标准

1. 两个 build-release 脚本调用同一个根级 provenance resolver。
2. development 模式在无 Git 仓库目录仍可返回 `unknown` 和显式时间。
3. official 模式拒绝无 Git、无 HEAD、非仓库根目录。
4. official 模式拒绝 tracked 修改、staged 修改和任意 untracked 文件。
5. official 模式拒绝与 HEAD 不一致的 commit 和缺失／非法 build time。
6. official 模式在干净临时仓库返回完整 HEAD 和显式 UTC 时间。
7. 两个产品在 development 模式仍能生成摘要正确、identity 一致的实际 bundle。
8. 根级 `make check` 自动执行 provenance 测试。
9. 当前无提交 BizFoundry 仓库的 official 门禁明确失败，且不产生正式 bundle。

## 4. 验证结果

- `ops/test-release-provenance` 使用隔离临时 Git 仓库覆盖所有成功和失败路径，通过；
- 当前仓库以 official 模式执行 resolver，因缺少 `HEAD` 返回明确失败；
- 两个产品以 development 模式构建 `v0.1.0-provenance.1` darwin/arm64 bundle，外部 SHA-256 通过，server 输出相同测试 commit 和 build time；
- 构建测试产物已可恢复地移至废纸篓，不进入仓库；
- `make check` 已包含该门禁测试并通过。
- 建立首个本地 Git 基线 `cdc7bc8349c8af4efda1b33d2d21f0498bffbffd` 后，official resolver 从干净 worktree 返回同一完整 HEAD；
- 两个 `v0.1.0-baseline.1` darwin/arm64 official-mode 候选包的六个二进制都报告该 commit、`2026-09-20T13:00:00Z` 和一致目标；
- 两个候选包的外部 SHA-256、组合安装／故障隔离、Contract 数据库+Evidence clean-room 备份恢复和 Money Flow 数据库 clean-room 备份恢复全部通过；
- 复验结束后 PostgreSQL 只剩各自开发库与系统库，8081／8083 开发服务仍 ready；候选产物保存在可恢复的废纸篓目录，不冒充已发布制品。

## 5. 仍不属于本切片

- 尚未配置远端仓库、签名、CI 发布任务、制品仓库或供应链证明；
- 候选包尚未推送或发布给任何外部环境；
- 不把之前的 `test.*` bundle 重新描述为正式发布。

这些事项不影响门禁和本地可追溯候选验证完成，但在对外正式 Release 前必须单独处理。决策依据见 [ADR-0032](../../adr/0032-official-release-provenance-requires-clean-git-head.md)。
