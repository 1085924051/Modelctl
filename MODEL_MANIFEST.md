# 模型清单 v1 草案

清单是模型目录的声明性入口。它描述模型是什么、从哪里来、需要哪些不可变工件、支持哪些能力和运行 profile。清单不包含安装脚本、shell 命令或任意可执行路径。机器上的工件内容必须根据清单逐项校验；清单本身也应通过 [JSON Schema](schemas/model-manifest.schema.json) 校验。

## 示例

```yaml
schema_version: 1
id: convaiinnovations/laya
display_name: Laya
version: 0.3.18
publisher:
  name: Convai Innovations
  homepage: https://github.com/NandhaKishorM/laya
  repository: https://github.com/NandhaKishorM/laya
license:
  spdx: Apache-2.0
  weights: Apache-2.0
runtime:
  id: laya-python
  version: 0.3.18
  adapter_protocol: v0
source:
  kind: huggingface
  revision: cf7c54c0586eede67d827dfaab8cd2d2007273e
variants:
  - id: english
    default: true
    artifacts:
      - path: model.safetensors
        uri: https://huggingface.co/convaiinnovations/laya/resolve/cf7c54c0586eede67d827dfaab8cd2d2007273e/model.safetensors
        sha256: REPLACE_WITH_64_HEX_DIGEST
        size_bytes: 842609210
        media_type: application/octet-stream
        executable: false
      - path: rl_agent_config.json
        uri: https://huggingface.co/convaiinnovations/laya/resolve/cf7c54c0586eede67d827dfaab8cd2d2007273e/rl_agent_config.json
        sha256: REPLACE_WITH_64_HEX_DIGEST
        size_bytes: 0
        media_type: application/json
        executable: false
capabilities:
  - name: system_one
    kind: structured_decision
    input_schema: schema://laya/system-one-input/v1
    output_schema: schema://laya/system-one-output/v1
    streaming: false
    media: [application/json]
    max_concurrency: 1
profiles:
  - id: auto
    device: auto
    platforms: [linux-x86_64, macos-arm64]
    max_concurrency: 1
```

示例中的摘要和大小占位值不能直接发布。真实清单必须列出完整 checkpoint 文件集合，包括 tokenizer、encoder 和配置，并从固定 revision 重新计算每个文件的 SHA-256。其他 variant 应作为额外的完整条目发布，不能用空的 `artifacts` 列表占位。

## 约束

- `id + version + variant` 标识一个可安装模型变体；Hub revision 是来源锁定信息，不能用浮动分支替代。
- `artifact.path` 是准备目录内的相对路径，禁止绝对路径和 `..` 逃逸；`uri` 首版只接受 HTTPS。
- 清单只能声明工件和参数，不能执行下载后脚本。适配器负责把工件准备为可运行目录，但准备过程属于受维护的适配器代码。
- `runtime` 指向适配器实现版本，和模型权重版本分开。适配器升级可能需要重新执行 `prepare`。
- `capabilities` 描述原生调用 schema；Laya 的 `system_one` 不被转换为聊天接口。MCP 和 Skill 引用能力名及 schema 版本。
- `profiles` 描述设备、平台和资源提示。profile 的 `config` 只能是声明性值，不允许 shell 命令、脚本或任意可执行文件路径。
- `extensions` 只用于命名空间化的元数据；未知扩展不得改变安全规则或自动获得权限。

## 目录发布与信任

首版目录由项目维护并随发布版本签名或校验。第三方目录、适配器和 Skill 的信任模型另行设计；模型清单来源不等于适配器代码来源。安装前显示许可证、来源、文件大小和预计磁盘占用；用户显式确认后才下载需要额外许可或较大空间的模型。

## Laya 目录实例

`catalog/laya-0.3.18.json` 是第一份实际目录清单，锁定 Laya Hub revision `cf7c54c0586eede67d827dfaab8cd2d2007273e`，包含 English 与 multilingual 两个 variant 的完整运行所需文件。大文件的 SHA-256 使用 Hugging Face tree API 返回的 `lfs.oid`；普通 Git 文件则由固定 revision 的实际下载内容计算 SHA-256。Hub API 中普通文件的 `oid` 是 Git blob SHA-1，不能当作工件 SHA-256 使用。

该清单的摘要和大小是资料冻结结果，不等于已完成下载校验；实现 `pull` 时仍需对下载字节重新计算 SHA-256，并拒绝与清单不一致的响应。
