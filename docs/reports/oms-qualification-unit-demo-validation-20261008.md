# OMS 资质单位参考 Demo v1 验证记录

日期：2026-10-08  
对应决策：BF-122  
规格：[OMS 资质单位参考 Demo v1](../specs/assembly/oms-qualification-unit-demo-v1.md)

## 结论

`https://oatest.antuinfo.cn/oms/` 已提供资质单位视角的只读 OMS 参考 Demo。公网入口、页面资源、核心交互、桌面／手机布局和 D1 两组件滚动更新均取得实际证据。该结论只说明 Demo 可供评审，不表示 OMS 产品设计、真实数据接入、权限、业务命令或 OA 完整体架构验证已经完成。

## 实现范围

- Gateway 新增 `/oms`、`/oms/` 的 GET／HEAD 入口，303 到 Workbench OA 预览视图；
- Workbench 新增 `oms-reference` 页面模板、样式和浏览器内交互；
- 页面包含运营总览、合同经营、收付计划、协作资源、经营待办五个工作区；
- 页面显式展示“演示数据”“不读取或写入真实业务”和原型来源 `27c6148`；
- 未新增数据库、migration、IAM Capability／Grant、业务 API 或跨应用写入。

## 本地自动化与浏览器验证

实际运行：

```text
cd backend/platform/gateway && GOWORK=off go test -count=1 ./... && GOWORK=off go vet ./...
cd frontend/oms-workbench && GOWORK=off go test -count=1 ./... && GOWORK=off go vet ./...
node --check frontend/oms-workbench/internal/server/reference_oms_demo.js
make architecture-boundary-static-audit
git diff --check
```

结果均为通过；静态边界结果为 15 个应用、0 跨应用 import、0 跨应用 replace、0 跨数据库 SQL 机制。格式化后重新交叉编译的两个二进制 SHA-256 与已部署候选逐字一致。浏览器使用独立 Workbench 进程实际检查：

- 初始合同 5 条，选择“履约中”后 2 条；
- “东港片区综合管廊施工服务”详情抽屉可打开；
- 可切换到收付计划工作区；
- 控制台无 warning／error；
- 390×844 手机视口的 `clientWidth=375`、`scrollWidth=375`，无横向溢出；
- 桌面和手机视觉层级均实际截图检查。

## D1 候选与上线验证

候选使用已部署镜像为运行时基底，只替换静态编译的 Linux/amd64 服务二进制：

| 组件 | 候选镜像 ID | 旧镜像摘要 |
| --- | --- | --- |
| Unified Workbench | `sha256:93bdeda70ecae83b96d65c44b2c353771bec51b8f25faaad87e17a9b1c80f4db` | `sha256:be69a0a853608042f29e728876830f852917220c2aed07fb7b0c42297bd604e1` |
| Gateway | `sha256:cb66b7bb1562e1e0c572abfd37440f8d5264bdef279bc9b7f461c5c0b9c69e4a` | `sha256:0ceb122595bbf9055986ec658ed07b34110baa0a4edcd4fc28fb3f4595a9531d` |

上线前候选容器实际验证：两个 `/health/ready` 均通过，Workbench 页面模板和 JavaScript 可读，Gateway 返回精确跳转 `/workbench/?preview=oa&view=oms-reference`。

上线后实际验证结果：

```text
public_oms_redirect=303
public_oms_page=200
public_oms_assets=200
workbench_regression=200
containers=17/17 healthy; non-target ids unchanged
```

运维证据与回滚 override 保存在 ECS：

```text
/opt/omsbizfoundry/acceptance/oms-qualification-demo-20261008a/
```

只有 `omsbizfoundry-unified-workbench-1` 和 `omsbizfoundry-gateway-1` 被重建；其余 15 个容器 ID 不变。旧镜像摘要已写入 `compose.rollback.json`。没有执行数据库、Grant、账号、业务事实或 Evidence 文件变更。

## 已知限制

- 当前代码工作树不是干净可追溯发布基线，因此镜像是评审候选，不是正式版本发布；
- 公网传输层使用命令行实际验证；交互和响应式浏览器验证使用同一候选二进制的本地独立进程完成；
- Demo 使用静态样例，不验证真实 Owner API、IAM allowed actions、并发、幂等或恢复后的 OMS 业务继续；
- OA 当前完成度继续按 BF-120 为 `insufficient-evidence`，本切片不关闭 BF-121 或 C1～C8 的任何缺口。
