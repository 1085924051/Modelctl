# Laya/Jev 真实场景应用规划

**状态：**设计与规划稿，不代表金融交易或机器人执行能力已经实现。  
**范围：**Modelctl Go 桌面客户端、多模型输入结构化层、Laya/Jev 决策执行及后续场景适配。

## 1. 概念边界

- **Laya**：本仓库当前运行的结构化决策模型，接受 `state` 和多个 `questions`，输出 `noul`、`choice`、`score` 及概率/置信度。它不是文本生成模型。
- **Jev**：TypeSafe 的结构化决策协议。本地 `/v1/systemone` 保持 Jev 兼容形状，不等于部署了 TypeSafe 官方 Jev 权重。
- **桌面客户端**：连接本机 Modelctl daemon，负责选模型/变体、管理实例、构造判断请求和查看结果。模型生命周期与权重仍由 daemon 管理。
- **输入结构化模型（LLM/VLM）**：可选的前处理器，把文本、表格、图片、音频或视频提炼为受 schema 限制的事实；它不是 Laya 的替代品，也不能在缺少证据时自行创造事实。
- **策略/执行层**：消费模型建议，并实施确定性规则、权限检查、风险阈值、人工确认和审计。不得把模型概率直接解释为可自动交易或机器人安全许可。

当前 Laya API 只接收 JSON `state/questions`。虽然 `state` 可以是文本、对象或对话轮次列表，但这不表示当前 checkpoint 可以直接理解图片/音频/视频。多模态数据必须由明确的 VLM/ASR/感知适配器预处理，或等待经过验证的原生多模态适配器。

## 2. 目标架构

```text
现场输入 / 市场数据 / 桌面粘贴 / API
                  |
                  v
输入连接器：文本、表格、图片、音频、视频、传感器
                  |
       可选的 LLM / VLM / ASR 前处理器
       提取事实 + 来源 + 时间 + 不确定性
                  |
                  v
Schema 校验 / 缺失信息检查 / 规则与权限门
                  |
                  v
Laya system_one / Jev 兼容本地 API
                  |
                  v
策略决策：阈值、限额、拒绝/升级、人工审批
                  |
          +-------+--------+
          |                |
     建议/告警          受控执行器
          |          订单审批/ROS 2/工单 API
          +-------审计与回放-------+
```

每一步保留独立接口和模型标识。前处理器输出必须可检查、可编辑、可重跑。原始输入通过受管媒体引用传递，不把大文件/base64 塞进普通 JSON，也不让模型任意读取本机文件路径。

### 桌面客户端的产品分区

1. **Models**：catalog、变体、平台兼容性、磁盘需求、Pull 进度、Run/Stop。
2. **Playground**：直接编辑 `state` 和多条 `noul/choice/score` 问题；JSON 模式支持完整协议与对话状态；展示每题结果、概率、置信度、路由和原始 JSON。
3. **Pipelines（后续）**：选择输入连接器和前处理模型，预览结构化 state/questions，再执行 Laya。
4. **Runs（后续）**：保存请求、模型/提示版本、前处理提取、Laya 输出、规则结果和人工审批；敏感数据须本地加密/保留期可配置。
5. **Settings**：daemon URL、代理、默认设备，以及后续 VLM/vLLM 服务连接。凭据应由系统 Keychain/Secret Service 管理，不能明文写日志。

## 3. 场景方案

### 金融量化研究与风控辅助

**合理用途**

- 把公告/研究文本、风险规则、交易计划整理成明确事实，再让 Laya 对固定问题作结构化判断：是否缺少必要信息、风险级别、事件类型、是否需要人工复核。
- 对研究员制定的候选策略进行标签分类、事件优先级排序和规则一致性检查。
- 对已有回测或模拟结果进行离线归因标签与风险问卷，不参与实盘订单路径。

**不允许的默认设计**

- 不以 `noul` 概率直接换算买卖信号或仓位。
- 不由 Laya/VLM 自动提交交易、绕过风控限额或替代持牌/授权决策人。
- 不在回测中使用未来数据；特征时间、市场时区、修订公告和数据版本必须留档。

**落地步骤**

1. 接入只读行情/公告快照，记录供应商、symbol、exchange、event time、ingest time 和数据哈希。
2. 使用确定性 parser 优先解析结构化字段；只有非结构化文本才交给 LLM 抽取。
3. LLM 输出 `schema_version` 事实对象和证据摘录；校验时间、数值、单位、字段来源，缺字段返回 `unknown` 而非猜测。
4. Laya 对预定义问题集合输出类型化判断；记录 `model revision`、variant、设备、问题版本和概率。
5. 确定性风险规则做阈值、限额和冲突检查；阶段一只产生研究标签/告警。
6. 在历史时间切分和 walk-forward 数据上评估校准、漂移、漏报/误报、阈值敏感性及不同市场状态；与人工基线对照。
7. 只有独立策略团队批准后，才进入 paper trading；实盘执行仍由现有 order gateway 及硬限额控制，不由桌面客户端执行。

### 具身智能与机器人任务门控

**合理用途**

- 用 VLM/视觉 pipeline 从相机帧中提取目标、障碍、人员存在、物体状态和观察时间。
- Laya 判断任务是否满足进入下一阶段的前置条件，例如“目标物体是否在工作区”“是否检测到人员进入禁区”“夹爪是否需要二次确认”。
- 决策输出只能选择已注册的有限动作标签，交给 ROS 2 行为树/状态机执行。

**安全边界**

- Laya 不是实时控制器，不直接输出关节位置、速度、电流、刹车或连续轨迹。
- `P(true)`、score、choice 都不是安全完整性等级认证；概率高不构成安全放行。
- 急停、安全 PLC、碰撞/力矩限制、地理围栏、看门狗和速度限制独立于 LLM/VLM/Laya。
- 图像/视频可能包含提示注入或伪造标识；视觉提取与用户/任务指令分离，并对执行动作使用 allowlist。
- 传感器过期、遮挡、低置信度、连接超时或模型版本未知时统一进入 stop/hold/人工检查状态。

**落地步骤**

1. 先在 rosbag/仿真回放做只读感知与场景问答，不接实际 actuator。
2. 定义机器人域 state schema：robot_id、map/frame、timestamp、pose 摘要、目标与障碍列表、传感器健康度、来源帧引用。
3. VLM 输出候选事实与空间坐标时，需由视觉/几何模块验证 frame、单位、深度和坐标变换；不让语言模型编造几何量。
4. Laya questions 只对应离散门控，如 `human_in_workspace`、`object_graspable`、`task_step_allowed`；allowed action 取有限枚举。
5. 确定性安全门检查传感器时效、禁区、碰撞距离、模式切换和操作员权限；模型“不确定/异常”时拒绝动作。
6. 在 Gazebo/Isaac/真实硬件的影子模式记录建议与人工/传统算法差异，测量漏检及延迟；达到预设安全验收后才由机器人团队批准有限动作试点。

### 其他适配场景

- **客服/工单**：从用户消息提取事实和语言，再由 Laya 做部门分流、退款意图和紧急度判断；最终退款/拒绝仍由业务规则或员工确认。
- **制造质检**：VLM 标注缺陷候选和证据区域，Laya 对检验标准做有限分类；图像原片和质检设备判定保留，模型结果不能抹除证据。
- **医疗/法律/招聘/信贷**：涉及高影响决策，MVP 只做检索/整理/辅助标签；必须有专业人员审阅、公平性和适用性验证，不自动决定个人资格或权益。
- **智能家居/运维**：告警归因和 runbook 路由；破坏性命令、门锁、电源、生产变更需独立确认。

## 4. LLM/VLM/vLLM 前处理设计

### 角色划分

- **VLM/ASR/OCR**：负责从媒体提取可观察事实、文本、时间戳、对象候选及证据位置。
- **通用 LLM（可由 vLLM 托管）**：负责把自然语言目标、业务上下文和候选 facts 映射成 `state/questions` 草稿。
- **Schema validator**：确定性验证 JSON schema、字段类型、枚举、数值范围、question 类型与 criteria 形状。
- **Laya**：只做经过验证的问题的 typed decision；不承担开放式摘要、生成或多模态感知。
- **Policy engine**：在 Laya 结果之后做阈值、权限、审批和动作白名单。

### 输入流水线契约

```json
{
  "pipeline_version": "1",
  "source": {
    "kind": "image|audio|video|text|table|sensor",
    "captured_at": "RFC3339 timestamp",
    "sha256": "source content hash",
    "media_ref": "managed local object reference"
  },
  "facts": [
    {"name":"invoice_total","value":1200,"unit":"USD","evidence":"page 1, row 4","confidence":0.99}
  ],
  "state": {"body":"..."},
  "questions": {"risk":{"type":"choice","instructions":"...","criteria":{"low":"...","high":"..."}}},
  "missing_fields": [],
  "provenance": {"extractor":"provider/model@revision","prompt_version":"..."}
}
```

媒体内容保持在本地 managed object store，通过引用交给已授权的 connector。vLLM 是推理服务后端之一，不是本身的图片/音频输入协议；实际传图、传音频取决于部署的多模态模型和服务接口。适配器应声明接受的 MIME 类型、最大大小、时间戳与取消能力。不能把 OpenAI-compatible 文本接口误当成所有 VLM 的统一多模态接口。

### 可靠性门

1. 前处理 prompt 要求只抽取证据，缺失信息输出 null/unknown，禁止填补常识推断。
2. 输出经过 JSON schema 校验；choice criteria 必须非空；score criteria 是有序数组；noul criteria/labels 遵守 true/false 语义。
3. 保留原始输入哈希、抽取模型 revision、提示版本、结构化草稿、用户修改、Laya 请求/响应及策略决定。
4. 对不支持的输入 schema、超大媒体、超时、模型 unavailable、JSON 不合法分别返回结构化错误；不静默退回直连第三方。
5. 默认本地处理。使用远端 vLLM 时，在 UI 显示数据目的地、字段、传输内容、日志保留与用户授权。
6. 保持 raw mode：用户可以绕过 LLM/VLM 前处理，直接提交 Laya `state/questions`；便于审计和排查。

## 5. 客户端和服务端扩展方案

### 桌面客户端

- Models 负责已安装/未安装、下载进度、平台兼容性和实例生命周期。
- Playground 提供 Form 和 JSON 两种输入；Form 仅作为协议构造器，JSON mode 保留 `state` string/object/list 的完整表达能力。
- Pipeline 页面后续增加 connector、preprocessor provider/model、输入预览和输出 schema；未经用户预览/确认不得启动外部动作。
- Runs 保存可重放的请求快照和模型元数据；默认敏感 state 不持久化，用户显式选择本地保存时再配置加密和保留期。
- MCP 工具可把已配置的 pipeline 暴露给 agent，但与 daemon 的安装/启动权限分离。

### 服务端新增能力（后续独立阶段）

- `POST /v1/pipelines/preview`：对文本/受管媒体运行指定前处理器，返回事实、证据和 schema 诊断，不调用 Laya。
- `POST /v1/pipelines/{id}/run`：固定 pipeline revision 后先 preview/validate，再调用目标 capability，返回可追踪的分阶段任务。
- Pipeline manifest 声明输入 MIME、processor endpoint、模型 revision、输出 schema、timeout、权限和数据保留策略；不允许任意 shell command。
- vLLM connector 支持明确选定的 HTTP API 与多模态模型类型；本机 vLLM 与远端 OpenAI-compatible endpoint 分开标识。
- 媒体以有上限的 multipart 上传或短期本机引用传输；daemon 不接受任意绝对路径，不把媒体写进普通事件日志。

## 6. 评估与治理

- 为每个业务能力维护带 revision 的 gold set、边界案例、缺失字段和拒答样例。
- 分层测量 LLM/VLM 抽取准确率、事实支持率、schema 成功率、Laya 分类指标/校准、policy 拒绝率、端到端延迟和人工覆盖率。
- `noul` 使用 Brier/ECE 和业务代价选阈值；阈值按问题版本验证，不能把 confidence 当成全局概率保证。
- `choice`/`score` 监视选项顺序偏差、criteria 长度与多语言差异；对很多选项先做可审计的 shortlist，不超过已验证的选项规模。
- 模型、prompt、schema 或特征变化都触发重评估；用时间切分防金融前视偏差，用场景/传感器/机器人版本切分防具身数据泄漏。
- 高影响或物理执行场景记录人类审批人与原因，并支持 kill switch、回滚和模型版本冻结。

## 7. 分阶段交付

| 阶段 | 交付 | 验收门 |
| --- | --- | --- |
| 0. 客户端发行 | macOS、Windows、Linux 客户端包和 daemon 安装前置说明 | 目标 OS 构建、校验和、签名/未签名状态分别明确 |
| 1. Laya 原生工作台 | 当前 Models/Running/Playground、Form/JSON、多问题、结果与 raw JSON | 本地两种变体真实调用、问题结构测试、可重放 JSON |
| 2. Pipeline preview | schema registry、文本结构化器、review-before-run | 缺字段和 hallucinated fields 测试；不触发执行器 |
| 3. 多模态连接器 | 图片 OCR/VLM、音频 ASR、视频抽帧；保留媒体引用和证据 | MIME/size/timeout/cancel/PII 测试，抽取事实可核对 |
| 4. 金融研究 sandbox | 只读市场/公告 connector、时间戳 facts、离线评估 | walk-forward、无前视测试，只输出标签/告警 |
| 5. 具身仿真 shadow mode | ROS 2/模拟器 state connector、有限 action vocabulary | no-actuation shadow run、传感器过期 fail-closed、安全指标过门 |
| 6. 有限试点 | 人工审批、策略门、审计和回滚 | 业务/安全责任人签字；仍不允许语言模型绕过确定性安全控制 |

## 8. 明确不做

- 不把 Laya/Jev 包装成通用聊天模型。
- 不把图片、音频、视频直接宣称为当前 Laya checkpoint 原生支持的输入。
- 不自动让 LLM/VLM 的 JSON 触发交易、机器人动作或破坏性运维操作。
- 不将 Laya 概率解释为校准完成、零风险或监管批准。
- 不将 TypeSafe Jev 协议兼容描述成获得或部署了官方 Jev 模型权重。
- 不在第一阶段做开放的第三方插件市场或任意代码执行。

## 9. 当前待执行的下一步

1. 在桌面 Settings 加 daemon proxy UI，使用户能无代理直连或保存 HTTP/HTTPS/ALL proxy，而不是要求预先编辑 JSON。
2. 加 pipeline preview API 与 text-only LLM JSON normalizer；先用 mock/OpenAI-compatible interface，不把 vLLM 绑定到 Laya runtime。
3. 多模态方案先完成数据 schema、数据流和隐私评估，逐一选 OCR/ASR/VLM 组件后才编码接入。

## 10. 参考资料

- [Laya 上游仓库](https://github.com/NandhaKishorM/laya)：System-1 typed decisions、问题类型和安装/设备说明。本文不把上游自报基准当作本项目独立评测结果。
- [Laya Agent API 源码](https://github.com/NandhaKishorM/laya/blob/main/laya/agent.py)：`system_one` 的 `state` 及 `noul/choice/score` criteria 形状；接入时仍以本项目固定的包版本和真实契约测试为准。
- [TypeSafe `hs-jev` 客户端](https://github.com/getmissionctrl/hs-jev)：协议互操作参考；兼容 `/v1/systemone` 不意味着本地部署官方 Jev 权重。
- [vLLM 多模态输入文档](https://docs.vllm.ai/en/stable/features/multimodal_inputs/)：模型输入形态依赖具体模型/服务接口，不能把通用 OpenAI-compatible 文本请求等同为所有多模态请求。
- [vLLM 安全配置文档](https://docs.vllm.ai/en/stable/usage/security/)：远程多模态 URL/域名访问应采用 allowlist 和网络隔离。
- [Fyne cross 编译器](https://github.com/fyne-io/fyne-cross)：Windows/Linux 原生 GUI 交叉构建工具；公开包仍需各 OS runner 做启动验收。

## 11. 跨平台客户端交付状态

- macOS：Apple Silicon arm64 DMG/ZIP 已在本地构建，采用 ad-hoc 签名。当前构建机没有 Developer ID，因此不是通过 Gatekeeper 公证的公共发行版。
- Windows：x64 GUI EXE 已通过 Fyne cross 的 Windows 目标镜像构建，附 ZIP 与 README。未配置代码签名证书，Windows SmartScreen 可能发出提示。
- Linux：x86_64 Fyne desktop 包已通过 Linux 目标镜像构建，附用户级安装脚本、桌面入口和 README。
- 仓库 workflow 在 `codex/go-desktop-mvp` 分支 push、`desktop-v*` tag 或手动触发时构建 Windows/Linux 并上传 Actions artifacts。
- 这三个包均只包含桌面客户端。daemon、Python runtime 和模型权重需要另行安装。打包成功不代替在真实 Windows/Linux 桌面机器上的人工启动验收。
