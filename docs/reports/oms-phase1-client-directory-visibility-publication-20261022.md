# OMS 一期甲方档案可见性纠偏发布报告

日期：2026-10-22  
事项：BF-176  
源码基线：`main@ee7862d`

## 结论

BF-174 把甲方和施工方同时过滤为“已有项目关系”属于过度修正，导致人工录入但尚未建项的甲方2、甲方3被隐藏。本次恢复甲方管理作为建项前置档案入口的语义：显式甲方以及尚未被识别为仅施工方的外部主体均可见，项目关系只补充关联项目数；施工方仍只投影活动 `contractor-party` 关系，避免甲方误入施工方页面。

本次只替换 Unified Workbench，没有修改 Party、Project、Contract、Billing、Money、IAM、Gateway、API、schema、Grant 或业务数据。当前 Party Directory 尚无持久化的“未关联施工方候选”角色，本次没有用浏览器状态或假数据补造。

## 发布制品与回滚点

- Unified Workbench：`v0.1.0-test.oms-v1.27@sha256:e7906954de08ab389083d82b5df615730f2d0ca9073bada7b136151f63e2bc68`
- D1 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.27.https.env`
- 升级前完整备份：`/var/backups/omsbizfoundry/20261022T114210Z`
- 公网入口：`https://oatest.antuinfo.cn/oms-v1/`

## 验证

本地实际通过：

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`
- 甲方／施工方投影回归：2/2
- Workbench Go test 与 vet
- 前端 TypeScript check、206/206 测试及双 bundle build
- 公共验收单测 10/10、Python 语法检查和 `git diff --check`

公网发布后实际通过：

- 候选预检与 IAM contract parser；
- D1 全栈 `check`；
- 17/17 个长运行容器 healthy；
- 正式登录、五个 Gateway Audience 授权与 Owner 只读；
- Owner 实数：Project 1、Party 4、Billing 0、Money 0、Contract workspace 6；
- 关系投影：甲方 `[甲方1]`、施工方 `[]`；
- 页面目录投影：甲方 `[甲方1, 甲方2, 甲方3]`、施工方 `[]`；
- 业务写入为 0。

## 边界

该修复恢复当前数据的正确可见性，但不把页面入口当作 Party 的永久业务角色。若第一版后续要求“先录入尚未关联项目的施工方并立即出现在施工方管理”，必须为该候选角色增加正式、可持久化且可迁移的产品语义；不能依赖浏览器缓存或复制一份 Party 数据。
