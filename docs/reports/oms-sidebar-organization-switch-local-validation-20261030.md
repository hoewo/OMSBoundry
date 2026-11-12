# OMS 侧边栏组织切换本地验证记录

日期：2026-10-30
事项：BF-295
结论：本地实现与回归验证通过；未发布公网

## 可验证结果

- 桌面侧边栏底部在账号卡片之前显示“当前组织”和组织名称。
- 只有一个启用组织时，入口不可点且不显示展开箭头；有多个启用组织时才启用原位展开。
- 组织清单向上展开，显示组织名称、组织内级别和当前项。点击外部或 Escape 会关闭。
- 切换复用 `POST /api/v1/session/organization`；成功后整页重载，由 Gateway 的 BF-294 逻辑清除旧组织 Capability Cookie。
- 没有新增侧边栏一级业务页，也没有修改 IAM、People、Owner、schema 或 Grant。

## 实际执行

```text
node --check frontend/oms-workbench/internal/server/oms_phase1_identity.js
  PASS

node --test frontend/oms-workbench/internal/server/oms_phase1_organization_switch.test.mjs
  PASS

(cd frontend/oms-workbench && GOWORK=off go test ./internal/server)
  PASS

git diff --check
  PASS
```

本记录仅声明本地候选。未构建或发布新 Workbench 镜像，未在公网账号上执行组织切换。
