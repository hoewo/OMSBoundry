# C8 Project → Evidence 最小运行组合

这个验收只启动一个临时 PostgreSQL 容器，以及本地 IAM、Evidence、Project 三个进程，用于验证 C8 第一条真实运行链。它不会启动 Nginx、Gateway、Workbench 或完整 D1。

运行前必须保证 `docker ps` 没有运行中的容器：

```bash
acceptance/c8-material-composition/run-isolated
```

脚本会验证真实 IAM 工作负载换票、精确 File／Project Scope、direct Evidence Link、stale revision、Evidence 停机／恢复、无下载权和存储位置不泄漏。退出时会停止全部本地进程、删除唯一临时容器，并要求 Docker 回到 0 个运行容器。

需要做受控浏览器检查时，同时设置 `C8_BROWSER_FIXTURE` 与 `C8_BROWSER_RELEASE`。脚本在全部后端断言通过后，把仅供本次本机检查使用的 Project URL、Project ID 和短期凭证写入权限为 `0600` 的临时 fixture，并等待 release 文件出现；退出 trap 会删除两者。该模式仍只运行一个 PostgreSQL 容器，不能把临时凭证复制到报告或仓库。

证据默认写入 `deployments/d1/.runtime/c8-material-composition-evidence/`；该目录不是发布输入。
