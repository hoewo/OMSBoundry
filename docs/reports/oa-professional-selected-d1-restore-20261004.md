# 原专业人员／奖金：所选 D1 组合冷恢复

2026-10-04 13:39 UTC 左右结束。实际 D1 脚本、当前候选镜像、真实 PostgreSQL 18、Nginx→Gateway→正式 IAM→正常 Workbench 的 **14 条检查通过、零页面错误**。这是本地 HTTP／build-mode 所选组合，不是完整 Suite、无源码 clean-host 发布、公网 TLS、历史主站升级或完整 OA 通过。Main 与 Grant 零变更。

问题 A→结果 B：人员和奖金分别恢复不能证明装配后还可继续 → 在一套隔离 D1 建立原生事实、联合备份，恢复到另一套新空目标，再用原页面、原身份／原命令继续。

## 实际链路

选择 IAM、Contract、People、Project、Evidence、Bonus、Workbench、Gateway，加 PostgreSQL／Nginx，共 10 个运行容器。D1 仍预建禁用应用的空数据库；没有声称它们不存在，也没有验证其业务。Contract 本批只有空业务库／元数据，不是合同旧事实升级证据。

1. 实际构建八镜像并运行 IAM 契约 preflight；独立 D1 启动及 `check`。
2. 全部业务夹具及精确 Grant 通过原生 API 创建，不用 SQL 灌名单；同人桥梁／道路两条专业分工。
3. 原奖金抽屉整批保存，两条 1.01 元×50% 的结果各 0.51 元。
4. Evidence 真正上传、完整性确认，保留 File ID／Version 和 45 字节摘要。
5. 实际 `d1 backup` 保存六所有者库、签名状态和本地文件字节；备份文件权限 0600，14 项选择／profile 有外层校验和。
6. 在活源尝试错 Bonus profile 的恢复：删卷前拒绝，容器 ID 和五业务库全表 canonical 摘要不变。
7. 停源但保留卷，在另一个无容器／无数据卷的任务目标执行真实 `d1 restore` 和 `check`。五业务数据库逐表计数／摘要完全相同；IAM 授权／稳定状态相同，故意不比较会话、工作负载 nonce 和运行审计。
8. 旧会话 401；原账号重新登录得到原 Subject 与精确 Grant。原奖金 command 201 重放，Bonus 全表零新增事实。
9. 恢复后的 Evidence 短期访问返回同 ID／Version 的原字节。
10. 正常原奖金名单刷新零写；原撤回确认追加一版，原历史抽屉可见两版。原审定抽屉继续添加分工，People 全表摘要不变。

实际查看了[恢复后人员配置](../../acceptance/oa-initiation-repair/evidence/professional-d1-20261004/restored-original-personnel.png)和[撤回两版历史](../../acceptance/oa-initiation-repair/evidence/professional-d1-20261004/restored-original-history.png)。前者也明确显示阶段、批复和审计未接通／未读取，没有把它们当零或完成。

[完整回执](../../acceptance/oa-initiation-repair/evidence/professional-d1-20261004/d1-browser.json)包含八个实际镜像 ID、备份摘要、前后逐表摘要、14 个场景和停止确认；[脚本／正常 bundle 摘要](../../acceptance/oa-initiation-repair/evidence/professional-d1-20261004/provenance.json)可复核。证据目录不复制 `.env`、密钥内容、数据库 dump、归档、Cookie 或 Token。私有 `/tmp/oa-professional-d1.lZITrm` 保留备份／日志，源与恢复卷保留；两套运行容器均为零。未删除用户或其他项目数据。

## 失败与限制

四次前序失败回执保留：Docker 自动网络池不足（app 及 one-shot 默认网）、驱动错误地用客户端 Bearer 穿过会剥离它的 Gateway、驱动等待原名单未显示的备注。分别改为只挑四段未占用的任务私网、走真实 Gateway 原生 scope-cookie 签发、等待真实两条名单。没有 prune 其他网络、放宽入口或业务校验；失败不计为最终通过。

复演需本机 Docker、Chrome 和足够资源；脚本只创建随机 task-labelled 环境，使用私有密码。不要将它指向 Main：

```sh
node acceptance/oa-initiation-repair/verify-professional-d1-restore.mjs \
  "$(mktemp -d /tmp/oa-professional-d1.XXXXXX)"
```

本批关闭 BF-105 **所选组合**联合恢复缺口，不关闭其他启用应用联合恢复、旧主站 schema 兼容发布、原 C 精确授权／公网点击、全角色／双租户、完整 D01／奖金／分类型阶段、OSS 或完整架构结论。下一功能是[原方案阶段逐动作闭环](../specs/assembly/oa-production-stage-original-operation-map.md)，不能以这一恢复证据代替页面业务交付。
