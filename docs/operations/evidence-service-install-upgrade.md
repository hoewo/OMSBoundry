# Evidence Service 安装与升级

## A→B

文件服务只能从源码临时启动 → 可以作为独立进程或单机 Docker 组合安装、检查版本、升级和回退。

## 独立 Docker 安装

1. 复制 `backend/services/evidence-service/.env.example` 为 `.env`，替换所有 `replace-with-` 值；正式组合由 Nginx／Gateway 路由到本机 `127.0.0.1:8085`。
2. 本地 Provider 使用 `compose.standalone.yaml` 的独立 PostgreSQL 18 volume 和对象 volume；OSS Provider 删除对象 volume 配置并填写私有 Bucket、Role ARN、Region 和默认凭据链。
3. 运行 `docker compose -f compose.standalone.yaml up -d --build --wait`。
4. 检查 `curl --fail http://127.0.0.1:8085/health/ready` 和 `docker compose ... exec evidence evidence-server --version`。

数据库不暴露公网端口。Evidence 仅绑定本机端口，完整套件的唯一公网入口仍是 Nginx → Gateway → Evidence；OSS 浏览器数据面按短期 STS 直达 OSS。

## 二进制安装

发布包包含 `evidence-server`、`evidence-ops`、`evidence-backup`、配置示例和固定契约。主机需提供 PostgreSQL 18；执行备份恢复的主机还需匹配同一主版本的 `pg_dump`／`pg_restore`。

启动前以最小权限操作系统账号创建专用对象目录，设置配置文件权限为 `0600`，再运行 `evidence-ops cleanup` 的定时任务。三个二进制均支持 `--version`，发布归属必须与发布包 `RELEASE-METADATA` 一致。

## 升级

1. 先按备份文档停止写入并制作完整备份；
2. 校验发布包 SHA-256 和 `--version`；
3. 停止旧进程，替换二进制或镜像；
4. 启动一个实例执行 migration（`MIGRATION_MODE=auto`），其余实例使用 `verify`；
5. 验证 live、ready、创建／上传／完成／访问闭环，再恢复流量；
6. 数据库 migration 一旦成功不通过旧二进制“降级”；回退使用升级前的数据库和对象完整备份。

正式发布由仓库根目录运行：

```bash
RELEASE_PROVENANCE_MODE=official RELEASE_BUILT_AT=2026-09-21T12:00:00Z \
  make evidence-service-release VERSION=v0.1.0
```
