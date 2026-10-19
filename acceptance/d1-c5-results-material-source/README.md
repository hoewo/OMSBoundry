# C5 Results 资料来源（来源切片，不是完整 C5／C8）

A→B：批复／审计文件引用已有自有事实但无只读装配端口 → 单个 Result 发布当前材料投影，目标以既有原生 read 能力和资源 Scope 授权。

`verify-ecs.py` 是 Auth58 2026-10-01 有界现场驱动：精确确认已停止的 C4 私有 PG 标签、镜像和 internal network；只运行私有 PG18 与一个 Results 测试／IAM8 容器，总上限 0.55 CPU／416 MiB，无公开端口。新 IAM 自有受限角色／DB／密钥；结果领域测试创建并清理自己独占的随机 schema，绝不传 Main DB 地址。Fixture 的 Project、Contract、Evidence 写入前引用是显式测试提供者，读取时这些端口不可调用。IAM 凭证则由真实 IAM8 签发，由 Results 正式 verifier 验证。

检查当前 v2、更正前 v1 历史、撤回后的保留引用、五表只读 canonical 不变、匿名／相邻 Result／缺 read／错 Tenant 拒绝，以及精确 Result／Project／Tenant 正例。不额外授予 amount.read。拒绝 SKIP 或非零退出，不以编译替代实际运行。

驱动依赖已有 Evidence 入口探针的只读 Main snapshot 工具，并检查 13 个 Main DB 与 17 个容器身份不变；始终停止这两个工作负载并保留所有容器、卷、IAM 数据与私有诊断。不得启动 restore20 或将来源单项通过解释为 Main 发布、Project 消费、联合恢复、原连续 C8 或完整 OA 验收。
