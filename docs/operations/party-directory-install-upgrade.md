# Party Directory 安装与升级

## 交付边界

Party Directory 独立拥有 Party 当前档案、稳定 ID 与不可变 revision。本包包含服务、迁移检查工具、全量备份恢复工具、环境变量样例、API 契约和数据模型；不包含 Contract、Money、Project 或 IAM 数据库，也不允许这些应用直接读取 Party 表。

## 运行前提

- Linux 或 macOS（发布包按目标平台构建）；
- PostgreSQL 18；
- 一套独占数据库和最小权限数据库账号；
- 32 字节以上随机 `CURSOR_HMAC_KEY`；
- 正式环境使用 IAM Provider；本地演示才使用 Template Provider；
- 公网入口由 Nginx 与 Gateway 提供，Party 服务端口只暴露在内部网络。

## 首次安装

1. 校验同目录 SHA-256 文件后解压发布包。
2. 复制 `config/party-directory.env.example` 到受权限保护的环境文件，并替换所有 `replace-with-` 值。
3. 创建空数据库，设置 `DATABASE_URL`。
4. 执行 `bin/party-directory-ops migrate`，再执行 `bin/party-directory-ops check`。
5. 用 systemd、Docker 或等价监督器启动 `bin/party-directory-server`。
6. 验证 `/health/live` 与 `/health/ready`；业务访问仍须携带受信凭证。

三个二进制都支持 `version` 或 `--version`，输出 release、commit、构建时间和目标平台。容器镜像以非 root `postgres` 用户运行，并自带与 PostgreSQL 18 同主版本的 `pg_dump`／`pg_restore`。

## 升级

1. 阅读变更与迁移说明，先在隔离恢复环境演练当前备份。
2. 创建完整备份并复制到服务器之外。
3. 停止 Party 服务；不要删除旧二进制和旧备份。
4. 部署新包，执行新版本的 `party-directory-ops migrate` 和 `check`。
5. 启动服务并验证健康检查、读取、写入和历史。
6. 回退必须使用升级前备份恢复到新的空数据库，再切换连接；不能在已升级库上覆盖安装旧 schema。

## 不变量

- 不重新生成 Party ID，不压平 revision，不覆盖历史；
- 不按名称自动合并主体；
- 不把消费应用角色或业务快照迁入 Party；
- 不从备份恢复到非空数据库；
- 数据库大版本不一致时拒绝恢复。
