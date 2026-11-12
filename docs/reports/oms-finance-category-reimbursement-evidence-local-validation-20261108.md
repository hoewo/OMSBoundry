# OMS 财务类别、报销附件与折算入口本地验证（2026-11-08）

## 结论

BF-315 已完成本地候选：费用类别弹窗改为紧凑清单；报销附件在提交 Finance 前转换为严格 Evidence 引用；折算政策入口及无政策时的生成摘要动作暂时隐藏。候选尚未部署，未执行线上业务写入。

## 问题与原因

- 费用类别把每项渲染成输入框加浏览器原生复选框，重复“平台类别／启用”标签，造成纵向占用和视觉噪声。
- 线上只读 Nginx 日志显示，报销上传发票时 Evidence upload session 创建返回 201、完成返回 200，紧随其后的 `POST /internal-finance/v1/api/v1/finance-requests` 返回 400。
- Evidence Bridge 返回 `display_name`、`content_sha256`、`mime_type` 等上传回执字段；Finance HTTP 端口启用 `json.Decoder.DisallowUnknownFields()`，只接受 `file_id`、`version`、`purpose`、`sha256`、`media_type`、`size_bytes` 等 `EvidenceRef` 字段。Workbench 直接传回执导致 Finance 在保存申请前拒绝请求，所以弹窗不关闭，列表也没有记录。
- 折算政策的当前产品说明不足，用户无法从入口理解其作用和适用口径。

## 实现

- 平台费用类别显示名称、稳定 key 和启停开关；租户自定义类别保留可编辑名称；新增自定义类别独立成组。
- 新增 `financeEvidenceRef()`，在组合层显式映射摘要与媒体类型并移除 Finance 不接受的上传字段。
- 保存仍遵守“Finance 返回成功后才关闭弹窗并重新加载列表”；失败保留当前弹窗与错误信息。
- 保留登记缺勤和请假，隐藏折算政策配置；没有有效政策时同时隐藏生成摘要动作并说明登记功能仍可用。

## 验证

```text
node --check: finance owner / workforce 通过
node --test: 11 / 11 通过
Workbench Go test: 通过
Workbench Go vet: 通过
headless Chrome 静态视觉复验: 通过
```

静态复验确认平台类别不再出现大复选框和重复标签，八项类别在单个可扫描列表中展示，自定义类别与保存操作仍可见。

## 未完成

- 尚未生成或发布 D1 release bundle。
- 尚未在线上用专门测试数据执行上传发票、保存报销、自动关闭和列表／详情回读。
- 未修改 Attendance、People、Payroll、Evidence 或 Finance Owner 的 schema、Grant 与业务事实。
