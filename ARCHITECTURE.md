# 本地开源模型部署工具：架构讨论稿

状态：讨论中。首版目标平台为 Linux 与 macOS。Laya 指 [NandhaKishorM/laya](https://github.com/NandhaKishorM/laya)，[Ollama](https://github.com/ollama/ollama) 是部署体验的参考。Jev 指 TypeSafe AI 的决策模型及其兼容协议；是否有可合法下载并本地运行的官方权重仍需核实。

## 已核实的现状（2026-09-24）

- Laya 是非自回归的结构化决策模型，接收 `state` 和带类型的 `questions`，输出 `choice`、`score`、`noul` 及概率，不生成聊天文本。上游提供 English、multilingual、typed-decisions 三种 checkpoint，以及自动路由。
- 上游 Python 包要求 Python 3.10+，提供 `laya-serve`、`laya-mcp-server` 和 Docker。`laya-serve` 已实现 Jev 兼容的 `POST /v1/systemone`，可用 `LAYA_DEVICE` 选择 `cpu`、`cuda`、`mps` 等 PyTorch 设备；默认绑定 `0.0.0.0:8000`，由本工具管理时必须覆盖为回环地址。
- Laya 在 Linux 可走 CPU 或符合条件的 NVIDIA CUDA；在 macOS 可走原生 Python + MPS。上游说明 Docker Desktop 的 Linux 容器不能使用 Apple MPS，故 macOS GPU 路径应采用原生进程。具体硬件组合仍需实机验证。
- Ollama 提供安装包/脚本、`run` 命令、模型管理和本地 REST API。我们参考其用户路径，不复用其以 llama.cpp 为核心的推理层来运行 Laya。
- Laya 已有独立服务与 MCP 接口。拟做工具的差异是跨模型的安装与版本、运行环境、资源、生命周期、统一发现与调用；不应声称 Laya 无法本地部署。

资料：[Laya README](https://github.com/NandhaKishorM/laya/blob/main/README.md)、[Laya pyproject.toml](https://github.com/NandhaKishorM/laya/blob/main/pyproject.toml)、[Laya Docker 平台说明](https://github.com/NandhaKishorM/laya/blob/main/docs/docker-platforms.md)、[Ollama README](https://github.com/ollama/ollama/blob/main/README.md)。Laya 资料核对时的 main 提交为 `84ca7240348e6fb3b8145d2d9f6266d5125747d8`；实现时应锁定正式发行版及权重 revision，不能依赖浮动的 `main`。

## 目标

让用户通过统一的命令和本地 API 完成模型的获取、校验、配置、启动、调用、监控、停止和删除。模型可以有不同的输入、输出、依赖和推理框架。工具还应为 MCP、Skill 等上层能力保留明确的扩展点。首版先把 Laya 的上游运行时纳入统一管理，保留接入其他开放模型及远端 Jev 兼容服务的边界。

一个期望的使用流程：

```text
tool pull <model-id>
tool inspect <model-id>
tool run <model-id> --profile <profile>
tool ps
tool stop <instance-id>
```

这些命令是产品接口草案，不代表已经确定命令名或实现。

## 核心边界

1. **模型管理**：处理模型元数据、权重及其他工件、版本、校验、许可证提示和本地缓存。
2. **运行管理**：选择适配器、检查硬件与依赖、分配资源、启动隔离的推理进程、健康检查和清理。
3. **推理接口**：暴露模型原生能力。文本生成、嵌入、图像、音频或其他结构化任务由能力描述决定；不能强行塞进 chat completion。
4. **扩展与编排**：MCP 和 Skill 位于调用模型的上层。MCP server 可供外部客户端发现和调用本地模型；Skill 定义可选的工作流与工具使用规则。它们不应成为安装或运行模型的必要条件。

## 建议的分层

```text
CLI / Desktop UI / 本地 HTTP API
             │
             ▼
控制面：目录、工件、配置、实例、资源、日志
             │
             ▼
运行时适配器接口 ── 进程监督与隔离
      │           │           │
  现有引擎适配器  Python 项目适配器  专用原生适配器
             │
             ▼
           模型进程

MCP server / Skill runner → 本地 API（可选扩展）
```

首版实现取舍：控制面建议用 Go 编写为一个常驻 daemon 和配套 CLI，Linux/macOS 打包简单，进程监督和本地 HTTP 服务可共用代码；Laya 适配器以独立 Python 虚拟环境运行上游 Laya 包及一个薄启动入口，用 HTTP 探活和代理 `/v1/systemone`。这只是一项工程建议，待真实模型与硬件验证后冻结。适配器协议对外先定义能力和生命周期，首个版本不需要自造新的推理算法。后续接入纯原生或 ONNX 运行时，可在同一适配器边界替换实现。

### Linux/macOS 的 Laya 运行矩阵

| 平台 | 首版路径 | 设备选择 | 主要限制 |
| --- | --- | --- | --- |
| Linux x86_64 / arm64 | 原生隔离的 Python 环境 | `cpu` | 内存与磁盘需按 checkpoint 核算；首次下载需能访问模型源 |
| Linux + NVIDIA | 原生 Python 环境；CUDA 依赖单独锁定 | `cuda` | 检查驱动、PyTorch CUDA 构建和显存；不得静默回退 CPU |
| macOS Apple Silicon | 原生 Python 环境 | `mps` | Docker 内无法使用 MPS；需检查 PyTorch MPS 可用性 |
| macOS Intel | 原生 Python 环境 | `cpu` | 性能与内存需实机评估 |

上表是支持方案，不是已经跑通的兼容性声明。Laya 的 English 权重文件约 843 MB，multilingual 权重约 644 MB；另外还需 tokenizer、配置、Python/PyTorch 依赖和运行时内存。首版默认只安装用户选择的 checkpoint；`auto` 路由应明确提示可能下载并加载多个 checkpoint。

安装体验建议分两层：CLI/daemon 作为独立发行包；每个适配器使用独立、受管理的 Python 环境并锁定包版本。用户不应因安装一个 Laya 模型而改动系统 Python。首版可要求机器有受支持的 Python 3.10+，但若产品目标是接近 Ollama 的单命令体验，应把自动获取受支持 Python 运行时列为正式发布前的交付项。CUDA 驱动仍属于宿主系统前提，工具负责探测和明确报错。

控制面负责共通生命周期。适配器负责某一类模型的依赖检查、加载、调用和能力声明。每个模型版本由一个清单连接到适配器、工件和运行配置；清单不能包含任意可执行安装脚本并自动运行。初期可以由项目维护已审查的适配器和清单，之后再设计第三方插件的信任与权限机制。

### 四个需要分开的对象

| 对象 | 含义 | 示例标识 |
| --- | --- | --- |
| 模型定义 | 某模型的一个确定版本、能力和来源 | `publisher/name@version` |
| 工件 | 权重、配置、分词器或其他下载文件 | 内容哈希 |
| 运行时 | 能加载一类模型的适配器和依赖环境 | `runtime-id@version` |
| 实例 | 某次启动后的进程、端点与资源占用 | 随机 `instance_id` |

这样可以让多个实例复用同一份工件，也能为同一模型提供不同运行时或量化配置。模型清单与用户机器上的实例状态应分开存放。

### 适配器契约（概念）

更具体的版本化消息、对象标识、错误和跨模型检验见 [适配器协议 v0 草案](ADAPTER_PROTOCOL.md)。模型来源、工件、能力和 profile 的声明格式见 [模型清单 v1 草案](MODEL_MANIFEST.md) 及其 [JSON Schema](schemas/model-manifest.schema.json)。

```text
probe(host, manifest) -> compatibility / required resources
prepare(artifacts, config) -> runnable environment
start(config, allocated resources) -> process handle + endpoint
health(handle) -> status
capabilities() -> operations + input/output schemas
invoke(operation, request) -> response or stream
stop(handle) -> cleanup
```

适配器至少需要说明支持的操作、请求和响应 schema、是否流式、需要的 GPU/CPU/内存、并发限制，以及错误类型。一次请求应携带取消信号与超时。大文件和多模态输入建议使用本地文件引用或流，避免将权重或大媒体文件塞进 JSON。

控制面与运行时适配器建议采用版本化的进程间协议。适配器作为子进程运行，通过标准输入输出或仅监听回环地址的连接通信；控制面不直接导入适配器的 Python 包。协议至少包含握手与版本协商、能力发现、启动、健康状态、推理、取消、关闭和结构化错误。适配器崩溃应只使对应实例失效，控制面记录退出原因并释放资源。

首个 Laya 适配器可用薄包装进程把现有 `/health` 和 `/v1/systemone` 映射到上述契约。其 `system_one` 能力应原样保留 `state`、`questions`、`model` 与概率输出。不能把它伪装成聊天或 OpenAI `chat/completions`。上游的模型自动路由可作为一个 profile，固定 English 或 multilingual checkpoint 则用单独 profile。MCP 侧优先代理已有能力；只有跨模型统一发现、权限和路由需要由本工具提供。

**权重版本固定的实现约束：**上游 `laya-serve` 的 `build_router()` 使用 Hub 默认模型名，`Agent` 内部调用 `snapshot_download` 时没有传入 `revision`。因此仅在本工具清单记录 Hub commit，仍不能保证上游服务使用该版本。首版应由工件管理器下载固定 revision、逐文件校验，再准备运行时模型目录，用极薄的 Python 入口创建 `Router(models={"english": "/.../english", ...})`，传给上游 `create_app(router=...)`，并启动 Uvicorn。Laya 可能修补 tokenizer 配置，故已校验的原始文件要保留为不可变工件；运行时使用配置副本。这样复用上游推理和 HTTP 行为，同时锁定权重来源。`laya-serve` 可作为手动接入的外部服务选项，但不能承担“确定版本安装”的验收。此入口需要随 Laya 包版本适配，不把上游内部函数当作永久稳定 API。

一次调用的路由是 `instance_id + operation`。如果用户只指定 `model_id`，控制面可以按配置选择或启动实例，但必须把实际使用的模型版本和实例写入响应元数据。流式调用要定义背压、取消后的清理和客户端断开后的行为。

### 清单（概念）

```yaml
schema_version: 1
id: example/model
display_name: Example model
version: 1.0.0
license:
  spdx: Apache-2.0
runtime:
  id: example-runtime
  version: 1.0.0
source:
  kind: curated
  revision: example-revision
variants:
  - id: default
    default: true
    artifacts:
      - path: model.bin
        uri: https://example.invalid/model.bin
        sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
        size_bytes: 123
        media_type: application/octet-stream
        executable: false
capabilities:
  - name: infer
    kind: structured_inference
    input_schema: schema://example/input/v1
    output_schema: schema://example/output/v1
    streaming: false
    media: [application/json]
profiles:
  - id: default
    device: auto
    max_concurrency: 1
```

每个实际清单仍需确定权重来源、许可证、可信发布者、依赖锁定、镜像来源、运行参数与升级策略。下载可断点续传，校验后原子地进入内容寻址缓存，多个模型版本可复用工件。

清单是声明性数据：哈希、大小、来源、协议版本、能力 schema、资源提示和运行时约束可以放入其中；任意 shell 命令不应在 `pull` 或 `run` 时隐式执行。对于必须运行上游安装脚本的模型，需先把安装过程封装进受维护的运行时适配器，或要求用户显式导入外部环境。

### 状态与失败处理

```text
模型工件：未安装 → 下载中 → 已校验 → 可用
                           ↘ 失败（可重试）

实例：创建中 → 就绪 → 停止中 → 已停止
             ↘ 失败     ↘ 失败
```

`pull` 和 `run` 都返回任务 ID，便于查询进度和取消。进程异常退出、显存不足、工件校验失败、适配器协议不兼容、依赖缺失要有不同错误码及下一步建议。重启服务后扫描实例记录与实际进程，清理陈旧状态；不能仅凭数据库字段认定模型仍在运行。

## 本地 API 与扩展

- 控制 API：列出模型、安装任务、实例状态、启动/停止、日志、资源用量。
- 推理 API：按 `model_id + operation` 路由，保留原生 schema。可为明确支持的模型额外提供 OpenAI 兼容端点，但不将其作为唯一协议。
- MCP 扩展：作为独立组件，把可调用的模型能力映射为 MCP tools/resources；权限和可见范围由本地配置控制。
- Skill 扩展：用版本化的 Skill 包描述工作流、提示词和需要的工具；执行器通过本地 API 或 MCP 调用模型。Skill 不能默认取得系统命令、网络、文件读写权限。
- 插件接口：先稳定适配器的进程间协议和清单 schema，再开放第三方适配器。插件版本与主程序协议版本分开管理。

### 公共接口草案

公共 API 只承诺通用生命周期与能力发现，模型操作保留其本身的 schema：

| 方法 | 路径 | 作用 |
| --- | --- | --- |
| `GET` | `/v1/models` | 列出可安装、已安装模型及其能力 |
| `GET` | `/v1/models/{id}` | 查看版本、工件、许可证、支持平台和 profile |
| `POST` | `/v1/pulls` | 创建安装任务；请求含模型 ID、确定版本与 checkpoint |
| `GET` | `/v1/tasks/{id}` | 查看下载、安装或启动进度与错误 |
| `POST` | `/v1/instances` | 选择模型版本及 profile，启动实例 |
| `GET` | `/v1/instances` | 查看实例、实际设备、健康状态与端点 |
| `DELETE` | `/v1/instances/{id}` | 停止指定实例 |
| `POST` | `/v1/instances/{id}/operations/{name}` | 按能力 schema 调用模型 |
| `POST` | `/v1/systemone` | Jev 兼容代理，路由到配置的 Laya 实例 |

`/v1/systemone` 需要一个明确的默认实例选择规则；若同时运行多个 Laya 实例且未配置默认值，应返回歧义错误，不应悄悄挑选。通用操作端点的请求/响应仍是 Laya 的 `state/questions/answers`，控制面在 HTTP header 或外层元数据标记模型版本和实例，不改写 Jev 兼容响应体。管理 API 与推理 API 应能分别配置访问范围。

MCP 扩展建议是一个**独立代理进程**，通过公共 API 查询能力并转为 MCP tool。首个 `system_one` 工具可以代理同一个 Laya 实例，避免上游 `laya-mcp-server` 再加载一套权重。Skill 包则引用稳定的能力名、输入 schema 与最小权限声明；具体工作流执行器可以后续实现。适配器插件、MCP 插件和 Skill 是三种不同的扩展，分别位于推理、协议和工作流层。

默认只监听 `127.0.0.1`。如果启用局域网访问，则必须显式设置监听地址与访问凭据，并对控制 API 和推理 API 分别授权。日志中避免记录完整输入、输出、访问令牌与本地媒体内容。模型下载源、适配器包和 Skill 包应分别标明来源与版本，避免把“下载模型”误当成授予执行代码的权限。

## 首版建议

先用 **CLI + 常驻本地服务 + Laya 适配器** 验证完整闭环：拉取、校验、兼容性检查、启动、一次结构化决策调用、停止、日志与失败恢复。Laya 适配器复用上游模型包和 HTTP 应用，以本工具的薄启动入口绑定已校验的本地 checkpoint。MCP 扩展经本工具 API 代理现有实例，避免额外加载权重；上游 `laya-mcp-server` 可作为独立运行的参考。第二个开放模型或运行时用来检验扩展性，选择时应优先与 Laya 有不同的任务或依赖。

首版以 Linux/macOS 单机为界；多机调度、市场、自动执行不可信安装脚本、通用训练/微调、完整 Agent 平台暂不纳入首版。MCP 作为随后可独立交付的代理扩展；Skill 先定义包格式和权限边界，再增加执行器。若需要 Jev 兼容性，首版直接提供 `/v1/systemone` 代理；连接 TypeSafe AI 的远端 Jev 服务与本地部署是两种不同能力，不能混称。

### 首版可验收的结果

1. 在 Linux 与 macOS 的目标机器上，用户能从已审查的模型清单安装 Laya 的一个确定版本；下载结果的哈希与清单一致。先选 CPU 为共同基线，再分别验证 Linux CUDA 和 macOS MPS 的设备路径。
2. `inspect` 能在启动前报告能力、依赖和资源要求；资源或依赖不满足时给出可操作的错误。
3. 用户能启动实例，调用 Laya 的 `system_one`，取得含 `choice`、`score`、`noul` 与概率的结果，并停止实例；Jev 兼容客户端能调用代理的 `/v1/systemone`。
4. 工件可复用；服务或模型进程重启后，实例状态、日志和占用资源不会错误地保持为“就绪”。
5. 适配器协议和模型清单已写成独立契约，首个 Laya 适配器只使用这些契约提供的字段与操作。第二个真实适配器的接入验证属于下一阶段，不能仅凭接口定义声称已经具备跨模型兼容性。

### 建议的推进顺序

| 阶段 | 要产出的决定或交付物 | 退出条件 |
| --- | --- | --- |
| 0. 模型调查 | Laya 已定位；补齐 Jev 官方权重可用性、第二模型候选、目标机器规格 | Laya 在 Linux/macOS 各有可复现的本地推理路径 |
| 1. 接口设计 | 清单 schema、适配器协议、本地 API、错误码 | 用两个不同模型的调用样例走通协议设计 |
| 2. 纵向切片 | CLI、服务、缓存、进程监督、首个真实适配器 | 达成上述首版验收项 1–4 |
| 3. 泛化 | 第二个真实适配器、MCP server | 不改变公共 API 接入第二类模型，并能由 MCP 客户端调用已运行的模型实例 |
| 4. 工作流 | Skill 包格式、权限配置、执行器 | 一个 Skill 能通过公开 API 完成可观察的工作流 |

阶段表是实施顺序草案；首发平台已定为 Linux/macOS，硬件范围和第二个模型确认后再定里程碑与技术栈。

## 需要先拍板的问题

1. Jev 是否指 TypeSafe AI 的官方托管模型？目前核对的 TypeSafe 官方文档与公开仓库展示的是 API/SDK，尚未找到可供本地部署的官方 Jev 权重。若要求本地部署 Jev，需取得权重与许可证；兼容 API 或社区替代模型不等于部署 Jev 本体。
2. Linux 目标机器是否主要使用 NVIDIA GPU？macOS 是否以 Apple Silicon 为主？最低 CPU、内存、显存与磁盘规格是什么？
3. 目标用户是个人开发者的单机工具，还是也包括局域网团队共享？这影响默认监听地址、鉴权和缓存位置。
4. 首版最重要的用户动作是命令行部署、桌面图形操作，还是让其他应用通过 API 调用？
5. 是否接受首版要求用户预装 Python/CUDA 等模型依赖，还是必须由工具完全管理环境？

## 下一步

下一步按 [Laya 适配器设计](LAYA_ADAPTER.md) 核对 Linux CPU/CUDA 与 macOS CPU/MPS 的实际路径，固定 Python 包版本和 Hugging Face 权重 revision，列出完整工件与资源要求。第二适配器候选为 Whisper + `whisper.cpp`，用于检验音频上传和原生 C/C++ 运行时；其模型版本与许可证仍需核定。随后细化清单 schema、进程间协议和第一版交互流程。首发平台已定为 Linux 与 macOS，具体加速设备仍待确认。
