# Go Native Desktop Client MVP

**Date:** 2026-09-28

**Status:** Approved

## Goal

提供一个简洁的 Go 原生 macOS 桌面客户端，让用户以接近 Ollama 客户端的方式查看 Laya 模型、选择变体、下载模型、启动或停止实例，并运行一次结构化判断。

## Scope

### Included

- macOS Apple Silicon 首版桌面窗口。
- 模型列表：Laya、English/Multilingual、安装状态和文件大小。
- 运行设备：Auto、CPU、MPS。
- Pull 下载任务和进度状态。
- Run/Stop 实例生命周期。
- Laya `system_one` 判断输入和结构化结果展示。
- daemon 连接状态和手动刷新。
- 默认连接 `http://127.0.0.1:11435`，支持 `MODELCTL_URL`。

### Excluded

- 浏览器 Web UI 改造。
- 社区、设置、MCP 管理页面。
- Go daemon 重写。
- 账号、远程服务、多机调度。
- 通用聊天 UI。

## Architecture

```text
Go/Fyne desktop app
        |
        v
http://127.0.0.1:11435
        |
        v
Existing Node daemon -> Python Laya adapter
```

Go 客户端只调用公开 HTTP API，不读取 `~/.modelctl`，不启动 Python 进程，也不复制模型管理逻辑。API client 单独封装模型目录、下载任务、实例和 `system_one` 调用；UI 层只负责状态和用户操作。

## UI Layout

- 左侧窄导航：`Models`、`Running`。
- 顶部：Modelctl 标识、daemon 地址、连接状态、刷新按钮。
- 主区域：模型卡片，展示模型名、变体、安装状态、设备选择和 Pull/Run 操作。
- 运行区域：活动实例、variant、实际设备、端口和 Stop 按钮。
- 判断区域：实例选择、文本输入、问题类型、问题说明、Run 判断按钮和结果卡。

视觉保持低饱和浅色背景、深色文字、细边框、少量绿色状态色，控件密度接近 Ollama 客户端；不放大幅 Hero、营销文案或浏览器式页面导航。

## Data Flow

1. 启动时读取 `MODELCTL_URL`，默认 `http://127.0.0.1:11435`。
2. 并行请求 `/health`、`/v1/models`、`/v1/instances`。
3. 用户点击 Pull 后调用 `POST /v1/pulls`，轮询 `/v1/tasks/{id}`，在 UI 中显示百分比。
4. 用户点击 Run 后调用 `POST /v1/instances`，默认 `default=true`，等待接口返回 `ready`。
5. 用户点击 Stop 后调用 `DELETE /v1/instances/{id}`。
6. 判断请求调用 `/v1/instances/{id}/operations/system_one`，保留 Laya 原生输入和输出结构。

## Error Handling

- daemon 不可用：显示连接状态和可重试错误。
- 下载失败：显示后端错误消息，不自动重复提交任务。
- 实例启动失败：保留错误状态和实例 ID，允许再次启动。
- 无 ready 实例：判断按钮禁用，并提示先启动模型。
- HTTP 请求设置 120 秒超时；下载任务轮询允许持续等待。

## Packaging

- `desktop/go.mod` 独立管理 Go 依赖。
- `desktop/main.go` 作为应用入口。
- `desktop/internal/api` 只包含 daemon API client 和 DTO。
- `desktop/internal/ui` 只包含 Fyne UI 状态和渲染。
- 提供 `desktop/build-macos.sh`，输出 `desktop/dist/Modelctl.app`。

## Acceptance

1. 在本机运行 `go test ./...` 全部通过。
2. `go build ./...` 成功。
3. `build-macos.sh` 生成可打开的 macOS `.app`。
4. 客户端能从真实 daemon 读取两个 Laya 变体。
5. 客户端能识别已运行的 multilingual MPS 实例。
6. 客户端能执行一次真实 `system_one` 请求并展示 `answers`。
7. 客户端不启动浏览器、不增加新的 Web UI 服务、不修改现有 daemon API。
