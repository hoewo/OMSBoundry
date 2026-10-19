# OMS 一期项目详情板块化布局验证报告

日期：2026-10-13  
决策：BF-154  
入口：<https://oatest.antuinfo.cn/oms-v1/>

## 结论

项目详情已经从“顶部集中放置所有操作”调整为按业务归属组织的项目档案：顶部只保留返回；项目资料、甲方业务、施工方业务和变更记录分别承担一类信息与动作。用户先看项目身份，再顺序阅读两条资金链，操作入口不再脱离上下文。

## 页面层级

1. 项目资料／基本信息：项目开启日期、甲方、施工方数量、更新时间、项目说明、创建时间和 revision；“编辑项目”位于本板块。
2. 甲方业务／合同与回款：上游合同状态、附件数量以及应收→开票→回款→未回款；“合同详情”位于本板块。
3. 施工方业务／合同与支付：应付→收票→支付→未支付和逐施工方折叠分账；“关联施工方”“设置应付”位于本板块。
4. 金额变更与项目记录：继续读取 Contract 正式变更和 Project revision，不增加业务操作。

桌面版使用明确的板块标题、细分隔线和甲方／施工方侧色彩提示；窄屏下板块操作在各自标题内换行，基本信息变为两列或单列，不恢复顶部横向工具栏。

## 验证

实际通过：

```text
node --check frontend/oms-workbench/internal/server/oms_phase1.js
node --test frontend/oms-workbench/internal/server/oms_phase1_pagination.test.mjs
go test ./frontend/oms-workbench/...
git diff --check -- <本轮代码与文档>
deployments/d1/ops/test-release-bundle dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.8_linux_amd64.tar.gz
远端 D1 check
```

公网 Chrome 在正式登录态打开项目 `2a112e09-c0bf-419d-b615-d3420faeeec5`，实际看到顶部只有“返回项目列表”；“编辑项目”“合同详情”“关联施工方”“设置应付”均位于对应板块。两条金额链、逐施工方分账、项目说明和四条 Project revision 正常显示。

## 发布与边界

仅替换 Unified Workbench：

```text
v0.1.0-test.oms-v1.8
sha256:968877897398e209b4af218602b65a4a9cb1c606b7a04b5c69117a943a99da28
```

远端 17 个长运行容器 healthy。本轮没有修改 Party、Project、Contract、Billing、Money、Evidence 的 schema、API、业务事实或 Grant，也没有新增项目页财务流水和新增动作。
