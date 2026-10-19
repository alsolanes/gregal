(() => {
  "use strict";

  const vscode = acquireVsCodeApi();
  const root = document.getElementById("app");
  const labelNode = document.getElementById("gregal-labels");
  const labels = Object.assign({
    title: "Gregal", connectionConnected: "Connected", connectionOffline: "Offline",
    connectionConnecting: "Connecting", modelFallback: "Assistant", newChat: "New chat",
    settings: "Settings", savedConversations: "Saved conversations", welcomeEyebrow: "YOUR CODING ASSISTANT", welcomeTitle: "What are we working on?",
    welcomeBody: "Ask about your code, explore an idea, or let Gregal take care of a change.",
    promptExplain: "Explain this code", promptFix: "Help me fix a bug", promptReview: "Review my changes",
    you: "You", gregal: "Gregal", thinking: "Thinking", conversation: "Conversation", modeSelector: "Conversation mode",
    statusReady: "Ready when you are",
    statusWorking: "Working…", statusWaiting: "Waiting for your approval", statusError: "Something went wrong",
    statusQuestion: "Gregal has a question", statusComplete: "Finished", statusFailed: "Run failed", statusCancelled: "Run cancelled", statusBlocked: "Blocked",
    composerPlaceholder: "Ask Gregal anything…", send: "Send message", stop: "Stop response",
    modeCode: "Code", modeChat: "Chat", modeInspect: "Inspect", includeContext: "Include editor context",
    attachSelection: "Add selection as context", contextLabel: "Editor context", removeContext: "Remove context",
    copy: "Copy", copied: "Copied", toolRunning: "Running", toolDone: "Completed",
    delegateRunning: "Working", delegateDone: "Finished", approvalRequired: "Approval required",
    error: "Error", clearChat: "Start a new chat?", review: "Review changes"
  }, (() => { try { return JSON.parse(labelNode?.textContent || "{}"); } catch { return {}; } })());
  const iconUri = root.dataset.iconUri || "";
  const MAX_MESSAGES = 150;
  const MAX_TRANSCRIPT_CHARS = 2_000_000;
  const MAX_ACTIVITIES = 50;
  const MAX_MESSAGE_CHARS = 250_000;
  const MAX_ACTIVITY_CHARS = 20_000;
  const truncateText = (text) => {
    const value = String(text || "");
    if (value.length <= MAX_MESSAGE_CHARS) return value;
    const marker = "\n\n… [message truncated]";
    return value.slice(0, MAX_MESSAGE_CHARS - marker.length) + marker;
  };
  const previous = vscode.getState() || {};
  const previousMessages = Array.isArray(previous.messages) ? previous.messages : [];
  const state = {
    busy: false, connected: false, model: "", project: "", context: null, error: "", terminal: "", awaiting: "",
    mode: previous.mode || "code", includeContext: previous.includeContext !== false,
    draft: previous.draft || "", messages: previousMessages.slice(-MAX_MESSAGES).map((message) => ({
      role: message?.role, text: truncateText(message?.text)
    })),
    activities: [], atBottom: true, activeAssistant: null, lastAssistant: null
  };
  const activityNodes = new Map();
  const messageItems = [];
  let pendingBubble = null;
  let bubbleFrame = null;
  let persistTimer = null;
  let renderedContextKey = null;

  const el = (tag, cls, text) => {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text != null) node.textContent = text;
    return node;
  };
  const icon = (name) => {
    const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.setAttribute("viewBox", "0 0 24 24"); svg.setAttribute("aria-hidden", "true");
    const paths = {
      plus: "M12 5v14M5 12h14", history: "M3 12a9 9 0 1 0 2.64-6.36L3 8m0-5v5h5m4-1v5l3 2", settings: "M12 8.5a3.5 3.5 0 1 0 0 7 3.5 3.5 0 0 0 0-7ZM19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-1.7 2.94-.08-.02a1.7 1.7 0 0 0-1.82.57l-.05.07h-3.4l-.04-.08a1.7 1.7 0 0 0-1.55-.96c-.3 0-.58.08-.82.22l-.08.04-2.94-1.7.02-.08a1.7 1.7 0 0 0-.57-1.82l-.07-.05v-3.4l.08-.04a1.7 1.7 0 0 0 .74-2.37l-.04-.08 1.7-2.94.08.02a1.7 1.7 0 0 0 1.82-.57l.05-.07h3.4l.04.08a1.7 1.7 0 0 0 2.37.74l.08-.04 2.94 1.7-.02.08a1.7 1.7 0 0 0 .57 1.82l.07.05v3.4l-.08.04A1.7 1.7 0 0 0 19.4 15Z",
      attach: "M12 5v14M5 12h14", close: "M6 6l12 12M18 6 6 18", send: "m5 12 14-7-4 14-3-6-7-1Zm7 1 7-8", stop: "M7 7h10v10H7z", copy: "M8 8h11v13H8zM5 16H4V3h11v2", chevron: "m9 18 6-6-6-6", spark: "m12 3 1.6 6.4L20 11l-6.4 1.6L12 19l-1.6-6.4L4 11l6.4-1.6L12 3Z"
    };
    svg.setAttribute("class", "icon");
    const p = document.createElementNS("http://www.w3.org/2000/svg", "path");
    p.setAttribute("d", paths[name] || paths.spark);
    if (["plus", "attach", "close", "send", "copy", "chevron"].includes(name)) {
      p.setAttribute("fill", "none"); p.setAttribute("stroke", "currentColor"); p.setAttribute("stroke-width", "1.7"); p.setAttribute("stroke-linecap", "round"); p.setAttribute("stroke-linejoin", "round");
    } else if (name === "stop") p.setAttribute("fill", "currentColor");
    else { p.setAttribute("fill", "none"); p.setAttribute("stroke", "currentColor"); p.setAttribute("stroke-width", "1.4"); p.setAttribute("stroke-linecap", "round"); p.setAttribute("stroke-linejoin", "round"); }
    svg.appendChild(p); return svg;
  };

  root.replaceChildren();
  root.classList.add("gregal-chat");
  const header = el("header", "chat-header");
  const brand = el("div", "brand");
  const brandMark = el("span", "brand-mark");
  if (iconUri) { const img = el("img"); img.src = iconUri; img.alt = ""; brandMark.appendChild(img); }
  else {
    const mark = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    mark.setAttribute("viewBox", "0 0 32 32"); mark.setAttribute("aria-hidden", "true");
    const wind = document.createElementNS("http://www.w3.org/2000/svg", "path");
    wind.setAttribute("d", "M4 11c6-5 12-5 18 0 2 2 4 2 6 0M4 17c6-5 12-5 18 0 2 2 4 2 6 0M4 23c6-5 12-5 18 0 2 2 4 2 6 0");
    wind.setAttribute("fill", "none"); wind.setAttribute("stroke", "currentColor"); wind.setAttribute("stroke-width", "2.4"); wind.setAttribute("stroke-linecap", "round");
    mark.appendChild(wind); brandMark.appendChild(mark);
  }
  brand.append(brandMark, el("div", "brand-copy", labels.title));
  const connection = el("div", "connection");
  const connectionDot = el("span", "connection-dot"); connectionDot.setAttribute("aria-hidden", "true");
  const connectionText = el("span", "connection-text", labels.connectionConnecting);
  connection.append(connectionDot, connectionText);
  const headerActions = el("div", "header-actions");
    const savedConversations = button("icon-button", labels.savedConversations, icon("history"), () => vscode.postMessage({ type: "savedConversations" }));
    const newChat = button("new-chat-button", labels.newChat, icon("plus"), () => vscode.postMessage({ type: "newChat" }));
  const reviewButton = button("review-button", labels.review, null, () => vscode.postMessage({ type: "review" }));
  reviewButton.appendChild(el("span", "review-label", labels.review));
  const settings = button("icon-button", labels.settings, icon("settings"), () => vscode.postMessage({ type: "settings" }));
  headerActions.append(reviewButton, savedConversations, newChat, settings); header.append(brand, connection, headerActions);

  const modelLine = el("div", "model-line");
  const modelName = el("span", "model-name", labels.modelFallback);
  const projectName = el("span", "model-project");
  const connectionError = el("span", "connection-error");
  connectionError.hidden = true;
  projectName.hidden = true;
  modelLine.append(modelName, projectName, connectionError);

  const feed = el("main", "chat-feed");
  feed.setAttribute("role", "log"); feed.setAttribute("aria-label", labels.conversation); feed.setAttribute("aria-live", "off");
  const welcome = el("section", "welcome");
  const welcomeIcon = el("div", "welcome-icon"); welcomeIcon.appendChild(icon("spark"));
  welcome.append(welcomeIcon, el("div", "welcome-eyebrow", labels.welcomeEyebrow), el("h1", "welcome-title", labels.welcomeTitle), el("p", "welcome-body", labels.welcomeBody));
  const starters = el("div", "starter-list");
  [labels.promptExplain, labels.promptFix, labels.promptReview].forEach((prompt) => {
    const b = button("starter-button", prompt, icon("chevron"), () => { composer.value = prompt; fitComposer(); persist(); composer.focus(); });
    b.classList.add("starter-button"); starters.appendChild(b);
  });
  welcome.appendChild(starters);
  const transcript = el("div", "transcript");
  feed.append(welcome, transcript);

  const bottomStatus = el("div", "live-status"); bottomStatus.setAttribute("role", "status"); bottomStatus.setAttribute("aria-live", "polite");
  const statusDot = el("span", "status-dot");
  const statusText = el("span", "status-text", labels.statusReady);
  bottomStatus.append(statusDot, statusText);

  const composerWrap = el("footer", "composer-area");
  const activityList = el("div", "activity-list");
  const contextCard = el("div", "context-card");
  const composerCard = el("div", "composer-card");
  const composer = el("textarea", "composer-input");
  composer.setAttribute("aria-label", labels.composerPlaceholder); composer.setAttribute("rows", "2"); composer.setAttribute("placeholder", labels.composerPlaceholder); composer.value = state.draft;
  const composerBottom = el("div", "composer-bottom");
  const controls = el("div", "composer-controls");
  const modeSelect = el("select", "mode-select"); modeSelect.setAttribute("aria-label", labels.modeSelector);
  [["code", labels.modeCode], ["chat", labels.modeChat], ["inspect", labels.modeInspect]].forEach(([value, label]) => { const o = el("option", "", label); o.value = value; modeSelect.appendChild(o); });
  modeSelect.value = state.mode; modeSelect.addEventListener("change", () => { state.mode = modeSelect.value; persist(); });
  const contextToggle = el("label", "context-toggle");
  const contextCheck = el("input", ""); contextCheck.type = "checkbox"; contextCheck.checked = state.includeContext;
  contextCheck.addEventListener("change", () => { state.includeContext = contextCheck.checked; persist(); });
  contextToggle.append(contextCheck, el("span", "context-toggle-text", labels.includeContext));
  controls.append(modeSelect, contextToggle);
  const composerActions = el("div", "composer-actions");
  const attach = button("attach-button", labels.attachSelection, icon("attach"), () => vscode.postMessage({ type: "attachContext" }));
  const sendButton = button("send-button", labels.send, icon("send"), send);
  const stopButton = button("stop-button", labels.stop, icon("stop"), () => vscode.postMessage({ type: "cancel" }));
  composerActions.append(attach, sendButton, stopButton);
  composerBottom.append(controls, composerActions);
  composerCard.append(composer, composerBottom);
  composerWrap.append(activityList, contextCard, composerCard);
  root.append(header, modelLine, feed, bottomStatus, composerWrap);

  function button(cls, label, glyph, action) {
    const b = el("button", cls); b.type = "button"; b.setAttribute("aria-label", label); b.title = label;
    if (glyph) b.appendChild(glyph); if (cls.includes("starter")) b.appendChild(el("span", "starter-label", label));
    b.addEventListener("click", action); return b;
  }
  function persist() {
    if (persistTimer !== null) { clearTimeout(persistTimer); persistTimer = null; }
    vscode.setState({ draft: composer.value, mode: state.mode, includeContext: state.includeContext,
      messages: state.messages.map((message) => ({ role: message.role, text: message.text })) });
  }
  function schedulePersist() {
    if (persistTimer !== null) clearTimeout(persistTimer);
    persistTimer = setTimeout(() => { persistTimer = null; persist(); }, 200);
  }
  function scheduleBubbleRender(item) {
    if (pendingBubble && pendingBubble !== item) flushBubbleRender();
    pendingBubble = item;
    if (bubbleFrame !== null) return;
    const render = () => {
      bubbleFrame = null;
      const target = pendingBubble; pendingBubble = null;
      if (target) renderBubble(target);
    };
    if (typeof window.requestAnimationFrame === "function") bubbleFrame = window.requestAnimationFrame(render);
    else bubbleFrame = setTimeout(render, 30);
  }
  function flushBubbleRender() {
    if (!pendingBubble) return;
    if (bubbleFrame !== null) {
      if (typeof window.cancelAnimationFrame === "function") window.cancelAnimationFrame(bubbleFrame);
      else clearTimeout(bubbleFrame);
      bubbleFrame = null;
    }
    const target = pendingBubble; pendingBubble = null;
    renderBubble(target);
  }
  function trimMessages() {
    let totalChars = state.messages.reduce((sum, message) => sum + message.text.length, 0);
    while (state.messages.length > MAX_MESSAGES || (state.messages.length > 1 && totalChars > MAX_TRANSCRIPT_CHARS)) {
      totalChars -= state.messages[0].text.length;
      state.messages.shift();
      const removed = messageItems.shift();
      if (removed) removed.article.remove();
    }
    messageItems.forEach((item, index) => { if (item) item.index = index; });
  }
  function send() {
    const text = composer.value.trim();
    if (!text || state.busy) return;
    vscode.postMessage({ type: "send", text, mode: state.mode, includeContext: state.includeContext });
    composer.value = ""; fitComposer(); persist(); renderStatus(); composer.focus();
  }
  function fitComposer() { composer.style.height = "auto"; composer.style.height = `${Math.min(composer.scrollHeight, 176)}px`; }
  function markdown(parent, source) {
    const lines = String(source || "").replace(/\r\n?/g, "\n").split("\n");
    let i = 0;
    while (i < lines.length) {
      const line = lines[i];
      const fence = line.match(/^\s*```([\w+-]*)\s*$/);
      if (fence) {
        const codeLines = []; i++;
        while (i < lines.length && !/^\s*```\s*$/.test(lines[i])) codeLines.push(lines[i++]);
        if (i < lines.length) i++;
        const box = el("div", "code-block"); const bar = el("div", "code-toolbar");
        bar.appendChild(el("span", "code-language", fence[1] || "code"));
        const codeText = codeLines.join("\n");
        bar.appendChild(button("copy-button", labels.copy, icon("copy"), async () => {
          try { await navigator.clipboard.writeText(codeText); vscode.postMessage({ type: "copy", text: codeText }); }
          catch { vscode.postMessage({ type: "copy", text: codeText }); }
        }));
        const pre = el("pre"); const code = el("code", "", codeText); pre.appendChild(code); box.append(bar, pre); parent.appendChild(box); continue;
      }
      const heading = line.match(/^\s{0,3}(#{1,3})\s+(.+)$/);
      if (heading) { parent.appendChild(inline(el(`h${heading[1].length}`, "md-heading"), heading[2])); i++; continue; }
      if (!line.trim()) { i++; continue; }
      if (/^\s*[-*+]\s+/.test(line)) {
        const list = el("ul", "md-list");
        while (i < lines.length && /^\s*[-*+]\s+/.test(lines[i])) list.appendChild(inline(el("li"), lines[i++].replace(/^\s*[-*+]\s+/, "")));
        parent.appendChild(list); continue;
      }
      if (/^\s*\d+[.)]\s+/.test(line)) {
        const list = el("ol", "md-list");
        while (i < lines.length && /^\s*\d+[.)]\s+/.test(lines[i])) list.appendChild(inline(el("li"), lines[i++].replace(/^\s*\d+[.)]\s+/, "")));
        parent.appendChild(list); continue;
      }
      const paragraph = [line.trim()]; i++;
      while (i < lines.length && lines[i].trim() && !/^\s*(?:```|#{1,3}\s|[-*+]\s|\d+[.)]\s)/.test(lines[i])) paragraph.push(lines[i++].trim());
      parent.appendChild(inline(el("p", "md-paragraph"), paragraph.join(" ")));
    }
  }
  function inline(parent, source) {
    const token = /(`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*|\[[^\]]+\]\((?:https?:\/\/)[^)]+\))/g;
    let cursor = 0; let match;
    while ((match = token.exec(source))) {
      if (match.index > cursor) parent.appendChild(document.createTextNode(source.slice(cursor, match.index)));
      const raw = match[0]; let node;
      if (raw.startsWith("`")) node = el("code", "inline-code", raw.slice(1, -1));
      else if (raw.startsWith("**")) node = el("strong", "", raw.slice(2, -2));
      else if (raw.startsWith("*")) node = el("em", "", raw.slice(1, -1));
      else { const parts = raw.match(/^\[([^\]]+)\]\((https?:\/\/[^)]+)\)$/); node = el("a", "", parts[1]); node.href = parts[2]; node.target = "_blank"; node.rel = "noopener noreferrer"; }
      parent.appendChild(node); cursor = token.lastIndex;
    }
    if (cursor < source.length) parent.appendChild(document.createTextNode(source.slice(cursor)));
    return parent;
  }
  function makeMessageBubble(role, text, index = state.messages.length) {
    const article = el("article", `message message-${role}`);
    const header = el("div", "message-header");
    const avatar = el("span", `message-avatar avatar-${role}`, role === "user" ? "Y" : "G");
    header.append(avatar, el("span", "message-author", role === "user" ? labels.you : role === "system" ? labels.error : labels.gregal));
    const body = el("div", "message-body");
    article.append(header, body); transcript.appendChild(article);
    const item = { article, body, role, text: truncateText(text), index };
    renderBubble(item); return item;
  }
  function renderBubble(item) {
    item.body.replaceChildren();
    if (item.role === "assistant") markdown(item.body, item.text);
    else item.body.appendChild(el("p", "md-paragraph", item.text));
    if (item.index < state.messages.length && item.index >= 0) state.messages[item.index].text = item.text;
    welcome.hidden = state.messages.length > 0 || Boolean(item.text);
    followIfAtBottom();
  }
  function appendMessage(message, save = true, historyIndex = -1) {
    if (!message || !["user", "assistant", "system"].includes(message.role)) return;
    const text = truncateText(message.text);
    if (save && message.role === "assistant" && (state.activeAssistant || state.lastAssistant)) {
      const item = state.activeAssistant || state.lastAssistant;
      item.text = text; renderBubble(item);
      state.activeAssistant = null; state.lastAssistant = item;
      if (item.index >= 0 && item.index < state.messages.length) state.messages[item.index].text = text;
      persist();
    } else {
      if (message.role === "user") {
        flushBubbleRender();
        state.activeAssistant = null;
        state.lastAssistant = null;
        state.awaiting = ""; state.terminal = ""; state.error = "";
        terminalizeActivities("stopped");
      }
      const item = makeMessageBubble(message.role, text, save ? state.messages.length : historyIndex);
      if (save) state.messages.push({ role: message.role, text });
      messageItems[item.index] = item;
      if (message.role === "assistant") state.lastAssistant = item;
      trimMessages();
      persist();
    }
    welcome.hidden = state.messages.length > 0;
    renderActivities(); renderStatus();
  }
  function replaceAssistantText(text) {
    if (typeof text !== "string") return;
    text = truncateText(text);
    flushBubbleRender();
    const item = state.activeAssistant || state.lastAssistant;
    if (item) {
      item.text = text; renderBubble(item);
      state.activeAssistant = null; state.lastAssistant = item;
      return;
    }
    const created = makeMessageBubble("assistant", text);
    created.index = state.messages.length;
    state.messages.push({ role: "assistant", text });
    messageItems[created.index] = created;
    trimMessages();
    state.lastAssistant = created; persist();
  }
  function appendAssistantText(text) {
    if (typeof text !== "string" || !text) return;
    if (!state.activeAssistant) {
      const item = makeMessageBubble("assistant", "", state.messages.length);
      item.index = state.messages.length;
      state.messages.push({ role: "assistant", text: "" });
      messageItems[item.index] = item;
      state.activeAssistant = item;
      state.lastAssistant = item;
      trimMessages();
    }
    const item = state.activeAssistant;
    item.text = truncateText(item.text + text);
    if (item.index >= 0 && item.index < state.messages.length) state.messages[item.index].text = item.text;
    scheduleBubbleRender(item); schedulePersist();
  }
  function finishAssistant(finalText) {
    if (typeof finalText === "string" && finalText.length) {
      replaceAssistantText(finalText);
      return;
    }
    if (state.activeAssistant) { flushBubbleRender(); state.activeAssistant = null; }
    persist();
  }
  function followIfAtBottom() { if (state.atBottom) feed.scrollTop = feed.scrollHeight; }
  feed.addEventListener("scroll", () => { state.atBottom = feed.scrollHeight - feed.scrollTop - feed.clientHeight < 72; });
  function renderContext() {
    if (!state.context) {
      if (renderedContextKey !== null) contextCard.replaceChildren();
      renderedContextKey = null; contextCard.hidden = true; return;
    }
    const contextLabel = state.context.label || "Editor";
    const contextDetail = state.context.detail || "";
    const contextKey = JSON.stringify([contextLabel, contextDetail]);
    if (contextKey === renderedContextKey) return;
    renderedContextKey = contextKey;
    contextCard.replaceChildren();
    contextCard.hidden = false;
    const details = el("div", "context-details"); details.append(el("span", "context-kicker", labels.contextLabel), el("span", "context-name", contextLabel));
    if (contextDetail) details.appendChild(el("span", "context-detail", contextDetail));
    const remove = button("remove-context", labels.removeContext, icon("close"), () => vscode.postMessage({ type: "clearContext" }));
    contextCard.append(details, remove);
  }
  function renderStatus() {
    connectionText.textContent = state.connected ? labels.connectionConnected : state.busy ? labels.connectionConnecting : labels.connectionOffline;
    connection.classList.toggle("is-connected", state.connected); connection.classList.toggle("is-offline", !state.connected && !state.busy);
    modelName.textContent = state.model || labels.modelFallback;
    statusText.textContent = state.error ? labels.statusFailed : state.awaiting === "approval" ? labels.statusWaiting : state.awaiting === "question" ? labels.statusQuestion : state.terminal === "failed" ? labels.statusFailed : state.terminal === "cancelled" ? labels.statusCancelled : state.terminal === "completed" ? labels.statusComplete : state.busy ? labels.statusWorking : labels.statusReady;
    connectionError.textContent = !state.connected && state.error ? state.error : "";
    connectionError.hidden = state.connected || !state.error;
    bottomStatus.classList.toggle("has-error", Boolean(state.error)); bottomStatus.classList.toggle("is-working", state.busy);
    bottomStatus.classList.toggle("is-terminal", Boolean(state.terminal) && !state.busy);
    sendButton.hidden = state.busy; stopButton.hidden = !state.busy; composer.disabled = false;
  }
  function eventText(value) {
    if (typeof value === "string") return value;
    if (value == null) return "";
    try { return JSON.stringify(value, null, 2); } catch { return String(value); }
  }
  function upsertActivity(key, values) {
    let item = state.activities.find((activity) => activity.key === key);
    if (!item) {
      item = { key, type: "tool", status: "running", title: "", detail: "" }; state.activities.push(item);
      while (state.activities.length > MAX_ACTIVITIES) {
        const removed = state.activities.shift();
        activityNodes.get(removed.key)?.remove(); activityNodes.delete(removed.key);
      }
    }
    const safeValues = { ...values };
    if (typeof safeValues.title === "string") safeValues.title = safeValues.title.slice(0, 500);
    if (typeof safeValues.detail === "string") safeValues.detail = safeValues.detail.length > MAX_ACTIVITY_CHARS
      ? `${safeValues.detail.slice(0, MAX_ACTIVITY_CHARS - 24)}\n… [details truncated]` : safeValues.detail;
    Object.assign(item, safeValues);
    let row = activityNodes.get(key);
    if (!row) {
      row = el("details", "activity");
      const summary = el("summary", "activity-summary");
      const indicator = el("span", "activity-indicator"); indicator.setAttribute("aria-hidden", "true");
      const title = el("span", "activity-label"); const chevron = icon("chevron");
      summary.append(indicator, title, chevron);
      const detail = el("div", "activity-detail");
      row.append(summary, detail); activityNodes.set(key, row);
      row._title = title; row._detail = detail;
    }
    row.className = `activity is-${item.status}`;
    row._title.textContent = item.title || defaultActivityTitle(item);
    row._detail.textContent = item.detail || "";
    row._detail.hidden = !item.detail;
    return item;
  }
  function defaultActivityTitle(item) {
    if (item.type === "delegate") return item.status === "completed" ? labels.delegateDone : item.status === "failed" ? labels.statusFailed : item.status === "cancelled" || item.status === "blocked" ? labels.statusCancelled : labels.delegateRunning;
    if (item.type === "approval") return labels.approvalRequired;
    if (item.type === "question") return labels.statusQuestion;
    if (item.status === "completed") return labels.toolDone;
    if (item.status === "failed") return labels.statusFailed;
    if (item.status === "blocked") return labels.statusBlocked;
    if (item.status === "cancelled" || item.status === "stopped") return labels.statusCancelled;
    return labels.toolRunning;
  }
  function renderActivities() {
    for (const item of state.activities) upsertActivity(item.key, item);
    const visible = state.activities.slice(-3);
    const keep = new Set(visible.map((item) => item.key));
    for (const child of [...activityList.children]) if (!keep.has(child.dataset.activityKey)) child.remove();
    const ordered = visible.map((item) => activityNodes.get(item.key));
    const current = [...activityList.children];
    if (current.length !== ordered.length || current.some((row, index) => row !== ordered[index])) {
      ordered.forEach((row, index) => { row.dataset.activityKey = visible[index].key; activityList.appendChild(row); });
    }
  }
  function terminalizeActivities(status) {
    state.activities.forEach((item) => { if (item.status === "running" || item.status === "waiting") item.status = status; });
    renderActivities();
  }
  function resumeWaitingActivities() {
    state.awaiting = "";
    state.activities.forEach((item) => {
      if (item.status !== "waiting") return;
      item.status = item.type === "approval" || item.type === "question" ? "resolved" : "running";
    });
  }
  function addEvent(kind, data = {}) {
    const text = eventText(data.message ?? data.text ?? data.summary ?? data.name);
    switch (kind) {
      case "token":
        appendAssistantText(typeof data.text === "string" ? data.text : ""); break;
      case "assistant": case "text":
        replaceAssistantText(typeof data.text === "string" ? data.text : ""); break;
      case "done": {
        finishAssistant(typeof data.reply === "string" ? data.reply : typeof data.text === "string" ? data.text : "");
        state.busy = false; state.awaiting = ""; state.terminal = "completed"; state.error = "";
        terminalizeActivities("completed"); break;
      }
      case "tool_call": {
        // The pre-tool response is a complete bubble; post-tool tokens start a new one.
        flushBubbleRender();
        state.activeAssistant = null; state.lastAssistant = null;
        const key = `tool:${data.id ?? `anonymous-${state.activities.length}`}`;
        upsertActivity(key, { type: "tool", toolId: data.id, status: "running", title: eventText(data.name) || labels.toolRunning, detail: eventText(data.args) });
        break;
      }
      case "tool_result": {
        const key = `tool:${data.id ?? `anonymous-${state.activities.length}`}`;
        const existing = state.activities.find((item) => item.key === key);
        const output = eventText(data.output ?? data.error);
        const failed = data.failed === true || Boolean(data.error) || /^ERROR:/i.test(output.trimStart());
        upsertActivity(key, { type: "tool", toolId: data.id, status: failed ? "failed" : "completed", title: eventText(data.name) || existing?.title || (failed ? labels.statusFailed : labels.toolDone), detail: output });
        break;
      }
      case "blocked": {
        const key = `tool:${data.id ?? `blocked-${state.activities.length}`}`;
        const existing = state.activities.find((item) => item.key === key);
        upsertActivity(key, { type: "tool", toolId: data.id, status: "blocked", title: existing?.title || labels.statusBlocked, detail: eventText(data.reason) });
        break;
      }
      case "delegate": {
        const key = `delegate:${data.id ?? "default"}`;
        const status = ["failed", "cancelled", "blocked", "completed"].includes(data.status) ? data.status : "running";
        const parts = [data.model, data.message, data.step && data.max_steps ? `${data.step} / ${data.max_steps}` : "", data.tool].filter(Boolean);
        const fallbackTitle = status === "failed" ? labels.statusFailed : status === "cancelled" ? labels.statusCancelled : status === "completed" ? labels.delegateDone : labels.delegateRunning;
        const detail = [parts.join(" · "), eventText(data.output)].filter(Boolean).join("\n\n");
        upsertActivity(key, { type: "delegate", delegateId: data.id, status, title: eventText(data.task) || fallbackTitle, detail });
        break;
      }
      case "approve_request": {
        state.awaiting = "approval";
        const pendingTool = [...state.activities].reverse().find((item) => item.type === "tool" && item.status === "running" && (!data.name || item.title === data.name));
        if (pendingTool) pendingTool.status = "waiting";
        upsertActivity(`approval:${data.id ?? state.activities.length}`, { type: "approval", status: "waiting", title: labels.approvalRequired, detail: [eventText(data.name), eventText(data.args)].filter(Boolean).join(" · ") });
        break;
      }
      case "question_request":
        state.awaiting = "question";
        upsertActivity(`question:${data.id ?? state.activities.length}`, { type: "question", status: "waiting", title: labels.statusQuestion, detail: eventText(data.query) || text });
        break;
      case "run_completed":
        finishAssistant(typeof data.reply === "string" ? data.reply : typeof data.text === "string" ? data.text : "");
        state.busy = false; state.awaiting = ""; state.terminal = "completed"; state.error = ""; terminalizeActivities("completed"); break;
      case "run_failed": case "error":
        flushBubbleRender(); state.busy = false; state.awaiting = ""; state.terminal = "failed"; state.error = text || labels.statusFailed; state.activeAssistant = null; persist();
        upsertActivity("run:failure", { type: "run", status: "failed", title: labels.statusFailed, detail: text });
        terminalizeActivities("failed"); break;
      case "run_cancelled": case "cancelled":
        flushBubbleRender(); state.busy = false; state.awaiting = ""; state.terminal = "cancelled"; state.error = ""; state.activeAssistant = null; persist(); terminalizeActivities("cancelled"); break;
      case "status":
        if (data.state === "running") { state.busy = true; state.terminal = ""; state.error = ""; resumeWaitingActivities(); }
        else if (data.state === "failed") { flushBubbleRender(); state.busy = false; state.terminal = "failed"; state.error = text || labels.statusFailed; state.activeAssistant = null; persist();
          upsertActivity("run:failure", { type: "run", status: "failed", title: labels.statusFailed, detail: text }); terminalizeActivities("failed"); }
        else if (data.state === "cancelled") { flushBubbleRender(); state.busy = false; state.terminal = "cancelled"; state.error = ""; state.activeAssistant = null; persist(); terminalizeActivities("cancelled"); }
        else if (data.state === "completed") { state.busy = false; state.terminal = "completed"; terminalizeActivities("completed"); }
        break;
      default: break;
    }
    renderActivities(); renderStatus();
  }
  window.addEventListener("message", (event) => {
    const message = event.data;
    if (!message || typeof message.type !== "string") return;
    if (message.type === "state") {
      state.busy = Boolean(message.busy); state.connected = Boolean(message.connected);
      if (state.busy) { state.terminal = ""; state.error = ""; resumeWaitingActivities(); }
      if (typeof message.model === "string") state.model = message.model;
      if (Object.prototype.hasOwnProperty.call(message, "project")) {
        const project = message.project;
        state.project = (typeof project === "string" ? project : project && typeof project.name === "string" ? project.name : "").slice(0, 500);
      }
      if (Object.prototype.hasOwnProperty.call(message, "context")) state.context = message.context;
      if (typeof message.error === "string" && message.error) { state.error = message.error; state.terminal = "failed"; }
      modelName.textContent = state.model || labels.modelFallback;
      projectName.textContent = state.project;
      projectName.hidden = !state.project;
      projectName.title = state.project;
      renderContext(); renderActivities(); renderStatus();
    } else if (message.type === "message") appendMessage(message);
    else if (message.type === "event") addEvent(String(message.kind || "status"), message.data || {});
    else if (message.type === "reset") {
      if (bubbleFrame !== null) {
        if (typeof window.cancelAnimationFrame === "function") window.cancelAnimationFrame(bubbleFrame); else clearTimeout(bubbleFrame);
      }
      bubbleFrame = null; pendingBubble = null;
      if (persistTimer !== null) clearTimeout(persistTimer); persistTimer = null;
      state.messages = []; state.activities = []; state.error = ""; state.terminal = ""; state.awaiting = ""; state.activeAssistant = null; state.lastAssistant = null;
      messageItems.length = 0; activityNodes.clear(); transcript.replaceChildren(); welcome.hidden = false; renderActivities(); persist(); renderStatus(); feed.scrollTop = 0;
    }
  });
  composer.addEventListener("input", () => { fitComposer(); persist(); });
  composer.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !event.shiftKey && !event.isComposing && event.keyCode !== 229) { event.preventDefault(); send(); }
  });
  trimMessages();
  state.messages.forEach((m, index) => appendMessage(m, false, index));
  renderContext(); renderStatus(); fitComposer();
  window.addEventListener("pagehide", () => { flushBubbleRender(); persist(); });
  vscode.postMessage({ type: "ready" });
})();
