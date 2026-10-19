# Money Flow Ledger 独立 Demo

这套入口只启动 Money Flow Ledger 和它自己的 PostgreSQL，用来验证业务架构，不启动 Nginx、Gateway、中央 IAM、统一工作台或其他业务应用。

```bash
deployments/demos/money-flow-ledger/ops/demo run
```

脚本会生成仅保存在本机、权限为 `0600` 的 `.env`，构建并启动两个容器，然后以真实浏览器表单协议自动完成：

1. 本地模板身份的未登录、错误密码与错误 reference token 失败关闭；
2. 两个业务主体；
3. 120,000 元应收义务确认、增加 5,000 元形成 revision 2，并为当前 125,000 元余额设置预计回款日；
4. 首笔 40,000 元收款与错误核销、Allocation Reversal、正确替代核销；
5. 第二笔 30,000 元收款与核销，形成两条有效 Allocation；
6. 不进入经营收支和义务核销的 5,000 元内部账户转账；
7. 55,000 元减免、零余额受控关闭，以及四个系统记录时点的历史余额重放；
8. 错误收款的完整冲销，以及已核销收款不能直接冲销；
9. 关闭后禁止调额、幂等命令重放、账龄排除、现金排期历史、经营汇总、受保护 ZIP 导出与 reference-read。

常用命令：

```bash
deployments/demos/money-flow-ledger/ops/demo replay
deployments/demos/money-flow-ledger/ops/demo status
deployments/demos/money-flow-ledger/ops/demo logs money-flow-ledger
deployments/demos/money-flow-ledger/ops/demo credentials
deployments/demos/money-flow-ledger/ops/demo down
deployments/demos/money-flow-ledger/ops/demo reset --confirm-destroy-data
```

默认浏览器入口是 <http://127.0.0.1:28083/operating-summary>。数据库不暴露宿主机端口，应用只绑定回环地址。

当前 Demo 已覆盖 Demo Validation 2 为 Money Flow Ledger 规定的全部能力簇。坏账、票据、真实退款和通用导入仍按既有业务边界审计结论延期；它们不是本 Demo 用猜测流程补齐的功能，也不影响本轮 Level A 能力簇判定。
