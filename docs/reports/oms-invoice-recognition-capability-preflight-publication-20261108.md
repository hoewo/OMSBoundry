# 发票识别授权预检与失败终态发布验证（2026-11-08）

## 结论

BF-314 已发布。截图中的“卡住”并非 OCR 或 Evidence Service 处理超时，而是浏览器在创建 Evidence 上传会话前，向 Gateway 申请 `evidence-service` Capability 时收到 403；旧前端把该授权失败包装成了 `Evidence service is unreachable.`，同时没有结束识别进度态。

Unified Workbench `v0.1.0-test.oms-v1.84` 现会在打开识别进度页前依次预检 Evidence 上传与发票识别 Capability。403 会返回付款申请表单，并明确提示当前组织缺少对应权限；上传或识别过程中的其他失败也会终止转圈并展示真实错误。此次修复没有复制其他组织的 Grant，也没有提升任何账号权限。

## 发布制品

- Workbench 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.84`
- 镜像摘要：`sha256:31e3e079772087511342e619715926cdab933cca4a7d6420e1736f3a967d891f`
- D1 发布包：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.84_linux_amd64.tar.gz`
- D1 发布包 SHA-256：`5242f9db6726f11e06b2c8feccb6709b67e0bdd20664898b66a681c7b2b6f086`
- 升级前备份：`/var/backups/omsbizfoundry/20261108T111419Z`

## 验证

- Workbench JavaScript 语法检查通过。
- Workforce 与 Finance Owner Node 测试共 9 项通过。
- Workbench Go 测试与 `go vet` 通过。
- D1 发布包测试通过，正式升级前合约预检返回 `contracts-valid`。
- 升级前备份包含 21 项校验和；正式升级和升级后独立 D1 检查均通过。
- 公网环境 21/21 个长运行容器健康且运行中。
- 公网 `oms-phase1-workforce.js?v=20261108.1` 与本地候选 SHA-256 均为 `66bc4f7f6cf419c93449da5e11515a9ef2e82d8870bbeea21a61c928f53f3af4`。

## 权限现状与未执行事项

- 复现账号在当前选择的组织中是普通账号，且没有有效 Grant；它在另一个组织中的管理员身份不得跨组织继承。
- 当前选择的组织只有系统管理员，没有组织管理员；系统管理员不会自动成为业务账号。
- 若要在当前组织完成识别，需要由系统管理员显式把目标账号设为该组织管理员，或授予 Evidence 上传与发票识别所需的最小 Capability；也可以切换到该账号已有权限的组织。
- 本次没有修改账号角色、Grant 或成员关联，也没有实际上传文件或写入付款申请。
