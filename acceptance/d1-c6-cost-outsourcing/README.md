# C6 Cost & Outsourcing D1 正式组合验收

该夹具使用独立 Compose project 和端口 `18170`，选择 People、Party、Project、Contract、Evidence 与 Cost & Outsourcing。Money、Business Engagement、Billing、Production、Results 和 C7 必须缺席。

完整执行：

```sh
acceptance/d1-c6-cost-outsourcing/run-isolated full
```

它通过正式 Nginx → Gateway → IAM → 应用链完成：

- Workbench 与 C6 人类页面、Gateway 路由和相对 API 入口；
- active Project、Party Directory、Evidence、成本、外协和评分；
- Project 范围凭证、`allowed_actions`、金额脱敏和越权拒绝；
- Evidence／Party 停机零写入与恢复；
- 旧 Contract 本地 Party 与 Directory Party 不一致时失败关闭，不以共享表或隐式映射绕过；
- 联合备份、空目标恢复、canonical digest／history 保持和恢复后继续写入；
- 取消选装后 Workbench、路由、进程、migration 与备份项缺席，重新选装后原数据仍在。

该夹具会删除并重建它自己的 `omsd1c6` 卷；不得把 `D1_PROJECT_NAME` 指向共享或生产 Compose project。测试凭证不能复制到真实环境。
