# OA C1 项目包、转换与权限恢复实验（2026-09-26～2026-09-27）

## 判定

- 实验切片：C1 的 Project Package、两级父子关系、既有 Project 转换、安全撤销、客户页面与权限变化恢复。
- 本切片判定：`supported`。
- OA 完整体阶段判定：`insufficient-evidence`。
- 未被本实验验证：项目包合同金额分配、公司／公网真实环境、跨租户运行矩阵、像素级浏览器视觉回归、C2～C8。

## 问题 A → 可验证结果 B

**问题 A**：参考 OA 需要项目包和子项目，但如果转换时复制合同、资金、生产数据或改变原 Project ID，Base 会重新形成跨应用耦合；如果撤销采用级联删除，又会破坏已发生事实。

**可验证结果 B**：Project Workspace 只增加自己拥有的两级关系和转换事实；既有 Project 的 ID、编号、Assignment、Resource Link 与外部业务引用保持不变；安全转换可显式撤销，存在新增事实时失败关闭且不产生部分写入。

## 假设与反证

- H1：`project/package + parent_project_id + conversion` 足以表达参考 OA 当前确认的两级项目包基础语义，无需把合同金额或生产状态复制进 Project。
- H2：`attach_existing` 与 `wrap_as_package` 可以在保留原 Project 身份和已有引用的情况下完成。
- H3：双 revision、幂等命令、不可变历史和安全撤销门禁可以阻止并发覆盖、第三层和破坏性回退。
- H4：项目包治理能继续沿 `Nginx → Gateway → IAM → Project` 授权边界工作，且不授予身份超级管理员、总经理或项目资源凭证隐式层级管理权。
- 零假设／反证条件：出现原 Project ID 变化、Assignment／Resource Link 搬移或复制、第三层成功、旧 revision 写入成功、不安全撤销产生部分写入、相邻角色可越权，任一项即否定本切片。

## 控制变量

- 单一 Project Workspace 镜像和同一 migration 集合；
- PostgreSQL 18.6；
- 同一 `project-workspace.*` Capability 命名空间；
- 正例和反例使用同一 HTTP 路由、同一 IAM audience 与同一 D1 入口；
- 不引入 Party、Contract、Money、Design 数据表或 ORM；
- 不实现项目包金额分配、客户专属编号后缀、自动汇总或生产继承。

## 实现事实

- `projects` 新增 `node_type`、`parent_project_id`、`sort_order`；Package 必须顶层，Project 最多属于一个 Package。
- 非空编号继续租户内不区分大小写唯一；Package 可以暂不编号，普通／子 Project 必须编号。
- `project_package_conversions` 保存操作、原位置、双 revision、是否自动创建 Package、Actor、状态和撤销原因。
- `attach_existing` 附加到既有 Package；`wrap_as_package` 原子创建外层 Package，并把原 Project 作为首个子项。
- 撤销不物理删除：attach 恢复原位置；安全 wrap 撤销把自动外层 Package 标记为 `cancelled`。
- 自动外层 Package 一旦有新增子项、Assignment、Resource Link、资料、状态或 revision 变化，返回 `package_rollback_unsafe`。
- `project-workspace.hierarchy.manage` 是 privileged tenant capability；资源级凭证不能执行同时改变两个节点的命令。

## 实验步骤与直接证据

### 1. 普通与 PostgreSQL 事务回归

```bash
go test ./backend/services/project-workspace/...

TEST_DATABASE_URL='postgres://project_workspace:***@127.0.0.1:55437/project_workspace?sslmode=disable' \
  go test -race -count=1 -tags=integration ./backend/services/project-workspace/...
```

结果：全部通过。直接覆盖：

- Package 无编号创建；
- attach／wrap 与相同 command 重放；
- 原 Project ID 与 Resource Link ID 在转换、撤销后不变；
- Package 读取返回直接子 Project 摘要；
- 跨租户 Package 不可见；
- 旧 Package revision 的竞争命令被拒绝且 Project 仍为顶层；
- Package 作为子项／第三层被拒绝；
- attach 撤销恢复顶层；
- 未继续使用的自动外层 Package 可撤销并保留 cancelled 历史；
- 自动外层 Package 增加 Evidence Link 后自动撤销被拒绝，原父子关系保持不变。

### 2. 正式 D1 入口与权限矩阵

隔离 D1 只重建 Project Workspace，并执行 migration `000002_project_package_hierarchy.sql`；IAM 重启后加载 `project-workspace-capabilities@0.2.0`。其余应用和 PostgreSQL 数据卷不重建。

```bash
D1_ENV_FILE="$PWD/deployments/d1/.env" \
D1_PROJECT_NAME=omsd1c1 \
D1_PUBLIC_URL=http://127.0.0.1:18130 \
  acceptance/d1-c1-authorization/verify-running
```

2026-09-26 的结构与权限基线得到 134 个 HTTP 断言。2026-09-27 加入客户页面资源、服务端 `allowed_actions`、权限刷新和撤权恢复后，同一脚本得到 152 个 HTTP 断言，状态不匹配 0，最长本地请求 0.263164 秒。链路经过 Nginx、Gateway、IAM／Policy、Project 和 People。直接覆盖：

- 身份超级管理员没有隐式 Project 业务权；
- OA 业务管理员通过显式 tenant Capability 创建 Package、attach／wrap 和撤销；
- 总经理可以读 Project，但创建 Package 返回 403；
- 项目负责人和参与者继续只取得 Project A 的资源凭证，不能获得层级治理；
- attach 相同 command 重放得到同一 conversion；
- Package 详情只返回一个直接子 Project；
- 旧双 revision、第三层和 active 状态资料绕过均被拒绝；
- 自动外层 Package 资料改变后撤销返回 `package_rollback_unsafe`，子 Project 仍指向该 Package；
- 未继续使用的另一个自动外层 Package 可安全撤销并进入 cancelled。
- 列表／详情按照 Credential、Scope 和 Project 当前状态返回 `allowed_actions`，客户页面只使用公开 API 完成 Package 创建、attach／wrap 和安全撤销；
- 撤销 `hierarchy.manage` 后重新授权，页面数据不再给出结构动作，直接写入返回 403；撤销全部 Project 入口能力后，Gateway 清除旧 Project Capability Cookie。

## 观察与解释

H1～H4 在本切片证据范围内成立。Project Package 的稳定 Base 不需要共享合同、资金或生产表；转换只改变 Project Workspace 自有关系。这支持“稳定 Base + 只做有界加法”的方向，也保留了以后由 Contract／投影层实现客户金额分配的空间。

本实验没有证明项目包业务已经完整。参考 OA 的合同金额如何分配到子项目、回退后如何结算仍缺业务证据。把这些规则现在加入 Project 会把未知客户策略误当公共内核，因此保持未实现是本轮的正确结果。页面资源和同一 API 已通过自动验收，但本轮没有用浏览器自动化做像素与手势级视觉回归。

## 下一步

1. 在获得参考业务证据前，只记录项目包金额分配问题，不实现猜测规则。
2. 转入 C2“经营承接与合同链”的最小闭环；先确认承接方式和合同链事实所有者，再编码。
3. 为后续浏览器演示补充项目包页面的人工／自动视觉走查，但不得用它替代 API 与数据库证据。
4. 公司 Ubuntu 或公网试验田可用后，用同一镜像和 migration 重跑入口、备份恢复与继续写入。
