# Internal Finance Requests

独立拥有报销和付款申请事实。本阶段只提供可持久化的草稿切片，用于验证 Tenant、Capability、resource scope、revision、命令幂等、审计和恢复；审批、附件与 Money 交付在 P5/P7 实现。

```sh
docker compose up -d --wait
set -a; source .env.local; set +a
go run ./cmd/server
```

备份与空目标恢复：

```sh
./ops/backup "$DATABASE_URL" /absolute/path/finance.dump
./ops/restore-empty "$EMPTY_DATABASE_URL" /absolute/path/finance.dump
```
