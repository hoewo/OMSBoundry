# Evidence Service

独立文件与证据服务。它确认 File ID／Version、实际大小、服务端媒体类型和 SHA-256，并通过本地文件或阿里云 OSS Provider 提供受控上传与短期访问；业务应用不保存文件字节或存储凭据。

## 本地启动

```bash
docker compose up -d --wait
cp .env.example .env
# 替换 .env 中的模板令牌和 LOCAL_SIGNING_KEY 后加载环境变量
set -a; source .env; set +a
GOWORK=off go run ./cmd/server
```

健康检查：

```bash
curl --fail http://127.0.0.1:8085/health/live
curl --fail http://127.0.0.1:8085/health/ready
```

所有控制面端点要求 `Authorization: Bearer ...`。本地数据面只接受控制面签发且绑定到单一对象的短期 HMAC URL。

新 File 使用 `POST /v1/upload-sessions` 建立 Version 1；已有 File 使用 `POST /v1/files/{file_id}/versions/upload-sessions` 追加由服务端分配的不可变版本。稳定 File 引用返回最新已完成版本，显式 Version 引用保持历史不变。Suite 页面经 Nginx／Gateway 使用 HttpOnly IAM 能力 Cookie；只有直连开发入口才需要人工提供模板 Bearer Token。

独立 Docker 组合使用全新 PostgreSQL 和对象 volumes：

```bash
cp .env.example .env
# 替换全部 replace-with- 值
docker compose -f compose.standalone.yaml up -d --build --wait
```

清理过期、取消和失败会话：

```bash
GOWORK=off go run ./cmd/ops cleanup
```

完整冷备份与空目标恢复由 `evidence-backup create|restore` 提供；本地模式备份数据库和对象字节，OSS 模式备份数据库并依赖 Bucket 自身的版本保护与复制。备份前必须停止全部 Evidence 写入者，详见 [`../../../docs/operations/evidence-service-backup-restore.md`](../../../docs/operations/evidence-service-backup-restore.md)。

验证：

```bash
GOWORK=off go test ./...
GOWORK=off go vet ./...
TEST_DATABASE_URL="$DATABASE_URL" GOWORK=off go test -race -count=1 -tags=integration \
  ./internal/evidence ./internal/httpserver ./internal/storage ./internal/systembackup
```

公开控制面由 [`../../../contracts/evidence/v1/openapi.json`](../../../contracts/evidence/v1/openapi.json) 定义；服务端规格见 [`../../../docs/specs/evidence/m10-002-evidence-service.md`](../../../docs/specs/evidence/m10-002-evidence-service.md)。

D1 组合通过 `ENABLE_EVIDENCE` 启用本服务，控制面经 Nginx／Gateway，Local Provider 的短期 PUT／GET URL 经 Nginx 直达本服务。装配、开关和联合恢复边界见 [`../../../docs/specs/evidence/m10-003-d1-integration.md`](../../../docs/specs/evidence/m10-003-d1-integration.md)。

真实阿里云 OSS 首次验收使用 [`oss-validation.env.example`](./oss-validation.env.example)、[浏览器验收页](../../../acceptance/evidence-oss-browser/README.md)和[账号侧配置／证据清单](../../../docs/operations/aliyun-oss-first-validation.md)。复制后的 `.env.oss-validation` 被 Git 忽略；不得保留 `LOCAL_SIGNING_KEY`，也不得把 AccessKey 写入仓库。

OSS 模式区分两个 HTTPS Endpoint：`OSS_BROWSER_ENDPOINT` 返回给浏览器用于直传和短期签名下载，必须是公网可达地址；`OSS_SERVICE_ENDPOINT` 只供 Evidence 后端复核、读取和清理对象，同地域 ECS 应使用 OSS 内网地址。本机验收两者都使用公网地址。旧 `OSS_ENDPOINT` 只作为两个配置的兼容回退，新部署不要继续依赖它。
