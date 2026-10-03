# Modelctl 自包含运行环境发布设计

## 目标

让 `Modelctl本地大模型部署与运行管理软件 V0.1.0` 在 macOS、Windows 和 Linux 上安装后即可启动，不要求用户额外安装 Node.js、Python、Laya、PyTorch 或创建虚拟环境。模型权重仍然按需下载并保存到用户数据目录。

## 范围

本设计覆盖桌面客户端、Node.js 控制服务、Python/Laya 适配器、三平台运行时打包、首次启动和发行版验收。它不改变现有模型目录协议、HTTP API 或 Laya `system_one` 输入输出结构。

明确不包含：

- 不把模型权重预装进平台安装包。
- 不承诺没有显卡驱动也能使用 GPU；平台默认 profile 由主机能力决定。
- 不把上游 Laya、PyTorch、模型权重或其他依赖声明为申请人的第一方代码。
- 不在运行时联网下载 Node.js、Python 或依赖包；这些运行时必须在构建阶段进入发行包。

## 用户体验

1. 用户安装平台包并打开 Modelctl。
2. Go 桌面端解析自身资源目录，校验内置运行时清单，启动随包 Node 控制服务。
3. 控制服务使用随包 Python/Laya runtime 启动适配器；所有进程绑定本机回环地址 `127.0.0.1:11435`。
4. 客户端等待 `/health` 成功后展示模型页；runtime 缺失、校验失败或端口被占用时显示可执行的修复提示。
5. 用户选择模型变体后，Modelctl 只下载模型权重和模型配置，下载过程继续使用设置页中的 HTTP/HTTPS/ALL Proxy。
6. 用户关闭客户端时，桌面端结束自己启动的 daemon 和适配器子进程；不会杀掉由用户独立启动的同端口服务。

## 运行时布局

桌面包按平台使用同一逻辑目录，路径差异由 Go supervisor 处理：

```text
Modelctl.app/Contents/Resources/           # macOS
Modelctl/                                  # Windows ZIP/installer
usr/lib/modelctl/                          # Linux AppImage
  runtime-manifest.json
  node/
    node(.exe)
  control-plane/
    package.json
    bin/
    src/
    web/
    catalog/
    schemas/
    community/
  python/
    python(.exe)
    lib/...
  laya/
    installed package files and native libraries
```

`runtime-manifest.json` 必须包含：软件版本、运行时构建版本、目标平台、Node/Python/Laya/PyTorch 版本、文件 SHA-256、构建提交号和第三方许可证索引。桌面端启动前先校验 manifest 中的关键文件；失败时禁止启动 daemon，并给出资源目录和重新安装提示。

## 进程监督

新增 Go supervisor 层，不把启动逻辑放进 Fyne 页面：

- `runtime.Resolve()`：根据应用可执行文件位置、macOS bundle Resources、Windows 安装目录和 Linux AppImage 目录解析资源根目录。
- `runtime.Verify()`：读取 manifest，校验版本、目标平台和关键文件 SHA-256。
- `runtime.StartDaemon()`：用内置 Node 可执行文件启动 `control-plane/bin/modelctl.js daemon`，设置 `MODELCTL_RUNTIME_ROOT`、`MODELCTL_DATA_DIR` 和用户代理变量。
- `runtime.WaitReady()`：轮询 `/health`，使用有限超时和明确错误分类。
- `runtime.Stop()`：只结束 supervisor 创建的子进程树，记录 PID 和日志路径。

客户端仍通过 `api.Client` 访问 HTTP API，但默认由 supervisor 返回实际 daemon URL。用户显式设置 `MODELCTL_URL` 指向远端服务时，客户端不启动本地 daemon。

## Python 与 Laya 运行时

构建阶段为三个目标平台分别生成可复制的 Python runtime 和依赖集合：

- macOS arm64：使用支持 Apple Silicon/MPS 的 Python 与 PyTorch/Laya 组合。
- Windows x86_64：默认 CPU 运行时；CUDA 不作为基础包依赖。
- Linux x86_64：默认 CPU 运行时；CUDA 作为后续独立加速包，不阻塞基础包发布。

当前 `scripts/setup-laya.sh` 和 `scripts/setup-laya.ps1` 只负责开发机在线安装，不能直接作为发布包运行时。发布构建必须生成平台锁定的依赖清单、wheelhouse/安装目录和许可证清单，并在干净机器上验证不访问 PyPI 即可导入 `laya`、`torch` 和适配器模块。

## 模型下载边界

模型权重不放入基础发行包。首次使用时：

- 控制服务从固定 revision 的 catalog 下载模型工件。
- 下载使用现有断点续传、临时文件、大小校验和 SHA-256 校验。
- 代理来自应用设置或环境变量，不能要求用户修改系统全局代理。
- 权重存放在 `MODELCTL_DATA_DIR`，平台卸载不默认删除用户模型。
- 没有网络或代理不通时，客户端必须保留“runtime 已就绪、模型未安装”的状态，而不是显示为 Offline。

## 平台发行物

### macOS

- 继续提供 DMG，但应用包内包含 Node/Python/Laya runtime。
- 保留 arm64 目标；在代码签名配置缺失时只生成开发用 ad-hoc 包，并在发行说明中明确。
- Developer ID + notarization 作为公开发布门槛，不伪装成已签名产品。

### Windows

- 优先提供安装程序，安装目录包含客户端和 runtime；同时保留 ZIP 便携包。
- 安装程序创建开始菜单快捷方式，快捷方式指向 supervisor 启动流程，而不是直接执行裸客户端。
- 不要求用户配置 PowerShell 执行策略；启动器使用签名/打包后的本地 supervisor。
- 未签名时保留 SmartScreen 说明；正式公开发布需要代码签名证书。

### Linux

- 基础发行物改为 AppImage，保证用户不需要安装 Node/Python。
- AppImage 内部包含控制服务、Python/Laya runtime 和桌面资源；模型仍写入用户目录。
- 同时提供 tar.xz 作为开发和服务器环境备用包，但 AppImage 是普通桌面用户的推荐入口。

## 版本与许可证

发行包必须同时携带：

- 第一方源码提交号和构建时间。
- Node、Python、Laya、PyTorch、Fyne 及其直接依赖版本。
- 每个平台的第三方许可证和 NOTICE 文件。
- 模型 catalog 的上游许可证、revision 和“权重不随包分发”的边界说明。

第一方软件著作权材料只覆盖申请人依法享有的代码和文档，不覆盖随包分发的第三方 runtime、开源库、模型权重或其许可证文本。

## 验收标准

每个平台必须在没有预装 Node.js、Python、pip、Laya 或项目源码的干净环境中完成：

1. 安装/解压后打开客户端，状态显示 `Connected`，不能显示 `Offline`。
2. `/health`、`/v1/models` 和设置页可用。
3. 不联网时仍能打开客户端并显示模型未安装；不能把“模型未下载”误报成 runtime Offline。
4. 配置代理后，可以下载一个固定 revision 的模型变体并通过 SHA-256 校验。
5. 启动一个实例并完成最小 `system_one` 调用；平台不支持的设备必须给出 profile 错误，而不是崩溃。
6. 关闭客户端后，supervisor 创建的 Node/Python 子进程全部退出，用户自行启动的服务不受影响。
7. 删除/移动应用包后，用户模型和设置目录仍按文档规则保留或可迁移。
8. 包内关键 runtime 文件与 `runtime-manifest.json` 的 SHA-256 一致。

## 分阶段交付

### 阶段一：运行时资源与监督器

实现资源解析、manifest 校验、Node daemon 启动、健康检查、进程回收和缺失资源错误界面。先使用现有开发机 runtime 目录做本地集成测试。

### 阶段二：平台 runtime 构建

分别产出 macOS arm64、Windows x86_64、Linux x86_64 的 Node/Python/Laya 固定 runtime，记录版本和许可证，禁止运行时 pip/npm 安装。

### 阶段三：发行包

改造 macOS DMG、Windows installer/ZIP、Linux AppImage 构建流程，把 runtime、manifest、NOTICE 和启动资源放进包内。

### 阶段四：干净环境验收

在 CI 或隔离虚拟机中执行三平台 smoke test，验证“无外部环境即可启动、无网络仍可进入客户端、模型按需下载、模型实例可运行”。

## 未决决策

本设计默认基础包使用 CPU 兼容路径，macOS arm64 额外启用 MPS；Windows/Linux 的 CUDA 不进入第一版基础包。若需要 CUDA 基础包，应作为另一个平台变体单独构建，因为它会显著增加体积、驱动约束和验收矩阵。
