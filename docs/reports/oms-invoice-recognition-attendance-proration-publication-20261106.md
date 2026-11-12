# 发票识别与考勤折算公网发布验证（2026-11-06）

> 后续状态：BF-309 已在同日补齐 JPEG／PNG、PDF 首页面本地 OCR 和原图位置标注，取代本文“图片／PDF 需要外部 Provider”的当前能力结论；领域所有权与人工确认边界不变。见[图片识别与人工校对发布验证](./oms-invoice-recognition-visual-review-publication-20261106.md)。

## 结论

BF-307／BF-308 已从本地领域核心完成到 PostgreSQL、HTTP／IAM、跨 Owner 端口、Workbench、D1 装配、备份恢复和公网只读复验。最终公网 Suite 为 `v0.1.0-test.oms-v1.77`；两个新 Owner 与 Payroll 变更使用 v1.76 镜像，v1.77 只修复 Workbench 核心脚本的不可变缓存版本，使新页面标题不再显示内部路由名。

本轮没有在线创建考勤、请假、工资或发票业务事实。登录后浏览器只验证新入口、当前组织授权读取、空状态和失败关闭提示。

## 发票识别纵向闭环

- 新增独立 `invoice-recognition` Go／PostgreSQL Owner、migration、Repository、HTTP、IAM Capability Manifest、工作负载密钥与 D1 数据库。
- 输入只接受 Evidence File ID／Version／SHA-256／MIME／大小；服务重新读取并核对精确版本，最大 20 MiB。
- 电子发票 XML 由本地 Parser 真实解析；图片、PDF、OFD 通过可替换 HTTP AI Provider，Provider 身份、模型版本和字段置信度进入候选 revision。
- 候选保持 `needs_review`，人工可更正并确认；确认交付内容摘要和规范化发票代码＋号码两个重复线索，不直接创建 Billing、报销或付款事实。
- Workbench 提供 Evidence 上传、识别列表、人工校对、确认与已确认只读详情。
- 当前公网未配置外部 AI Provider URL／Secret，因此 XML 可用，图片／PDF／OFD 稳定失败关闭；没有用假结果掩盖该外部依赖。

## 考勤与工资折算纵向闭环

- 新增独立 `workforce-attendance` Go／PostgreSQL Owner、migration、Repository、HTTP、IAM Capability Manifest、工作负载密钥与 D1 数据库。
- 月度政策显式选择自然日或明确工作日；考勤和请假使用 `10000` 整数单位表示一日，带薪比例使用整数基点。
- People & Groups 增加追加式任职区间投影；Attendance 通过版本化端口读取同一 Person 在当月的多个任职区间、间隔和 revision，而不复制人员事实。
- 月度摘要冻结人员快照、政策 revision／digest、标准／任职／无薪假／缺勤／计薪单位；单日扣减和月度等式失败关闭。
- Payroll 增加 Attendance 摘要连接器和 migration。经办人显式应用后，Payroll 复验摘要，以大整数中间值和半入法计算 `fixed salary × payable units ÷ standard units`，追加工资行 revision，不反向修改 Attendance 或 People。
- Workbench 提供政策、缺勤、请假、摘要计算和“应用到工资”操作。

## 验证

- 两个契约模块、两个新 Owner、Payroll、People、Workbench 相关 Go test／vet 通过。
- Workforce Attendance、Invoice Recognition 与 Payroll Attendance Execution 的真实 PostgreSQL 集成测试通过，覆盖 Tenant 隔离、幂等／冲突、revision、恢复后复读和精确计算。
- Workbench Node 回归 `294/294` 通过；服务端测试通过；D1 release bundle 回归通过。
- 全仓 Go 回归只保留一条与本轮无关的既有基线断言：`reference_oa_permissions_test.go` 仍期望 49 条，但提交前既有目录为 48 条。本轮未以放宽权限或篡改断言掩盖该差异。

## 发布证据

- Payroll：`v0.1.0-test.oms-v1.76@sha256:5f4a2ba8c1249d037a05ed02bda9cb4073cb3462f32784c08620fe381ac6df1a`
- Workforce Attendance：`v0.1.0-test.oms-v1.76@sha256:a902cde7d2d95c50ae703ebe3f1dcdfdf78e23a399d36592188cd7577da3f639`
- Invoice Recognition：`v0.1.0-test.oms-v1.76@sha256:95cefceb11e3dab74483ad763134ff37559e58654568546a1bde1cfd1c41c61c`
- Unified Workbench：`v0.1.0-test.oms-v1.77@sha256:08b48c5f8fe2d750edbf666fa884558201293a7eb0ca02f5a889a4b8a3f51fa9`
- 最终升级前恢复点：`/var/backups/omsbizfoundry/20261105T164650Z`，`checksums.sha256` 21／21 通过。
- 最终 D1 `check` 通过；21／21 长运行容器 healthy；运行 Workbench Image ID 与发布摘要一致。
- Chrome 使用既有当前组织管理员 Session 复验：“公司内部管理”下显示“考勤与请假”“发票识别”；两个页面标题、说明、操作入口和 Owner 空集合读取正常，无授权或 Console 表面错误。

## 仍需外部输入

图片／PDF／OFD 的真实 OCR／AI 需要确定可撤换 Provider、提供部署侧 URL／Secret 并用脱敏发票样本做准确率、超时、限流、费用和数据合规验收。该依赖不阻塞 XML 识别、人工确认、考勤折算、D1 恢复和现有公网入口，但在完成前不得宣称全格式 AI 识别可用。
