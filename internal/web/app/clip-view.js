// Pàgina pública d'un missatge compartit (/share/<id>/, winds_share.go): pinta
// el text amb md.js com a l'app, amb les fotos copiades al costat i els
// gràfics. Script clàssic: la pàgina viu en un origen opac (sandbox) i els
// mòduls hi necessitarien CORS.
(function () {
  "use strict";
  const dataEl = document.getElementById("gclip-data");
  const body = document.getElementById("gclip-body");
  if (!dataEl || !body || !window.gregalMD) return;
  let data;
  try { data = JSON.parse(dataEl.textContent); } catch (_) { return; }
  if (window.matchMedia && window.matchMedia("(prefers-color-scheme: light)").matches) document.documentElement.dataset.theme = "light";
  // Les etiquetes de les targetes (md.js les demana a gregalT): en català,
  // tret que el navegador sigui d'una altra llengua.
  const ca = /^(ca|es)\b/i.test(navigator.language || "ca");
  const L = ca ? { "result.sources.one": "1 font", "result.sources.n": "{n} fonts", "card.details": "Detalls", "card.less": "Menys",
    "card.serves": "{n} racions", "card.ingredients": "Ingredients", "card.steps": "Passos", "card.total": "Total" } : {};
  if (!ca) { document.documentElement.lang = "en"; const f = document.querySelector(".gclip-foot"); if (f) f.textContent = "Shared from Gregal · read-only"; }
  window.gregalT = (k) => L[k] || "";
  // Les fotos dels vents van a img/ al costat de la pàgina; md.js només
  // pinta imatges amb adreça sencera.
  const text = String(data.text || "")
    .replace(/!\[[^\]\n]*\]\(img\/missing\)/g, "")
    .replace(/\]\((img\/[a-f0-9]{24}\.(?:jpg|png|webp))\)/g, (m, rel) => "](" + new URL(rel, location.href).href + ")");
  body.innerHTML = window.gregalMD.render(text);
  // Aquí no hi ha xat: els botons que responen al vent no hi fan res.
  body.querySelectorAll("[data-gregal-say]").forEach((b) => b.remove());
  document.addEventListener("click", (e) => {
    const t = e.target.closest && e.target.closest("[data-gc-toggle]");
    const card = t && t.closest(".gregal-card");
    if (card) t.setAttribute("aria-expanded", String(card.classList.toggle("gc-open")));
    const step = e.target.closest && e.target.closest(".gregal-recipe .gc-stepper button");
    if (step) {
      const rc = step.closest(".gregal-recipe"), base = +rc.dataset.serves || 1;
      const now = Math.max(1, Math.min(99, (+rc.dataset.now || base) + (step.classList.contains("gc-more") ? 1 : -1)));
      rc.dataset.now = now;
      rc.querySelectorAll(".gc-serves-n").forEach((n) => { n.textContent = now; });
      rc.querySelectorAll(".gc-qty").forEach((q) => { q.textContent = window.gregalMD.scaleQty ? window.gregalMD.scaleQty(q.dataset.qty, base, now) : q.dataset.qty; });
    }
  });
  // Gràfics: Chart.js de l'app, només si n'hi ha.
  const plots = [...body.querySelectorAll(".lang-gregal-plot")];
  if (!plots.length || !window.gregalPlotsCore) return;
  const s = document.createElement("script");
  s.src = "/app/vendor/chart.umd.min.js";
  s.onload = () => {
    const css = getComputedStyle(document.documentElement);
    const palette = [1, 2, 3, 4, 5, 6].map((i) => css.getPropertyValue("--chart-" + i).trim()).filter(Boolean);
    for (const pre of plots) {
      let cfg;
      try { cfg = window.gregalPlotsCore.config(JSON.parse(pre.textContent), css.getPropertyValue("--text").trim() || "#888", palette); } catch (_) { continue; }
      const box = document.createElement("div"); box.className = "gclip-chart";
      const canvas = document.createElement("canvas"); box.append(canvas); pre.replaceWith(box);
      new window.Chart(canvas, cfg);
    }
  };
  document.head.append(s);
})();
