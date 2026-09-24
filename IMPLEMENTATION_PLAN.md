# 首版实现计划

这份计划把设计稿转换成可执行的工程切片。它默认首版是 CLI + 本地 API，Laya 是第一个真实适配器，Linux/macOS 先以 CPU 共同路径为基线；CUDA、MPS 和桌面界面保持可插入但不作为尚未验证的承诺。

## 建议技术选型

| 部分 | 选择 | 理由 |
| --- | --- | --- |
| 控制面 daemon | Go | 适合 Linux/macOS 的单文件发行、子进程监督、HTTP 服务和并发任务 |
| CLI | Go，与 daemon 共用内部客户端 | CLI 不重复实现状态和权限逻辑 |
| 状态数据库 | SQLite | 单机部署无需外部服务；记录模型、工件、任务、实例和事件 |
| 工件缓存 | 内容寻址目录 + SQLite 索引 | 按 SHA-256 复用大文件，下载完成后原子提交 |
| 适配器通信 | 本机 HTTP 或 Unix domain socket；协议字段保持一致 | 跨语言，便于 Python Laya 和 C/C++ 适配器共存 |
| Laya 运行时 | 独立 Python 环境 + 受维护的薄入口 | 复用上游推理代码；权重从本地准备目录加载。`laya` wheel 不包含 PyTorch，运行时必须按平台锁定重依赖 |
| API 描述 | JSON Schema + OpenAPI 文档 | 能表达 Laya 结构化输入和未来媒体/流式能力 |
| 日志 | 结构化 JSON 日志，按 daemon、任务、实例分组 | 支持 CLI 查询，也避免把模型输入当普通文本混入日志 |

Go 不是模型推理语言；适配器才负责 Python、C/C++ 或其他引擎。若后续发现单文件发行或子进程管理不是主要约束，控制面可以替换实现语言，但公共 API、清单和适配器协议保持独立。

## 建议仓库结构

```text
cmd/tool/                  CLI 入口
cmd/toold/                 daemon 入口
internal/catalog/          清单解析、签名/来源和模型查询
internal/artifacts/        下载、断点续传、哈希校验、原子提交
internal/runtime/          适配器协议、探针、prepare、实例生命周期
internal/process/          子进程、端口、信号、日志和重启恢复
internal/api/              本地 HTTP 路由、鉴权、请求限制
internal/store/            SQLite schema、迁移和事件记录
adapters/laya/             Laya 清单、Python 入口和适配器包装
adapters/whisper/          后续 whisper.cpp 适配器
schemas/                   清单和能力的 JSON Schema
catalog/                   项目维护的模型清单
runtime/                   受维护的适配器包和锁文件
docs/                      API、安装和平台说明
```

模型清单、适配器代码和下载工件分开发布。`catalog/` 中的清单不得包含本地绝对路径；路径由 daemon 在安装时解析到自己的数据目录。

## 数据目录

建议由平台目录规则决定根目录，并允许显式覆盖：

```text
<data-root>/
  state.db
  artifacts/sha256/ab/cd...       不可变原始文件
  prepared/<runtime>/<digest>/    prepare 生成的运行目录
  runtimes/<runtime>/<version>/   适配器环境
  instances/<instance-id>/        实例配置、凭据和日志
  tasks/<task-id>/                任务临时文件
```

数据库只保存索引和状态，不能作为权重内容的唯一来源。删除模型前检查实例引用和正在执行的任务；垃圾回收只删除没有引用的工件，并保留可审计事件。

## 第一条纵向切片

### Slice 1：服务骨架

- daemon 在回环地址监听，CLI 能启动、停止和查询 daemon。
- SQLite 完成初始迁移：models、variants、artifacts、tasks、instances、events。
- `GET /v1/models`、`GET /v1/tasks/{id}`、`GET /v1/instances` 可返回结构化状态。
- 进程监督器能记录 PID、端口、启动时间、退出码和日志路径。

退出条件：daemon 重启后能从数据库和实际进程探测结果恢复状态，不把旧记录直接当作 `ready`。

### Slice 2：Laya 清单与工件

- 固定 Laya 包版本、依赖锁和一个 checkpoint 的 Hub revision。
- 生成完整清单，逐项记录 URL、大小、SHA-256、许可证和能力 schema。
- 实现 HTTPS 下载、断点续传、临时文件、哈希校验和原子提交。
- `inspect` 在不加载模型的情况下报告平台、磁盘、内存和设备预检结果。

退出条件：人为破坏一个下载文件时，启动被拒绝并返回 `ARTIFACT_CORRUPT`；重复 `pull` 复用已校验文件。

### Slice 3：Laya 适配器

- 适配器 `hello/probe/prepare/start/health/invoke/stop` 对应协议 v0。
- 受维护的 Python 入口从准备目录创建 `Router`，传入上游 `create_app(router=...)`，仅绑定回环地址。
- `system_one` 请求和响应保持 Laya 原生 schema；`/v1/systemone` 代理明确绑定默认实例。
- 启动失败、端口冲突、设备不可用、模型加载失败和进程退出映射为结构化错误。

退出条件：CPU 路径完成一次 `choice`、`score`、`noul` 请求；实例停止后端口和资源记录释放；daemon 重启能识别实例实际状态。

### Slice 4：发布体验

- CLI 完成 `catalog`、`inspect`、`pull`、`run`、`invoke`、`ps`、`logs`、`stop`、`remove`。
- 增加 Linux/macOS 安装说明和数据目录迁移说明。
- 明确 Python 预览版前提；正式版是否内置 Python 另做发布决策。
- 记录模型来源、许可证、适配器版本和实际设备，避免只显示一个模糊的模型名。

正式发行包要为 Linux CPU、Linux CUDA、macOS CPU 和 macOS MPS 分别记录 Python/PyTorch 依赖集合；不能用一个通用的 `requirements.txt` 假设四种设备都可安装。

退出条件：新用户按 [首版用户流程](USER_FLOWS.md) 在一台目标机器上完成闭环，不需要手动编辑 Python 代码或启动第二个服务。

## 后续切片

1. **资源与加速**：Linux CUDA 和 Apple MPS 探针、显存/内存限制、profile 选择和真实基准。
2. **MCP 代理**：从能力目录生成允许的 MCP tools，共享已有实例，不重复加载权重。
3. **第二适配器**：固定 whisper.cpp 版本和 Whisper GGML 权重，加入音频临时目录、multipart/二进制输入和 `transcribe_audio`。
4. **Skill**：定义包元数据、能力依赖和最小权限；执行器只通过公共 API 调用。
5. **桌面界面**：在 API 稳定后实现，不复制模型生命周期逻辑。
6. **正式发行**：自动管理 Python 运行时、签名目录、适配器升级和安装包。

## 首版风险与处理

| 风险 | 影响 | 处理 |
| --- | --- | --- |
| 上游 Laya 内部 API 变化 | 薄入口失效 | 锁定 Laya 发行版，适配器启动时做版本检查，升级单独验证 |
| Hub 或网络中断 | 安装半成品 | 临时目录 + 断点续传 + 哈希后原子提交 |
| 权重/运行时目录被修改 | 复现和并发失败 | 原始工件不可变，prepare 生成私有可写配置 |
| GPU 依赖不匹配 | 启动失败或静默 CPU 回退 | `probe` 明确报告设备，显式 profile 不允许静默回退 |
| 不可信清单或插件执行代码 | 本地安全边界被绕过 | 清单声明性、插件显式安装、适配器权限和来源分开 |
| Laya 不是文本生成模型 | 用户误用 API | 能力发现和 CLI 文案明确 `system_one` 语义，不提供假聊天接口 |
