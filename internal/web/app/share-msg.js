// Compartir un missatge d'un vent: text per a WhatsApp o Telegram (les
// targetes passen a text llegible), enviar-lo a una altra persona de l'app o
// fer-ne un enllaç públic de només lectura (winds_share.go). També desar-lo
// com a nota amb tipus (notes.go): les targetes hi queden tal com eren.
//
// Script clàssic: window.gregalShare = { button, saveButton, plainText }.
// plainText i saveReply també s'exporten per a node (tests); plainText és la
// mateixa regla que ShareText.kt.
(function (root) {
  "use strict";

  const CARD = /```(gregal-[a-z]+|chart)[ \t]*\n([\s\S]*?)```/g;
  const W = {
    ca: { ing: "Ingredients", steps: "Passos", total: "Total", serves: "{n} racions" },
    en: { ing: "Ingredients", steps: "Steps", total: "Total", serves: "{n} servings" },
  };
  const s = (v) => String(v == null ? "" : v).trim();
  const join = (xs, sep) => xs.map(s).filter(Boolean).join(sep);

  // cardText: una targeta en línies de text; "" si no s'entén (es deixa el bloc).
  function cardText(lang, kind, json) {
    let o;
    try { o = JSON.parse(json); } catch (_) { return null; }
    if (!o || typeof o !== "object") return null;
    const w = W[lang] || W.ca, out = [];
    const head = (t) => { if (s(t)) out.push("*" + s(t) + "*"); };
    if (kind === "gregal-result") {
      head(o.title); if (s(o.subtitle)) out.push(s(o.subtitle));
      for (const r of Array.isArray(o.rows) ? o.rows : []) {
        if (!r) continue;
        const what = join([r.lead, r.text], " — ");
        out.push("• " + what + (s(r.value) ? ": " + s(r.value) : "") + (r.pick ? " ✓" : "") + (/^https?:\/\//.test(s(r.url)) ? "\n  " + s(r.url) : ""));
      }
    } else if (kind === "gregal-plan") {
      head(o.title); if (s(o.subtitle)) out.push(s(o.subtitle));
      for (const x of Array.isArray(o.items) ? o.items : []) {
        if (!x) continue;
        out.push((s(x.time) ? s(x.time) + " · " : "• ") + join([x.title || x.text, x.detail], " — "));
      }
      if (o.total) {
        const t = typeof o.total === "object" ? o.total : { label: w.total, value: o.total };
        out.push(join([t.label, t.value], ": "));
      }
    } else if (kind === "gregal-route") {
      head(join([o.from || o.from_stop, o.to || o.to_stop], " → "));
      if (s(o.title)) out.push(s(o.title));
      out.push(join([join([o.depart, o.arrive], " → "), o.service, o.duration, o.price], " · "));
      for (const a of Array.isArray(o.alternatives) ? o.alternatives : []) if (a) out.push("  · " + join([a.time, a.service, a.price], " · "));
    } else if (kind === "gregal-recipe") {
      head(o.title);
      const n = parseInt(o.servings, 10);
      const meta = join([o.time, o.level, n ? w.serves.replace("{n}", n) : ""], " · ");
      if (meta) out.push(meta);
      const ing = Array.isArray(o.ingredients) ? o.ingredients : [];
      if (ing.length) {
        out.push("", w.ing + ":");
        for (const x of ing) out.push("• " + (typeof x === "string" ? s(x) : join([x.qty, x.name], " ")));
      }
      const steps = Array.isArray(o.steps) ? o.steps : [];
      if (steps.length) {
        out.push("", w.steps + ":");
        steps.forEach((x, i) => out.push((i + 1) + ". " + (typeof x === "string" ? s(x) : s(x.text) + (parseInt(x.minutes, 10) ? " (" + parseInt(x.minutes, 10) + " min)" : ""))));
      }
      if (s(o.tip)) out.push("", s(o.tip));
    } else if (kind === "chart" || kind === "gregal-plot") {
      head(o.title);
      const labels = Array.isArray(o.labels) ? o.labels : [];
      const series = Array.isArray(o.series) ? o.series : [];
      labels.slice(0, 24).forEach((l, i) => out.push("• " + s(l) + ": " + series.map((x) => (series.length > 1 ? s(x.label) + " " : "") + s((x.values || [])[i])).join(", ")));
    } else return null;
    if (!out.length) return null;
    const src = (Array.isArray(o.sources) ? o.sources : []).map((x) => s(x && x.url)).filter((u) => /^https?:\/\//.test(u)).slice(0, 3);
    if (src.length) out.push("", ...src);
    return out.join("\n");
  }

  // plainText: el missatge tal com s'ha d'enganxar a un xat de fora.
  function plainText(md, lang) {
    let t = String(md || "").replace(/\r/g, "");
    t = t.replace(CARD, (m, kind, json) => { const c = cardText(lang, kind, json.trim()); return c == null ? m : c + "\n"; });
    // Fotos: la d'internet com a enllaç; la d'un vent no es pot obrir de fora.
    t = t.replace(/!\[[^\]\n]*\]\((https?:\/\/[^\s)]+)\)/g, "$1");
    t = t.replace(/!\[[^\]\n]*\]\([^)\n]*\)/g, "");
    // Documents dels vents: només el títol (l'enllaç demana la clau).
    t = t.replace(/\[([^\]\n]+)\]\(\/api\/[^)\s]*\)/g, "$1");
    t = t.replace(/\[([^\]\n]+)\]\((https?:\/\/[^\s)]+)\)/g, (m, txt, u) => (txt === u ? u : txt + " (" + u + ")"));
    t = t.replace(/^#{1,6}\s+(.+)$/gm, "*$1*");
    t = t.replace(/\*\*([^*\n]+)\*\*/g, "*$1*").replace(/__([^_\n]+)__/g, "*$1*");
    t = t.replace(/^[ \t]*[-*][ \t]+/gm, "• ");
    t = t.replace(/`([^`\n]+)`/g, "$1");
    return t.replace(/\n{3,}/g, "\n\n").trim();
  }

  // --- Interfície (només al navegador) ------------------------------------

  const tx = (ca, en) => (document.documentElement.lang === "en" ? en : ca);
  function el(tag, cls, text) { const n = document.createElement(tag); if (cls) n.className = cls; if (text != null) n.textContent = text; return n; }
  async function api(url, body) {
    const call = root.gregal && root.gregal.api ? root.gregal.api : root.api || fetch;
    const r = await call(url, body === undefined ? {} : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    if (!r.ok) throw new Error((await r.text()).trim() || r.status);
    return r.json();
  }
  async function copy(text) {
    try { await navigator.clipboard.writeText(text); return true; } catch (_) { return false; }
  }
  // saveReply: desa la resposta com a nota; el servidor en treu el tipus i el títol de la primera targeta.
  // El xat es repinta (i torna a crear els botons) a cada missatge i la història
  // arriba nova del servidor, així que el que ja s'ha desat es recorda pel text.
  const saved = new Map();
  const savedNote = (md) => saved.get(md);
  async function saveReply(md) {
    if (saved.has(md)) return { id: saved.get(md) };
    const note = await api("/api/notes", { action: "save", body: md, kind: "auto" });
    saved.set(md, note.id);
    return note;
  }

  const ICON = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 15V3m0 0L7.5 7.5M12 3l4.5 4.5M5 13v5a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-5"/></svg>';
  let open = null;
  function close() { if (open) { open.remove(); open = null; } document.removeEventListener("pointerdown", outside, true); }
  function outside(e) { if (open && !open.contains(e.target) && !e.target.closest(".gshare-btn")) close(); }

  // button(getText, meta): el botó «Comparteix» d'un missatge. meta.from = nom del vent.
  function button(getText, meta) {
    meta = meta || {};
    const b = el("button", "gshare-btn");
    b.type = "button";
    b.innerHTML = ICON;
    b.title = tx("Comparteix", "Share");
    b.setAttribute("aria-label", b.title);
    b.addEventListener("click", (e) => { e.stopPropagation(); open && open.anchor === b ? close() : menu(b, getText(), meta); });
    return b;
  }

  const SAVE_ICON = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 3h12v18l-6-4-6 4z"/></svg>';
  // saveButton(getText, openNote): «Desa» d'un missatge; un cop desat diu «Desat ✓» i openNote(id) obre la nota.
  function saveButton(getText, openNote) {
    const b = el("button", "gshare-btn gsave-btn");
    b.type = "button";
    const done = () => {
      b.classList.add("gsave-text");
      b.textContent = tx("Desat ✓", "Saved ✓");
      b.title = tx("Obre la nota desada", "Open the saved note");
      b.setAttribute("aria-label", b.title);
    };
    b.innerHTML = SAVE_ICON;
    b.title = tx("Desa", "Save");
    b.setAttribute("aria-label", b.title);
    if (savedNote(getText())) done();
    b.addEventListener("click", async (e) => {
      e.stopPropagation();
      const id = savedNote(getText());
      if (id) { openNote(id); return; }
      b.disabled = true;
      try {
        await saveReply(getText());
        done();
      } catch (err) {
        b.classList.add("gsave-text");
        b.textContent = tx("No s'ha desat", "Not saved");
        b.title = tx("No s'ha pogut desar: ", "Couldn't save: ") + (err.message || err);
      } finally { b.disabled = false; }
    });
    return b;
  }

  function menu(anchor, md, meta) {
    close();
    const lang = tx("ca", "en");
    const plain = plainText(md, lang);
    const box = el("div", "gshare-menu");
    box.anchor = anchor;
    box.setAttribute("role", "menu");
    const status = el("p", "gshare-status");
    const say = (msg, bad) => { status.textContent = msg; status.classList.toggle("bad", !!bad); };
    const item = (label, sub, fn) => {
      const it = el("button", "gshare-item"); it.type = "button"; it.setAttribute("role", "menuitem");
      it.append(el("span", "", label)); if (sub) it.append(el("small", "", sub));
      it.addEventListener("click", async () => { it.disabled = true; try { await fn(); } catch (err) { say(String(err.message || err), true); } finally { it.disabled = false; } });
      box.append(it);
      return it;
    };
    item(tx("Copia el text", "Copy text"), tx("Les targetes, com a text", "Cards as plain text"), async () => {
      say(await copy(plain) ? tx("Copiat.", "Copied.") : tx("No s'ha pogut copiar.", "Couldn't copy."));
    });
    item("WhatsApp", "", () => { root.open("https://wa.me/?text=" + encodeURIComponent(plain), "_blank", "noopener"); close(); });
    item("Telegram", "", () => { root.open("https://t.me/share/url?url=" + encodeURIComponent(plain), "_blank", "noopener"); close(); });
    const person = item(tx("Envia a algú de Gregal…", "Send to someone on Gregal…"), tx("Li arriba a Desat", "It lands in their Saved"), () => people(box, md, meta, say));
    person.classList.add("gshare-sep");
    item(tx("Crea un enllaç públic", "Create a public link"), tx("Només lectura, sense compte", "Read-only, no account needed"), async () => {
      say(tx("Creant l'enllaç…", "Creating the link…"));
      const r = await api("/api/winds/clip", { text: md, from: meta.from || "" });
      const ok = await copy(r.link);
      say((ok ? tx("Enllaç copiat: ", "Link copied: ") : "") + r.link + (r.private ? "\n" + tx("Ull: és una adreça de casa; de fora no s'obrirà.", "Heads up: it's a home address; it won't open from outside.") : ""), r.private);
    });
    box.append(status);
    document.body.append(box);
    const a = anchor.getBoundingClientRect(), w = box.offsetWidth, h = box.offsetHeight;
    box.style.left = Math.max(8, Math.min(innerWidth - w - 8, a.right - w)) + "px";
    box.style.top = (a.bottom + h + 8 > innerHeight ? Math.max(8, a.top - h - 6) : a.bottom + 6) + "px";
    open = box;
    setTimeout(() => document.addEventListener("pointerdown", outside, true));
  }

  async function people(box, md, meta, say) {
    let list;
    try { list = (await api("/api/winds/shared/people")).people || []; }
    catch (_) { say(tx("Per enviar a algú, el servidor ha de tenir comptes.", "To send to someone, the server needs accounts."), true); return; }
    if (!list.length) { say(tx("Encara no hi ha ningú més en aquest servidor.", "There's nobody else on this server yet."), true); return; }
    const pick = el("div", "gshare-people");
    const chosen = new Set();
    for (const n of list) {
      const c = el("button", "gshare-person", n); c.type = "button";
      c.addEventListener("click", () => { chosen.has(n) ? chosen.delete(n) : chosen.add(n); c.classList.toggle("on", chosen.has(n)); send.disabled = !chosen.size; });
      pick.append(c);
    }
    const send = el("button", "gshare-send", tx("Envia", "Send")); send.type = "button"; send.disabled = true;
    send.addEventListener("click", async () => {
      send.disabled = true;
      try {
        const r = await api("/api/winds/send", { to: [...chosen], text: md, from: meta.from || "" });
        say(tx("Enviat a ", "Sent to ") + (r.sent || []).join(", ") + ".");
        pick.remove();
      } catch (err) { say(String(err.message || err), true); send.disabled = false; }
    });
    pick.append(send);
    box.querySelector(".gshare-status").before(pick);
  }

  const api_ = { plainText, cardText, button, saveButton, saveReply, savedNote, close };
  root.gregalShare = api_;
  if (typeof module !== "undefined" && module.exports) module.exports = api_;
})(typeof globalThis !== "undefined" ? globalThis : this);
