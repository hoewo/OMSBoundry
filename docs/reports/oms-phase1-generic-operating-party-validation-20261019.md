# OMS 一期通用经营主体与真实主体列表验证

日期：2026-10-19  
决策：BF-164  
结论：本地通过；公网未发布

## 修正结果

- “资质单位”只保留为历史记录和历史合同角色的读取兼容，不再是活动 OMS 的产品概念。
- 当前使用系统的公司统一称为“本方经营主体”，新部署变量为 `OMS_OPERATING_PARTY_ID`。
- 旧 `OMS_QUALIFICATION_PARTY_ID` 仅用于已有部署兼容；两项同时配置且值不同会失败关闭。
- 新合同写入中性角色“本方主体”，已有合同角色不静默改写。
- 甲方和施工方列表仅使用 Party Directory API 返回的持久化集合，不包含静态、示例、默认或失败兜底主体。
- Party Directory 为空或不可读取时，列表保持空状态，不以假数据掩盖真实状态。

## 实际检查

- `make fmt-check && make test && make vet`：通过。
- Workbench 前端 `npm test`：206/206 通过。
- `npm run check` 与 `npm run build`：通过。
- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`：通过。
- `sh deployments/d1/ops/test-release-bundle`：通过。
- `sh -n deployments/d1/ops/d1` 与 `git diff --check`：通过。
- 本地 `127.0.0.1:18182` 使用本次源码重新构建并重启，`/health/ready` 返回 ready。
- 浏览器刷新后实际打开甲方管理与施工方管理，分别显示“暂无人工录入的甲方／施工方”和“系统不会补充演示或默认数据”。

## 边界

- 本轮没有删除 Party Directory 中已经存在的真实持久化主体。
- 系统目前没有可靠的“人工录入／脚本录入”来源字段，因此不会用名称或创建人猜测并删除已有记录；若要追溯来源，应由 Party Directory 后续增加版本化来源事实。
- 本轮没有改 Owner API、schema、Grant、数据库或既有业务事实。
- 本轮没有发布到 `oatest.antuinfo.cn`，公网版本仍以最近一次实际发布记录为准。
