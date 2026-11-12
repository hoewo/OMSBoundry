# OMS 登录／注册认证界面品牌清理发布记录

日期：2026-10-30
状态：已发布并完成公网复验

## 问题 A → 结果 B

OMS 登录与邀请注册页面仍显示旧“京宏”文字水印、建筑／Logo 拼贴和公司标题 → 公网认证界面只保留中性的 OMS 产品标题、说明和认证表单，不再加载或渲染旧客户品牌装饰。

## 实现范围

- OMS 登录与邀请注册共用 `login-page--oms` 中性认证画布。
- OMS 浏览器标题固定为“OMS 经营管理系统”，不再附加旧公司名称。
- OMS 未认证页面不再输出 `login-page__watermark` 或 `assets/jinghong-logo.webp` 标记。
- 旧版工作台的客户资料、旧入口品牌和相关业务演示不在本次范围内；本次只清理 OMS 登录／注册认证表面。
- 登录、邀请预览、本人注册、既有账号验证加入、IAM CSRF 和 Session 契约均未改变。

## 本地验证

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `node --test oms_phase1_registration_invitation.test.mjs`：3/3 通过。
- 本地浏览器视觉检查：页面保留 OMS 标题和认证卡片，背景为中性浅色层次，不再显示文字水印、建筑或 Logo 拼贴。
- `git diff --check`：通过。

## 发布制品与恢复点

- Unified Workbench：`v0.1.0-test.oms-v1.67`
- 镜像摘要：`sha256:026027e80f91b037ad260b1fc0980d681f9960be837b7ca2dc372cf8076ce8b3`
- 平台：`linux/amd64`
- 构建元数据：`deployments/d1/.runtime/build-metadata/v67-workbench.json`
- D1 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.67.https.env`
- 升级前联合备份：`/var/backups/omsbizfoundry/20261030T153032Z`
- `checksums.sha256`：19/19 项通过。

## 公网复验

1. 候选环境从 v1.66 派生，只替换 `D1_WORKBENCH_IMAGE`；排除该行后的配置 SHA-256 一致。
2. 候选 IAM／装配契约预检返回 `contracts-valid`。
3. 正式 upgrade 成功，随后独立 D1 `check` 退出 `0`。
4. 19/19 个正式长运行容器健康；Workbench 运行 Image ID 与 Registry 摘要一致，环境标签指向 v1.67。
5. `https://oatest.antuinfo.cn/workbench/?view=oms-v1` 返回 `200`。
6. 公网未认证 HTML 标题为“OMS 经营管理系统”，包含 `login-page--oms`，不含 `login-page__watermark`、登录标题中的“北京京宏”或 `assets/jinghong-logo.webp`。

## 数据边界

本次是纯 Workbench 表现层清理。发布和验证没有创建邀请、账号、成员关联或业务事实，也没有修改 IAM、People、Owner、schema、Grant、Session 或数据库数据。
