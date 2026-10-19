# D1 C4 Production Operations 验收

该验收只选择 People & Groups、Project Workspace、Evidence Service 和 Production Operations，使用正式 Nginx → Gateway → IAM／Policy → 目标应用入口。Contract、Money、Party、Business Engagement 和 Billing 必须真实缺席。

```sh
acceptance/d1-c4-production-authorization/run-isolated run
```

默认使用独立 Compose project `omsd1c4`、端口 `18140` 和显式测试网段，不影响当前 `omsbizfoundry` 本地组合。`d1.env.example` 只含本机验收凭证，不能复制到共享或生产环境。

真实浏览器权限生命周期验收在 `run-isolated verify` 成功后使用受限的本地验收账号，并用以下命令切换权限与账号状态：

```sh
acceptance/d1-c4-production-authorization/browser-permission-control revoke-stage
acceptance/d1-c4-production-authorization/browser-permission-control restore-stage
acceptance/d1-c4-production-authorization/browser-permission-control disable-account
acceptance/d1-c4-production-authorization/browser-permission-control enable-account
```

浏览器必须点击“刷新权限”后才验收动作收敛；旧 Capability Credential 只保留签名的短时剩余窗口。验收结束必须恢复权限并启用账号。

完整恢复复验：

```sh
D1_ENV_FILE="$PWD/acceptance/d1-c4-production-authorization/d1.env.example" \
D1_PROJECT_NAME=omsd1c4 \
D1_COMPOSE_OVERRIDE_FILE="$PWD/acceptance/d1-c4-production-authorization/compose.override.yaml" \
  deployments/d1/ops/d1 backup

D1_CONFIRM_EMPTY_RESTORE=yes \
D1_ENV_FILE="$PWD/acceptance/d1-c4-production-authorization/d1.env.example" \
D1_PROJECT_NAME=omsd1c4 \
D1_COMPOSE_OVERRIDE_FILE="$PWD/acceptance/d1-c4-production-authorization/compose.override.yaml" \
  deployments/d1/ops/d1 restore <BACKUP_ID>

acceptance/d1-c4-production-authorization/run-isolated verify-restored
```

恢复会删除 `omsd1c4` 专属卷，只能对该隔离验收项目执行。`verify-restored` 比较验收保存的 canonical 状态摘要、revision、成果和历史。
