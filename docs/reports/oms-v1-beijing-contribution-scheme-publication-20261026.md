# OMS1 北京五险一金参考方案公网发布记录（2026-10-26）

## 结论

BF-273 已发布到 `https://oatest.antuinfo.cn/`。新 Tenant 的工资页可以读取 Payroll 自有的“北京 2026 五险一金参考方案（最低可选档）”，并从“设置比例”查看来源、复制为本公司方案后调整。此次发布不替客户确认实际工伤行业档、公积金比例或其他申报参数。

## 发布制品

- 源码分支：`codex/company-internal-management`
- 源码提交：`37f666185a20b714dd706955595f09a10f3bda02`
- D1 Suite：`v0.1.0-test.oms-v1.37`
- Linux AMD64 bundle：`omsbizfoundry-d1_v0.1.0-test.oms-v1.37_linux_amd64.tar.gz`
- Payroll 镜像：`sha256:635b0d097da3fea6bd8d8e376452591b1fc0435b6afeab6f6e7d7dd0bb323f16`
- Unified Workbench 镜像：`sha256:c4fd181e002835b1502a3d584611d1bcd458fc1994798dddeae2becd0b98606b`

Payroll 与 Workbench 使用已经推送且按 digest 固定的 `v0.1.0-test.oms-v1.34` 镜像；Suite 版本是 `v0.1.0-test.oms-v1.37`。这两个版本号分别表示组件制品标签和完整装配发布，不得混为同一版本号。

## 升级与恢复点

- 上传后同时核对外层 SHA-256 与 bundle 内部 checksum。
- 发布前 `config`、`images`、`prepare-images`、`preflight-upgrade` 均通过。
- 升级前联合备份：`/var/backups/omsbizfoundry/20261026T132424Z`。
- 升级过程先收敛公网入口并完成选定数据库与 Evidence 备份，再保留卷替换容器。
- 备份 checksum 在升级后重新验证通过。
- 发布使用的临时 Registry 登录已退出；服务器 Docker 凭证中不保留本轮临时登录。

## 发布后实际验证

- D1 `check` 通过；Payroll 报告 schema 为 `000004_beijing_2026_reference_scheme.sql`。
- 19/19 个长期运行容器处于 healthy。
- Payroll 与 Workbench 运行镜像 ID 分别与上述 digest 一致。
- 真实 Payroll PostgreSQL 只读核对得到标准方案标题、`CN-BJ`、生效月 `2026-07`、revision `1` 和 6 个组件。
- `https://oatest.antuinfo.cn/` 返回 HTTPS 登录重定向，`https://oatest.antuinfo.cn/workbench/?view=oms-v1` 返回 200，HTTP 根入口按预期 308 到 HTTPS。
- 公网 Payroll 页面脚本返回 200，摘要为 `7b22a5db131173fb51b26563075b5f00e94e6b23f8586f93668fecf6ebb794e4`，实际包含“确认北京参考方案”“以此创建本公司方案”和 Tenant 自定义方案 API 标记。
- 公网 CSS 返回 200、正确 content type 和 immutable cache；发布后独立 D1 `check` 再次通过。

本轮发布后验证没有复制租户方案、生成工资表或写入其他业务数据。登录后的实际租户页面点击和创建公司自定义方案留给用户体验；标准方案的 migration、精确计算、Tenant 隔离与复制／revision 行为已由[本地验证报告](./oms-v1-beijing-contribution-scheme-validation-20261026.md)覆盖。

## 边界

- 该参考方案不是法律、税务或社保申报建议。
- 工伤 0.2% 与公积金 5% 只是最低可选预填值，租户必须核对后再复制使用。
- 本次只发布 Payroll migration、计算表达与 Workbench 设置入口，不建立全国政策平台，不自动判断企业适用档位。
- P9 全组合真实旅程、Feedback 外部回复、账号激活真实送达和其他此前标记“未部署”的阶段不因本次 BF-273 发布自动转为完成。
