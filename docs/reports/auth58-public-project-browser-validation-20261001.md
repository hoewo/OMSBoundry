# Auth58 公网项目创建与架构透镜验证

日期：2026-10-01，首轮约 16:33～16:36，反馈回归约 18:12～18:16，负责人选择／立项及布局回归约 18:25～18:40（上海）。结论：**正式 Chrome 中的 IAM 登录、Workbench 新建、持久化复读、真实 Project 入口、缺口拒绝反馈、能力过期恢复，以及选双负责人完成立项通过；只关闭这些操作切片，不等于完整 OA 可用。**

问题 A：此前主要有 API 证据，用户不能确认页面按钮能提交。结果 B：从 `https://oatest.antuinfo.cn/workbench/` 使用原已部署 IAM 账号，直接点击并观察真实记录与失败反馈。首轮未更新主镜像；后续仅部署下述 Project 页面反馈修正。没有关闭鉴权或修改密码。

## 实际操作与结果

1. Chrome 正式 HTTPS 页面从 IAM 账号密码入口登录，进入项目工作台。拒绝浏览器保存密码；临时 0600 登录助手关闭并删除，密码未出现在 URL 或消息日志，剪贴板已用用户名覆盖。
2. 项目立项读取 11 条真实待立项记录。打开新建表单，填写稳定编号、名称和概况，点击“创建真实待立项项目”。提交期间按钮禁用并显示正在提交；完成后对话框关闭，列表显示 12 条。
3. 新增编号 `OA-VERIFY-20261001-BROWSER-1634`，名称 `OA 公网浏览器创建验证 20261001（验收夹具）`。架构透镜显示输入校验、Gateway 受保护入口、IAM Project 能力、Project 写入 `revision 1`。
4. 刷新整个页面，重新进入项目立项，仍能读到该记录。点击“进入真实 Project”，正式路径 `/project/?project_id=b6e26691-cfd6-4ef4-ba53-8d9f9a8c2cc2` 显示相同编号、概况、`draft / revision 1`。
5. 未设置经营／生产负责人时，点击“检查缺口并完成立项”返回“立项资料尚未补全”，没有变为 active。后续刷新权限并读取历史确认：该动作记录了 `project_initiation_pending / revision 2`，实际为 `pending_completion / revision 2`，不是零写入。初始错误提示后页面仍显示旧 revision 1，是待修复的反馈缺陷。

该草稿明确为验收夹具，保留用于后续复测；首轮没有删除、修改其他项目，也没有进行签约、收付款或完成立项。后续仅此夹具完成双负责人立项，见下文。

## 证据边界与遗留项

- 这是实际浏览器操作，不是把接口成功改写成页面通过。架构透镜是基于操作结果的边界解释，不是完整分布式 tracing；Nginx 的 TLS 由当前 HTTPS 入口另行佐证，不能把静态说明视为每一步的服务端审计。
- 首轮手填 UUID 的缺口已由下文负责人选择器关闭：默认从受保护 People 目录选择，手填仅为显式高级排查。大名册分页、其他业务选择器和全部 UX 仍未验收。
- 首轮发现的过期编辑态和拒绝后旧 revision 问题，已由下述真实浏览器反馈回归关闭；没有自动重放写命令。该结果不替代账号撤权、相邻角色或双 Tenant 矩阵。
- 当前透镜按 Gateway 可信的管理员标识显示；“仅发起人本人可见”的精确授权尚未验证，不能把“管理员可见”描述为已经满足仅本人要求。
- Workbench 项目管理列表、经营／成本等其他视图仍有明确标记的界面验证数据。本轮没有把这些控件逐一接入真实业务。
- Files OSS 的正式浏览器上传、取消、错误摘要、断网续传、权限拒绝，以及 Standard Web OSS 静态部署，均不由此项目创建证据替代。完整 OA 仍为 `insufficient-evidence`。

## Project 页面反馈修正与独立恢复（约 17:00 上海）

仅修改 Project 自有页面：立项缺口拒绝后重新读取目标 Project 的 revision／状态／`allowed_actions`，显示当前“待补全”和缺失负责人；短期能力跳转 HTML 视为未认证并收回编辑态；并发重复点击只发一次命令。重新读取失败则停止编辑，不自动重放写命令，不从管理员名称推断允许动作。没有修改领域流程、API 或数据库 migration。

- 嵌入页面 JavaScript 的 Node VM 回归通过：422 后复读 revision 2、读取失败关闭编辑、HTML 登录跳转、重复点击与用户切换 Project 边界。
- Project 普通 Go 全量测试和 vet 串行通过；这些不是全部 PostgreSQL 集成测试或全仓测试。
- 固定 ACR 镜像 `project-workspace:v0.1.0-test.43`，索引摘要 `sha256:b03d2cda182d420e6db6f1c937d6f66512136df49652ff055e3300a2a07140aa`，ECS 实际拉取 Linux/amd64、非 root 用户。
- 停止单个 Project 写入服务后取得冷备份，恢复到空库 `project_ui_auth58_20261001p43`。全部 8 张表、0 个序列的 canonical 摘要 `e9768f8f4c25805c23719ea8f8644924f7d4598ba75e5a8927e649dbd5c343cb` 与源库一致；更新后再次一致，没有运行 migration。
- 只重建 Project，其他 16 个主服务容器 ID 不变，主栈 17/17 healthy；Evidence 仍为 Local。权威环境保存实际 17 个镜像固定摘要，私有原配置／回退配置保留于备份目录，不输出密钥。
- 冷备份 `/var/backups/omsbizfoundry/project-ui-test43-20261001/project-workspace.dump`；离机副本 `deployments/aliyun/.runtime/auth58-evidence-oss-20261001/project-workspace-test43.dump`，57,060 B、0600，SHA-256 两端一致：`e767180a7f4c582c37a2ff9d0e548f3c661938f2b77c6850fb380afc52dba4f9`。

本节部署与恢复通过，后续真实浏览器反馈另见下一节，不能以 Node 回归或镜像 healthy 代替。当时人员 UUID 选择器尚待实施，现已补齐下文切片；仅本人透镜授权和其他业务页面仍是遗留项。

公网上另行执行只读 HTTPS 检查：由正式 IAM 登录和 Gateway 换取当前能力，读取 HTML、外部 `/project/assets/demo.js` 和原验收 Project；确认新反馈函数、重复提交保护、HTML 跳转 401 处理确实来自正式部署。脚本 SHA-256 `bea73a3871f20f026c6fb51ba0f4d46c4fa1369169c27c54287074889898a3f1`，Project 仍为 `pending_completion / revision 2`，8 表摘要不变；探针会话已单独注销。这是制品交付／API 复读，不是浏览器点击。

伪造客户端管理员、Subject、Tenant Header 和无签名 Bearer，公网 Project 入口返回 303、IAM session 401、目标 Project API 401；匿名工作台不渲染管理员透镜按钮。重建后的 Project 容器仍不能读取 ECS IMDS 角色路径，Project 事实不变。这仅关闭 Header 伪造负例，不等于相邻角色、撤权、完整双 Tenant 或“仅本人”透镜矩阵。

## 正式浏览器反馈回归

18:12，在原验收项目 `pending_completion / revision 2` 上单次点击“检查缺口并完成立项”。Chrome 显示“立项尚未完成，当前为待补全 / revision 3；缺少：经营负责人、生产负责人。请补全后再提交。”详情与列表均同步为 revision 3，没有假称 active。只读数据库后检确认恰好新增一个 command 和一条 `project_initiation_pending` 历史，原两条历史及全部其他 Project 事实不变。

随后应用能力自然到期，点击只读“读取历史”。页面明确提示能力到期并退出编辑状态，收回新建、责任调整及完成等入口；点击“刷新权限”后可再次读取全部三条历史。第二次数据库后检确认 revision 3 与八表事实保持不变，权限刷新没有重复提交写命令。期间没有更换主镜像、变更 IAM 授权或修改密码。

复核入口：`acceptance/evidence-oss-ecs/verify-project-browser-feedback.py` 的 `before`、`after`、`frozen` 三步。它仅比对指定夹具和其他事实，不发业务命令；云端私有前后快照保存在本次验收目录。本节是真实 Chrome 点击证据，范围仅限缺口反馈及自然过期恢复，不等于撤权矩阵、负责人完整立项或其他 OA 页面通过。

## 真实负责人选择、完成立项与布局修正

Project `test.46` 默认从 People & Groups 受保护列表加载 active 人员／编组，只保留稳定 ID、revision、名称和引用码。浏览器先通过 Gateway 换取已有 People 读取能力，Project 写命令仍由原服务端再次核对来源；没有共享数据库、按姓名伪造引用或新增 IAM Grant。人员／编组切换清空旧选择，加载、无候选及迟到响应不允许提交；目录失败不自动退回手填。显式高级手填和独立本地模板仍经过相同服务端验证。

18:25～18:28，在原 revision 3 夹具上真实加载 17 名 active 人员，分别选择两名现有 D1 合成人员为经营／生产负责人，再点击完成立项。两名来源均未关联 IAM Subject，不会因此给真实员工扩大项目访问权。实际状态依次为 revision 4、5，最终 `active / revision 6`；页面不再显示完成立项动作。另行读取 3 个 active 编组，类型切换确实清空选择；没有提交编组 Assignment。

`verify-project-picker-browser.py before/after/frozen` 只读后检均通过：恰好新增 3 个 command、两条 Assignment 和三条历史（两次 `assignment_started`、一次 `project_activated`）；原三条历史保持。其他 Project 事实及 People 全部 14 张表的 canonical 摘要不变。刷新、打开／取消弹窗等后续只读操作没有重放命令。该夹具当前 revision 6，不能再把早期 revision 2／3 专项脚本视为当前状态断言。

`test.47` 将技术能力标签折叠为可展开说明，修正新建按钮被挤成竖排。Chrome 已实际展开／收起两处动作列表，按钮横排完整，刷新后仍是同一 revision 6 和双负责人。回归发现空新建表单的“取消”触发必填校验，原因是按钮被当作 submit；`test.48` 把四类取消按钮改为非提交按钮，只关闭所属弹窗。正式 Chrome 空项目取消已通过；项目包、本地凭证和撤销确认目前仅有处理器回归，Mac 随后锁屏，不能描述为全部真人弹窗通过。

本地串行 Project 普通 Go 测试／vet、负责人选择器 13 项、立项反馈 5 项，以及四类取消处理器回归均通过；不是全部数据库集成或跨应用浏览器矩阵。三次仅 Project 的部署均先冷备份、空库恢复并核对全部 8 张表、0 序列，不执行 migration，其他 16 个主容器不变、17/17 healthy，Evidence 保持旧 Local。固定镜像与离机备份如下：

| 发布 | ACR 索引 SHA-256 | 冷备份 SHA-256 | 空库 canonical SHA-256 |
| --- | --- | --- | --- |
| test.46 | `70e0d10bb531412aef496678cd3ea5d25faf56bf107ce913417b2fbaf267b84d` | `3086c36bc154a9fa3036f5164b633fd75f50732fb4d01231990285e4eafb260b` | `a64564a5d7da63a167215886cb2e82ec9c9eebdfc1d4df47c52bd17fc9062966` |
| test.47 | `b2dc8f7fe97ff32743e4c6c59eec5d173e117021a2812d6c10a75059d5308198` | `620b8e2b966085d68615e5bfd4826c237abcb1d015adaa43fb96d0813c5e3ca0` | `c0b5cdb9c9caf6d3901e5af2c460b7f0b61aec0756d34497c66cadd6980edd46` |
| test.48（当前） | `d671251d6cc65e74f11f673d73f1cc57a9ae40f59a46599e9c75311c70dfdd6e` | `ba2e10a73babad5d12796d67c81024ed50cc133b5d7ae93fc2250a2a14be39d8` | `c0b5cdb9c9caf6d3901e5af2c460b7f0b61aec0756d34497c66cadd6980edd46` |

云端备份位于 `/var/backups/omsbizfoundry/project-ui-testNN-20261001/project-workspace.dump`，空目标库为 `project_ui_auth58_20261001pNN`；NN 分别为 46、47、48。离机文件位于 `deployments/aliyun/.runtime/auth58-evidence-oss-20261001/project-workspace-testNN.dump`，均为私有备份，不包含在源码发布输入中。三份离机 SHA-256 均与源端相同，权限为 0600，分别为 57,232／58,255／58,255 B。

## 本批改动归属

| 位置 | 解决的问题 | 没有承担的职责 |
| --- | --- | --- |
| Project 自有页面 | 来源选择、拒绝后显示目标确认的 revision 与可用动作、能力过期收回编辑、按钮布局与取消 | 不改变立项规则，不写 People／IAM 表 |
| Evidence 稳定组件 | 历史 Local 与新 OSS 可兼容读取、混合恢复、严格字节校验与 CNAME 传输 | 不拥有合同、成本或项目事实 |
| D1 部署运维 | 显式出口、元数据隔离、启动模板和声明／运行状态核对 | 不保存账号或签发业务身份 |
| 验收脚本 | 保留真实字节、恢复、权限及制品证据 | 不作为客户业务运行服务 |

这些包含公共组件缺陷修复和部署能力，并不都是连接器定制；也没有新增业务系统、共享数据库或万能平台。Standard Web／Customer Overlay 产品化及其长期升级效果仍未验收。
