# Modelctl Capability API 文档

业务系统应调用版本化的 Capability API。Modelctl 负责下载、加载和调度模型，并将业务字段转换成 Laya/Jev 需要的内部格式。默认服务地址为 `http://127.0.0.1:11435`，请求与响应使用 UTF-8 JSON。

## 认证与版本

本机默认无需 token。企业服务设置 `MODELCTL_API_TOKEN` 作为管理凭据，并可设置 `MODELCTL_API_INVOKE_TOKEN` 供业务程序调用。业务 token 可读取能力契约、调用 `/invoke` 和 `/batch`、轮询任务；不能发布能力、下载模型或管理运行时。

需要认证时发送 `Authorization: Bearer <token>`。生产系统在能力路径后加 `?version=1.0.0` 固定版本；省略版本会使用当前 active 版本。发布同一个 ID 的新版本不会覆盖旧版本，管理员可在验收后激活。

## 接口清单

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/health` | 服务健康检查 |
| GET | `/v1/models` | 查询模型目录与已下载版本 |
| GET | `/v1/models/{encoded_id}` | 查询模型详情、variant、设备支持和磁盘预检 |
| POST | `/v1/pulls` | 开始下载模型，返回 `202` 与 `task_id` |
| GET | `/v1/tasks/{task_id}` | 查询下载或批量调用任务 |
| POST | `/v1/tasks/{task_id}/cancel` | 取消任务，需管理权限 |
| GET | `/v1/capabilities` | 查询已发布能力 |
| POST | `/v1/capabilities` | 发布能力，需管理权限 |
| GET | `/v1/capabilities/{id}` | 查询能力及版本 |
| GET | `/v1/capabilities/{id}/schema` | 查询输入/输出 JSON Schema |
| GET | `/v1/capabilities/{id}/status` | 查询模型安装与运行时状态 |
| GET | `/v1/capabilities/{id}/integration` | 查询固定地址、认证说明及调用示例 |
| GET | `/v1/capabilities/{id}/openapi` | 获取该能力的 OpenAPI 3.1 文档 |
| POST | `/v1/capabilities/{id}/invoke` | 同步调用能力 |
| POST | `/v1/capabilities/{id}/batch` | 提交 1–1000 条批量任务，返回 `202` |
| POST | `/v1/capabilities/{id}/activate` | 激活指定版本，需管理权限 |
| GET | `/v1/runs/{run_id}` | 查询一次调用记录，需管理权限 |

`{id}` 要做 URL 编码；模型 ID 中的 `/` 尤其要编码，例如 `convaiinnovations%2Flaya`。`schema`、`status`、`integration`、`openapi`、`invoke` 和 `batch` 都支持 `?version=`。

## 从发布到调用

管理员先在桌面端下载模型、创建并测试能力，也可以通过 CLI 发布仓库中的 [示例能力定义](../examples/integration/refund-check.capability.json)。能力定义声明 `input.fields`、文本模板、模型绑定和 `noul`/`choice`/`score` 问题。业务方只提交字段值。

```http
GET /v1/capabilities/refund-check/schema?version=1.0.0
GET /v1/capabilities/refund-check/status?version=1.0.0
```

`status` 为 `model_not_installed` 时先下载模型；为 `not_started` 时模型已下载，首次调用可自动启动；为 `ready` 时可直接调用。

```bash
curl 'http://127.0.0.1:11435/v1/capabilities/refund-check/invoke?version=1.0.0' \
  -H 'Content-Type: application/json' \
  -d '{"input":{"text":"The customer was charged twice and wants a refund.","order_id":"O-100"},"metadata":{"ticket_id":"T-100"}}'
```

企业部署时再加 `-H "Authorization: Bearer $MODELCTL_API_INVOKE_TOKEN"`。

Windows PowerShell 可用以下方式调用同一接口：

```powershell
$body = @{
  input = @{ text = 'The customer wants a refund.'; order_id = 'O-100' }
  metadata = @{ ticket_id = 'T-100' }
} | ConvertTo-Json -Depth 5
Invoke-RestMethod -Method Post `
  -Uri 'http://127.0.0.1:11435/v1/capabilities/refund-check/invoke?version=1.0.0' `
  -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($body))
```

企业环境在 `Invoke-RestMethod` 中加 `-Headers @{ Authorization = "Bearer $env:MODELCTL_API_INVOKE_TOKEN" }`。

响应的关键字段如下，概率数值仅示意，实际结果由模型决定：

```json
{
  "request_id": "run_example",
  "capability": { "id": "refund-check", "version": "1.0.0" },
  "output": {
    "refund": { "type": "noul", "value": 0.92, "noul": 0.92, "confidence": 0.91 },
    "team": { "type": "choice", "value": "billing", "choice": "billing" }
  },
  "metadata": { "ticket_id": "T-100" },
  "run_id": "run_example"
}
```

`output` 的键来自能力中的问题 ID。`noul` 的 `value` 为 0–1 数值；`choice` 的 `value` 为选项键；`score` 的 `value` 为模型返回的评分值。响应还可能包含 `raw` 和 `model`，业务逻辑应优先使用规范化的 `output`。

## 批量任务与错误

```http
POST /v1/capabilities/refund-check/batch?version=1.0.0
Content-Type: application/json

{"items":[{"input":{"text":"Please refund my order."},"metadata":{"ticket_id":"T-1"}},{"input":{"text":"I cannot sign in."},"metadata":{"ticket_id":"T-2"}}]}
```

响应返回 `task_id` 与 `poll`。按 `GET /v1/tasks/{task_id}` 轮询到 `succeeded`、`failed` 或 `cancelled`。任务记录保留每条的 `index`、`run_id`、`output` 或 `errors`；任务总体成功也可能包含单条错误，因此要检查 `errors`。批任务锁定提交时的能力版本。

错误响应统一为 `{ "error": { "code": "...", "message": "...", "request_id": "..." } }`。常见错误：`401 UNAUTHORIZED`、`403 FORBIDDEN`、`404 CAPABILITY_NOT_FOUND`、`409 MODEL_NOT_INSTALLED`、`409 CAPABILITY_VERSION_EXISTS`、`422 INPUT_INVALID`、`422 BATCH_INVALID`。客户端按 HTTP 状态和 `error.code` 处理，不依赖错误文字。

Python 与 JavaScript SDK 见 [SDK 说明](../sdk/README.md)。每个能力的 `/integration` 和 `/openapi` 返回按真实字段与版本生成的契约，接入时以它们为准。
