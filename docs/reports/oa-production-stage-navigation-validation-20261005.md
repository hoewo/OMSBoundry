# OA 生产阶段导航修复验证（2026-10-05）

## 结论

问题 A：初步设计、施工图设计、变更竣工等原 OA 栏目因为原生后端端口尚未实现而被整个禁用，用户连 UI 和后续缺口都无法检查。可验证结果 B：页面导航与原生业务接线状态已分离，适用阶段 Tab 可立即进入并复用原 OA 阶段布局；未接通事实明确显示“未读取”，写操作保持关闭，不伪造空数据、时间、数量或评分。

该修复已发布到 `https://oatest.antuinfo.cn/workbench/`。当前 Workbench 镜像为：

`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench@sha256:be69a0a853608042f29e728876830f852917220c2aed07fb7b0c42297bd604e1`

## 实现边界

- `ready` 只表示原 OA 工作面已装入并可导航；`native` 独立表示事实所有者端口已接通。
- 方案阶段在既有显式 profile 下继续走真实 Production／Evidence 端口。
- 初步设计、施工图设计、变更洽商、竣工验收复用阶段面板的原布局，未知值显示 `—`／“未读取”。
- 未接通页不调用通用 Production reader，也不把方案阶段接口映射为其他阶段。
- Workbench 不新增数据库、Grant、业务事实、能力或跨应用所有权。

## 实际验证

- frontend Node：166／166 通过；
- TypeScript、Vite production build 通过；
- Workbench Go test／vet 通过；
- Business runtime：28／28 通过，包含未接通阶段选择零通用读取；
- 公网浏览器依次点击 10 个生产 Tab，全部 enabled 且成为 active；
- 初步设计、施工图设计分别呈现完整阶段资料、校审单和评分结构；
- 变更竣工同时呈现“变更洽商”和“竣工验收”两区；
- 公网过程 0 个 403、0 个 429、0 个 5xx、0 个页面错误、0 个业务写请求；
- 仅滚动 Unified Workbench；发布前后业务事实摘要与 IAM Grant 摘要一致，17／17 容器健康。

结构化摘要见 [oa-production-stage-navigation-20261005.json](../../acceptance/oa-initiation-repair/evidence/oa-production-stage-navigation-20261005.json)。

## 尚未完成

本批只关闭“Tab 不能点击／页面不可见”的网页故障，不把可见 UI 记作完整业务。初步设计、施工图设计、试验、咨询报告、变更洽商与竣工验收仍需各自的 Production 原生契约、Evidence 引用、IAM 能力、状态／revision 规则、丢回复续办、权限矩阵和备份恢复。完整 OA 与整体架构结论仍为 `insufficient-evidence`。
