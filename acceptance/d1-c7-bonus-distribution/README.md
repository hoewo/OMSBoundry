# C7 Bonus Distribution D1 正式组合验收

该夹具使用独立 Compose project `omsd1c7` 和回环端口 `18180`，选择 Contract、Money、People、Project 与 Bonus Distribution。Party、Evidence、C2～C6 及 C8 必须缺席。

完整执行：

```sh
acceptance/d1-c7-bonus-distribution/run-isolated full
```

验收只通过正式 Nginx → Gateway → IAM → 应用入口读写 C7；初始化 SQL 只在各来源应用自己的数据库中建立已确认前置事实。夹具验证：

- Bonus 独立镜像、数据库、migration、工作负载身份、Suite Manifest、Workbench 和 Gateway 路由；
- 经营奖金读取 Contract Settlement，生产奖金读取 Money Receipt Set，均冻结 Project／Person／Assignment／来源 revision；
- 工作负载凭证审计包含四个目标应用和精确资源；
- 联合备份与空目标恢复保持 Pool、Allocation、来源快照、revision 与历史，恢复后可继续追加撤回；
- 取消选装后页面、路由、进程、migration、工作负载 key 服务和备份项均缺席，重新选装后数据仍在；
- C7 不包含 Money 应付建议，C8 不被夹带进本轮。

`full` 会删除并重建它自己的 `omsd1c7` 卷和 C7 专用本地证据目录。不得把 `D1_PROJECT_NAME` 指向共享或生产 Compose project。
