# 客户业务系统接入手册

Modelctl 的客户接入方式是“发布一次业务能力，业务系统按能力调用”。客户的业务代码不需要安装 Laya/Jev，不需要管理模型文件、Python 环境、adapter 端口，也不需要拼接 Laya 的 `state` 和 `questions`。

## 一条完整接入链路

```text
业务人员定义业务目标
  ↓
在 Modelctl Desktop 下载并加载模型
  ↓
创建能力（输入字段、问题、输出类型、模型绑定）
  ↓
用真实样本测试并发布版本，例如 refund-check@1.0.0
  ↓
开发者读取 schema，接入 REST / SDK / CLI
  ↓
业务系统传入自己的业务数据和 metadata
  ↓
Modelctl 自动调用匹配模型并返回结构化结果
  ↓
业务系统保存 run_id、结果和 capability_version
```

客户端负责部署、下载、加载、能力配置、测试、版本发布和运行记录。客户自己的系统负责用户权限、业务数据库、订单/工单、队列和最终业务动作。

## 四种接入场景

| 场景 | Modelctl 部署位置 | 业务系统调用方式 | 适合的客户 |
| --- | --- | --- | --- |
| 本地开发 | 开发者电脑 `127.0.0.1:11435` | REST、SDK、CLI | 个人开发者、PoC、离线工具 |
| 随桌面产品交付 | 用户电脑 | 产品内 HTTP 客户端调用本机 Modelctl | 设计工具、办公软件、行业桌面软件 |
| 企业内网服务 | GPU/CPU 服务器 | 内网 HTTPS + Bearer Token | 客服、金融、制造、政企 |
| 异步批处理 | 企业内网 Modelctl | `/batch` + 任务轮询 | 文档导入、历史工单、夜间任务 |

## 业务系统实际怎么调用

### 1. 先读取能力契约

开发者在接入时先读取 schema，把它当作接口契约：

```http
GET /v1/capabilities/refund-check/schema?version=1.0.0
```

schema 会说明输入字段、必填项、输出字段、`choice` 枚举、`noul` 范围和 `score` 结构。前端表单、后端校验和接口测试都可以由这个 schema 生成。

能力可以声明多个业务输入字段，并通过模板转换为模型可读文本。例如：

```json
{
  "input": {
    "type": "object",
    "fields": {
      "text": { "type": "string", "required": true },
      "order_id": { "type": "string", "required": false }
    },
    "template": "Customer message: {{text}}\nOrder: {{order_id}}"
  }
}
```

业务系统传入结构化字段即可，Modelctl 会负责渲染模板并生成 Laya/Jev 所需的内部输入。旧版单字段 `{ "type": "text", "field": "text" }` 仍然兼容。

### 2. 同步调用

订单、工单或用户请求需要立即得到判断时，业务服务调用：

```http
POST /v1/capabilities/refund-check/invoke
Content-Type: application/json

{
  "input": {
    "text": "客户被重复扣款，希望退款"
  },
  "metadata": {
    "tenant_id": "acme",
    "ticket_id": "T-100",
    "operator_id": "u-42"
  }
}
```

返回结果包含：

```json
{
  "run_id": "run_123",
  "capability_id": "refund-check",
  "capability_version": "1.0.0",
  "output": {
    "refund": { "type": "noul", "value": 0.92, "noul": 0.92, "confidence": 0.91 },
    "team": { "type": "choice", "value": "billing", "choice": "billing" }
  },
  "metadata": { "tenant_id": "acme", "ticket_id": "T-100", "operator_id": "u-42" }
}
```

业务系统只消费 `output.*.value` 和自己关心的类型字段；`run_id`、`metadata`、`capability_version` 用于审计、重放和问题排查。

### 3. 批量调用

导入历史数据或处理队列时提交一批输入：

```http
POST /v1/capabilities/ticket-routing/batch
Content-Type: application/json

{
  "items": [
    { "input": { "text": "无法登录" }, "metadata": { "ticket_id": "T-1" } },
    { "input": { "text": "发票抬头需要修改" }, "metadata": { "ticket_id": "T-2" } }
  ]
}
```

接口返回 `task_id`。业务服务轮询 `GET /v1/tasks/{task_id}`，按 `index` 保存每条结果。某一条失败不会阻塞其他条目，任务也支持取消。

## 企业部署方式

在企业内网的模型服务器运行 daemon：

```bash
MODELCTL_HOST=0.0.0.0 \\
MODELCTL_API_TOKEN=从企业 Secret Manager 注入 \\
node bin/modelctl.js daemon
```

实际生产部署建议：

1. 只通过企业 API 网关暴露 HTTPS 地址。
2. 业务服务从 Secret Manager 注入 `MODELCTL_API_TOKEN`。
3. 只开放 `/v1/capabilities/*`、`/v1/tasks/*` 和必要的健康检查接口。
4. 模型目录和 adapter 端口只允许 Modelctl 主机访问。
5. 给不同业务服务分配独立 token，并在网关记录调用方、耗时和错误码。

业务服务调用时只需要稳定地址和 token：

```bash
curl https://modelctl.internal.example/v1/capabilities/refund-check/invoke \\
  -H "Authorization: Bearer $MODELCTL_API_TOKEN" \\
  -H 'Content-Type: application/json' \\
  -d '{"input":{"text":"客户被重复扣款"},"metadata":{"ticket_id":"T-100"}}'
```

模型未下载时，接口会返回 `409 MODEL_NOT_INSTALLED`，并告诉管理员需要准备的模型、版本和 variant。模型已准备后，第一次能力调用会自动启动匹配运行时，业务系统不需要管理实例端口。

## 接入时的版本策略

- 开发环境可以省略版本，调用当前 active 版本。
- 生产服务在验收后建议固定 `?version=1.0.0`。
- 修改问题、模型绑定或输出结构时发布新版本，例如 `1.1.0`。
- 先用 `activate: false` 发布新版本，跑样本或批任务验收后再激活。
- 每次调用保存 `capability_version`，这样模型升级后仍能重放历史结果。

## 客户交付物

客户完成接入后，应得到以下四项可交付物：

1. 能力 ID 和当前版本，例如 `refund-check@1.0.0`。
2. schema 地址，用于前端和后端校验。
3. REST、Python 或 JavaScript 示例代码。
4. 错误处理约定：`MODEL_NOT_INSTALLED`、能力版本错误、单条批处理失败和任务取消。

完整命令示例见 [`examples/integration`](../examples/integration)，SDK 说明见 [`sdk/README.md`](../sdk/README.md)，完整 API 约定见 [`docs/integration.md`](integration.md)。
