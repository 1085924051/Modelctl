# Modelctl 使用案例

以下案例展示从业务问题到能力接口的落地方式。示例输出只是说明字段结构，不代表模型对某段文字的确定判断；上线前应使用本行业样本验证阈值和误判处理。

## 个人开发者：给本地应用增加退款识别

场景：独立开发者写了一个订单管理工具，希望在客服消息中提示“可能需要退款处理”。在自己的电脑安装 Modelctl，下载 Laya `english`，使用 **Capabilities** 中的退款模板，测试后发布 `refund-check@1.0.0`。应用调用：

```python
import sys
sys.path.insert(0, "sdk/python")
from modelctl_client import Modelctl

client = Modelctl("http://127.0.0.1:11435")
result = client.invoke(
    "refund-check",
    {"text": "I was charged twice. Please refund the extra charge.", "order_id": "O-100"},
    {"ticket_id": "T-100"},
    "1.0.0",
)
refund_probability = result["output"]["refund"]["value"]
print(result["run_id"], refund_probability)
```

业务程序可以据概率**提示人工复核**，不要仅凭一个模型值自动退款。完整能力定义和请求文件位于 [examples/integration](../examples/integration)。

## 企业客服：将工单路由到正确队列

场景：客服系统每天收到大量售后、账户和付款问题。管理员把 Modelctl 部署在企业内网，下载模型并发布 `ticket-routing@1.0.0`，字段为 `text` 和可选的 `customer_tier`，输出包含 `team`（choice）及 `priority`（score）。客服服务使用 invoke-only token 调用：

```json
{
  "input": { "text": "发票抬头需要修改", "customer_tier": "enterprise" },
  "metadata": { "ticket_id": "T-2048", "tenant_id": "acme" }
}
```

服务读取 `output.team.value` 决定候选队列，读取 `output.priority.value` 设置待处理优先级，保存 `run_id` 和能力版本以便审计。中文消息应先选择并验证 `multilingual` variant。模型不确定或队列不匹配时进入人工队列。上线时先读取 `/schema?version=1.0.0` 校验字段，再固定版本调用 `/invoke?version=1.0.0`。

## 文档平台：夜间批量风险初筛

场景：企业每天导入大量合同或申请材料，希望把需要人工复核的文档排到前面。管理员发布 `document-risk@1.0.0`，字段为 `document` 与可选的 `document_type`，输出 `review`（noul）和 `risk`（choice）。批处理服务向 `/batch?version=1.0.0` 提交最多 1000 条，每条用 `metadata.document_id` 关联原文档。

任务服务轮询 `/v1/tasks/{task_id}`，按 `results[].index` 回写结果，并单独重试 `errors[]` 中的失败项。高风险或需要复核的结果只进入人工审核清单；Modelctl 不负责最终审批动作。模型版本升级时先发布不激活的新版本，用历史文档对比结果，再切换 active 版本。

这三个案例都遵循同一条链路：**下载模型 → 定义结构化能力 → 用真实数据测试 → 发布版本 → 业务系统按固定版本调用 → 保存运行记录**。桌面操作见 [使用手册](user-guide.md)，接口细节见 [API 文档](api-guide.md)。

