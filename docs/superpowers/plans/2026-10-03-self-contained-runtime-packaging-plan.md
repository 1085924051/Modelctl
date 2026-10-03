# 自包含运行环境发布实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (\`- [ ]\`) syntax for tracking.

**Goal:** 将 Modelctl 三平台桌面发行物改造成无需用户安装 Node.js、Python、Laya 或创建虚拟环境即可启动的自包含应用，同时保留模型权重按需下载。

**Architecture:** Go/Fyne 桌面端新增 runtime supervisor，解析并校验应用包内的 \`runtime-manifest.json\`，启动随包 Node 控制服务并设置随包 Python/Laya 路径。平台构建在发布阶段分别下载并冻结 Node、Python、Laya/PyTorch runtime，把它们装入 macOS app bundle、Windows installer/ZIP 和 Linux AppImage；用户数据和模型权重仍放在 \`~/.modelctl\` 或平台等价目录。

**Tech Stack:** Go 1.25、Fyne 2.6.1、Node.js ES Modules、Python 3.10+、Laya 0.3.18、PyTorch、GitHub Actions、Docker/fyne-cross、macOS DMG、Windows installer/ZIP、Linux AppImage。

**Spec:** \`docs/superpowers/specs/2026-10-03-self-contained-runtime-packaging-design.md\`

## Global Constraints

- 运行时包内必须包含 Node.js、Python、Laya、PyTorch 及直接依赖，运行时禁止 \`npm install\`、\`pip install\` 或联网下载运行环境。
- 模型权重不随基础平台包分发，只能通过现有 catalog、代理、断点续传和 SHA-256 校验按需下载。
- macOS arm64 使用 MPS 路径；Windows x86_64 和 Linux x86_64 第一版基础包默认 CPU，不内置 CUDA。
- 客户端默认只绑定/访问 \`127.0.0.1:11435\`，远程 \`MODELCTL_URL\` 时不启动本地 daemon。
- 运行时清单必须包含目标平台、构建提交、运行时版本、许可证索引和关键文件 SHA-256。
- 不把 Fyne、Node、Python、PyTorch、Laya、模型权重或其许可证文本声明为申请人的第一方代码。
- 每个任务先写一个能失败的测试或离线 smoke check，再实现最小代码；不回退现有用户修改。

## Task 1: Runtime Manifest And Resource Layout

**Files:**
- Create: \`desktop/internal/runtime/manifest.go\`
- Create: \`desktop/internal/runtime/manifest_test.go\`
- Create: \`desktop/runtime/manifest.schema.json\`
- Create: \`desktop/runtime/README.md\`

**Interfaces:**
- \`runtime.Manifest\` 包含软件版本、构建提交、目标平台、Node/Python/Laya/PyTorch 版本、文件哈希和许可证索引。
- \`runtime.Resolve(appPath string) (Paths, error)\` 返回资源根目录、manifest、Node、control plane、Python 和日志路径。
- \`runtime.Verify(paths Paths) error\` 校验目标平台、必需文件和 SHA-256。

- [ ] 写测试覆盖有效 manifest、目标平台错误、缺少文件和哈希错误。
- [ ] 运行 \`cd desktop && go test ./internal/runtime -run 'TestManifest|TestResolve' -v\`，确认先因符号不存在而失败。
- [ ] 使用 \`encoding/json\`、\`crypto/sha256\`、\`runtime.GOOS\` 和 \`runtime.GOARCH\` 实现资源解析与校验；拒绝绝对路径和未知平台。
- [ ] 运行 \`gofmt -w desktop/internal/runtime && cd desktop && go test ./internal/runtime ./internal/api ./internal/ui\`。
- [ ] 提交：\`feat: add packaged runtime manifest validation\`。

## Task 2: Go Runtime Supervisor

**Files:**
- Create: \`desktop/internal/runtime/supervisor.go\`
- Create: \`desktop/internal/runtime/supervisor_test.go\`
- Modify: \`desktop/main.go\`
- Modify: \`desktop/internal/ui/app.go\`
- Modify: \`desktop/internal/api/client.go\`

**Interfaces:**
- \`runtime.NewSupervisor(paths Paths, dataDir string) *Supervisor\`
- \`Supervisor.Start(ctx context.Context) (baseURL string, err error)\`
- \`Supervisor.Close() error\`

- [ ] 用临时假 daemon 写失败测试：成功启动、健康检查超时、关闭只结束自己创建的进程。
- [ ] 运行 \`cd desktop && go test ./internal/runtime -run TestSupervisor -v\`，确认测试先失败。
- [ ] 用 \`os/exec.Cmd\` 启动随包 Node 的 \`bin/modelctl.js daemon\`，传入 \`MODELCTL_PYTHON\`、\`MODELCTL_RUNTIME_ROOT\`、\`MODELCTL_DATA_DIR\` 和代理变量。
- [ ] 增加 context 超时、日志文件、PID/所有权记录和子进程回收；不杀掉用户独立启动的同端口 daemon。
- [ ] 修改 \`main.go\` 和 Fyne 生命周期：无 \`MODELCTL_URL\` 时启动本地 supervisor，有远程 URL 时跳过；manifest 错误显示可执行提示。
- [ ] 运行 \`cd desktop && gofmt -w . && go test ./...\`。
- [ ] 提交：\`feat: supervise bundled modelctl runtime\`。

## Task 3: Control Plane Runtime Configuration

**Files:**
- Create: \`desktop/runtime/runtime.env.example\`
- Create: \`desktop/runtime/licenses/README.md\`
- Modify: \`src/core.js\`
- Modify: \`src/cli.js\`
- Modify: \`src/server.js\`
- Modify: \`scripts/setup-laya.sh\`
- Modify: \`scripts/setup-laya.ps1\`
- Create: \`test/runtime-config.test.js\`

- [ ] 写 Node 测试：\`MODELCTL_PYTHON\` 优先于开发 venv；control plane 资源从 \`MODELCTL_RUNTIME_ROOT\` 读取；只写 \`MODELCTL_DATA_DIR\`。
- [ ] 运行 Node focused test，确认当前行为失败。
- [ ] 实现环境变量优先级和只读资源路径校验；没有变量时保持现有开发模式兼容。
- [ ] 运行 \`npm test\` 以及 \`MODELCTL_RUNTIME_ROOT=/tmp/modelctl-runtime MODELCTL_DATA_DIR=/tmp/modelctl-data node bin/modelctl.js doctor\`，确认资源目录没有写入。
- [ ] 提交：\`feat: support read-only packaged control plane\`。

## Task 4: Platform Runtime Builders

**Files:**
- Create: \`desktop/runtime/build-runtime.mjs\`
- Create: \`desktop/runtime/build-runtime.ps1\`
- Create: \`desktop/runtime/build-runtime.sh\`
- Create: \`desktop/runtime/requirements/macos-arm64.in\`
- Create: \`desktop/runtime/requirements/windows-amd64.in\`
- Create: \`desktop/runtime/requirements/linux-amd64.in\`
- Create: \`desktop/runtime/licenses/generate-notice.mjs\`
- Create: \`desktop/runtime/tests/verify-runtime.mjs\`

- [ ] 在已验证开发机上记录 Node、Python、Laya、PyTorch 和直接依赖版本，写入三平台依赖输入；Windows/Linux 使用 CPU 依赖，macOS 使用 MPS 兼容依赖。
- [ ] 先写离线 verifier，覆盖缺 manifest、目标错、缺 Node/Python、哈希错和有效 runtime；运行后确认无效样例失败。
- [ ] 构建 control plane 目录：复制 \`bin\`、\`src\`、\`web\`、\`catalog\`、\`schemas\`、\`community\` 和最小 package metadata，不复制模型权重、测试或 \`node_modules\`。
- [ ] 构建随包 Node 和 Python/Laya runtime，在构建阶段安装 wheel，运行时禁止 pip/npm。
- [ ] 生成 runtime manifest、SHA-256 和 NOTICE，并离线执行 \`node desktop/runtime/verify-runtime.mjs <runtime-dir>\`。
- [ ] 提交：\`build: bundle platform modelctl runtimes\`。

## Task 5: Platform Packages And CI

**Files:**
- Modify: \`desktop/build-macos.sh\`
- Modify: \`desktop/release-macos.sh\`
- Modify: \`desktop/release-cross.sh\`
- Modify: \`.github/workflows/desktop-cross-build.yml\`
- Create: \`.github/workflows/desktop-runtime-build.yml\`
- Create: \`desktop/packaging/windows-installer.iss\`
- Create: \`desktop/packaging/appimage.yml\`
- Modify: \`desktop/packaging/README-windows.txt\`
- Modify: \`desktop/packaging/README-linux.txt\`
- Modify: \`desktop/RELEASE_NOTES.md\`

- [ ] 给发行脚本增加 package-layout assertions：缺 manifest、Node、Python、control plane 或 supervisor 时构建失败。
- [ ] macOS 将 runtime 放入 \`Contents/Resources/runtime\`，复制资源后再签名并验证。
- [ ] Windows 同时产出带 runtime 的 installer 和便携 ZIP；快捷方式必须指向 supervisor。
- [ ] Linux 产出 AppImage，保留 tar.xz 作为开发备用包；AppImage 内含 runtime 和 AppRun。
- [ ] CI 使用 macOS arm64、Windows x86_64、Linux x86_64，上传分平台 artifacts 和 checksum。
- [ ] 运行 \`sh -n desktop/release-macos.sh desktop/release-cross.sh && git diff --check\`，提交：\`build: ship self-contained desktop packages\`。

## Task 6: Clean Environment Smoke Tests

**Files:**
- Create: \`desktop/tests/smoke-runtime.sh\`
- Create: \`desktop/tests/smoke-runtime.ps1\`
- Create: \`desktop/tests/runtime-fixtures/README.md\`
- Modify: \`.github/workflows/desktop-runtime-build.yml\`
- Modify: \`README.md\`
- Modify: \`desktop/README.md\`

- [ ] 先对当前 client-only 包运行 smoke test，确认在无外部 Node/Python 时失败，证明测试能抓住原问题。
- [ ] 实现 smoke test：清理外部 PATH，使用临时 data dir 和小型 fixture catalog，轮询 \`/health\`、\`/v1/models\`，检查资源目录只读和子进程回收。
- [ ] 在 macOS 本地运行；Windows/Linux 在对应 CI runner 运行。
- [ ] 加入真实发布验收清单：无网络可启动、配置代理后拉取固定 revision、最小 system_one、包哈希和许可证文件。
- [ ] 提交：\`test: verify desktop packages without external runtimes\`。

## Final Verification

- [ ] \`cd desktop && go test ./...\`
- [ ] \`npm test\` 和三个目标 runtime 的离线 verifier
- [ ] 发行脚本语法、package-layout assertions、\`git diff --check\`
- [ ] macOS、Windows、Linux clean-environment smoke test
- [ ] 每个平台包 checksum 和 archive integrity
- [ ] 每个平台打开应用，记录 \`/health\`、\`/v1/models\`、模型拉取和最小 \`system_one\` 结果
- [ ] 用最终源码提交重新生成软著准备包的源码快照、第三方 runtime 边界和版本清单
