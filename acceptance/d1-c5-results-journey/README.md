# C5 Approval & Audit Results 独立进程验收

该夹具启动一套隔离 D1，真实经过 Nginx → Gateway → IAM → People／Project／Contract／Evidence／Results，验证：

- Project 立项、精确 Contract Resource Link、Evidence 上传；
- Evidence／Contract 停服时 Results 零写入，同一 Command 恢复后只成功一次；
- approval → audit → Project C5 completion confirmation；
- Results Grant 撤销、重新签发、金额脱敏和 Project Scope；
- 确认后纠正 approval，Project 只读评估显示来源 drift；
- Results 停服时评估为 unavailable，新确认失败关闭，已成功 Command 可本地重放。

执行：

```sh
acceptance/d1-c5-results-journey/run-isolated run
acceptance/d1-c5-results-journey/run-isolated down
```

证据默认写入 `deployments/d1/.runtime/c5-journey-evidence/`。该验收只证明 C5 检查点，不宣称整个 OA Project 已完成。
