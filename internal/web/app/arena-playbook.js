// La guia «què guanya a què» de l'editor de l'Arena. Les dades vénen del
// servidor (games[].playbook, generades de les taules del motor): cada secció
// és una decisió amb les meves opcions (files) contra les del rival
// (columnes) i una nota de -2 a +2. Aquí: triar què fa el rival, veure les
// respostes ordenades amb el perquè, la taula sencera, i afegir la regla al .md.

const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

export const GRADE = {
  2: { cls: 'g2', sign: '++', word: 'Great' },
  1: { cls: 'g1', sign: '+', word: 'Good' },
  0: { cls: 'g0', sign: '=', word: 'Even' },
  '-1': { cls: 'gm1', sign: '−', word: 'Risky' },
  '-2': { cls: 'gm2', sign: '−−', word: 'Bad' },
};

// Les meves opcions contra la columna j, de la millor a la pitjor (a igual
// nota, el número del motor més alt; després l'ordre de la taula).
export function answers(sec, j) {
  return sec.mine.map((o, i) => ({ ...o, i, grade: sec.grid[i][j], value: sec.values ? sec.values[i][j] : null }))
    .sort((a, b) => b.grade - a.grade || (b.value ?? 0) - (a.value ?? 0) || a.i - b.i);
}

// La regla per al .md: la plantilla de la secció amb els ids.
export function ruleFor(sec, i, j) {
  return sec.rule.replace('{theirs}', sec.theirs[j].id).replace('{mine}', sec.mine[i].id);
}

export const RULES_HEAD = '## Counters (from the playbook)';

// Afegeix la regla al .md, sota «## Counters (from the playbook)» (es crea al
// final si no hi és). Si ja hi ha una regla per a la mateixa situació (el
// mateix text fins als dos punts) la canvia. Torna {md, line, kind} amb la
// línia (0-based) de la regla; kind = added | replaced | same.
export function addRule(md, rule) {
  const lines = String(md || '').replace(/\s+$/, '').split('\n');
  if (lines.length === 1 && !lines[0]) lines.length = 0;
  const key = rule.slice(0, rule.lastIndexOf(':') + 1);
  const at = lines.findIndex(l => l.trim() === rule.trim());
  if (at >= 0) return { md: lines.join('\n') + '\n', line: at, kind: 'same' };
  const old = key ? lines.findIndex(l => l.startsWith(key)) : -1;
  if (old >= 0) { lines[old] = rule; return { md: lines.join('\n') + '\n', line: old, kind: 'replaced' }; }
  let h = lines.findIndex(l => l.trim() === RULES_HEAD);
  if (h < 0) {
    if (lines.length) lines.push('');
    lines.push(RULES_HEAD, 'These rules come first when they apply: they counter what the opponent does most.');
    h = lines.length - 2;
  }
  // Al final del bloc (fins al títol següent o el final).
  let end = h + 1;
  while (end < lines.length && !/^#{1,6}\s/.test(lines[end])) end++;
  while (end > h + 1 && !lines[end - 1].trim()) end--;
  lines.splice(end, 0, rule);
  return { md: lines.join('\n') + '\n', line: end, kind: 'added' };
}

function badge(g) {
  const k = GRADE[g] || GRADE[0];
  return `<span class="pb-g ${k.cls}" title="${esc(k.word)}">${esc(k.sign)}</span>`;
}

// La guia d'un joc: pestanyes de secció, «què fa el rival» i les respostes.
// st = {sec, their} (índexs). Les accions van amb data-pb-*.
export function playbookHTML(pb, st) {
  if (!pb || !pb.sections || !pb.sections.length) return '';
  const si = Math.min(st.sec || 0, pb.sections.length - 1), sec = pb.sections[si];
  const j = Math.min(st.their || 0, sec.theirs.length - 1), th = sec.theirs[j];
  const tabs = pb.sections.length > 1 ? `<div class="pb-tabs" role="tablist">${pb.sections.map((s, k) =>
    `<button type="button" role="tab" data-pb-sec="${k}" class="${k === si ? 'on' : ''}" aria-selected="${k === si}">${esc(s.title)}</button>`).join('')}</div>` : `<h4 class="pb-title">${esc(sec.title)}</h4>`;
  const chips = sec.theirs.map((o, k) => `<button type="button" data-pb-their="${k}" class="pb-chip${k === j ? ' on' : ''}" title="${esc(o.plain)}">${esc(o.label)}</button>`).join('');
  const list = answers(sec, j).map((a, n) => `<li class="pb-ans ${(GRADE[a.grade] || GRADE[0]).cls}" style="--d:${n * 35}ms">
      ${badge(a.grade)}<div><b>${esc(a.label)}</b> <code>${esc(a.id)}</code>${a.value != null ? ` <small>${a.value}%</small>` : ''}<p>${esc(a.plain)}</p></div>
      ${a.grade > 0 ? `<button type="button" class="pb-add" data-pb-add="${a.i}" title="${esc(ruleFor(sec, a.i, j))}">＋ Rule</button>` : ''}</li>`).join('');
  return `${tabs}<p class="pb-intro">${esc(sec.intro)}</p>
    <div class="pb-q">If they play…</div><div class="pb-chips">${chips}</div>
    <p class="pb-their"><b>${esc(th.label)}</b> — ${esc(th.plain)}</p>
    <div class="pb-q">…your answers${sec.unit ? ` <small>(${esc(sec.unit)})</small>` : ''}</div><ol class="pb-list">${list}</ol>
    <button type="button" class="ar-link pb-full" data-pb-table="${si}">▦ See the whole table</button>`;
}

// La taula sencera (un diàleg): files = jo, columnes = ells; clicar una
// casella afegeix aquella regla.
export function tableHTML(sec) {
  const head = `<tr><th class="pb-corner">You ↓ · They →</th>${sec.theirs.map(o => `<th title="${esc(o.plain)}"><span>${esc(o.label)}</span></th>`).join('')}</tr>`;
  const rows = sec.mine.map((o, i) => `<tr><th title="${esc(o.plain)}">${esc(o.label)}</th>${sec.theirs.map((t, j) => {
    const g = sec.grid[i][j], k = GRADE[g] || GRADE[0];
    return `<td class="${k.cls}" data-pb-cell="${i},${j}" title="${esc(o.label + ' vs ' + t.label + ': ' + k.word)}">${sec.values ? sec.values[i][j] : k.sign}</td>`;
  }).join('')}</tr>`).join('');
  return `<h3>${esc(sec.title)}</h3><p class="ar-muted">${esc(sec.intro)}</p>
    <div class="pb-legend">${[2, 1, 0, -1, -2].map(g => `${badge(g)} ${esc(GRADE[g].word)}`).join(' ')}${sec.unit ? ` · numbers: ${esc(sec.unit)}` : ''} · click a cell to add that rule</div>
    <div class="pb-table-wrap"><table class="pb-table">${head}${rows}</table></div>`;
}

export function playbookCSS() {
  if (document.getElementById('pbCSS')) return;
  const st = document.createElement('style'); st.id = 'pbCSS';
  st.textContent = `
.pb-tabs{display:flex;gap:4px;flex-wrap:wrap;margin:2px 0 8px}
.pb-tabs button{flex:1;min-width:0;font-size:12px;padding:6px 8px;border-radius:8px;border:1px solid var(--line,#d6dae4);background:transparent;cursor:pointer;color:inherit}
.pb-tabs button.on{background:var(--accent,#4f5bd5);border-color:var(--accent,#4f5bd5);color:#fff}
.pb-title{margin:2px 0 6px}
.pb-intro{font-size:12.5px;line-height:1.45;opacity:.85;margin:0 0 10px}
.pb-q{font-size:11px;font-weight:700;letter-spacing:.06em;text-transform:uppercase;opacity:.6;margin:8px 0 5px}
.pb-chips{display:flex;flex-wrap:wrap;gap:5px}
.pb-chip{font-size:12px;padding:4px 9px;border-radius:999px;border:1px solid var(--line,#d6dae4);background:transparent;color:inherit;cursor:pointer;transition:background .15s,border-color .15s,transform .15s}
.pb-chip:hover{border-color:var(--accent,#4f5bd5);transform:translateY(-1px)}
.pb-chip.on{background:var(--accent,#4f5bd5);border-color:var(--accent,#4f5bd5);color:#fff}
.pb-their{font-size:12.5px;line-height:1.4;margin:8px 0 2px;padding:7px 9px;border-radius:8px;background:color-mix(in srgb,var(--accent,#4f5bd5) 8%,transparent)}
.pb-list{list-style:none;margin:0;padding:0;display:grid;gap:5px}
.pb-ans{display:grid;grid-template-columns:auto 1fr auto;gap:8px;align-items:start;padding:7px 8px;border-radius:9px;border:1px solid var(--line,#e3e6ee);animation:pbIn .28s ease-out both;animation-delay:var(--d,0ms)}
.pb-ans p{margin:2px 0 0;font-size:12px;line-height:1.35;opacity:.8}
.pb-ans code{font-size:11px;opacity:.65}
.pb-ans small{font-size:11px;opacity:.7;font-variant-numeric:tabular-nums}
.pb-ans.gm1,.pb-ans.gm2{opacity:.62}
.pb-g{display:inline-grid;place-items:center;min-width:26px;height:22px;padding:0 4px;border-radius:6px;font-weight:800;font-size:12px;color:#fff}
.pb-g.g2{background:#1f9d5a}.pb-g.g1{background:#5cbf7e}.pb-g.g0{background:#9aa4bd}.pb-g.gm1{background:#e8866f}.pb-g.gm2{background:#d9453c}
.pb-add{font-size:11.5px;padding:4px 8px;border-radius:7px;border:1px solid var(--accent,#4f5bd5);background:transparent;color:var(--accent,#4f5bd5);cursor:pointer;white-space:nowrap}
.pb-add:hover{background:var(--accent,#4f5bd5);color:#fff}
.pb-add.done{border-color:#1f9d5a;color:#1f9d5a;pointer-events:none}
.pb-full{margin-top:8px;font-size:12px}
@keyframes pbIn{from{opacity:0;transform:translateY(4px)}to{opacity:1;transform:none}}
.pb-dialog{position:fixed;inset:0;margin:auto;height:fit-content;max-height:90vh;max-width:min(980px,94vw);width:auto;border:1px solid var(--line-hi,transparent);background:var(--surface,#fff);color:var(--ink,#111);border-radius:14px;padding:18px 20px;box-shadow:0 24px 80px rgba(0,0,0,.35)}
.pb-dialog::backdrop{background:rgba(10,14,30,.55);backdrop-filter:blur(2px)}
.pb-dialog [data-close]{font:inherit;padding:7px 14px;border-radius:9px;border:1px solid var(--line,#d6dae4);background:transparent;color:inherit;cursor:pointer}
.pb-legend{font-size:12px;margin:6px 0 10px;display:flex;gap:6px;align-items:center;flex-wrap:wrap}
.pb-table-wrap{overflow:auto;max-height:70vh}
.pb-table{border-collapse:separate;border-spacing:3px;font-size:12px}
.pb-table th{font-weight:600;text-align:left;white-space:nowrap;padding:4px 6px}
.pb-table thead th,.pb-table tr:first-child th{text-align:center;vertical-align:bottom}
.pb-table tr:first-child th span{display:inline-block;max-width:84px;white-space:normal;line-height:1.15}
.pb-table td{min-width:46px;height:30px;text-align:center;border-radius:6px;color:#fff;font-weight:700;cursor:pointer;font-variant-numeric:tabular-nums;transition:transform .12s,box-shadow .12s}
.pb-table td:hover{transform:scale(1.08);box-shadow:0 2px 10px rgba(0,0,0,.25)}
.pb-table td.g2{background:#1f9d5a}.pb-table td.g1{background:#5cbf7e}.pb-table td.g0{background:#9aa4bd}.pb-table td.gm1{background:#e8866f}.pb-table td.gm2{background:#d9453c}
.pb-corner{font-size:11px;opacity:.6}
@media (prefers-reduced-motion:reduce){.pb-ans{animation:none}.pb-chip,.pb-table td{transition:none}}
`;
  document.head.append(st);
}
