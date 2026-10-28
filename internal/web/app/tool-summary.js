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
      case 'bash': case 'bash_background': out.target = oneLine(s('command'), 200); break;
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
      case 'browser': out.target = [s('action'), s('url') || s('target') || s('key')].filter(Boolean).join(' '); break;
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

  const api = { summarize, detail };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.gregalToolSummary = api;
})(typeof window !== 'undefined' ? window : globalThis);
