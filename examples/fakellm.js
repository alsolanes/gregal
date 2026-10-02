// Proveïdor OpenAI-compatible fals per provar Gregal d'extrem a extrem sense
// gastar tokens. Escenari determinista segons l'últim missatge d'usuari:
//  - conté "eina"  → primer torn: tool_call glob; segon torn (hi ha tool result): text final amb markdown
//  - conté "llarg" → text llarg en molts tokens (per veure streaming)
//  - conté "error" → HTTP 500
//  - altrament    → resposta curta en streaming
const http = require("http");
const PORT = process.env.PORT || 5999;

// Amb stream:false (el mode -p del TUI, el revisor…) la resposta és un sol
// JSON: es plega la llista de deltes en un missatge sencer.
let wantStream = true;
function sse(res, chunks) {
  if (!wantStream) {
    const msg = { role: "assistant", content: "" };
    let finish = "stop";
    for (const c of chunks) {
      const d = c.choices[0].delta || {};
      if (d.content) msg.content += d.content;
      if (d.reasoning_content) msg.reasoning_content = (msg.reasoning_content || "") + d.reasoning_content;
      for (const tc of d.tool_calls || []) {
        msg.tool_calls = msg.tool_calls || [];
        const cur = msg.tool_calls[tc.index] || (msg.tool_calls[tc.index] = { id: "", type: "function", function: { name: "", arguments: "" } });
        if (tc.id) cur.id = tc.id;
        if (tc.function?.name) cur.function.name += tc.function.name;
        if (tc.function?.arguments) cur.function.arguments += tc.function.arguments;
      }
      if (c.choices[0].finish_reason) finish = c.choices[0].finish_reason;
    }
    res.writeHead(200, { "Content-Type": "application/json" });
    return res.end(JSON.stringify({ choices: [{ message: msg, finish_reason: finish }] }));
  }
  res.writeHead(200, { "Content-Type": "text/event-stream", "Cache-Control": "no-cache" });
  let i = 0;
  const tick = () => {
    if (i < chunks.length) {
      res.write("data: " + JSON.stringify(chunks[i++]) + "\n\n");
      setTimeout(tick, 40);
    } else {
      res.write("data: [DONE]\n\n");
      res.end();
    }
  };
  tick();
}
const delta = (d, finish) => ({ choices: [{ delta: d, ...(finish ? { finish_reason: finish } : {}) }] });
const words = (s) => s.split(/(?<= )/);

http.createServer((req, res) => {
  if (req.url.endsWith("/models")) {
    res.writeHead(200, { "Content-Type": "application/json" });
    return res.end(JSON.stringify({ data: [{ id: "fals-1" }] }));
  }
  let body = "";
  req.on("data", (c) => (body += c));
  req.on("end", () => {
    let msgs = [];
    try { const req = JSON.parse(body); msgs = req.messages || []; wantStream = !!req.stream; } catch {}
    const lastUser = [...msgs].reverse().find((m) => m.role === "user");
    const text = lastUser ? (typeof lastUser.content === "string" ? lastUser.content : JSON.stringify(lastUser.content)) : "";
    const hasToolResult = msgs.some((m) => m.role === "tool");
    console.log(new Date().toISOString(), "→", text.slice(0, 60).replace(/\n/g, " "), hasToolResult ? "(amb tool result)" : "");

    // Mode objectiu: el system prompt demana un bloc ```goal. Primer torn
    // pregunta; si l'usuari ja concreta ("sí"/"endavant"), torna el bloc.
    const sys = msgs.find((m) => m.role === "system");
    if (sys && /```goal/.test(String(sys.content))) {
      if (!/s[íi]|endavant|concret/i.test(text)) {
        return sse(res, [...words("Abans de res: vols cobrir només el TUI o també l'escriptori? I tens algun límit de temps?").map((w) => delta({ content: w })), delta({}, "stop")]);
      }
      const goal = "Perfecte, ho deixo concretat.\n\n```goal\ntasca: Afegir scroll amb la roda al TUI\ncontext: l'historial no es pot moure amb la roda del ratolí\ncriteris:\n- la roda mou el viewport\n- PageUp/PageDown funcionen\npassos:\n- tocar refresh()\n- afegir bindings\nriscos:\n- perdre l'autoscroll\n```\n";
      return sse(res, [...words(goal).map((w) => delta({ content: w })), delta({}, "stop")]);
    }
    if (/error/i.test(text)) {
      res.writeHead(500, { "Content-Type": "application/json" });
      return res.end(JSON.stringify({ error: { message: "Internal server error (fals)" } }));
    }
    // "aprova": una ordre de bash que NO és de la llista segura → l'agent ha
    // de demanar permís (flux d'aprovació a l'escriptori i al TUI).
    if (/aprova/i.test(text) && !hasToolResult) {
      return sse(res, [
        delta({ content: "Creo el directori de prova." }),
        delta({ tool_calls: [{ index: 0, id: "b1", type: "function", function: { name: "bash", arguments: '{"command":"mkdir -p /tmp/gregal-prova && echo creat"}' } }] }),
        delta({}, "tool_calls"),
      ]);
    }
    if (/aprova/i.test(text) && hasToolResult) {
      const last = [...msgs].reverse().find((m) => m.role === "tool");
      const out = (last && typeof last.content === "string" ? last.content : "").slice(0, 120);
      return sse(res, [...words("Fet. La sortida de l'ordre ha estat: `" + out.replace(/\n/g, " ") + "`").map((w) => delta({ content: w })), delta({}, "stop")]);
    }
    if (/eina/i.test(text) && !hasToolResult) {
      return sse(res, [
        delta({ content: "Deixa'm " }), delta({ content: "mirar què hi ha." }),
        delta({ tool_calls: [{ index: 0, id: "c1", type: "function", function: { name: "glob", arguments: "" } }] }),
        delta({ tool_calls: [{ index: 0, function: { arguments: '{"pattern":"*.md"' } }] }),
        delta({ tool_calls: [{ index: 0, function: { arguments: ',"dir":"."}' } }] }),
        delta({}, "tool_calls"),
      ]);
    }
    if (/eina/i.test(text) && hasToolResult) {
      const md = "## Què he trobat\n\nHe llistat els fitxers markdown de l'arrel:\n\n- `README.md` — la porta d'entrada\n- `CHANGELOG.md` — l'historial\n\n```go\nfmt.Println(\"tot bé\")\n```\n\n**Conclusió:** el projecte està ben documentat.";
      return sse(res, [...words(md).map((w) => delta({ content: w })), delta({}, "stop")]);
    }
    if (/llarg/i.test(text)) {
      const long = Array.from({ length: 60 }, (_, i) => `Frase ${i + 1} del text llarg. `).join("");
      return sse(res, [...words(long).map((w) => delta({ content: w })), delta({}, "stop")]);
    }
    sse(res, [delta({ reasoning_content: "penso una mica" }), delta({ content: "Hola! " }), delta({ content: "Sóc el model fals. " }), delta({ content: "Tot funciona." }, "stop")]);
  });
}).listen(PORT, "127.0.0.1", () => console.log("fakellm escoltant a http://127.0.0.1:" + PORT + "/v1"));
