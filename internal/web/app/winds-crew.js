// Convoca la colla: when a work request has no fitting wind, offer to summon
// a crew (existing winds that fit + new specialists), then follow the thread
// while they discuss the design and do the work on the server.
(function (root) {
  'use strict';
  const ca = () => document.documentElement.lang === 'ca', t = (a, b) => ca() ? a : b;
  const el = (tag, cls, text) => { const n = document.createElement(tag); if (cls) n.className = cls; if (text !== undefined) n.textContent = text; return n; };
  async function call(url, method = 'GET', body) {
    const r = await root.api(url, { method, ...(body === undefined ? {} : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }) });
    if (!r.ok) throw new Error(await r.text());
    return r.json();
  }
  function member(face, name, role, why, isNew) {
    const row = el('div', 'vw-crew-member' + (isNew ? ' vw-crew-new' : ''));
    row.append(face);
    const copy = el('div');
    const title = el('strong', '', name);
    if (isNew) title.append(el('span', 'vw-crew-tag', t('Nou', 'New')));
    copy.append(title, el('span', 'vw-crew-role', role || ''));
    if (why) copy.append(el('p', 'vw-crew-why', why));
    row.append(copy);
    return row;
  }
  // offer resolves with the new rotllana, or null to go on with the usual crew.
  // crewIds: the winds that would do it anyway; the offer appears when the
  // best crew needs someone else (a new specialist, or a wind summoned before).
  async function offer(task, winds, crewIds = []) {
    const lang = ca() ? 'ca' : 'en';
    let proposal;
    try { proposal = await call('/api/winds/crew/propose', 'POST', { task, lang }); } catch (_) { return null; }
    const outsider = (proposal?.reuse || []).some(r => !crewIds.includes(r.id));
    if (!proposal?.create?.length && !outsider) return null;
    return new Promise(resolve => {
      let settled = false;
      const done = value => { if (!settled) { settled = true; resolve(value); } };
      const d = el('dialog', 'vw-dialog vw-crew-dialog');
      d.setAttribute('aria-label', t('Convoca la colla', 'Summon a crew'));
      const head = el('header');
      head.append(el('h2', '', t('Convoquem una colla?', 'Summon a crew?')));
      d.append(head, el('p', 'vw-muted', proposal.create.length
        ? t('Cap vent no domina tota aquesta feina. Et proposo aquesta colla: en parlaran, un la farà i un altre la revisarà. Els vents nous es queden, amb la seva memòria, per a les feines que vinguin.',
          'No wind covers all of this yet. Here is a crew: they will discuss it, one builds and another reviews. New winds stay, with their own memory, for future tasks.')
        : t('Per a aquesta feina hi ha vents que ja en saben, amb el que recorden de feines anteriors. Et proposo reunir-los: en parlaran, un la farà i un altre la revisarà.',
          'Some winds already know this from earlier work. Here is a crew with them: they will discuss it, one builds and another reviews.')));
      const list = el('div', 'vw-crew-list');
      for (const r of proposal.reuse || []) {
        const w = winds.find(x => x.id === r.id) || { id: r.id, name: r.name };
        list.append(member(root.gregalWinds.face(w), w.name, w.role || r.role, r.why, false));
      }
      for (const c of proposal.create) {
        list.append(member(root.gregalWinds.face({ name: c.name, color: c.color, style: c.style, specialty: c.kit }), c.name, c.role, c.why, true));
      }
      d.append(list);
      if (proposal.plan) d.append(el('p', 'vw-crew-plan', proposal.plan));
      const status = el('p', 'vw-error'); status.setAttribute('role', 'status');
      const actions = el('div', 'vw-actions');
      const later = el('button', 'vw-button', t('Fes-ho amb la colla actual', 'Use the current crew')); later.type = 'button';
      const go = el('button', 'vw-primary', t('Convoca la colla', 'Summon the crew')); go.type = 'button';
      later.onclick = () => { d.close(); done(null); };
      go.onclick = async () => {
        go.disabled = later.disabled = true; status.textContent = t('Convocant la colla…', 'Summoning the crew…');
        try {
          const out = await call('/api/winds/crew/start', 'POST', { task, lang, reuse: (proposal.reuse || []).map(r => r.id), create: proposal.create });
          d.close(); done(out.rotllana);
        } catch (e) {
          status.textContent = t('No s’ha pogut convocar: ', 'Could not summon: ') + e.message; go.disabled = later.disabled = false;
        }
      };
      actions.append(later, go);
      d.append(status, actions);
      d.addEventListener('close', () => { d.remove(); done(null); });
      document.body.append(d);
      d.showModal();
      go.focus();
    });
  }
  // follow polls a crew thread while its job runs, so the discussion and the
  // result appear without reloading. onTick returns false to stop.
  async function follow(id, onTick) {
    for (let i = 0; i < 400; i++) {
      await new Promise(r => setTimeout(r, 4000));
      let status;
      try { status = await call('/api/winds/crew/status?rotllana=' + encodeURIComponent(id)); } catch (_) { continue; }
      if ((await onTick(status)) === false) return;
      if (!status.running) return;
    }
  }
  root.gregalCrew = { offer, follow };
})(window);
