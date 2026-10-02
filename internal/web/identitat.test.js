// docs/identitat.md és la font de veritat de la paleta i el :root
// d'index.html n'és un derivat. Aquest test és el que impedeix que la web
// torni a anar pel seu compte: fins a la 1.1.1 duia un blau genèric que no
// era de cap lloc, i ningú se n'havia adonat perquè res ho comprovava.
//
// S'executa amb `node --test internal/web/identitat.test.js` (sense npm ni
// dependències) i la CI el crida al costat dels tests de Go.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const doc = fs.readFileSync(path.join(__dirname, "..", "..", "docs", "identitat.md"), "utf8");
const html = fs.readFileSync(path.join(__dirname, "index.html"), "utf8");

// paleta: nom → hex, de la taula "## Paleta".
function paleta() {
  const out = {};
  let dins = false;
  for (const l of doc.split("\n")) {
    if (l.startsWith("## ")) { dins = l.startsWith("## Paleta"); continue; }
    if (!dins) continue;
    const m = /^\| `([A-Za-z]+)` \| `(#[0-9A-Fa-f]{6})` \|/.exec(l);
    if (m) out[m[1]] = m[2].toUpperCase();
  }
  assert.ok(Object.keys(out).length >= 10, "la taula de la paleta ha canviat de format");
  return out;
}

// root: token CSS → valor, del bloc :root d'index.html.
function root() {
  const m = /:root\s*\{([^}]*)\}/.exec(html);
  assert.ok(m, "index.html sense :root");
  const out = {};
  for (const [, k, v] of m[1].matchAll(/(--[a-z-]+)\s*:\s*([^;]+);/g)) out[k] = v.trim();
  return out;
}

// La correspondència de docs/identitat.md, secció "Web i escriptori".
const MAPA = {
  "--bg": "fons", "--sidebar": "abisme", "--panel": "abisme", "--surface": "night",
  "--surface-hi": "superficieAlta", "--ink": "ink", "--text": "text", "--muted": "slate",
  "--faint": "faint", "--accent": "escuma", "--accent-hi": "escumaClara", "--amber": "arena",
  "--green": "green", "--red": "red", "--onada": "onada", "--lila": "lila", "--lila-fons": "lilaFons",
};

test("els tokens del :root valen el que diu docs/identitat.md", () => {
  const p = paleta();
  const r = root();
  for (const [token, nom] of Object.entries(MAPA)) {
    assert.ok(nom in p, `${nom} no és a la taula del document`);
    assert.ok(token in r, `${token} no és al :root`);
    assert.equal(r[token].toUpperCase(), p[nom], `${token} hauria de ser ${nom} (${p[nom]}), és ${r[token]}`);
  }
});

test("les línies fines són escuma translúcida, no un blau d'una altra app", () => {
  const r = root();
  assert.equal(r["--line"], "rgba(159,224,196,.10)");
  assert.equal(r["--line-hi"], "rgba(159,224,196,.20)");
});

test("cap color escrit a mà fora dels blocs de tokens", () => {
  // Els colors només poden viure als blocs :root (el fosc de base i el clar
  // de data-theme); a qualsevol altre lloc han de ser var(--…).
  const sense = html.replace(/:root(\[[^\]]*\])?\s*\{[^}]*\}/g, "");
  const solts = [...sense.matchAll(/#[0-9A-Fa-f]{6}\b/g)].map((m) => m[0]);
  assert.deepEqual(solts, [], `hex solts a index.html: ${solts.join(", ")} — passa'ls a var(--…)`);
});

// Els rgba() escrits a mà són el forat pel qual s'havien colat un blau de la
// paleta antiga i un munt de vels blancs: en tema clar, blanc sobre blanc no
// es veu. Només s'admeten les ombres negres, que van bé en tots dos temes.
test("cap rgba de color fora dels blocs de tokens", () => {
  const sense = html.replace(/:root(\[[^\]]*\])?\s*\{[^}]*\}/g, "");
  const dolents = [...sense.matchAll(/rgba\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*,[^)]*\)/g)]
    .filter((m) => !(m[1] === "0" && m[2] === "0" && m[3] === "0")) // ombres: OK
    .map((m) => m[0]);
  assert.deepEqual(dolents, [],
    `rgba de color fora del :root: ${dolents.join(", ")} — fes-ne un token (--hover, --accent-soft…)`);
});

// Cada tema amb data-theme és el mateix joc de noms amb altres valors:
// cap token del fosc pot quedar-se sense equivalent, o en aquell tema es
// veuria un color del fosc enmig (text fosc sobre fons fosc, posem).
test("cada tema defineix tots els tokens de color del fosc", () => {
  const blocs = [...html.matchAll(/:root\[data-theme="([a-z]+)"\]\s*\{([^}]*)\}/g)];
  assert.ok(blocs.length >= 4, "falten blocs :root[data-theme] (light, gpt, claude, opencode)");
  const fosc = root();
  // --mono, les mides i l'escala tipogràfica no són colors: no han de
  // canviar amb el tema.
  const noColor = new Set(["--mono", "--fs", "--gap", "--pad",
    "--lh", "--lh-titol", "--llegible"]);
  for (const [, nom, cos] of blocs) {
    const te = new Set([...cos.matchAll(/(--[a-z-]+)\s*:/g)].map((m) => m[1]));
    const falten = Object.keys(fosc).filter((k) => !noColor.has(k) && !te.has(k));
    assert.deepEqual(falten, [], `el tema ${nom} no defineix: ${falten.join(", ")}`);
  }
});

test("els valors del tema clar surten de docs/identitat.md", () => {
  const doc2 = doc;
  const taula = {};
  let dins = false;
  for (const l of doc2.split("\n")) {
    if (l.startsWith("## ")) { dins = l.startsWith("## Paleta clara"); continue; }
    if (!dins) continue;
    const m = /^\| `([A-Za-z]+)` \| `(#[0-9A-Fa-f]{6})` \|/.exec(l);
    if (m) taula[m[1]] = m[2].toUpperCase();
  }
  assert.ok(Object.keys(taula).length >= 10, "la taula de la paleta clara ha canviat de format");
  const blocLight = /:root\[data-theme="light"\]\s*\{([^}]*)\}/.exec(html)[1];
  const light = {};
  for (const [, k, v] of blocLight.matchAll(/(--[a-z-]+)\s*:\s*([^;]+);/g)) light[k] = v.trim();
  const MAPA_CLAR = {
    "--bg": "fonsClar", "--sidebar": "abismeClar", "--panel": "abismeClar", "--surface": "nightClar",
    "--surface-hi": "superficieAltaClar", "--ink": "inkClar", "--text": "textClar", "--muted": "slateClar",
    "--faint": "faintClar", "--accent": "escumaClar", "--accent-hi": "escumaClaraClar", "--amber": "arenaClar",
    "--green": "greenClar", "--red": "redClar", "--onada": "onadaClar", "--lila": "lilaClar", "--lila-fons": "lilaFonsClar",
  };
  for (const [token, nom] of Object.entries(MAPA_CLAR)) {
    assert.ok(nom in taula, `${nom} no és a la taula clara del document`);
    assert.equal((light[token] || "").toUpperCase(), taula[nom], `${token} hauria de ser ${nom} (${taula[nom]})`);
  }
});

test("la marca Ventet usa l'asset de la capçalera, sense degradat", () => {
  assert.ok(!/linear-gradient\(145deg,#446cff/.test(html), "el degradat blau-lila ha de desaparèixer");
  assert.match(html, /class="brand-mark"><img src="\/app\/icon\.png"/, "la capçalera ha de carregar la icona Ventet");
});

// --hover:var(--hover) és circular: CSS el descarta i el token queda sense
// valor, així que tots els hovers del tema fosc eren transparents i no es
// veia res. No fa cap error a la consola; només deixa de funcionar.
test("cap token es defineix a partir de si mateix", () => {
  const blocs = [...html.matchAll(/:root(?:\[[^\]]*\])?\s*\{([^}]*)\}/g)].map((m) => m[1]);
  for (const bloc of blocs) {
    for (const [, k, v] of bloc.matchAll(/(--[a-z-]+)\s*:\s*([^;]+);/g)) {
      const propis = [...v.matchAll(/var\(\s*(--[a-z-]+)/g)].map((r) => r[1]);
      assert.ok(!propis.includes(k), `${k} es defineix amb si mateix: ${v.trim()}`);
    }
  }
});

// Una traducció a mitges no fa cap error: només et surt mitja pantalla en
// l'idioma que no toca. Els dos diccionaris han de tenir les mateixes claus.
test("els diccionaris d'idioma tenen exactament les mateixes claus", async () => {
  const { CLAUS } = await import("./app/i18n.js");
  const ca = Object.keys(CLAUS.ca).sort();
  const en = Object.keys(CLAUS.en).sort();
  assert.deepEqual(en.filter((k) => !CLAUS.ca[k]), [], "claus que només són a l'anglès");
  assert.deepEqual(ca.filter((k) => !CLAUS.en[k]), [], "claus que només són al català");
  // I cap valor buit: una clau amb "" es veu com un forat a la pantalla.
  for (const l of ["ca", "en"]) {
    for (const [k, v] of Object.entries(CLAUS[l])) {
      assert.ok(String(v).trim() !== "", `${l}: ${k} és buida`);
    }
  }
});

// Cada data-i18n de la pàgina ha d'existir al diccionari: si no, a l'hora
// de traduir el text es queda com estava i sembla que funcioni en català.
test("cada data-i18n de la pàgina té traducció", async () => {
  const { CLAUS } = await import("./app/i18n.js");
  const claus = [...html.matchAll(/data-i18n(?:-title|-ph|-aria|-html)?="([^"]+)"/g)]
    .map((m) => m[1])
    // Algunes targetes es munten des de JS i el data-i18n hi surt com a
    // tros de plantilla ("' + s.clau + '.t"). Una clau de debò no porta
    // espais ni cometes: comprovar la plantilla seria comprovar el no-res.
    .filter((k) => /^[a-z0-9.]+$/i.test(k));
  assert.ok(claus.length > 10, `n'hi hauria d'haver unes quantes, n'he trobat ${claus.length}`);
  const orfes = [...new Set(claus)].filter((k) => !(k in CLAUS.ca));
  assert.deepEqual(orfes, [], "claus a l'HTML que no són al diccionari");
});

// Els data-i18n de l'HTML ja es comprovaven, però els missatges que
// l'app diu mentre treballa —la cua, el torn aturat, els passos esgotats—
// es demanen amb gregalT('clau') des del codi, i això no ho mirava ningú.
// Traduir-los va deixar catorze claus escrites a l'HTML i mai desades al
// diccionari: gregalT retornava el nom de la clau i a la pantalla sortia
// "ui.passos". Els dos diccionaris hi eren igual de buits, així que la
// prova de paritat passava tan tranquil·la.
test("cada gregalT('clau') del codi té traducció", async () => {
  const { CLAUS } = await import("./app/i18n.js");
  const claus = [...new Set([...html.matchAll(/gregalT\('([^']+)'\)/g)].map((m) => m[1]))];
  assert.ok(claus.length > 20, `n'hi hauria d'haver moltes, n'he trobat ${claus.length}`);
  const orfes = claus.filter((k) => !(k in CLAUS.ca));
  assert.deepEqual(orfes, [], "claus demanades des del codi que no són al diccionari");
});

// Les claus mortes són el mateix forat vist de l'altra banda: hi havia
// 'graphs.lead', 'graphs.auto' i 'graphs.empty' al diccionari, en català i
// en anglès, i l'HTML no en feia servir cap — el text seguia escrit a mà a
// la pàgina i no canviava mai de llengua. Escrites i mai connectades.
test('cap clau del diccionari es queda sense fer servir', async () => {
  const { CLAUS } = await import('./app/i18n.js');
  const moduls = fs.readdirSync(path.join(__dirname, 'app'))
    .filter((f) => f.endsWith('.js'))
    .map((f) => fs.readFileSync(path.join(__dirname, 'app', f), 'utf8'))
    .join("\n");
  const tot = html + "\n" + moduls;
  const mortes = Object.keys(CLAUS.ca).filter((k) => !tot.includes("'" + k + "'") && !tot.includes('"' + k + '"'));
  assert.deepEqual(mortes, [], 'claus al diccionari que no fa servir ningú');
});

// La finestra de l'escriptori pinta el fons i els botons natius abans que
// la pàgina carregui, i aquests colors viuen a desktop/main.js. Si es
// desenganxen dels tokens, torna a passar el que va passar: botons foscos
// sobre una barra clara. Aquí es lliguen a la mateixa paleta.
test("els colors d'arrencada de l'escriptori surten dels tokens", () => {
  const main = fs.readFileSync(path.join(__dirname, "..", "..", "desktop", "main.js"), "utf8");
  const m = /const TEMA = \{([\s\S]*?)\};/.exec(main);
  assert.ok(m, "desktop/main.js ha de tenir la taula TEMA");
  // Regex literal, no new RegExp: amb el constructor els escapats es
  // perden pel camí i la comprovació passa a no comprovar res.
  const files = {};
  for (const [, nom, bg, barra, glif] of m[1].matchAll(
    /(fosc|clar):\s*\{\s*bg:\s*"(#[0-9A-Fa-f]{6})",\s*barra:\s*"(#[0-9A-Fa-f]{6})",\s*glif:\s*"(#[0-9A-Fa-f]{6})"\s*\}/g)) {
    files[nom] = { bg: bg.toUpperCase(), barra: barra.toUpperCase(), glif: glif.toUpperCase() };
  }
  const llegeix = linia => {
    assert.ok(files[linia], `falta la fila ${linia} a TEMA`);
    return files[linia];
  };
  const tok = bloc => {
    const out = {};
    for (const [, k, v] of bloc.matchAll(/(--[a-z-]+)\s*:\s*([^;]+);/g)) out[k] = v.trim().toUpperCase();
    return out;
  };
  const fosc = tok(/:root\s*\{([^}]*)\}/.exec(html)[1]);
  const clar = tok(/:root\[data-theme="light"\]\s*\{([^}]*)\}/.exec(html)[1]);

  const f = llegeix("fosc");
  assert.equal(f.bg, fosc["--bg"], "fons fosc");
  assert.equal(f.barra, fosc["--sidebar"], "barra fosca");
  assert.equal(f.glif, fosc["--ink"], "glifs foscos");

  const c = llegeix("clar");
  assert.equal(c.bg, clar["--bg"], "fons clar");
  assert.equal(c.barra, clar["--sidebar"], "barra clara");
  assert.equal(c.glif, clar["--ink"], "glifs clars");
});

// Contrast de la paleta, mesurat i clavat aquí perquè no pugui empitjorar
// sense que ningú se n'assabenti. Mesurat amb el DOM real de la pàgina en
// tots dos temes: 25-26 elements de text queden per sota del 4.5 que demana
// la WCAG per a text petit, i tots són --faint, que és el token que existeix
// per apagar. Això no és un error: és una tria de disseny que viu a
// docs/identitat.md, i canviar-la és decisió de qui mana la identitat.
//
// El que sí que és un error és que --muted o --text baixin fins on és
// --faint, o que --faint caigui encara més. Els números són els d'avui.
test("el contrast de la paleta no empitjora", () => {
  const lum = (hex) => {
    const n = parseInt(hex.slice(1), 16);
    const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); };
    return 0.2126 * f((n >> 16) & 255) + 0.7152 * f((n >> 8) & 255) + 0.0722 * f(n & 255);
  };
  const ratio = (a, b) => {
    const la = lum(a), lb = lum(b);
    return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
  };
  const bloc = (re) => {
    const m = re.exec(html);
    assert.ok(m, "no trobo el bloc de tokens");
    const o = {};
    for (const [, k, v] of m[1].matchAll(/(--[a-z-]+)\s*:\s*([^;]+);/g)) o[k] = v.trim();
    return o;
  };
  // [tema, tokens, mínims mesurats avui]
  const casos = [
    ["fosc", bloc(/:root\s*\{([^}]*)\}/), { "--faint": 2.2, "--muted": 6.5, "--text": 13.5 }],
    ["clar", bloc(/:root\[data-theme="light"\]\s*\{([^}]*)\}/), { "--faint": 3.2, "--muted": 5.9, "--text": 13.5 }],
  ];
  for (const [tema, t, minims] of casos) {
    for (const [fg, min] of Object.entries(minims)) {
      for (const bg of ["--bg", "--surface"]) {
        const r = ratio(t[fg], t[bg]);
        assert.ok(r >= min, `tema ${tema}: ${fg} sobre ${bg} ha baixat a ${r.toFixed(2)} (mínim ${min})`);
      }
    }
  }
});
