# OA Party 管理与 Project 客户选择实验

日期：2026-09-27  
结论：`supported-for-local-d1-human-workflow`

## 验证问题

> Party Directory 的领域边界、API、Project 精确引用和 D1 故障恢复已经成立，但人还不能完成日常主体管理和建项选客 → 在不共享数据库、不复制 Party 模型、不削弱 IAM 的前提下，让操作者从一个工作台完成 Party 管理，并在 Project 立项时选择当前有效客户、由服务端复验后冻结历史快照。

本实验只验证本地 D1 的人类工作流、API 链和故障恢复。它不声明 Contract／Money 已迁移、客户改绑语义已成立、双租户矩阵已完成，或真实 Ubuntu／公网入口已经验收。

## 本轮保持的边界

1. Party Directory 独立拥有主体当前档案、状态、稳定 ID 和不可变 revision。
2. Project Workspace 独立拥有“项目选择了哪个客户”的关系，以及选择发生时的最小主体快照。
3. 浏览器发现候选时使用当前用户对 Party Directory 的原生 Capability；Project 服务保存前仍使用自身工作负载身份，向 IAM 交换单 action、精确到 `party:{id}` 的短期凭证并复验所选 revision。
4. 浏览器列表不是可信授权结果，不能代替服务端精确引用校验。
5. Base 允许不选 Party；参考 OA Overlay 可以把客户选择收紧为必填，但不能修改 Base 数据所有权。
6. 首个切片不提供改绑或解绑。未来若有证据需要，必须新增带 expected revision、历史和明确失败语义的命令，不能静默覆盖既有客户快照。

## 实现与操作链

### 1. Party Directory 人类管理入口

Party Directory 进入 Suite Manifest 和 Unified Workbench，提供同一页面中的完整第一版任务：

- 按名称、主体类型和状态查询；
- 查看当前详情、状态和 revision；
- 登记组织或个人；
- 使用 expected revision 更正名称或登记标识；
- 停用和恢复；
- 查看不可变 revision 历史；
- 显式刷新 IAM 权限；
- 对 `409 REVISION_CONFLICT` 重新加载当前事实后再决定是否重试；
- 对依赖不可用保留用户输入并提供显式重试。

页面写入口只按 Party API 返回的 `allowed_actions` 开放；前端没有自行解释客户职位或扩大能力。

### 2. Project 新建时选择客户

Project 新建页增加可选客户选择器：

1. 用户打开“新建项目”；
2. 浏览器向 Gateway 请求 Party Directory 用户凭证；
3. 使用该凭证读取 active Party 列表；
4. 用户可保持“不选择主体”，或选择一条 Party revision；
5. 浏览器只向 Project 提交 `party_id + source_revision`，不提交可信名称或类型；
6. Project 服务使用自身工作负载身份读取 Party 精确引用；
7. Party Directory 最终校验 tenant、action、scope、active 状态和 revision；
8. Project 只保存 `party_id + source_revision + kind + legal_name` 快照。

候选列表无权或暂时不可用时，页面显示降级说明，但不阻断 Base 的无 Party 立项。既有 Project 详情只展示自身已冻结快照，不在线拼接 Party 当前档案。

## 自动化和运行证据

本轮实际执行：

```bash
gofmt -w \
  backend/services/party-directory/internal/party/model.go \
  backend/services/party-directory/internal/httpserver/server.go \
  backend/services/party-directory/internal/httpserver/demo.go \
  backend/services/party-directory/internal/httpserver/server_test.go \
  backend/services/project-workspace/internal/httpserver/demo_overview.go

go test \
  ./backend/services/party-directory/... \
  ./backend/services/project-workspace/... \
  ./frontend/oms-workbench/... \
  ./backend/platform/gateway/...

sh -n acceptance/d1-delivery/verify-running \
  deployments/d1/ops/render-suite-manifest

deployments/d1/ops/d1 up
deployments/d1/ops/d1 check

D1_ACCEPTANCE_DISRUPTIVE=no acceptance/d1-delivery/verify-running
D1_ACCEPTANCE_DISRUPTIVE=yes acceptance/d1-delivery/verify-running
```

浏览器脚本还分别抽取后执行 `node --check`，两项均通过。

实际结果：

- Go 单元／HTTP／装配测试全部通过；
- Party 人类页面、脚本、active 列表和动态允许动作通过运行断言；
- Project 客户选择器、跨应用授权入口和只提交 ID／revision 的页面契约通过运行断言；
- Party 已显示在 Workbench；
- D1 schema、健康与入口检查通过；
- 非破坏性 D1 验收通过；
- 破坏性验收再次通过 Party 停机／恢复：无 Party 的 Base Project 可建，含新引用的命令返回 503 且无部分写入，既有快照可读，恢复后以新 Command ID 重试只写一次；
- Evidence、People、Contract 等既有回归未被本轮修改破坏。

破坏性验收摘要：

```text
D1 running-stack acceptance passed: nginx-only exposure, one login, enabled-app read/write,
People roster snapshot, Party to Project exact-reference lifecycle and Evidence byte lifecycle,
fault isolation and recovery
```

## 证据矩阵

| 要证明的事实 | 直接证据 | 结果 |
| --- | --- | --- |
| Party 是可操作的独立产品入口 | Workbench 条目、管理页、列表／详情／历史和 HTTP 测试 | 支持 |
| 用户发现候选不等于服务端信任 | 浏览器使用 Party 用户凭证，Project 服务另行交换 workload credential | 支持 |
| Project 不复制 Party 当前档案 | 创建请求只含 ID／revision，服务端冻结最小快照 | 支持 |
| Base 不强依赖 Party | Party 停机时无 Party Project 仍可创建 | 支持 |
| 新引用失败关闭且可恢复 | Party 停机 503、无部分写入、恢复后新命令单次成功 | 支持 |
| 既有项目不受 Party 当前变化／故障影响 | Project 详情读取本地冻结快照 | 支持 |
| 首个切片没有隐式改绑 | 更新契约继续拒绝夹带客户变化，页面无改绑入口 | 支持 |

## 结论

本轮支持以下局部结论：

1. Party Directory 不只是 API 或“辅助数据表”，已经能够作为独立主体管理产品进入同一 D1 人类工作流；
2. 当 Party 在 Project 组合中扮演显式依赖时，人类发现和服务端确认可以分层，既保留使用体验，也没有把浏览器列表变成可信事实；
3. Project 保存业务发生时快照，Party 保持当前档案所有权，二者通过版本化契约和 IAM 工作负载凭证协作，无需共享表或复制领域模型；
4. “最小搭建并可扩展”的边界在该切片中成立：不选 Party 的 Base 仍独立，参考 OA 可通过 Overlay 要求选择，未来改绑只能增加显式命令而不能重写历史。

## 未证明和下一步

- Mac 自动化表面持续报告宿主机锁定，因此本轮没有把人工可视点击巡检写成通过；页面脚本语法、HTTP、运行断言和完整 D1 自动化已经通过，待宿主机可被自动化会话读取时补做可视点击证据。
- Contract／Money 仍各自拥有当前本地 Party 目录；尚未完成映射、切换、回退和历史保持迁移。
- 未验证 Party 改绑／解绑的业务需要和语义。
- 未完成 Party／Project 的完整双租户交叉矩阵。
- 未在公司私有 Ubuntu 或个人试验田公网 HTTPS 上复演本轮工作流。
- OA 完整体的 Billing、成本外委、奖金、多生产类型和跨域投影仍未完成。

因此，下一优先切片是 Contract／Money Party 迁移方案与第一条迁移验收；双租户矩阵和真实 Ubuntu 保持独立证据轴。OA 完整体阶段结论继续是 `insufficient-evidence`，不能由本轮局部成功提前升级为整体可行。
