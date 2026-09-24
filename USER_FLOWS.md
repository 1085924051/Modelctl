# 首版用户流程草案

本文把架构转换成用户可执行的流程。命令名、端口和 JSON 字段属于草案，需在实现前与公共 API 一起冻结。当前按 CLI + 本地 API、Linux/macOS CPU 共同基线的推进假设描述；CUDA、MPS、桌面界面和免预装 Python 仍未完成产品拍板。

## 1. 安装并查看 Laya

```bash
tool daemon start
tool catalog show laya
tool inspect convaiinnovations/laya --variant english --profile auto
```

`inspect` 只做读取和预检，不下载完整权重。输出至少包括：模型版本与来源、variant、能力名与 schema、许可证、所需磁盘空间、预计常驻内存、可用 profile、当前主机是否支持 CPU/CUDA/MPS，以及不支持时的修复建议。

## 2. 拉取确定版本

```bash
tool pull convaiinnovations/laya \
  --revision cf7c54c0586eede67d827dfaab8cd2d2007273e \
  --variant english
tool task <task-id>
```

CLI 显示下载进度、暂存空间和校验结果。只有所有声明文件完成校验，模型才进入 `installed`。中断后可以继续；校验失败的暂存文件不能被启动流程复用。`pull` 不执行模型仓库里的任意脚本。

## 3. 启动实例

```bash
tool run convaiinnovations/laya \
  --revision cf7c54c0586eede67d827dfaab8cd2d2007273e \
  --variant english --profile auto
tool ps
tool logs <instance-id>
```

实例状态依次显示 `preparing`、`starting`、`ready` 或带错误码的 `failed`。`ready` 之前不能把端点交给调用方。输出记录实际使用的设备、已加载 variant、运行时版本和本地端口；`auto` 设备回退 CPU 时必须显式提示。

## 4. 调用 Laya

通用调用：

```bash
tool invoke <instance-id> system_one --json request.json
```

`request.json`：

```json
{
  "state": {"body": "We were charged twice. Please refund us."},
  "questions": {
    "department": {
      "type": "choice",
      "instructions": "Which team should handle this?",
      "criteria": {"billing": "payments and refunds", "support": "technical help"}
    },
    "urgent": {
      "type": "noul",
      "instructions": "Does this require immediate action?"
    }
  }
}
```

同一能力经 HTTP：

```http
POST /v1/instances/{instance_id}/operations/system_one
Content-Type: application/json

{ "state": {"body": "..."}, "questions": {"urgent": {"type": "noul", "instructions": "..."}} }
```

同一实例也可通过 Jev 兼容入口调用：

```http
POST /v1/systemone
Content-Type: application/json

{ "state": {"body": "..."}, "questions": {"urgent": {"type": "noul", "instructions": "..."}} }
```

Jev 兼容入口必须有默认实例或显式实例选择规则；多实例且无默认值时返回歧义错误。响应保留 Laya 的 `answers`、概率、`usage` 和 `routing`，并在通用 API 的外层元数据中给出 `instance_id`、模型 revision、variant、runtime 和实际设备。

## 5. 停止与卸载

```bash
tool stop <instance-id> --grace 10s
tool remove convaiinnovations/laya --variant english
```

停止实例只释放进程和运行时资源，不删除已校验工件。`remove` 需要先检查是否有实例引用；带 `--purge` 才删除权重缓存，并显示将释放的空间。正在调用时先拒绝新请求，等待宽限期后终止，日志记录未完成请求。

## 6. Whisper 适配器的后续流程

候选适配器可以使用相同的控制面：

```bash
tool pull ggml-org/whisper --variant base.en
tool run ggml-org/whisper --variant base.en --profile auto
tool transcribe <instance-id> --file ./meeting.wav --format json
```

`transcribe` 是 CLI 便利命令，内部仍调用能力名 `transcribe_audio`。控制面负责读取文件、限制大小和放入受管理的临时目录；适配器不接收任意绝对路径。输出可以包含文本、语言和时间戳。若上游需要 ffmpeg，依赖应在适配器 profile 中声明并在 `inspect` 阶段检查。

## 7. MCP 和 Skill

MCP 代理启动后读取能力目录，只暴露用户允许的已运行实例：

```text
MCP tool: laya_system_one
  -> instance_id + capability=system_one
  -> control API invoke
  -> structured answers
```

Skill 包只声明能力与权限，例如 `requires: [system_one@v1]`、`permissions: []`。执行器通过控制 API 调用，不能在 Skill 中内嵌下载脚本或直接启动任意进程。后续的 Whisper Skill 可声明音频文件读取权限和最大输入大小。

## 首版操作闭环

```text
catalog/inspect
      ↓
pull -> 校验 -> installed
      ↓
run -> preflight -> start -> health -> ready
      ↓
invoke system_one -> structured response
      ↓
logs/status -> stop -> stopped
```

这条闭环是首版的最小产品主线。桌面界面可以在不改变语义的情况下调用相同的控制 API；第二适配器、MCP 代理和 Skill 执行器应复用它们，而不是另建一套模型生命周期。
