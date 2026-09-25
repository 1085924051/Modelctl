"use strict";

const $ = (id) => document.getElementById(id);
const state = { models: [], details: new Map(), instances: [], tasks: [], variants: new Map(), profiles: new Map(), refreshing: false, acting: false };
const labels = { ready: "就绪", starting: "启动中", preparing: "准备中", stopping: "停止中", stopped: "已停止", failed: "失败", orphaned: "需清理", queued: "排队中", running: "进行中", succeeded: "已完成", cancelled: "已取消" };
let toastTimer;

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = String(text);
  return node;
}

function empty(node) { node.replaceChildren(); }
function pill(status) { return element("span", `status-pill ${status || ""}`, labels[status] || status || "未知"); }
function shortId(id) { return id ? `${id.slice(0, 13)}…` : "—"; }
function formatBytes(bytes) { return bytes >= 1e9 ? `${(bytes / 1e9).toFixed(2)} GB` : bytes >= 1e6 ? `${(bytes / 1e6).toFixed(0)} MB` : `${bytes || 0} B`; }
function percent(value) { return `${Math.round(Math.max(0, Math.min(1, Number(value) || 0)) * 100)}%`; }
function activeTask(modelId, variant) { return state.tasks.find((task) => task.model_id === modelId && task.variant === variant && ["queued", "running"].includes(task.status)); }

async function api(url, method = "GET", data) {
  const response = await fetch(url, { method, headers: data === undefined ? undefined : { "content-type": "application/json" }, body: data === undefined ? undefined : JSON.stringify(data), cache: "no-store" });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(body.error?.message || `${response.status} ${response.statusText}`);
    error.code = body.error?.code;
    error.details = body.error?.details;
    throw error;
  }
  return body;
}

async function refresh() {
  if (state.refreshing) return;
  state.refreshing = true;
  try {
    const [models, instances, tasks] = await Promise.all([api("/v1/models"), api("/v1/instances"), api("/v1/tasks")]);
    state.models = models.items || [];
    state.instances = instances.items || [];
    state.tasks = tasks.items || [];
    const details = await Promise.all(state.models.filter((model) => model.id).map(async (model) => [model.id, await api(`/v1/models/${encodeURIComponent(model.id)}`)]));
    state.details = new Map(details);
    $("service-dot").className = "status-dot online";
    $("service-label").textContent = "本地服务已连接";
    render();
  } catch (error) {
    $("service-dot").className = "status-dot offline";
    $("service-label").textContent = "本地服务不可用";
    if (!state.models.length) $("model-list").textContent = `无法读取模型：${error.message}`;
  } finally { state.refreshing = false; }
}

function render() {
  $("stat-models").textContent = state.models.filter((item) => item.id).length;
  $("stat-installed").textContent = state.models.reduce((sum, item) => sum + (item.installed_variants?.length || 0), 0);
  $("stat-running").textContent = state.instances.filter((item) => item.status === "ready").length;
  $("stat-pulls").textContent = state.tasks.filter((item) => ["queued", "running"].includes(item.status)).length;
  $("model-count").textContent = `${state.models.filter((item) => item.id).length} 个模型`;
  renderModels();
  renderInstances();
  renderTasks();
  renderInstanceSelect();
}

function renderModels() {
  const list = $("model-list"); empty(list);
  if (!state.models.length) { list.append(element("div", "empty-state", "目录中暂无模型")); return; }
  for (const model of state.models) {
    if (!model.id) { list.append(element("div", "empty-state", `目录错误：${model.error || "无法读取模型"}`)); continue; }
    const detail = state.details.get(model.id);
    if (!detail) continue;
    const variants = detail.variants || [];
    let variantId = state.variants.get(model.id);
    if (!variants.some((variant) => variant.id === variantId)) variantId = variants.find((variant) => variant.default)?.id || variants[0]?.id;
    state.variants.set(model.id, variantId);
    const selected = variants.find((variant) => variant.id === variantId);
    const running = state.instances.find((instance) => instance.model?.id === model.id && instance.model?.variant === variantId && instance.status === "ready");
    const task = activeTask(model.id, variantId);
    const card = element("article", "model-card");
    const top = element("div", "model-top"); top.append(element("div", "model-icon", model.display_name?.[0] || "M"), pill(running ? "ready" : task ? task.status : selected?.installed ? "succeeded" : "stopped")); card.append(top);
    card.append(element("h3", "", detail.display_name || model.id), element("div", "model-subtitle", `${model.id} · v${model.version}`), element("p", "model-desc", "Laya 结构化决策模型。支持是非概率、单选和等级评分。"));
    const tags = element("div", "model-meta"); tags.append(element("span", "mini-tag", model.license || "模型"), element("span", "mini-tag", `${variants.length} 个变体`), element("span", "mini-tag", "SYSTEM ONE")); card.append(tags);
    const fields = element("div", "card-fields");
    const variantLabel = element("label"); variantLabel.append(element("span", "", "模型变体"));
    const variantSelect = element("select"); variantSelect.setAttribute("aria-label", `${model.id} 模型变体`);
    for (const variant of variants) { const option = element("option", "", variant.id === "english" ? "English" : variant.id === "multilingual" ? "Multilingual" : variant.id); option.value = variant.id; variantSelect.append(option); }
    variantSelect.value = variantId; variantSelect.addEventListener("change", () => { state.variants.set(model.id, variantSelect.value); renderModels(); }); variantLabel.append(variantSelect); fields.append(variantLabel);
    const profileLabel = element("label"); profileLabel.append(element("span", "", "运行设备"));
    const profileSelect = element("select"); profileSelect.setAttribute("aria-label", `${model.id} 运行设备`);
    const supported = detail.preflight?.profiles?.filter((profile) => profile.supported) || [];
    for (const profile of supported) { const option = element("option", "", profile.id.toUpperCase()); option.value = profile.id; profileSelect.append(option); }
    if (!supported.length) { const option = element("option", "", "当前平台不支持"); option.value = ""; profileSelect.append(option); profileSelect.disabled = true; }
    const savedProfile = state.profiles.get(model.id);
    profileSelect.value = supported.some((profile) => profile.id === savedProfile) ? savedProfile : supported.find((profile) => profile.id === "auto")?.id || supported[0]?.id || "";
    profileSelect.addEventListener("change", () => state.profiles.set(model.id, profileSelect.value)); profileLabel.append(profileSelect); fields.append(profileLabel); card.append(fields);
    if (task) {
      const track = element("div", "progress-track"); const fill = element("span"); fill.style.width = task.progress?.bytes_total ? percent(task.progress.bytes_done / task.progress.bytes_total) : "2%"; track.append(fill); card.append(track);
    }
    const actions = element("div", "card-actions");
    const pull = element("button", "secondary-button", task ? "下载中…" : selected?.installed ? "重新校验" : "下载模型"); pull.type = "button"; pull.disabled = !!task || state.acting; pull.addEventListener("click", () => doPull(model.id, variantId)); actions.append(pull);
    const start = element("button", "primary-button", running ? "已在运行" : "启动实例 →"); start.type = "button"; start.disabled = !!running || !supported.length || state.acting; start.addEventListener("click", () => doStart(model.id, variantId, profileSelect.value)); actions.append(start); card.append(actions);
    const bytes = selected?.artifacts?.reduce((sum, item) => sum + item.size_bytes, 0) || 0;
    card.append(element("div", "card-note", selected?.installed ? `已下载 · ${formatBytes(bytes)} · 启动后可在判断工作台使用` : `需要下载约 ${formatBytes(bytes)} · 点击启动也会先下载`));
    list.append(card);
  }
}

function renderInstances() {
  const list = $("instance-list"); empty(list);
  const instances = [...state.instances].sort((a, b) => (b.started_at || "").localeCompare(a.started_at || "")).slice(0, 8);
  $("instance-count").textContent = `${state.instances.filter((item) => ["ready", "starting"].includes(item.status)).length} 个活动实例`;
  if (!instances.length) { list.append(element("div", "empty-state", "还没有实例。先下载并启动一个模型。")); return; }
  for (const instance of instances) {
    const row = element("article", "instance-card"); row.append(element("div", "instance-icon", "◈"));
    const main = element("div", "instance-main"); const title = element("div", "instance-title"); title.append(element("strong", "", `${instance.model?.variant || "模型"} · ${instance.model?.id || "未知模型"}`), pill(instance.status)); main.append(title);
    main.append(element("div", "instance-meta", `${shortId(instance.id)} · ${instance.device || instance.profile || "设备未知"} · ${instance.runtime || "运行时未知"}${instance.error ? ` · ${instance.error.message}` : ""}`)); row.append(main);
    if (["ready", "starting", "failed"].includes(instance.status)) { const stop = element("button", "danger-button", "停止"); stop.type = "button"; stop.disabled = state.acting; stop.addEventListener("click", () => doStop(instance.id)); row.append(stop); }
    list.append(row);
  }
}

function renderTasks() {
  const list = $("task-list"); empty(list);
  const tasks = [...state.tasks].sort((a, b) => (b.started_at || b.updated_at || "").localeCompare(a.started_at || a.updated_at || "")).slice(0, 6);
  if (!tasks.length) { list.append(element("div", "empty-state", "暂无下载任务")); return; }
  for (const task of tasks) {
    const row = element("article", "task-card"); const main = element("div", "task-main"); const title = element("div", "task-title"); title.append(element("span", "", `${task.model_id || "模型"} · ${task.variant || ""}`), pill(task.status)); main.append(title);
    main.append(element("div", "task-meta", task.error?.message || `${shortId(task.id)} · ${formatBytes(task.progress?.bytes_done)} / ${formatBytes(task.progress?.bytes_total)}`)); row.append(main);
    if (["queued", "running"].includes(task.status)) { const progress = element("div", "task-progress"); const value = task.progress?.bytes_total ? task.progress.bytes_done / task.progress.bytes_total : 0; const track = element("div", "progress-track"); const fill = element("span"); fill.style.width = percent(value); track.append(fill); progress.append(track, element("span", "", percent(value))); row.append(progress); const cancel = element("button", "secondary-button", "取消"); cancel.type = "button"; cancel.addEventListener("click", () => runAction(async () => { await api(`/v1/tasks/${encodeURIComponent(task.id)}/cancel`, "POST", {}); showToast("下载已取消"); })); row.append(cancel); }
    list.append(row);
  }
}

function renderInstanceSelect() {
  const select = $("instance-select"); const chosen = select.value; empty(select);
  const ready = state.instances.filter((instance) => instance.status === "ready" && instance.capabilities?.includes("system_one"));
  if (!ready.length) { const option = element("option", "", "请先启动 Laya 实例"); option.value = ""; select.append(option); select.disabled = true; return; }
  select.disabled = false;
  for (const instance of ready) { const option = element("option", "", `${instance.model.variant} · ${instance.device || instance.profile} · ${shortId(instance.id)}`); option.value = instance.id; select.append(option); }
  select.value = ready.some((instance) => instance.id === chosen) ? chosen : ready.find((instance) => instance.default)?.id || ready[0].id;
}

async function runAction(action) {
  if (state.acting) return;
  state.acting = true; render();
  try { await action(); } catch (error) { showToast(error.message, true); }
  finally { state.acting = false; await refresh(); render(); }
}

async function startPull(modelId, variantId) {
  try { return await api("/v1/pulls", "POST", { model_id: modelId, variant: variantId }); }
  catch (error) { if (error.code === "TASK_EXISTS" && error.details?.task_id) return { task_id: error.details.task_id }; throw error; }
}

async function waitForPull(taskId) {
  while (true) {
    const task = await api(`/v1/tasks/${encodeURIComponent(taskId)}`);
    await refresh();
    if (task.status === "succeeded") return;
    if (["failed", "cancelled"].includes(task.status)) throw new Error(task.error?.message || `下载${labels[task.status]}`);
    await new Promise((resolve) => setTimeout(resolve, 1200));
  }
}

function doPull(modelId, variantId) { return runAction(async () => { const task = await startPull(modelId, variantId); showToast("模型下载已开始，可在下方查看进度"); await refresh(); await waitForPull(task.task_id); showToast("模型下载完成"); }); }
function doStart(modelId, variantId, profile) { return runAction(async () => {
  const detail = state.details.get(modelId); const variant = detail?.variants?.find((item) => item.id === variantId);
  if (!variant?.installed) { const task = await startPull(modelId, variantId); showToast("正在下载模型，完成后自动启动"); await waitForPull(task.task_id); }
  showToast("正在加载模型，请稍候…");
  const instance = await api("/v1/instances", "POST", { model_id: modelId, variant: variantId, profile, default: true });
  showToast(`实例已启动：${shortId(instance.id)}`);
}); }
function doStop(instanceId) { return runAction(async () => { await api(`/v1/instances/${encodeURIComponent(instanceId)}`, "DELETE"); showToast("实例已停止"); }); }

function addQuestion() {
  const list = $("question-list"); const index = list.children.length + 1;
  const row = element("div", "question-row");
  const head = element("div", "question-row-head");
  const id = element("input"); id.className = "question-id"; id.placeholder = "问题 ID，例如 refund"; id.setAttribute("aria-label", "问题 ID"); id.value = index === 1 ? "refund" : `question_${index}`;
  const type = element("select"); type.className = "question-type"; type.setAttribute("aria-label", "问题类型");
  for (const [value, label] of [["noul", "是非判断"], ["choice", "单选"], ["score", "评分"]]) { const option = element("option", "", label); option.value = value; type.append(option); }
  const remove = element("button", "remove-question", "×"); remove.type = "button"; remove.title = "删除问题"; remove.setAttribute("aria-label", "删除问题"); remove.addEventListener("click", () => { if (list.children.length > 1) row.remove(); else showToast("至少保留一个问题", true); });
  head.append(id, type, remove); row.append(head);
  const instructions = element("input", "question-instructions"); instructions.placeholder = "例如：客户是否要求退款？"; instructions.setAttribute("aria-label", "判断问题"); row.append(instructions);
  const criteria = element("textarea", "criteria"); criteria.setAttribute("aria-label", "选项或等级"); row.append(criteria);
  type.addEventListener("change", () => updateCriteria());
  function updateCriteria() { criteria.hidden = type.value === "noul"; criteria.placeholder = type.value === "choice" ? "billing: 账单和退款\nsupport: 技术支持" : "完全不紧急\n有些紧急\n非常紧急"; if (type.value === "noul") criteria.value = ""; }
  updateCriteria(); list.append(row);
}

function readQuestions() {
  const questions = {};
  for (const row of $("question-list").children) {
    const id = row.querySelector(".question-id").value.trim();
    const type = row.querySelector(".question-type").value;
    const instructions = row.querySelector(".question-instructions").value.trim();
    const lines = row.querySelector(".criteria").value.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
    if (!id || !instructions) throw new Error("请填写每个问题的 ID 和判断问题");
    if (Object.hasOwn(questions, id)) throw new Error(`问题 ID 重复：${id}`);
    const question = { type, instructions };
    if (type === "choice") {
      if (!lines.length) throw new Error(`单选问题 ${id} 需要填写选项`);
      question.criteria = {};
      for (const line of lines) { const delimiter = line.search(/[:：]/); if (delimiter < 1) throw new Error(`选项格式应为“键: 描述”：${line}`); const key = line.slice(0, delimiter).trim(); const description = line.slice(delimiter + 1).trim(); if (!key || !description || Object.hasOwn(question.criteria, key)) throw new Error(`选项无效或重复：${line}`); question.criteria[key] = description; }
    }
    if (type === "score") { if (!lines.length) throw new Error(`评分问题 ${id} 需要至少一个等级`); question.criteria = lines; }
    questions[id] = question;
  }
  return questions;
}

async function submitDecision(event) {
  event.preventDefault();
  const instanceId = $("instance-select").value;
  const content = $("state-input").value.trim();
  if (!instanceId) { showToast("请先启动一个 Laya 实例", true); return; }
  if (!content) { showToast("请填写待判断文本", true); return; }
  let questions;
  try { questions = readQuestions(); } catch (error) { showToast(error.message, true); return; }
  const button = $("submit-decision"); button.disabled = true; button.textContent = "正在判断…"; $("result-caption").textContent = "模型正在处理";
  try { const result = await api(`/v1/instances/${encodeURIComponent(instanceId)}/operations/system_one`, "POST", { state: content, questions }); renderResult(result); }
  catch (error) { showToast(error.message, true); $("result-caption").textContent = "调用失败"; }
  finally { button.disabled = false; button.textContent = "运行判断 →"; }
}

function renderResult(result) {
  const content = $("result-content"); empty(content);
  const answers = result.answers || {}; const items = element("div", "result-items");
  for (const [questionId, answer] of Object.entries(answers)) {
    const card = element("article", "answer-card"); const head = element("div", "answer-head"); head.append(element("strong", "", questionId), element("span", "mini-tag", answer.type || "判断")); card.append(head);
    if (answer.type === "choice") card.append(element("div", "answer-value", answer.choice || "—"));
    else if (answer.type === "score") card.append(element("div", "answer-value", answer.score ?? "—"));
    else if (answer.type === "noul") card.append(element("div", "answer-value", `${percent(answer.noul)} 是`));
    else card.append(element("div", "answer-value", "已返回结果"));
    if (answer.confidence !== undefined) card.append(element("div", "answer-note", `置信度 ${percent(answer.confidence)}`));
    if (answer.type === "noul") addProbability(card, "是", answer.noul);
    if (answer.probabilities && typeof answer.probabilities === "object") for (const [name, value] of Object.entries(answer.probabilities).sort((a, b) => b[1] - a[1])) addProbability(card, name, value);
    items.append(card);
  }
  if (!items.children.length) items.append(element("div", "empty-state", "模型未返回判断项，请查看原始响应"));
  content.append(items);
  const meta = element("div", "result-meta"); if (result.routing?.model) meta.append(element("span", "", `模型 ${result.routing.model}`)); if (result.usage?.input_tokens !== undefined) meta.append(element("span", "", `输入 ${result.usage.input_tokens} tokens`)); content.append(meta);
  const raw = element("details", "raw-result"); raw.append(element("summary", "", "查看原始 JSON"), element("pre", "", JSON.stringify(result, null, 2))); content.append(raw);
  $("result-caption").textContent = `${Object.keys(answers).length} 项判断已完成`;
}

function addProbability(card, name, value) {
  let group = card.querySelector(".answer-probabilities"); if (!group) { group = element("div", "answer-probabilities"); card.append(group); }
  const row = element("div", "probability-row"); const track = element("div", "progress-track"); const fill = element("span"); fill.style.width = percent(value); track.append(fill); row.append(element("span", "", name), track, element("span", "", percent(value))); group.append(row);
}

function showToast(message, error = false) {
  const toast = $("toast"); toast.textContent = message; toast.className = `toast show${error ? " error" : ""}`;
  clearTimeout(toastTimer); toastTimer = setTimeout(() => { toast.className = "toast"; }, error ? 6000 : 3500);
}

$("service-address").textContent = location.host;
$("refresh-button").addEventListener("click", refresh);
$("add-question").addEventListener("click", addQuestion);
$("decision-form").addEventListener("submit", submitDecision);
addQuestion();
refresh();
setInterval(refresh, 2500);
