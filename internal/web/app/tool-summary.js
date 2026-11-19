// Human summary of a tool call for the transcript: "Read src/app.py ·
// lines 10–40" instead of the raw JSON arguments. The raw arguments stay
// available (title attribute, expanded card); this only decides what a
// person reads at a glance. Pure: also loaded by node tests.
(function (root) {
  const L = {
    en: {
      read: 'Read', write: 'Write', edit: 'Edit', patch: 'Edit', glob: 'Find files', grep: 'Search',
      bash: 'Run', bash_background: 'Start', bash_output: 'Check output', bash_kill: 'Stop process',
      web_search: 'Search the web', web_fetch: 'Fetch', read_image: 'View image',
      gh_issue: 'GitHub issue', gh_pr: 'Pull request', git_commit: 'Commit', gh_pr_create: 'Create pull request', office_read: 'Read document',
      office_open: 'Open document', office_edit: 'Edit document', office_create: 'Create document',
      dashboard_save: 'Save to dashboard', delegate: 'Delegate', question: 'Ask', todowrite: 'Update plan', todoread: 'Read plan',
      skill: 'Open skill', preview: 'Preview', browser: 'Browser', mcp: 'MCP',
      knowledge_search: 'Search knowledge', knowledge_note: 'Save note',
      lines: 'lines', line: 'line', from: 'from', in: 'in', edits: 'edits', edit1: 'edit',
      tasks: 'tasks', task1: 'task', background: 'background',
      list: 'List', test: 'Test', git: 'Git', explored: 'Explored',
      file1: 'file', files: 'files', search1: 'search', searches: 'searches', listing1: 'listing', listings: 'listings',
    },
    ca: {
      read: 'Llegeix', write: 'Escriu', edit: 'Edita', patch: 'Edita', glob: 'Cerca fitxers', grep: 'Cerca',
      bash: 'Executa', bash_background: 'Engega', bash_output: 'Mira la sortida', bash_kill: 'Atura el procés',
      web_search: 'Cerca a la web', web_fetch: 'Obre', read_image: 'Mira la imatge',
      gh_issue: 'Issue de GitHub', gh_pr: 'Pull request', git_commit: 'Fes commit', gh_pr_create: 'Obre una pull request', office_read: 'Llegeix el document',
      office_open: 'Obre el document', office_edit: 'Edita el document', office_create: 'Crea el document',
      dashboard_save: 'Desa al dashboard', delegate: 'Delega', question: 'Pregunta', todowrite: 'Actualitza el pla', todoread: 'Llegeix el pla',
      skill: 'Obre la skill', preview: 'Previsualitza', browser: 'Navegador', mcp: 'MCP',
      knowledge_search: 'Cerca al coneixement', knowledge_note: 'Desa una nota',
      lines: 'línies', line: 'línia', from: 'des de', in: 'a', edits: 'canvis', edit1: 'canvi',
      tasks: 'tasques', task1: 'tasca', background: 'segon pla',
      list: 'Llista', test: 'Prova', git: 'Git', explored: 'Explorat',
      file1: 'fitxer', files: 'fitxers', search1: 'cerca', searches: 'cerques', listing1: 'llistat', listings: 'llistats',
    },
  };

  const lineCount = s => (typeof s === 'string' && s.length ? s.replace(/\n$/, '').split('\n').length : 0);
  const oneLine = (s, max) => {
    s = String(s == null ? '' : s).replace(/\s+/g, ' ').trim();
    return s.length > max ? s.slice(0, max - 1) + '…' : s;
  };

  function parse(args) {
    if (args && typeof args === 'object') return args;
    try {
      const o = JSON.parse(String(args || ''));
      return o && typeof o === 'object' && !Array.isArray(o) ? o : null;
    } catch (e) { return null; }
  }

  // Ordres de shell que només miren (ls, cat, sed -n, grep…): es llegeixen
  // com el que fan i, seguides, es pleguen en una fila «Explorat». Qualsevol
  // cosa que podria escriure (redirecció a un fitxer, $(…), sed -i, find
  // -exec, un programa desconegut) deixa l'ordre com a «Executa».
  function shellSegments(cmd) {
    const segs = [];
    let words = [], w = '', has = false, q = '', pipe = false;
    const endWord = () => { if (has) words.push(w); w = ''; has = false; };
    const endSeg = next => { endWord(); if (words.length) segs.push({ pipe, words }); words = []; pipe = next; };
    for (let i = 0; i < cmd.length; i++) {
      const c = cmd[i];
      if (q) {
        if (c === q) q = '';
        else if (q === '"' && (c === '`' || (c === '$' && cmd[i + 1] === '('))) return null;
        else if (q === '"' && c === '\\' && i + 1 < cmd.length) w += cmd[++i];
        else w += c;
        continue;
      }
      if (c === "'" || c === '"') { q = c; has = true; continue; }
      if (c === '\\' && i + 1 < cmd.length) { w += cmd[++i]; has = true; continue; }
      if (c === '`' || c === '<' || (c === '$' && cmd[i + 1] === '(')) return null;
      if (c === '>') {
        // Només es tolera llençar la sortida (2>/dev/null) o ajuntar-la (2>&1).
        if (has && !/^\d$/.test(w)) return null;
        w = ''; has = false;
        const m = /^>?\s*(&[12]|\/dev\/null)(?=$|[\s;|&])/.exec(cmd.slice(i + 1));
        if (!m) return null;
        i += m[0].length;
        continue;
      }
      if (c === ' ' || c === '\t') { endWord(); continue; }
      if (c === ';' || c === '\n') { endSeg(false); continue; }
      if (c === '&') { if (cmd[i + 1] !== '&') return null; i++; endSeg(false); continue; }
      if (c === '|') { if (cmd[i + 1] === '|') { i++; endSeg(false); } else endSeg(true); continue; }
      w += c; has = true;
    }
    if (q) return null;
    endSeg(false);
    return segs;
  }
  // Opcions que s'enduen la paraula següent (head -n 50, grep -A 3…).
  const VALUED = {
    head: ['-n', '-c'], tail: ['-n', '-c'], grep: ['-e', '-f', '-A', '-B', '-C', '-m'],
    rg: ['-e', '-g', '-t', '-T', '-A', '-B', '-C', '-m', '--glob', '--type'],
  };
  const READERS = new Set(['cat', 'head', 'tail', 'nl', 'wc']);
  const FILTERS = new Set(['head', 'tail', 'sort', 'uniq', 'wc', 'grep', 'rg', 'cut', 'nl']);
  const FIND_WRITES = new Set(['-delete', '-exec', '-execdir', '-ok', '-okdir', '-fprint', '-fprint0', '-fprintf', '-fls']);
  // Opcions amb què programes que normalment només miren escriuen un fitxer
  // o executen un altre programa (sort -o, rg --pre, tree -o…).
  function writes(prog, rest) {
    const has = re => rest.some(a => re.test(a));
    switch (prog) {
      case 'find': return rest.some(a => FIND_WRITES.has(a));
      case 'tree': return has(/^-[^-]*o/);
      // GNU accepta abreviacions de les opcions llargues: cap ni una.
      case 'sort': return has(/^--|^-[^-]*o/);
      case 'uniq': return rest.filter(a => a[0] !== '-' || a === '-').length > 1;
      case 'rg': return has(/^--(pre|hostname-bin)/);
    }
    return false;
  }
  function splitArgs(prog, rest) {
    const flags = [], ops = [], vals = {};
    for (let i = 0; i < rest.length; i++) {
      const a = rest[i];
      if (a.length > 1 && a[0] === '-') {
        flags.push(a);
        if ((VALUED[prog] || []).includes(a) && i + 1 < rest.length) vals[a] = rest[++i];
      } else ops.push(a);
    }
    return { flags, ops, vals };
  }
  // Les accions d'un tros d'ordre: [] si és neutre (cd, echo), null si no només mira.
  function shellAction(prog, rest) {
    if (prog === 'cd' || prog === 'pwd' || prog === 'echo' || prog === 'printf') return [];
    if (writes(prog, rest)) return null;
    const { ops, vals } = splitArgs(prog, rest);
    if (prog === 'ls' || prog === 'tree') return [{ kind: 'list', target: ops.join(', ') || '.' }];
    if (prog === 'find') {
      if (rest.some(a => FIND_WRITES.has(a))) return null;
      const roots = [];
      for (const a of rest) { if (a[0] === '-' || a === '(' || a === '!') break; roots.push(a); }
      return [{ kind: 'list', target: roots.join(', ') || '.' }];
    }
    if (READERS.has(prog)) return ops.length ? ops.map(f => ({ kind: 'read', target: f })) : null;
    if (prog === 'sed') {
      // Llista blanca: sed pot escriure (-i, w) o executar (e, -f amb un
      // guió), així que només val «sed -n» amb un sol guió de rangs a imprimir.
      let quiet = false, script = null;
      const files = [];
      for (let i = 0; i < rest.length; i++) {
        const a = rest[i];
        if (a === '--quiet' || a === '--silent' || /^-[nErs]+$/.test(a)) { quiet = quiet || a[1] === '-' || a.includes('n'); }
        else if (a === '-e') { if (script !== null || i + 1 >= rest.length) return null; script = rest[++i]; }
        else if (a.length > 1 && a[0] === '-') return null;
        else if (script === null) script = a;
        else files.push(a);
      }
      if (!quiet || !/^(\d+(,\d+)?p;?)+$/.test(script || '') || !files.length) return null;
      const lines = script.split(';').filter(Boolean).map(r => r.slice(0, -1).replace(',', '–')).join(', ');
      return files.map(f => ({ kind: 'read', target: f, lines }));
    }
    if (prog === 'grep' || prog === 'rg') {
      const pattern = vals['-e'] !== undefined ? vals['-e'] : ops.shift();
      if (pattern === undefined) return null;
      return [{ kind: 'search', target: pattern, dir: ops.join(', ') }];
    }
    return null;
  }
  function shellActions(cmd) {
    const segs = shellSegments(String(cmd || ''));
    if (!segs || !segs.length) return null;
    const out = [];
    for (const { pipe, words } of segs) {
      const [prog, ...rest] = words;
      if (pipe && FILTERS.has(prog)) { if (writes(prog, rest)) return null; continue; }
      const a = pipe ? null : shellAction(prog, rest);
      if (!a) return null;
      out.push(...a);
    }
    return out.length ? out : null;
  }

  // explore torna què mira una crida d'exploració ([{kind, target}], amb kind
  // read, list o search), o null si la crida fa alguna altra cosa.
  function explore(name, args) {
    const o = parse(args);
    if (!o) return null;
    const s = k => (typeof o[k] === 'string' ? o[k] : '');
    switch (String(name || '')) {
      case 'read': return [{ kind: 'read', target: s('path') }];
      case 'grep': return [{ kind: 'search', target: s('pattern') }];
      case 'glob': return [{ kind: 'list', target: s('pattern') }];
      case 'bash': {
        const acts = shellActions(s('command'));
        return acts && acts.map(a => ({ kind: a.kind, target: a.target }));
      }
    }
    return null;
  }
  // exploreSummary resumeix una fila plegada: fitxers diferents llegits,
  // cerques i llistats, a partir dels resultats d'explore de cada crida.
  function exploreSummary(calls, lang) {
    const t = L[lang] || L.en;
    const files = new Set();
    let searches = 0, listings = 0;
    for (const acts of calls) for (const a of acts || []) {
      if (a.kind === 'read') files.add(a.target);
      else if (a.kind === 'search') searches++;
      else if (a.kind === 'list') listings++;
    }
    const part = (n, one, many) => (n ? `${n} ${n === 1 ? one : many}` : '');
    return {
      label: t.explored,
      target: [part(files.size, t.file1, t.files), part(searches, t.search1, t.searches), part(listings, t.listing1, t.listings)]
        .filter(Boolean).join(' · '),
    };
  }

  // summarize returns {label, target, meta}: label is the action, target the
  // thing acted on (path, command, URL…), meta a short qualifier. Unknown
  // tools keep their name and show key: value pairs instead of JSON.
  function summarize(name, args, lang) {
    const t = L[lang] || L.en;
    const o = parse(args);
    const tool = String(name || '');
    const out = { label: t[tool] || tool, target: '', meta: '' };
    if (!o) { out.target = oneLine(args, 160); return out; }
    const s = k => (typeof o[k] === 'string' ? o[k] : '');
    switch (tool) {
      case 'read': {
        out.target = s('path');
        const off = +o.offset || 0, lim = +o.limit || 0;
        if (off > 1 && lim > 0) out.meta = `${t.lines} ${off}–${off + lim - 1}`;
        else if (lim > 0) out.meta = `${t.lines} 1–${lim}`;
        else if (off > 1) out.meta = `${t.from} ${t.line} ${off}`;
        break;
      }
      case 'write': {
        out.target = s('path');
        const n = lineCount(s('content'));
        if (n) out.meta = `${n} ${n === 1 ? t.line : t.lines}`;
        break;
      }
      case 'edit': {
        out.target = s('path');
        const add = lineCount(s('new_string')), del = lineCount(s('old_string'));
        out.meta = `+${add} −${del}`;
        break;
      }
      case 'patch': {
        out.target = s('path');
        const n = Array.isArray(o.edits) ? o.edits.length : 0;
        if (n) out.meta = `${n} ${n === 1 ? t.edit1 : t.edits}`;
        break;
      }
      case 'glob': out.target = s('pattern'); if (s('dir')) out.meta = `${t.in} ${s('dir')}`; break;
      case 'symbols': out.target = s('query') ? `"${oneLine(s('query'), 80)}"` : ''; out.meta = [s('kind'), s('file')].filter(Boolean).join(' · '); break;
      case 'grep':
        out.target = s('pattern') ? `"${oneLine(s('pattern'), 80)}"` : '';
        out.meta = [s('dir') && `${t.in} ${s('dir')}`, s('include')].filter(Boolean).join(' · ');
        break;
      case 'bash': case 'bash_background': {
        const cmd = s('command');
        const acts = tool === 'bash' ? shellActions(cmd) : null;
        if (acts) {
          const kind = acts[0].kind, same = acts.filter(a => a.kind === kind);
          out.label = t[kind === 'search' ? 'grep' : kind];
          if (kind === 'search') {
            out.target = `"${oneLine(same[0].target, 80)}"`;
            if (same[0].dir) out.meta = `${t.in} ${same[0].dir}`;
          } else {
            out.target = oneLine([...new Set(same.map(a => a.target))].join(', '), 200);
            const lines = same.length === 1 && same[0].lines;
            if (lines) out.meta = `${/[–,]/.test(lines) ? t.lines : t.line} ${lines}`;
          }
          break;
        }
        const head = cmd.replace(/^\s*cd\s+\S+\s*&&\s*/, '');
        // El verb només per a una ordre sola: una cadena pot fer una altra cosa després.
        const single = tool === 'bash' && !/[;\n]|&&|\|\|/.test(head);
        if (single && /^(go test|cargo test|pytest|uv run pytest|python3? -m (pytest|unittest)|node --test|(npm|pnpm|yarn)( run)? test)\b/.test(head)) out.label = t.test;
        else if (single && /^git\s/.test(head)) { out.label = t.git; out.target = oneLine(head.replace(/^git\s+/, ''), 200); break; }
        out.target = oneLine(cmd, 200);
        break;
      }
      case 'bash_output': case 'bash_kill': out.target = s('id'); break;
      case 'web_search': out.target = oneLine(s('query'), 120); break;
      case 'knowledge_search': out.target = s('query') ? `"${oneLine(s('query'), 110)}"` : ''; out.meta = s('collection'); break;
      case 'knowledge_note': out.target = oneLine(s('title') || s('text'), 110); out.meta = s('collection'); break;
      case 'web_fetch': out.target = s('url'); if (s('find')) out.meta = `"${oneLine(s('find'), 40)}"`; break;
      case 'read_image': case 'office_read': case 'office_open': case 'office_create': out.target = s('path'); break;
      case 'office_edit': out.target = s('path'); out.meta = [s('op'), s('sheet'), s('cell')].filter(Boolean).join(' · '); break;
      case 'gh_issue': case 'gh_pr':
        out.target = [s('action'), o.number ? '#' + o.number : '', s('query') && `"${oneLine(s('query'), 60)}"`].filter(Boolean).join(' ');
        out.meta = s('repo');
        break;
      case 'git_commit':
        out.target = oneLine(String(s('message')).split(/\r?\n/)[0], 120);
        out.meta = Array.isArray(o.paths) && o.paths.length ? o.paths.length + ' files' : (o.all ? 'all' : '');
        break;
      case 'gh_pr_create': out.target = oneLine(s('title'), 120); out.meta = [s('base') && '→ ' + s('base'), o.draft ? 'draft' : ''].filter(Boolean).join(' · '); break;
      case 'delegate': {
        const tasks = Array.isArray(o.tasks) ? o.tasks : [];
        out.target = tasks.length ? oneLine(tasks[0] && tasks[0].prompt, 110) : '';
        if (tasks.length > 1) out.meta = `${tasks.length} ${t.tasks}`;
        break;
      }
      case 'question': out.target = oneLine(s('query'), 140); break;
      case 'dashboard_save': out.target = s('dashboard'); if (Array.isArray(o.plots)) out.meta = String(o.plots.length) + (o.plots.length === 1 ? ' chart' : ' charts'); break;
      case 'skill': out.target = s('nom') || s('name'); break;
      case 'preview': out.target = s('title') || s('kind'); break;
      case 'browser': out.target = [s('action'), s('url') || s('target') || s('key') || (s('action') === 'handoff' ? oneLine(s('text'), 120) : '')].filter(Boolean).join(' '); break;
      case 'todowrite': case 'todoread': break;
      default: {
        const m = /^mcp_([^_]+)_(.+)$/.exec(tool);
        if (m) out.label = `${t.mcp} · ${m[1]} › ${m[2]}`;
        out.target = oneLine(Object.keys(o).map(k => {
          const v = o[k];
          return k + ': ' + (typeof v === 'string' ? v : JSON.stringify(v));
        }).join(' · '), 160);
      }
    }
    return out;
  }

  // detail torna el text sencer de la crida per a la targeta oberta: la
  // capçalera la resumeix i la retalla, però en clicar s'ha de poder llegir
  // (i copiar) l'ordre o els arguments complets. Els textos molt llargs
  // (el contingut d'un write, per exemple) es retallen amb el compte.
  const LONG = 1500;
  const clip = v => (typeof v === 'string' && v.length > LONG ? v.slice(0, LONG) + `… (+${v.length - LONG})` : v);
  function detail(name, args) {
    const o = parse(args);
    const tool = String(name || '');
    if (!o) return String(args == null ? '' : args).trim();
    const s = k => (typeof o[k] === 'string' ? o[k] : '');
    if ((tool === 'bash' || tool === 'bash_background') && s('command')) {
      const extra = Object.keys(o).filter(k => k !== 'command' && o[k] !== '' && o[k] != null);
      return s('command') + (extra.length ? '\n\n' + extra.map(k => `# ${k}: ${typeof o[k] === 'string' ? o[k] : JSON.stringify(o[k])}`).join('\n') : '');
    }
    const shown = {};
    for (const k of Object.keys(o)) shown[k] = clip(o[k]);
    return JSON.stringify(shown, null, 2);
  }

  const api = { summarize, detail, explore, exploreSummary };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.gregalToolSummary = api;
})(typeof window !== 'undefined' ? window : globalThis);
