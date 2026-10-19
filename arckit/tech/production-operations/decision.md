# Production Operations 多类型边界决策

状态：🟡 目标边界已采用；类型注册、兼容迁移、双类型生命周期、canonical 权限、D1 正式最小组合／恢复与真实浏览器权限生命周期已实现，双 Tenant 与真实 Ubuntu 待验

日期：2026-09-27

## 决策

现有 Design Operations 不复制成 Consultation／Survey／Mapping 三个同构服务，也不把阶段规则迁入 Project。它将以兼容方式演进为一个独立 Production Operations 数据所有者；共同生命周期只实现一次，业务差异通过代码拥有、版本化、逐类型验收的 Type Definition 表达。

当前目录、镜像、数据库物理表和旧 API 名称属于兼容迁移问题，不构成第二数据所有者。任何迁移时刻只能有一份可写 Work Item／Stage／Deliverable／History 真相。

## 依据

- 已确认需求要求四种类型、多选、分类型独立进度和栏目并集；
- 参考实现已经用 `project_type_key + stage_key` 表达类型差异，并复用一套顺序推进；
- 现有 Design Operations 已验证 Work Item、owner、Stage、Evidence、revision、幂等、回退和历史共同内核；
- 当前 Design migration、API、Capability 和 Project 完成立项仍硬编码 design，证明需要真实兼容改造，不能用页面开关冒充完成；
- 批复审计、成本、外委和奖金拥有不同事实与权限，不能成为 Type Definition 的任意字段。

完整证据与方案比较见 `docs/reports/oa-c4-multi-production-boundary-audit-20260927.md`。

## 类型注册表

注册表是编译期／发布物内的不可变定义集合，不是数据库可编辑流程：

```text
Definition {
  key
  version
  required_project_type_key
  ordered_stages[] {
    key
    sequence
    skippable
    required_requirement_keys[]
  }
}
```

Work Item 保存 definition key/version；Stage 与 requirement 在创建时实例化。运行时按 Work Item 的版本读取对应定义，未知版本拒绝启动或读写，不能退回“当前最新配置”解释历史。

第一增量注册：

- `design-v1@1`：保留现有四阶段；
- `consultation-v1@1`：`scheme` 可不适用，`report` 必需且不可跳过；两者均使用 `primary-output`。

## 兼容数据演进

第一条 migration 必须是 additive／compatibility-first：

1. 放宽 Work Item type 约束以接受注册定义；
2. 放宽 Stage key／sequence 硬编码，但继续用外键、唯一 frontier 和服务端定义校验保护；
3. 保留现有 `design-v1` 行及所有 ID、revision、时间、成果和历史；
4. 禁止把旧表复制到新表后双写；若未来重命名物理表，必须单次迁移并用兼容视图／路由读取；
5. 升级前后对既有数据做确定性摘要比较，失败时回滚发布而不重写事实。

物理表可以暂时保留 `design_*` 名称，以降低首轮数据迁移风险；对外目标语义、模型和新 API 使用 Production 命名。物理命名债务要显式记录，但不能成为复制数据库的理由。

## API 与兼容入口

目标 canonical API 使用 `/api/v1/production-work-items` 并在创建时提交 `type_key`。读取和写入继续以稳定 Work Item ID 定位，Stage 写入带 stage key。

兼容期保留现有 `/api/v1/design-work-items`：

- 创建固定注入 `design-v1`；
- 读取／写入只允许目标 Work Item 为 `design-v1`；
- 旧 `design-work-item.*` Capability 不得越权访问其他类型；
- reference-read 保留旧 resource type，新的通用端口发布 `production-work-item`；
- 兼容入口移除前需要版本弃用、消费者清单和迁移证据。

## 权限和工作负载

canonical audience 目标为 `production-operations`，发布 `production-work-item.*` Capability。参考 OA 客户角色继续位于 Overlay；active `production_owner` 是业务前置与 Policy Fact，不等于自动授权。

Audience 与 action namespace 不要求字符串相同。Capability Manifest 与 Suite Manifest 通过可选 `capability_namespace` 显式声明 `production-work-item`；旧应用未声明时回退 application ID。Gateway 用 namespace 校验 entry action，IAM 仍以目标 Capability Catalog 的精确成员关系作为 Grant／Policy／Workload 最终门禁，不用 audience 前缀替代能力所有权。

目标应用复验 issuer、tenant、audience、action、scope、type、resource、state 和 revision。兼容 audience／Capability 只服务 design 旧端口，不能被 IAM 映射成咨询能力。Project 和 Evidence reference-read 使用请求绑定、单 action、精确资源的工作负载凭证；失败不写本地成功事实。

## Project 边界

Project 保留 `project_type_keys`，但完成立项不再硬编码必须含 `design`，而是至少含一个当前组合支持的生产类型。支持目录来自版本化装配／契约，不把 Stage Definition 复制进 Project 数据库。

Production 创建时再次读取 Project：type 必须被 Project 选择且 Project 为 active。Project 类型后来变化不静默删除已有 Work Item；如何停止或迁移已存在类型需要显式命令与后续业务证据，首切片禁止自动级联。

## 失败与一致性

- Project／Assignment／Evidence 仅在需要新鲜外部事实的命令中在线读取；普通详情和历史不依赖其在线；
- 外部校验在本地事务前完成，随后本地投影、Command 和 History 原子提交；
- 成功 command 重放先返回本地结果，不重新依赖上游；
- 同 Project 不同类型使用不同 Work Item lock／revision，互不阻塞和回退；
- Type Definition 只决定本应用 Stage 与成果规则，不能调用 Contract、Money、Billing 或 Project 写接口。

## 被拒绝方案

### 每类型独立应用

当前四类型共享的生命周期、负责人、Evidence 和历史已构成第二个稳定使用点。复制服务会制造重复 migration、Capability、恢复和组合成本。若勘察试验后续出现独立结构化事实、专属一致性与独立产品闭环，再以证据复审拆分，不预先拆四份。

### Project 类型化扩展

Project type key 是项目分类事实；Stage、成果和纠错是生产事实。放入 Project 会让薄容器吸收行业流程，并让未选 Production 的 Project 产品携带无关字段。

### 通用工作流平台

任意阶段编辑、JSON 条件、脚本、动态表单或跨应用动作不能保证历史解释、权限和升级兼容，且违反当前“不先建通用平台”约束。

## 复审条件

出现以下证据时重新评估：

- 新类型无法用共同 Work Item／Stage／成果／纠错语义表达；
- 不同类型需要独立事务、独立部署或独立购买且共享服务造成不可接受耦合；
- 类型定义为了客户差异持续增加任意字段、脚本或条件；
- 兼容迁移必须双写或破坏 `design-v1` 历史；
- 最小组合无法在未选择其他类型时独立运行。

当前已经取得代码注册、旧库完整 fixture 确定性迁移、同 Project 双类型隔离、负责人、适用性、成果替换、完成／重开、依赖失败零写入与恢复证据；canonical API、双 audience、Capability Manifest、OA Overlay、Project Assignment Policy、精确 Project／Work Item Scope 和真库 consultation HTTP 闭环也已通过。最小组合、未选缺席、聚合回归、Nginx→Gateway→IAM 正式入口主链、Suite Manifest／Workbench 和联合空目标恢复已经在隔离 D1 通过。Production 列表与详情现在返回目标应用最终收敛的 `allowed_actions`；86 条自动断言与真实浏览器已验证 Grant 撤销／恢复、账号停用／启用、Assignment 结束、旧签名凭证的最长 300 秒残余窗口，以及页面刷新后退出提交态。双 Tenant 和真实 Ubuntu 仍必须作为独立证据取得。
