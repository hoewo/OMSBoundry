# OMS 发票购销方名称字段边界 v1.110 发布记录

日期：2026-11-17（GMT+8）  
事项：BF-342  
状态：已发布；新上传样本待用户复验

## 发布结果

- 公网入口：`https://oatest.antuinfo.cn/`
- Suite 制品：`v0.1.0-test.oms-v1.110`
- 源码提交：`fe5d2b53731053723ac130dfb4cecb6a0336bcde`
- Invoice Recognition 镜像：`sha256:aba641ab9fc1cd7479fe3040d36ab26d74321b6dc194fba0690c9b13b9700385`
- 发布包 SHA-256：`41583d1c27e887aae88452ab3f3465ecfc117f44b89776a0e6c8f637f252b03f`
- 升级前联合备份：`/var/backups/omsbizfoundry/20261117T135421Z`

## 问题与结果

问题 A：Tesseract 将票面纵排“购买方／销售方”和横排“名称”合并为同一行，并插入空格或识别成错字，旧规则因找不到连续“名称”而把 `藿名称:`、`墨芎名称:` 一并写入公司名称。  
可验证结果 B：购销方名称只取 OCR 行内第一个半角或全角冒号右侧；候选值和标注文本不再包含角色／标签噪声。

## 实际证据

- 生产只读查询确认 Proposal `3b1a570a-2dec-4cbf-8275-a638f3bf4270` 于 2026-11-17 21:35（GMT+8）保存了截图所示错误值；查询未修改业务数据。
- 自动回归使用同一票号与 `墨芎 名 称 :`／`藿 名 称 :` 形态，验证左右两栏分别得到“北京飞天橙子科技有限公司”与“河北雄安臻玺餐饮服务有限公司”，区域文本也不含冒号。
- Invoice Recognition `go test ./...` 与 `go vet ./...` 已通过。
- v1.110 release bundle 外层与包内校验通过，候选 IAM preflight 通过；正式 D1 `check` 通过，21/21 长运行容器均 healthy。
- 运行中 Invoice Recognition Image ID 与上述 Registry 摘要一致；公网根入口正常返回登录跳转。
- 标准 `upgrade` 在备份完成后命中既有 Cost／Outsourcing 一次性工作负载容器占用旧默认网络；数据卷未删除，使用同一候选执行 `d1 up` 后完整收敛。

## 边界

- 已存在 Proposal 和确认历史保持原值，不静默重写；重新上传才使用新 Provider 规则。
- 本项不改变 Proposal schema、Evidence 文件、IAM／Gateway、Finance／Billing 或任何正式发票与资金事实。
- 本轮没有代用户重新上传、确认或删除发票；新规则的真实公网样本结果不从自动回归外推。
