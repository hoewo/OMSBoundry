# 员工发票报销开放 API V1

状态：P7 本地实现；正式域名与 D1 发布留待 P9。

## 1. 安全边界

- 基地址：`https://<oms-host>/open-api/v1`。
- 凭证：`Authorization: ApiKey $OMS_API_KEY`。Key 只能从登录后的开放平台页面创建，完整 secret 仅展示一次。
- Tenant、账号和 Person 均从 Key 的当前 IAM 映射派生；请求不能自报或切换这些身份。
- 所有写请求必须带 `Idempotency-Key`。网络超时、`503` 或结果未知时只可复用原键，不得换键重写。
- Key、银行账号、发票正文和文件内容不得写入命令历史、共享终端输出或日志。

```sh
export OMS_BASE_URL='https://<oms-host>'
export OMS_API_KEY='从开放平台一次性复制的 Key'
```

可下载规范：`GET /open-api/v1/openapi.json`。同一 V1 只做向后兼容新增；破坏性变化使用新主版本。

## 2. 上传一张测试发票

首版允许 PDF、OFD、XML、JPG／JPEG、PNG；单文件不超过 20 MiB。每张报销单最多 20 个文件、合计不超过 100 MiB。示例数据必须使用虚构信息。

```sh
FILE='./test-invoice.pdf'
SIZE=$(wc -c <"$FILE" | tr -d ' ')
SHA256=$(shasum -a 256 "$FILE" | awk '{print $1}')

curl --fail --silent --show-error \
  -H "Authorization: ApiKey $OMS_API_KEY" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-upload-create-0001' \
  --data "{\"file_name\":\"test-invoice.pdf\",\"size_bytes\":$SIZE,\"media_type\":\"application/pdf\",\"sha256\":\"$SHA256\",\"client_context\":{\"purpose\":\"invoice\"}}" \
  "$OMS_BASE_URL/open-api/v1/evidence/upload-sessions" >upload-session.json

UPLOAD_URL=$(jq -er '.authorization.upload_url' upload-session.json)
SESSION_ID=$(jq -er '.session_id' upload-session.json)
curl --fail --silent --show-error -D upload-headers.txt \
  -X PUT -H 'Content-Type: application/pdf' --data-binary "@$FILE" "$UPLOAD_URL"
ETAG=$(awk 'BEGIN{IGNORECASE=1} /^etag:/ {gsub(/[\r\n\"]/,"",$2); print $2}' upload-headers.txt)

curl --fail --silent --show-error \
  -H "Authorization: ApiKey $OMS_API_KEY" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-upload-complete-0001' \
  --data "{\"observed_size_bytes\":$SIZE,\"observed_sha256\":\"$SHA256\",\"etag\":\"$ETAG\"}" \
  "$OMS_BASE_URL/open-api/v1/evidence/upload-sessions/$SESSION_ID/complete" >evidence.json
```

完成响应中的 `file_id` 与 `version` 是业务附件的稳定引用。校验或安全状态不是 `passed` 时不能提交报销。

## 3. 建立并提交本人报销

```sh
FILE_ID=$(jq -er '.file_id' evidence.json)
VERSION=$(jq -er '.version' evidence.json)

jq -n --arg file "$FILE_ID" --argjson version "$VERSION" '{
  title:"测试差旅发票报销",
  lines:[{
    business_date:"2026-10-25",
    category_key:"travel",
    category_name:"差旅费",
    category_revision:1,
    purpose:"客户现场交通费",
    amount_minor:12345,
    evidence:[{file_id:$file,version:$version,purpose:"invoice"}],
    invoice:{code:"TEST",number:"000001",issuer_name:"测试开票方",invoice_date:"2026-10-25",amount_minor:12345}
  }],
  payee:{bank_name:"测试银行",account_number:"6222020000000000"}
}' >reimbursement-create.json

curl --fail --silent --show-error \
  -H "Authorization: ApiKey $OMS_API_KEY" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-reimbursement-create-0001' \
  --data @reimbursement-create.json \
  "$OMS_BASE_URL/open-api/v1/reimbursements" >reimbursement.json

REQUEST_ID=$(jq -er '.id' reimbursement.json)
REVISION=$(jq -er '.revision' reimbursement.json)
curl --fail --silent --show-error \
  -H "Authorization: ApiKey $OMS_API_KEY" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-reimbursement-submit-0001' \
  --data "{\"expected_revision\":$REVISION,\"suspected_duplicate_acknowledged\":false}" \
  "$OMS_BASE_URL/open-api/v1/reimbursements/$REQUEST_ID/submit"
```

当前审批策略为 `not_required`，提交后进入待支付，但不会自动付款。只有授权财务人员在 OMS1 确认实际付款后，Money 才保存资金与核销事实。

## 4. 查询本人记录

```sh
curl --fail --silent --show-error \
  -H "Authorization: ApiKey $OMS_API_KEY" \
  "$OMS_BASE_URL/open-api/v1/reimbursements?status=paid&limit=20"

curl --fail --silent --show-error -D detail-headers.txt \
  -H "Authorization: ApiKey $OMS_API_KEY" \
  "$OMS_BASE_URL/open-api/v1/reimbursements/$REQUEST_ID" >detail.json
ETAG=$(awk 'BEGIN{IGNORECASE=1} /^etag:/ {gsub(/[\r\n]/,"",$2); print $2}' detail-headers.txt)
curl --silent --show-error -o /dev/null -w '%{http_code}\n' \
  -H "Authorization: ApiKey $OMS_API_KEY" -H "If-None-Match: $ETAG" \
  "$OMS_BASE_URL/open-api/v1/reimbursements/$REQUEST_ID"
```

列表支持 `status`、`submitted_from`、`submitted_to`、`updated_since`、`cursor`、`limit` 和 `archived=include`。默认每页 20 条，最大 100 条，排序固定为更新时间倒序加稳定 ID。

## 5. 草稿恢复、删除与错误恢复

- 连续 90 天未修改的草稿由运维任务归档；使用 `POST /reimbursements/{id}/restore` 恢复。
- 仅从未提交且没有成功或未知付款的草稿可删除；`DELETE /reimbursements/{id}` 的 JSON 必须为 `{"confirmation":"DELETE"}`。
- `401`：Key 无效、到期、撤销或账号／人员映射不再有效，不应自动重试。
- `403`：当前能力或资源授权不足，不应自动重试。
- `409`：revision、幂等内容、状态或强重复冲突；先查询原业务 ID，再决定后续动作。
- `413`／`422`：容量或字段／文件校验失败，修正输入后使用新的业务命令键。
- `429`：遵守 `Retry-After`。
- `503`、网络超时或 `UNKNOWN_OUTCOME`：只使用原 `Idempotency-Key` 有界重试或查询稳定 ID。

V1 不提供 Webhook、AI、CLI runtime、自动代付或他人报销查询。Webhook 设置仅为不可编辑占位。
