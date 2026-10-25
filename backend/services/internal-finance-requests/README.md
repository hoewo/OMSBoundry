# Internal Finance Requests

独立拥有报销和付款申请事实。P5 已实现草稿／提交／撤回 revision、费用与用途明细、类别字典、收款快照、费用归属、Evidence 稳定引用、`not_required` 审批快照，以及逐笔／批量付款确认。实际付款通过版本化端口交付 Money，申请只保存返回的 Payment／Allocation 稳定引用；普通读取和导出只返回账号尾号，完整账号读取与敏感导出分别授权并逐次审计。P7 才负责 Evidence 真实上传、实时引用校验和员工开放 API 闭环。

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
