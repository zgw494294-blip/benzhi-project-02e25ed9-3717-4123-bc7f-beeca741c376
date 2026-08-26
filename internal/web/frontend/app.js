"use strict";

const state = { cases: [], current: null, activeTab: "segments", verification: null, risk: null, evidence: null, evidenceError: null };
const statusNames = {
  draft: "草拟", pending_check: "待校核", remediation: "整改中", pending_review: "待复核",
  returned: "已退回", pending_approval: "待批准", sealed: "已封存"
};
const severityNames = { block: "阻断", warning: "警告", pass: "通过" };
const actionNames = { keep: "保留", replace: "替换", redact: "遮蔽" };

const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => Array.from(document.querySelectorAll(selector));
const escapeHTML = (value) => String(value ?? "").replace(/[&<>'"]/g, (char) => ({"&":"&amp;","<":"&lt;",">":"&gt;","'":"&#39;",'"':"&quot;"})[char]);
const idem = () => crypto.randomUUID();
const formatTime = (value) => value ? new Date(value).toLocaleString("zh-CN", { hour12: false }) : "—";
const formatMillis = (value) => {
  const minutes = Math.floor(value / 60000);
  const seconds = Math.floor((value % 60000) / 1000);
  const millis = value % 1000;
  return `${String(minutes).padStart(2,"0")}:${String(seconds).padStart(2,"0")}.${String(millis).padStart(3,"0")}`;
};

async function api(path, options = {}) {
  const init = { method: options.method || "GET", headers: { Accept: "application/json" } };
  if (options.body !== undefined) {
    init.headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(options.body);
  }
  const response = await fetch(path, init);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(payload.error?.message || `请求失败 (${response.status})`);
    error.fields = payload.error?.fields || {};
    error.details = payload.error?.details;
    throw error;
  }
  return payload;
}

function notify(message) {
  const toast = $("#toast");
  toast.textContent = message;
  toast.classList.remove("hidden");
  window.setTimeout(() => toast.classList.add("hidden"), 3200);
}

async function loadCases(selectID) {
  const payload = await api("/api/v1/cases");
  state.cases = payload.cases || [];
  renderCaseList();
  if (selectID) await loadCase(selectID);
  else if (state.current) await loadCase(state.current.id);
  else if (state.cases.length) await loadCase(state.cases[0].id);
  else renderEmpty();
}

async function loadCase(id) {
  const payload = await api(`/api/v1/cases/${encodeURIComponent(id)}`);
  state.current = payload.case;
  state.verification = null;
  state.risk = await api(`/api/v1/cases/${encodeURIComponent(id)}/risk-summary`);
  state.evidence = null;
  state.evidenceError = null;
  if (state.current.status === "sealed") {
    try { state.evidence = await api(`/api/v1/cases/${encodeURIComponent(id)}/package`); }
    catch (error) { state.evidenceError = error; }
  }
  renderCaseList();
  renderCase();
}

function renderEmpty() {
  state.current = null;
  $("#empty-state").classList.remove("hidden");
  $("#case-view").classList.add("hidden");
  $("#action-rail").classList.add("hidden");
}

function renderCaseList() {
  $("#case-count").textContent = state.cases.length;
  $("#case-list").innerHTML = state.cases.map((item) => `
    <button class="case-item ${state.current?.id === item.id ? "active" : ""}" data-case-id="${escapeHTML(item.id)}">
      <strong>${escapeHTML(item.title)}</strong>
      <span class="case-meta"><span>${escapeHTML(statusNames[item.status] || item.status)}</span><span>v${item.version}</span></span>
    </button>`).join("");
  $$("[data-case-id]").forEach((button) => button.addEventListener("click", () => loadCase(button.dataset.caseId).catch(showError)));
}

function renderCase() {
  const c = state.current;
  if (!c) return renderEmpty();
  $("#empty-state").classList.add("hidden");
  $("#case-view").classList.remove("hidden");
  $("#action-rail").classList.remove("hidden");
  $("#case-kicker").textContent = `${c.id} · 访谈 ${c.interviewDate}`;
  $("#case-title").textContent = c.title;
  $("#case-status").textContent = statusNames[c.status] || c.status;
  $("#case-version").textContent = `FIXED SNAPSHOT · v${c.version}`;
  renderBoundary(c);
  renderSegments(c);
  renderFindings(c);
  renderPreview(c);
  renderReview(c);
  renderEvidence(c);
  renderActions(c);
  activateTab(state.activeTab);
}

function renderBoundary(c) {
  $("#boundary-card").innerHTML = `
    <div><label>标题与访谈日期</label><p>${escapeHTML(c.title)} · ${escapeHTML(c.interviewDate)}</p></div>
    <div><label>拟开放用途</label><p>${escapeHTML(c.intendedUse)}</p></div>
    <div><label>参与者授权范围</label><p>${escapeHTML((c.consentScope || []).join(" · "))}</p></div>
    <div><label>限制条款</label><p>${escapeHTML((c.restrictionTerms || []).join(" · ") || "无额外限制")}</p></div>
    ${c.status === "draft" ? `<div class="boundary-action"><button id="edit-boundary-button" class="button secondary">修订边界</button></div>` : ""}`;
  $("#edit-boundary-button")?.addEventListener("click", openBoundaryDialog);
}

function renderSegments(c) {
  $("#add-segment-button").disabled = c.status !== "draft";
  $("#segment-list").innerHTML = (c.segments || []).map((segment) => `
    <article class="segment-card">
      <div><div class="segment-sequence">#${String(segment.sequence).padStart(2,"0")}</div><div class="segment-time">${formatMillis(segment.startMillis)}<br>${formatMillis(segment.endMillis)}</div></div>
      <div><span class="speaker">${escapeHTML(segment.speakerLabel)}</span><p class="segment-text">${escapeHTML(segment.originalText)}</p><div>${(segment.sensitivityTags || []).map(tag => `<span class="tag">${escapeHTML(tag)}</span>`).join("")}${segment.riskNote ? `<span class="tag">风险：${escapeHTML(segment.riskNote)}</span>` : ""}</div></div>
      <div>${segment.needsRecheck ? `<div class="recheck">需定向校核</div><button class="button secondary revise-segment" data-segment-id="${escapeHTML(segment.id)}">整改</button>` : `<span class="quiet">r${segment.revision}</span>`}</div>
    </article>`).join("") || `<p class="hint">尚未登记片段。片段顺序必须从 1 连续排列，时间范围不得重叠。</p>`;
  $$(".revise-segment").forEach(button => button.addEventListener("click", () => openSegmentDialog(c.segments.find(s => s.id === button.dataset.segmentId))));
}

function renderFindings(c) {
  const visibleFindingIDs = state.risk ? new Set((state.risk.findings || []).map(item => item.findingId)) : null;
  const active = (c.findings || []).filter(item => item.resolutionStatus !== "obsolete" && (!visibleFindingIDs || visibleFindingIDs.has(item.id)));
  const counts = active.reduce((sum, item) => (sum[item.severity]++, sum), { block: 0, warning: 0, pass: 0 });
  $("#finding-summary").textContent = `${counts.block} 阻断 · ${counts.warning} 警告 · ${counts.pass} 通过`;
  renderRiskPanel(c);
  $("#finding-list").innerHTML = active.map((finding) => {
    const decision = (c.decisions || []).find(item => item.findingId === finding.id);
    const canDecide = finding.severity === "block" && ["remediation", "returned"].includes(c.status);
    return `<article id="finding-${escapeHTML(finding.id)}" class="finding-card ${finding.severity}">
      <div class="finding-top"><span class="finding-code">${escapeHTML(finding.ruleCode)}</span><span class="severity ${finding.severity}">${severityNames[finding.severity]} · ${escapeHTML(finding.resolutionStatus)}</span></div>
      <p>${escapeHTML(finding.explanation)}</p>
      ${decision ? `<div class="decision">${actionNames[decision.action]} · ${escapeHTML(decision.rationale)}${decision.replacementText ? ` · “${escapeHTML(decision.replacementText)}”` : ""}</div>` : ""}
      ${canDecide ? `<div class="finding-actions"><label><input type="checkbox" class="decision-select" data-finding-id="${escapeHTML(finding.id)}"> 纳入批量</label><button class="button secondary decide-button" data-finding-id="${escapeHTML(finding.id)}">${decision ? "修改处置" : "闭合阻断"}</button></div>` : ""}
    </article>`;
  }).join("") || `<p class="hint">冻结案件后运行确定性校核，这里将显示逐项依据。</p>`;
  $$(".decide-button").forEach(button => button.addEventListener("click", () => openDecisionDialog(button.dataset.findingId)));
  $$(".decision-select").forEach(input => input.addEventListener("change", updateBatchDecisionButton));
  updateBatchDecisionButton();
}

function renderRiskPanel(c) {
  const risk = state.risk;
  if (!risk) { $("#risk-panel").innerHTML = ""; return; }
  const differences = (risk.differences || []).filter(item => item.changed);
  $("#risk-panel").innerHTML = `<div class="risk-counts">
      <span><strong>${risk.counts.block}</strong> 阻断</span><span><strong>${risk.counts.warning}</strong> 警告</span><span><strong>${risk.counts.pass}</strong> 通过</span><span><strong>${risk.counts.openBlocks}</strong> 未闭合</span>
    </div><div class="risk-filters">
      <select id="risk-severity"><option value="">全部严重度</option><option value="block">阻断</option><option value="warning">警告</option><option value="pass">通过</option></select>
      <input id="risk-rule" placeholder="ruleCode"><input id="risk-segment" placeholder="片段 ID">
      <select id="risk-changed"><option value="">全部差异</option><option value="true">仅变化</option><option value="false">未变化</option></select>
      <button id="risk-filter-button" class="button ghost">筛选</button>
    </div>
    <div class="risk-groups"><span>规则：${(risk.byRuleCode || []).map(item => `${escapeHTML(item.key)}(${item.count})`).join(" · ") || "无"}</span><span>敏感类别：${(risk.bySensitivity || []).map(item => `${escapeHTML(item.key)}(${item.count})`).join(" · ") || "无"}</span></div>
    <div class="risk-differences">${differences.map(item => `<a href="#finding-${escapeHTML(item.current?.findingId || item.previous?.findingId || "")}">${escapeHTML(item.segmentId)} · ${escapeHTML(item.ruleCode)} · ${escapeHTML(changeName(item.status))}</a>`).join("") || `<span>本次运行无结论变化；未重跑片段均标记为未变化。</span>`}</div>`;
  $("#risk-filter-button").addEventListener("click", () => filterRisk(c.id).catch(showError));
}

function changeName(value) { return ({added:"新增", removed:"已消除", severity_changed:"严重度变化", evidence_changed:"依据变化", unchanged:"未变化"})[value] || value; }

async function filterRisk(caseID) {
  const params = new URLSearchParams();
  const values = { severity: $("#risk-severity").value, ruleCode: $("#risk-rule").value.trim(), segmentId: $("#risk-segment").value.trim(), changed: $("#risk-changed").value };
  Object.entries(values).forEach(([key, value]) => { if (value) params.set(key, value); });
  state.risk = await api(`/api/v1/cases/${encodeURIComponent(caseID)}/risk-summary?${params}`);
  renderFindings(state.current);
}

function renderPreview(c) {
  const preview = c.preview;
  $("#preview-list").innerHTML = preview ? preview.segments.map(segment => `
    <article class="preview-card"><div class="preview-side"><label>BEFORE · ${escapeHTML(segment.segmentId)}</label><p>${escapeHTML(segment.before)}</p></div><div class="preview-side after"><label>AFTER · BOUNDED</label><p>${escapeHTML(segment.after)}</p></div></article>`).join("") : `<p class="hint">闭合全部阻断项后生成预览。</p>`;
  $("#normalized-text").textContent = preview?.text || "尚无规范化发布文本";
}

function renderReview(c) {
  const active = (c.findings || []).filter(item => item.resolutionStatus !== "obsolete" && item.severity !== "pass");
  const submitted = c.timeline?.at(-1)?.type === "review.submitted";
  if (c.status === "pending_review" && submitted) {
    $("#review-panel").innerHTML = `<p class="hint">复核员须逐项确认有效警告与处置，批准后固定送审版本；退回时必须指定理由代码和受影响片段。</p><div class="review-checks">${active.map(item => `<label class="review-check"><input type="checkbox" class="review-item" value="${escapeHTML(item.id)}"><span><strong>${escapeHTML(item.ruleCode)}</strong><br>${escapeHTML(item.explanation)}</span></label>`).join("") || `<p>没有需要逐项确认的风险项。</p>`}</div><div class="review-actions"><button id="review-approve" class="button primary">复核批准</button><button id="review-return" class="button danger">结构化退回</button></div>`;
    $("#review-approve").addEventListener("click", approveReview);
    $("#review-return").addEventListener("click", openReturnDialog);
  } else {
    $("#review-panel").innerHTML = `<p class="hint">${c.status === "pending_review" ? "先从右侧控制点提交独立复核。" : "当前状态尚不可作出复核决定。"}</p>`;
  }
  $("#review-trail").innerHTML = (c.reviews || []).map(record => `<div class="review-record"><strong>${record.outcome === "approved" ? "复核通过" : "退回整改"}</strong> · ${escapeHTML(record.reviewer)} · v${record.caseVersion}<br><span class="quiet">${escapeHTML(record.reasonCode || "逐项确认")} ${escapeHTML(record.reason || "")}</span></div>`).join("");
}

function renderEvidence(c) {
  if (state.evidence) {
    const evidence = state.evidence;
    $("#package-panel").innerHTML = `<article class="package-card"><p class="eyebrow">SEALED EVIDENCE CATALOG</p><h3>${escapeHTML(evidence.packageId)}</h3><p>${escapeHTML(evidence.versionSummary)}</p><p class="quiet">批准人 ${escapeHTML(evidence.approval.approvedBy)} · ${formatTime(evidence.approval.sealedAt)}</p>
      <div class="evidence-counts"><span>发布文本 ${evidence.counts.normalizedTexts}</span><span>决定 ${evidence.counts.decisions}</span><span>复核 ${evidence.counts.reviews}</span><span>版本摘要 ${evidence.counts.versionSummaries}</span><span>批准 ${evidence.counts.approvals}</span><span>校验 ${evidence.counts.checksums}</span></div>
      <div class="evidence-filters"><input id="evidence-rule" placeholder="决定 ruleCode"><select id="evidence-outcome"><option value="">全部复核结论</option><option value="approved">approved</option><option value="returned">returned</option></select><button id="evidence-filter-button" class="button ghost">目录定位</button></div>
      <details><summary>规范化发布文本</summary><pre class="release-text">${escapeHTML(evidence.normalizedText)}</pre></details>
      <details><summary>决定快照（${evidence.decisions.length}）</summary>${evidence.decisions.map(item => `<p class="evidence-row"><strong>${escapeHTML(item.ruleCode)}</strong> · ${escapeHTML(item.action)} · ${escapeHTML(item.findingId)}<br>${escapeHTML(item.rationale)}</p>`).join("") || "<p>无匹配决定</p>"}</details>
      <details><summary>复核快照（${evidence.reviews.length}）</summary>${evidence.reviews.map(item => `<p class="evidence-row"><strong>${escapeHTML(item.outcome)}</strong> · ${escapeHTML(item.reviewer)} · v${item.caseVersion}</p>`).join("") || "<p>无匹配复核</p>"}</details>
      <p class="digest">${escapeHTML(evidence.checksum.storedDigest)}</p>
      <div class="verification">✓ 查看前重算一致<br><span class="digest">${escapeHTML(evidence.checksum.calculatedDigest)}</span></div>
      <div class="evidence-actions"><button id="verify-package" class="button secondary">再次重算摘要</button><a class="button primary" href="/api/v1/cases/${encodeURIComponent(c.id)}/package/download" download>下载规范化 JSON</a></div>
      ${state.verification ? `<div class="verification">${state.verification.valid ? "✓ 内容未变，摘要一致" : "✕ 摘要不一致"}</div>` : ""}</article>`;
    $("#verify-package").addEventListener("click", verifyPackage);
    $("#evidence-filter-button").addEventListener("click", () => filterEvidence(c.id).catch(showError));
  } else if (state.evidenceError) {
    $("#package-panel").innerHTML = `<div class="integrity-error"><strong>完整性校验失败</strong><p>${escapeHTML(state.evidenceError.message)}</p><p>为保护封存证据，下载已禁止。</p></div>`;
  } else {
    $("#package-panel").innerHTML = `<p class="hint">开放负责人最终批准后，这里将显示不可变发布文本、决定快照、版本摘要和 SHA-256 校验摘要。</p>`;
  }
  $("#timeline").innerHTML = (c.timeline || []).slice().reverse().map(event => `<li><span class="timeline-index">${String(event.sequence).padStart(2,"0")}</span><div><strong>${escapeHTML(event.type)}</strong> · ${escapeHTML(event.actor)} · v${event.caseVersion}<br><span>${escapeHTML(event.reason || "状态留痕")}</span><time>${formatTime(event.at)}</time></div></li>`).join("");
}

async function filterEvidence(caseID) {
  const params = new URLSearchParams();
  const ruleCode = $("#evidence-rule").value.trim();
  const outcome = $("#evidence-outcome").value;
  if (ruleCode) params.set("ruleCode", ruleCode);
  if (outcome) params.set("outcome", outcome);
  state.evidence = await api(`/api/v1/cases/${encodeURIComponent(caseID)}/package?${params}`);
  renderEvidence(state.current);
}

function renderActions(c) {
  const controls = [];
  let title = "等待输入", description = "根据当前案件状态完成下一控制点。";
  const add = (label, action, style = "primary") => controls.push({ label, action, style });
  if (c.status === "draft") {
    title = c.segments.length ? "冻结案件版本" : "登记转录片段";
    description = c.segments.length ? "冻结后授权边界与片段成为待校核固定版本。" : "先登记至少一个顺序连续的转录片段。";
    if (c.segments.length) add("冻结并进入待校核", () => metaCommand("freeze", "口述史整理员"));
    add("批量登记片段", openBatchSegmentsDialog, "secondary");
  } else if (c.status === "pending_check") {
    title = "执行确定性校核"; description = "按授权范围、限制条款、用途和敏感标注生成稳定结论。";
    add("运行全量校核", () => metaCommand("checks", "口述史整理员"));
  } else if (c.status === "remediation") {
    const open = c.findings.filter(f => f.severity === "block" && f.resolutionStatus === "open").length;
    title = open ? `闭合 ${open} 个阻断项` : "固定遮蔽预览";
    description = open ? "逐项记录保留、替换或遮蔽决定及理由。" : "全部阻断项已闭合，可以生成带片段边界的发布预览。";
    if (!open) add("生成发布预览", () => metaCommand("preview", "口述史整理员"));
  } else if (c.status === "returned") {
    title = "定向整改与重校核"; description = "只能修改复核理由关联的片段；重跑时旧修订结论会标记为过期。";
    add("运行定向重校核", () => metaCommand("rechecks", "口述史整理员"));
  } else if (c.status === "pending_review") {
    if (!c.preview) {
      title = "生成送审预览"; description = "校核未产生阻断项，但仍需固定发布预览。";
      add("生成发布预览", () => metaCommand("preview", "口述史整理员"));
    } else if (c.timeline?.at(-1)?.type !== "review.submitted") {
      title = "提交独立复核"; description = "确认发布预览版本后交由隐私复核员逐项判断。";
      add("提交独立复核", () => metaCommand("review/submit", "口述史整理员"));
    } else {
      title = "等待复核决定"; description = "请在“复核决定”页逐项确认后批准或结构化退回。";
      add("打开复核页", () => activateTab("review"), "secondary");
    }
  } else if (c.status === "pending_approval") {
    title = "最终批准与封存"; description = "开放负责人批准当前固定版本，生成不可变发布包和摘要。";
    add("最终批准并封存", openApprovalDialog);
  } else if (c.status === "sealed") {
    title = "发布包已封存"; description = "发布文本与决定证据已固定，可随时重算摘要验证完整性。";
    add("查看并校验凭据", () => activateTab("evidence"), "secondary");
  }
  $("#next-title").textContent = title;
  $("#next-description").textContent = description;
  const host = $("#action-buttons");
  host.innerHTML = controls.map((control, index) => `<button class="button ${control.style}" data-action-index="${index}">${escapeHTML(control.label)}</button>`).join("");
  controls.forEach((control, index) => host.querySelector(`[data-action-index="${index}"]`).addEventListener("click", () => Promise.resolve(control.action()).catch(showError)));
}

function field(name, label, type = "text", value = "", wide = false, options = []) {
  if (type === "textarea") return `<div class="field ${wide ? "wide" : ""}"><label for="f-${name}">${label}</label><textarea id="f-${name}" name="${name}">${escapeHTML(value)}</textarea></div>`;
  if (type === "select") return `<div class="field ${wide ? "wide" : ""}"><label for="f-${name}">${label}</label><select id="f-${name}" name="${name}">${options.map(item => `<option value="${escapeHTML(item.value)}">${escapeHTML(item.label)}</option>`).join("")}</select></div>`;
  return `<div class="field ${wide ? "wide" : ""}"><label for="f-${name}">${label}</label><input id="f-${name}" name="${name}" type="${type}" value="${escapeHTML(value)}"></div>`;
}

let modalSubmit = null;
function openDialog(kicker, title, fields, submitLabel, onSubmit) {
  $("#dialog-kicker").textContent = kicker;
  $("#dialog-title").textContent = title;
  $("#dialog-fields").innerHTML = fields;
  $("#dialog-submit").textContent = submitLabel;
  $("#dialog-error").classList.add("hidden");
  modalSubmit = onSubmit;
  $("#form-dialog").showModal();
}

function closeDialog() { $("#form-dialog").close(); modalSubmit = null; }
function formValues() { return Object.fromEntries(new FormData($("#modal-form")).entries()); }

function openCaseDialog() {
  openDialog("01 · CASE BOUNDARY", "创建开放案", [
    field("title", "案件标题", "text", "", true), field("interviewDate", "访谈日期", "date"),
    field("actor", "整理员", "text", "口述史整理员"), field("intendedUse", "拟开放用途", "textarea", "公开研究与教育展览", true),
    field("consentScope", "授权范围（每行一项）", "textarea", "公开研究利用", true), field("restrictionTerms", "限制条款（每行一项）", "textarea", "", true)
  ].join(""), "创建草拟版本", async (values) => {
    const payload = await api("/api/v1/cases", { method: "POST", body: {
      title: values.title, interviewDate: values.interviewDate, intendedUse: values.intendedUse,
      consentScope: lines(values.consentScope), restrictionTerms: lines(values.restrictionTerms), actor: values.actor, idempotencyKey: idem()
    }});
    closeDialog(); notify("开放案已创建"); await loadCases(payload.case.id);
  });
}

function openBoundaryDialog() {
  const c = state.current;
  openDialog("01 · CASE BOUNDARY REVISION", "修订草拟案件边界", [
    field("title", "案件标题", "text", c.title, true), field("interviewDate", "访谈日期", "date", c.interviewDate),
    field("actor", "整理员", "text", "口述史整理员"), field("intendedUse", "拟开放用途", "textarea", c.intendedUse, true),
    field("consentScope", "授权范围（每行一项）", "textarea", (c.consentScope || []).join("\n"), true),
    field("restrictionTerms", "限制条款（每行一项）", "textarea", (c.restrictionTerms || []).join("\n"), true)
  ].join(""), "保存修订版本", async (values) => {
    await api(`/api/v1/cases/${c.id}/boundary`, { method: "PUT", body: {
      expectedVersion: c.version, idempotencyKey: idem(), actor: values.actor, title: values.title,
      interviewDate: values.interviewDate, intendedUse: values.intendedUse,
      consentScope: lines(values.consentScope), restrictionTerms: lines(values.restrictionTerms)
    }});
    closeDialog(); notify("案件边界已修订并写入字段摘要"); await loadCases(c.id);
  });
}

function batchSegmentRow(index, defaults = {}) {
  return `<div class="batch-row" data-row="${index}">
    <div class="batch-row-head"><strong>第 ${index + 1} 行</strong><button type="button" class="icon-button remove-batch-row" aria-label="删除行">×</button></div>
    <div class="batch-grid">
      <label>顺序<input data-key="sequence" type="number" value="${defaults.sequence ?? index + 1}"></label>
      <label>人物身份<input data-key="speakerLabel" value="${escapeHTML(defaults.speakerLabel || "受访者")}"></label>
      <label>开始毫秒<input data-key="startMillis" type="number" value="${defaults.startMillis ?? index * 10000}"></label>
      <label>结束毫秒<input data-key="endMillis" type="number" value="${defaults.endMillis ?? (index + 1) * 10000}"></label>
      <label class="wide">转录文本<textarea data-key="originalText">${escapeHTML(defaults.originalText || "")}</textarea></label>
      <label>敏感类别（逗号分隔）<input data-key="sensitivityTags" value="${escapeHTML((defaults.sensitivityTags || []).join(","))}"></label>
      <label>风险说明<input data-key="riskNote" value="${escapeHTML(defaults.riskNote || "")}"></label>
    </div><p class="batch-parsed"></p></div>`;
}

function openBatchSegmentsDialog() {
  const c = state.current;
  const next = c.segments.length + 1;
  openDialog("02 · TRANSCRIPT BATCH", "批量登记转录片段", `<div class="field wide"><label for="f-actor">操作者</label><input id="f-actor" name="actor" value="口述史整理员"></div><div id="batch-segment-rows" class="wide"></div><button type="button" id="add-batch-row" class="button ghost wide">＋ 增加一行</button>`, "原子登记整批片段", async (values) => {
    const segments = $$(".batch-row").map(row => ({
      sequence: Number(row.querySelector('[data-key="sequence"]').value),
      speakerLabel: row.querySelector('[data-key="speakerLabel"]').value,
      startMillis: Number(row.querySelector('[data-key="startMillis"]').value),
      endMillis: Number(row.querySelector('[data-key="endMillis"]').value),
      originalText: row.querySelector('[data-key="originalText"]').value,
      sensitivityTags: row.querySelector('[data-key="sensitivityTags"]').value.split(/[,，]/).map(item => item.trim()).filter(Boolean),
      riskNote: row.querySelector('[data-key="riskNote"]').value
    }));
    await api(`/api/v1/cases/${c.id}/segments/batch`, { method: "POST", body: { expectedVersion: c.version, idempotencyKey: idem(), actor: values.actor, segments }});
    closeDialog(); notify(`已原子登记 ${segments.length} 个片段`); await loadCases(c.id);
  });
  const rows = $("#batch-segment-rows");
  let count = 0;
  const addRow = () => {
    const index = count++;
    rows.insertAdjacentHTML("beforeend", batchSegmentRow(index, { sequence: next + index, startMillis: (next + index - 1) * 10000, endMillis: (next + index) * 10000 }));
    bindBatchRows(); updateBatchSegmentPreview();
  };
  $("#add-batch-row").addEventListener("click", addRow);
  addRow(); addRow(); addRow();
}

function bindBatchRows() {
  $$(".remove-batch-row").forEach(button => button.onclick = () => { button.closest(".batch-row").remove(); updateBatchSegmentPreview(); });
  $$(".batch-row input, .batch-row textarea").forEach(input => input.oninput = updateBatchSegmentPreview);
}

function updateBatchSegmentPreview() {
  $$(".batch-row").forEach((row, index) => {
    const sequence = Number(row.querySelector('[data-key="sequence"]').value);
    const start = Number(row.querySelector('[data-key="startMillis"]').value);
    const end = Number(row.querySelector('[data-key="endMillis"]').value);
    const speaker = row.querySelector('[data-key="speakerLabel"]').value.trim() || "未填写";
    const tags = row.querySelector('[data-key="sensitivityTags"]').value.trim() || "无敏感类别";
    row.querySelector(".batch-row-head strong").textContent = `第 ${index + 1} 行`;
    row.querySelector(".batch-parsed").textContent = `解析：#${sequence} · ${formatMillis(start)}—${formatMillis(end)} · ${speaker} · ${tags}`;
  });
}

function openSegmentDialog(segment) {
  const c = state.current;
  const revise = Boolean(segment);
  openDialog(revise ? "RETURNED SCOPE" : "02 · TRANSCRIPT", revise ? "定向整改片段" : "登记转录片段", [
    field("sequence", "稳定顺序", "number", segment?.sequence || c.segments.length + 1), field("speakerLabel", "人物身份", "text", segment?.speakerLabel || "受访者"),
    field("startMillis", "开始毫秒", "number", segment?.startMillis ?? 0), field("endMillis", "结束毫秒", "number", segment?.endMillis ?? 10000),
    field("originalText", "转录原文", "textarea", segment?.originalText || "", true), field("sensitivityTags", "敏感类别（每行一项）", "textarea", (segment?.sensitivityTags || []).join("\n"), true),
    field("riskNote", "风险说明", "textarea", segment?.riskNote || "", true), field("actor", "操作者", "text", "口述史整理员")
  ].join(""), revise ? "保存定向整改" : "登记片段", async (values) => {
    const body = { expectedVersion: c.version, idempotencyKey: idem(), actor: values.actor, sequence: Number(values.sequence), startMillis: Number(values.startMillis), endMillis: Number(values.endMillis), originalText: values.originalText, speakerLabel: values.speakerLabel, sensitivityTags: lines(values.sensitivityTags), riskNote: values.riskNote };
    const path = revise ? `/api/v1/cases/${c.id}/segments/${segment.id}` : `/api/v1/cases/${c.id}/segments`;
    await api(path, { method: revise ? "PUT" : "POST", body }); closeDialog(); notify(revise ? "受影响片段已整改" : "片段已登记"); await loadCases(c.id);
  });
}

function openDecisionDialog(findingID) {
  const c = state.current;
  const existing = (c.decisions || []).find(item => item.findingId === findingID);
  openDialog("03 · BLOCK RESOLUTION", "记录阻断处置", [
    field("action", "处置方式", "select", existing?.action, false, [{value:"redact",label:"遮蔽整个片段"},{value:"replace",label:"使用替代文本"},{value:"keep",label:"有理由保留"}]),
    field("actor", "处置人", "text", existing?.decidedBy || "口述史整理员"), field("replacementText", "替代文本（仅替换时）", "textarea", existing?.replacementText || "", true),
    field("rationale", "处置理由", "textarea", existing?.rationale || "", true)
  ].join(""), "保存并闭合", async (values) => {
    await api(`/api/v1/cases/${c.id}/decisions`, { method: "POST", body: { expectedVersion: c.version, idempotencyKey: idem(), actor: values.actor, findingId: findingID, action: values.action, replacementText: values.replacementText, rationale: values.rationale }});
    closeDialog(); notify("阻断项已闭合"); await loadCases(c.id);
  });
}

function updateBatchDecisionButton() {
  const selected = $$(".decision-select:checked");
  const button = $("#batch-decision-button");
  button.classList.toggle("hidden", selected.length === 0);
  button.textContent = `批量处置（${selected.length}）`;
}

function openBatchDecisionDialog() {
  const c = state.current;
  const findingIDs = $$(".decision-select:checked").map(input => input.dataset.findingId);
  if (!findingIDs.length) return;
  const findings = findingIDs.map(id => c.findings.find(item => item.id === id));
  const openCount = c.findings.filter(item => item.severity === "block" && item.resolutionStatus === "open").length;
  const selectedOpenCount = findings.filter(item => item.resolutionStatus === "open").length;
  const rows = findings.map((finding, index) => {
    const existing = (c.decisions || []).find(item => item.findingId === finding.id);
    return `<div class="decision-batch-row" data-finding-id="${escapeHTML(finding.id)}">
      <div><strong>${escapeHTML(finding.ruleCode)}</strong><span>${escapeHTML(finding.segmentId)}</span></div>
      <select data-key="action"><option value="redact" ${existing?.action === "redact" ? "selected" : ""}>遮蔽</option><option value="replace" ${existing?.action === "replace" ? "selected" : ""}>替换</option><option value="keep" ${existing?.action === "keep" ? "selected" : ""}>保留</option></select>
      <input data-key="replacementText" placeholder="替代文本" value="${escapeHTML(existing?.replacementText || "")}">
      <input data-key="rationale" placeholder="非空处置理由" value="${escapeHTML(existing?.rationale || "")}">
      <p class="decision-candidate"></p>
    </div>`;
  }).join("");
  openDialog("03 · BLOCK RESOLUTION BATCH", "阻断项批量处置", `<div class="field wide"><label for="f-actor">处置人</label><input id="f-actor" name="actor" value="口述史整理员"></div><div class="batch-status wide">已选择 <strong>${findingIDs.length}</strong> 项；成功后预计剩余 <strong>${Math.max(0, openCount - selectedOpenCount)}</strong> 个未闭合阻断。</div><div class="decision-batch wide">${rows}</div>`, "预检并原子闭合", async (values) => {
    const decisions = $$(".decision-batch-row").map(row => ({
      findingId: row.dataset.findingId, action: row.querySelector('[data-key="action"]').value,
      replacementText: row.querySelector('[data-key="replacementText"]').value,
      rationale: row.querySelector('[data-key="rationale"]').value
    }));
    try {
      await api(`/api/v1/cases/${c.id}/decisions/batch`, { method: "POST", body: { expectedVersion: c.version, idempotencyKey: idem(), actor: values.actor, decisions }});
    } catch (error) {
      const conflicts = error.details?.conflicts || [];
      if (conflicts.length) error.message += `：${conflicts.map(item => `${item.findingIds.join(" / ")} → ${Object.values(item.candidateResults).join(" / ")}`).join("；")}`;
      throw error;
    }
    closeDialog(); notify(`已原子闭合 ${decisions.length} 个阻断项`); await loadCases(c.id);
  });
  const refresh = () => $$(".decision-batch-row").forEach(row => {
    const finding = c.findings.find(item => item.id === row.dataset.findingId);
    const segment = c.segments.find(item => item.id === finding.segmentId);
    const action = row.querySelector('[data-key="action"]').value;
    const replacement = row.querySelector('[data-key="replacementText"]').value.trim().replace(/\s+/g, " ");
    const after = action === "redact" ? "〔内容已遮蔽〕" : action === "replace" ? (replacement || "〔等待替代文本〕") : segment.originalText;
    row.querySelector(".decision-candidate").textContent = `上下文预检：${segment.originalText} → ${after}`;
  });
  $$(".decision-batch-row input, .decision-batch-row select").forEach(input => input.addEventListener("input", refresh));
  refresh();
}

async function metaCommand(suffix, actor) {
  const c = state.current;
  await api(`/api/v1/cases/${c.id}/${suffix}`, { method: "POST", body: { expectedVersion: c.version, idempotencyKey: idem(), actor }});
  notify("控制点已完成并写入时间线"); await loadCases(c.id);
}

async function approveReview() {
  const c = state.current;
  const items = $$(".review-item").map(input => ({ findingId: input.value, confirmed: input.checked }));
  await api(`/api/v1/cases/${c.id}/review/decision`, { method: "POST", body: { expectedVersion: c.version, idempotencyKey: idem(), actor: "隐私复核员", outcome: "approved", items }});
  notify("独立复核已批准"); await loadCases(c.id);
}

function openReturnDialog() {
  const c = state.current;
  openDialog("05 · STRUCTURED RETURN", "退回定向整改", [
    field("reasonCode", "理由代码", "select", "PREVIEW_CONTEXT", false, [{value:"PREVIEW_CONTEXT",label:"预览上下文不足"},{value:"REDACTION_SCOPE",label:"遮蔽范围不当"},{value:"CONSENT_EVIDENCE",label:"授权依据不足"}]),
    field("actor", "复核员", "text", "隐私复核员"), field("affected", "受影响片段 ID（每行一项）", "textarea", c.segments[0]?.id || "", true), field("reason", "退回说明", "textarea", "", true)
  ].join(""), "确认退回", async (values) => {
    const items = $$(".review-item").map(input => ({ findingId: input.value, confirmed: input.checked }));
    await api(`/api/v1/cases/${c.id}/review/decision`, { method: "POST", body: { expectedVersion: c.version, idempotencyKey: idem(), actor: values.actor, outcome: "returned", reasonCode: values.reasonCode, reason: values.reason, affectedSegmentIds: lines(values.affected), items }});
    closeDialog(); notify("案件已按指定片段退回"); await loadCases(c.id);
  });
}

function openApprovalDialog() {
  const c = state.current;
  openDialog("06 · FINAL AUTHORITY", "批准并封存发布包", [field("actor", "开放负责人", "text", "档案开放负责人", true), `<div class="field wide"><label>固定版本</label><p>案件 ${escapeHTML(c.id)} · v${c.version}。批准后生成规范化文本、决定清单、复核快照与 SHA-256 摘要，且不可再次变更。</p></div>`].join(""), "最终批准", async (values) => {
    await api(`/api/v1/cases/${c.id}/approval`, { method: "POST", body: { expectedVersion: c.version, idempotencyKey: idem(), actor: values.actor }});
    closeDialog(); notify("发布包已不可变封存"); state.activeTab = "evidence"; await loadCases(c.id);
  });
}

async function verifyPackage() {
  const c = state.current;
  state.verification = await api(`/api/v1/cases/${c.id}/package/verify`);
  renderEvidence(c);
  notify(state.verification.valid ? "摘要重算一致" : "摘要校验失败");
}

function lines(value) { return String(value || "").split(/\r?\n/).map(item => item.trim()).filter(Boolean); }
function showError(error) { notify(error.message || "操作失败"); console.error(error); }
function activateTab(name) {
  state.activeTab = name;
  $$(".tab").forEach(button => button.classList.toggle("active", button.dataset.tab === name));
  $$(".tab-panel").forEach(panel => panel.classList.toggle("hidden", panel.id !== `tab-${name}`));
}

$("#new-case-button").addEventListener("click", openCaseDialog);
$("#empty-new-button").addEventListener("click", openCaseDialog);
$("#add-segment-button").addEventListener("click", openBatchSegmentsDialog);
$("#batch-decision-button").addEventListener("click", openBatchDecisionDialog);
$("#dialog-close").addEventListener("click", closeDialog);
$("#dialog-cancel").addEventListener("click", closeDialog);
$("#modal-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!modalSubmit) return;
  const submit = $("#dialog-submit");
  const errorBox = $("#dialog-error");
  submit.disabled = true;
  errorBox.classList.add("hidden");
  try { await modalSubmit(formValues()); }
  catch (error) {
    const fields = Object.entries(error.fields || {}).map(([name, message]) => `${name}：${message}`);
    errorBox.textContent = [error.message, ...fields].join("\n");
    errorBox.classList.remove("hidden");
  }
  finally { submit.disabled = false; }
});
$$(".tab").forEach(button => button.addEventListener("click", () => activateTab(button.dataset.tab)));

loadCases().catch(showError);
