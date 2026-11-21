// Renderitzador de markdown de Gregal: el que el model escriu i el que la UI
// mostra. El d'abans només feia codi, negreta i salts de línia, i les
// respostes sortien amb els "##" i els guions en cru. Aquest cobreix el que
// els models fan servir de debò —títols, llistes (amb nivells), cites, regles,
// taules, enllaços, èmfasi, codi— i res més: no és un CommonMark complet.
//
// Seguretat: primer s'escapa TOT el text i després es construeix l'HTML a
// sobre. Els enllaços només poden ser http(s); qualsevol altre esquema queda
// com a text. No hi ha cap camí pel qual el model pugui injectar HTML.
//
// Script clàssic (no mòdul): la pàgina el necessita abans de l'script inline.
// També s'exporta per a node (tests) via module.exports.
(function (root) {
  "use strict";

  function esc(s) {
    return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }

  // inline aplica codi, enllaços, negreta, cursiva i ratllat a una línia ja
  // escapada. El codi va primer i es protegeix perquè dins no s'hi apliqui res.
  function inline(t) {
    const codes = [];
    t = t.replace(/`([^`\n]+)`/g, (m, c) => { codes.push("<code>" + c + "</code>"); return "<<C" + (codes.length - 1) + "<<C"; });
    // «\*» és un asterisc literal (com al markdown de debò): es guarda i no fa cursiva.
    t = t.replace(/\\([\\*_`[\]()#+.!|~-])/g, (m, c) => { codes.push(c); return "<<C" + (codes.length - 1) + "<<C"; });
    // ![alt](image: consulta) — la foto que el servidor encara ha de buscar
    // (winds_card_images.go): mentre arriba la resposta, no es veu.
    t = t.replace(/!\[[^\]\n]*\]\(\s*image:[^)\n]*(?:\)|$)/g, "");
    // ![alt](http://…) — imatge; abans que l'enllaç perquè el prefix ! el distingeix.
    t = t.replace(/!\[([^\]\n]*)\]\((https?:\/\/[^\s)]+)\)/g, (m, alt, src) => '<img class="md-img" src="' + src + '" alt="' + esc(alt) + '" loading="lazy" referrerpolicy="no-referrer" onerror="this.style.display=\'none\'">');
    // ![alt](/api/winds/image?id=…) — una imatge que ha fet un vent: l'API vol
    // la clau, així que l'app la baixa i hi posa el src (attach.js).
    t = t.replace(/!\[([^\]\n]*)\]\((\/api\/winds\/image\?id=[a-f0-9]{24}\.(?:jpg|png|webp))\)/g, '<img class="md-img gregal-img" data-gregal-img="$2" alt="$1" loading="lazy">');
    // [text](/api/winds/doc?…) — un document que ha fet un vent (o un adjunt
    // editat): l'app el baixa amb la seva autenticació (attach.js).
    t = t.replace(/\[([^\]\n]+)\]\((\/api\/(?:winds\/doc|office\/download)\?[^\s)"<]+)\)/g, '<a href="$2" class="gregal-doc" data-gregal-doc="1">$1</a>');
    // [text](http://…) — només http/https; la resta es deixa com a text.
    t = t.replace(/\[([^\]\n]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
    // URL sola.
    t = t.replace(/(^|[\s(])(https?:\/\/[^\s<)]+)/g, '$1<a href="$2" target="_blank" rel="noopener noreferrer">$2</a>');
    t = t.replace(/\*\*([^*\n]+)\*\*/g, "<b>$1</b>");
    t = t.replace(/__([^_\n]+)__/g, "<b>$1</b>");
    // L'asterisc obre cursiva només si l'enganxa text a banda i banda:
    // "2 * 3 * 4" és multiplicació, no èmfasi.
    t = t.replace(/(^|[^*\w])\*(\S(?:[^*\n]*\S)?)\*(?!\w)/g, "$1<i>$2</i>");
    t = t.replace(/(^|[^_\w])_(\S(?:[^_\n]*\S)?)_(?!\w)/g, "$1<i>$2</i>");
    t = t.replace(/~~([^~\n]+)~~/g, "<s>$1</s>");
    return t.replace(/<<C(\d+)<<C/g, (m, i) => codes[+i]);
  }

  // Ressaltat de sintaxi mínim: comentaris, cadenes, números i paraules
  // clau. No és un analitzador de debò —no en cal un per llegir un bloc de
  // vint línies— però sense res el codi es veia com text pla i era el que
  // més distància marcava amb ChatGPT o Claude.
  const KEYWORDS = ("func return if else for range var const type struct interface map chan go defer package import " +
    "function let await async class extends new this null undefined true false typeof instanceof throw try catch finally switch case break continue default export " +
    "def lambda None True False elif import from as with yield pass raise while in not and or is del global nonlocal assert " +
    "public private protected static void int string bool float double select where from group by " +
    "echo fi esac then do done local readonly declare unset source exit").split(" ");
  const KW = new RegExp("\\b(" + [...new Set(KEYWORDS)].join("|") + ")\\b", "g");

  // Un sol escàner d'esquerra a dreta. Fer-ho amb passades seguides (primer
  // tots els comentaris, després totes les cadenes…) està malament: el "#"
  // de dins d'una cadena es pintava com a comentari i se'n menjava la resta
  // de la línia. Amb una alternança, guanya el que comença abans, i el que
  // obre cadena o comentari s'empassa el que hi ha a dins.
  const TOKEN = new RegExp(
    "(\\/\\/[^\\n]*|#[^\\n]*|\\/\\*[\\s\\S]*?\\*\\/)" + // 1: comentari
    "|(&quot;[^\\n]*?&quot;|&#39;[^\\n]*?&#39;|`[^`]*`)" + // 2: cadena
    "|(\\b\\d+(?:\\.\\d+)?\\b)" + // 3: número
    "|(\\b(?:" + [...new Set(KEYWORDS)].join("|") + ")\\b)", // 4: paraula clau
    "g");

  function highlight(escaped) {
    return escaped.replace(TOKEN, (m, com, str, num, kw) => {
      const cls = com ? "hl-com" : str ? "hl-str" : num ? "hl-num" : "hl-kw";
      return '<span class="' + cls + '">' + m + "</span>";
    });
  }

  // label: el text de la UI si la pàgina ja té l'idioma (window.gregalT);
  // als tests de node, el d'anglès per defecte.
  function label(key, fallback) {
    const t = root.gregalT;
    return esc((typeof t === "function" && t(key)) || fallback);
  }

  // Targeta de resultat (```gregal-result {json}``): opcions o comparacions de
  // Gregal, com a Android (ResultBlocks.kt). El codi arriba escapat: es desfà per
  // llegir el JSON i tot es torna a escapar en pintar-lo; només enllaços http(s).
  function unesc(s) {
    return String(s).replace(/&quot;/g, '"').replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&amp;/g, "&");
  }
  function safeUrl(u) {
    u = String(u || "").trim();
    return /^https?:\/\//i.test(u) ? esc(u) : "";
  }
  // Una imatge de targeta: una foto de la web (http/https) o una que ha fet un
  // vent (/api/winds/image, que l'app baixa amb la clau: attach.js).
  const WIND_IMG = /^\/api\/winds\/image\?id=[a-f0-9]{24}\.(?:jpg|png|webp)$/;
  function cardImage(u, alt) {
    u = String(u || "").trim();
    const a = esc(String(alt || "").trim());
    if (WIND_IMG.test(u)) return '<img class="gc-hero gregal-img" data-gregal-img="' + esc(u) + '" alt="' + a + '" loading="lazy">';
    const url = safeUrl(u);
    return url ? '<img class="gc-hero" src="' + url + '" alt="' + a + '" loading="lazy" referrerpolicy="no-referrer" onerror="this.remove()">' : "";
  }
  const str = (v) => esc(String(v == null ? "" : v).trim());
  function parseCard(code) {
    let o;
    try { o = JSON.parse(unesc(code)); } catch (_) { return null; }
    return o && typeof o === "object" && !Array.isArray(o) ? o : null;
  }
  // Peu de fonts compartit (punts, «N fonts», «més»): com a les opcions.
  function sourcesFoot(o, extra) {
    const seen = new Set();
    const sources = (Array.isArray(o.sources) ? o.sources : []).slice(0, 6).map((x) => {
      const url = String((x && x.url) || "");
      const host = /^https?:\/\//i.test(url) ? url.replace(/^https?:\/\//i, "").split("/")[0].replace(/^www\./, "") : "";
      const parts = host.split(".");
      const site = parts.length > 1 ? parts[parts.length - 2] : host;
      return { url, name: site || String((x && x.name) || "") };
    }).filter((x) => x.name && !seen.has(x.name) && seen.add(x.name));
    const more = o.more && typeof o.more === "object" ? o.more : null;
    if (!sources.length && !(more && more.label) && !extra) return "";
    const dots = sources.map((x, i) => {
      const url = safeUrl(x.url), name = str(x.name);
      const dot = '<i class="gr-dot gr-dot' + (i % 6) + '"></i>';
      return url ? '<a href="' + url + '" target="_blank" rel="noopener" title="' + name + '" aria-label="' + name + '">' + dot + "</a>"
        : '<span title="' + name + '">' + dot + "</span>";
    }).join("");
    const count = sources.length === 1 ? label("result.sources.one", "1 source")
      : label("result.sources.n", "{n} sources").replace("{n}", sources.length);
    let h = '<div class="gr-foot"><span class="gr-sources">' + (sources.length ? '<span class="gr-dots">' + dots + '</span><span class="gr-age">' + count + "</span>" : "") + "</span>";
    if (more && more.label) {
      const url = safeUrl(more.url);
      h += url ? '<a class="gr-more" href="' + url + '" target="_blank" rel="noopener">' + str(more.label) + " ›</a>" : '<span class="gr-more">' + str(more.label) + "</span>";
    }
    return h + (extra || "") + "</div>";
  }

  // Botons d'acció sota la targeta: el primer (o el marcat primary) és el gran
  // i fosc; el segon, un enllaç discret. En tocar-los s'envia el prompt (o el
  // text del botó) com a resposta; només es veuen on el xat ho sap fer.
  function cardActions(o) {
    const list = (Array.isArray(o.actions) ? o.actions : []).filter((a) => a && String(a.label || "").trim()).slice(0, 2);
    if (!list.length) return "";
    const k = list.findIndex((a) => a.primary);
    if (k > 0) list.unshift(list.splice(k, 1)[0]);
    const say = (a) => esc(String(a.prompt || a.label).trim().slice(0, 400));
    let h = '<div class="gc-actions">';
    h += '<button type="button" class="gc-cta" data-gregal-say="' + say(list[0]) + '"><span>' + str(list[0].label) + '</span><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14M13 6l6 6-6 6"/></svg></button>';
    if (list[0].note) h += '<small class="gc-note">' + str(list[0].note) + "</small>";
    if (list[1]) h += '<button type="button" class="gc-alt" data-gregal-say="' + say(list[1]) + '">' + str(list[1].label) + "</button>";
    return h + "</div>";
  }

  function detailsToggle() {
    return '<button type="button" class="gc-toggle" data-gc-toggle aria-expanded="false"><span class="gc-open-l">' + label("card.details", "Details") +
      '</span><span class="gc-close-l">' + label("card.less", "Less") + '</span><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 17 17 7M9 7h8v8"/></svg></button>';
  }

  const TICKET = '<svg class="gc-ico" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 8.5 15.5 4l1.4 3.2a2 2 0 0 0 2.6 1.1L21 12 9.5 16.5 8.1 13.3a2 2 0 0 0-2.6-1.1z"/></svg>';
  // Icones de línia per al trajecte (els emojis canvien d'un sistema a l'altre).
  const PATHS = {
    train: '<rect x="6" y="3" width="12" height="14" rx="3"/><path d="M6 11h12M9 21l1.5-4M15 21l-1.5-4"/><circle cx="9.5" cy="14" r=".6"/><circle cx="14.5" cy="14" r=".6"/>',
    bus: '<rect x="5" y="3" width="14" height="15" rx="2.5"/><path d="M5 10h14M8 21v-3M16 21v-3"/><circle cx="8.5" cy="14.5" r=".6"/><circle cx="15.5" cy="14.5" r=".6"/>',
    plane: '<path d="M10.5 3.5c.8-.8 2.2-.8 3 0 .4.4.5.9.5 1.4V9l6 4v2l-6-2v4l2 1.5V20l-3.5-1L9 20v-1.5l2-1.5v-4l-6 2v-2l6-4V4.9c0-.5.1-1 .5-1.4z"/>',
    car: '<path d="M5 16V12l2-5h10l2 5v4M3 16h18v2H3zM5 12h14"/><circle cx="8" cy="16" r="1.5"/><circle cx="16" cy="16" r="1.5"/>',
    walk: '<circle cx="13" cy="4.5" r="1.5"/><path d="M10 21l2-6 3 3v3M9 12l2-4 3 1 2 3M12 15l-1-4"/>',
    boat: '<path d="M4 15h16l-2 4H6zM7 15V9h10v6M12 9V4"/>',
  };
  const ALIAS = { rail: "train", metro: "train", tram: "train", coach: "bus", flight: "plane", drive: "car", bike: "walk", ferry: "boat" };
  const modeIcon = (m) => {
    m = String(m || "").toLowerCase(); m = ALIAS[m] || m;
    return PATHS[m] ? '<svg class="gc-mode" viewBox="0 0 24 24" aria-hidden="true">' + PATHS[m] + "</svg>" : "→";
  };

  // ```gregal-route: un trajecte (d'on a on, hores, servei, durada, preu) amb
  // les alternatives a sota, com la targeta de bitllet del disseny.
  function routeCard(code) {
    const o = parseCard(code);
    if (!o) return "";
    const depart = String(o.depart || "").trim(), arrive = String(o.arrive || "").trim();
    if (!depart || !arrive || !(o.from || o.from_stop) || !(o.to || o.to_stop)) return "";
    const icon = modeIcon(o.mode);
    let h = '<div class="gregal-card gregal-route">' + cardImage(o.image, String(o.title || "")) + '<div class="gc-body">';
    h += '<span class="gc-kicker">' + str(o.from || o.from_stop) + ' <span class="gc-arrow">→</span> ' + str(o.to || o.to_stop) + "</span>";
    if (o.title) h += '<strong class="gc-title">' + str(o.title) + "</strong>";
    h += '<div class="gc-leg"><div class="gc-end"><b>' + esc(depart) + "</b><small>" + str(o.from_stop || o.from) + '</small></div>' +
      '<div class="gc-line" aria-hidden="true"><i></i><span>' + icon + '</span><i></i></div>' +
      '<div class="gc-end gc-arr"><b>' + esc(arrive) + "</b><small>" + str(o.to_stop || o.to) + "</small></div></div>";
    const meta = [o.service, o.duration].filter((x) => String(x || "").trim()).map((x) => str(x)).join(" · ");
    if (meta || o.badge || o.price) {
      h += '<div class="gc-legfoot"><span>' + meta + "</span>" + (o.badge ? '<em class="gc-badge">' + str(o.badge) + "</em>" : "") +
        (o.price ? '<b class="gc-price">' + str(o.price) + "</b>" : "") + "</div>";
    }
    h += "</div>";
    const alts = (Array.isArray(o.alternatives) ? o.alternatives : []).filter((a) => a && (a.time || a.service)).slice(0, 4);
    if (alts.length) {
      h += '<ul class="gc-alts">';
      for (const a of alts) {
        const url = safeUrl(a.url);
        const inner = "<span>" + [a.time, a.service].filter((x) => String(x || "").trim()).map((x) => str(x)).join(" · ") + "</span><b>" + str(a.price) + "</b>" + (url ? "<i>›</i>" : "");
        h += "<li>" + (url ? '<a href="' + url + '" target="_blank" rel="noopener">' + inner + "</a>" : "<div>" + inner + "</div>") + "</li>";
      }
      h += "</ul>";
    }
    return h + sourcesFoot(o) + "</div>" + cardActions(o);
  }

  // La quantitat d'un ingredient per a n racions: escala el primer número
  // («400 g» → «600 g», «1,5 l», «½ ceba»); el que no en té («al gust») no canvia.
  const FRACTIONS = { "¼": 0.25, "½": 0.5, "¾": 0.75, "⅓": 1 / 3, "⅔": 2 / 3 };
  function scaleQty(qty, from, to) {
    qty = String(qty || "");
    if (!from || !to || from === to) return qty;
    // «1/2» és mitja, no un 1: la fracció escrita amb barra va primer.
    const m = /(\d+)\/(\d+)|(\d+(?:[.,]\d+)?)(?:\s*([¼½¾⅓⅔]))?|([¼½¾⅓⅔])/.exec(qty);
    if (!m) return qty;
    const frac = m[4] || m[5];
    const comma = m[3] && m[3].includes(",");
    const n = m[1] ? (+m[2] ? +m[1] / +m[2] : 0)
      : (m[3] ? parseFloat(m[3].replace(",", ".")) : 0) + (frac ? FRACTIONS[frac] : 0);
    if (!n) return qty;
    let v = n * to / from;
    v = v >= 10 ? Math.round(v) : Math.round(v * 4) / 4;
    let s = Number.isInteger(v) ? String(v) : String(+v.toFixed(2));
    if (comma) s = s.replace(".", ",");
    return qty.slice(0, m.index) + s + qty.slice(m.index + m[0].length);
  }

  // Recepta (```gregal-recipe {json}): racions que es poden canviar, ingredients
  // per marcar, passos amb el temps de cada un i un consell. attach.js fa anar
  // els botons; aquí només hi ha l'HTML (tot escapat).
  function recipeCard(code) {
    const o = parseCard(code);
    if (!o) return "";
    const title = String(o.title || "").trim();
    const ing = (Array.isArray(o.ingredients) ? o.ingredients : []).filter((x) => x && (x.name || typeof x === "string")).slice(0, 30);
    const steps = (Array.isArray(o.steps) ? o.steps : []).filter((x) => x && (x.text || typeof x === "string")).slice(0, 20);
    if (!title || (!ing.length && !steps.length)) return "";
    const serves = Math.max(0, Math.min(99, parseInt(o.servings, 10) || 0));
    let h = '<div class="gregal-card gregal-recipe"' + (serves ? ' data-serves="' + serves + '"' : "") + ">";
    h += cardImage(o.image, title);
    h += '<div class="gc-body"><strong class="gc-title">' + esc(title) + "</strong>";
    const meta = [];
    if (o.time) meta.push('<span class="gc-meta-time">' + str(o.time) + "</span>");
    if (o.level) meta.push('<span class="gc-meta-level">' + str(o.level) + "</span>");
    if (serves) meta.push('<span class="gc-meta-serves">' + label("card.serves", "{n} servings").replace("{n}", '<b class="gc-serves-n">' + serves + "</b>") + "</span>");
    if (meta.length) h += '<div class="gc-meta">' + meta.join("") + "</div>";
    if (ing.length) {
      h += '<div class="gc-sub"><span>' + label("card.ingredients", "Ingredients") + "</span>";
      if (serves) h += '<span class="gc-stepper"><button type="button" class="gc-less" aria-label="−">−</button><b class="gc-serves-n">' + serves +
        '</b><button type="button" class="gc-more" aria-label="+">+</button></span>';
      h += "</div><ul class=\"gc-checks\">";
      for (const x of ing) {
        const name = typeof x === "string" ? x : x.name, qty = typeof x === "string" ? "" : String(x.qty || "");
        h += '<li><label><input type="checkbox" class="gc-check"><span class="gc-name">' + inline(esc(String(name).trim())) + "</span>" +
          (qty ? '<span class="gc-qty" data-qty="' + esc(qty) + '">' + esc(qty) + "</span>" : "") + "</label></li>";
      }
      h += "</ul>";
    }
    if (steps.length) {
      if (ing.length) h += '<div class="gc-sub"><span>' + label("card.steps", "Steps") + "</span></div>";
      h += '<ol class="gc-steps">';
      for (const x of steps) {
        const text = typeof x === "string" ? x : x.text;
        const min = typeof x === "object" ? parseInt(x.minutes, 10) || 0 : 0;
        h += "<li><span>" + inline(esc(String(text).trim())) + "</span>" + (min ? '<em class="gc-timer">' + min + " min</em>" : "") + "</li>";
      }
      h += "</ol>";
    }
    if (o.tip) h += '<p class="gc-tip">' + inline(esc(String(o.tip).trim())) + "</p>";
    return h + "</div>" + sourcesFoot(o) + "</div>" + cardActions(o);
  }

  // Pla o seqüència (```gregal-plan {json}): una línia de temps amb hores,
  // una foto a dalt, el pressupost i les fonts. Serveix per a escapades,
  // dies, horaris o qualsevol cosa que va en ordre.
  function planCard(code) {
    const o = parseCard(code);
    if (!o) return "";
    const title = String(o.title || "").trim();
    const items = (Array.isArray(o.items) ? o.items : []).filter((x) => x && (x.title || x.text)).slice(0, 12);
    if (!title || !items.length) return "";
    let h = '<div class="gregal-card gregal-plan">' + cardImage(o.image, title) + '<div class="gc-body">';
    if (o.kicker) h += '<span class="gc-kicker">' + str(o.kicker) + "</span>";
    h += '<strong class="gc-title">' + esc(title) + "</strong>";
    if (o.subtitle) h += '<span class="gc-subtitle">' + str(o.subtitle) + "</span>";
    // Compacte, com al disseny: quatre files (hora + què) i la resta, amb els
    // detalls, en obrir «Detalls».
    const SHOW = 4;
    let hidden = items.length > SHOW;
    h += '<ol class="gc-agenda">';
    items.forEach((x, k) => {
      if (x.detail) hidden = true;
      h += "<li" + (k >= SHOW ? ' class="gc-extra"' : "") + ">" + (x.time ? '<span class="gc-pill">' + str(x.time) + "</span>" : "") + '<span class="gc-what"><b>' + inline(esc(String(x.title || x.text).trim())) + "</b>" +
        (x.detail ? '<small class="gc-detail">' + inline(esc(String(x.detail).trim())) + "</small>" : "") + "</span></li>";
    });
    h += "</ol>";
    if (o.total) {
      const t = o.total && typeof o.total === "object" ? o.total : { label: label("card.total", "Total"), value: o.total };
      h += '<div class="gc-budget">' + TICKET + "<span>" + str(t.label) + (t.label ? " · " : "") + "<b>" + str(t.value) + "</b></span></div>";
    }
    return h + "</div>" + sourcesFoot(o, hidden ? detailsToggle() : "") + "</div>" + cardActions(o);
  }

  function resultCard(code) {
    let o;
    try { o = JSON.parse(unesc(code)); } catch (_) { return ""; }
    if (!o || typeof o !== "object") return "";
    const title = String(o.title || "").trim();
    const rows = (Array.isArray(o.rows) ? o.rows : []).filter((r) => r && (r.lead || r.text)).slice(0, 5);
    if (!title || !rows.length) return "";
    let h = '<div class="gregal-result' + (o.numbered ? " gr-numbered" : "") + '">' + cardImage(o.image, title);
    h += '<div class="gr-head">' + (o.icon ? '<span class="gr-icon">' + str(String(o.icon).slice(0, 4)) + "</span>" : "") +
      '<div><strong>' + esc(title) + "</strong>" + (o.subtitle ? "<small>" + str(o.subtitle) + "</small>" : "") + "</div></div>";
    rows.forEach((r, k) => {
      const url = safeUrl(r.url);
      // Com a Android: el nom a dalt, la descripció a sota (més suau) i el preu a la dreta.
      const inner = (o.numbered ? '<span class="gr-rank">' + String(k + 1).padStart(2, "0") + "</span>" : "") + '<span class="gr-main">' + (r.lead ? '<span class="gr-lead">' + str(r.lead) + "</span>" : "") +
        (r.text ? '<span class="gr-text' + (r.lead ? "" : " gr-solo") + '">' + str(r.text) + "</span>" : "") +
        (r.tag ? "<em>" + str(r.tag) + "</em>" : "") + '</span><span class="gr-value">' + str(r.value) + "</span>";
      h += url ? '<a class="gr-row' + (r.pick ? " gr-pick" : "") + '" href="' + url + '" target="_blank" rel="noopener">' + inner + "</a>"
        : '<div class="gr-row' + (r.pick ? " gr-pick" : "") + '">' + inner + "</div>";
    });
    // Peu: punts (cada web una vegada), «N fonts» i, si el missatge té hora,
    // «· fa X min» (stamp).
    return h + sourcesFoot(o) + "</div>" + cardActions(o);
  }

  // Comparació (```gregal-compare): 2-3 coses una al costat de l'altra, cada
  // una amb la seva foto, els mateixos trets en files i el veredicte a sota.
  // {title, items:[{name, image, facts:{Tret: valor} | [{label, value}], pick}], verdict}
  function compareFacts(f) {
    if (Array.isArray(f)) return f.filter((x) => x && x.label).map((x) => [String(x.label), String(x.value == null ? "" : x.value)]);
    if (f && typeof f === "object") return Object.keys(f).map((k) => [k, String(f[k] == null ? "" : f[k])]);
    return [];
  }
  function compareCard(code) {
    const o = parseCard(code);
    if (!o) return "";
    const title = String(o.title || "").trim();
    const items = (Array.isArray(o.items) ? o.items : []).filter((x) => x && x.name).slice(0, 3);
    if (!title || items.length < 2) return "";
    const facts = items.map((x) => compareFacts(x.facts));
    const keys = [];
    facts.forEach((f) => f.forEach(([k]) => { if (!keys.includes(k)) keys.push(k); }));
    let h = '<div class="gregal-card gregal-compare"><div class="gc-body"><strong class="gc-title">' + esc(title) + "</strong>" +
      (o.subtitle ? '<span class="gc-subtitle">' + str(o.subtitle) + "</span>" : "") +
      '<div class="gx-grid" style="--gx-n:' + items.length + '"><span class="gx-corner"></span>';
    items.forEach((x) => {
      h += '<div class="gx-item' + (x.pick ? " gx-pick" : "") + '">' + cardImage(x.image, String(x.name)).replace('class="gc-hero', 'class="gx-photo') +
        "<b>" + str(x.name) + "</b>" + (x.pick ? '<em class="gx-badge">' + label("card.pick", "La tria") + "</em>" : "") + "</div>";
    });
    keys.slice(0, 8).forEach((k) => {
      h += '<span class="gx-key">' + esc(k) + "</span>";
      facts.forEach((f) => { const v = (f.find(([kk]) => kk === k) || [])[1]; h += '<span class="gx-val">' + (v ? inline(esc(v)) : "—") + "</span>"; });
    });
    h += "</div>";
    if (o.verdict) h += '<p class="gx-verdict">' + inline(esc(String(o.verdict).trim())) + "</p>";
    return h + "</div>" + sourcesFoot(o) + "</div>" + cardActions(o);
  }

  // Xifres clau (```gregal-facts): 2-6 números grans amb el que volen dir.
  // {title, facts:[{value, label, note}]}
  function factsCard(code) {
    const o = parseCard(code);
    if (!o) return "";
    const facts = (Array.isArray(o.facts) ? o.facts : []).filter((x) => x && x.value != null && String(x.value).trim() && x.label).slice(0, 6);
    if (facts.length < 2) return "";
    let h = '<div class="gregal-card gregal-facts"><div class="gc-body">' + (o.title ? '<strong class="gc-title">' + str(o.title) + "</strong>" : "") + '<div class="gf-grid">';
    facts.forEach((f) => {
      h += '<div class="gf-tile"><b>' + str(f.value) + "</b><span>" + str(f.label) + "</span>" + (f.note ? "<small>" + str(f.note) + "</small>" : "") + "</div>";
    });
    return h + "</div></div>" + sourcesFoot(o) + "</div>" + cardActions(o);
  }

  // La web que el vent ha fet (```gregal-web, winds_webcard.go): la targeta
  // amb el nom i l'adreça de la previsualització; md-preview.js hi posa
  // l'iframe aïllat. Només adreces /p/ del mateix servidor.
  function webCard(code) {
    const o = parseCard(code);
    if (!o) return "";
    const u = String(o.url || "").trim();
    if (!/^\/p\/[a-f0-9]{8,}\/[^\s"'<>]+$/.test(u)) return "";
    const title = String(o.title || o.path || "web").trim();
    return '<div class="gregal-card gregal-web" data-src="' + esc(u) + '"><div class="gw-head"><span class="gw-icon" aria-hidden="true">◧</span><div class="gw-name"><strong>' +
      esc(title) + '</strong><small>' + esc(String(o.path || "")) + '</small></div><a class="gw-open" href="' + esc(u) + '" target="_blank" rel="noopener noreferrer">' +
      label("md.open", "Obre") + ' ↗</a></div><div class="gw-frame"></div></div>';
  }

  function codeBlock(lang, code) {
    const cos = code.replace(/\n$/, "");
    const card = lang === "gregal-result" ? resultCard(cos)
      : lang === "gregal-recipe" ? recipeCard(cos)
      : lang === "gregal-plan" ? planCard(cos)
      : lang === "gregal-route" ? routeCard(cos)
      : lang === "gregal-compare" ? compareCard(cos)
      : lang === "gregal-facts" ? factsCard(cos)
      : lang === "gregal-web" ? webCard(cos) : "";
    if (card) return card;
    // ```chart (el format dels vents i d'Android) i ```gregal-plot són el mateix gràfic.
    if (lang === 'gregal-plot' || lang === 'chart') return '<pre class="codeblock lang-gregal-plot"><code>' + cos + '</code></pre>';
    return '<div class="codeblock"><div class="codehead"><span>' + (lang || label("md.code", "code")) +
      '</span><button class="copybtn" data-i18n="md.copy">' + label("md.copy", "Copy") + '</button></div><pre><code>' + highlight(cos) + "</code></pre></div>";
  }

  function table(rows) {
    const cells = (r) => r.replace(/^\s*\|/, "").replace(/\|\s*$/, "").split("|").map((c) => c.trim());
    const alignment = cells(rows[1]).map((rule) => {
      if (!rule.endsWith(":")) return "";
      return rule.startsWith(":") ? ' class="md-center"' : ' class="md-right"';
    });
    const row = (r, tag) => "<tr>" + cells(r).map((c, i) =>
      "<" + tag + (alignment[i] || "") + ">" + inline(c) + "</" + tag + ">"
    ).join("") + "</tr>";
    const body = rows.slice(2).map((r) => row(r, "td")).join("");
    return '<div class="md-table-scroll"><table class="md"><thead>' + row(rows[0], "th") +
      "</thead><tbody>" + body + "</tbody></table></div>";
  }

  const isTableSep = (l) => /^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$/.test(l);
  const listItem = (l) => /^(\s*)([-*+]|\d+[.)])\s+(.*)$/.exec(l);

  const FENCE = /```([\w+-]*)([^\n]*)(?:\n([\s\S]*?))?(```|$)/g;

  // Proposta de desar (```gregal-keep {"title","kind"}```, CardsGuide): un
  // xip sota la resposta; share-msg.js el fa desar. Trencada, no ensenya res.
  const KEEP = /```gregal-keep[ \t]*\n?([^\n`]*)\n?[ \t]*```[ \t]*/g;
  const KEEP_KINDS = ["recipe", "tip", "plan", "route", "compare", "facts", "result", "chart", "web", "text"];
  function keepChip(json) {
    let o;
    try { o = JSON.parse(json); } catch (_) { return ""; }
    const title = String((o && o.title) || "").trim();
    if (!title) return "";
    const kind = KEEP_KINDS.includes(o.kind) ? o.kind : "auto";
    return '<div class="gc-keep-row"><button type="button" class="gc-keep" data-keep-kind="' + kind + '" data-keep-title="' + esc(title) + '">' +
      label("card.keep", "Save «{t}»?").replace("{t}", esc(title)) + "</button></div>";
  }

  function render(text) {
    if (text == null) return "";
    let keep = "";
    text = String(text).replace(KEEP, (m, json) => { keep = keepChip(json.trim()) || keep; return ""; });
    // Blocs de codi abans de res: el seu contingut no es toca (ni s'hi
    // busquen llistes ni títols).
    const blocks = [];
    let t = esc(text).replace(FENCE, (m, l, rest, c) => {
      c = c || "";
      // Els models escriuen sovint la targeta a la mateixa línia de l'obertura
      // («```gregal-recipe {…}»): el JSON és el contingut, no el nom del bloc.
      if (/^(?:gregal-[a-z]+|chart)$/.test(l) && rest.trim().startsWith("{")) c = rest.trim() + "\n" + c;
      blocks.push(codeBlock(l, c));
      return "<<B" + (blocks.length - 1) + "<<B";
    });

    const lines = t.split("\n");
    const out = [];
    let para = [];
    const flushPara = () => {
      if (!para.length) return;
      const html = inline(para.join("<br>"));
      // Un paràgraf només d'imatges és una galeria: una foto gran, o dues o
      // tres en fila (abans sortien l'una sota l'altra, cadascuna a mitja amplada).
      const imgs = html.match(/<img [^>]*>/g) || [];
      if (imgs.length && !html.replace(/<img [^>]*>|<br>|\s/g, "")) out.push('<div class="md-gallery md-gallery-' + Math.min(imgs.length, 3) + '">' + imgs.join("") + "</div>");
      else out.push("<p>" + html + "</p>");
      para = [];
    };

    for (let i = 0; i < lines.length; i++) {
      const l = lines[i];
      if (/^<<B\d+<<B$/.test(l.trim())) { flushPara(); out.push(l.trim()); continue; }
      if (!l.trim()) { flushPara(); continue; }
      let m;
      if ((m = /^(#{1,6})\s+(.+?)\s*#*\s*$/.exec(l))) { flushPara(); const n = m[1].length; out.push("<h" + n + ">" + inline(m[2]) + "</h" + n + ">"); continue; }
      if (/^\s*([-*_])(\s*\1){2,}\s*$/.test(l)) { flushPara(); out.push("<hr>"); continue; }
      if (/^\s*&gt;/.test(l)) {
        flushPara();
        const q = [];
        while (i < lines.length && /^\s*&gt;/.test(lines[i])) { q.push(lines[i].replace(/^\s*&gt;\s?/, "")); i++; }
        i--;
        out.push("<blockquote>" + render(q.join("\n").replace(/&gt;/g, ">")) + "</blockquote>");
        continue;
      }
      if (/^\s*\|/.test(l) && i + 1 < lines.length && isTableSep(lines[i + 1])) {
        flushPara();
        const rows = [];
        while (i < lines.length && /^\s*\|/.test(lines[i])) { rows.push(lines[i]); i++; }
        i--;
        out.push(table(rows));
        continue;
      }
      if (listItem(l)) {
        flushPara();
        // Llista amb nivells per sagnat (2 espais per nivell, o el que hi hagi).
        const stack = []; // {ol: bool, indent: n}
        // Una llista numerada separada per línies en blanc («1.», buit, «2.»)
        // arriba a trossos: start manté la numeració (abans tot eren «1.»).
        const open = (ol, indent, n) => { out.push(ol ? (n > 1 ? '<ol start="' + n + '" style="counter-reset:md-step ' + (n - 1) + '">' : "<ol>") : "<ul>"); stack.push({ ol, indent }); };
        const close = () => { const s = stack.pop(); out.push(s.ol ? "</ol>" : "</ul>"); };
        while (i < lines.length && (m = listItem(lines[i]))) {
          const indent = m[1].length, ol = /\d/.test(m[2]);
          while (stack.length && indent < stack[stack.length - 1].indent) close();
          const num = parseInt(m[2], 10);
          if (!stack.length || indent > stack[stack.length - 1].indent) open(ol, indent, num);
          else if (stack[stack.length - 1].ol !== ol) { close(); open(ol, indent, num); }
          let item = m[3];
          const task = /^\[( |x|X)\]\s+(.*)$/.exec(item);
          // Checklist: es pot marcar (attach.js en recorda l'estat en aquest aparell).
          if (task) { out.push('<li class="md-task"><label><input type="checkbox" class="md-check"' + (task[1] !== " " ? " checked" : "") + "><span>" + inline(task[2]) + "</span></label></li>"); i++; continue; }
          out.push("<li>" + inline(item) + "</li>");
          i++;
        }
        i--;
        while (stack.length) close();
        continue;
      }
      para.push(l);
    }
    flushPara();
    return out.join("\n").replace(/<<B(\d+)<<B/g, (m, i) => blocks[+i]) + keep;
  }

  // stamp afegeix «· fa X min» al peu de les targetes de resultat d'un
  // missatge ja pintat (ts en segons). Els preus caduquen: saber quan es van
  // mirar és part del resultat.
  function stamp(node, ts) {
    if (!node || !node.querySelectorAll || !ts) return;
    const secs = Math.max(0, Math.round(Date.now() / 1000 - ts));
    const t = root.gregalT, label = (key, fb) => (typeof t === "function" && t(key)) || fb;
    const ago = secs < 60 ? label("result.ago.now", "just now")
      : secs < 3600 ? label("result.ago.min", "{n} min ago").replace("{n}", Math.round(secs / 60))
      : secs < 86400 ? label("result.ago.h", "{n} h ago").replace("{n}", Math.round(secs / 3600))
      : label("result.ago.d", "{n} d ago").replace("{n}", Math.round(secs / 86400));
    node.querySelectorAll(".gregal-result .gr-age").forEach((el) => {
      if (!el.dataset.base) el.dataset.base = el.textContent;
      el.textContent = el.dataset.base + " · " + ago;
    });
  }

  // liveCut diu fins on un text que encara arriba ja no canviarà de forma:
  // l'última línia en blanc fora d'un bloc de codi. render tanca tots els
  // blocs en una línia en blanc, així que pintar les dues meitats per separat
  // dona el mateix que pintar-ho tot.
  function liveCut(text) {
    // Un bloc de codi sense tancar arriba fins al final del text: tot ell
    // queda a la part oberta.
    const fences = [];
    for (const m of text.matchAll(FENCE)) fences.push([m.index, m.index + m[0].length]);
    let cut = 0;
    for (const m of text.matchAll(/\n[ \t\r]*\n/g)) {
      if (!fences.some(([a, b]) => m.index >= a && m.index < b)) cut = m.index + m[0].length;
    }
    return cut;
  }

  // paintLive pinta a body el text d'una resposta que encara arriba. Refer-ho
  // tot a cada tros feia saltar les respostes llargues i desfeia la selecció:
  // els blocs acabats s'escriuen un sol cop i només es refà el que creix, que
  // viu en un contenidor propi (.md-live) perquè el que altres scripts
  // afegeixen al costat d'un bloc acabat (la targeta d'un gràfic) no es toqui.
  const livePainted = new WeakMap(); // body → { cut, tail }
  function paintLive(body, text) {
    let s = livePainted.get(body);
    if (!s) {
      body.innerHTML = "";
      body.insertAdjacentHTML("beforeend", '<div class="md-live"></div>');
      s = { cut: 0, tail: body.lastChild };
      livePainted.set(body, s);
    }
    const cut = liveCut(text);
    if (cut > s.cut) {
      s.tail.insertAdjacentHTML("beforebegin", render(text.slice(s.cut, cut)));
      s.cut = cut;
    }
    s.tail.innerHTML = render(text.slice(cut));
  }

  const api = { render, inline, esc, stamp, scaleQty, liveCut, paintLive };
  root.gregalMD = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : this);
