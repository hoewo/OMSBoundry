# 原 OA 项目资料管理 F01／F02 原生闭环 v1

状态：2026-10-06 原 OA 源码审计完成，边界候选；实现与运行证据待补。  
上游基线：BF-111～BF-118、小系统边界演变账、各来源 `material-reference-v1`、Evidence File／Version／Access Grant。  

问题 A → 可验证结果 B：原 OA 在单体数据库直接连接 Project、File、用户和下载记录，目标组合的资料却分别由多个来源应用拥有业务含义、由 Evidence 拥有字节 → 用户仍按原页面看到“项目目录 → 展开资料 → 下载 → 查看下载人员”，但不复制来源事实或文件字节，不以 Workbench／Project 建立万能文件表，并能解释每次显示、授权、失败和恢复。

## 1. 原 OA 已确认动作

1. 进入资料管理先读取项目目录、更新时间和当前用户可下载的 `file_count`，不读取全部文件行和下载人员；
2. 项目无资料时不可展开；点击整行或“查看 N 份”才读取该项目的文件、更新时间和下载统计；
3. 每份资料显示名称、更新时间、下载次数；下载人员不超过 3 人时直接显示姓名，更多时打开“下载人员”弹窗；
4. 点击下载先检查 Project 和文件类别权限，再签发 15 分钟附件地址，然后写一条下载记录并刷新该项目；
5. 原实现的“下载次数／人员”实际统计的是**下载地址成功签发**，不证明浏览器已取得完整字节；目标 UI 和 API 必须使用“访问地址签发”或等价准确语义，不能误称完成下载；
6. 本页面没有上传、修改或删除入口，文件写入仍发生在各业务栏目。

## 2. 候选事实所有者

| 语义 | Owner | 本页读取 | 明确禁止 |
| --- | --- | --- | --- |
| Project 当前目录、访问上下文、Resource Link | Project Workspace | 可访问项目及其已连接来源资源 | 复制文件字节、来源业务状态或下载审计 |
| 文件业务含义、当前有效引用 | Contract／Production／Results／Cost／Billing／Business Engagement 等来源应用 | 版本化 `material-reference`，含 source、resource revision、relation、File ID／Version 和 display name | Evidence 猜业务类别；Project 改写来源引用 |
| File／Version、摘要、对象字节、短期访问地址 | Evidence Service | 精确 metadata 和 `access-grants` | Workbench 持有 OSS SDK／长期密钥；来源应用签发物理地址 |
| 地址签发审计 | Evidence Service | 已有 `file_version.access_granted` append-only audit：Actor、File／Version、时间、disposition、delivery mode、provider | 把签发事件冒充字节下载完成；覆盖或删除旧审计 |
| 账号与人员显示 | IAM／People & Groups | 可信 Actor Subject；需要时批量解析当前人员显示 | Evidence 或 Project 建人员主档 |
| 页面装配与短缓存 | Unified Workbench | 原目录、展开组、下载人员弹窗；按展开懒加载 | 持久化文件目录、统计事实或跨来源第二份真相 |

## 3. Project／Production／Contract 边界影响

- **Project**：继续只拥有 Project、访问上下文和 Resource Link。目录可以从 Link 判断有哪些来源聚合可展开，但文件项及其 revision 必须来自来源 `material-reference`；如果为了首屏 `file_count` 引入派生摘要，必须带来源 revision／digest／freshness，明确是可重建投影，不能成为文件事实。
- **Production**：只发布自己 Work Item 当前有效的阶段资料引用；资料页面不能调用 Production 写命令，也不能把项目资料下载变成 Production 状态变化。
- **Contract**：只发布合同、变更、结算等聚合当前有效的材料引用；资料页面下载不改变法律合同 revision，Evidence 地址签发审计也不写 Contract。

这正是下一轮边界压力：Project 依赖面可能因“跨来源目录”变宽，但不能因此成为文件聚合数据库；Production／Contract 的公共只读材料端口必须能够独立演进和失败。

## 4. 候选读取链

### 4.1 首屏目录

1. Workbench 读取当前用户可见 Project 目录和 `allowed_actions`；
2. 只读取 Project 自有 Resource Link／可重建摘要，不向每个 Project × 每个来源递归扇出；
3. 目录中的 0、无权、来源未读取和来源不可用必须是不同状态。没有可靠计数时先显示“展开读取”，不得把未知伪装成 0。

### 4.2 展开一个 Project

1. 读取该 Project 当前 Resource Links；
2. 对每个明确 Link 用正式 IAM 交换来源原生、精确资源 Scope 的 `material-reference.read`；
3. 合并时保留 `source_application`、`resource_type／id／revision`、`relation_key`、File ID／Version 与 digest，不按文件名去重；
4. 对每个可见文件按需读取 Evidence metadata；同 File／Version 可以缓存，但权限变化后必须重新授权；
5. 一个来源失败只标记该来源 unavailable／forbidden，其他来源仍显示；失败不能显示为空项目。

### 4.3 下载与人员

1. 用户点击下载时，Workbench 用该用户对 Evidence 的原生 File Scope 能力请求精确 File／Version access grant；
2. Evidence 最终授权、签发有界地址并追加 `file_version.access_granted`；返回的只是地址签发成功；
3. 浏览器直接读取 Evidence／OSS 数据面，WorkBench 不代理大文件；
4. 下载人员视图读取 Evidence 审计的聚合投影，显示“访问地址签发次数／最近签发”；人员名称缺失或已离职不能删除历史 Subject。

## 5. 首个实现切片

只做 F01／F02，不同时铺统计：

1. 复用原 `FileManagement.tsx`、CSS、项目表、展开行、sticky 项目上下文、下载按钮和人员弹窗；
2. 先接一个已有真实 Project，至少包含 Production、Results、Cost 三种来源和三份真实 Evidence；
3. 先验证 Project Links → 来源 material references → Evidence metadata／access grant；不新增全局 File Catalog 服务；
4. 若首屏精确 `file_count` 无法在不扇出的条件下提供，界面暂显“展开读取”，并把是否需要 Project 可重建 Link 摘要作为运行证据后的决策，不先建共享索引；
5. 不改变任何来源写模型，不增加全站上传／删除入口。

## 6. 验收门禁

从全新夹具实际运行至少覆盖：

1. 首屏目录零来源文件明细读取；展开一个 Project 才读取其已连接来源；
2. Production／Results／Cost 三来源真实文件都可见，来源和 relation 不丢；同名文件不误合并；
3. 无文件、无权、未读取、来源 403、来源 503 分开显示；一个来源失败不清空其他来源；
4. 下载只请求精确 File／Version，返回短期地址，真实字节一致；未授权／相邻 File／跨 Tenant 拒绝；
5. 每次成功签发只追加一条 Evidence access audit；回复丢失的重试语义不能重复计数，除非确实再次签发新 grant；
6. 页面明确称“地址签发”而非“字节下载完成”；人员聚合、最近时间和离职 Subject 可解释；
7. 返回页面缓存、新旧 Project 快速切换、并发合并和显式刷新有请求数上界，无 429；
8. Project、来源应用、Evidence、IAM／People 逐项撤权／停机后行为符合边界；
9. 所选组合逻辑空目标恢复后 Link、source revision／digest、File／Version、字节和 access audit 保持；
10. Project、Production、Contract 数据摘要在纯资料读取／下载中只有 Evidence access audit 合法增长，其他事实零写。

## 7. 明确不负责

- 不建设通用 ECM、网盘、全文检索、任意标签或全站上传中心；
- 不把 access grant 签发冒充下载完成、阅读确认或交付验收；
- 不为首屏计数引入共享业务表、跨库 SQL、每项目每来源扇出或 Workbench 持久库；
- 不在本切片实现 S01～S03 金额统计；统计必须在 F01／F02 边界验证后另立规格；
- 不把一个客户的来源集合写死进 Base，来源注册和 Suite 选择必须显式版本化。

## 8. 当前开放决策

原页面首屏要求精确文件数，但当前稳定边界只保证 Project Resource Link 与来源 material-reference。先用运行测量判断：A）现有 Link 是否可直接给出不泄密的计数；B）是否需要 Project 自有、可重建且带来源 revision／digest 的摘要；C）是否允许首屏暂显“展开读取”。未取得请求量、故障和恢复证据前，不新增 File Catalog 应用或跨来源持久索引。
