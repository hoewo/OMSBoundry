# Party Directory

Party Directory 独立拥有租户内主体当前档案、稳定 Party ID 与不可变 revision。客户、甲方、资质单位、供应商、合同相对方和债权债务角色仍由消费应用拥有。

本地最小组合：

```bash
docker compose up -d --wait
set -a; source .env.local; set +a
go run ./cmd/server
```

应用不共享 Contract 或 Money 数据库。`GET /api/v1/references/parties/{id}` 只接受 `party-directory.reference.read` 与精确 `party:{id}` scope。

独立发布与恢复：

```bash
make party-directory-release VERSION=v0.1.0
make party-directory-delivery-recovery-acceptance
```

发布包含 `party-directory-server`、`party-directory-ops`、`party-directory-backup`、环境样例、运维文档、7 份 API 契约和 2 份模型。`Dockerfile` 使用非 root 用户，最终镜像携带 PostgreSQL 18 同主版本备份工具。安装升级见 `docs/operations/party-directory-install-upgrade.md`，备份恢复见 `docs/operations/party-directory-backup-restore.md`。
