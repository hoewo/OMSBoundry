# OMS 最后使用组织 v1.103 发布记录

日期：2026-11-13

事项：BF-336

状态：已发布并完成分层验收

## 发布结果

- 公网入口：`https://oatest.antuinfo.cn/`
- Suite／Identity：`v0.1.0-test.oms-v1.103`
- 源码提交：`a2e0734f20e0833b37f01f33be7e9cd83f1d731c`
- Identity 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/identity-access:v0.1.0-test.oms-v1.103`
- Identity 镜像摘要：`sha256:493363b292687a9343aa0755c0d995783b7990465b61072599c5279d4d6d3107`
- 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.103_linux_amd64.tar.gz`
- 发布包 SHA-256：`52752a8d209de3b357dbbad9876d41f283cb5cba66bdf96be1f551a12e41b415`
- 升级前备份：`/var/backups/omsbizfoundry/20261112T164331Z`

## 验证

- Identity `go test ./...`、`go vet ./...`、PostgreSQL 18.6 database／HTTP integration、P9 preflight、D1 composition 与发布包校验均通过。
- 正式 Identity schema 为 `000013_last_used_organization.sql`，独立 D1 check 通过；21/21 长运行容器运行，0 unhealthy。
- 线上 43 个登录账号全部有 last-used 指针；指针跨账号不匹配为 0，当前有 2 个多组织账号。
- 真实 Chrome 从 `feitianchengzi` 切换到 `OMSBizFoundry Auth58 验证环境` 后，页面首次数据投影随新组织完整重载；随后切回 `feitianchengzi`，原用户偏好已恢复。
- PostgreSQL 18.6 HTTP integration 使用新客户端证明：切换第二组织后重新登录直接进入第二组织；停用 last-used 后回退 primary 并在成功登录时自愈。
- Gateway readiness、Workbench 和公网根入口读取通过；根入口仍跳转到 `/workbench/?view=oms-v1`。

## 升级说明

- 标准升级在移除旧默认网络时遇到仍占用网络的一次性 Cost Outsourcing workload-key 端点；候选发布未删除卷，随后以同一候选执行 D1 up 完成收敛，并重新独立运行 D1 check。
- 该网络清理问题没有改变 Identity migration、镜像、业务数据或最终运行版本。

## 未外推

- 公网验收没有使用或记录账号密码，因此没有执行“退出后再次输入凭据登录”；该路径由真实 PostgreSQL HTTP integration 覆盖。
- 本次没有修改组织、成员、Grant、People 或业务 Owner 数据；只通过既有组织切换端口更新受控测试账号的 last-used 指针，并在验收后恢复原组织。
- 浏览器没有写入 Local Storage／Session Storage 组织偏好，业务 Owner 也没有获得该账号偏好。
