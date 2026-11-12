# OMS Feedback V3 产品面竞态纠错与发布（BF-328，2026-11-11）

## 纠错结论

用户截图明确显示“FEEDBACK SDK／反馈建议／提交反馈”，这是 V2 兼容表单，不是 V3 统一支持面。此前以 `sdk-v3` URL、3.0.2 channel、`supported_contracts: ['v3']` 和 configured ACK 判断升级完成，验收结论不充分。

正式 `sdk-v3` 现已提升到 3.0.4。实际可见产品面为“帮助与反馈”，包含“智能客服／我的反馈”两个 Tab；从 OMS 父域的 Chrome 验证中，旧“反馈建议”标题计数为 0。OMS Suite／Workbench 保持 v1.98，不需要再次部署。

## 根因与修复

SDK 在 React App 挂载前安装 bridge 并向父窗口发送 ready。OMS 收到后很快发送 configure；如果配置完成早于 `App.useEffect` 安装 `feedback-sdk:configured` 监听器，配置事实已经写入内存，但边沿事件丢失。App 初次检查时因尚无传输鉴权选择 V2，之后没有再读取当前配置，于是永久停留在 V2 表单，即使 ready 已声明 V3。

3.0.4 在监听安装后立即再次调用当前配置判断：事件先到则由复核读取，事件后到则由监听器处理，两种顺序都收敛到 V3。该修复不改变 V2 HTTP、API Key／Session、OMS Gateway、IAM 或 Feedback 事实所有权。

## 验证与发布

- Feedback SDK App 专项 3/3；全套 23 个测试文件 142/142；release 脚本 8/8；V3 production build 通过。
- source commit：`693e173474a9a2dd7723e6f08ffec95bfa9b23d8`。
- 3.0.4 artifact SHA-256：`e44d4f8e1388fd78dc12ff66080d38d64521d517217fe6b9dcee0536ca12f260`。
- JavaScript SHA-256：`23a04e5d8599f3945362e857613718b54277df248ec9880f5491a4de01b6897a`；CSS SHA-256：`efd052452f484e6190479cd7d3b3d30c74cd03fe5bad176d29b53ac025c068a3`；公网与本地一致。
- 3.0.4 显式构建 `https://oatest.antuinfo.cn` 父域白名单，先提升 `sdk-v3-canary` 并做产品面检查，再提升 `v3-stable/sdk-v3`；minimum／recommended 均为 3.0.4。
- 正式入口 Chrome 结果：`sdkVersion=3.0.4`、`supportedContracts=['v3']`、“帮助与反馈／智能客服／我的反馈”可见、旧“反馈建议”标题计数 0。

3.0.3 曾上传到不可变 canary，但灰度检查发现构建未携带 OMS 父域白名单，因此没有提升 stable；按不可变发布约束保留该历史制品，改由 3.0.4 取代。浏览器中已加载的 3.0.2 iframe 不会被 channel 原地替换，须刷新 OMS 页面或新开页面后再打开“帮助与反馈”。本轮使用非秘密测试配置验证产品面，没有提交反馈或写入 OMS 业务事实。
