# Payroll

独立拥有工资领域事实。本阶段只提供可持久化的月度工资批次草稿切片，用于验证 Tenant、Capability、resource scope、revision、命令幂等、审计和恢复；薪酬档案、计算与发放在 P3/P4 实现。

```sh
docker compose up -d --wait
set -a; source .env.local; set +a
go run ./cmd/server
```

备份与空目标恢复：

```sh
./ops/backup "$DATABASE_URL" /absolute/path/payroll.dump
./ops/restore-empty "$EMPTY_DATABASE_URL" /absolute/path/payroll.dump
```
