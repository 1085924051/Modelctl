# Modelctl 桌面端使用手册

Modelctl 帮你在本机下载和运行 Laya 模型，再把模型包装成可供业务程序调用的“能力”。模型权重不包含在安装包内，首次使用需要下载。当前公开安装包支持 macOS Apple Silicon、Windows x64 和 Linux x86_64。

## 安装与首次启动

从 GitHub Actions 中选择**成功完成**、提交版本为当前目标版本的 `Desktop self-contained runtimes` 构建，登录 GitHub 后下载对应桌面 artifact。桌面 artifact 与 `modelctl-runtime-*` 是不同的：后者只是运行时，不是可直接打开的客户端。**旧构建中的 `modelctl-desktop-windows-linux-self-contained` 同时包含 Windows 与两种 Linux 安装包，约 6 GB；Windows 用户不要再下载这个合并包。**

| 系统 | 选择的桌面 artifact | 打开方式 |
| --- | --- | --- |
| Windows x64 | `modelctl-desktop-windows-x64` 内的 `Modelctl-windows-x64-v*.zip` | 解压两次，保持 `Modelctl.exe` 与 `runtime` 文件夹同级，运行 `Modelctl.exe` |
| Linux x86_64 | `modelctl-desktop-linux-appimage-x86_64` 或 `modelctl-desktop-linux-tar-x86_64` | AppImage 加执行权限后运行；或解压 tar.xz 后按随包 README 启动 |
| macOS Apple Silicon | `modelctl-desktop-macos-self-contained` 内的 `.dmg` 或 `.zip` | 从 DMG/ZIP 打开 `Modelctl.app`；当前包为 ad-hoc 签名 |

如果浏览器下载 Actions artifact 时中断，先确认已登录 GitHub、磁盘有足够空间，并只选自己的平台包。Windows 用户也可安装 GitHub CLI 后在 PowerShell 中执行 `gh auth login`，再运行 `gh run download <构建编号> -R 1085924051/Modelctl -n modelctl-desktop-windows-x64 -D .\Modelctl-download`。其中 `<构建编号>` 是 Actions 页面网址末尾的数字。Actions artifact 下载后外面还有一层 ZIP，里面的 `Modelctl-windows-x64-v*.zip` 才是客户端安装包；需要解压两次。

程序默认在 `http://127.0.0.1:11435` 启动本机模型服务。首次启动时 Models 页面会先显示“正在连接模型服务”。如果出现“模型服务未就绪”，先检查完整解压、`runtime` 目录是否在正确位置，然后点击“重试连接”。不要只复制单独的可执行文件。

## 下载并试用模型

1. 打开左侧 **模型下载 / Models**。看到 **Laya** 卡片后选择 `english`（英文内容）或 `multilingual`（含中文等多语言内容）。
2. 点击卡片中的 **下载模型 / Download model**。等待进度完成并显示下载校验成功。模型约需数百 MB，下载位置在用户数据目录（默认 `~/.modelctl`，Windows 对应用户主目录下的 `.modelctl`）。
3. 点击 **加载并运行 / Load & run**。等待状态显示就绪。Windows/Linux 基础包默认提供 CPU 路径；macOS Apple Silicon 可使用兼容的 MPS 路径。首次加载可能需要一些时间。
4. 打开 **试用模型 / Playground**，在“引导表单”里填入待分析资料，并用一句话写出希望模型判断的业务问题。可以直接点击“填入示例”试用退款识别。答案形式可选“判断是否成立”“从选项中选择”“评定等级”；选项和等级每行写一个即可，无需了解 Laya 的内部类型。熟悉原始协议的开发者仍可切换 JSON 模式。

右上角可切换中文和 English，语言选择会保存在本机。切换时正在填写的试用表单会保留。业务能力页的接口格式、批量测试、接入代码放在可展开区域；鼠标停留在长文本输入区上滚动时，页面会继续滚动。输入长文本时可使用键盘方向键在输入区内移动。

下载模型与加载模型是两步。**下载模型 / Download model** 将权重保存到本机；**加载并运行 / Load & run** 将已下载的权重加载到运行时。也可以直接点击后者，缺少权重时程序会先下载。

## 让应用调用模型

1. 打开 **Capabilities**，选择“退款识别”“工单路由”等模板，填写输入字段、判断问题和输出类型。
2. 用真实样例执行单条或批量测试，再发布一个版本，例如 `refund-check@1.0.0`。
3. 查看该能力的 readiness。若提示 `model_not_installed`，下载它绑定的模型版本；若是 `not_started`，首次调用会自动启动运行时。
4. 打开 **Integration**，复制该能力的 Schema、OpenAPI 或 curl/Python/JavaScript 示例。业务程序只传业务字段，不需要构造 Laya 的 `state/questions`。

生产环境建议固定能力版本（`?version=1.0.0`），并保存返回的 `run_id`、`capability.version` 和自己的业务单号。完整接口见 [API 文档](api-guide.md)，端到端场景见 [使用案例](use-cases.md)。

## 常见问题

| 现象 | 检查方法 |
| --- | --- |
| Models 页面看不到 Laya 卡片 | 看页面中央的连接错误；检查完整解压与 `runtime` 目录，确认 `http://127.0.0.1:11435/health` 可访问后点 Refresh |
| 下载失败 | 在 Settings 配置代理后重试；下载支持断点续传，失败时查看页面提示 |
| 加载失败 | 先确认模型状态为 Downloaded，再查看设备选项；Windows/Linux 首先选 `auto` 或 `cpu` |
| 能力调用返回 `MODEL_NOT_INSTALLED` | 下载该能力绑定的 model ID 和 variant，而不是另一个语言版本 |
| 应用无法远程访问 | 桌面端默认只监听本机；企业内网部署需显式配置监听地址和 token，见 [API 文档](api-guide.md) |
