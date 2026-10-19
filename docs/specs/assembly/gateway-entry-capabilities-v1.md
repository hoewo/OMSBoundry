# Gateway 应用入口能力集合 v1

问题 A：单个遗留 `view` 粗门禁阻断已经获得 metadata／download 原生能力的用户。可验证结果 B：Suite Manifest 可以显式声明有限的备选入口能力，签名凭证命中任意一项即可被转交目标；目标仍独立复验动作、Tenant、资源 Scope 和当前状态。

## 契约与不变量

- 兼容原 `entry_action` 单字符串；新增可选 `entry_actions` 非空数组（最多 16 项），两者必须且只能选择一种，平台 IAM／Workbench 不声明业务入口能力。
- 数组语义是 ANY，不是同时授予全部能力；每项必须是应用原生 namespace 的完整 action，无通配符、重复或空项。没有入口声明仍拒绝业务路由。
- BF-103：同一个 audience 可显式声明互斥的 `capability_namespace` 或 `capability_namespaces`（1～16、唯一合法标识）；不声明仍回退 audience。入口及投影 action 只能属于所声明的原生 namespace。新字段只用于已发布能力的路由准入，不发明能力、不改变 IAM Catalog 或目标资源规则。
- Bonus 的空 profile 保持 `bonus-pool`／`bonus-pool.read`；`reference-oa-bonus-v2` 与在其上增加专业人员候选能力的 `reference-oa-bonus-v3` 均声明 `bonus-pool`＋`bonus-manual-allocation` 和两者的 read ANY。旧 project／Pool 凭证不能读写手工分配，经营／生产精确 Scope 不互换；record-only 不能因为“写入”被推导为 read 入场权。
- Gateway 的验签、issuer／Tenant／audience／TTL／Scope 校验不变。它不能签发、补齐或扩宽 actions／scope，内部只传递原签名 Token，客户端 Authorization／身份 Header 仍删除。
- Evidence 的第一组声明仅包含既有 Capability Manifest 中的 `view`（兼容原入口）、`metadata.read`、`upload`、`download`。metadata-only 不会因此获得 download，download-only 不会因此获得 metadata，upload 仍不能使用 File Scope 写入；目标 Evidence 是最终判定者，不在入口处发明删除等新 action。
- 授权失败、失效及退出登录清除对应路径 Cookie；ANY 入口应用也必须清除。授权交换仍来自现有 IAM Suite Session，不新增账号、角色或密码。
- Workbench 识别这类已启用入口，只显示导航；它不能依据入口集合推导用户已获全部动作或绕过 `allowed_actions`。

## 兼容与验收

旧 Manifest 在新 Gateway／Workbench 中语义保持。旧 strict reader 会拒绝新字段，不能把新 Manifest 单独下发给旧二进制；必须依 BF-077 同时交付兼容 Gateway、Workbench、schema／验收器和渲染器，其他镜像继续固定实际摘要。本切片没有数据 migration。

Bonus profile 的同 audience 多 namespace 采用相同匹配交付门禁；`d1` 从 env 显式读取 profile 并传给渲染器，未知值在产出 Manifest 前拒绝。目标 Bonus 的 schema 3 属 BF-102 的应用自有升级，不是入口 migration。候选证据与未运行的公网／D1 阶段见[奖金报告](../../reports/oa-bonus-manual-ui-candidate-20261004.md)。

先验证 manifest 正反例、合法多入口签名与错误身份／Scope／签名拒绝、Cookie 清理、原 Token 与 Header 清洗、目标 metadata／download 分权、旧单入口回归和 D1 渲染。再用受保护真实入口证明 metadata-only 拒绝下载、metadata＋download 与 download-only 无需遗留 view、错误文件／租户拒绝。未跑的阶段不得标为通过；原连续 C8 与真人浏览器仍独立补证。

本切片不新增策略引擎、按路径业务授权表、领域职责或公共数据库，不处理 C4～C6 资料来源投影、OSS Provider 切换或标准 Web 静态分离。
