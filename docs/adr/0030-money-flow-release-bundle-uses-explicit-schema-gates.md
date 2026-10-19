# ADR-0030：Money Flow 版本化交付包使用显式 Schema 门禁

日期：2026-09-20  
状态：Accepted

## 背景

Money Flow Ledger 已具备核心业务闭环、owner 范围台账导出和全量数据库备份恢复，但当前仍由 `go run` 启动，服务进程在监听前逐文件自动执行 migration。这个方式适合早期开发，却不能回答交付时正在运行哪个版本、升级前能否识别 pending／未知 schema，也不能保证多个 pending migration 中后一个失败时前一个不会残留。

目标客户的最终部署形态尚未确定，因此此时引入 Kubernetes、systemd 专用包或云平台模板会把未知基础设施固化为产品约束。需要先提供与部署编排无关的最小交付协议。

## 决策

1. Money Flow release bundle 固定包含 server、ops、backup 三个静态风格二进制、示例配置和安装升级／备份恢复文档，并提供外部 SHA-256。
2. 三个二进制嵌入同一 version、commit、build time；build identity 与数据库 schema version 分离。
3. 新增独立 ops 入口执行 `check`、`migrate` 和 `version`，server 不再是唯一 migration 入口。
4. 数据库 schema 只接受嵌入 migration 清单的空状态、严格前缀或完全一致；未知版本、缺口和非托管已有 relation 都是 incompatible。
5. 一次 `ops migrate` 的全部 pending migration 在同一 PostgreSQL 事务和 advisory transaction lock 中提交；该独占锁与全量备份的 shared lock 使用同一 key。
6. server 支持开发用 `auto` 和正式交付用 `verify`。`verify` 只检查 current schema，绝不在启动时写 schema。
7. PostgreSQL 18 和 schema current 都是服务监听前条件。
8. migration 仅前向。升级后需要回退时，不执行 down migration；使用升级前全量备份恢复到新目标，再运行旧 bundle。

## 原因

- build identity 让运维能够把二进制、日志、归档和验收记录对应到同一交付物。
- 独立预检和迁移把“检查”“修改 schema”“启动业务流量”分开，失败位置更清楚。
- 严格前缀规则能识别旧二进制面对未来 schema、手工插入版本和 migration 缺口，避免带着未知契约继续运行。
- 单事务 pending migration 比逐文件提交更容易安全重试，也减少升级到一半的状态。
- 与备份共享锁协议，能保证 snapshot 建立与 migration 不交错。
- `verify` 模式适合受控部署，保留 `auto` 默认则不破坏当前本地开发效率。
- 恢复到新目标比 down migration 或覆盖原库更符合不可变事实和已验证备份边界。

## 后果

- 正式升级增加显式 `check → backup → stop → migrate → check → start` 步骤，但每一步都有可验证结果。
- 未来 migration 不能直接使用非事务 DDL；如确有需要，必须单独设计阶段化升级协议。
- 本轮只生成当前 GOOS／GOARCH 或显式指定目标的 bundle，不替代多平台发布流水线和代码签名。
- MF-M4-014 之前构建的旧二进制没有完整 unknown-schema 门禁，不能据此认为它们可在未来 schema 上安全运行。
- 应用不管理进程监督、证书、反向代理或流量切换，安装手册必须明确这些外部责任。

## 被否决方案

### 继续只让 server 自动迁移

无法在停机前确认兼容性，也把 schema 修改和接受业务流量绑定到同一进程启动动作。

### 延续逐 migration 独立提交

后续 migration 失败会留下中间 schema，使重试、旧版本启动和故障判断都更复杂。

### 删除自动迁移并立即强制所有开发环境使用 ops

会不必要地破坏当前本地快速开发入口。保留 `auto` 默认、正式部署显式使用 `verify` 可以逐步收敛。

### 提供 down migration

已有 migration 包含不可变事实、回填、约束和触发器变化，通用 down migration 可能丢数据或制造假历史；全量备份恢复到新目标更可信。

### 现在提供 Kubernetes／systemd／云平台安装包

客户部署约束尚未确认，过早选择编排平台会扩大交付面并形成新的强耦合。

### 用 release version 代替 schema migration 状态

同一二进制可能面对空库、旧库或恢复库；build identity 不能证明数据库已经应用哪些 migration，两者必须分别报告。
