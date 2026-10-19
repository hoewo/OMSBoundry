# 参考 OA UI/UX 逐页对齐审计（2026-09-30）

## 结论

参考 OA 的已登记路由现在都已有可达页面：登录、应用外壳、项目列表、立项、项目详情、资料、统计、公司收入、项目收入、合同详情、设置、人员、权限、甲方、资质单位和使用指南均复用参考项目的 CSS／class 层级，并已在本地浏览器完成关键点击验证。公司 D1 已完成新版登录页真人浏览器复核，并通过容器内可信 Actor 页面探测确认全部新增路由和业务样式随镜像交付。这个结果关闭的是“参考 OA 页面是否能按当前装配架构呈现”的 UI/UX 可行性问题，不等于完整 OA 业务已经最终验收：若干列表和明细仍明确标注为“界面验证数据”，新增／编辑写动作、真实投影绑定及登录后的 D1 全路由人工点击仍须分别给出证据。

本审计以用户拥有的参考项目 RoadSystemOA/road-design-oa-system/frontend/src/App.tsx 为路由真值，以对应页面 TSX/CSS 为视觉和交互真值。BizFoundry 不能用“风格相近”代替逐页对齐。

## 逐页矩阵

| 参考路由 | 参考页面 | 当前承载 | UI 状态 | 行为/事实状态 | 优先级 |
|---|---|---|---|---|---|
| /login | 登录 | Unified Workbench 未认证页 | 默认可见状态已复用源级品牌、水印、手机号和“获取验证码”结构 | 短信 Provider 尚未接入，点击后明确提示未实现；同卡片可切换到真实 IAM 用户名／密码，未用假验证码冒充认证 | UI 已完成，短信能力待选 |
| /projects | 项目管理 | projects | 源级年表／分页、筛选、状态珠、项目包／子项目层级与展开态已复用 | 搜索子项目会自动展开所属项目包；进入详情会同步项目名、编号、日期与负责人；列表仍含演示事实 | P0 深层 UI 已补齐 |
| /project-initiation | 项目立项 | initiation-reference | 源级范围筛选、搜索、分页、新建／完善立项抽屉、项目／项目包切换、字段校验和删除确认已复用 | 本地深层点击与 D1 静态交付通过；真实创建／补录仍通过 Gateway 进入 Project Workspace | P0 深层 UI 已补齐 |
| /projects/:id | 项目详情 | project-detail | 顶部、一级页签、基本信息只读网格、经营 9 个二级页签和生产 11 个二级页签已按源组件层级复用 | 本地已点击基本信息、经营总览、合同、生产总览和施工图阶段；D1 登录后全页点击仍待验 | P0 深层 UI 已补齐 |
| /projects/:id/revenue | 项目收入 | project-revenue | 源级收入卡片、结算和台账结构已复用 | 页面数据明确标注为界面验证数据；真实资金／合同事实仍由独立应用拥有 | P0 已完成 UI |
| /contracts/:id | 合同详情 | contract-detail | 源级合同、补充协议与文件页签结构已复用 | 文件列表／上传页签可切换；真实合同写入仍由 Contract／Evidence 承担 | P0 已完成 UI |
| /files | 项目资料管理 | files-reference | 源级项目/文件展开结构、下载摘要和人员明细弹窗已复用 | 两个有资料项目可独立展开／收起；Evidence 下载入口可用；页面列表仍含演示事实 | P0 深层 UI 已补齐 |
| /statistics | 统计信息 | statistics-reference | 源级 5 页签、项目台账、公司统计、空状态与人员奖金展开表已复用 | 经营指标投影可读取；其余汇总明确为演示事实 | P0 深层 UI 已补齐 |
| /company-revenue | 公司收入 | company-revenue | 源级统计矩阵与项目收入台账结构已复用 | 页面事实明确标注为界面验证数据；实时指标仍保留在统计投影区 | P1 已完成 UI |
| /settings | 设置 | settings | 2026-09-30 已复用源 CSS 和双卡片结构 | 导航到人员/权限页可用 | P1 |
| /users | 人员管理 | users | 已复用源 CSS、目录列表、详情以及新增／编辑抽屉结构 | D1 已点击新增抽屉和带既有值的编辑抽屉；预览提交只做界面校验，真实账号与人员写入仍由 IAM、People & Groups 承担 | P1 深层 UI 已补齐 |
| /permissions | 权限配置 | permissions | 2026-09-30 已复用源 CSS、左栏与权限组结构 | IAM 仍是最终授权来源；模板编辑尚未接真实写入 | P0 |
| /clients | 甲方管理 | clients | 源级列表、搜索、详情、新增／编辑抽屉和删除保护弹窗已复用 | D1 已点击新增、编辑及“已关联项目不可删除”；预览不伪造写入，真实保存仍由 Party Directory 承担 | P1 深层 UI 已补齐 |
| /qualification-units | 资质单位管理 | qualification-units | 源级列表、搜索、详情、新增／编辑抽屉和删除保护弹窗已复用 | D1 已点击新增抽屉；编辑与删除使用相同的页面内作用域交互，真实保存仍由 Party Directory 承担 | P1 深层 UI 已补齐 |
| /guide | 操作手册 | guide | 源级 Hero、模块、流程和折叠问答结构已复用 | 问答折叠和返回路径已实际点击验证 | P2 已完成 |

## 完成口径

“完整仿 OA”只有在以下证据同时成立时才能结束：

1. 上表所有参考路由均存在可达页面，视觉结构与源 TSX/CSS 对齐。
2. 侧栏、设置入口、列表筛选、折叠、页签、返回、详情进入和权限受限状态均实际点击验证。
3. 页面展示的业务事实要么来自对应业务应用的版本化 API/投影，要么明确标注为演示数据；不得把静态样本冒充真实链路。
4. 登录与受保护访问继续通过 Nginx → Gateway → IAM，目标应用继续执行最终授权。
5. 至少完成一轮公司 D1 实机浏览器验收，记录镜像 digest、关键页面截图/探测和恢复边界。
6. 验证报告更新为逐项证据结论；未完成项不得用总百分比掩盖。

## 当前批次边界

本批次只扩展 Unified Workbench 前端装配，不改变业务应用数据所有权，不共享数据库，不把 IAM 权限事实复制进工作台，也不一次启动整套本地 Docker。

## 2026-09-30 批次证据

- 设置、人员管理、权限配置三个页面已在本地浏览器实际点击验证，返回路径正常，浏览器无 error/warning。
- frontend/oms-workbench 的 go test、go vet、工作台 JavaScript 语法检查和 git diff --check 通过。
- Linux/AMD64 接受镜像更新为 `unified-workbench:v0.1.0-test.44@sha256:58714f4d04c6b7a08250fb19d73859c114ca45a0c4cd865de08b56019971c47b`，AMD64 manifest 为 `sha256:916a8771167374dfcb092a0941f101b77d91d0267b879106f7ed58692166e7b8`。
- 公司 D1 只重建 Unified Workbench，17/17 服务 healthy；其他 16 个主栈服务继续保持原运行代次。容器内带可信 Actor Header 的页面探测确认甲方、资质单位、公司收入、项目收入、合同详情、使用指南及业务 CSS 均存在，页面大小 `123528` bytes、业务 CSS `47839` bytes。
- `http://127.0.0.1:18090/workbench/` 返回 HTTP 200；真人浏览器实际看到参考 OA 的品牌标题、水印背景和居中登录卡片。登录仍使用真实 IAM 用户名／密码，不伪造参考项目的短信验证码服务。
- 更新前环境文件备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui44-full.env`。

### 第二批源级路由补齐（本地与 D1 发布通过）

- 直接复用参考 OA 的 CompanyRevenue、CompanyRevenueStatistics、RevenueProjectLedger、ContractAmendmentList、ContractForm 与 UsageGuide 样式，并补入甲方／资质单位、公司收入、项目收入、合同详情和使用指南页面。
- 本地浏览器实际确认：登录品牌卡片、项目列表、项目详情经营入口、公司统计两列响应式布局、甲方列表和档案抽屉、合同文件“文件列表／上传文件”页签、使用指南问答展开均可用；1280 像素视口 `scrollWidth=viewportWidth`，没有横向溢出。
- 登录页结构和 CSS 与参考 OA 一致；认证字段继续使用真实 IAM 用户名／密码，不伪造参考项目的短信验证码服务。
- `frontend/oms-workbench` 单元测试、`go vet`、嵌入 JavaScript `node --check` 和变更空白检查均通过；公司 D1 已发布上述 `test.44` 固定摘要并完成登录页真人浏览器与容器内全路由存在性复核。登录后的 D1 全路由人工点击仍是独立待验项，不能由容器探测或本地点击替代。

### 第三批项目详情深层工作台（本地点击与 D1 发布通过）

- 纠正了项目详情只有少量应用入口卡、不能代表参考 OA 工作面的缺口。基本信息现使用参考 OA 的只读字段网格和经营／生产双进度；经营区加入总览、甲方、参与人、招投标、合同、支付、开票、经营成本和奖金分配；生产区加入总览、人员、批复审计、方案、初设、施工图、变更竣工、咨询报告、生产成本、外委和奖金分配。
- 二级页签和“下一步”快捷入口在浏览器中真实切换，不通过共享数据库或复制领域对象实现；进入正式写操作仍提交已有 Gateway 授权表单到对应独立应用。静态项目与金额继续显示“界面验证数据”，不冒充来源投影。
- 本地只启动一个 Workbench 进程和轻量 Actor Header 代理，实际点击基本信息、经营总览、合同信息、生产总览和施工图设计，浏览器控制台没有 error/warning。串行 `go test -p 1 ./...`、`go vet ./...`、嵌入 JavaScript `node --check` 和变更空白检查通过。
- Linux/AMD64 接受镜像为 `unified-workbench:v0.1.0-test.45@sha256:d23d4959436c73818b1a265445541553eb2343c13cdc1092dd5c510b5b3ae155`，AMD64 manifest 为 `sha256:7f6fb9b6b2db0c1da51907bbfee0eeed5d0d0631ac63df9f5f1907b98d027680`。
- 公司 D1 只重建 Workbench，主栈仍为 17/17 healthy，其他 16 个服务未重建；容器内可信 Actor 探测得到页面 `159591` bytes、深层详情 CSS `15659` bytes，并确认经营／生产二级工作台标记随镜像交付。更新前环境文件备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui45-full.env`。
- D1 浏览器当前停留在真实 IAM 登录页，尚未输入用户凭据。因此当前界面状态更新为 `reference-oa-deep-project-workspace-d1-published / authenticated-d1-click-through-pending`，不是完整 OA 业务通过。

### 第四批资料与统计深层交互（本地点击与 D1 发布通过）

- 纠正了统计页只有第一个页签有内容、资料页默认摊开静态文件行的浅层实现。统计页现按参考 `Statistics.tsx` 提供项目台账、公司统计、人事考勤统计、薪酬统计和人员奖金统计五个可切换页签；人员奖金行可展开项目级金额明细，考勤／薪酬保留参考 OA 的明确空状态。
- 项目资料页现按参考 `FileManagement.tsx` 从收起状态开始，两个有资料项目可分别展开／收起；文件行提供更新时间、下载次数、少量人员摘要和多人下载明细弹窗。真正下载按钮仍通过 Gateway 授权进入 Evidence Service，没有在 Workbench 复制文件事实或伪造签名地址。
- 本地只运行单 Workbench 进程和轻量 Actor Header 代理，真人浏览器依次验证资料项目展开、3 个文件名、4 人下载明细弹窗、统计 5 页签和 3 项人员奖金明细；浏览器控制台无 error/warning。目视复核确认表格层级、展开背景、页签选中态和弹窗均与参考页面层级一致。
- 串行 `go test -p 1 ./...`、`go vet ./...`、嵌入 JavaScript 语法检查和 `git diff --check` 通过。Linux/AMD64 接受镜像为 `unified-workbench:v0.1.0-test.46@sha256:855389824a9af82c1c27becf2fce8d507db796149cc3812090fa064da52db54c`，AMD64 manifest 为 `sha256:a562b44513d6d6e1cb7c4f8b1beb0160aeefb5986d603ed861af240a4d7bff23`。
- 公司 D1 只替换原 `omsbizfoundry-unified-workbench-1`，最终原项目保持 17/17 healthy；其他 16 个服务未重建。容器内探测确认资料／统计模板、页签／展开脚本和补充样式，页面 `159813` bytes、脚本 `17561` bytes、工作区 CSS `75440` bytes；SSH 隧道入口返回 HTTP 200。更新前环境备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui46-full.env`。
- D1 真实 IAM 登录后的逐页人工点击仍待用户登录后执行；静态界面事实也仍需按业务优先级替换成版本化投影。因此当前界面状态为 `reference-oa-p0-deep-interactions-d1-published / authenticated-d1-click-through-pending`，完整 OA 总结论保持不变。

### 第五批项目列表、人员／权限交互与登录页纠偏（本地点击与 D1 发布通过）

- 项目列表补齐参考 OA 的年表／分页视图、关键字、经营阶段、生产阶段、筛选计数、清空、年份定位和分页交互。分页实测从“第 1-2 条 / 共 3 条”前进到“第 3-3 条 / 共 3 条”，并只显示第三个项目；搜索和经营阶段组合只保留 `S207省道改扩建工程`。
- 人员管理补齐搜索、角色／状态筛选和人员详情抽屉；本地实测筛选只返回 `赵倩`，点击后详情抽屉显示相同人员。权限配置补齐职位／项目角色区域、权限组勾选、未保存态、撤销／重置和保存确认。保存仅记录当前页面验证状态并明确“未写入真实 IAM”，没有伪造权限持久化。
- 用户指出登录字段仍不像参考 OA 后，默认未认证页从 IAM 用户名／密码改回参考源码的“手机号 → 获取验证码”首屏，原 `Login.css`、品牌资源和 class 层级继续复用。手机号格式校验与未接短信提示已实际点击；同卡片提供明确的 IAM 账号密码入口，真实登录仍经过 Nginx → Gateway → IAM。当前没有短信 Provider，因此没有制造固定验证码或假登录成功。
- 本地仅启动一个 Workbench 进程，浏览器实际检查默认验证码页、合法手机号启用按钮、未接短信提示和 IAM 登录切换；登录页、项目列表、人员抽屉、权限编辑均无 console error/warning。1280×720 下页面与主工作区均无横向溢出。串行 `go test -p 1 ./...`、`go vet ./...`、两个嵌入 JavaScript 语法检查和 `git diff --check` 通过。
- 最终 Linux/AMD64 接受镜像为 `unified-workbench:v0.1.0-test.48@sha256:ad5996bb12975ec4cd73aa3edef17529be01349ce3c60da2b30b08c6c11fa28c`；OCI index 为 `sha256:acb7880ad6e0104f448b0fd5b4bb71922e06504575e742f54bae8332cbba5331`。中间 `test.47` 只完成 ACR 发布，未作为 D1 接受版本。
- 公司 D1 只重建 `omsbizfoundry-unified-workbench-1`，主项目最终 17/17 healthy，其他 16 个服务未重建；SSH 隧道匿名探测确认 `data-login-panel="sms"`、`data-login-panel="iam"`、手机号文案、登录脚本和补充样式全部来自 D1。更新前环境备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui48-oa-login.env`。
- 当前界面状态更新为 `reference-oa-primary-list-admin-login-ui-d1-published / authenticated-d1-click-through-and-sms-provider-pending`。短信登录是否进入 Base 仍需产品／安全决策；在此之前，页面必须明确能力缺口，不能把 UI 复刻等同于短信认证完成。

### 第六批项目立项深层交互（本地点击与 D1 发布通过）

- 纠正此前项目立项只有静态两行和直接跳转、不能代表参考 `ProjectInitiation.tsx` 的问题。现在支持“关于我的／全部项目”、名称或编号搜索、两页分页、新建抽屉、普通项目／项目包切换、必填校验、完善立项抽屉、项目编号格式校验和删除确认。
- 工作台只验证源级界面工作流；真实创建、补录和删除不在 Workbench 伪造。字段通过页面校验后，最终提交继续通过 Gateway 与 IAM 进入 Project Workspace；删除确认明确保持业务事实不变。
- 本地只运行一个 Workbench 进程和轻量 Actor Header 代理。实际验证范围筛选得到 3 条、分页第 2 页只显示“南城片区综合管廊设计”、搜索只显示“北环快速路工程项目包”；项目包抽屉切换、空字段错误、完善立项非法编号错误、删除确认与不写入提示均可见。视口 `scrollWidth=clientWidth=1239`，浏览器 console 0 error/warning。
- 串行 `go test -p 1 ./...`、`go vet ./...`、嵌入 Workbench JavaScript 语法和 `git diff --check` 通过。Linux/AMD64 接受镜像为 `unified-workbench:v0.1.0-test.49@sha256:50f4a647235dbb3e2bd4ec4b47eb7de22e765e140b453d623b1b14a14fbd7027`，OCI index 为 `sha256:e3406da07b84a5489314dd0f6f95822f619d51d10a99170a78bec1b98d0b7cd5`。
- 公司 D1 只重建 `omsbizfoundry-unified-workbench-1`，主项目保持 17/17 healthy，其他 16 个服务未重建。容器内可信 Actor 探测确认立项模板、两个抽屉、Project Workspace 真实跳转、交互脚本与 CSS；匿名入口继续显示参考 OA 手机验证码首屏。更新前环境备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui49-initiation.env`。
- 当前界面状态更新为 `reference-oa-initiation-deep-ui-d1-published / authenticated-d1-full-route-click-through-and-real-projection-binding-pending`。本批关闭立项页浅层 UI 缺口，不代表所有静态界面样本已经接入真实版本化投影，也不改变完整 OA 的 `insufficient-evidence` 结论。

### 第七批“登录入口不等于 OA Demo”纠偏（只读演示入口与 D1 发布通过）

- 用户实际验收指出：把匿名登录页交付为“仿 OA 网站”是错误口径。登录页即使逐 class 复用了参考源码，也只能证明入口 UI，不能让验收者看到项目列表、项目详情、经营／生产页签和管理页面。
- Workbench 新增显式 `?preview=oa` 只读界面演示。它直接进入参考 OA 主界面，使用已明确标注的界面验证数据；预览 Actor 不进入 IAM，也不调用个人工作／经营指标等受保护投影。所有 `/gateway/*` 表单提交在浏览器端被拦截并提示“真实业务操作必须通过 IAM 登录”，正式入口的 Nginx → Gateway → IAM 边界保持不变。
- 本地单进程浏览器实际打开项目年表，进入“京哈高速改造工程”，切换到经营信息并看到经营 9 个二级入口；点击退出表单得到只读提示且 URL／页面状态不变。浏览器 console 为 0 error／warning。
- `go test -p 1 ./frontend/oms-workbench/...`、`go vet ./frontend/oms-workbench/...` 与变更空白检查通过。Linux/AMD64 接受镜像为 `unified-workbench:v0.1.0-test.50@sha256:159af0e3c102c9533ac1bb3c5526e7561898f2575e3110127a7a3ac226f40f1e`，OCI index 为 `sha256:f96c0b817a5dbd70ae60dfb1c815c83bf37a659e70fd194133884f8465024623`。
- 公司 D1 只重建 Workbench，主项目保持 17/17 healthy，其他 16 个服务未重建；SSH 隧道入口 `http://127.0.0.1:18090/workbench/?preview=oa` 返回 200，并由真人浏览器复核项目列表与经营详情。更新前环境备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui50-preview.env`。
- 该入口解决“看见的只有登录页”这一验收阻断，不把只读演示冒充 IAM 登录、真实投影或完整 OA 业务验收。剩余结论仍是 `reference-oa-readonly-demo-d1-published / authenticated-d1-full-route-click-through-and-real-projection-binding-pending`。

### 第八批项目包层级与详情上下文（本地与 D1 真人点击通过）

- 对照参考 `ProjectList.tsx` 补上此前平铺列表缺失的项目包／子项目层级：项目包显示源级角标、子项目数量、展开箭头和缩进连接线，两条子项目使用参考 `project-list-page__children`、`project-list-page__project-node--child` 与 `project-list-page__item--child` class 契约。
- 展开／收起在本地和公司 D1 均实际点击；收起后子项目不可见，搜索 `S207` 时所属项目包自动展开且结果摘要为 1 个项目。点击子项目后，项目详情顶部同步为 `S207省道改扩建工程 / S207-2026-006 / 2026-08-16 / 王可 / 待设置`，侧栏继续保持“项目管理”选中。
- `go test -p 1 ./frontend/oms-workbench/...`、`go vet ./frontend/oms-workbench/...` 和变更空白检查通过；本地与 D1 浏览器 console 均为 0 error／warning。仅启动一个本地 Workbench 进程，验收后已停止。
- 公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.51@sha256:ae07d038fba4af54f20f64f0423d4003be89061bc51a1a301d850ba0f668836c`。部署时发现未显式指定 Compose project 会生成孤立 `d1-unified-workbench-1`；该容器和 `d1_app` 网络已立即清理，并用 `-p omsbizfoundry` 只替换主栈 Workbench。最终主栈 17/17 healthy，其他 16 个主栈服务容器 ID 保持不变。
- 更新前环境文件备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui51-project-package.env`。当前可见入口仍为 `http://127.0.0.1:18090/workbench/?preview=oa`；本批关闭项目列表层级交互缺口，不替代真实 IAM 登录、真实业务投影／写动作或最终完整 OA 验收。

### 第九批项目包／子项目详情层级（本地与 D1 真人点击通过）

- 清除 Workbench 内旧的立项、资料、统计、人员、权限、项目列表与旧详情占位 DOM；本地浏览器确认所有目标页由参考模板唯一注入，旧节点为 0，项目包与两条子项目未回退。该清理与本批可见改动一并进入 `test.52`。
- 对照参考 `ProjectDetail.tsx`／`ProjectDetail.css` 补齐项目详情的 `project`、`package`、`child` 三种视觉与交互状态。项目包显示绿色角标、包级渐变、子项目导航及添加入口；子项目显示紫色角标、所属项目包返回路径；普通项目会完全移除上述层级控件。
- 项目详情正文不再只换顶部标题：项目名称、编号、承接日期、经营／生产负责人、项目类型、项目概况和两类阶段均随所选项目切换。项目包→S207 子项目→所属项目包→项目列表→京哈普通项目的完整往返已在本地与 D1 真人浏览器实际点击；1280 像素视口 `scrollWidth=clientWidth`。
- `go test -p 1 ./frontend/oms-workbench/...`、`go vet ./frontend/oms-workbench/...` 与变更空白检查通过。公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.52@sha256:55bb75727845df8e2eb6510919658167645b2202712e74165bbb97882130b83b`，只重建主栈 Workbench；主栈 17/17 healthy，其他 16 个容器 ID 不变。
- 更新前环境文件备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui52-project-detail-hierarchy.env`。首次 pull 使用了构建终端中被截断的摘要，ACR 在容器重建前拒绝；读取 tag 的完整 manifest 摘要后修正并成功发布，失败尝试没有替换正在运行的容器。
- 本批关闭详情层级和正文上下文复用错误；经营／生产深层样本仍有静态验证数据，真实版本化投影、真实写动作和最终完整 OA 验收仍分别待证。

### 第十批主体与人员维护抽屉（本地与 D1 真人点击通过）

- 继续直接对照参考 OA 的 `ClientManagement.tsx`、`QualificationUnitManagement.tsx`、`UserManagement.tsx` 及其表单组件，补齐此前列表可看但新增／编辑仍直接跳往其他应用的浅层交互。甲方与资质单位现在具有名称、邮箱、地址、统一社会信用代码、开户银行、银行账号及编辑状态字段；人员具有账号信息、本地资料、部门、角色、验证和启用状态分区。
- 这些表单属于只读 OA 演示的界面契约验证：字段校验通过后只显示“真实保存由版本化契约完成”，不会制造主体、IAM 账号或人员目录写入。正式写入边界仍分别属于 Party Directory、IAM 与 People & Groups。
- 甲方与资质单位详情增加参考式删除阻断。D1 真人回归发现初版弹窗被放在另一个隐藏业务 view 内，点击只关闭详情而不显示提示；最终将弹窗按实体目录放入各自页面作用域并增加双实例测试，随后在 D1 实际确认“已关联项目、暂时不能删除”弹窗可见。
- D1 真人浏览器已验证项目首页、项目详情、甲方新增／编辑／删除保护、资质单位新增、人员新增及带 `lichen / li.chen@example.cn / 李晨 / 经营部 / 经营经理` 既有值的编辑抽屉。最终可见入口仍为 `http://127.0.0.1:18090/workbench/?preview=oa`，不是匿名登录页。
- 串行 `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...` 和变更空白检查通过。最终 D1 接受镜像为 `unified-workbench:v0.1.0-test.54@sha256:407e94db48244aabe84b49eb9699fbb83dbd69d88fdb06c624637da3b9b1752a`；`test.53` 只短暂用于发现删除弹窗作用域问题，不是最终接受版本。
- 公司 D1 用 `docker compose -p omsbizfoundry ... up -d --no-deps unified-workbench` 只替换 Workbench，最终 17/17 healthy，其他 16 个容器 ID 与本批开始前逐项一致。最终回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui54-delete-guard.env`。
- 本批关闭的是三个管理目录的主要 UI/UX 交互缺口，不是主体／账号／人员真实写入验收。完整 OA 结论仍为 `insufficient-evidence`，剩余核心证据是把界面验证数据逐步绑定到版本化真实投影／命令，并完成正式 IAM 登录后的连续业务旅程。

### 第十一批项目基本信息编辑抽屉（本地与 D1 真人点击通过）

- 用户再次指出“仿 OA”不能用登录页或只有只读字段的工作台代替。验收入口继续固定为 `http://127.0.0.1:18090/workbench/?preview=oa`；最终交付页停留在“京哈高速改造工程 → 基本信息 → 编辑项目基本信息”，不再把匿名登录页交给用户当 OA 成品页。
- 直接对照参考 `BasicInfoTab.tsx` 实现宽 760px 的右侧编辑抽屉，字段与顺序保持一致：项目名称、项目编号、承接日期、资质单位、管理费率、项目概况、项目类型、经营负责人和生产负责人；项目包会把编号规则改为“选填”，普通项目保持必填。日期、人员、项目类型和概况字数均从当前项目上下文带入。
- 本地单 Workbench 进程实际验证普通项目打开／预填／保存校验提示，以及项目包编号选填、待设置生产负责人和类型默认值；随后停止本地进程。串行 `go test -p 1`、`go vet`、嵌入 JavaScript 语法和 `git diff --check` 全部通过。
- 公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.55@sha256:915624572ae03063a2f9c101f445960bccff8517069f42c394333275c0992cb4`，AMD64 manifest 为 `sha256:d68745a3172ced5f99e90c8bc76f9afc0e58705464495f290bb1a5624a6b1db4`。只重建主栈 Workbench；最终 17/17 healthy，其他 16 个容器 ID 与发布前逐项一致。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui55-project-profile-drawer.env`。
- D1 真人浏览器实际确认项目列表、京哈详情、编辑抽屉和全部预填值；表单通过后仍明确说明正式保存必须经 Project Workspace 版本化 API 和 IAM，不在只读演示中伪造业务写入。因此本批关闭编辑交互的可见缺口，不把 UI 复刻冒充真实命令或完整 OA 验收，整体结论仍为 `insufficient-evidence`。

### 第十二批项目层级高风险操作（本地与 D1 真人点击通过）

- 对照参考 `ProjectDetail.tsx` 补齐普通项目“项目包调整（试用）／删除项目”、项目包“添加子项目／删除项目包及全部子项目”和子项目“停用子项目”。转换流程包含两种模式、安全预检、保留业务事实清单和精确确认文字；删除流程按项目／项目包显示不同风险；停用明确保留生产、收付款和文件事实。
- 本地浏览器发现转换模式的网格样式覆盖 `hidden`，导致两组字段同时显示；增加显式隐藏规则后复验为“加入已有项目包”只显示目标项目包，“转换为项目包”只显示新包字段。普通项目、项目包、子项目三类保护流程均实际点击，console 0 error。
- 所有按钮仍只验证参考 UI/UX、必填校验和保护流程；演示不创建、调整、删除或停用任何 Project Workspace 事实，真实写入必须经过 IAM、事务、幂等和审计。
- 公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.56@sha256:17cab9a49dc7b26e18a2d28334ea2f415dd24f09b91802c63e976d3389eedb10`，AMD64 manifest 为 `sha256:c1a9ca083a13823800d5952a365fd87c174e5b838aa10b42de558530e9a84c79`。只重建主栈 Workbench；最终 17/17 healthy，其他 16 个容器 ID 逐项不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui56-project-actions.env`。
- D1 真人浏览器完成普通项目模式切换、项目包添加子项目及删除保护、S207 子项目停用弹窗复验，console 0 error。当前可见页保留在 S207 子项目停用确认界面；总体结论继续为 `insufficient-evidence`，因为真实写命令与完整连续 OA 业务验收仍未通过。

### 第十三批经营合同双链与维护抽屉（本地与 D1 真人点击通过）

- 纠正“经营信息 → 合同信息”仍是一张简化表格、无法代表参考 OA `ContractAndAmendments`／`ContractForm` 的缺口。页面现按参考结构分为“甲方合同”和“京宏合同”两条链，合同卡片显示编号、日期、结算金额、原合同金额、三类费用以及合同／补充／定案文件状态。
- 合同详情与新增使用 780px 右侧抽屉，保留参考页面的结算概览、同步开关、原始合同区域、费用自动汇总、合同文件、补充区域和定案表区域；新增基础合同完成前，后两段保持参考式遮罩和解释文案。已有甲方合同可看到一份补充协议和定案表，京宏合同保持自己的金额与文件状态。
- 本地单 Workbench 进程实际验证：`JH-JF-2026-018` 预填值和 `¥8,920,000.00` 结算金额正确；设计费变更后原合同与同步结算金额联动；保存、文件选择和删除仅显示只读验证结果；新增京宏合同保持空编号、隐藏删除入口、锁定补充／定案，填入必填字段后校验通过。浏览器 console 0 error／warning，验收后本地进程已停止。
- `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...`、嵌入 JavaScript 语法和变更空白检查全部通过。
- 公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.57@sha256:13cd4891daf757c20777049ef821e3945891f5e85406b329a1af786299a94498`，AMD64 manifest 为 `sha256:f06b472dbaedd600728c407be568d9a4c0009965590b5dcf6d41353074f25e4d`。只重建主栈 Workbench；最终 17/17 healthy，其他 16 个容器 ID 逐项不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui57-contract-drawer.env`。
- D1 真人浏览器从项目列表进入京哈项目、经营信息、合同信息并打开甲方合同抽屉，编号、金额、费用、补充和定案信息正确，console 0 error／warning。可见页停留在该真实 D1 合同抽屉，不再是登录页。该批证明参考合同 UI/UX 和保护交互进入 D1，不证明 Contract Ledger 写命令或完整 OA 连续旅程已经通过；总体结论仍为 `insufficient-evidence`。

### 第十四批招投标文件工作面（本地与 D1 真人点击通过）

- 纠正“经营信息 → 招投标信息”只有一条“不适用”空状态、与参考 OA `BiddingMethodControl`／`BiddingFileList` 明显不符的问题。页面现在保留参考结构：项目承接方式、三类材料要求、已处理计数、当前状态、最近更新、下一步，以及招标文件／投标文件／中标通知书三个独立文件槽位。
- 三个文件槽位使用参考 OA 的蓝／橙／绿业务色、文件名／大小／归档日期、继续上传、预览、下载和删除操作。文件预览使用右侧抽屉，展示 Evidence 版本与受控访问说明；删除使用文件位与文件名确认框。承接方式、上传、下载和删除均只验证交互并明确不写业务事实，正式命令仍属于 Business Engagement、Evidence Service 与 IAM。
- 本地仅运行一个 Workbench 进程，实际完成承接方式切换再恢复、招标文件预览、继续上传提示、下载提示、删除确认与保护提示；浏览器 console 0 error／warning。串行 `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...` 与 `git diff --check` 通过，验收后本地进程与临时标签页已停止／关闭。
- 公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.58@sha256:61c140994ca151d653fd84e1bca7f26556f8c5bcbac4c64433683b4c54fc9a17`，AMD64 manifest 为 `sha256:e6d1c6ef394a8a8f8b5e2adac9bf9b6206221d78433a5f63da4244cc7ba02e73`。只重建主栈 Workbench；最终 17/17 healthy，其他 16 个容器 ID 逐项不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui58-bidding-files.env`。
- D1 真人浏览器重复项目列表 → 京哈项目 → 经营信息 → 招投标信息，并打开 `京哈高速改造工程招标文件.pdf` 预览；文件类型、5.8 MB、2026-02-12 与 Evidence v1 均正确，console 0 error／warning。可见页保留在该真实 D1 预览抽屉。本批证明参考招投标 UI/UX 与保护交互进入 D1，不证明三类文件是来自真实投影，也不改变完整 OA 的 `insufficient-evidence` 结论。

### 第十五批支付／开票双链与维护抽屉（本地与 D1 真人点击通过）

- 纠正“经营信息 → 支付信息／开票信息”此前只有三张金额卡、无法代表参考 OA `BusinessTransactionsPanel`、`PaymentRecordList`、`InvoiceRecordList`、`ClientPaymentForm` 和 `OurInvoiceForm` 的缺口。支付与开票现在都按“甲方 → 资质单位”和“资质单位 → 京宏”分组，记录行保留费用类型、日期、金额、往来对象、关联发票／支付和右侧详情入口。
- 支付／开票共用 640px 参考式维护抽屉。支付详情带入业务链、往来对象、支付时间、费用类型、金额、关联发票和备注；开票详情切换为开票时间／金额、关联支付记录，并增加由 Evidence Service 承担的发票文件区域。新增、更新、文件选择和删除均只验证 UI、字段校验和保护路径：资金事实仍属于 Money Flow Ledger，开票事实属于 Billing，文件事实属于 Evidence Service，所有正式写入继续经过 IAM。
- 本地只运行一个低优先级 Workbench 进程，实际完成项目列表 → 京哈项目 → 经营信息 → 支付信息 → 设计费详情 → 更新保护，再切换开票信息 → 设计费发票详情；字段与关联记录正确，本地浏览器 console 0 error／warning。串行 `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...`、嵌入 JavaScript 语法和 `git diff --check` 通过，验收后本地进程与临时标签页已停止／关闭。
- 公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.59@sha256:3ef9575b25a694ac79e0aa25ae8a714e6344f545ef06a79eec9c9f15e446cf17`，AMD64 manifest 为 `sha256:922118ddd2b914c2bff6911b5820f28f1bf13f35ac918f0f840c3987b9e5529f`。只重建主栈 Workbench；最终 17/17 healthy，其他 16 个容器 ID 逐项不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui59-payments-invoices.env`。
- D1 Nginx 入口的页面、脚本和 CSS 标记探测通过，真人浏览器重复进入支付详情和开票详情，浏览器 console 0 error／warning。当前用户可见页保留在“京哈高速改造工程 → 经营信息 → 开票信息 → 开票记录详情”，不是登录页。本批证明参考支付／开票 UI/UX 与边界提示已经进入 D1，不证明静态记录来自真实投影，也不改变完整 OA 的 `insufficient-evidence` 结论。

### 第十六批经营成本与奖金分配工作面（本地与 D1 真人点击通过）

- 纠正“经营成本／奖金分配”仍只有三张摘要卡的问题。经营成本现在直接采用参考 OA `ProductionCostList`／`ProductionCostForm` 的统计条、费用色记录、金额／日期／说明／发票摘要和 640px 维护抽屉；既有记录、新增、发票选择／预览／下载与删除保护均可到达。
- 奖金页直接采用参考 `BonusPoolEditor`、`BonusAllocation`、`BonusAllocationStatistics` 和 `BonusAllocationList` 的奖池、比例、发布时间、统计卡、人员分配行和详情表单。按比例／固定金额可切换，比例池和最终奖金会联动；已发布事实只允许在界面中验证“撤回／取代并保留历史”的保护语义。
- 工作台继续不拥有上述业务事实：经营成本属于 Cost & Outsourcing，成本发票属于 Evidence Service；奖池、策略输入快照、人员分配与发布／取代／撤回历史属于 Bonus Distribution，已发布结果只能向 Money Flow 提交版本化建议。静态记录均标明为只读仿制证据，不写共享表。
- 本地只运行一个 Workbench 进程，实际完成成本详情、表单校验、文件动作、奖金详情、按比例改固定金额、金额联动与删除保护；浏览器 console 0 error／warning。串行 `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...`、嵌入 JavaScript `node --check` 和 `git diff --check` 通过，测试后本地进程与临时标签页已关闭。
- 公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.60@sha256:79f821b9ca614fe7a484ae6560b70750ca5e45b312836680982ee5883c44c69a`，AMD64 manifest 为 `sha256:5d3a77343c5dea5f38f64adc0b1d7808c2411105d5cf7571ff7bb7a754c57215`。只重建主栈 Workbench；最终 17/17 healthy，其他 16 个容器 ID 逐项不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui60-business-cost-bonus.env`。
- D1 Nginx 入口页面／CSS／JavaScript 标记与脚本语法检查通过；真人浏览器复验成本详情和经营奖金分配详情，字段与计算正确，console 0 error／warning。当前用户可见页保留在“京哈高速改造工程 → 经营信息 → 奖金分配 → 经营奖金分配详情”。本批只关闭这两个参考页面的 UI/UX 深度缺口，不证明静态记录来自真实版本化投影或写命令，完整 OA 结论继续为 `insufficient-evidence`。

### 第十七批生产成本与生产奖金工作面（本地与 D1 真人点击通过）

- 用户再次指出登录页不是“仿 OA”本体；验收入口继续固定为 `http://127.0.0.1:18090/workbench/?preview=oa`，并直接保留在参考 OA 的项目工作台，不再以匿名登录页或 IAM 表单作为 OA 成品展示。登录页仍只是 Nginx → Gateway → IAM 的正式入口边界。
- “生产信息 → 生产成本”由摘要占位补为参考 `ProductionCostList`／`ProductionCostForm` 工作面：显示 `¥775,000.00 / 12 条`、费用色记录、日期、金额、说明、发票摘要，以及新增／详情／文件／删除保护。方案评审专家费详情准确带入 `2026-09-29`、`¥240,000.00` 和 `施工图方案评审专家费.pdf`。
- “生产信息 → 奖金分配”补齐参考 `BonusPoolEditor` 与人员分配工作面：生产奖金基数 `¥8,520,000.00`、奖金池 `¥400,000.00`、v2 已发布、6 次／6 人；周宁按 25% 得到 `¥100,000.00`，其余比例和固定金额分配合计与奖金池一致。经营／生产两个作用域的详情与删除弹窗改为就近查找，避免同页重复组件互相串线。
- 正式边界没有因 UI 复刻改变：生产成本仍属于 Cost & Outsourcing，发票版本属于 Evidence Service；人员来自 People & Groups 版本化投影，奖池、输入快照、人员分配及发布历史属于 Bonus Distribution，结果只能向 Money Flow 提交版本化建议。预览写动作仍只验证字段和保护流程，不写共享表或制造业务事实。
- 本地低负载单进程实际打开成本列表／详情与奖金列表／详情；串行 `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...`、JavaScript 语法和 `git diff --check` 通过。本地临时服务与标签页已停止／关闭。
- 公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.61@sha256:df86eae492c38ac1c6c20d43b0662d4ae843d77939f7534f0f4445b6a3955b0f`，AMD64 manifest 为 `sha256:ff56ece5c7f63f3151958130688e61eb0386182f417798aee6361118a8390b67`。仅重建主栈 Workbench，新容器 ID 为 `4298d5cf20683c59386fe67610aa8696864250c2b87de093614daead8922e8c8`；最终 17/17 healthy，其他 16 个容器 ID 逐项不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui61-production-cost-bonus.env`。
- D1 Nginx 入口页面与外置 JavaScript 标记、脚本语法检查通过；真人浏览器实际完成项目列表 → 京哈项目 → 生产信息 → 生产成本 → 成本详情，再进入奖金分配，页面字段与金额正确，console 0 error／warning。当前用户可见页保留在生产奖金分配工作面。本批关闭生产成本／奖金的 UI/UX 深度缺口，不证明静态记录已经接入真实投影／写命令，完整 OA 结论继续为 `insufficient-evidence`。

### 第十八批生产人员与批复审计工作面（本地与 D1 真人点击通过）

- 再次纠正“匿名登录页就是仿 OA 网站”的验收误差：登录只承担身份入口，当前可见交付页固定为 `http://127.0.0.1:18090/workbench/?preview=oa` 的项目业务工作台。本批最终停留在“京哈高速改造工程 → 生产信息 → 人员配置”，不把登录卡片当成 OA 完整体。
- 对照参考 `ProductionPersonnelManager.tsx`、`DisciplinePersonnelModal.tsx` 与 `ReviewerModal.tsx`，把生产人员从姓名摘要补为关键角色、三个专业、设计／参与／复核／审核分工、添加／编辑抽屉和删除确认。正式人员与组织事实仍属于 People & Groups，项目只持有项目角色快照。
- 对照参考 `ApprovalAuditView.tsx` 与 `ApprovalAuditModal.tsx`，把批复／审计从金额卡补为双栏报告、文件操作、三类费用、总额、编辑抽屉和引用上游金额联动。批复引用甲方当前有效合同金额后总额由 `¥8,200,000.00` 更新为 `¥8,600,000.00`；正式结果／revision 与 Evidence 文件分别归 Approval & Audit Results 和 Evidence Service。
- 本地低资源单进程和公司 D1 真人浏览器都实际点击了人员编辑、批复编辑及金额引用；两处 console 均为 0 error／warning。`go test`、`go vet`、外置 JavaScript 语法和变更空白检查通过。
- 公司 D1 接受 `unified-workbench:v0.1.0-test.62@sha256:e4dff3b94004a7d25c56b53be5b8e5bdcef84a06e4e4a5ad34beed85e4bc6d0a`，AMD64 manifest 为 `sha256:465b795769b3c107fde2853a66473a4238a58661beaa2fd3e052fcba4f152f3b`；只重建 Workbench，主栈 17/17 healthy，其他 16 个容器 ID 不变。新容器 ID 为 `1b8e9cdd28a1d2741347ac962cdc0c7223924d127ad6f3b83db5e8a8dc13cdb4`，回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui62-production-people-audit.env`。
- 本批继续关闭浅层 UI/UX 缺口，但演示表单没有伪造真实写入；阶段结论仍是 `insufficient-evidence`，因为完整 OA 的真实版本化投影／命令和正式 IAM 连续旅程仍未全部验收。

### 第十九批生产阶段文件工作面（本地与 D1 真人点击通过）

- 直接对照参考 OA 的 `StageFilePanel`、`SchemeStageEditModal`、`PreliminaryStageEditModal`、`ConstructionStageEditModal`、`AuxiliaryFilePanel`、`ChangeFileUpload`、`CompletionFileUpload` 与 `ConsultationReportPanel`，把方案、初设、施工、变更竣工、咨询五个浅层占位页改为参考业务工作面。
- 方案／初设／施工现包含阶段状态、文件进度、最近维护、主文件、校审单、阶段评分和 800px 编辑抽屉；变更与竣工各自具有文件统计、列表及上传／替换／删除入口；咨询报告支持主报告、附件清单与追加入口。CAD 文件只提供下载语义，符合参考 OA 行为。
- 本地只运行一个 Workbench 进程，实际点击方案编辑、变更上传抽屉、咨询预览保护；表单通过后只显示边界说明，不伪造上传或正式写入。`go test`、`go vet`、JavaScript `node --check`、`git diff --check` 和浏览器 console 0 warning／error 均通过。
- 公司 D1 接受 `unified-workbench:v0.1.0-test.63@sha256:3bca615051c3728cfc0b6965b78882ffd9e718b7cd4d0e3bac3fb6cfd980e8a2`，AMD64 manifest 为 `sha256:0bc90595b752003fb95a25d42f65744fa10f27a128336a690e9715ae48b3c744`。只重建 Workbench，新容器 ID 为 `b730db28e09b2b37302b64d05f16a83a33ef63652def57f68c5f657a36cd876e`；主栈 17/17 healthy，其他 16 个容器 ID 不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui63-production-stages.env`。
- D1 可见页已停在“京哈高速改造工程 → 生产信息 → 方案阶段”，不是登录页；真人浏览器确认深层页面和文件操作提示可见且 console 无异常。静态验证数据尚未全部绑定 Production Operations／Evidence Service 的真实版本化投影与命令，因此完整 OA 结论仍为 `insufficient-evidence`。

### 第二十批完整使用指南与交付入口纠偏（本地与 D1 真人点击通过）

- 再次明确交付口径：登录页只是 Nginx → Gateway → IAM 的身份入口，不是“仿 OA 网站”。用户可见交付页继续固定为 `http://127.0.0.1:18090/workbench/?preview=oa` 的参考 OA 业务工作台。
- 对照参考 OA `UsageGuide.tsx`／`UsageGuide.css`，将原先只有 3 条 FAQ 的摘要页替换为完整操作手册：保留系统结构、基础流程、经营模块、生产模块、资料管理、人员管理和常见问题 7 项目录、5 张模块卡、6 步完整流程及 26 个可展开问题；CSS 继续使用参考 Ant Design class 结构，服务端适配补齐展开状态和箭头旋转。
- 本地仅运行一个 Workbench 进程，浏览器确认 8 个页面结构区、26 个展开项；“如何新建项目？”展开后显示完整步骤，console 0 warning／error。`GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...`、JavaScript `node --check` 与 `git diff --check -- frontend/oms-workbench/internal/server` 均通过，验收后本地进程和临时页已关闭。
- 公司 D1 接受 `unified-workbench:v0.1.0-test.64@sha256:8fb118d6c53dd045e82fa805d5500b9efaf26f5a1ac75ad4bd9af06deedc8f0f`，AMD64 manifest 为 `sha256:59de94c953d6f36704e0b38d7b8cc4dfca3a47c47ca91bbbeb257512c7c866cb`。只重建主栈 Workbench，新容器 ID 为 `feb5cc370d95cde285ee2a5a053fa4c0af0838d9fca7f92a4c2a44c5db26d25f`；主栈 17/17 healthy，其他 16 个容器 ID 不变。恢复演练栈未触碰，回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui64-full-guide.env`。
- D1 容器内和 Nginx 入口标记探测均通过；真人浏览器打开完整使用指南并展开“如何新建项目？”，答案可见且 console 0 warning／error。当前用户可见标签页保留在该操作手册，而不是登录页。本批关闭指南深度与交付入口认知缺口，不证明所有静态演示数据已绑定真实投影／命令，完整 OA 最终结论仍为 `insufficient-evidence`。

### 第二十一批公司／项目收入与完整合同详情可达性（本地与 D1 真人点击通过）

- 路由登记不再被当作“页面可用”的证据：审计发现 `company-revenue`、`project-revenue` 和 `contract-detail` 虽已登记，但缺少用户可点击的完整入口。其中项目收入仍是简化摘要，合同完整详情只能从脚本内部打开。
- 公司统计现增加“进入公司收入管理”，项目收入按参考 OA 补齐“管理费比例配置”、当前 15.00% 自定义值、公司默认／项目自定义切换、四项财务统计和设计／勘察／咨询三行费用表。切换公司默认时比例字段正确隐藏；提交只验证表单，不写 Project Workspace、Money Flow Ledger 或 IAM 事实。
- 项目经营信息 → 合同信息 → 合同抽屉现增加“查看完整详情”，可进入完整合同页并看到合同金额、三类费用、补充协议、合同文件与上传页签；跳转时抽屉关闭，返回路径可用。正式合同、文件和授权边界仍分别属于 Contract Ledger、Evidence Service 与 IAM。
- 本地只运行一个 Workbench 进程，实际完成公司统计 → 公司收入 → 项目收入 → 管理费配置，以及项目列表 → 京哈项目 → 经营信息 → 合同信息 → 合同抽屉 → 完整详情 → 上传文件页签；`GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...` 和 `git diff --check -- frontend/oms-workbench/internal/server` 通过，浏览器 console 0 warning／error，随后停止本地进程并关闭临时页。
- 公司 D1 接受 `unified-workbench:v0.1.0-test.65@sha256:d5c68d79e1c579a78b8f8c1f3ea05334722a329ead1f3e69fe963bafb60badee`，AMD64 manifest 为 `sha256:62539efe05e99ee32db240fc99530cf36e2633468df95bf257502642027bed8d`。只重建主栈 Workbench，新容器 ID 为 `f0146235c9d5e06a9479732259d2a982ffd8741d4ca1764ee2b9a71463307a10`；主栈 17/17 healthy，其他 16 个容器 ID 不变，恢复演练栈未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui65-revenue-contract-route.env`。
- D1 Nginx 入口命中三项新页面标记，真人浏览器重复上述两条链路，管理费切换、三行费用表、补充协议和文件页签均可见，console 0 warning／error。用户可见标签页保留在“项目收入信息”，不是登录页。本批证明深层 UI/UX 与导航可达，不证明只读演示已经替换为真实版本化投影／写命令；完整 OA 最终结论仍为 `insufficient-evidence`。

### 第二十二批生产对外委托工作面（本地与 D1 真人点击通过）

- 完成度审计发现“生产信息 → 对外委托”仍是两行简化表格，与参考 OA `ExternalCommissionList.tsx`、`ExternalCommissionSummary.tsx`、`ExternalCommissionForm.tsx` 和 `ExternalCommissionDeleteConfirm.tsx` 存在明显深度差距。
- 页面现按参考结构显示委托总数、总金额、平均评分、受托方、联系人、委托内容、金额、合同文件、支付次数与已付金额。新增／编辑抽屉包含完整字段、动态支付记录、合同预览、必填校验和删除保护。
- 本地只启动一个 Workbench 进程，真人浏览器实际验证两条委托、编辑回填、`2 → 3 → 2` 条支付增删、表单校验、`京哈高速桥梁检测委托合同.pdf` 预览和删除保护。同时发现并修复新增状态下“预览现有文件”被样式覆盖 `hidden` 后误显示的问题；console 0 warning／error。
- 事实边界没有因 UI 仿制改变：委托承接、金额和交付状态属于 Cost & Outsourcing，合同文件属于 Evidence Service，支付事实属于 Money Flow Ledger，正式操作继续通过 IAM。Workbench 只组合版本化投影，只读预览不伪造任何写入。
- 公司 D1 接受 `unified-workbench:v0.1.0-test.66@sha256:9b4c10ddc7b3ad754181d5e9123e92a70d1b7025aab4847f7a3c293c761829ce`，AMD64 manifest 为 `sha256:213fb32c5918a1900b1e7c0f4e4697fbace591c14335f1234dab6e541ea4e8f3`。只重建主栈 Workbench，新容器 ID 为 `5fb1acfbefd983436eeba7c5eeb0b407dc7a849ae7c12cece429c505d873e0e0`；主栈 17/17 healthy，其他 16 个容器 ID 不变，`omsbizfoundry-restore20-*` 恢复演练栈未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui66-external-commission.env`。
- D1 Nginx 入口标记探测通过，真人浏览器重复编辑、支付增删、保存校验、合同预览和删除保护，console 0 warning／error。用户可见页保留在“京哈高速改造工程 → 生产信息 → 对外委托”，而不是登录页。本批关闭该页 UI/UX 深度缺口，但真实版本化投影／写命令与完整 OA 最终验收仍待证，结论继续为 `insufficient-evidence`。

### 第二十三批经营参与人工作面（本地与 D1 真人点击通过）

- 继续按“可见业务页面而非登录页”验收，并纠正“经营信息 → 经营参与人”把经营负责人和参与人混成头像摘要、只有一个泛化管理按钮的问题。对照参考 OA `BusinessPersonnelList.tsx`／CSS 后，经营负责人明确留在项目基本信息，当前页面只维护经营参与人。
- 页面现在逐行展示姓名、账号、部门、项目角色和公司角色，已有王可、赵清、陈雪三条样本；添加抽屉只列可选人员，删除确认显示人员和公司角色。正式人员／公司角色与候选人属于 People & Groups，项目角色快照属于 Project Workspace，正式配置继续经过 IAM；只读仿制不伪造新增或删除。
- 本地仅运行一个 Workbench 进程，真人浏览器完成添加抽屉、候选人选择、必填校验、删除确认和保护提示，console 0 warning／error；`GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...` 和 `git diff --check` 通过，随后停止本地进程并关闭临时标签页。
- 公司 D1 接受 `unified-workbench:v0.1.0-test.67@sha256:15961d97403d6d867bab96e6b0e1edeb9f4dcf3577240f889052fcbb928ef8aa`，AMD64 manifest 为 `sha256:1b40cafec32d84bdca036183a3bae24f1cfe369f4188b13e39db1c1e73d175ab`。只重建主栈 Workbench，新容器 ID 为 `1f5ea4cba6108b20e73cd26faf023aa46799f9d81e39f8fe0d1b4a2d4513eca6`；主栈 17/17 healthy，其他 16 个主栈容器 ID 逐项不变，17 个 `omsbizfoundry-restore20-*` 恢复演练容器未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui67-business-personnel.env`。
- D1 真人浏览器实际完成项目列表 → 京哈项目 → 经营信息 → 经营参与人 → 添加周宁校验 → 王可删除保护，字段与边界提示正确，console 0 warning／error。用户可见页保留在该 OA 经营参与人工作面，而不是登录页。本批关闭该页的 UI/UX 深度缺口，但真实版本化投影／命令与完整 OA 最终连续验收仍待证，结论继续为 `insufficient-evidence`。

### 第二十四批甲方信息内联维护与 OA 默认登录页（本地与 D1 真人点击通过）

- 继续审计深层页后发现“经营信息 → 甲方信息”仍是三字段只读摘要。对照参考 OA `BusinessInfoTab.tsx` 的 `clientInfoContent`，补齐行内编辑、必填甲方名称、历史名称候选、联系人、电话格式、取消和保存校验。
- 界面和数据边界同时保留：Project Workspace 只保存当前项目的甲方名称与联系人快照；Party Directory 拥有主体档案、稳定 ID 与 revision；Workbench 不修改主体主数据，正式写入仍需 IAM 保护的版本化命令。
- 本地单 Workbench 进程实际完成打开、预填、修改为“北方高速建设集团有限公司／李经理／010-88886666”、保存校验和取消恢复，console 0 warning／error；验收后本地进程与临时标签页已停止／关闭。
- 公司 D1 接受 `unified-workbench:v0.1.0-test.68@sha256:55c6dbbe790aecd76139195cf3e56b3e8f55375d9c3dfb45a079ef260b1b98d9`，AMD64 manifest 为 `sha256:f9c8302131a4928beb560c327354d88c475755e86c715b99f88856ccf3c7459f`。只重建主栈 Workbench，新容器 ID 为 `ac2ec8ad3897231f2cc63d8f2f728d9d3db28a02d2f963349f0e65ecf31628f9`；主栈 17/17 healthy，其他 16 个容器 ID 逐项不变，17 个 `omsbizfoundry-restore20-*` 恢复演练容器未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui68-project-client.env`。
- D1 真人浏览器重复上述甲方信息修改与校验，console 0 warning／error。另外针对“通用 IAM 登录壳不是仿 OA”的反馈，通过 D1 未登录入口实际确认：默认页直接使用参考 `Login.tsx`／`Login.css` 的公司全称、水印背景、手机号和验证码交互；真实 IAM 账号密码表单仅作当前无短信环境的显式备用入口，不再冒充 OA 默认登录页。手机格式启用按钮、IAM 切换和返回 OA 页均实际通过。
- 本批证明甲方页的参考 UI/UX 和 OA 默认登录面已进入 D1，不证明甲方静态样本已绑定真实投影／命令，也不把登录页当成 OA 业务网站本体。完整 OA 最终结论仍为 `insufficient-evidence`。

### 第二十五批项目类型驱动的生产阶段与勘察试验页（本地与 D1 真人点击通过）

- 对照参考 OA `ProductionInfo.tsx`、`SurveyTestStagePanel.tsx` 与项目类型约束，纠正所有项目共用同一组生产页签的问题。项目详情现在保留设计／咨询／勘察类型；设计页签只对设计项目显示，试验阶段只对勘察项目显示，咨询报告按咨询、勘察或测绘类型显示。
- “临河新区市政道路勘察设计”现显示“设计／勘察”，并具有独立试验阶段工作面：至少一份文件校验、多选待保存计数、既有试验报告与受控下载提示均可操作；“京哈高速改造工程”显示“设计／咨询”，试验阶段正确隐藏。
- Workbench 继续只验证参考 UI/UX，不伪造文件或生产事实：Production Operations 保存阶段状态和业务含义，Evidence Service 保存 File ID／Version 与受控访问，正式命令由 IAM 授权。
- 本地只启动一个 Workbench 进程，实际验证无文件保护、选择 `README.md` 后 `1 待保存`、表单通过提示、既有报告下载保护和跨项目页签切换；浏览器 console 0 warning／error。`GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...`、JavaScript `node --check` 与 `git diff --check` 均通过，随后停止本地进程并关闭临时页。
- 公司 D1 接受 `unified-workbench:v0.1.0-test.69@sha256:1608a506c95529599e4303eb1e8c4f84e505bbb2d980492348cb92de4fcdb69a`，AMD64 manifest 为 `sha256:d9f50679ce581b7a6502faa6ba792f4260d5efaccdc345ffd3b3455961754d19`。只重建主栈 Workbench，新容器 ID 为 `d0deeddd3185`；主栈 17/17 healthy，其他 16 个容器 ID 不变，17 个 `omsbizfoundry-restore20-*` 恢复演练容器未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui69-survey-test.env`。
- D1 Nginx 入口标记探测和真人浏览器重复验证通过，console 0 warning／error；用户可见页保留在“临河新区市政道路勘察设计 → 生产信息 → 试验阶段”。本批关闭项目类型／试验页深度缺口，但审计同时发现临河项目顶部生产概览仍沿用京哈演示数字，后续必须继续消除跨项目上下文残留；完整 OA 最终结论仍为 `insufficient-evidence`。

### 第二十六批生产概览项目上下文一致性（本地与 D1 真人点击通过）

- 紧接第二十五批处理已发现的跨项目残留。生产概览不再把京哈的“施工图设计／3/5／周宁／18 份文件”固定显示给所有项目，而是随当前项目更新阶段、进度、文件数、负责人、阶段资料行和首要维护入口。
- 临河项目现显示“施工配合、5/5、16 份文件、陈工”，阶段资料含方案、初设、施工图、咨询、试验、变更与竣工，首要入口直达试验阶段；S207 子项目显示“待启动、0/5、0 份文件、待设置”，五个设计阶段均为未设置且试验阶段隐藏。
- 本地低资源单 Workbench 实际切换临河与 S207，验证概览文本、阶段资料、快捷入口和试验页可见性；console 0 warning／error。`go test`、`go vet`、JavaScript `node --check` 和 `git diff --check` 均通过。
- 公司 D1 接受 `unified-workbench:v0.1.0-test.70@sha256:65b3df424e60089e45fb7c445010ef20aa996741e1bb64fd29c2ce1c88093145`，AMD64 manifest 为 `sha256:3b1c078441fc4617979e14559c47e99eb85fc8d690e70a6942fd0ddf9dbea79f`。只重建 Workbench，新容器 ID 为 `645d117f751c`；主栈 17/17 healthy，其他 16 个容器 ID 不变，恢复演练栈仍为 17 个且未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui70-contextual-production.env`。
- D1 真人浏览器重复验证临河上下文及快捷入口，console 0 warning／error；可见页继续保留在临河试验阶段。该修复消除了当前已知的生产概览串项目问题，但不把界面样本当作真实投影／命令证据，完整 OA 最终结论继续为 `insufficient-evidence`。

### 第二十七批生产快捷入口与按需维护折叠（本地与 D1 真人点击通过）

- 对照参考 OA `ProductionInfo.tsx`，补回生产概览中的阶段快捷入口，并按当前项目类型决定方案、初设、施工图、变更竣工、试验和咨询入口是否出现；“按需维护”恢复为默认折叠结构，不再把低频动作永久铺满页面。
- 本地单 Workbench 实际验证临河项目显示 6 个阶段入口、S207 只显示设计链 4 个入口；试验快捷入口可直达试验页，“按需维护”可以展开和收起，浏览器 console 0 warning／error。
- 公司 D1 中间接受镜像为 `unified-workbench:v0.1.0-test.71@sha256:5732a1988d9b23242762f9d0bff187ef42f3218315d6593f76c63173d1533d79`，AMD64 manifest 为 `sha256:b4eb5b6b7b48302f1c1c3eeb0dd3eb5026fb82d5543ee7cde2eace43fbd9a628`；只重建主栈 Workbench，17/17 healthy，其他 16 个容器 ID 不变。该版本随后被第二十八批 v72 取代；回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui71-production-navigation.env`。

### 第二十八批源码可达组件审计、流程助手与产品反馈（本地与 D1 真人点击通过）

- 重新从参考前端入口按真实 import 图审计，而不是只看路由名：参考项目共 123 个 `.tsx` 组件，其中 79 个可从 14 个页面真实到达。审计发现两个仍未进入当前业务界面的可达组件：侧栏 `FeedbackEntry` 和项目详情 `ProjectWorkflowAssistant`。
- 侧栏现恢复参考式“产品反馈”入口和“提交反馈／我的反馈”菜单；项目详情在经营信息与生产信息右侧恢复“进度与设置”助手，显示当前经营／生产进度、项目类型相关的无／不适用设置，并支持折叠／展开。基本信息页不显示助手，跨项目切换会重置上下文。
- 这些开关只验证参考 UI 与边界提示，不伪造正式设置：正式状态仍属于 Project Workspace／Production Operations，相应授权必须经过 IAM；反馈 SDK／权限服务尚未接入时明确显示边界说明。
- 本地低资源单 Workbench 实际验证反馈菜单、经营／生产助手、项目类型差异、无／不适用切换和折叠／展开；`GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...`、嵌入 JavaScript 语法和变更空白检查全部通过，console 0 warning／error。
- 公司 D1 最终接受镜像为 `unified-workbench:v0.1.0-test.72@sha256:69b59db42fe2a678e74942786856cc20d1208cdb92bb20157d3a944a62004ea7`，AMD64 manifest 为 `sha256:4a9e6e79547c91da9e80658738d8b87e8b622982ec7759dad8aa7617a45466ae`。只重建主栈 Workbench，新容器 ID 为 `eda28550134f`；主栈 17/17 healthy，其他 16 个容器 ID 逐项不变，17 个恢复演练容器未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui72-workflow-feedback.env`。
- D1 真人浏览器从项目列表进入“临河新区市政道路勘察设计 → 生产信息”，实际确认右侧助手、8 个生产设置、产品反馈菜单、开关状态和折叠／展开；console 0 warning／error。用户可见标签页保留在该生产业务页，不是登录页。
- 到此，参考 OA 的 14 个可达页面和 79 个可达组件已经完成源级覆盖核对，当前已知的可见组件遗漏已关闭；因此“参考 OA 界面与保护性交互复刻”可以单独判定通过。该结论只针对 UI/UX，不把静态演示数据当成真实版本化投影／命令，也不取代完整 OA 业务连续旅程的 `insufficient-evidence` 结论。
