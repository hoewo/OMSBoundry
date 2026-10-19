# 原 OA 生产工作面布局与按需读取复验

2026-10-04。状态：本地候选有限通过，未发布 Main，完整生产／OA 未通过。

## A→B 和本批变化

生产页将任务、成本、奖金堆在一起并同时读取隐藏业务 → 复用原 `ProductionInfo` 的总览、AntD 页签、类型并集和样式，让用户在同项目的原栏目内操作，只读取选中的事实所有者。

只读参照原提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`：`ProductionInfo.tsx` 的 TabLabel／withTabScroll、585～759 页签及 759～846 总览渲染。`ProductionInfo.css` 完整逐字复制，SHA `f37b6be103db9ceb2a87eedd2514c1b5171eed07116019596aad58c4343450b6`。这是原布局外壳的抽取，不是复制全部子栏目；54 份[来源摘要](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/original-source-provenance.json)一致也不证明原站现场等效。

保留原总览→人员配置→批复审计→分类型阶段→成本→外委→奖金的顺序、图标、配色、TabLabel 和滚动容器。咨询／勘察／测绘都有原“咨询报告”，勘察另有试验；设计有初设／施工图／变更竣工，多类型按并集展示。成本、奖金只在对应应用选装时挂载。

`production_tab` 深链与经营 `business_tab` 分开；刷新、Back／Forward、键盘 Home／End 后 Enter 保持同项目和原生表单 DOM。隐藏页签挂载 DOM 不等于读取源业务。进入总览不再同时调用 Cost／Bonus；切到相应页才由既有适配器重新授权读取。未接通栏目保持明确禁用提示；汇总未知为“—／未读取”，不能把未读金额计算成零或总成本。没有业务数据库、Owner、IAM、Scope、Money／Evidence 变化。

## 已实际运行的证据

| 验证层 | 结果 | 限制 |
| --- | --- | --- |
| 实际 Workbench＋Gateway＋IAM＋Bonus／Project／People／Contract | [17 场景](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/native-workbench.json)，0 页面错误；原页签、隐藏 Bonus 零读、键盘／深链／Back／Forward、取消零写、正常登记和撤回两版历史通过 | 7 个独立进程、独立 PG18 业务库；来源事实由各自 SQL 测试夹具准备，不是来源创建旅程；无 Nginx／公网 |
| 原成本组件＋新生产布局 | [13 场景](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/cost-components.json)，0 页面错误；总览零成本读取，切页／重入才读，取消零写、原五类选择和连续确认通过 | 显式测试端口；不是生产 IAM／PG／公网 |
| 相邻原合同／支付／开票组件 | 合同 [22](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/contract-components.json)、支付 [14](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/cash-components.json)、开票 [15](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/invoice-components.json) 场景通过，各自零页面错误 | 测试端口，不能替代同项目正式入口 |
| 逻辑／编译 | 前端 125＋embedded runtime 182＝[307／307、0 skip](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/node-tests.json)；TypeScript、Vite、Workbench `go test ./... -count=1`／vet 实际通过 | 非全应用整体验收 |
| 独立启动 | 最新 Workbench 二进制两次启动／退出，匿名只见登录，五个 JS 与 CSS HTTP 摘要[一致](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/standalone-startup.json) | 使用声明的本地 example Suite，无受保护业务调用／DB 写入 |
| Main 原 C 只读可见复查 | [两定案、三协议、640×400 实际预览](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/public-commercial-readonly.json)，C revision 18；Billing 登记按钮 0，明确缺权 | 当前已发布 `.6`，不是本批生产布局候选；0 业务写，但授权及 Evidence access-grant 是技术 POST，不说所有审计零变化 |

实际组合最终 7 条登记／9 版历史／7 个成功命令／9 条审计，旧 Pool／Money 建议均零新增。8 次 POST 尝试＝组件端口 6 次（含丢回复重放）＋实际 Workbench 2 次。工作台正常生产登记 4.04×25%=1.01 后明确原因撤回，两个 revision 保留；Project／People 源摘要不变，Contract 空库零新增。金额撤权后当前和历史五个金额字段缺席，拒绝盲写；错误侧别、旧 Scope、伪造 Header、来源故障零写分别通过。

本轮 Bonus 单库恢复检查点 dump SHA `4af59864f21961f499b56d21770e822a925d653c41e38fb2b6017363f1be39cd`，私有路径 `/tmp/oa-bonus-components.jiONro/bonus-native.dump`。它早于恢复后继续登记和 Workbench 两次写入，不是最终全部事实或 Suite 备份。密码／密钥／Cookie／SQL／dump 不入证据目录。

已实际看图：[生产总览](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/production-overview.png)、[奖金原抽屉](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/production-bonus-form.png)、[撤回历史](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/production-bonus-history.png)、[生产成本原列表](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/production-cost.png)。看到未接通提示不算该子栏目通过。

## 失败没有抹去

两个生产组合失败是定位原图标与 rcTabs 键盘 live label 的旧 exact-name 断言；[首轮](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/failed-tab-icon-locator.json)／[次轮](../../acceptance/oa-initiation-repair/evidence/production-layout-20261004/failed-tab-live-label-locator.json)保留。装饰图标明确 `aria-hidden`，测试匹配末尾可见中文并等待原动画；没有 force 点击、关闭权限或注入业务写。

三个相邻驱动旧夹具使用不存在的 `project-workspace.resource.manage`，新增按钮正确隐藏；改为真实 `resource-link.manage` 后又暴露 GET Details 冒充 POST ResourceLinkResult 的旧夹具。仅修夹具为真实 `{project,resource_link}`，严格关联校验保持。开票卡片定位另误用旧冒号键，实际页面正确显示甲方链记录；改为当前闭合关联词汇。九份失败 JSON 和最后通过回执一并归档，不把夹具错误说成 Main 权限故障。

## 剩余工作与复演

全部类型选装时仍有 9 个子栏目未接通：人员配置、批复审计、方案、初设、施工图、变更竣工、试验、报告、外委。生产总览任务读取仍不是阶段操作。D01 必须先按原专业／角色／抽屉拆解，不能将 production_owner 或通用 participant 冒充设计／复核／审核；见[原操作缺口](../specs/assembly/oa-production-personnel-clone-gap.md)。B11、全角色、原 C 公网、OSS 和 Suite 联合恢复仍未完成。

复演：前端目录 `npm run check && npm run build`；Workbench 目录 `node --test frontend/src/*.test.mjs internal/server/*.test.mjs` 和 `GOWORK=off go test ./... -count=1`。各浏览器驱动使用 `mktemp -d /tmp/oa-<业务>-components.XXXXXX`；Bonus native 带 `--live-sources`，要求自有 PG18 测试容器。`collect-production-layout-receipts.mjs` 只整理已跑本轮固定路径的非私有回执，不是重新验收。

本批不新增稳定架构选择，沿用 BF-084／096／102／103。没有部署 Main、migration 或新增 Grant；Billing／奖金精确授权仍须单独确认，Cost 批准不能代用。候选发布需匹配 Gateway／Workbench／Suite schema／renderer、Bonus schema 3／profile／IAM 目录及 BF-077 备份兼容门禁，不能只推新前端。
