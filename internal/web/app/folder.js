// On treballa la sessió: la carpeta del projecte, sempre a la vista. A la
// capçalera, un xip (icona + nom de la carpeta, ruta sencera en passar-hi
// per sobre i a sota) que obre el selector de projecte, amb un menú:
// canviar de carpeta, obrir-la a l'explorador del sistema (només a
// l'escriptori: el procés principal la resol i l'obre) i copiar la ruta. A
// la pàgina d'inici, «Treballant a <carpeta>» amb la ruta, la branca, si és
// un repo git i si és un worktree aïllat. Al peu del compositor, un discret
// «a <carpeta>». Ho pinta tot a partir de /api/state (event gregal:state).
(function () {
  // ---- Funcions pures (també les fan servir els tests) ----
  // Nom de la carpeta: l'últim tros de la ruta («C:\» i «/» es queden).
  function folderName(p) {
    const raw = String(p || '').trim();
    if (!raw) return '';
    const parts = raw.split(/[\\/]+/).filter(Boolean);
    if (!parts.length) return raw.slice(0, 1);
    const last = parts[parts.length - 1];
    return /^[A-Za-z]:$/.test(last) && parts.length === 1 ? last + '\\' : last;
  }
  // El que cal saber de la carpeta a partir de l'estat de la sessió.
  function folderInfo(st) {
    const s = st || {};
    const path = typeof s.cwd === 'string' ? s.cwd : '';
    const wt = typeof s.worktree === 'string' ? s.worktree : '';
    const repo = typeof s.worktree_repo === 'string' ? s.worktree_repo : '';
    return {
      path,
      name: folderName(path) || (typeof s.project === 'string' ? s.project : ''),
      branch: typeof s.branch === 'string' ? s.branch : '',
      git: s.git === true || !!s.branch,
      worktree: wt,
      repoName: wt ? folderName(repo) || '' : '',
      repoPath: wt ? repo : '',
    };
  }
  // «Worktree: <nom> (de <repo>)», o «Worktree: <nom>» si no se sap el repo.
  function worktreeLabel(info, t) {
    if (!info || !info.worktree) return '';
    const tpl = t('folder.worktree') || 'Worktree: {name} (from {repo})';
    if (!info.repoName) return tpl.replace(/\s*\([^)]*\{repo\}[^)]*\)/, '').replace('{name}', info.worktree);
    return tpl.replace('{name}', info.worktree).replace('{repo}', info.repoName);
  }
  // Text del botó «Obre a…» segons el sistema.
  function openLabelKey(platform) {
    if (platform === 'darwin') return 'folder.openFinder';
    if (platform === 'win32') return 'folder.openExplorer';
    return 'folder.openFiles';
  }

  const pure = { folderName, folderInfo, worktreeLabel, openLabelKey };
  if (typeof module !== 'undefined' && module.exports) module.exports = pure;
  if (typeof document === 'undefined' || typeof window === 'undefined') return;

  // ---- Vista ----
  const $ = id => document.getElementById(id);
  const T = (key, fallback) => {
    try { const v = window.gregalT && window.gregalT(key); if (v && v !== key) return v; } catch (e) {}
    return fallback || '';
  };
  const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const ICON = '<svg class="folder-ico" viewBox="0 0 16 16" aria-hidden="true" focusable="false"><path d="M1.75 4.25c0-.83.67-1.5 1.5-1.5h3l1.5 1.75h5c.83 0 1.5.67 1.5 1.5v6.25c0 .83-.67 1.5-1.5 1.5H3.25c-.83 0-1.5-.67-1.5-1.5z"/></svg>';
  const desktop = () => window.gregalDesktop || null;
  const canOpen = () => { const d = desktop(); return !!(d && typeof d.openWorkspaceFolder === 'function'); };
  const state = () => (window.gregal && window.gregal.state) || {};
  let info = folderInfo({});
  let busy = false;

  function changeFolder() {
    const ws = window.gregalPanels && window.gregalPanels.workspaces;
    if (ws && !busy) ws.pick();
  }
  async function openFolder() {
    if (!canOpen()) return false;
    let ok = false;
    try { ok = await desktop().openWorkspaceFolder(window.gregalSession || 'default'); } catch (e) { ok = false; }
    if (!ok) flash(T('folder.openFailed', 'Could not open the folder'));
    return ok;
  }
  async function copyPath() {
    const text = info.path;
    if (!text) return false;
    let ok = false;
    try { await navigator.clipboard.writeText(text); ok = true; } catch (e) {
      try {
        const ta = document.createElement('textarea');
        ta.value = text; ta.setAttribute('readonly', ''); ta.style.position = 'fixed'; ta.style.opacity = '0';
        document.body.appendChild(ta); ta.select(); ok = document.execCommand('copy'); ta.remove();
      } catch (_) { ok = false; }
    }
    if (ok) flash(T('folder.copied', 'Path copied'));
    return ok;
  }
  const ACTIONS = { change: changeFolder, open: openFolder, copy: copyPath };

  // Avís curt al costat del xip (copiat, no s'ha pogut obrir).
  let flashTimer = 0;
  function flash(text) {
    let el = $('folderFlash');
    if (!el) {
      el = document.createElement('div');
      el.id = 'folderFlash';
      el.className = 'folder-flash';
      el.setAttribute('role', 'status');
      document.body.appendChild(el);
    }
    el.textContent = text;
    el.hidden = false;
    clearTimeout(flashTimer);
    flashTimer = setTimeout(() => { el.hidden = true; }, 1800);
  }

  function actionsHTML(cls) {
    const b = (act, label, extra) => '<button type="button" class="' + cls + '" data-folder-act="' + act + '"' + (extra || '') + '>' + esc(label) + '</button>';
    return b('change', T('folder.change', 'Change folder…'), busy ? ' disabled' : '') +
      (canOpen() ? b('open', T(openLabelKey(desktop().platform), 'Open in file manager')) : '') +
      b('copy', T('folder.copy', 'Copy path'), info.path ? '' : ' disabled');
  }

  // ---- Menú (xip de la capçalera i peu del compositor) ----
  let menu = null, menuAnchor = null;
  function closeMenu(focusBack) {
    if (!menu) return;
    menu.remove();
    menu = null;
    if (menuAnchor) {
      menuAnchor.setAttribute('aria-expanded', 'false');
      if (focusBack) menuAnchor.focus();
    }
    menuAnchor = null;
    document.removeEventListener('pointerdown', onOutside, true);
    document.removeEventListener('keydown', onMenuKey, true);
  }
  function onOutside(e) {
    if (menu && !menu.contains(e.target) && !(menuAnchor && menuAnchor.contains(e.target))) closeMenu(false);
  }
  function onMenuKey(e) {
    if (!menu) return;
    const items = [...menu.querySelectorAll('button:not([disabled])')];
    const i = items.indexOf(document.activeElement);
    if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeMenu(true); }
    else if (e.key === 'ArrowDown') { e.preventDefault(); (items[i + 1] || items[0])?.focus(); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); (items[i - 1] || items[items.length - 1])?.focus(); }
    else if (e.key === 'Tab') closeMenu(false);
  }
  function openMenu(anchor) {
    if (menu) { const same = menuAnchor === anchor; closeMenu(false); if (same) return; }
    menu = document.createElement('div');
    menu.className = 'folder-menu';
    menu.setAttribute('role', 'menu');
    menu.setAttribute('aria-label', T('folder.actions', 'Folder actions'));
    menu.innerHTML = '<div class="folder-menu-path" title="' + esc(info.path) + '">' + ICON + '<span>' + esc(info.path || info.name) + '</span></div>' +
      actionsHTML('folder-menu-item');
    menu.querySelectorAll('button').forEach(b => b.setAttribute('role', 'menuitem'));
    menu.addEventListener('click', e => {
      const b = e.target.closest('[data-folder-act]');
      if (!b || b.disabled) return;
      closeMenu(false);
      ACTIONS[b.dataset.folderAct]();
    });
    document.body.appendChild(menu);
    const r = anchor.getBoundingClientRect();
    const w = menu.offsetWidth, h = menu.offsetHeight;
    const left = Math.max(8, Math.min(r.left, window.innerWidth - w - 8));
    const below = r.bottom + 6 + h <= window.innerHeight - 8;
    menu.style.left = left + 'px';
    menu.style.top = (below ? r.bottom + 6 : Math.max(8, r.top - h - 6)) + 'px';
    menuAnchor = anchor;
    anchor.setAttribute('aria-expanded', 'true');
    document.addEventListener('pointerdown', onOutside, true);
    document.addEventListener('keydown', onMenuKey, true);
    menu.querySelector('button:not([disabled])')?.focus();
  }

  // ---- Pintar ----
  function paintHeader() {
    const title = T('folder.chipTitle', 'Working folder: {path}').replace('{path}', info.path || info.name);
    const main = $('folderChipBtn');
    if (main) { main.title = title; main.setAttribute('aria-label', title); }
    const proj = $('project');
    if (proj) proj.title = info.path;
    const wt = $('hdrWorktree');
    if (wt) {
      // A la capçalera, curt (sense «de <repo>»); sencer al tooltip.
      const label = worktreeLabel(info, T);
      wt.hidden = !label;
      wt.textContent = label ? '⎇ ' + worktreeLabel({ ...info, repoName: '' }, T) : '';
      wt.title = label ? label + ' — ' + T('folder.worktreeHelp') + (info.repoPath ? '\n' + info.repoPath : '') : '';
    }
    const branch = $('branch');
    if (branch && info.path) branch.title = info.git ? (info.branch ? T('folder.branch', 'Branch: {branch}').replace('{branch}', info.branch) : T('folder.detached', 'Detached HEAD')) : T('folder.noGit', 'Not a git repository');
  }

  function paintHome() {
    const box = $('heroFolder');
    if (!box) return;
    if (!info.path) { box.hidden = true; return; }
    box.hidden = false;
    const badges = [];
    if (info.git) {
      badges.push('<span class="hf-badge" title="' + esc(T('folder.git', 'Git repository')) + '">⎇ ' + esc(info.branch || T('folder.detached', 'Detached HEAD')) + '</span>');
    } else {
      badges.push('<span class="hf-badge muted">' + esc(T('folder.noGit', 'Not a git repository')) + '</span>');
    }
    const wt = worktreeLabel(info, T);
    if (wt) badges.push('<span class="hf-badge wt" title="' + esc(T('folder.worktreeHelp') + (info.repoPath ? '\n' + info.repoPath : '')) + '">' + esc(wt) + '</span>');
    box.innerHTML =
      '<div class="hf-row" role="button" tabindex="0" aria-haspopup="menu" title="' + esc(info.path) + '">' + ICON + '<span class="hf-label">' + esc(T('folder.workingIn', 'Working in')) + '</span> ' +
      '<b class="hf-name" title="' + esc(info.path) + '">' + esc(info.name) + '</b>' + badges.join('') + '</div>' +
      '<div class="hf-path" title="' + esc(info.path) + '">' + esc(info.path) + '</div>' +
      '<div class="hf-actions">' + actionsHTML('hf-act') + '</div>';
  }

  function paintHint() {
    const el = $('hintFolder');
    if (!el) return;
    el.hidden = !info.name;
    el.innerHTML = info.name ? '<span class="hint-folder-sep">·</span>' + ICON + '<span>' + esc(T('folder.in', 'in')) + ' <b>' + esc(info.name) + '</b></span>' : '';
    const title = T('folder.hintTitle', 'Gregal is working in {path}').replace('{path}', info.path);
    el.title = title;
    el.setAttribute('aria-label', title);
  }

  function paint(st) {
    info = folderInfo(st || state());
    busy = !!(st || state()).agent_busy;
    paintHeader();
    paintHome();
    paintHint();
    if (menu && menuAnchor) { const a = menuAnchor; closeMenu(false); openMenu(a); }
  }

  function wire() {
    const main = $('folderChipBtn');
    // Al mòbil no hi ha ▾ (no hi cap): tocar el xip obre el menú, que
    // també té «Canvia de carpeta…».
    const narrow = () => !!(window.matchMedia && window.matchMedia('(max-width:480px)').matches);
    if (main) main.addEventListener('click', () => { if (narrow()) openMenu(main); else changeFolder(); });
    const caret = $('folderChipMenu');
    if (caret) caret.addEventListener('click', e => { e.stopPropagation(); openMenu(caret); });
    // Clic dret al xip: el mateix menú.
    const chip = $('folderChip');
    if (chip) chip.addEventListener('contextmenu', e => { e.preventDefault(); openMenu(caret || chip); });
    const hint = $('hintFolder');
    if (hint) hint.addEventListener('click', () => openMenu(hint));
    const home = $('heroFolder');
    if (home) home.addEventListener('click', e => {
      const b = e.target.closest('[data-folder-act]');
      if (b && !b.disabled) { ACTIONS[b.dataset.folderAct](); return; }
      // A l'inici la carpeta és un xip: el menú hi té canviar i copiar.
      const row = e.target.closest('.hf-row');
      if (row) openMenu(row);
    });
    if (home) home.addEventListener('keydown', e => {
      const row = e.target.closest('.hf-row');
      if (row && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); openMenu(row); }
    });
    document.addEventListener('gregal:state', e => paint(e.detail));
    document.addEventListener('gregal:idioma', () => paint());
    paint();
  }

  window.gregalFolder = { ...pure, changeFolder, openFolder, copyPath, openMenu, paint };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', wire);
  else wire();
})();
