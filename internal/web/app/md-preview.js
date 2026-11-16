// Vista prèvia dels blocs ```html i ```svg (com els artefactes de Claude o el
// canvas de Grok): el bloc de codi passa a ser una targeta amb «Vista» (la
// pàgina funcionant), «Codi», «Copia» i «Amplia». La pàgina corre en un
// iframe sandbox sense origen (allow-scripts, sense allow-same-origin) i amb
// una CSP que no deixa sortir a la xarxa: no pot tocar l'app ni la sessió.
//
// Script clàssic: s'enganxa sol a qualsevol .codeblock que aparegui (xat dels
// vents, Codi, la pàgina pública), i window.gregalMdPreview exposa la lògica
// pura per als tests.
(function (root) {
  "use strict";
  const CSP = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-src 'none'";
  const MAX = 200000;

  // previewKind: "html" | "svg" | "" segons el llenguatge i el contingut.
  function previewKind(lang, code) {
    const l = String(lang || "").trim().toLowerCase();
    const c = String(code || "").trim();
    if (!c || c.length > MAX) return "";
    if (l === "svg" || (l === "xml" && /^<svg\b/i.test(c))) return /<svg\b[\s\S]*<\/svg>\s*$/i.test(c) ? "svg" : "";
    if (l === "html" || l === "htm") return /<[a-z][\s\S]*>/i.test(c) ? "html" : "";
    return "";
  }

  // srcdoc: el document aïllat que va dins l'iframe.
  function srcdoc(kind, code) {
    const meta = '<meta http-equiv="Content-Security-Policy" content="' + CSP + '">';
    const c = String(code).trim();
    if (kind === "svg") {
      return "<!doctype html><html><head>" + meta + '<meta charset="utf-8"><style>html,body{margin:0;height:100%;display:grid;place-items:center;background:#fff}svg{max-width:100%;max-height:100%}</style></head><body>' + c + "</body></html>";
    }
    // Un document sencer: la CSP va just després de <head> (o al davant si no n'hi ha).
    if (/<head[^>]*>/i.test(c)) return c.replace(/<head([^>]*)>/i, "<head$1>" + meta);
    if (/<html[^>]*>/i.test(c)) return c.replace(/<html([^>]*)>/i, "<html$1><head>" + meta + "</head>");
    return "<!doctype html><html><head>" + meta + '<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body>' + c + "</body></html>";
  }

  const api = { previewKind, srcdoc };
  root.gregalMdPreview = api;
  if (typeof module !== "undefined" && module.exports) { module.exports = api; return; }
  if (!root.document) return;

  const T = (k, ca, en) => {
    try { const v = root.gregalT && root.gregalT(k); if (v && v !== k) return v; } catch (_) {}
    return (root.document.documentElement.lang || "ca").startsWith("en") ? en : ca;
  };
  function frame(kind, code, tall) {
    const f = root.document.createElement("iframe");
    f.className = "gc-prev-frame";
    f.setAttribute("sandbox", "allow-scripts allow-modals");
    f.setAttribute("referrerpolicy", "no-referrer");
    f.setAttribute("title", T("md.preview", "Vista prèvia", "Preview"));
    f.style.height = tall ? "100%" : (kind === "svg" ? "280px" : "380px");
    f.srcdoc = srcdoc(kind, code);
    return f;
  }
  function full(kind, code) {
    const d = root.document.createElement("dialog");
    d.className = "gc-prev-full";
    const bar = root.document.createElement("div"); bar.className = "gc-prev-fullbar";
    const close = root.document.createElement("button"); close.type = "button"; close.textContent = "✕";
    close.setAttribute("aria-label", T("md.close", "Tanca", "Close"));
    close.addEventListener("click", () => d.close());
    bar.append(close);
    d.append(bar, frame(kind, code, true));
    d.addEventListener("close", () => d.remove());
    root.document.body.append(d);
    d.showModal();
  }
  function mount(box) {
    if (box.dataset.gcPrev) return;
    const pre = box.querySelector("pre"), head = box.querySelector(".codehead");
    const label = head && head.querySelector("span");
    if (!pre || !head || !label) return;
    const code = pre.textContent;
    const kind = previewKind(label.textContent, code);
    box.dataset.gcPrev = kind || "no";
    if (!kind) return;
    box.classList.add("gc-preview");
    const view = root.document.createElement("div"); view.className = "gc-prev-view";
    view.append(frame(kind, code));
    const tab = (k, text) => {
      const b = root.document.createElement("button"); b.type = "button"; b.className = "gc-prev-tab"; b.dataset.tab = k; b.textContent = text;
      b.addEventListener("click", () => {
        const showView = k === "view";
        view.hidden = !showView; pre.hidden = showView;
        box.querySelectorAll(".gc-prev-tab").forEach((x) => x.classList.toggle("on", x === b));
      });
      return b;
    };
    const vt = tab("view", T("md.preview", "Vista", "Preview")), ct = tab("code", T("md.codeTab", "Codi", "Code"));
    vt.classList.add("on");
    const grow = root.document.createElement("button"); grow.type = "button"; grow.className = "gc-prev-grow"; grow.textContent = "⤢";
    grow.title = T("md.expand", "Amplia", "Expand"); grow.setAttribute("aria-label", grow.title);
    grow.addEventListener("click", () => full(kind, code));
    label.after(vt, ct);
    head.append(grow);
    pre.hidden = true;
    pre.before(view);
  }
  // La web d'un vent (targeta gregal-web): la pàgina de debò, en un iframe
  // sense origen; «Obre» la dona sencera en una pestanya.
  function mountWeb(card) {
    if (card.dataset.gcPrev) return;
    card.dataset.gcPrev = "web";
    const src = card.dataset.src || "";
    const slot = card.querySelector(".gw-frame");
    if (!/^\/p\//.test(src) || !slot) return;
    const f = root.document.createElement("iframe");
    f.className = "gc-prev-frame";
    f.setAttribute("sandbox", "allow-scripts allow-forms allow-modals allow-popups");
    f.setAttribute("referrerpolicy", "no-referrer");
    f.setAttribute("loading", "lazy");
    f.setAttribute("title", card.querySelector("strong")?.textContent || "web");
    f.style.height = "420px";
    f.src = src;
    slot.append(f);
  }
  function scan(el) {
    (el || root.document).querySelectorAll(".gregal-web:not([data-gc-prev])").forEach((c) => {
      if (!c.closest(".streaming, [data-streaming='true']")) mountWeb(c);
    });
    (el || root.document).querySelectorAll(".codeblock:not([data-gc-prev])").forEach((b) => {
      // Mentre el missatge s'escriu, el bloc encara pot canviar: s'espera al final.
      if (b.closest(".streaming, [data-streaming='true']")) return;
      mount(b);
    });
  }
  let queued = false;
  const kick = () => { if (queued) return; queued = true; setTimeout(() => { queued = false; scan(); }, 250); };
  const start = () => { scan(); new MutationObserver(kick).observe(root.document.body, { childList: true, subtree: true }); };
  if (root.document.body) start(); else root.document.addEventListener("DOMContentLoaded", start);
})(typeof globalThis !== "undefined" ? globalThis : this);
