# OMS Open API live test CLI

这个 CLI 用于以真实 API Key 检查 `/open-api/v1/` 链路。它不会把 Key、认证 Header、签名上传 URL、完整工资／收款账号或请求正文写进固定证据。

## 安全冒烟

推荐把 Key 放进仅本人可读的临时文件：

```sh
umask 077
printf '%s\n' '粘贴一次性显示的完整 Key' > /tmp/oms-open-api.key
chmod 600 /tmp/oms-open-api.key

./acceptance/open-api-live/oms-open-api-test smoke \
  --key-file /tmp/oms-open-api.key
```

`smoke` 会验证 Key、`/me` 和设置，探测账号是否已关联当前有效成员，然后创建一个 Evidence 上传会话并立即以 `client_shutdown` 终止。它不上传字节，不创建报销单。`check` 是完全只读版本。

若结果中的 `person_mapping` 为 `missing`，说明 Key 和组织账号链路有效，但本人报销尚不能运行。CLI 会同时返回 `repair_path`；管理员打开该入口并明确选择成员完成关联后，再用 `--expect-person active` 复测。系统不会按姓名自动匹配账号和成员。

CI 也可以用进程环境变量传入：

```sh
OMS_API_KEY="$OMS_API_KEY" \
  ./acceptance/open-api-live/oms-open-api-test check --expect-person either
```

不要把 Key 直接写进命令行参数、脚本、Git、聊天截图或固定测试报告。

## 真实业务写入

`draft` 会上传 CLI 生成的测试 PDF 并留下真实报销草稿；`submit` 还会把它提交为待支付。这两个模式不会自动删除或冲销数据，必须显式确认：

```sh
umask 077
printf '%s\n' '测试收款账号' > /tmp/oms-payee-account
chmod 600 /tmp/oms-payee-account

./acceptance/open-api-live/oms-open-api-test draft \
  --key-file /tmp/oms-open-api.key \
  --expect-person active \
  --confirm-business-write \
  --bank-name '测试银行' \
  --account-file /tmp/oms-payee-account
```

提交为待支付还必须追加 `--confirm-submit`。完整参数见 `--help`。

## 本地回归

```sh
node acceptance/open-api-live/oms-open-api-test.test.mjs
```

回归使用短生命周期回环服务覆盖缺成员映射、安全终止上传、显式提交业务写入、确认门禁以及输出不泄露 Key、账号和签名 URL；它不替代公网真实链路。
