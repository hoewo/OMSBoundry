# Contract Ledger 独立 Demo

这是一套 Level A 业务演示，不是完整 OA 部署。它只启动 Contract Ledger 与该应用独占的 PostgreSQL，并使用应用正式支持的本地身份 Provider；不会启动 Nginx、Gateway、中心 IAM、Workbench 或其他业务应用。

## 一次运行

```sh
./deployments/demos/contract-ledger/ops/demo run
```

首次运行会在本目录生成权限为 `0600` 的 `.env`，构建并启动两个容器，然后通过真实网页表单完成以下合成业务：

1. 未登录访问与错误密码被拒绝；
2. 登记委托方、受托方，创建合同草稿并关联两方；
3. 激活合同，形成不可变 revision 1；
4. 登记金额增加 `5000.00 CNY` 的补充协议，形成 revision 2；
5. 受控更正录入标题，形成 revision 3；
6. 创建第二份正式合同并建立不可变合同关系；
7. 为主合同上传当前遗留本地 Evidence v1／v2，并核对最新版原始字节下载；
8. 登记终止文书，形成 `terminated / revision 4`，并拒绝终止后的普通变更；
9. 登记阶段结算和最终结算，重放最终结算命令不新增事实；
10. 在最终结算后操作性关闭合同，形成 `closed / revision 4`，并拒绝关闭后的新结算；
11. 重放补充协议命令，核对关系、Evidence、结算、查询、审计和 ZIP 账簿导出；
12. 核对版本化 reference-read，错误 reference token 被拒绝。

演示状态只保存资源 ID 和步骤标记，不直接写数据库。再次运行 `demo` 或 `replay` 会复用同一合同并再次证明命令重放安全：

```sh
./deployments/demos/contract-ledger/ops/demo replay
```

浏览器访问地址默认为 <http://127.0.0.1:28081/contracts>。查看本地演示账号：

```sh
./deployments/demos/contract-ledger/ops/demo credentials
```

这些凭据仅用于监听 `127.0.0.1` 的合成数据演示，不能复制到共享、测试或生产环境。

## 运行管理与清理

```sh
./deployments/demos/contract-ledger/ops/demo status
./deployments/demos/contract-ledger/ops/demo logs contract-ledger
./deployments/demos/contract-ledger/ops/demo down
./deployments/demos/contract-ledger/ops/demo reset --confirm-destroy-data
```

`down` 保留本 Demo 的数据库和文件；`reset` 只删除 compose project `omsbizfoundry-contract-ledger-demo` 的数据库卷、Evidence 卷和本目录 `.runtime`。它不会删除其他应用的数据。`.env` 保留，便于下次使用同一账号；不再需要时可在 reset 后手动删除。

## 当前边界

本 Demo 已把 Demo Validation 2 为 Contract Ledger 规定的能力簇放入同一条可重复操作者故事：主体、草稿与合同方、激活、补充／更正、关系、遗留本地 Evidence v1／v2、终止、阶段／最终结算、操作性关闭、查询、审计、导出、模板鉴权、失败关闭和幂等重放。

这里演示的文件能力是 Contract Ledger 的既有本地实现，只用于如实覆盖当前产品能力；它已明确进入待迁移范围，不改变“文件技术事实由独立 Evidence Service 拥有”的目标边界。单合同恢复仍由既有发布／备份恢复验收负责，不重复塞入业务操作者故事；正式 IAM、Nginx、Gateway、公司 Ubuntu 和公网入口属于其他验证轴。
