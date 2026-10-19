# Access Contract v1

本目录是 M7 统一入口与 IAM 的机器可读契约入口，不是某个 Go 应用的内部数据模型。

固定内容：

- `iam-context.schema.json`：目标应用在验签后提供给业务层的可信上下文；
- `capability-credential-claims.schema.json`：IAM 签发的 Ed25519 JWT Claims；
- `capability-manifest.schema.json`：应用发布的稳定 Capability；应用可以用互斥的 `capability_namespace` 声明一个原生命名空间，或用 `capability_namespaces` 声明多个原生命名空间，未声明时兼容回退 application ID；
- `customer-role-capability-overlay.schema.json`：把客户站点角色映射为某一目标应用的显式 Capability 集合；空集合用于证明身份治理权不会自动产生业务权；
- `suite-manifest.schema.json`：装配层声明的应用、路由和健康入口；`entry_action`／`entry_actions` 互斥且仅作准入 ANY，单个 `capability_namespace`／有限 `capability_namespaces` 同样互斥，未声明回退 audience；不授予动作或扩宽 Scope。可选 `show_in_workbench` 只控制统一工作台是否展示技术应用，不影响路由、进程或授权。新字段须与 Gateway／Workbench strict reader、schema 和渲染器匹配交付；
- `workload-assertion-claims.schema.json`：应用向 IAM 证明工作负载身份、请求绑定和一次性断言的 JWT Claims；
- `service-credential-request.schema.json`：工作负载用断言交换单 audience、单 action、精确资源凭证的请求；
- `workload-public-key-bundle.schema.json`：调用应用公开给 IAM 的 Ed25519 active／retiring 公钥集合；
- `workload-identity-policy.schema.json`：部署层允许某工作负载申请哪些 audience、action 和资源类型；
- `examples/`：当前 v1 正常样例，包括参考 OA C1／C2 站点角色 Overlay、Project 与 Business Engagement 按 audience 分离的 Assignment Policy，以及工作负载身份策略。Project Assignment 角色只由独立版本化 Policy 映射，不复制到站点角色 Overlay；目标应用只解释自身稳定 Capability，不解释客户岗位名。

`schema_version`、`version` 或 JWT `ver` 未知时必须失败关闭。工作负载私钥不属于本目录，也不得交给 IAM；IAM 只读取公钥集合并签发最终 Capability Credential。字段扩展、字段语义变化或兼容规则变化需要先更新规格和负例测试，不能由 Gateway、IAM 或业务应用单方面解释。跨字段规则由 `acceptance/access-contract` 验证；完整语义见 [`docs/specs/identity/access-contract-v1.md`](../../../docs/specs/identity/access-contract-v1.md)。
