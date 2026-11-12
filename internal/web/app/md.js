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
    // ![alt](http://…) — imatge; abans que l'enllaç perquè el prefix ! el distingeix.
    t = t.replace(/!\[([^\]\n]*)\]\((https?:\/\/[^\s)]+)\)/g, (m, alt, src) => '<img class="md-img" src="' + src + '" alt="' + esc(alt) + '" loading="lazy" referrerpolicy="no-referrer" onerror="this.style.display=\'none\'">');
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
  function resultCard(code) {
    let o;
    try { o = JSON.parse(unesc(code)); } catch (_) { return ""; }
    if (!o || typeof o !== "object") return "";
    const title = String(o.title || "").trim();
    const rows = (Array.isArray(o.rows) ? o.rows : []).filter((r) => r && (r.lead || r.text)).slice(0, 5);
    if (!title || !rows.length) return "";
    const str = (v) => esc(String(v == null ? "" : v).trim());
    let h = '<div class="gregal-result">';
    h += '<div class="gr-head">' + (o.icon ? '<span class="gr-icon">' + str(String(o.icon).slice(0, 4)) + "</span>" : "") +
      '<div><strong>' + esc(title) + "</strong>" + (o.subtitle ? "<small>" + str(o.subtitle) + "</small>" : "") + "</div></div>";
    for (const r of rows) {
      const url = safeUrl(r.url);
      const inner = '<span class="gr-lead">' + str(r.lead) + '</span><span class="gr-text">' + str(r.text) +
        (r.tag ? '<em>' + str(r.tag) + "</em>" : "") + '</span><span class="gr-value">' + str(r.value) + "</span>";
      h += url ? '<a class="gr-row' + (r.pick ? " gr-pick" : "") + '" href="' + url + '" target="_blank" rel="noopener">' + inner + "</a>"
        : '<div class="gr-row' + (r.pick ? " gr-pick" : "") + '">' + inner + "</div>";
    }
    const sources = (Array.isArray(o.sources) ? o.sources : []).slice(0, 6);
    const more = o.more && typeof o.more === "object" ? o.more : null;
    if (sources.length || (more && more.label)) {
      h += '<div class="gr-foot"><span class="gr-sources">' + sources.map((x, i) => {
        const url = safeUrl(x && x.url), name = str((x && (x.name || String(x.url || "").replace(/^https?:\/\//, "").split("/")[0])) || "");
        return '<i class="gr-dot gr-dot' + (i % 6) + '"></i>' + (url ? '<a href="' + url + '" target="_blank" rel="noopener">' + name + "</a>" : name);
      }).join(" ") + "</span>";
      if (more && more.label) {
        const url = safeUrl(more.url);
        h += url ? '<a class="gr-more" href="' + url + '" target="_blank" rel="noopener">' + str(more.label) + " ›</a>" : '<span class="gr-more">' + str(more.label) + "</span>";
      }
      h += "</div>";
    }
    return h + "</div>";
  }

  function codeBlock(lang, code) {
    const cos = code.replace(/\n$/, "");
    if (lang === "gregal-result") {
      const card = resultCard(cos);
      if (card) return card;
    }
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

  function render(text) {
    if (text == null) return "";
    // Blocs de codi abans de res: el seu contingut no es toca (ni s'hi
    // busquen llistes ni títols).
    const blocks = [];
    let t = esc(text).replace(/```([\w+-]*)[^\n]*\n([\s\S]*?)(```|$)/g, (m, l, c) => {
      blocks.push(codeBlock(l, c));
      return "<<B" + (blocks.length - 1) + "<<B";
    });

    const lines = t.split("\n");
    const out = [];
    let para = [];
    const flushPara = () => {
      if (para.length) { out.push("<p>" + inline(para.join("<br>")) + "</p>"); para = []; }
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
        const open = (ol, indent, n) => { out.push(ol ? (n > 1 ? '<ol start="' + n + '">' : "<ol>") : "<ul>"); stack.push({ ol, indent }); };
        const close = () => { const s = stack.pop(); out.push(s.ol ? "</ol>" : "</ul>"); };
        while (i < lines.length && (m = listItem(lines[i]))) {
          const indent = m[1].length, ol = /\d/.test(m[2]);
          while (stack.length && indent < stack[stack.length - 1].indent) close();
          const num = parseInt(m[2], 10);
          if (!stack.length || indent > stack[stack.length - 1].indent) open(ol, indent, num);
          else if (stack[stack.length - 1].ol !== ol) { close(); open(ol, indent, num); }
          let item = m[3];
          const task = /^\[( |x|X)\]\s+(.*)$/.exec(item);
          if (task) item = '<input type="checkbox" disabled' + (task[1] !== " " ? " checked" : "") + "> " + task[2];
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
    return out.join("\n").replace(/<<B(\d+)<<B/g, (m, i) => blocks[+i]);
  }

  const api = { render, inline, esc };
  root.gregalMD = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : this);
