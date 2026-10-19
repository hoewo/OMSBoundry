# 原 OA 项目甲方信息修复验证

日期：2026-10-02  
状态：行内录入候选分层验证；B01 全项、Main 与完整 OA 未通过  
关联：BF-088、[操作基线](../specs/assembly/oa-original-usage-workflow-baseline.md)

## 结论

按原 OA 的实际路径接通“同一项目 → 经营信息 → 甲方信息 → 行内编辑／保存”，不跳到 Party 产品，不创建或修改主体主档。名称必填、联系人和电话可选，取消不提交；保存后回读同一项目，刷新仍在经营页。**这是候选代码，不是公网主站已更新或完整仿制已完成。**

问题 A → 可验证结果 B：原甲方工作面无法提交、用户被要求跨系统录入 → 在原工作面提交 Project 自有联系资料，保留历史、权限和安全重试。

只读核对原仓库 `826e0fc65aa09947fc4f64f62a2644fa5d11a302` 的 `BusinessInfoTab.tsx`、`ProjectPartyNameInput.tsx`、`useProjectPermissions.ts`、`oa2026/service.go` 和权限目录。原界面明确说明名称和联系人直接保存到当前项目，不新建或修改甲方主数据。原仓库仍干净；原站真人复核未运行。

## 所有权与权限

[Counterparty Context v1](../../arckit/spec/project-workspace/counterparty-context.md) 只拥有本项目的甲方名称、联系人和电话，以 Project 自有不可变历史快照为权威事实；最新值只读投影，Project revision、历史和幂等回执同事务。没有新应用、共享表、schema 升级或外部双写。

创建时的精确 `client_party` 身份快照保持不变，界面分开说明。自由文本不是稳定 Party ID，不自动匹配、改绑主体，也不能作为合同／资金正式身份。普通 Project GET／Reference 不新增联系资料；读取历史但没有联系资料读取权时，此事件内容脱敏。

新增独立原生 read／manage 能力，检查 Tenant、Project Scope 与 active 状态。Capability Catalog 0.6.0、Overlay 1.3.0、Assignment Policy 1.2.0 仅供显式选择；不改变旧策略／Grant，不按客户岗位名写领域判断。原经营人员有甲方维护权，但当前通用 `participant` 不足以证明其就是经营参与人；此映射仍待补，不能给所有参与人扩大权限。

## 实际证据

| 层级 | 已运行结果 | 不代表什么 |
| --- | --- | --- |
| 本地无容器 | Project／Workbench Go 测试与 vet；IAM 相关策略／能力测试与 vet；Node 29 项零 skip、两份 JS 语法通过 | 不是正式联合入口 |
| 私有 PG18 `o` | Project 109／HTTP 75 条含子测试 PASS、零 skip；普通项目／包、历史、no-op、重放、stale、隔离、回滚、并发与原有回归 | 测试身份，不是 Main／公网 |
| 追加恢复 `q` | Project 全组 110 条含子测试 PASS、零 skip；新联系资料专属 `pg_dump → 空目标 pg_restore` 后继续修改 | 单应用恢复，不是新 Suite 联合恢复 |
| 独立启动 `p` | 新二进制、自有空库 schema 10、非 root 只读进程；七个匿名原生路由 401，重启事实不变 | 公开测试 JWKS，不是正式 IAM 签发链 |
| 合成浏览器 | 行内保存／回读／刷新、取消零写、必填及电话拒绝、no-op、可选项清除、只读无入口、保存后读取 503 只重读 | 明示替身，不是 IAM／PG 与浏览器一次联合成功 |

恢复前完整自有事实摘要 `70afe760c948ead18cb8e72f88463f2af24421f462560bead5e9ce7be92204c8`，dump SHA256 `37736069807ad5456187a3c4eae6ecbd0c1d238caaa7b261f74b97e0827fe969`。空库恢复后全量摘要、当前联系资料、两次历史、原主体身份与命令结果相等；旧命令重放零新增，恢复 revision 5 后继续到 6。测试中 Party／People 为显式端口替身，不因此宣称跨系统身份链通过。

三个浏览器探针分别执行：故障 153.05 秒（1 写／3 读）；最终正常 217.63 秒（3 写／5 读，包含一次 no-op）；只读 127.15 秒（0 写／1 读）。其他 update／assignment／correction／dispatch／complete 命令均为 0。最终正常状态 revision 5，可选联系人与电话明确为空。1280×720 实测保存按钮 y=452.4375、center hit 为按钮，主区域可滚动；控制台无 error。截图仅工具内检查，没有虚构截图归档文件。

首轮 `n` 的独立事务测试夹具忘填立项类型，激活前置条件失败；保留失败诊断，补齐夹具再跑 `o`，没有放宽正式规则。初次浏览器刷新会退回基本信息，原因是先改列表 URL 再读取请求 tab；改为预先捕获，最终正常探针验证刷新仍为经营信息。首个探针误用不存在的 `/workbench/` 路径，未绕过浏览器阻断，改用夹具实际根路径，不算成功证据。

资源保持串行：私有 runner＋PG 上限 0.45 CPU／448 MiB，不发布端口、不启动本地 Docker／restore20。`n`／`o`／`q` Main 13 库摘要及 17 容器 ID 不变，诊断保留；测试探针全部停止、无 OOM，失败 `n` exit 1，其余 exit 0。

独立启动 `p` 同样保持 Main 摘要／容器 ID 不变，server＋PG 上限 0.4 CPU／352 MiB、没有公网端口；进程与私有 PG 已停止，无 OOM。结果见[真库／恢复／启动摘要](../../acceptance/oa-initiation-repair/evidence/client-context-results-20261002.json)和[合成状态](../../acceptance/oa-initiation-repair/browser-client-synthetic-state-20261002.json)，运行入口见[验收说明](../../acceptance/oa-initiation-repair/README.md)。

## 仍需接通

后续更新：名称补全已在 BF-089 取得候选分层证据，见[后续报告](./oa-client-name-clone-repair-validation-20261002.md)。下段保留本报告批次当时的未实现状态；经营参与人映射、B01 全项与正式部署仍未关闭。

原控件还带历史名称的幽灵文字提示：无下拉，Tab 显式接受且继续正常焦点移动，原 API 查询历史项目名称。此功能未实现；目标须按可信 Project Scope 与资料读取权过滤，不能照搬原全局搜索或将名称当主体身份。原经营参与人权限映射、其余经营栏目及 P02 资质／费率等也未完成。

所有新版本正式 IAM、兼容发布、Suite 联合备份／空目标恢复及公网真人操作是后续独立门禁。完整 OA 结论继续 `insufficient-evidence`，不以 API PASS 数或单张可保存表单代替整站可用。
