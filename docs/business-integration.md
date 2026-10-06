# Modelctl 业务系统接入指南

Modelctl 的接入边界很简单：Modelctl 负责模型文件、运行时和能力版本；客户自己的系统负责用户、订单、工单、权限和最终业务动作。业务代码只调用稳定的 capability API，不需要了解 Laya/Jev 的 `state`、`questions` 或 adapter 进程。

## 一次完整的接入流程

1. 管理员在 Desktop 下载并加载模型。
2. 在 Capabilities 中选择模板，定义业务输入字段、输入模板和输出问题。
3. 用真实样本测试，发布一个不可变版本，例如 `refund-check@1.0.0`。
4. 在 Integration 页面复制 endpoint、schema、错误约定和代码示例。
5. 开发者在业务服务中固定调用 `?version=1.0.0`，把自己的业务数据放进 `input`，把订单号、租户号或工单号放进 `metadata`。
6. 业务服务根据结构化 `output` 执行后续动作，并保存 `run_id` 和 `capability_version` 以便审计和重放。

发布新规则或模型时创建 `1.1.0`，先用测试数据验证，再激活新版本。生产服务继续固定旧版本，直到完成切换。

能力定义可以在 Desktop 中复制为 portable JSON，提交到 Git 或制品库，再在另一台机器的 Capabilities 页面通过 **Import capability JSON** 发布。导入默认不会激活版本，管理员可以先下载对应模型、运行测试，再手动激活。

## 四种常见部署方式

| 场景 | Modelctl 放在哪里 | 业务系统怎么调用 |
| --- | --- | --- |
| 个人开发者、本地工具 | 开发机或产品用户电脑 | `127.0.0.1:11435`，REST、Python SDK 或 JavaScript SDK |
| 桌面软件交付 | 随产品安装在用户电脑 | 产品内部 HTTP 客户端调用本机 Modelctl；用户只在 Desktop 管理模型 |
| 企业内网服务 | GPU/CPU 服务器 | HTTPS + Bearer Token；多个业务服务共享能力服务 |
| 文档、历史工单、夜间任务 | 企业内网 Modelctl | `/batch` 提交任务，轮询 `/v1/tasks/{task_id}` |

## 同步调用

业务服务只提交业务字段：

```bash
curl "$MODELCTL_URL/v1/capabilities/refund-check/invoke?version=1.0.0" \
  -H "Authorization: Bearer $MODELCTL_API_INVOKE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "input": {
      "text": "客户说被重复扣款，希望退款",
      "order_id": "O-100"
    },
    "metadata": {
      "tenant_id": "acme",
      "ticket_id": "T-100"
    }
  }'
```

返回值是稳定的结构化结果：

```json
{
  "run_id": "run_123",
  "capability_id": "refund-check",
  "capability_version": "1.0.0",
  "output": {
    "refund": { "type": "noul", "value": 0.92, "confidence": 0.91 },
    "team": { "type": "choice", "value": "billing" }
  },
  "metadata": { "tenant_id": "acme", "ticket_id": "T-100" }
}
```

业务代码只依赖 `output.<name>.value` 和自己的阈值。例如 `refund.value >= 0.8` 可以进入人工退款审核；`team.value` 可以路由到客服队列。原始模型响应保留在 run history 中，供排查使用。

## Python 和 JavaScript

```python
from modelctl_client import Modelctl

client = Modelctl(token="...")
result = client.invoke(
    "refund-check",
    {"text": "客户希望退款", "order_id": "O-100"},
    metadata={"ticket_id": "T-100"},
    version="1.0.0",
)
if result["output"]["refund"]["value"] >= 0.8:
    enqueue_manual_review(result)
```

```js
import { Modelctl } from "./sdk/javascript/modelctl-client.mjs";

const client = new Modelctl(
  process.env.MODELCTL_URL,
  process.env.MODELCTL_API_INVOKE_TOKEN,
);
const result = await client.invoke(
  "refund-check",
  { text: "客户希望退款", order_id: "O-100" },
  { ticket_id: "T-100" },
  "1.0.0",
);
```

## 批处理和异步任务

导入历史工单或文档时调用 `/batch`。接口立即返回 `task_id`，每条结果带有 `index`、`run_id`、`output` 和 `metadata`。业务系统可以重试失败条目，也可以调用 `POST /v1/tasks/{task_id}/cancel` 取消未完成任务，不需要保持长连接。

## 上线前检查

部署脚本或健康检查可以先调用：

```http
GET /v1/capabilities/refund-check/status?version=1.0.0
```

`ready: true` 表示模型已安装且运行时可用；`model_not_installed` 表示管理员需要先在 Desktop 或 CLI 下载指定 variant。第一次真正调用时，Modelctl 会自动启动匹配的 adapter。

## 权限和密钥

- 管理员使用 `MODELCTL_API_TOKEN`，负责发布、激活、模型和运行时管理。
- 业务服务使用 `MODELCTL_API_INVOKE_TOKEN`，只读能力契约、调用 invoke、提交 batch 和轮询任务。
- Token 放在企业 Secret Manager 或系统钥匙串中，不写入 capability JSON、前端代码或 Git。

## 接入交付物

每个上线能力至少交付四项内容：能力 ID 与固定版本、schema/OpenAPI 地址、REST 或 SDK 示例、错误处理约定。这样业务系统只依赖版本化契约，模型下载、加载和 adapter 升级都由 Modelctl 管理。
