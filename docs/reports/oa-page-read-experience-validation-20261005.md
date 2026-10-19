# OA 页面读取体验修复与公网验证（2026-10-05）

## 问题与可验证结果

问题 A：经营／生产页面每次切换栏目都会清空当前内容并重新换票、读取多个事实所有者。用户只点一次栏目，也会看到空白等待并产生一组网络请求；这不是小应用独立边界的必然代价，而是 Workbench 页面读取编排不合理。

可验证结果 B：首次进入显示与既有 OA 视觉连续的骨架，占位期间不闪成空白；30 秒内返回已读栏目立即显示内存投影且不发网络请求；投影过期时保留旧内容并在后台刷新；同一读取中的快速重复动作共用请求；显式“刷新”仍绕过页面缓存到事实所有者读取。

## 实现

- 新增只保存页面读取投影和时间戳的内存缓存，不保存 Credential，不成为业务事实所有者。
- React 经营栏目、生产人员与阶段资料，以及现有原生经营／生产／甲方信息适配器统一采用 `fresh → stale-while-refresh → explicit refresh` 规则。
- 首次读取使用轻量 Skeleton；已有内容刷新时保留内容并显示更新状态；后台失败保留最后成功投影并明确警告。
- 项目变更、页面重载或账户会话变化不会跨上下文复用投影；显式刷新始终到源系统。
- Workbench 仍通过 Gateway／正式 IAM 取得有界能力，事实所有者仍逐请求作最终授权。页面缓存不跨越应用数据所有权，也不替代 BF-107 的 30 秒能力租约。

## 自动检查

实际执行并通过：

- frontend Node：158／158；
- Workbench server Node：226／226；
- TypeScript `npm run check`；
- Vite `npm run build`；
- Unified Workbench `go test ./...`。

新增测试覆盖首次读取、fresh 返回零读、显式刷新到源、过期后台更新、后台失败保留旧投影和并发读取合并。

## 公网浏览器验收

使用正式 `https://oatest.antuinfo.cn`、既有管理员登录与验收项目 `3b4c44d3-7b33-4a29-add3-3f6211ca893e`，实际运行：

```sh
node acceptance/oa-initiation-repair/verify-page-read-experience-browser.mjs /tmp/oa-page-read-browser.<mktemp后缀>
```

结果：

- 登录、深链和本轮检查合计 60 个请求；
- 返回已读“支付信息”：0 个请求；
- 显式刷新“支付信息”：4 个请求，分别为一次有界授权、Project 入口、Money 工作区和精确 Project 读取；
- 从生产信息返回已读经营信息：0 个请求；
- HTTP 429：0；业务写请求：0；页面错误：0。

结构化回执见 [page-read-experience-20261005.json](../../acceptance/oa-initiation-repair/evidence/page-read-experience-20261005.json)，复演脚本见 [verify-page-read-experience-browser.mjs](../../acceptance/oa-initiation-repair/verify-page-read-experience-browser.mjs)。

## 发布范围

只滚动 Unified Workbench：

`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench@sha256:0dd14307b36810e2e3c692287eb75286734845ef06978b5ac371567bf0c38755`

发布后 17／17 容器 running／healthy，其他 16 个容器的 ID 与镜像不变；没有数据库、migration、IAM Grant 或 Gateway 变更。

## 不负责范围

本批只关闭“切页清空重读、返回仍等待、重复请求”的体验与编排问题。它不补齐尚未实现的 OA 表单、命令、权限角色、恢复矩阵或全部连续旅程，也不把内存投影称为业务缓存平台。后续 OA 功能仍须按原组件和事实所有者逐项完成；新页面应直接复用本批读取规则，不能再以空白等待作为默认交互。
