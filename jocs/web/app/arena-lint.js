// A free, instant check of the .md while you type: no match, no model call.
// It looks for the same five prompting principles the coach scores after a
// match (clarity, specificity, coverage, adaptivity, consistency) with plain
// patterns, so you fix the obvious problems before spending a practice match.
// Pure functions (tested without a DOM in arena-lint.test.cjs).

const VAGUE = /\b(smart(ly)?|good|well|properly|appropriate(ly)?|as needed|if needed|when needed|try to|maybe|sometimes|somewhat|a lot|a bit|usually|generally|be careful|be aggressive|be smart|use judg(e)?ment|balanced|mix it up)\b/i;
const RULE = /(->|→|=>|\bif\b|\bwhen\b|\bon\b[^.]{0,24}:|\bagainst\b|\bunder\b|\bover\b|\bafter\b|\bbefore\b)/i;
const DEFAULT = /\b(otherwise|by default|default|else|fallback|fall back|in any other|if nothing)\b/i;
const OPP = /\b(they|their|them|opponent|opponents|rival|enemy|enemies|habit|habits|tendency|tendencies|repeat(s|ed)?|adapt|counter|keeps?|keying)\b/i;
const PRIORITY = /(\bpriorit(y|ies)\b|\bfirst rule\b|\bin order\b|\btop to bottom\b|^\s*\d+[.)]\s)/im;

const norm = l => l.toLowerCase().replace(/^[\s>*+-]*(\d+[.)])?\s*/, '').replace(/\s+/g, ' ').trim();
const ruleLines = md => String(md || '').split('\n').filter(l => l.trim() && !/^\s*(#|```|---|===)/.test(l));
const short = (l, n = 70) => { const t = l.replace(/^[\s>*+-]*(\d+[.)])?\s*/, '').trim(); return t.length > n ? t.slice(0, n - 1) + '…' : t; };

export const PRINCIPLES = [
  ['clarity', 'Clarity'], ['specificity', 'Specificity'], ['coverage', 'Coverage'], ['adaptivity', 'Adaptivity'], ['consistency', 'Consistency'],
];

// → { score 0-100, checks: [{ id, principle, ok, label, tip }], tip }
export function lintMD(md, max = 12000) {
  const text = String(md || '');
  const lines = ruleLines(text);
  const checks = [];
  const add = (id, principle, ok, label, tip) => checks.push({ id, principle, ok: !!ok, label, tip: ok ? '' : tip });

  const vague = lines.filter(l => VAGUE.test(l));
  add('vague', 'clarity', vague.length === 0, 'No vague wording',
    vague.length ? `“${short(vague[0])}” leaves the model guessing. Say exactly when and what, with an option name or a number.` : '');

  const withNumbers = lines.filter(l => /\d/.test(l.replace(/^\s*\d+[.)]\s/, '')));
  add('numbers', 'specificity', withNumbers.length >= 3, 'Numbers and thresholds',
    'Add thresholds the game can check: a down and distance, a clock time, a percentage, an amount of money.');

  const rules = lines.filter(l => RULE.test(l));
  add('rules', 'coverage', rules.length >= 6, 'At least 6 if/then rules',
    `Only ${rules.length} rule${rules.length === 1 ? '' : 's'} found. Write one for each situation you expect: “when X → do Y”.`);

  add('default', 'coverage', DEFAULT.test(text), 'A default for everything else',
    'Add an “Otherwise: …” line, so the model has an answer when no rule matches.');

  add('opponent', 'adaptivity', OPP.test(text), 'Reacts to the opponent',
    'Tell the model what to do when the rival repeats something: “if they key on X, switch to Y”.');

  add('order', 'consistency', PRIORITY.test(text), 'Priorities in order',
    'Number your rules or say “the first rule that applies wins”, so two rules never fight.');

  const seen = new Map();
  for (const l of lines) { const k = norm(l); if (k.length > 12) seen.set(k, (seen.get(k) || 0) + 1); }
  const dup = [...seen].find(([, n]) => n > 1);
  add('dups', 'consistency', !dup, 'No repeated rules',
    dup ? `“${short(dup[0])}” appears ${dup[1]} times. Keep one.` : '');

  const size = text.length;
  add('size', 'clarity', size >= 300 && size <= max, 'A sensible length',
    size < 300 ? 'Very short: the model has little to go on. Aim for 600 characters or more.' : `Over the ${max}-character limit.`);

  const ok = checks.filter(c => c.ok).length;
  const bad = checks.find(c => !c.ok);
  return { score: Math.round(100 * ok / checks.length), ok, total: checks.length, checks, tip: bad ? bad.tip : 'All clear. Play a practice match and let the coach find the subtle problems.' };
}

const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

export function lintHTML(r) {
  if (!r) return '';
  const chips = PRINCIPLES.map(([id, label]) => {
    const bad = r.checks.some(c => c.principle === id && !c.ok);
    return `<span class="lt-chip ${bad ? 'bad' : 'ok'}"><i aria-hidden="true">${bad ? '!' : '✓'}</i>${esc(label)}<span class="lt-sr">${bad ? ': to fix' : ': OK'}</span></span>`;
  }).join('');
  // Every failing check with its fix, in the page (a hover tooltip is out of
  // reach by keyboard and touch).
  const fails = r.checks.filter(c => !c.ok);
  return `<div class="lt-head"><b>Prompt check</b><span class="lt-score">${r.ok}/${r.total}</span>${chips}</div>${fails.length
    ? `<ul class="lt-fails">${fails.map(c => `<li><b>${esc(c.label)}.</b> ${esc(c.tip)}</li>`).join('')}</ul>`
    : `<p class="lt-tip">${esc(r.tip)}</p>`}`;
}

export function lintCSS() {
  if (document.getElementById('ltCSS')) return;
  const st = document.createElement('style'); st.id = 'ltCSS';
  st.textContent = `
.ar-lint{margin:8px 0 2px;padding:10px 12px;border-radius:12px;border:1px solid var(--line,rgba(127,127,127,.25));background:rgba(127,127,127,.06);font-size:13px}
.ar-lint[hidden]{display:none}
.lt-head{display:flex;flex-wrap:wrap;gap:6px;align-items:center}
.lt-head b{margin-right:2px}
.lt-score{font-weight:800;margin-right:6px;opacity:.8}
.lt-chip{position:relative;display:inline-flex;gap:5px;align-items:center;padding:2px 9px;border-radius:999px;font-weight:700;font-size:12px}
.lt-sr{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%);white-space:nowrap}
.lt-fails{margin:7px 0 0;padding-left:18px;line-height:1.45}
.lt-fails li+li{margin-top:3px}
.lt-chip i{font-style:normal;font-weight:900}
.lt-chip.ok{background:rgba(52,199,150,.16);color:#1f9d73}
.lt-chip.bad{background:rgba(255,170,60,.18);color:#c77a08}
.lt-tip{margin:7px 0 0;opacity:.85;line-height:1.45}`;
  document.head.appendChild(st);
}
