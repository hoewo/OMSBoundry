# D1 Evidence OSS 云档与元数据保护

问题 A：ECS 实例角色对同机有出口的进程可见，默认 internal 网络又不能获取身份。结果 B：只有 Evidence 获得专属出口与实例身份，其他应用仍在内部网络；Docker 每次启动前先恢复拒绝边界。

## 先配置，再启动

默认 `D1_EVIDENCE_STORAGE_PROVIDER=local` 不变。云档通过 `compose.evidence-oss.yaml` 显式启用 `hybrid` 或 `aliyun-oss`；已有 Local 历史应使用 hybrid。`EVIDENCE_PRIMARY_STORAGE_PROVIDER` 只决定新会话。完整变量见 D1 `.env.example`；云档保留 IAM、本地历史签名键、独立数据库和固定镜像摘要，不需要长期 AccessKey。

必须提前提供带 `oms.component=evidence-egress`（或明确验收标签）的 external bridge，只允许 Evidence 成员。不要将 Nginx、Gateway 或整个 app 网络加入它。浏览器使用公网 `EVIDENCE_OSS_BROWSER_ENDPOINT`，服务器使用同地域 internal Endpoint；绑定 Files 自有域名时 `EVIDENCE_OSS_BROWSER_CNAME=true`。实例凭据 SDK 明确强制 IMDSv2。

以 root 对精确网络运行 `ops/evidence-imds-guard apply` 和 `status`。它只在 `DOCKER-USER` 首项保护 `100.100.100.200/32`，允许验证过的专用 bridge／subnet，其余容器拒绝；不改主机 OUTPUT、SSH 或其他目的地址。D1 云档 up／apply／upgrade／restore／backup／check／preflight-upgrade 在发生变化前都要求 Guard status 通过。陌生链、过宽边界或前置绕过规则均拒绝，不能靠清空别人的 iptables 恢复。

## 启动持久化模板

发布包提供 `systemd/evidence-imds-guard.conf` 与非密钥环境模板。安装前核对 `docker network inspect` 的实际 bridge 和 IPv4 subnet；不要根据示例猜测。将 guard 安装到 `/usr/local/libexec/omsbizfoundry/evidence-imds-guard`（root 所有、0755），将两项非敏感环境保存为 `/etc/omsbizfoundry/evidence-imds-guard.env`（root 所有、0600），将 drop-in 放到 `/etc/systemd/system/docker.service.d/evidence-imds-guard.conf`（root 所有、0644），父目录均不允许普通用户写入，然后 `systemctl daemon-reload`。

Drop-in 使用 `ExecStartPre`，每次 Docker 启动都运行 guard `boot`，不依赖 Docker API 或已存在网络接口。失效配置会阻止 Docker 启动，因此**安装与实际 restart／reboot 必须安排维护窗口，保留 SSH 和原始配置备份**。不要为了启动成功关闭保护；应先修复精确网络配置或陌生规则。

模板不等于已经持久化。安装后必须依次取得证据：Docker restart → 所有声明服务 healthy → Guard 首项与专属网络核对 → Evidence 官方 SDK 正向 → Nginx／Gateway／IAM／业务应用 IMDS 拒绝；主机 reboot 再重复。改网络或防火墙也要重验。启动前规则链的设计依据见 [Docker 官方防火墙说明](https://docs.docker.com/engine/network/firewall-iptables/)。

## 切换与恢复门禁

先运行固定镜像候选 `preflight-upgrade`，再做主环境冷备份与离机摘要；一次只切换一个风险边界。不要直接将旧 Local-only 服务改为 OSS。主库、历史 File ID／Version 与业务引用必须保持；回退用兼容镜像改 Primary，不回退旧二进制。

混合 v2 ZIP 保存本地字节与外部引用，不含 OSS 字节；另行实施全量对象保护、保留与恢复。完成正式公网浏览器上传／取消／错误摘要／续传／权限拒绝后才能关闭切换门禁。

2026-10-01 Auth58 `test.44` 候选云档渲染、IAM 契约预检、配置／运行时错档拒绝、新 netns 的 boot／幂等／失败拒绝、单份合成对象恢复，以及真实上传 STS 到期拒绝与刷新已通过。生产 Docker／主机重启、drop-in 激活、基础身份长时自动刷新和完整对象保护尚未执行；正式主环境仍 Local。见 [实测报告](../reports/auth58-evidence-oss-adapter-validation-20261001.md)。
