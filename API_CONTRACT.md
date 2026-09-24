# 本地控制 API 契约草案

这是控制面 HTTP API 的语言无关草案。它只负责模型目录、工件任务、实例生命周期和能力调用；具体模型请求仍由能力 schema 定义。默认监听 `127.0.0.1`，管理 API 与推理 API 可以配置不同的凭据和访问策略。

## 通用约定

- Base URL：`http://127.0.0.1:<port>/v1`
- 请求和响应默认使用 `application/json`；媒体能力按能力声明使用 multipart 或二进制流。
- 每个请求可带 `X-Request-ID`；服务若未收到则生成并在响应 header 返回。
- 所有长任务返回 `202 Accepted` 和 `task_id`，客户端通过任务资源查询进度。
- 时间使用 UTC RFC 3339；大小使用整数 bytes；ID 使用不透明字符串。
- `model_id` 不直接嵌入路径，避免 `publisher/name` 中的斜杠破坏路由；用 JSON 字段或 URL 编码。

## 错误响应

```json
{
  "error": {
    "code": "ARTIFACT_CORRUPT",
    "message": "sha256 does not match the manifest",
    "request_id": "req_01",
    "retryable": false,
    "details": {
      "artifact": "model.safetensors",
      "expected_sha256": "...",
      "actual_sha256": "..."
    }
  }
}
```

`message` 面向用户但不包含 token、完整输入或本地敏感路径；详细诊断进入实例日志。客户端按 `code` 分支，不按 message 文本匹配。常用 HTTP 映射：

| HTTP | 代码示例 | 说明 |
| --- | --- | --- |
| 400 | `INVALID_REQUEST` | JSON、字段或能力 schema 无效 |
| 401/403 | `UNAUTHORIZED` / `FORBIDDEN` | 凭据或权限不足 |
| 404 | `MODEL_NOT_FOUND` / `INSTANCE_NOT_FOUND` | 资源不存在 |
| 409 | `INSTANCE_AMBIGUOUS` / `TASK_EXISTS` | 当前状态不允许或选择不明确 |
| 413 | `INPUT_TOO_LARGE` | 请求或媒体超过能力限制 |
| 422 | `INVALID_OPERATION_INPUT` | 能力输入未通过 schema |
| 429 | `RESOURCE_BUSY` | 并发或资源配额不足 |
| 500 | `ADAPTER_ERROR` | 适配器内部错误 |
| 502/503 | `WORKER_EXITED` / `SERVICE_UNAVAILABLE` | 模型进程退出或尚未就绪 |

## 模型目录

### `GET /v1/models`

返回模型定义和本地状态，不触发下载：

```json
{
  "items": [
    {
      "id": "convaiinnovations/laya",
      "version": "0.3.18",
      "revision": "cf7c...7273e",
      "variants": ["english", "multilingual"],
      "installed_variants": ["english"],
      "capabilities": ["system_one"],
      "license": "Apache-2.0"
    }
  ]
}
```

### `POST /v1/pulls`

```json
{
  "model_id": "convaiinnovations/laya",
  "version": "0.3.18",
  "revision": "cf7c...7273e",
  "variant": "english"
}
```

响应 `202`：

```json
{
  "task_id": "task_01",
  "status": "queued",
  "poll": "/v1/tasks/task_01"
}
```

### `GET /v1/models/{encoded_id}`

返回清单、工件摘要、平台 profile、能力 schema 引用和安装状态。此请求不能把清单中的 URL 直接变成可执行命令。

## 任务

### `GET /v1/tasks/{task_id}`

```json
{
  "id": "task_01",
  "kind": "pull",
  "status": "running",
  "progress": {"bytes_done": 410000000, "bytes_total": 842609210},
  "started_at": "2026-09-24T08:00:00Z",
  "updated_at": "2026-09-24T08:01:10Z",
  "error": null
}
```

状态：`queued`、`running`、`succeeded`、`failed`、`cancelled`。取消通过 `POST /v1/tasks/{task_id}/cancel`，重复取消应幂等。

## 实例

### `POST /v1/instances`

```json
{
  "model_id": "convaiinnovations/laya",
  "version": "0.3.18",
  "revision": "cf7c...7273e",
  "variant": "english",
  "profile": "auto",
  "default": true
}
```

响应 `202` 返回启动任务；任务完成后 `GET /v1/instances/{instance_id}`：

```json
{
  "id": "inst_01",
  "status": "ready",
  "model": {"id": "convaiinnovations/laya", "version": "0.3.18", "variant": "english"},
  "runtime": "laya-python@0.1",
  "device": "cpu",
  "capabilities": ["system_one"],
  "default": true,
  "started_at": "2026-09-24T08:02:00Z"
}
```

状态至少包括：`preparing`、`starting`、`ready`、`stopping`、`stopped`、`failed`、`orphaned`。只有 `ready` 实例接受调用；`orphaned` 需要显式清理，不能自动当作健康实例。

### `DELETE /v1/instances/{instance_id}`

停止请求应幂等。可以用 `?grace_seconds=10` 指定等待时间；超时后强制终止，并在事件日志中标明未完成请求。

## 能力调用

### `POST /v1/instances/{instance_id}/operations/{operation}`

请求体完全由能力 schema 定义。Laya 示例：

```json
{
  "state": {"body": "We were charged twice."},
  "questions": {
    "refund": {
      "type": "noul",
      "instructions": "Does the customer ask for a refund?"
    }
  }
}
```

响应外层统一带实例元数据，模型结果放在 `result`：

```json
{
  "request_id": "req_02",
  "instance_id": "inst_01",
  "operation": "system_one",
  "model": {"id": "convaiinnovations/laya", "version": "0.3.18", "variant": "english"},
  "device": "cpu",
  "result": {
    "answers": {"refund": {"noul": 0.98}},
    "usage": {"input_tokens": 42, "output_tokens": 0},
    "routing": {"model": "english"}
  }
}
```

若能力声明 `streaming: true`，响应使用明确的事件格式：`started`、`delta`、`completed` 或 `error`，每个事件带 request ID 和序号。客户端断开后控制面发送取消；适配器报告是否已停止。

## Jev 兼容入口

`POST /v1/systemone` 是 Laya 的兼容代理，不增加通用外层包装，直接返回 Laya/TypeSafe Jev 形状。默认实例通过配置确定；若没有默认实例且有多个候选，返回 `409 INSTANCE_AMBIGUOUS`。该入口不承担模型安装，也不接受任意模型 ID 让服务隐式下载。

## MCP 与 Skill

MCP 代理使用 `/v1/instances` 和能力目录发现可用实例，再把允许的能力映射成 MCP tools。Skill 执行器只使用这些公开 API；它不能绕过 API 直接访问 daemon 数据目录、适配器端口或模型文件。权限范围与模型能力分开配置。
