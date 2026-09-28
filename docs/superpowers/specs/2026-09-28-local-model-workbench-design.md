# Local Model Workbench Design

**Date:** 2026-09-28

**Status:** Proposed for review

## Goal

把 Modelctl 从 Laya 的本地控制 API 扩展为一个类似 Ollama 使用体验的本地模型工作台。用户可以在 Web 页面管理模型和实例，配置本地代理与运行偏好，查看受维护的社区扩展，并生成 MCP 配置来连接模型能力。

第一阶段继续使用现有的 Node.js 控制面和 Python 模型适配器，不迁移到 Go，也不改变 Laya 的 `system_one` 原生协议。

## Scope

### Included

- Web 导航：模型、实例、判断工作台、社区、设置。
- 本地设置：HTTP/HTTPS/ALL/NO proxy、`NO_PROXY`、默认运行设备、数据目录展示和 daemon 地址展示。
- 设置 API：读取和更新持久化配置；下载器按配置优先级使用代理。
- MCP 配置：查看当前 Modelctl MCP 入口、复制配置片段、启用或停用受维护的 MCP 项。
- 社区目录：展示项目内置的受维护清单，包含 MCP server、Skill 和模型适配器的元数据、来源、版本、权限说明和安装方式。
- API 契约和数据文件：所有新增对象使用版本字段和结构化错误。
- Web UI 对设置、社区项和 MCP 配置的错误状态、空状态和保存状态给出明确反馈。

### Excluded

- 将 daemon 重写为 Go。
- 通用聊天接口或把 Laya 伪装成 OpenAI Chat Completions。
- 多机调度、账户系统、远程市场和自动更新。
- 在 `pull`、`run` 或打开社区页面时自动执行任意第三方 shell 命令。
- 允许社区条目直接获得任意文件、网络或系统命令权限。

## Architecture

### Existing runtime boundary

继续保留三层：

```text
Web UI / CLI / MCP proxy
          |
          v
Node.js local daemon
          |
          v
Python Laya adapter process
```

Node.js daemon 负责目录、工件、配置、实例生命周期、日志和 HTTP 路由。Python 进程只负责加载已校验的本地 checkpoint 和调用 Laya。新增设置和社区能力不能绕过这个边界直接读取模型进程或模型文件。

### Persistent data

在 `MODELCTL_DATA_DIR` 下增加以下文件，由 Node.js 以临时文件加 rename 的方式原子写入：

- `settings.json`：用户可调整的本地设置。
- `mcp.json`：Modelctl 管理的 MCP 配置项。

缺少文件时使用内存中的默认值；读取损坏文件时返回结构化错误并保留原文件，不能静默覆盖用户配置。

设置字段：

```json
{
  "schema_version": 1,
  "proxy": {
    "http": "http://127.0.0.1:7897",
    "https": "http://127.0.0.1:7897",
    "all": "socks5://127.0.0.1:7897",
    "no_proxy": "127.0.0.1,localhost"
  },
  "default_profile": "auto"
}
```

代理字段在 API 返回和 Web 展示时必须隐藏认证信息。代理使用优先级为：`MODELCTL_*` 环境变量、持久化设置、标准代理环境变量、无代理。`MODELCTL_*` 环境变量只覆盖对应字段，不覆盖其他字段。

`MODELCTL_DATA_DIR` 和 daemon 监听地址仍属于进程启动参数；第一阶段在设置页只读展示它们，避免页面修改后造成当前进程和用户预期不一致。

### Community registry

初始社区目录放在仓库的 `community/index.json`，由服务端作为只读资源暴露。条目必须是声明性元数据：

```json
{
  "schema_version": 1,
  "items": [
    {
      "id": "modelctl/laya-system-one-mcp",
      "kind": "mcp",
      "name": "Laya System One MCP",
      "version": "0.1.0",
      "description": "Expose the local Laya system_one capability as an MCP tool.",
      "source": {
        "type": "repository",
        "url": "https://github.com/1085924051/Modelctl.git",
        "revision": "cbaa378"
      },
      "permissions": ["loopback:modelctl-api"],
      "configuration": {
        "command": "modelctl-mcp",
        "args": []
      }
    }
  ]
}
```

服务端校验 `schema_version`、`id`、`kind`、版本、来源和权限字段。Web UI 可以展示和复制配置，但不会因为用户浏览或选择条目而下载、安装或执行第三方代码。后续要支持安装时，必须另行设计来源校验、权限确认和显式安装动作。

### MCP management

Modelctl 当前 MCP 入口继续使用 `modelctl-mcp`，通过 daemon 的公共 API 调用已运行的 Laya 实例，避免 MCP 客户端重复加载模型。

新增本地配置 API：

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/v1/mcp/config` | 返回脱敏后的已配置 MCP 项和 Modelctl 推荐入口 |
| `PUT` | `/v1/mcp/config` | 保存经过 schema 校验的 MCP 项启用状态和非敏感配置 |
| `GET` | `/v1/community` | 返回校验后的内置社区目录 |
| `GET` | `/v1/settings` | 返回脱敏设置和只读运行信息 |
| `PUT` | `/v1/settings` | 校验并保存可修改的设置字段 |

`PUT /v1/settings` 只允许 loopback daemon 接收。代理 URL 只保存在本机配置，不写入日志、任务错误或响应中的认证部分。设置生效范围在响应中明确：下载代理立即用于后续下载；默认 profile 用于后续启动；daemon 地址和数据目录只读。

### Web information architecture

使用现有 hash 路由，保持 `#models` 作为默认入口：

- `#models`：模型目录、变体下载、设备选择和实例启动。
- `#instances`：实例列表、设备、健康状态、日志入口和停止操作。
- `#playground`：按实例调用 Laya `system_one`，保留原生结构化结果。
- `#community`：社区条目、来源、版本、权限和配置复制操作。
- `#settings`：代理、默认设备、运行目录和 MCP 配置入口。

页面共享同一份刷新状态，但设置和社区错误不能让模型页变成空白。保存、复制、加载和接口失败都通过页面内状态提示反馈。

## Security and trust boundaries

- 默认只监听 `127.0.0.1`。
- 社区目录按不可信输入处理，即使它位于仓库中也不能把字段拼接成 shell 命令执行。
- 社区条目展示来源 URL、revision、版本和声明的权限。
- MCP 配置只允许 `command`、`args`、受限的环境变量名和启用状态；不允许 Web UI 写入任意工作目录或隐式安装脚本。
- 日志和错误消息不包含完整代理认证信息、MCP 环境变量值、模型输入和本地敏感路径。

## Error handling

新增错误码：

- `SETTINGS_INVALID`：设置字段或代理 URL 无效。
- `SETTINGS_UNREADABLE`：配置文件存在但无法解析。
- `COMMUNITY_INVALID`：社区清单未通过 schema 校验。
- `MCP_CONFIG_INVALID`：MCP 配置项不符合允许的结构。

HTTP 响应沿用现有 `{ error: { code, message, request_id, retryable, details } }` 结构。Web UI 显示可操作的简短消息，详细错误保留在 API 响应和本地日志中。

## Testing and acceptance

### Unit and API checks

- 设置缺省值、读写、原子替换和损坏文件保护。
- 代理优先级、认证信息脱敏和下载环境生成。
- 社区清单 schema 校验及无效条目错误。
- MCP 配置允许字段、启用状态和脱敏返回。
- 新增 GET/PUT API 的成功与错误响应。
- 现有模型目录、下载、实例和 `system_one` 接口保持通过。

### Web checks

- 首屏 `#models` 正常加载，既有模型启动流程不回归。
- 切换到 `#settings` 后可以看到当前代理为 `7897`，保存后重新读取仍保持一致。
- `#community` 能展示清单条目、来源和权限，并能复制 MCP 配置。
- `#instances` 和 `#playground` 能看到当前可用实例并完成一次 Laya 调用。
- 页面无空白、脚本错误和启动按钮误禁用问题。

### Acceptance

1. 用户可以从 Web UI 查看并修改本地下载代理，后续下载使用保存的代理设置。
2. 用户可以从社区页面查看受维护的 MCP/Skill 元数据并复制安全的配置片段。
3. 用户可以从设置页查看和启用 Modelctl MCP 入口，MCP 调用仍通过本地 daemon 路由到已运行的 Laya 实例。
4. Laya 的 English 与 multilingual 变体、auto/cpu/mps 启动和既有 `system_one` 调用保持可用。
5. 代码、文档和测试提交到当前 GitHub 仓库；提交前不覆盖已有本地权限改动。

## Phasing

第一批实现顺序：

1. 配置存储和设置 API。
2. 社区清单及社区 API。
3. MCP 配置 API 和 Web 页面。
4. Web 路由、设置页、社区页和 MCP 配置复制。
5. API、页面和已有模型流程回归验证。

不在本批次实现第三方扩展自动安装。后续若需要真正的社区安装能力，单独增加设计和权限确认流程。
