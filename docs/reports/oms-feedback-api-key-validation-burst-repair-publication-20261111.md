# OMS Feedback API Key 并发校验修复发布记录（BF-329，2026-11-11）

## 结论

OMS 的 Feedback V3 产品面已经正确升级到 3.0.4。截图中的“服务暂时不可用”发生在智能客服首屏加载咨询历史时：Feedback 公网网关已收到 `/workshop/v2/apikey/agent-chats/conversations`，但 auth-server 的 API Key 校验精确在 5 秒超时，Workshop 因而没有收到请求。

根因不是 SDK 版本、浏览器缓存、Key 撤销或 OpenHands 健康。旧 API Key 只保存 bcrypt verifier；auth-server 每次校验都读取所有有效 Key 并逐个比较。V3 打开时并发读取能力、会话和通知，四个同 Key 校验同时执行，使平时约 1.8～4.3 秒的单次冷校验放大到 12.8～23.2 秒，超过网关 5 秒预算并返回 500。

## 修复

- `nebula-auth@fea711f` 在 auth-server 内按 bearer SHA-256 合并同 Key 的在途校验，明文 Key 不进入缓存键或日志。
- 只缓存 30 秒成功结果；无效／未知 Key 不缓存，缓存期限不超过 Key 自身到期时间。
- 新建或删除 Key 清除成功缓存；每个真实验证请求仍更新 `last_used_at`。
- 网关继续逐请求执行认证并失败关闭；未放大 5 秒超时，未改变服务范围、API、数据库 schema、Feedback 事实所有权或 OpenHands 配置。
- OMS D1 仍遵守 BF-299 的一期 `FEEDBACK_AUTH_MODE=apiKey` 基线，没有在热修中静默切到 Session。

## 验证

本地：

- `go test -race ./internal/api/apikey/...` 通过；8 个同 Key 并发只调用一次底层加载。
- `go test -run '^$' ./...` 与 `go vet ./...` 通过。
- 完整 `go test ./...` 未登记通过：仓库既有注册集成测试在本机 Redis 不可用时对空 Verification Service 解引用并失败，与本次 API Key 包无关。

公网只读：

- 发布前日志复现四个同 Key 请求在 5 秒返回 500，auth-server 后台实际继续运行至 23 秒；Workshop 无对应请求。
- 发布后真实 OMS Key 的 8 路冷 `agent-capabilities` 请求全部 200，约 3.25 秒同时完成；8 路热请求全部 200，95～127ms。
- 截图对应的 `agent-chats/conversations` 以不创建数据的诊断作用域并发 8 路，全部返回 HTTP 200、`code=OK`、列表 envelope，94～125ms；Feedback 网关与 Workshop 均记录 200。
- `nebula-auth-server`、`todo-service`、`openhands-agent` 均为 running；Workshop 与 OpenHands health 为 healthy，restart count 为 0。

## 发布与回滚

- auth-server 代码提交：`fea711f`。
- Linux AMD64 运行镜像 ID：`sha256:1db4587b4a695079d98012f1848b0bd6d3a37c7ddf76f42cdfab62add26150cb`。
- 上传制品 SHA-256：`64fb8f2c08130646669ca62bdefddcb0b7f8a6ac7dd0f46812358ae92edbb44d`。
- 旧镜像回滚标签：`nebula-auth-auth-server:backup-20261111T144010Z`。
- 只重建 auth-server；Feedback Gateway、Workshop、OpenHands、数据库和 OMS D1 均未重启或迁移。

本轮没有发送智能客服消息、创建反馈、修改 OMS 业务事实或读取用户反馈正文。页面刷新后会重新发起已修复的只读首屏请求。
