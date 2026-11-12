# OMS Feedback V2 提交终点发布记录（2026-11-02）

## 结论

“产品反馈暂时不可用”不是 Feedback SDK 新页面导致的故障，而是线上组件版本不匹配：Unified Workbench `v0.1.0-test.oms-v1.71` 已在打开 V2 提交页前调用 `/gateway/feedback/submission-snapshot`，Gateway 仍为 `v0.1.0-test.oms-v1.63`，因此两次真实请求均返回 404。公网 SDK V2 页面与构建资源同时返回 200，近期 V3 隔离变更没有替换 `/sdk-v2/`。

本次只发布包含既有快照端口的 Gateway `v0.1.0-test.oms-v1.72`，没有修改 Feedback SDK。端口继续要求有效 IAM Session 与 CSRF，并只返回当前 Tenant＋Subject 派生匿名身份下的 Feedback ID 字符串集合与总数；API Key、IAM ID 和反馈内容不会返回浏览器。

## 本地验证

- `GOWORK=off go test ./internal/gateway -run 'Test.*Feedback|TestServer|Test.*Organization' -count=1`：通过；
- `GOWORK=off go test ./...`：通过；
- `GOWORK=off go vet ./...`：通过；
- Gateway 镜像为 Linux AMD64，摘要 `sha256:f04f5b0eecb0180ffa0cfa03832c2f8fa20672b8ae019184e972df485d493b67`。

## 公网发布

- 候选环境从 `/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.71.https.env` 复制，除 `D1_GATEWAY_IMAGE` 外的内容 SHA-256 同为 `579013e1c9d5896f834fd489221d2cdc909525b349bb7abf82e418e79b2477e8`；
- 候选预检与 IAM 契约解析通过；
- 正式 D1 upgrade 生成恢复点 `/var/backups/omsbizfoundry/20261102T123614Z`；
- 独立 D1 `check` 通过，Identity schema 12 及所有启用 Owner 检查为 current；
- 19/19 长运行容器 healthy；运行 Gateway 精确为 `v0.1.0-test.oms-v1.72@sha256:f04f5b0eecb0180ffa0cfa03832c2f8fa20672b8ae019184e972df485d493b67`；
- 公网未登录 POST `/gateway/feedback/submission-snapshot` 从部署前 404 变为 `403 csrf_failed`，证明请求已命中真实端口并进入预期 Session／CSRF 安全边界。
- 使用既有受保护测试管理员执行只读链路，CSRF 建立返回 200、登录返回 303、CSRF 更新返回 200、快照返回 200，响应形状包含 `feedback_ids` 与 `feedback_count`；凭据、Cookie、匿名 ID 和反馈 ID 均未写入报告或日志。

发布验收没有提交反馈、没有写入业务 Owner，也没有读取或记录 Feedback API Key。真实登录账号下一次提交时，Workbench 会在检测到新增 Feedback 后结束提交 iframe并显示成功；“我的反馈”入口保持可用。
