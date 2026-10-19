# OMS 一期详情与选择控件统一验证记录

日期：2026-10-14  
决策：BF-156  
范围：Unified Workbench OMS 一期 UI；不改变业务 Owner、公开 API、数据库、schema、Grant 或业务事实

## 本轮目标

解决两类一致性问题：一是应收／应付、回款／支付等详情仍以原始标签和值纵向堆叠，阅读层次不清；二是筛选、分页和表单中的原生 `select` 在不同系统与浏览器中可能显示为黑色系统菜单，弹层锚点也不稳定。

## 实现结果

- 新增统一详情结构：当前状态、核心金额、关键事实、备注、附件逐层展开；
- 应收／应付详情统一展示总额、已发生、未发生三段金额；
- 回款／支付详情统一展示本次金额、业务日期、往来主体和备注，并继续复用 Evidence 凭证区；
- 票据详情以统一状态条进入更正表单，合同详情沿用专用摘要但保持同一视觉层级；
- 纯查看详情隐藏提交按钮，底部只保留“关闭”；
- 所有静态与动态 `select` 统一增强为白色组合框和页面顶层弹层；
- 弹层按触发器边界实时定位，支持左右边缘避让、上下空间判断、键盘选择、Esc／Tab 关闭、必填、禁用和异步选项更新；
- 原生 `select` 仍保留在 DOM 中作为值、校验和原有 `change` 事件来源，没有重写业务提交逻辑。

## 本地检查

实际通过：

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`
- `go test ./frontend/oms-workbench/internal/server`
- `go test ./frontend/oms-workbench/...`
- 定向 `git diff --check`

本地浏览器实际打开经营总览期间选择器，弹层为白色并位于触发器下方。触发器靠近视口右侧时，弹层左边界自动向内调整，没有超出视口，也没有调用系统原生黑色菜单。

## 发布与公网复核

Unified Workbench 不可变镜像：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.10
sha256:30e4a2a4b7e418eefe8f678cced85e283dff53d50494b58807a672d3d76cf112
```

D1 发布包构建和包检查通过，服务器发布目录为：

```text
/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.oms-v1.10_linux_amd64
```

只重建 Unified Workbench，正式 D1 `check` 通过，17/17 容器 healthy。

公网 Chrome 在刷新后实际确认：

- 经营总览的期间选择器显示统一白色弹层，选中项为绿色弱底和勾选状态；
- 弹层与触发器对齐，靠近屏幕右侧时自动向内避让；
- “应付支付 → 应付管理 → 微信1111 → 详情”显示分层状态卡、三段金额、往来主体、事实来源和备注；
- 纯查看详情底部只显示关闭动作。

## 边界

本轮没有新增或修改任何业务命令、Owner、跨系统写链、数据库表、migration、Grant 或金额计算规则。统一控件只是 Workbench 表现层；底层原生表单值和既有版本化 Owner 调用保持不变。
