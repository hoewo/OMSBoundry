# 财务重复发票处置与草稿审计删除发布验证（BF-324，2026-11-09）

## 问题与结果

问题 A：重复发票在提交时虽然会被 Finance 阻断，但用户缺少一个明确的处置面；重复草稿也无法从日常列表移除，容易反复干扰经办，同时不能为了“删掉”而破坏已提交记录、付款事实或审计历史。

结果 B：Workbench 在提交命中强重复时显示不可忽略的处置对话框，明确禁止覆盖或重复提交，并提供“查看原申请”“修改／更换当前草稿发票”和“删除当前草稿”三个有界入口。删除只允许本人、从未提交且未进入付款的草稿，必须填写 1～1000 字删除原因；Finance 设置 `deleted_at` 后从默认列表隐藏，同时向不可变 `finance_request_retention_events` 追加删除原因、操作人 Subject、Credential ID、Command ID 和发生时间。

## 产品与数据边界

- 已提交申请不能覆盖、软删除或被新草稿改写；如原申请有误，继续使用撤回和既有 revision 链纠正。
- 当前草稿可以修改票面字段、重新上传正确原件或移除错误发票后再保存；Finance 在正式提交事务内继续执行最终重复校验。
- 删除不是物理删除：`finance_requests`、历史 revision、Evidence 引用和保留事件不清除；默认业务列表只过滤 `deleted_at` 非空记录。
- 删除资格由 Owner 根据当前申请所有人、状态、历史提交事件和付款 delivery 判断；Workbench 只消费 `allowed_actions.delete`，不能自行放宽。
- 删除原因由服务端去空白并强制校验，不能只依赖浏览器 `required`；操作人和凭证来自已验证 Principal，不接受客户端自报。
- 开放 API 与内部 Workbench 删除端点使用同一 `DeleteDraft` 用例；开放 API 的删除请求也必须携带 `reason`。

## 验证

- Internal Finance Requests：`go test -count=1 ./...`、`go vet ./...` 通过；HTTP 测试覆盖内部和开放 API 的必填删除原因传递。
- PostgreSQL 18.6 隔离集成：`go test -tags=integration -count=1 ./internal/finance` 通过；覆盖默认列表隐藏、审计列表保留、不可变删除事件中的原因／操作人／凭证和历史 revision 保留。
- Unified Workbench：`go test -count=1 ./...`、`go vet ./...` 通过；`node --check` 通过；全量 `node --test internal/server/*.test.mjs` 共 302 项通过。
- Gateway：`go test -count=1 ./...`、`go vet ./...` 通过；OpenAPI 契约测试确认删除请求同时必填 `confirmation` 与 1～1000 字 `reason`。
- Workbench 专项覆盖重复发票强提示、查看／修改／删除解决路径、必填删除原因、DELETE 命令和默认列表刷新。
- 定向 `git diff --check` 通过。

## 公网发布

- 最终 release：`v0.1.0-test.oms-v1.96`，发布包 SHA-256 `fcf9596d1e8ed6fd5f53940c93a10e075cd26cbed438e0808cedef6d58088fdd`；Finance／Workbench 功能镜像先随 v1.95 上线，v1.96 补齐并发布同一删除原因约束的 OpenAPI；
- Internal Finance Requests 镜像：`sha256:5417e1b71bf2c6229f2f5937a90161294d60c4927b2ff9396e28f7d781e8d433`；
- Unified Workbench 镜像：`sha256:c27670f44038b5958d98c4a74ec1dd43efe3eed51aa6e2151d614c3903f4d9dc`；
- Gateway 镜像：`sha256:074d2ac9f20d98343caad9cc148c4b27b106e1fb1245eca6110c13699a3b35fd`；
- 最终切换前一致性备份：`20261109T130558Z`，21 项 `checksums.sha256` 全部通过；候选 IAM 契约预检和发布后独立 D1 `check` 通过；
- v1.95 自动替换阶段再次遇到已退出一次性容器与 Compose 网络删除的已知竞态，脚本在删除数据卷前失败关闭；确认 35 个数据卷保留且残留端点消失后，用同一候选环境显式 `d1 up` 完成启动，没有删除或恢复业务卷；v1.96 在独立备份和候选预检后以 `d1 up` 对齐 OpenAPI 契约；
- 最终 21/21 正式长运行容器 healthy，Finance、Workbench 与 Gateway 运行上述精确 digest；
- 公网 Finance JavaScript与 CSS 分别和本地逐字节一致，SHA-256 为 `09141bce67dfd5cd32fe1b3cd55de55f82226fc5cfe3a6bbe0267f1d063e15ce`、`480f973807fb72688b5f56b27f6499dd397784bf38f27194c84cadd3ef7b3287`，包含“这张发票已经使用过”“不能覆盖或再次提交”“删除当前草稿”和必填删除原因逻辑；
- 公网 `/open-api/v1/openapi.json` 与本地逐字节一致，SHA-256 为 `0e0e74c29cb79e6b42191ce9fd105be91c94412818a4052f35443da662c9ad50`，删除 schema 的 `required` 为 `confirmation, reason`，原因边界为 1～1000 字；
- 发布后只读复核确认“1245123”仍为 `draft / revision 1 / deleted_at NULL`，“4·14·12”仍为 `payable / revision 2 / deleted_at NULL`；从本轮切换起新增的 `deleted` 保留事件为 0。

本轮只发布能力，没有替用户删除现有“1245123”草稿，也没有提交、撤回、覆盖或修改任何线上财务申请。用户可在页面选择“删除草稿”，填写原因后自行执行实际业务命令。
