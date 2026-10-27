# OMS1 账号操作列排版验证与发布记录（BF-278）

日期：2026-10-27

## 问题与结果

账号与租户表格的操作列只有通用最小宽度，Flex 子项可以被继续压缩，导致“重置密码／设为管理员／停用”逐字换行，操作区与页面其他列明显失衡。

本次只修正 Unified Workbench 表现层：账号表增加专用 `oms-v1__identity-table`，最后一列表头明确为“操作”，列宽与最小宽度固定为 190px 并右对齐；动作容器禁止换行，按钮禁止收缩和文字换行。窄视口继续使用既有表格横向滚动，不改变按钮语义或权限。

## 变更边界

- 未修改 IAM、账号身份、组织、Session、Grant 或 Capability。
- 未修改 Owner、API、schema、业务事实或部署拓扑。
- 未执行“重置密码／设为管理员／停用”等真实账号动作。
- 本次公网发布只替换 Unified Workbench。

## 本地验证

以下检查已实际通过：

```sh
(cd frontend/oms-workbench && GOMAXPROCS=2 go test -p 1 ./internal/server)
(cd frontend/oms-workbench && GOMAXPROCS=2 go vet ./internal/server)
(cd frontend/oms-workbench && GOMAXPROCS=2 go test -p 1 ./...)
(cd frontend/oms-workbench && GOMAXPROCS=2 go vet ./...)
git diff --check
deployments/d1/ops/test-release-bundle dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.43_linux_amd64.tar.gz
(cd dist && shasum -a 256 -c omsbizfoundry-d1_v0.1.0-test.oms-v1.43_linux_amd64.tar.gz.sha256)
```

静态回归同时断言账号表专用类、“操作”表头、190px 专用列宽、动作容器不换行，以及按钮不收缩／不换行。

## 公网发布与验证

发布镜像：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.43@sha256:bb17534eee43ea047c7d40a48f229916e4cee53bd75502cf3e22932773727ee9
```

发布使用现有 D1 正式运维入口执行 `preflight-upgrade`、`upgrade` 和独立 `check`，证据目录为：

```text
/opt/omsbizfoundry/acceptance/oms-v1-account-actions-layout-v43-20261027
```

三项云助手命令均成功；附加只读断言确认：

- 19 个 `omsbizfoundry` 容器处于运行状态；
- unhealthy 容器为 0；
- Unified Workbench 使用上面的 v43 精确 digest；
- D1 `check` 通过。

随后以既有管理员 Session 刷新 `https://oatest.antuinfo.cn/workbench/?view=oms-v1` 并进入“账号与租户”：页面显示“操作”表头，普通用户行的“重置密码／设为管理员／停用”保持一行并整体右对齐，管理员行的“重置密码”同样右对齐。页面走查没有点击任何真实动作，没有修改账号、组织或业务数据。

## 追溯

- 代码提交：`6e003b5 fix(workbench): keep account actions aligned`
- Suite：`v0.1.0-test.oms-v1.43`
- 静态资源版本：`20261027.2`
