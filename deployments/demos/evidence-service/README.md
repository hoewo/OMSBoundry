# Evidence Service 独立 Demo

这套入口只启动 Evidence Service、它自己的 PostgreSQL 和本地对象卷，用来验证文件业务边界；不启动 Nginx、Gateway、中央 IAM、统一工作台、OSS 或其他业务应用。

```bash
deployments/demos/evidence-service/ops/demo run
```

脚本会生成仅保存在本机、权限为 `0600` 的 `.env`，构建并启动两个容器，然后用模板身份自动完成：

1. Local Storage Provider 就绪，以及匿名、错误令牌和无效标识失败关闭；
2. 创建上传会话、创建命令幂等重放和同键异载荷冲突；
3. 签名上传地址防篡改、对象一次写入且不可覆盖；
4. 服务端大小、媒体类型和 SHA-256 完整性确认，以及完成命令幂等重放；
5. 稳定 File ID／Version 引用读取；
6. 短期签名访问、完整下载和 HTTP Range 下载；
7. 下载授权查询参数防篡改；
8. 摘要不一致完成失败、主动取消及取消重放；
9. 清理任务移除失败完成与已取消对象，同时保护已经完成的文件版本。

常用命令：

```bash
deployments/demos/evidence-service/ops/demo replay
deployments/demos/evidence-service/ops/demo status
deployments/demos/evidence-service/ops/demo logs evidence-service
deployments/demos/evidence-service/ops/demo credentials
deployments/demos/evidence-service/ops/demo cleanup
deployments/demos/evidence-service/ops/demo down
deployments/demos/evidence-service/ops/demo reset --confirm-destroy-data
```

数据库不暴露宿主机端口，HTTP 服务只绑定回环地址，对象只保存在专用 Docker Volume。每次 `run`／`replay` 都新建一份可用文件和两份清理候选，再要求清理任务实际删除候选。

本 Demo 刻意不连接阿里云 OSS，也不覆盖浏览器 OSS 直传、STS 或正式 IAM。它验证的是可替换 Local Storage Provider 下的 Evidence 核心契约；OSS 与正式入口链仍需在相应的集成验收中单独证明。
