const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

class ClassList {
  constructor(node) { this.node = node; }
  values() { return new Set(this.node.className.split(/\s+/).filter(Boolean)); }
  add(...names) { const values = this.values(); names.forEach((name) => values.add(name)); this.node.className = [...values].join(" "); }
  contains(name) { return this.values().has(name); }
  toggle(name, force) {
    const values = this.values(); const shouldAdd = force === undefined ? !values.has(name) : Boolean(force);
    if (shouldAdd) values.add(name); else values.delete(name);
    this.node.className = [...values].join(" "); return shouldAdd;
  }
}

class FakeNode {
  constructor(tagName = "div", text = "") {
    this.tagName = tagName.toUpperCase(); this.children = []; this.parentNode = null;
    this.attributes = {}; this.className = ""; this.classList = new ClassList(this); this.style = {};
    this.dataset = {}; this.hidden = false; this.value = ""; this.open = false;
    this.scrollHeight = 0; this.scrollTop = 0; this.clientHeight = 0; this.listeners = {};
    this._text = text; this.focusCount = 0;
  }
  set textContent(value) { this.children.forEach((child) => { child.parentNode = null; }); this.children = []; this._text = String(value); }
  get textContent() { return this._text + this.children.map((child) => child.textContent).join(""); }
  setAttribute(name, value) { this.attributes[name] = String(value); if (name === "class") this.className = String(value); }
  append(...nodes) { nodes.forEach((node) => this.appendChild(typeof node === "string" ? new FakeNode("#text", node) : node)); }
  appendChild(node) {
    if (node.parentNode) node.parentNode.removeChild(node);
    this.children.push(node); node.parentNode = this; return node;
  }
  removeChild(node) { this.children = this.children.filter((child) => child !== node); node.parentNode = null; return node; }
  remove() { if (this.parentNode) this.parentNode.removeChild(this); }
  replaceChildren(...nodes) { this.children.forEach((child) => { child.parentNode = null; }); this.children = []; this._text = ""; this.append(...nodes); }
  addEventListener(type, listener) { (this.listeners[type] ||= []).push(listener); }
  dispatch(type, options = {}) {
    const event = { type, target: this, key: "", shiftKey: false, isComposing: false, keyCode: 0,
      preventDefault() { this.defaultPrevented = true; }, ...options };
    (this.listeners[type] || []).forEach((listener) => listener(event)); return event;
  }
  focus() { this.focusCount++; }
  get childrenArray() { return this.children; }
}

class FakeDocument {
  constructor() { this.root = new FakeNode("div"); this.labels = new FakeNode("script", "{}"); }
  getElementById(id) { return id === "app" ? this.root : id === "gregal-labels" ? this.labels : null; }
  createElement(tag) { return new FakeNode(tag); }
  createElementNS(_ns, tag) { return new FakeNode(tag); }
  createTextNode(text) { return new FakeNode("#text", text); }
}

function harness(labels = {}) {
  const document = new FakeDocument(); document.labels.textContent = JSON.stringify(labels);
  const listeners = {}; const posted = []; let saved = {}; let stateWrites = 0; let frameId = 0; const frames = new Map();
  const window = { addEventListener(type, listener) { (listeners[type] ||= []).push(listener); },
    dispatch(type, data) { (listeners[type] || []).forEach((listener) => listener({ data })); },
    requestAnimationFrame(callback) { const id = ++frameId; frames.set(id, callback); return id; },
    cancelAnimationFrame(id) { frames.delete(id); } };
  const vscode = { postMessage(message) { posted.push(message); }, getState() { return saved; }, setState(value) { saved = value; stateWrites++; } };
  const context = { document, window, navigator: { clipboard: { writeText: async () => {} } },
    acquireVsCodeApi: () => vscode, console, Set, Map, Array, Object, String, Math, JSON, setTimeout, clearTimeout };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, "..", "media", "chat.js"), "utf8"), context, { filename: "chat.js" });
  const all = (node, predicate) => node.children.flatMap((child) => [ ...(predicate(child) ? [child] : []), ...all(child, predicate) ]);
  const byClass = (cls, node = document.root) => all(node, (item) => item.classList.contains(cls));
  const dispatch = (message) => window.dispatch("message", message);
  const flushFrames = () => { const pending = [...frames.values()]; frames.clear(); pending.forEach((callback) => callback()); };
  return { document, posted, saved: () => saved, stateWrites: () => stateWrites, dispatch, all, byClass, flushFrames };
}

test("starter prompts are localized drafts and review is an explicit action", () => {
  const ui = harness({ promptExplain: "Localized explanation", promptFix: "Localized repair", promptReview: "Localized review", review: "Review changes" });
  const starters = ui.byClass("starter-button");
  assert.equal(starters.length, 3);
  starters[1].dispatch("click");
  const composer = ui.byClass("composer-input")[0];
  assert.equal(composer.value, "Localized repair");
  assert.equal(composer.focusCount, 1);
  assert.equal(ui.posted.some((message) => message.type === "send"), false);
  ui.byClass("review-button")[0].dispatch("click");
  assert.equal(ui.posted.at(-1).type, "review");
});

test("saved conversation control opens the native history picker", () => {
  const ui = harness({ savedConversations: "Saved conversations" });
  const historyButton = ui.byClass("icon-button").find((node) => node.attributes["aria-label"] === "Saved conversations");
  assert.ok(historyButton);
  historyButton.dispatch("click");
  assert.equal(ui.posted.at(-1).type, "savedConversations");
});

test("project name appears beside the model", () => {
  const ui = harness();
  ui.dispatch({ type: "state", busy: false, connected: true, model: "gregal-code", project: "gregal-agent" });
  const modelLine = ui.byClass("model-line")[0];
  assert.match(modelLine.textContent, /gregal-code/);
  assert.match(modelLine.textContent, /gregal-agent/);
  assert.equal(ui.byClass("model-project")[0].hidden, false);
});

test("connection errors remain visible while the service is offline", () => {
  const ui = harness();
  ui.dispatch({ type: "state", busy: false, connected: false, error: "Connection refused at localhost" });
  const error = ui.byClass("connection-error")[0];
  assert.equal(error.hidden, false);
  assert.match(error.textContent, /Connection refused at localhost/);
});

test("identical context updates keep the remove button node and its focus", () => {
  const ui = harness();
  const context = { label: "main.go", detail: "Lines 8–14" };
  ui.dispatch({ type: "state", busy: false, connected: true, context });
  const remove = ui.byClass("remove-context")[0]; remove.focus();
  ui.dispatch({ type: "state", busy: false, connected: true, context: { ...context } });
  assert.equal(ui.byClass("remove-context")[0], remove);
  assert.equal(remove.focusCount, 1);
});

test("Enter sends while IME Enter and Shift+Enter preserve the draft", () => {
  const ui = harness();
  const composer = ui.byClass("composer-input")[0]; composer.value = "Review this";
  composer.dispatch("keydown", { key: "Enter", isComposing: true, keyCode: 229 });
  composer.dispatch("keydown", { key: "Enter", shiftKey: true });
  assert.equal(ui.posted.some((message) => message.type === "send"), false);
  const enter = composer.dispatch("keydown", { key: "Enter" });
  assert.equal(enter.defaultPrevented, true);
  assert.deepEqual(JSON.parse(JSON.stringify(ui.posted.at(-1))), { type: "send", text: "Review this", mode: "code", includeContext: true });
  assert.equal(composer.value, "");
  assert.equal(composer.focusCount, 1);
});

test("token and text events stream into one bubble and done replaces its text", () => {
  const ui = harness();
  ui.dispatch({ type: "state", busy: true, connected: true });
  ui.dispatch({ type: "message", role: "user", text: "Say hello" });
  ui.dispatch({ type: "event", kind: "token", data: { text: "Hello" } });
  ui.dispatch({ type: "event", kind: "token", data: { text: " there" } });
  assert.equal(ui.saved().messages.at(-1).text, "Say hello", "stream tokens wait for the persistence debounce");
  ui.flushFrames();
  let assistants = ui.byClass("message-assistant");
  assert.equal(assistants.length, 1);
  assert.match(assistants[0].textContent, /Hello there/);
  ui.dispatch({ type: "event", kind: "done", data: { reply: "Hello, world." } });
  ui.dispatch({ type: "message", role: "assistant", text: "Hello, world." });
  assistants = ui.byClass("message-assistant");
  assert.equal(assistants.length, 1);
  assert.match(assistants[0].textContent, /Hello, world\./);
  assert.doesNotMatch(assistants[0].textContent, /Hello there/);
  assert.equal(ui.byClass("live-status")[0].textContent, "Finished");
  assert.equal(ui.saved().messages.at(-1).text, "Hello, world.", "done flushes the complete assistant answer");
});

test("assistant and durable text events replace tokens; legacy done does not duplicate them", () => {
  const ui = harness();
  ui.dispatch({ type: "event", kind: "token", data: { text: "partial answer" } });
  ui.dispatch({ type: "event", kind: "assistant", data: { text: "Final answer" } });
  ui.dispatch({ type: "event", kind: "done", data: { reply: "Final answer" } });
  assert.equal(ui.byClass("message-assistant").length, 1);
  assert.match(ui.byClass("message-assistant")[0].textContent, /Final answer/);
  assert.doesNotMatch(ui.byClass("message-assistant")[0].textContent, /partial answer/);

  ui.dispatch({ type: "event", kind: "text", data: { text: "Durable final" } });
  assert.equal(ui.byClass("message-assistant").length, 1);
  assert.match(ui.byClass("message-assistant")[0].textContent, /Durable final/);
});

test("tool calls separate pre-tool tokens from the post-tool response", () => {
  const ui = harness();
  ui.dispatch({ type: "event", kind: "token", data: { text: "I will inspect this." } });
  ui.dispatch({ type: "event", kind: "tool_call", data: { id: "t", name: "read_file", args: "main.go" } });
  ui.dispatch({ type: "event", kind: "token", data: { text: "The issue is here." } });
  ui.dispatch({ type: "event", kind: "done", data: { reply: "The issue is here." } });
  const bubbles = ui.byClass("message-assistant");
  assert.equal(bubbles.length, 2);
  assert.match(bubbles[0].textContent, /I will inspect this\./);
  assert.match(bubbles[1].textContent, /The issue is here\./);
});

test("tool results update the matching disclosure and preserve its open state", () => {
  const ui = harness();
  ui.dispatch({ type: "event", kind: "tool_call", data: { id: "tool-7", name: "read_file", args: { path: "main.go" } } });
  const row = ui.byClass("activity")[0]; row.open = true;
  ui.dispatch({ type: "event", kind: "tool_call", data: { id: "tool-8", name: "run_tests", args: {} } });
  ui.dispatch({ type: "event", kind: "tool_result", data: { id: "tool-7", name: "read_file", output: "package main" } });
  const updated = ui.byClass("activity").find((item) => item.dataset.activityKey === "tool:tool-7");
  assert.equal(updated, row);
  assert.equal(updated.open, true);
  assert.ok(updated.classList.contains("is-completed"));
  assert.match(updated.textContent, /package main/);
  assert.ok(ui.byClass("activity").some((item) => item.dataset.activityKey === "tool:tool-8" && item.classList.contains("is-running")));
});

test("tool errors are red and blocked tools remain blocked after the run resumes", () => {
  const ui = harness({ statusFailed: "Run failed", statusBlocked: "Blocked" });
  ui.dispatch({ type: "event", kind: "tool_call", data: { id: "bad", name: "write_file", args: "{}" } });
  ui.dispatch({ type: "event", kind: "tool_result", data: { id: "bad", name: "write_file", output: "ERROR: access denied" } });
  const failed = ui.byClass("activity").find((item) => item.dataset.activityKey === "tool:bad");
  assert.ok(failed.classList.contains("is-failed"));
  assert.equal(failed.classList.contains("is-completed"), false);

  ui.dispatch({ type: "event", kind: "tool_call", data: { id: "denied", name: "delete_file", args: "{}" } });
  ui.dispatch({ type: "event", kind: "approve_request", data: { name: "delete_file", args: "{}" } });
  const denied = ui.byClass("activity").find((item) => item.dataset.activityKey === "tool:denied");
  assert.ok(denied.classList.contains("is-waiting"));
  ui.dispatch({ type: "event", kind: "blocked", data: { id: "denied", reason: "User declined" } });
  ui.dispatch({ type: "event", kind: "status", data: { state: "running" } });
  assert.ok(denied.classList.contains("is-blocked"));
  assert.equal(denied.classList.contains("is-completed"), false);
  assert.equal(ui.byClass("live-status")[0].textContent, "Working…");
  ui.dispatch({ type: "event", kind: "done", data: { reply: "I left the file unchanged." } });
  assert.ok(denied.classList.contains("is-blocked"), "run completion must not turn a denied tool green");

  ui.dispatch({ type: "event", kind: "tool_call", data: { id: "flagged", name: "write_file", args: "{}" } });
  ui.dispatch({ type: "event", kind: "tool_result", data: { id: "flagged", name: "write_file", output: "No changes written", failed: true } });
  const flagged = ui.byClass("activity").find((item) => item.dataset.activityKey === "tool:flagged");
  assert.ok(flagged.classList.contains("is-failed"));
});

test("approval and question waits resolve when the run resumes", () => {
  const ui = harness({ statusWaiting: "Waiting", statusQuestion: "Question", statusWorking: "Working" });
  ui.dispatch({ type: "event", kind: "tool_call", data: { id: "write", name: "write_file", args: "{}" } });
  ui.dispatch({ type: "event", kind: "approve_request", data: { name: "write_file" } });
  ui.dispatch({ type: "event", kind: "status", data: { state: "running" } });
  const approval = ui.byClass("activity").find((item) => item.dataset.activityKey.startsWith("approval:"));
  const tool = ui.byClass("activity").find((item) => item.dataset.activityKey === "tool:write");
  assert.ok(approval.classList.contains("is-resolved"));
  assert.ok(tool.classList.contains("is-running"));

  ui.dispatch({ type: "event", kind: "question_request", data: { query: "Which file?" } });
  ui.dispatch({ type: "state", busy: true, connected: true });
  const question = ui.byClass("activity").find((item) => item.dataset.activityKey.startsWith("question:"));
  assert.ok(question.classList.contains("is-resolved"));
  assert.equal(ui.byClass("live-status")[0].textContent, "Working");
});

test("failed and cancelled delegates use terminal labels without a task title", () => {
  const ui = harness({ statusFailed: "Failed", statusCancelled: "Cancelled" });
  ui.dispatch({ type: "event", kind: "delegate", data: { id: "failed", status: "failed" } });
  ui.dispatch({ type: "event", kind: "delegate", data: { id: "cancelled", status: "cancelled" } });
  const labels = ui.byClass("activity-label").map((node) => node.textContent);
  assert.ok(labels.includes("Failed"));
  assert.ok(labels.includes("Cancelled"));
});

test("failed and cancelled terminal events stop child activity without marking success", () => {
  const failed = harness({ statusFailed: "Run failed" });
  failed.dispatch({ type: "event", kind: "delegate", data: { id: "d1", task: "Inspect files", status: "running", step: 1, max_steps: 4 } });
  failed.dispatch({ type: "event", kind: "run_failed", data: { message: "provider unavailable" } });
  assert.ok(failed.byClass("activity")[0].classList.contains("is-failed"));
  assert.equal(failed.byClass("live-status")[0].textContent, "Run failed");
  assert.equal(failed.byClass("live-status")[0].classList.contains("is-working"), false);

  const cancelled = harness({ statusCancelled: "Run cancelled" });
  cancelled.dispatch({ type: "event", kind: "tool_call", data: { id: "t", name: "edit", args: "{}" } });
  cancelled.dispatch({ type: "event", kind: "run_cancelled", data: {} });
  assert.ok(cancelled.byClass("activity")[0].classList.contains("is-cancelled"));
  assert.equal(cancelled.byClass("activity")[0].classList.contains("is-completed"), false);
  assert.equal(cancelled.byClass("live-status")[0].textContent, "Run cancelled");
});

test("a new user message closes the prior stream before the next answer", () => {
  const ui = harness();
  ui.dispatch({ type: "event", kind: "token", data: { text: "partial" } });
  ui.dispatch({ type: "message", role: "user", text: "Next question" });
  ui.dispatch({ type: "event", kind: "assistant", data: { text: "next answer" } });
  assert.equal(ui.byClass("message-assistant").length, 2);
  assert.match(ui.byClass("message-assistant")[0].textContent, /partial/);
  assert.match(ui.byClass("message-assistant")[1].textContent, /next answer/);
});

test("transcript and per-message text stay within the configured bounds", () => {
  const ui = harness();
  for (let index = 0; index < 151; index++) ui.dispatch({ type: "message", role: "user", text: `message ${index}` });
  const huge = "x".repeat(250_100);
  ui.dispatch({ type: "message", role: "user", text: huge });
  const messages = ui.saved().messages;
  assert.equal(messages.length, 150);
  assert.equal(ui.byClass("message-user").length, 150);
  assert.ok(messages.at(-1).text.length <= 250_000);
  assert.match(messages.at(-1).text, /message truncated/);

  const totalBounded = harness();
  const maxMessage = "y".repeat(250_100);
  for (let index = 0; index < 9; index++) totalBounded.dispatch({ type: "message", role: "user", text: maxMessage });
  const chars = totalBounded.saved().messages.reduce((sum, message) => sum + message.text.length, 0);
  assert.ok(chars <= 2_000_000);
});
