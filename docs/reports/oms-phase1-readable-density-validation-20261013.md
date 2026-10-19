# OMS 一期可读性与工作区密度验证

日期：2026-10-13  
决策：BF-155  
公网入口：`https://oatest.antuinfo.cn/workbench/?view=oms-v1`

## 1. 问题与结果

问题 A：OMS 一期原页面依靠 8～11px 字号和 1440px／1080px 窄容器承载业务内容；在笔记本上文字难读，在大屏上内容缩在中央，侧栏和页面两侧留下大量无效空白。  
可验证结果 B：侧栏收窄、工作区放宽，列表、详情、金额链和表单建立一致的可读字号，同时不抬高顶栏、标题区和筛选区。

## 2. 实施范围

- 桌面侧栏由 216px 调整为 196px，1100px 以下为 184px；手机仍隐藏侧栏；
- 普通内容容器取消 1440px 上限，使用侧栏外全部可用宽度；
- 项目详情上限由 1080px 放宽到 1720px；
- 列表正文提升到 13px，表头 11px，次级说明 11px，行高保持 60px；
- 同步提高总览图表、项目金额链、逐施工方分账、项目历史、Evidence 文件行、抽屉表单和预览层字号；
- 顶栏保持 50px，筛选仍为紧凑工具栏，没有新增说明区或重复标题。

本轮只修改 Unified Workbench CSS、缓存版本和回归断言。没有修改 Project、Party、Contract、Billing、Money、Evidence、IAM 的 API、数据库、schema、Grant、命令或业务事实。

## 3. 自动验证

实际通过：

- `go test ./frontend/oms-workbench/...`；
- `node --test frontend/oms-workbench/internal/server/oms_phase1_pagination.test.mjs`，4/4；
- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`；
- 定向 `git diff --check`；
- `deployments/d1/ops/test-release-bundle dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.9_linux_amd64.tar.gz`；
- 远端 `deployments/d1/ops/d1 check`；
- 公网静态资源包含 BF-155 样式标记，`/oms-v1/` 继续 303 到受保护 Workbench。

## 4. 公网浏览器复核

在已登录 Chrome 中实际刷新并检查：

1. 经营总览：回款、支付、月度收支和项目资金占满主工作区，图表标题、图例和金额可直接读取；
2. 项目列表：展开“项目经营 → 项目列表”，首屏展示 20 条，项目、甲方、应收／未回、应付／未付和详情入口保持清晰；
3. 项目详情：打开项目 `2a112e09-c0bf-419d-b615-d3420faeeec5`，基本信息、甲方金额链、施工方金额链、逐施工方分账、金额变更和项目记录全部保持原事实且横向空间明显增加。

验证过程中没有新增、编辑或作废任何业务记录。

## 5. 发布结果

- Unified Workbench：`v0.1.0-test.oms-v1.9@sha256:fd14c22d71694a17c8bb3a822afa49dbb794f9dfa51c4e007c3226a880f6ef38`；
- D1 健康容器：17/17；
- 其他应用镜像、数据库、schema 和 Grant 未改变。

结论：BF-155 通过。该结果确认的是当前 OMS 一期页面的可读性与空间密度，不代表所有显示器缩放、浏览器字体设置或客户真实数据量都已经覆盖；后续只应基于实际使用反馈微调列宽和信息优先级，不应重新整体缩小业务文字。
