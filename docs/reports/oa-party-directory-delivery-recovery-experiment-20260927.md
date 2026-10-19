# OA Party Directory 独立交付恢复实验报告

日期：2026-09-27

结论：**Party Directory 已从“核心可运行”提升为“可生成独立发布物、以非 root 容器运行、可完整备份到空目标并在恢复后继续写入”；D1 正式入口、管理页面和消费应用迁移仍未通过。**

## 1. 本轮假设

如果 Party Directory 真能作为客户组合中的独立产品或显式依赖，它不能只在源码工作区里启动。至少要证明同一边界可以被固定发布、校验、安装、迁移、备份和恢复，而且恢复不得重建 Party ID、压平 revision 或覆盖非空数据库。

本轮只检验 Party 自身的交付与恢复因果链，不把 Contract／Money 消费者接入、D1 Nginx／Gateway／IAM、Workbench 页面或旧目录迁移混入结论。这样一旦失败，可以区分“应用自身不可恢复”和“跨应用装配错误”。

## 2. 新增交付机制

### 2.1 固定发布包

`backend/services/party-directory/ops/build-release` 生成带 SHA-256 的平台归档，包含：

- `party-directory-server`；
- `party-directory-ops`，提供 `migrate`／`check`；
- `party-directory-backup`，提供 `create`／`restore`；
- 环境样例与 PostgreSQL Compose；
- 安装升级、备份恢复和应用说明；
- 7 份版本化 Party API 契约与 `Party`／`PartyRevision` 模型。

三个二进制都输出 service、version、commit、built_at、Go version 和 target，发布脚本拒绝覆盖已有归档。

### 2.2 非 root 镜像

多阶段 Dockerfile 编译静态 Go 二进制，最终层使用 `postgres:18.6-alpine`，声明 `USER postgres`。选择该最终层不是让 Party 拥有 PostgreSQL 进程，而是让离线备份工具获得与服务端相同主版本的 `pg_dump`／`pg_restore`；容器默认入口仍是 Party HTTP 服务。

### 2.3 完整备份格式

备份 ZIP 只允许 `manifest.json` 与 `database.dump` 两个普通、未压缩条目。manifest 记录格式版本、UTC 时间、源数据库、PostgreSQL 版本、schema migration、dump 大小／SHA-256 和公开表逐表行数。应用事务导出 PostgreSQL snapshot，`pg_dump` 使用同一 snapshot，避免清单与 dump 观察到不同写入时点。

恢复在写入前拒绝：未知格式、非法 ZIP 路径、额外或重复条目、摘要／大小不符、客户端或目标 PostgreSQL 主版本不一致，以及任何非空目标数据库。`pg_restore` 使用单事务、`--no-owner` 和 `--no-privileges`；完成后再次比较 schema、表集合和逐表行数。

Party 备份只拥有 Party 当前档案、revision、幂等命令和审计，不包含 IAM 账号、Grant、Session 或密钥。这修正了早期技术稿中把 IAM 环境组件含混写进 Party 备份的边界错误。

## 3. 实验环境与隔离

本机 Docker 已存在较多长期运行的 OA 验收组合，且新建 Compose 网络会触发地址池耗尽。为避免停止或改写任何现有业务容器，本轮复用一个已存在的用户定义 bridge，只在其上启动：

- 一个 `postgres:18.6-alpine` 临时容器；
- `/var/lib/postgresql` tmpfs 数据目录；
- source／target 两个独立空数据库；
- source／target 两个短生命周期 Party 容器。

验收结束删除这些临时容器、验收镜像和工作目录。它没有读取或写入既有应用数据库。

第一次运行暴露了 PostgreSQL 18 官方镜像的数据目录变更：把 tmpfs 或 named volume 挂到旧的 `/var/lib/postgresql/data` 会被镜像主动拒绝。验收脚本和 Party Compose 均改为 `/var/lib/postgresql` 后重跑。第二个工程性改进是在数据库 URL 中明确 `connect_timeout=5`，使错误网络拓扑快速失败而不是无限等待。这两个失败都保留为交付学习，不计作通过。

## 4. 实际验收序列

执行入口：

```bash
backend/services/party-directory/ops/verify-delivery-recovery
```

实际通过的顺序：

1. 构建 `v0.1.0-acceptance.1` 开发验收发布包；
2. 校验 SHA-256、归档根、三份可执行文件、版本／commit、7 份契约与 2 份模型；
3. 构建最终镜像，确认声明用户不是 root、实际 UID 非 0，并确认镜像内 PostgreSQL 客户端主版本为 18；
4. 在 source 空库执行 migration／check；
5. 经 HTTP 创建 Party revision 1；
6. 更正名称形成 revision 2；
7. 停用形成 revision 3；
8. 停止 source 服务并创建完整备份；
9. 校验 manifest 格式、schema 和关键表清单；
10. 恢复到 target 空库；
11. 对同一非空 target 再次恢复，确认明确返回 `restore target database is not empty`；
12. 对 target 执行 schema check；
13. 启动恢复后的服务，确认相同 Party ID、revision 3、inactive、已更正名称和 3 条历史；
14. 使用 Expected Revision 3 重新启用，成功形成 revision 4。

额外通过：

```bash
cd backend/services/party-directory
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

## 5. 架构结论

这份证据支持以下结论：

- Party 的数据所有权可以形成独立发布和恢复边界，不需要 Contract／Money 数据库参与；
- Party ID、current projection、不可变 revision、幂等命令和审计能作为一个整体迁移；
- 空目标保护与恢复后继续写入比“dump 命令退出 0”更接近可运营恢复；
- 将 API 契约和模型随发布物交付，有助于消费应用锁定版本，而不是导入 Party 内部代码；
- 非 root 应用容器与数据库客户端工具可以共存，不需要给 HTTP 服务 root 权限。

它不支持以下更大结论：

- Party-only 完整客户产品已经完成，因为管理页面尚未实现；
- Nginx／Gateway／IAM／Workbench 的 D1 正式链已经包含 Party；
- Contract、Money 或 Project 已经消费 Party API 并冻结历史快照；
- 旧本地 Party 目录已经安全迁移；
- Party 停机时消费应用“既有历史可读、新鲜引用失败关闭”的组合故障已经实证；
- 整个 OA 完整体或跨客户 Base 已经验证成功。

## 6. 下一轮选择建议

最有价值的下一因果切片不是继续给 Party 增加字段，而是选择一个真实消费者，完成“精确引用 → 本地业务角色和不可变快照 → Party 更名／停用不改历史 → Party 停机时旧事实可读、新引用失败 → 恢复后新命令单次成功”。这会直接检验独立 Party 是否减少耦合，而不是只把耦合从数据库搬到 HTTP。页面和 D1 可以随后复用已稳定的消费契约；现有双目录迁移应在第一个消费者模型被证明后再做，避免过早固化错误映射。
