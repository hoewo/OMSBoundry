# C4 Project 生产资料消费者隔离验证

A→B：生产资料已有版本化来源端口 → Project 经类型化 HTTP 引用资料，仅写自己的 Link，异常不伪装成功，关联在空目标恢复后保持且可继续追加。

- 单个私有 PostgreSQL 18 + 有界测试进程，无发布端口；不启动 restore20、不改主站。
- 7→8 migration 保持七张业务表、旧 Design Link、ID 和 revision；重复 migrate 无额外变化。
- Project 真实 PostgreSQL + Production/Evidence 显式 HTTP 测试提供者。消费者请求精确 native Action / Work Item 或 File Scope，但不把测试凭证当作真实 IAM 证据。
- 错项目零写入、同 Command 在来源离线时复读、GET 零事实变化、来源／文件版本变化、403／503、空资料、恢复后追加 unlink。
- 自有数据库 custom dump → 空库 restore，校验全表 canonical、schema 8、ID/revision/Link；保留诊断与 dump，结束停止探针。

脚本固定使用此前创建且停止的 source 专项私有 PostgreSQL，先核对名字、用途标签、镜像、网络、限额和无公网端口。`verify-ecs.py` 仅用于 Auth58 专项；数据库密码不输出。执行前上传当前两个 Linux AMD64 测试二进制及脚本到它声明的专用目录。

本批不证明完整 Nginx→Gateway→IAM→Project→Production→Evidence 连续链，也不证明主站升级、OSS、浏览器或 C5/C6 来源。实际结果见专项报告；未经运行不得记为通过。
