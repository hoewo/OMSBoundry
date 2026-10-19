# M10-003 Evidence Service D1 装配

日期：2026-09-22  
状态：已实现并验证

## A→B

**Evidence Service 已能独立运行，但完整组合还不能统一启用、访问、备份和恢复文件能力 → 在不改变应用数据所有权的前提下，把 Evidence 作为可选技术应用接入 D1 全链路。**

## 装配边界

- Evidence 使用独立 `evidence_service` 数据库账号和数据库、独立 migration、独立对象卷及独立备份格式；其他应用不读写其内部表或对象目录。
- `ENABLE_EVIDENCE` 同时控制 Compose profile、Suite Manifest 控制面路由和 Nginx 本地数据面路由；关闭不删除数据库或对象卷。
- IAM 加载 Evidence Capability Manifest，控制面使用 audience `evidence-service` 的 `view`／`upload` 能力凭证；Gateway 只完成入口路由与基础验签，Evidence 作最终授权。
- Evidence 是技术服务，Suite Manifest 通过 `show_in_workbench: false` 让 Workbench 不把它显示成最终用户业务菜单；业务应用将来通过自己的交互使用它。
- 当前 D1 使用 Local Provider。控制面固定走 `Browser → Nginx → Gateway → Evidence`；PUT／GET 字节路径使用 Evidence 签发、绑定单一对象和短期限的 URL，由 Nginx 直接转给 Evidence，不经过 Gateway。
- 本地数据面入口删除客户端 Authorization 与全部内部身份 Header，关闭请求／响应缓冲，允许独立的大文件上限，并且访问日志只记录 `$uri`、不记录带 Token 的查询串。
- OSS Provider 的目标路径仍是浏览器直连 OSS；本项不把本地 Nginx 数据面误称为 OSS 直传。

## 开关不变量

启用时：

1. 生成 Evidence Suite 路由和 Nginx 本地数据路由；
2. 创建／迁移独立数据库并启动 Evidence；
3. `d1 check` 验证 schema 与 Provider；
4. 首管授权脚本按 Capability Manifest 授予能力。

关闭时：

1. Gateway 控制面路由返回 `404`；
2. Nginx 不再生成本地上传／下载特殊路由，数据面同样返回 `404`；
3. Evidence 进程停止；
4. PostgreSQL 数据库和 `evidence-data` 卷保留；
5. Workbench 在启用和关闭状态都不显示 Evidence 技术入口。

## 备份与恢复

- D1 备份前停止 Evidence 写入进程，再调用 Evidence 冷备份生成 `evidence-service.zip`；无停写证明不能生成成功备份。
- 本地模式归档同时包含 Evidence 数据库和被已确认 Version 引用的对象字节，并逐项校验大小与 SHA-256。
- D1 外层 `checksums.sha256` 覆盖 Evidence 归档；恢复只进入全新 Compose 卷，先校验外层摘要，再恢复 Evidence 空数据库和新对象根。
- 恢复后重新执行 Evidence schema／Provider 检查，并验证原 File ID、Version、摘要和对象字节；应用仍可继续创建新 Version 事实。
- OSS 模式的对象耐久性和跨环境恢复仍由 Bucket 版本保护、复制与逐对象校验负责，不由本地 ZIP 假装覆盖。

## 验收证据

2026-09-21 在两个隔离 Compose 项目上完成实际验证：

- 全应用 D1 从空卷创建五个独立数据库／账号，且只有 Nginx 发布宿主端口；
- 一次 IAM 登录和 Evidence 单 Audience 授权后，经 Gateway 创建 PDF Upload Session，经 Nginx 数据面无 Suite Cookie PUT 字节，再完成服务端大小／媒体类型／SHA-256 确认；
- Access Grant 后通过 Range GET 取得 `%PDF`，证明短期 URL、本地数据面和断点读取有效；
- Workbench 不显示 Evidence 技术菜单；Nginx 数据日志没有记录 Token 查询串；
- 停止 Evidence 时其控制面返回 `503`，Contract、Money Flow 与 People & Groups 继续 `200`，重启后恢复；
- 联合备份生成 `evidence-service.zip`；删除测试项目全部卷后恢复，原 File ID、Version、SHA-256 与对象字节一致，恢复后继续写入通过；
- `ENABLE_EVIDENCE=false` 后控制面和本地数据面均为 `404`、容器停止、对象卷保留；重新开启后原 Version 仍存在；
- 额外的空卷首次启动回归验证 PostgreSQL provisioning 在真实网络连接边界等待，避免新 Compose 网络的短暂就绪竞态导致首次安装失败。

持续验收入口：

```bash
D1_PROJECT_NAME=isolated D1_ENV_FILE=/absolute/path/to/test.env \
  D1_ACCEPTANCE_DISRUPTIVE=yes acceptance/d1-delivery/verify-running
```

破坏性恢复必须使用独立项目和显式 `D1_CONFIRM_EMPTY_RESTORE=yes`；不能在生产营业环境直接运行。

## 本项明确不负责

- 不迁移 Contract Ledger 当前内嵌 Evidence；该迁移必须另行保持旧业务引用、历史审计和原字节。
- 本切片自身不宣称真实阿里云 Bucket、CORS 或 150 MiB 浏览器 multipart 已通过；这些能力后来于 2026-09-22 在独立 OSS 验收组合中通过核心链，不能倒算成 D1 公网入口证据。
- 不给 Evidence 增加业务分类、Project ID、合同含义或最终用户独立菜单。
- 不证明 D2～D4 已部署，也不改变 D1 单点、非高可用的性质。
