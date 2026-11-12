# OMS Feedback V3 父域发布门禁与 3.0.6 发布记录

日期：2026-11-13

事项：BF-337

状态：已发布并完成真实 OMS 复验

## 故障与恢复

Feedback SDK V3.0.5 的发布构建没有注入 `https://oatest.antuinfo.cn`。iframe 能发出 ready，但 SDK 因父域不在允许清单而拒绝 OMS 的 configure；Workbench 等待 configured 超时后显示“产品反馈暂时不可用”。Gateway `/gateway/feedback/config` 和提交快照均正常，因此故障不是 API Key、Session 或 OMS Owner 不可用。

稳定通道先回退到已验证 3.0.4 恢复服务；修复版完成后再提升到 3.0.6。没有修改 OMS 业务数据、Gateway 配置、长期 API Key 或 Grant。

## 永久门禁

- Feedback 源码仓库：`feitianchengzi/arckit`，分支 `codex/fix-feedback-agent-unavailable`。
- 修复提交：`d27e3d9fff286b2efcba09c4b1f1be76e7c42fc2`。
- `config/sdk-release-policy.json` 版本化声明 canary／stable 所需父域。
- managed release build 从 policy 注入 `VITE_SDK_PARENT_ORIGINS`，并在构建后扫描真实 JavaScript；缺一项直接失败。
- 不可变 `release.json` 记录 `parent_origins`，同版本元数据不同仍禁止覆盖。
- `promote` 在上传 loader 或 channel 前按目标 policy 校验；缺失父域或旧 manifest 没有声明都会失败关闭。

## 制品与验证

- SDK 版本：3.0.6。
- release artifact SHA-256：`caaee79a994e923f6927160236509714983ba9b73a0c4ad1fa71858e4e95ef32`。
- JavaScript SHA-256：`82eabedd4eb50428341c2ffd8601e4c90ac9c5f0aaa2407daae33165a959d258`。
- SDK 测试：23 文件、143/143 通过。
- release 测试：11/11 通过。
- TypeScript 与 Vite V3 production build：通过，并确认四个正式父域都出现在远端 JavaScript 与 release manifest。
- 旧 3.0.4 release 在新 promote 门禁下因没有 `parent_origins` 被明确拒绝，证明门禁生效。
- 3.0.6 先提升 `v3-canary`，远端 channel 与 release 摘要一致后再提升 `v3-stable`。
- 真实 Chrome 完整刷新 OMS，点击“帮助与反馈 → 智能客服”；iframe URL 为 `/sdk-releases/3.0.6/submit?embed=web`，统一面板、智能客服、我的反馈、历史和输入区均正常显示，不再出现 OMS 反馈不可用提示。

## 未外推

- 本次验证没有发送新咨询、提交反馈或修改 Feedback 记录。
- 该门禁只解决 SDK 父域制品完整性；Feedback 上游服务、模型和网络仍须各自健康检查。
