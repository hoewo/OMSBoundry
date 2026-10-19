# Evidence OSS 浏览器验收页

只用于真实阿里云 OSS 外部验收。页面消费正式 `@omsbizfoundry/file-transfer-core`，固定运行在已加入 Bucket CORS 的 `http://127.0.0.1:5173`，不是产品 UI。

最小根因：**单元测试只能证明契约和适配器行为 → 在真实浏览器、真实 STS 和真实私有 Bucket 上留下 multipart 直传与服务端确认的直接证据。**

## 安全边界

- RAM 用户长期 AccessKey 只存在于 Evidence Service 进程环境；
- Vite 本地代理从 `backend/services/evidence-service/.env.oss-validation` 读取模板访问令牌，只把它添加到 `/evidence` 控制面代理请求；
- 浏览器不会收到 RAM 长期 AccessKey，页面也不会显示或记录完整 STS 临时凭证、签名 URL 或模板令牌；
- 文件字节由 File Transfer Core 直接发送到 OSS，不经过 Vite 代理或 Evidence 控制面；
- 页面只展示 File ID、Version、服务端 SHA-256、媒体类型和 Object Key 等非秘密验收结果。

## 当前本机拓扑

本次验收并非所有进程都在 Docker：Evidence Service 与 PostgreSQL 由 `compose.standalone.yaml` 运行在 Docker 中，Vite 验收页是宿主机进程。Evidence 端口只映射到宿主机 `127.0.0.1:8085`，PostgreSQL 只在 Compose 私有网络内；浏览器经公网 HTTPS 直连 OSS，当前本机 Evidence 也经公网访问 STS／OSS。

同地域 ECS 生产部署时，浏览器上传和短期签名下载仍必须使用 `OSS_BROWSER_ENDPOINT` 公网地址；Evidence 后端复核、读取和清理应改用 `OSS_SERVICE_ENDPOINT` 内网地址。内网 Endpoint 不能返回给用户浏览器。

## 启动

先启动 OSS 模式的 Evidence Service：

```bash
cd backend/services/evidence-service
EVIDENCE_ENV_FILE=.env.oss-validation \
  docker compose --env-file .env.oss-validation \
  -f compose.standalone.yaml up -d --build --wait
```

然后构建公共组件并启动验收页：

```bash
cd packages/file-transfer-core
npm ci --ignore-scripts --no-audit --no-fund
npm run build

cd ../../acceptance/evidence-oss-browser
npm ci --ignore-scripts --no-audit --no-fund
npm run dev
```

浏览器访问 `http://127.0.0.1:5173`。可以选择真实文件，也可以生成 1 MiB 或 150 MiB 合成样本。150 MiB 样本大于服务端返回的 20 MiB multipart 阈值，会使用 8 MiB 分片和 3 路并行。

如果环境文件位于其他路径，用相对于仓库根目录的 `EVIDENCE_ENV_FILE` 覆盖；如果 Evidence 不在默认的 `http://127.0.0.1:8085`，设置 `EVIDENCE_BASE_URL`。两项只影响本地开发服务器。

## 静态验证

```bash
npm run check
```

页面和 Evidence 已启动后，可额外验证临时 STS 的单 Object Key 边界：

```bash
npm run verify:object-boundary
```

该脚本故意用本次临时 STS 写入相邻错误 Key，只有 OSS 返回 `403`／`AccessDenied` 才通过；随后取消测试会话。脚本不输出 STS 凭证，运行后仍应执行 Evidence cleanup 清理已取消会话。

验收结论必须以真实上传、Evidence complete、短期读取和负例结果为准；页面能构建不等于云端链路已通过。
