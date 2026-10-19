# 原 OA 招投标组件复用与真实文件链验证

日期：2026-10-03；BF-095／BF-096。当前是 Auth58 开发测试部署，不是正式 clean-HEAD 发布或完整 OA 验收。

问题 A → 可验证结果 B：手写简化界面偏离原 OA、已有页面不能操作真实文件 → 原招投标组件直接挂入同项目经营页，连接 Evidence 文件版本、BE 材料引用和 Project 只读资料投影。

## 本批实现与边界

直接复用原仓库的 `BiddingFileList`、`BiddingMethodControl`、`SectionTitle`、`FilePreviewDrawer`、预览 hooks 和 `ConfirmDeleteDialog` 及样式。原组件来源和 SHA 见[来源账](../../frontend/oms-workbench/frontend/source-provenance.json)。使用原 React 18／Ant Design 5／React Query 5 组合；Vite 生成本地 JS／CSS，由既有 Go Workbench 嵌入，不建设另一套 UI 或组件库，也不把整个原后台搬来。

必要差异：把原整数组更新端口替换成明确 `appendUploadedFile`／`removeMaterialReference`；移除只结束 BE 的这一引用，使用原确认组件，不调用物理文件删除。原生权限及可信 Actor 仍由 Gateway／IAM／各目标应用检查；Workbench 不签身份、不拥有材料或文件事实。

文件传输直接用已有 File Transfer SDK。修复原生 `Window.fetch` 因错误 receiver 导致的 Illegal Invocation，并补回归；不另写 OSS 客户端。Evidence `/v1` 路由显式接入既有 helper，保持原鉴权。未知材料回包保存 24 小时内、按 Tenant／Subject／Project 隔离的有界 File／Version 和原命令暂态；不存 STS、密码或授权 URL，不作为业务事实。

BE 自有库从 000001 前向到 000002，兼容旧 put/remove、hash／回执；Project 产品镜像、schema 10 和事实不变。没有新小系统、共享业务表、跨库事务、IAM Grant 或权限旁路。

## 实际部署与恢复证据

发布版本 `v0.1.0-oa.20261003.1`；源归档 SHA `f95a62ad1957c7ee52596c1f2195fffe8f28c0b25f1bbb491e41c409c366d254`。两个 ACR 不可变 digest：

- BE：`sha256:3edd046daf159bef2373b3a71a5214e2b265b76e62e4a89f4bed705ec0233a35`。
- Workbench：`sha256:c53999c753e8363e7b0f8ec9c24548933094e5fb49202751da0aa4ffe24ce688`。

Main 只替换这两个容器，其余 15 个 ID 不变；部署时 D1 check、17/17 healthy、BE 前向迁移及旧事实保持通过，IAM 契约和 Grant 不改，见[发布回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/cloud/release/publication/results.json)与[部署回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/cloud/backup/rollout.json)。当前运维入口是 `/opt/omsbizfoundry/acceptance/oa-bidding-material-release-20261003a/d1-active`，不能再用历史 k wrapper 执行 up。

升级前联合备份 13 个库、Evidence 字节和 IAM keys，13 个独立空目标逐库还原，canonical facts、sequence、文件摘要保持。原 BE 二进制检查 000001；候选迁移 000002、重复 migration 保持旧事实，Project schema／事实保持，见[恢复回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/cloud/backup/restore.json)。**这不是完整 Suite 还原后的运行旅程。** 私有恢复 PG 完成后停止，restore20 未重开。

首轮恢复 one-shot 把 `/app/ops` 传给 server ENTRYPOINT 而失败，Main 尚未修改。checker v2 显式设置 ops ENTRYPOINT，在同一批已还原目标继续验证，没有重建目标来掩盖失败。v2 是补充验证驱动，不能称它已包含在此前不可变源归档内；产品发布物没有为此重编。旧库＋旧镜像联合恢复才是回退方案，禁止旧镜像直接读取 000002。

离机私有备份已实际复制并核对 SHA `ac4ac2d7173f288406ece902b7df6b09bcc9ec368e7c769e0a67e3c1c90ca8ba`，见[本机安全回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/offhost-local.json)。文件本身只在 ignored runtime 中，不进入仓库。该次复制完成／摘要验证与 apply 执行有重叠，不声称部署驱动自动执行了离机前置门禁；后续必须先等待并验证复制，再 dispatch apply。

## 已实测操作

真正入口为 [oatest 工作台](https://oatest.antuinfo.cn/workbench/)，使用原生 IAM 会话、Gateway、Main 自有库和 Chrome；不是 127.0.0.1 合成 API。

只继续既有 G 验收项目 `3b4c44d3-7b33-4a29-add3-3f6211ca893e`／BE `231f2977-79d4-4b36-a178-f19a0d8d3119`，不替换夹具或修改用户“测试项目101”。

- 原三类组件真实上传两份招标、一份投标、一份中标 PNG；四个 available 版本的大小及 SHA 复读相等，BE 从 revision 7 到 11／ready。ready 仅表示材料齐备。
- 原预览抽屉实测显示图片；原下载按钮得到实际字节，SHA 与上传一致。
- 原确认弹窗精确移除第一份招标材料，BE 到 revision 12，另三份当前引用保留；四个物理文件仍 available，没有 Evidence DELETE。
- Project 读取三份对应资料，项目 revision 4 不变。来源 BE revision 已变化，投影状态为 `revision_changed`，不是错误地写成所有 segment available；没有偷偷刷新旧 Link 或写 Project。
- 显式刷新和浏览器 reload 复读同项目、零新增业务事实，见[public-f 的分阶段观察](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/public-f/public-react-continue.json)。该批整体 passed=false，不能把上述已观察的阶段说成其未知回复验收也通过。

未知回复续验 h 在同一 G 从原生 revision 14 继续：新 append 已真实提交到 15，浏览器故意丢弃回包；等操作结束后产品有界暂态包含精确原命令。真实 reload 后点击“继续原操作”，两个请求的 command ID／完整载荷相等，最终仍 revision 15、七份历史材料／六份有效引用、Project revision 4／六个资料 segment。没有重传或物理删除，暂态在确认后清除，浏览器 pageerror 为零。见[续验 passed 回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/public-h/public-retry.json)和[真实组件终态](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/public-h/continued.png)。这是提交成功后丢回复的故障注入，不是自然网络事故；也没有关闭 429 压力／撤权等全部负例。

最终所有权门禁已实际通过：13 库逐自有表 canonical 摘要比对，仅排除精确 G 的 BE 行／历史／回执与其文件／上传会话，IAM 登录／审计／nonce 明确动态；其余事实（包括全部 Project、用户101及 IAM Grant）与浏览器前一致。终态 BE revision 15／ready、七份历史／六份有效材料，Project revision 4。检查器只读，不能以浏览器零 Project 请求计数替代这项整库比对；见[前态](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/cloud/release/material-browser-ownership/before.json)／[后态](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/cloud/release/material-browser-ownership/after.json)。

公网续验后再次执行当前 wrapper check exit 0；只有 Main 17 个容器运行且全部 healthy，私有探针／恢复 PG／restore20 未运行。九份安全 JSON 已离机复制并核对 SHA 相等，没有导出 env、key 或私有备份到仓库，见[只读收尾回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/public-closing-check.json)。

## 自动化与保留反例

Workbench Node 全量 172 项（原 156＋新适配器 16）零 fail／skip；Workbench／BE Go tests、vet、原生 Workbench 启停两次与嵌入资源摘要核对；前端 TypeScript／Vite build、SDK 17 项／类型检查／build 均实际运行通过。续验／配置／挂载纯门禁 17 项及显式 ops ENTRYPOINT 3 项再次运行通过。一次误用 test_bidding_material*.py 的 discover 匹配零测试，不算通过，随后以上两个正确模式分别运行 17／3 项。PG18 d/e、独立启动 i 和旧库／单库恢复证据保留在[此前报告](./oa-bidding-material-collection-validation-20261003.md)，不拼成全部原生依赖／角色矩阵。

本机组件 Chrome 测试是明确 HTTP 夹具，不等于 Main，见[独立组件观察](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/react-d/browser-fixture.json)。前面的 JSX 构建缺少环境常量、SDK receiver、Ant Design 双字按钮匹配、页面隐藏 demo 列表选择器及相对脚本路径反例均保留；修正后才重跑，不放松产品校验。

公网 a 过期入口 Cookie、d/f 密集读取触发 Gateway 429 均保留，未扩限流或延长凭证期限。f 中首次模拟 native POST 已成功但浏览器丢失回复，随后 reload 因 429 未回原项目；这次续验不通过。g 的按钮可见时仍 disabled／上传中，脚本提前读到尚未 finally/persist 的 null command；同一来源已提交到 revision 14，不回滚或删掉它。h 修正为等待按钮启用及产品自行保存的原命令后观察，仍在同一 G 继续；绝不注入重建的 pending 状态冒充刷新恢复。

## 明确未关闭

Main Evidence 仍为 `local`／`local-http`，本批文件链不是 OSS／STS 直传验收；前端仍由 ECS 上 Workbench 提供，OSS 静态前端分离仍待办。CSP 只为当前来源及 blob 预览精确设置，未来启用 OSS 仍需其准确来源与专项负例。

未关闭完整 B03 的角色／撤权／错误文件与大文件矩阵、原站现场行为等效、完整 Suite 恢复运行、立项剩余字段、B02 经营成员、合同／财务／成本／奖金、分类型生产、资料统计或完整 OA。轻量透镜已随 Workbench 发布，但 Evidence 独立 step 被 key 白名单遗漏，完整文件调用学习链仍需补验；不能把默认说明当作完整运行轨迹。原列表的“覆盖上传”文案也需按新明确追加语义小修，不改变实际 append 操作。

整体 OA 架构结论继续为 `insufficient-evidence`；后续按原 OA 操作基线逐切片复用组件、验证业务，不以 UI 复用数量替代可用系统。
