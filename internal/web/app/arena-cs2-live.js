// El Counter-Strike 2 en directe de l'Arena (arena-cs2-live.html): la
// persona porta un jugador en primera persona (WASD, ratolí, dispara,
// planta, desactiva, compra), els bots fan la resta (arena-cs2-live-sim.js)
// i el model, al servidor, crida el pla de cada ronda des del .md de cada
// equip (/api/arena/live). La vista reaprofita el dibuix de la càmera en
// primera persona del visor (arena-cs2-view.js): parets, figures, fums,
// l'arma a les mans.

import { Cs2View, projectPoint, angDiff, weaponClass, EYE, BODY_H, fitMap, fmtClock, money, prettyCall } from './arena-cs2-view.js';
import { LiveRound, TICK, FREEZE, BUY_TIME, ROUND_TIME, BOMB_TIME, WEAPONS, NADES, NADE_NAME, nadeCost, PLANT_TIME, KIT_TIME, BARE_TIME, zoneAt, inZone } from './arena-cs2-live-sim.js';

const $ = id => document.getElementById(id);
const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
const lerp = (a, b, t) => a + (b - a) * t;
const FONT = '"Segoe UI", system-ui, sans-serif';
const q = new URLSearchParams(location.hash.slice(1));
const headers = q.get('token') ? { Authorization: 'Bearer ' + q.get('token') } : {};
const POST_TIME = 5;
const BOT_ROLES = ['entry', 'support', 'lurker', 'awper', 'anchor', 'rifler'];
const POS = ['IGL', 'AWPer', 'Entry', 'Support', 'Lurker'];
const DEFAULT_ROLE = ['rifler', 'awper', 'entry', 'support', 'lurker'];
const ROLE_HELP = { entry: 'first in, takes duels', support: 'follows the entry, flashes, trades', lurker: 'plays apart, flanks late', awper: 'holds long angles', anchor: 'holds the site, rotates late', rifler: 'standard' };
const REASON = { elim: 'Elimination', bomb: 'Bomb exploded', defuse: 'Bomb defused', time: 'Time ran out' };

async function call(body) {
  const r = await fetch('/api/arena/live', { method: 'POST', headers: { 'Content-Type': 'application/json', ...headers }, body: JSON.stringify(body) });
  const j = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(j.error || r.status);
  return j;
}
const getArena = () => fetch('/api/arena', { headers }).then(r => r.json());
const store = { get(k, d) { try { const v = localStorage.getItem('cs2live.' + k); return v == null ? d : JSON.parse(v); } catch (e) { return d; } }, set(k, v) { try { localStorage.setItem('cs2live.' + k, JSON.stringify(v)); } catch (e) {} } };

// ---- el lobby ----

async function lobby(game) {
  const card = $('lobbyCard');
  let d;
  try { d = await getArena(); } catch (e) { card.innerHTML = `<p class="err">${esc(e.message)}</p>`; return; }
  const mine = (d.mine || {}).cs2;
  const bots = (d.bots || {}).cs2 || [];
  const others = ((d.leaderboard || {}).cs2 || []).filter(r => !r.bot && r.owner !== d.me);
  const next = (d.next || {}).cs2;
  const cfg = store.get('cfg', { slot: 2, format: 'short', skill: 'normal', sens: 2.2, roles: DEFAULT_ROLE.slice(), notes: ['', '', '', '', ''] });
  card.innerHTML = `<h1>💣 Counter-Strike 2 · live</h1>
    <p class="sub">You play one player of your team with keyboard and mouse. Before every round the coach model reads your strategy .md${mine ? ` (<b>${esc(mine.display || '')}</b>)` : ' (the starter template, until you write one)'} and calls the buy and the plan; your four bots play it with the roles below. The opponent is a bot team led by the same model with its own .md. Not ranked.</p>
    <div class="row"><label>Opponent <select id="lbOpp">${bots.length ? '' : `<option value="">🤖 ${esc(next ? next.display : 'Bot Template')}</option>`}${bots.map(b => `<option value="${esc(b.owner)}"${next && next.owner === b.owner ? ' selected' : ''}>🤖 ${esc(b.display)} (${esc(b.level || '')})</option>`).join('')}${others.map(o => `<option value="${esc(o.owner)}">👤 ${esc(o.display)}</option>`).join('')}</select></label>
      <label>Format <select id="lbFormat"><option value="short">Short · MR6 (first to 7)</option><option value="full">Full · MR12 (first to 13)</option></select></label>
      <label>Bots <select id="lbSkill"><option value="easy">Easy</option><option value="normal">Normal</option><option value="hard">Hard</option></select></label>
      <label>Mouse <input id="lbSens" type="range" min="0.6" max="5" step="0.1" style="width:110px"> <span id="lbSensV" class="muted"></span></label></div>
    <h2>Your team</h2>
    <p class="muted">Pick the position you play (highlighted). For each bot pick a role and, if you like, a short instruction: the coach reads them every round and may change a role for one round.</p>
    <div class="slots" id="lbSlots"></div>
    <div class="row"><button type="button" id="lbCoach">✨ Ask the coach to set the roles</button><span id="lbCoachMsg" class="muted"></span></div>
    <h2>Controls</h2>
    <div class="keys"><span><kbd>W</kbd><kbd>A</kbd><kbd>S</kbd><kbd>D</kbd> move</span><span><kbd>Mouse</kbd> look · <kbd>Left</kbd> shoot</span><span><kbd>Right</kbd> AWP scope</span><span><kbd>Shift</kbd> walk · <kbd>C</kbd> crouch</span>
      <span><kbd>1</kbd> rifle <kbd>2</kbd> pistol <kbd>3</kbd> knife</span><span><kbd>4</kbd> grenades (again: next) · <kbd>G</kbd> throw</span><span><kbd>R</kbd> reload · <kbd>Q</kbd> last weapon</span><span><kbd>E</kbd> hold: plant / defuse</span>
      <span><kbd>B</kbd> buy menu</span><span><kbd>Tab</kbd> scoreboard</span><span><kbd>Esc</kbd> free the mouse</span><span><kbd>F</kbd> autopilot (a bot plays you)</span></div>
    <div class="row" style="margin-top:16px"><button type="button" class="primary" id="lbStart">▶ Start the match</button><span id="lbMsg" class="muted"></span></div>`;
  $('lbFormat').value = cfg.format; $('lbSkill').value = cfg.skill; $('lbSens').value = cfg.sens;
  const sensV = () => { $('lbSensV').textContent = (+$('lbSens').value).toFixed(1); };
  $('lbSens').oninput = sensV; sensV();
  const paintSlots = () => {
    $('lbSlots').innerHTML = POS.map((p, i) => i === cfg.slot
      ? `<div class="slot me" data-i="${i}"><b>${p}</b><small>you play this one</small></div>`
      : `<div class="slot" data-i="${i}"><b>${p}</b><small>bot · <a href="#" data-me="${i}" style="color:#8fb4ff">play this</a></small>
          <select data-role="${i}">${BOT_ROLES.map(r => `<option value="${r}"${cfg.roles[i] === r ? ' selected' : ''}>${r} — ${ROLE_HELP[r]}</option>`).join('')}</select>
          <input data-note="${i}" maxlength="140" placeholder="Instruction (optional)" value="${esc(cfg.notes[i] || '')}"></div>`).join('');
    $('lbSlots').querySelectorAll('[data-me]').forEach(a => a.onclick = e => { e.preventDefault(); cfg.slot = +a.dataset.me; paintSlots(); });
    $('lbSlots').querySelectorAll('[data-role]').forEach(s => s.onchange = () => { cfg.roles[+s.dataset.role] = s.value; });
    $('lbSlots').querySelectorAll('[data-note]').forEach(s => s.oninput = () => { cfg.notes[+s.dataset.note] = s.value; });
  };
  paintSlots();
  $('lbCoach').onclick = async () => {
    $('lbCoach').disabled = true; $('lbCoachMsg').textContent = 'The coach is reading your .md…';
    try {
      const r = await call({ action: 'roles', slot: cfg.slot });
      r.roles.forEach((ro, i) => { if (i !== cfg.slot && BOT_ROLES.includes(ro)) cfg.roles[i] = ro; });
      r.notes.forEach((n, i) => { if (i !== cfg.slot && n) cfg.notes[i] = n; });
      paintSlots(); $('lbCoachMsg').textContent = 'Done: change anything you like.';
    } catch (e) { $('lbCoachMsg').textContent = e.message; }
    $('lbCoach').disabled = false;
  };
  $('lbStart').onclick = async () => {
    cfg.format = $('lbFormat').value; cfg.skill = $('lbSkill').value; cfg.sens = +$('lbSens').value;
    store.set('cfg', cfg);
    $('lbStart').disabled = true; $('lbMsg').textContent = 'Setting up the match…';
    try {
      const r = await call({ action: 'start', opponent: $('lbOpp').value, format: cfg.format, slot: cfg.slot, roles: cfg.roles.map((x, i) => i === cfg.slot ? '' : x), notes: cfg.notes.map((x, i) => i === cfg.slot ? '' : x) });
      $('lobby').hidden = true;
      game.begin(r, cfg);
    } catch (e) { $('lbMsg').textContent = e.message; $('lbStart').disabled = false; }
  };
}

// ---- el joc ----

class LiveGame {
  constructor(cv) {
    this.cv = cv;
    const r = this.r = Object.create(Cs2View.prototype); // els pinzells de la vista en primera persona
    r.ctx = cv.getContext('2d'); r.mapCache = null; r.signs = null; r.povBoomT = 0; r.layout = 'full';
    this.keys = {}; this.mouse = { dx: 0, dy: 0, fire: false, scope: false };
    this.feed = []; this.stats = {}; this.score = [0, 0]; this.state = 'lobby'; this.hits = [];
    this.cam = { bob: 0, spd: 0 }; this.lastSlot = 'pistol'; this.nadeSel = 0; this.autopilot = false;
    this.resize(); addEventListener('resize', () => this.resize());
    this.bindInput();
    this.last = performance.now(); this.acc = 0;
    requestAnimationFrame(t => this.loop(t));
  }

  resize() {
    const b = this.cv.getBoundingClientRect(), d = devicePixelRatio || 1;
    this.cv.width = Math.max(320, Math.round(b.width * d)); this.cv.height = Math.max(200, Math.round(b.height * d));
    const r = this.r; r.W = this.cv.width; r.H = this.cv.height; r.dpr = d;
    r.u = d * clamp(Math.min(b.width / 960, b.height / 600), 0.66, 1.35); r.narrow = b.width < 640;
    r.box = { x: 0, y: 0, w: r.W, h: r.H };
    this.radar = null;
  }

  begin(start, cfg) {
    this.id = start.id; this.meta = start.meta; this.names = start.names; this.humanId = start.human; this.team = start.team; this.cfg = cfg;
    this.r.meta = this.meta; this.r.signs = null; this.r.mapCache = null;
    for (const t of this.meta.teams) for (const p of t.players) this.stats[p.id] = { k: 0, d: 0, dmg: 0, handle: p.handle };
    this.nextRound();
  }

  async nextRound() {
    this.state = 'loading'; this.R = null; this.sent = false;
    this.loadingMsg = 'The coaches are calling the round…';
    try {
      const p = this.planReq || call({ action: 'plan', id: this.id });
      this.planReq = null;
      const r = await p;
      this.startRound(r.setup, r.decisions || []);
    } catch (e) { this.loadingMsg = 'Could not get the plan: ' + e.message + ' — retrying…'; setTimeout(() => this.nextRound(), 3000); }
  }

  startRound(setup, decisions) {
    this.setup = setup; this.decisions = decisions; this.score = setup.score;
    if (this.G) { this.R = new LiveRound(this.meta, setup, { skill: this.cfg.skill, grid: this.G, nav: this.N, humanBot: this.autopilot }); }
    else { this.R = new LiveRound(this.meta, setup, { skill: this.cfg.skill, humanBot: this.autopilot }); this.G = this.R.G; this.N = this.R.N; }
    const me = this.R.human;
    this.bought = []; this.startLoadout = this.snapshot(me);
    this.loadout = this.snapshot(me);
    this.state = 'round'; this.banner = null; this.spec = null;
    this.mine = me.side; this.r.povBoomT = 0;
    this.radar = null;
    // auto-buy on the pistol round: kevlar if there is nothing else to buy
    if (setup.round === 1 || me.money >= 3700 && !me.primary) this.autoBuy(true);
    this.toast(`Round ${setup.round} · you are ${me.side === 'T' ? 'Terrorist' : 'Counter-Terrorist'} · press B to buy`, 4);
  }

  snapshot(p) { return { primary: p.primary, pistol: p.pistol, armor: p.armor > 0 ? (p.helmet ? 2 : 1) : 0, kit: p.kit, nades: p.nades.slice() }; }

  toast(text, dur = 2.5) { this.msg = { text, until: performance.now() + dur * 1000 }; }

  // ---- entrada ----
  bindInput() {
    const cv = this.cv;
    cv.addEventListener('click', () => { if (this.state === 'round' && $('buy').hidden && $('board').hidden && document.pointerLockElement !== cv) cv.requestPointerLock && cv.requestPointerLock(); });
    addEventListener('mousemove', e => { if (document.pointerLockElement === cv) { this.mouse.dx += e.movementX; this.mouse.dy += e.movementY; } });
    addEventListener('mousedown', e => { if (document.pointerLockElement !== cv) return; if (e.button === 0) this.mouse.fire = true; if (e.button === 2) this.mouse.scope = true; });
    addEventListener('mouseup', e => { if (e.button === 0) this.mouse.fire = false; });
    addEventListener('contextmenu', e => { if (document.pointerLockElement === cv) e.preventDefault(); });
    addEventListener('wheel', e => { if (document.pointerLockElement === cv && this.R) this.cycleWeapon(e.deltaY > 0 ? 1 : -1); }, { passive: true });
    addEventListener('keydown', e => {
      if (e.target && /INPUT|SELECT|TEXTAREA/.test(e.target.tagName)) return;
      const k = e.code;
      if (k === 'Tab') { e.preventDefault(); this.showBoard(true); return; }
      if (k === 'KeyB') { this.toggleBuy(); return; }
      if (!$('buy').hidden && /^Digit[1-9]$/.test(k)) { const btn = $('buyCard').querySelectorAll('[data-w]')[+k.slice(5) - 1]; if (btn) btn.click(); return; }
      if (k === 'KeyF' && !e.repeat) { this.autopilot = !this.autopilot; if (this.R) this.R.opts.humanBot = this.autopilot; this.toast(this.autopilot ? 'Autopilot ON: a bot plays you (F to take over)' : 'You have the controls'); }
      this.keys[k] = true;
      if (e.repeat) return;
      if (!this.R || !this.R.human.alive) { if (k === 'Space') this.nextSpec(); return; }
      const me = this.R.human;
      if (k === 'Digit1' && me.primary) this.pickSlot('primary');
      if (k === 'Digit2') this.pickSlot('pistol');
      if (k === 'Digit3') this.pickSlot('knife');
      if (k === 'Digit4') { const ns = this.nadeList(); if (ns.length) { if (me.slot === 'nade') this.nadeSel = (this.nadeSel + 1) % ns.length; else this.pickSlot('nade'); } }
      if (k === 'KeyQ') this.pickSlot(this.lastSlot);
      if (k === 'KeyR') this.wantReload = true;
      if (k === 'KeyG') this.wantThrow = true;
    });
    addEventListener('keyup', e => { this.keys[e.code] = false; if (e.code === 'Tab') this.showBoard(false); });
    addEventListener('blur', () => { this.keys = {}; this.mouse.fire = false; });
  }

  nadeList() { const me = this.R && this.R.human; return me ? NADES.filter(n => me.nades.includes(n)) : []; }
  pickSlot(s) {
    const me = this.R.human; if (s === me.slot) return;
    if (s === 'primary' && !me.primary) return;
    if (s === 'nade' && !this.nadeList().length) return;
    this.lastSlot = me.slot === 'nade' ? this.lastSlot : me.slot;
    if (s === 'nade') { me.slot = 'nade'; me.scoped = false; return; }
    this.R.switchTo(me, s);
  }
  cycleWeapon(dir) {
    const me = this.R.human, order = ['primary', 'pistol', 'knife', 'nade'].filter(s => s !== 'primary' || me.primary).filter(s => s !== 'nade' || this.nadeList().length);
    const i = order.indexOf(me.slot); this.pickSlot(order[(i + dir + order.length) % order.length]);
  }

  input() {
    const k = this.keys, sens = (this.cfg ? this.cfg.sens : 2.2) * 0.0011 * (this.R && this.R.human.scoped ? 0.4 : 1);
    const inp = {
      fwd: (k.KeyW ? 1 : 0) - (k.KeyS ? 1 : 0), side: (k.KeyD ? 1 : 0) - (k.KeyA ? 1 : 0),
      walk: !!(k.ShiftLeft || k.ShiftRight), crouch: !!(k.KeyC || k.ControlLeft), use: !!k.KeyE,
      dAng: this.mouse.dx * sens, dPitch: -this.mouse.dy * sens, reload: this.wantReload, scope: this.mouse.scope,
    };
    this.mouse.dx = this.mouse.dy = 0; this.wantReload = false; this.mouse.scope = false;
    const me = this.R.human;
    if (me.slot === 'nade') {
      const ns = this.nadeList(), n = ns[this.nadeSel % Math.max(1, ns.length)];
      if ((this.mouse.fire || this.wantThrow) && n) { inp.throw = n; this.mouse.fire = false; }
      if (!ns.length || inp.throw && ns.length === 1) setTimeout(() => { if (this.R && this.R.human.slot === 'nade') this.R.switchTo(this.R.human, this.lastSlot === 'nade' ? 'pistol' : this.lastSlot); }, 0);
    } else {
      inp.fire = this.mouse.fire && (document.pointerLockElement === this.cv || this.unlocked);
      if (this.wantThrow) { const ns = this.nadeList(); if (ns.length) inp.throw = ns[this.nadeSel % ns.length]; }
      if (weaponClass(this.R.weaponId(me)) !== 'rifle' && this.R.weaponId(me) !== 'awp' && this.R.weaponId(me) !== 'mac10' && this.R.weaponId(me) !== 'mp9') this.mouse.fire = false; // pistols and knife: one click, one shot
    }
    this.wantThrow = false;
    // Sense el ratolí capturat no es mou ni dispara (unlocked: proves automàtiques).
    if (document.pointerLockElement !== this.cv && !this.unlocked) { inp.fwd = inp.side = 0; inp.fire = false; }
    return inp;
  }

  // ---- compra ----
  canBuy() { const R = this.R, me = R && R.human; if (!me || !me.alive || R.over || R.t > BUY_TIME) return false; return inZone(R.byZone[me.side === 'T' ? 't_spawn' : 'ct_spawn'], me.x, me.y, 2); }
  toggleBuy(force) {
    const open = force != null ? force : $('buy').hidden;
    if (open && !this.canBuy()) { this.toast(this.R && this.R.t > BUY_TIME ? 'Buy time is over' : 'You can only buy in your spawn'); return; }
    $('buy').hidden = !open;
    if (open) { document.exitPointerLock && document.exitPointerLock(); this.paintBuy(); }
  }
  spendable() { return this.R.human.money; }
  paintBuy() {
    const me = this.R.human, side = me.side, list = id => { const w = WEAPONS[id]; return w && (!w.side || (w.side === 1) === (side === 'T')); };
    const W = (id, n) => list(id) ? `<button type="button" data-w="${id}" class="${me.primary === id || me.pistol === id ? 'own' : ''}">${n}. ${esc(WEAPONS[id].name)} <span>${money(WEAPONS[id].cost)}</span></button>` : '';
    let n = 0;
    const col = (title, ids) => `<div><h3>${title}</h3>${ids.filter(list).map(id => W(id, ++n)).join('')}</div>`;
    const gear = [['kevlar', 'Kevlar', 650], ['helmet', 'Kevlar + helmet', me.armor > 0 && !me.helmet ? 350 : 1000]].concat(side === 'CT' ? [['kit', 'Defuse kit', 400]] : []);
    $('buyCard').innerHTML = `<div class="row" style="justify-content:space-between"><h1>Buy</h1><span class="money">${money(me.money)}</span></div>
      <p class="muted">Click to buy (a new gun replaces the old one; what you bought this round is refunded when you swap it). B or Esc closes. Your coach called <b>${esc(prettyCall((side === 'T' ? this.setup.t : this.setup.ct).buy))}</b> for the team.</p>
      <div class="buygrid">${col('Pistols', ['glock', 'usp', 'p250', 'deagle'])}${col('SMGs', ['mac10', 'mp9'])}${col('Rifles', ['galil', 'famas', 'ak47', 'm4a1s'])}${col('Sniper', ['awp'])}
        <div><h3>Gear</h3>${gear.map(([id, nm, c]) => `<button type="button" data-g="${id}">${nm} <span>${money(c)}</span></button>`).join('')}
        <h3 style="margin-top:8px">Grenades</h3>${NADES.map(g => `<button type="button" data-n="${g}">${NADE_NAME[g]} <span>${money(nadeCost(g, side))}</span></button>`).join('')}</div></div>
      <div class="row"><button type="button" id="bAuto">Auto-buy (rifle, armor, utility)</button><button type="button" id="bSell">Undo this round's buys</button><span class="muted">Carrying: ${esc([me.primary && WEAPONS[me.primary].name, WEAPONS[me.pistol].name, me.armor ? (me.helmet ? 'kevlar+helmet' : 'kevlar') : '', me.kit ? 'kit' : '', ...me.nades.map(x => NADE_NAME[x])].filter(Boolean).join(' · '))}</span></div>`;
    $('buyCard').querySelectorAll('[data-w]').forEach(b => b.onclick = () => this.buyWeapon(b.dataset.w));
    $('buyCard').querySelectorAll('[data-g]').forEach(b => b.onclick = () => this.buyGear(b.dataset.g));
    $('buyCard').querySelectorAll('[data-n]').forEach(b => b.onclick = () => this.buyNade(b.dataset.n));
    $('bAuto').onclick = () => { this.autoBuy(false); this.paintBuy(); };
    $('bSell').onclick = () => { this.undoBuys(); this.paintBuy(); };
  }
  pay(c, undo) { const me = this.R.human; if (me.money < c) { this.toast('Not enough money'); return false; } me.money -= c; this.bought.push({ c, undo }); return true; }
  buyWeapon(id) {
    const me = this.R.human, w = WEAPONS[id]; if (!this.canBuy()) return;
    const slot = w.cls === 'pistol' ? 'pistol' : 'primary', prev = me[slot];
    if (prev === id) return;
    // refund a gun bought this round that this one replaces
    const i = this.bought.findIndex(b => b.slot === slot);
    let refund = 0; if (i >= 0) { refund = this.bought[i].c; }
    if (me.money + refund < w.cost) { this.toast('Not enough money'); return; }
    if (i >= 0) { me.money += refund; me[slot] = this.bought[i].prev; this.bought.splice(i, 1); }
    const before = me[slot];
    me.money -= w.cost; me[slot] = id; me.ammo[id] = [w.mag, w.res];
    this.bought.push({ c: w.cost, slot, prev: before, undo: () => { me[slot] = before; } });
    this.R.switchTo(me, slot);
    this.afterBuy();
  }
  buyGear(g) {
    const me = this.R.human; if (!this.canBuy()) return;
    if (g === 'kevlar' && me.armor < 100 && this.pay(650, () => { me.armor = 0; })) me.armor = 100;
    else if (g === 'helmet' && !me.helmet) { const had = me.armor > 0; if (this.pay(had ? 350 : 1000, () => { me.helmet = false; if (!had) me.armor = 0; })) { me.armor = 100; me.helmet = true; } }
    else if (g === 'kit' && me.side === 'CT' && !me.kit && this.pay(400, () => { me.kit = false; })) me.kit = true;
    this.afterBuy();
  }
  buyNade(n) {
    const me = this.R.human; if (!this.canBuy()) return;
    const c = me.nades.filter(x => x === n).length;
    if (me.nades.length >= 4 || c >= (n === 'flash' ? 2 : 1)) { this.toast('No room for that grenade'); return; }
    if (this.pay(nadeCost(n, me.side), () => { const i = me.nades.lastIndexOf(n); if (i >= 0) me.nades.splice(i, 1); })) me.nades.push(n);
    this.afterBuy();
  }
  undoBuys() { const me = this.R.human; for (const b of this.bought.reverse()) { me.money += b.c; b.undo && b.undo(); } this.bought = []; if (me.slot === 'primary' && !me.primary) this.R.switchTo(me, 'pistol'); this.afterBuy(); }
  autoBuy(quiet) {
    const me = this.R.human, T = me.side === 'T', rifle = T ? 'ak47' : 'm4a1s', cheap = T ? 'galil' : 'famas', smg = T ? 'mac10' : 'mp9';
    if (!me.primary) { for (const w of [rifle, cheap, smg]) if (me.money >= WEAPONS[w].cost + 650) { this.buyWeapon(w); break; } }
    if (me.money >= 1000 && !me.helmet) this.buyGear('helmet'); else if (me.money >= 650 && !me.armor) this.buyGear('kevlar');
    if (!T && me.money >= 400 && !me.kit && me.primary) this.buyGear('kit');
    for (const n of ['smoke', 'flash', T ? 'molotov' : 'he']) if (me.money >= nadeCost(n, me.side) && me.primary) this.buyNade(n);
    if (!quiet) this.toast('Auto-bought');
  }
  afterBuy() { this.loadout = this.snapshot(this.R.human); if (!$('buy').hidden) this.paintBuy(); }

  showBoard(on) {
    $('board').hidden = !on;
    if (!on || !this.meta) return;
    const R = this.R, row = (p, ti) => { const s = this.stats[p.id] || {}, pl = R && R.byId[p.id]; return `<tr class="${p.id === this.humanId ? 'me' : ''} ${pl && !pl.alive ? 'dead' : ''}"><td>${esc(p.handle)}${p.id === this.humanId ? ' (you)' : ''}</td><td>${esc(pl ? pl.bot : '')}</td><td>${s.k || 0}</td><td>${s.d || 0}</td><td>${Math.round((s.dmg || 0) / Math.max(1, (this.setup ? this.setup.round - 1 : 0) + (R && R.over ? 1 : 0)))}</td><td>${pl ? (pl.alive ? pl.hp + ' HP' : '✕') : ''}</td><td>${pl && ti === this.team ? money(pl.money) : ''}</td></tr>`; };
    $('boardCard').innerHTML = `<h1>${esc(this.names[0])} ${this.score[0]} – ${this.score[1]} ${esc(this.names[1])}</h1>` + this.meta.teams.map((t, ti) => `<h2 style="color:${t.color}">${esc(t.name)}</h2><table class="sb"><tr><th>Player</th><th>Role</th><th>K</th><th>D</th><th>ADR</th><th></th><th></th></tr>${t.players.map(p => row(p, ti)).join('')}</table>`).join('');
  }

  // ---- el bucle ----
  loop(now) {
    const dt = Math.min(0.1, (now - this.last) / 1000); this.last = now;
    this.update(dt);
    this.draw(dt);
    requestAnimationFrame(t => this.loop(t));
  }

  // Avança el joc dt segons (passos fixos de TICK).
  update(dt) {
    if (this.state === 'round' && this.R) {
      this.acc += dt;
      let n = 0;
      while (this.acc >= TICK && n++ < 6) {
        const before = this.R.kills.length;
        this.R.step(TICK, this.input());
        if (this.R.kills.length > before) this.onKills(this.R.kills.slice(before));
        this.acc -= TICK;
      }
      if (this.R.t > BUY_TIME && !$('buy').hidden) this.toggleBuy(false);
      if (this.R.over && !this.sent) this.finishRound();
      if (this.R.over && this.R.t - this.R.endT > POST_TIME && this.resultDone) { this.resultDone = false; if (this.final) this.showFinal(); else this.nextRound(); }
    }
  }

  onKills(ks) {
    for (const k of ks) {
      const s = this.stats[k.killer], v = this.stats[k.victim];
      if (s && k.killer !== k.victim) s.k++;
      if (v) v.d++;
      this.feed.push({ ...k, at: performance.now() });
      if (k.killer === this.humanId && k.victim !== this.humanId) this.hits.push({ at: performance.now(), kill: true, hs: k.hs });
    }
  }

  async finishRound() {
    this.sent = true;
    const R = this.R, me = R.human;
    for (const p of R.players) if (this.stats[p.id]) this.stats[p.id].dmg += p.dmg;
    const win = R.winner, winTeam = win === 'T' ? this.setup.tTeam : 1 - this.setup.tTeam;
    const oppCall = this.mine === 'T' ? this.setup.ct.setup : this.setup.t.strategy;
    this.banner = { side: win, team: winTeam, reason: R.reason, opp: oppCall, at: performance.now() };
    try {
      const r = await call({ action: 'result', id: this.id, result: R.result(this.loadout) });
      this.score = r.score; this.banner.score = r.score;
      if (r.done) { this.final = { winner: r.winner, score: r.score }; }
      else this.planReq = call({ action: 'plan', id: this.id }); // the next call is ready by the end of the post-round
      void me;
    } catch (e) { this.toast('Could not save the round: ' + e.message, 6); }
    this.resultDone = true;
  }

  showFinal() {
    this.state = 'over';
    document.exitPointerLock && document.exitPointerLock();
    const f = this.final, won = f.winner === this.team;
    const rows = Object.entries(this.stats).sort((a, b) => b[1].k - a[1].k).slice(0, 10).map(([id, s]) => `<tr class="${id === this.humanId ? 'me' : ''}"><td>${esc(s.handle)}${id === this.humanId ? ' (you)' : ''}</td><td>${s.k}</td><td>${s.d}</td></tr>`).join('');
    $('msgCard').innerHTML = `<h1>${won ? '🏆 You win' : 'You lose'} ${f.score[0]} – ${f.score[1]}</h1>
      <p class="sub">${esc(this.names[0])} vs ${esc(this.names[1])}. The match is saved in the Arena as a live match (not ranked): replay it in the match viewer, with the tactical map and every player's POV.</p>
      <table class="sb"><tr><th>Player</th><th>K</th><th>D</th></tr>${rows}</table>
      <div class="row" style="margin-top:14px"><button type="button" class="primary" id="fReplay">▶ Watch the replay</button><button type="button" id="fAgain">Play again</button><button type="button" id="fBack">Back to the Arena</button></div>`;
    $('msg').hidden = false;
    $('fReplay').onclick = () => parent.postMessage({ arenaOpen: this.id }, location.origin);
    $('fBack').onclick = () => parent.postMessage({ arenaTV: 'close' }, location.origin);
    $('fAgain').onclick = () => location.reload();
  }

  nextSpec() {
    const R = this.R; if (!R) return;
    const mates = R.players.filter(p => p.alive && p.team === this.team && p.id !== this.humanId);
    if (!mates.length) return;
    const i = mates.findIndex(p => p.id === this.spec);
    this.spec = mates[(i + 1) % mates.length].id;
  }

  // ---- dibuix ----
  draw(dt) {
    const g = this.r.ctx, W = this.r.W, H = this.r.H, u = this.r.u;
    g.fillStyle = '#070a10'; g.fillRect(0, 0, W, H);
    if (!this.R) {
      if (this.state === 'loading') { g.fillStyle = '#c5d0e3'; g.font = `700 ${18 * u}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(this.loadingMsg || '', W / 2, H / 2); this.drawPlanCard(null); }
      return;
    }
    const R = this.R; let me = R.human;
    let view = me;
    if (!me.alive) {
      if (!this.spec || !R.byId[this.spec].alive) { this.spec = null; this.nextSpec(); }
      if (this.spec && R.t - me.deathT > 2) view = R.byId[this.spec];
    }
    this.drawPov(R, view, dt);
    this.drawHud(R, view);
  }

  drawPov(R, pl, dt) {
    const r = this.r, g = r.ctx, W = r.W, H = r.H;
    const moved = Math.hypot(pl.vx, pl.vy);
    this.cam.spd = lerp(this.cam.spd, clamp(moved / 4.5, 0, 1), Math.min(1, dt * 6));
    this.cam.bob += moved * dt * 1.7;
    const scoped = pl.alive && pl.scoped;
    const eye = !pl.alive ? lerp(EYE, 0.35, clamp((R.t - pl.deathT) / 0.6, 0, 1)) : R.eyeOf(pl) + (this.cam.spd > 0.1 ? Math.sin(this.cam.bob) * 0.03 * this.cam.spd : 0);
    const focal = Math.max(W * 0.5, H * 0.72) * (scoped ? 2.6 : 1);
    let shakeX = 0, shakeY = 0;
    if (R.bomb.exploded) { if (!r.povBoomT) r.povBoomT = performance.now(); const a = (performance.now() - r.povBoomT) / 1000; if (a < 1.2) { const m = (1 - a / 1.2) * 9 * r.u; shakeX = (Math.random() - 0.5) * m; shakeY = (Math.random() - 0.5) * m; } }
    const pitch = pl.human && !R.opts.humanBot ? pl.pitch + pl.punch * 0.5 : 0;
    const C = { x: pl.x, y: pl.y, ang: pl.ang, eye, focal, cx: W / 2 + shakeX, horizon: H / 2 + Math.tan(pitch) * focal + shakeY };
    this.C = C;
    r.povSky(C, 0, 0, W, H);
    const Z = r.povWalls(this.G, C, 0, 0, W, H);
    this.drawSprites(R, C, Z, pl);
    // l'arma (o la bomba a les mans)
    let muzzle = null;
    const last = R.shots.filter(s => s.from === pl.id).pop(), age = last ? R.t - last.t : 9;
    if (pl.alive && !scoped) {
      const planting = R.bomb.planting === pl, defusing = R.bomb.defusing === pl;
      const bomb = { planting: planting ? { prog: (R.t - R.bomb.plantStart) / PLANT_TIME } : null, defusing: defusing ? { prog: (R.t - R.bomb.defuseStart) / (pl.kit ? KIT_TIME : BARE_TIME) } : null };
      const kick = age < 0.16 ? 1 - age / 0.16 : 0, own = age < 0.07 ? { age, k: { n: Math.floor(last.t * 60) } } : null;
      if (pl.slot === 'knife' && !planting && !defusing) this.drawKnife(pl, kick);
      else if (pl.slot === 'nade' && !planting && !defusing) this.drawNade(pl);
      else muzzle = r.povWeapon({ side: pl.side, weapon: R.weaponId(pl) }, this.cam, kick, own, planting, defusing, bomb, 0, 0, W, H);
    }
    this.drawTracers(R, C, pl, muzzle);
    if (scoped) r.povScope(0, 0, W, H);
    // fum on ets, flaix als ulls, l'explosió
    let grey = 0;
    for (const s of R.smokes()) { const d = Math.hypot(s.x - pl.x, s.y - pl.y), rr = R.smokeR(s); if (d < rr) grey = Math.max(grey, clamp(1.2 - d / rr, 0, 1) * 0.92); }
    if (grey > 0) { g.fillStyle = `rgba(196,200,208,${grey})`; g.fillRect(0, 0, W, H); }
    if (R.bomb.exploded && r.povBoomT) { const a = (performance.now() - r.povBoomT) / 1000, near = clamp(1 - Math.hypot(R.bomb.x - pl.x, R.bomb.y - pl.y) / 45, 0, 1); if (a < 1.6) { g.fillStyle = `rgba(255,170,90,${0.8 * near * (1 - a / 1.6)})`; g.fillRect(0, 0, W, H); } }
    if (pl.alive && R.t < pl.flashUntil) { const left = pl.flashUntil - R.t; g.fillStyle = `rgba(255,255,255,${clamp(left / 1.2, 0, 1) * clamp(0.55 + pl.flashK, 0, 0.98)})`; g.fillRect(0, 0, W, H); }
    if (!pl.alive) { g.fillStyle = `rgba(120,0,0,${0.35 * clamp((R.t - pl.deathT) / 0.6, 0, 1)})`; g.fillRect(0, 0, W, H); }
    if (pl.alive) this.drawCrosshair(R, pl, scoped);
  }

  drawSprites(R, C, Z, me) {
    const r = this.r, g = r.ctx, items = [];
    for (const p of R.players) if (p !== me) items.push({ kind: 'player', p, x: p.x, y: p.y });
    let n = 0;
    for (const u of R.utils) {
      n++;
      const age = R.t - u.t;
      if (u.type === 'smoke' && age < 18) items.push({ kind: 'util', u: { n, type: 'smoke' }, st: { kind: 'smoke', x: u.x, y: u.y, r: R.smokeR(u) * 1.1, a: 0.9 * clamp((18 - age) / 2.5, 0, 1) }, x: u.x, y: u.y });
      else if (u.type === 'molotov' && age < 7 && !u.out) items.push({ kind: 'util', u: { n, type: 'molotov' }, st: { kind: 'molotov', x: u.x, y: u.y, r: 2.6, a: clamp((7 - age) / 1.5, 0, 1) }, x: u.x, y: u.y });
      else if ((u.type === 'flash' || u.type === 'he') && age < 0.35) items.push({ kind: 'util', u: { n, type: u.type }, st: { kind: u.type, x: u.x, y: u.y, r: u.type === 'he' ? 6 : 5, a: 1 - age / 0.35 }, x: u.x, y: u.y });
    }
    for (const f of R.flying) { const k = clamp((R.t - f.t0) / (f.t - f.t0), 0, 1); items.push({ kind: 'util', u: { n: 0, type: f.type }, st: { kind: 'flight', x: lerp(f.fx, f.x, k), y: lerp(f.fy, f.y, k), h: Math.sin(Math.PI * k) }, x: lerp(f.fx, f.x, k), y: lerp(f.fy, f.y, k) }); }
    const b = R.bomb;
    if (b.planted || b.dropped) items.push({ kind: 'bomb', x: b.x, y: b.y, planted: b.planted, b: { exploded: b.exploded, defused: b.defused, left: b.plantT + BOMB_TIME - R.t } });
    for (const s of r.siteSigns(this.G)) items.push({ kind: 'sign', site: s.site, x: s.x, y: s.y });
    const vis = [];
    for (const it of items) { it.pr = projectPoint(C, it.x, it.y, 0); if (!it.pr.behind || (it.kind === 'util' && it.st.kind === 'smoke')) vis.push(it); }
    vis.sort((a, c) => c.pr.depth - a.pr.depth);
    for (const it of vis) {
      if (it.kind === 'player') this.drawFigure(R, C, Z, it, me);
      else if (it.kind === 'util') r.povUtil(C, Z, it);
      else if (it.kind === 'bomb') r.povBomb(C, Z, it);
      else if (it.kind === 'sign') {
        const pr = projectPoint(C, it.x, it.y, 2.6); if (pr.behind || pr.depth > 70) continue;
        const k = C.focal / pr.depth, fs = 2.2 * k;
        if (!r.clipRuns(Z, pr.x - fs, pr.x + fs, pr.depth - 0.6)) continue;
        g.save(); g.clip(); g.globalAlpha = clamp(1 - pr.depth / 80, 0.3, 0.9);
        g.strokeStyle = 'rgba(150,30,20,.9)'; g.lineWidth = Math.max(1, 0.14 * k); g.beginPath(); g.arc(pr.x, pr.y, 1.35 * k, 0, Math.PI * 2); g.stroke();
        g.fillStyle = 'rgba(165,35,22,.92)'; g.font = `900 ${fs}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(it.site, pr.x, pr.y + 0.08 * k);
        g.restore();
      }
    }
  }

  drawFigure(R, C, Z, it, me) {
    const r = this.r, g = r.ctx, q = it.p, pr = it.pr, u = r.u;
    if (pr.depth > 75 || pr.depth < 0.6) return;
    const k = C.focal / pr.depth, H = (q.crouch ? 1.3 : BODY_H) * k, foot = C.horizon + C.eye * k, x = pr.x;
    const col = r.teamColor(q.team), mate = q.team === me.team;
    const lat = Math.cos(q.ang) * -Math.sin(C.ang) + Math.sin(q.ang) * Math.cos(C.ang);
    const walk = q.alive && Math.hypot(q.vx, q.vy) > 0.5 ? Math.sin(R.t * 9 + q.id.charCodeAt(1)) : 0;
    const fall = q.alive ? 0 : clamp((R.t - q.deathT) / 0.5, 0, 1);
    // els companys es veuen a través de les parets (com al joc); els rivals, no
    if (mate && q.alive && pr.depth < 60) { g.save(); g.globalAlpha = 0.22; r.figure(x, foot, H, q, lat, walk, 0, 0, col, true); g.restore(); }
    let tip = null;
    if (r.clipRuns(Z, x - H * 0.7, x + H * 0.7, pr.depth)) {
      g.save(); g.clip(); if (!q.alive && fall >= 1) g.globalAlpha = 0.85;
      tip = r.figure(x, foot, H, q, lat, walk, fall, q.id.charCodeAt(1) % 2 ? -1 : 1, col, false);
      g.restore();
    }
    const last = R.shots.filter(s => s.from === q.id).pop();
    if (last && R.t - last.t < 0.06 && tip) r.flashStar(tip.x, tip.y, Math.max(6 * u, 0.5 * k), Math.floor(last.t * 60));
    if (mate && q.alive && pr.depth < 70) {
      const fs = clamp(0.3 * k, 9 * u, 13 * u), ty = foot - H - 0.25 * k - 2 * u;
      g.font = `800 ${fs}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'bottom';
      g.lineWidth = 3 * r.dpr; g.strokeStyle = 'rgba(0,0,0,.75)'; g.strokeText(q.handle, x, ty); g.fillStyle = col; g.fillText(q.handle, x, ty);
    }
  }

  drawTracers(R, C, me, muzzle) {
    const g = this.r.ctx, d = this.r.dpr;
    for (const s of R.shots) {
      const age = R.t - s.t; if (age > 0.12) continue;
      const a = 1 - age / 0.12;
      let A = null;
      if (s.from === me.id) A = muzzle || { x: C.cx, y: C.horizon + 0.2 * this.r.H };
      else { const pa = projectPoint(C, s.ax, s.ay, 1.38); if (!pa.behind) A = pa; }
      const B = projectPoint(C, s.bx, s.by, s.bz);
      if (!A || B.behind) continue;
      g.strokeStyle = `rgba(255,220,140,${0.8 * a})`; g.lineWidth = 1.6 * d; g.beginPath(); g.moveTo(A.x, A.y); g.lineTo(B.x, B.y); g.stroke();
      if (s.hit) { g.fillStyle = `rgba(200,30,30,${a})`; g.beginPath(); g.arc(B.x, B.y, Math.max(2 * d, 0.12 * C.focal / Math.max(1, B.depth)), 0, Math.PI * 2); g.fill(); }
      else { g.fillStyle = `rgba(60,50,40,${a})`; g.fillRect(B.x - d, B.y - d, 2 * d, 2 * d); }
    }
  }

  drawKnife(pl, kick) {
    const r = this.r, g = r.ctx, W = r.W, H = r.H, S = Math.min(H, W * 0.75) / 235;
    const bob = Math.sin(this.cam.bob) * 6 * S * this.cam.spd;
    g.save(); g.translate(W * 0.74 + bob, H - 40 * S - kick * 30 * S); g.rotate(-0.5 - kick * 0.6); g.scale(S, S);
    g.fillStyle = pl.side === 'T' ? '#7a6446' : '#33476a'; g.beginPath(); g.moveTo(-20, 30); g.lineTo(30, 30); g.lineTo(90, 160); g.lineTo(-30, 160); g.fill();
    g.fillStyle = '#1d1e21'; g.fillRect(-10, -8, 22, 46);
    g.fillStyle = '#c9ced6'; g.beginPath(); g.moveTo(-8, -8); g.lineTo(10, -8); g.lineTo(4, -90); g.lineTo(-6, -70); g.closePath(); g.fill();
    g.fillStyle = '#e8ecf2'; g.fillRect(-2, -80, 3, 70);
    g.restore();
  }

  drawNade(pl) {
    const r = this.r, g = r.ctx, W = r.W, H = r.H, S = Math.min(H, W * 0.75) / 235, ns = this.nadeList(), n = ns[this.nadeSel % Math.max(1, ns.length)];
    const col = { smoke: '#9aa3ad', flash: '#d8dde4', molotov: '#b8602e', he: '#5d7a3d' }[n] || '#888';
    g.save(); g.translate(W * 0.72, H - 30 * S); g.scale(S, S);
    g.fillStyle = pl.side === 'T' ? '#7a6446' : '#33476a'; g.beginPath(); g.moveTo(-10, 20); g.lineTo(40, 20); g.lineTo(80, 140); g.lineTo(-30, 140); g.fill();
    g.fillStyle = col; g.beginPath(); g.roundRect(-14, -46, 30, 52, 10); g.fill();
    g.fillStyle = '#2a2c30'; g.fillRect(-8, -56, 18, 12);
    g.fillStyle = '#3b3226'; g.beginPath(); g.ellipse(4, 4, 22, 16, 0, 0, Math.PI * 2); g.fill();
    g.restore();
  }

  drawCrosshair(R, pl, scoped) {
    const g = this.r.ctx, u = this.r.u, cx = this.r.W / 2, cy = this.r.H / 2;
    if (!scoped) {
      const sp = R.spreadOf(pl), gap = (3 + Math.min(18, sp * 260)) * u, len = 7 * u, th = 2 * u;
      for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
        const x = dx ? cx + dx * (gap + len / 2) : cx, y = dy ? cy + dy * (gap + len / 2) : cy, ww = dx ? len : th, hh = dx ? th : len;
        g.fillStyle = 'rgba(0,0,0,.7)'; g.fillRect(x - ww / 2 - u, y - hh / 2 - u, ww + 2 * u, hh + 2 * u);
        g.fillStyle = '#4dff6a'; g.fillRect(x - ww / 2, y - hh / 2, ww, hh);
      }
    }
    // hit marker (els meus impactes)
    const s = R.shots.filter(z => z.from === pl.id && z.hit).pop();
    const kh = this.hits.filter(h => performance.now() - h.at < 300).pop();
    if ((s && R.t - s.t < 0.15) || kh) {
      const a = kh ? 1 - (performance.now() - kh.at) / 300 : 1 - (R.t - s.t) / 0.15, r0 = 6 * u, r1 = 12 * u;
      g.strokeStyle = (kh && kh.hs) || (s && s.hs) ? `rgba(255,70,60,${a})` : `rgba(255,255,255,${a})`; g.lineWidth = 2.2 * u; g.beginPath();
      for (const [dx, dy] of [[1, 1], [-1, 1], [1, -1], [-1, -1]]) { g.moveTo(cx + dx * r0, cy + dy * r0); g.lineTo(cx + dx * r1, cy + dy * r1); }
      g.stroke();
    }
  }

  // ---- HUD ----
  pill(x, y, w, h, fill) { const g = this.r.ctx; g.fillStyle = fill; g.beginPath(); g.roundRect(x, y, w, h, Math.min(h / 2, 10 * this.r.u)); g.fill(); }

  drawHud(R, view) {
    const g = this.r.ctx, u = this.r.u, W = this.r.W, H = this.r.H, me = R.human, S = this.setup;
    g.textBaseline = 'middle';
    // marcador i rellotge
    const tc = this.meta.teams, sw = 380 * u, sx = W / 2 - sw / 2, sy = 8 * u;
    this.pill(sx, sy, sw, 44 * u, 'rgba(6,9,14,.82)');
    g.font = `800 ${13 * u}px ${FONT}`; g.textAlign = 'right'; g.fillStyle = tc[0].color; g.fillText(this.names[0], W / 2 - 62 * u, sy + 15 * u, 110 * u);
    g.textAlign = 'left'; g.fillStyle = tc[1].color; g.fillText(this.names[1], W / 2 + 62 * u, sy + 15 * u, 110 * u);
    g.font = `900 ${22 * u}px ${FONT}`; g.fillStyle = '#fff'; g.textAlign = 'center';
    g.fillText(String(this.score[0]), W / 2 - 40 * u, sy + 22 * u); g.fillText(String(this.score[1]), W / 2 + 40 * u, sy + 22 * u);
    const b = R.bomb;
    let clock;
    if (R.t < 0) clock = '0:' + String(Math.ceil(-R.t)).padStart(2, '0');
    else if (b.planted && !b.defused && !b.exploded) clock = '💣';
    else clock = fmtClock(Math.max(0, ROUND_TIME - (R.over ? R.endT : R.t)));
    g.font = `900 ${16 * u}px ${FONT}`; g.fillStyle = b.planted && !R.over ? '#ff5a4e' : R.t < 0 ? '#ffd34d' : '#fff'; g.fillText(clock, W / 2, sy + 16 * u);
    g.font = `700 ${9.5 * u}px ${FONT}`; g.fillStyle = 'rgba(255,255,255,.6)'; g.fillText(R.t < 0 ? `BUY · ROUND ${S.round}` : `ROUND ${S.round} · FIRST TO ${S.target}`, W / 2, sy + 33 * u);
    // vius de cada equip
    for (let t = 0; t < 2; t++) {
      const ps = R.players.filter(p => p.team === t);
      ps.forEach((p, i) => { const x = t === 0 ? W / 2 - 70 * u - i * 11 * u : W / 2 + 70 * u + i * 11 * u; g.fillStyle = p.alive ? tc[t].color : 'rgba(255,255,255,.18)'; g.fillRect(x - 4 * u, sy + 27 * u, 8 * u, 12 * u); });
    }
    // feed de morts
    const now = performance.now();
    this.feed = this.feed.filter(f => now - f.at < 6000);
    this.feed.slice(-5).forEach((f, i) => {
      const k = R.byId[f.killer] || this.statsP(f.killer), v = R.byId[f.victim] || this.statsP(f.victim);
      const txt = f.killer === f.victim ? `✕ ${v.handle}` : `${k.handle}  ▸ ${(WEAPONS[f.weapon] || {}).name || f.weapon}${f.hs ? ' ◎' : ''} ▸  ${v.handle}`;
      g.font = `700 ${12 * u}px ${FONT}`; const tw = g.measureText(txt).width + 20 * u, y = 62 * u + i * 26 * u;
      this.pill(W - tw - 12 * u, y, tw, 22 * u, f.killer === this.humanId ? 'rgba(160,30,30,.85)' : 'rgba(6,9,14,.75)');
      g.fillStyle = '#fff'; g.textAlign = 'right'; g.fillText(txt, W - 22 * u, y + 11 * u);
    });
    // radar
    const rw = Math.min(W * 0.2, 190 * u);
    this.drawRadar(R, view, 12 * u, 12 * u, rw);
    // el pla de l'equip i què fa cada bot
    this.drawPlanCard(R, 12 * u, 12 * u + rw * 0.8 + 14 * u);
    // vida, armilla, diners, munició
    const bot = H - 14 * u;
    if (view.alive) {
      this.pill(12 * u, bot - 36 * u, 190 * u, 36 * u, 'rgba(6,9,14,.7)');
      g.textAlign = 'left'; g.font = `900 ${20 * u}px ${FONT}`; g.fillStyle = view.hp < 30 ? '#ff5a4e' : '#fff';
      g.fillText('✚ ' + Math.max(0, Math.ceil(view.hp)), 24 * u, bot - 18 * u);
      g.fillStyle = 'rgba(255,255,255,.85)'; g.fillText('⛨ ' + Math.round(view.armor) + (view.helmet ? '' : ''), 110 * u, bot - 18 * u);
      const id = R.weaponId(view), w = WEAPONS[id], am = view.ammo[id];
      this.pill(W - 200 * u, bot - 36 * u, 188 * u, 36 * u, 'rgba(6,9,14,.7)');
      g.textAlign = 'right'; g.font = `900 ${20 * u}px ${FONT}`; g.fillStyle = am && am[0] <= Math.ceil((w.mag || 1) * 0.2) ? '#ff5a4e' : '#fff';
      const label = view.slot === 'nade' ? (NADE_NAME[this.nadeList()[this.nadeSel % Math.max(1, this.nadeList().length)]] || '') : am ? `${am[0]} / ${am[1]}` : '';
      g.fillText(view.reloadUntil ? 'RELOADING' : label, W - 24 * u, bot - 18 * u);
      g.font = `700 ${10 * u}px ${FONT}`; g.fillStyle = 'rgba(255,255,255,.65)';
      g.fillText((view.slot === 'nade' ? 'GRENADE' : w.name.toUpperCase()) + (view.nades.length ? '   ' + view.nades.map(n => ({ smoke: '◌', flash: '✦', molotov: '🔥', he: '●' })[n]).join(' ') : ''), W - 24 * u, bot - 46 * u);
    }
    if (view === me && me.alive) {
      g.textAlign = 'left'; g.font = `800 ${15 * u}px ${FONT}`; g.fillStyle = '#7fe39a'; g.fillText(money(me.money), 24 * u, bot - 50 * u);
      if (me.kit) { g.fillStyle = '#5aa9e6'; g.fillText('✂ kit', 110 * u, bot - 50 * u); }
      if (R.bomb.carrier === me.id) { g.fillStyle = '#ffb347'; g.fillText('💣 you have the bomb', 110 * u, bot - 50 * u); }
    }
    // mensajes del centro: plantar/desactivar, buy, autopilot
    let hint = '';
    if (me.alive && !R.over) {
      if (R.bomb.carrier === me.id && R.inSite(me.x, me.y) && R.t >= 0) hint = 'Hold E to plant';
      if (me.side === 'CT' && b.planted && !b.defused && Math.hypot(me.x - b.x, me.y - b.y) < 1.8) hint = `Hold E to defuse (${me.kit ? 5 : 10} s)`;
      if (b.planting === me || b.defusing === me) {
        const prog = b.planting === me ? (R.t - b.plantStart) / PLANT_TIME : (R.t - b.defuseStart) / (me.kit ? KIT_TIME : BARE_TIME);
        this.pill(W / 2 - 120 * u, H * 0.62, 240 * u, 12 * u, 'rgba(6,9,14,.7)'); g.fillStyle = '#ffd34d'; g.fillRect(W / 2 - 118 * u, H * 0.62 + 2 * u, 236 * u * clamp(prog, 0, 1), 8 * u);
        hint = b.planting === me ? 'Planting…' : 'Defusing…';
      }
      if (R.t < 0 && !hint) hint = this.canBuy() ? 'Freeze time · B to buy · click to grab the mouse' : '';
      if (document.pointerLockElement !== this.cv && R.t >= 0 && $('buy').hidden && !hint && !this.autopilot) hint = 'Click to play';
    }
    if (!me.alive && !R.over) hint = `You are dead · watching ${view === me ? '…' : view.handle} (Space: next)`;
    if (this.autopilot && me.alive) hint = (hint ? hint + ' · ' : '') + 'AUTOPILOT (F)';
    if (hint) { g.font = `800 ${14 * u}px ${FONT}`; const tw = g.measureText(hint).width + 26 * u; this.pill(W / 2 - tw / 2, H * 0.66, tw, 26 * u, 'rgba(6,9,14,.75)'); g.fillStyle = '#fff'; g.textAlign = 'center'; g.fillText(hint, W / 2, H * 0.66 + 13 * u); }
    if (this.msg && now < this.msg.until) { g.font = `700 ${13 * u}px ${FONT}`; const tw = g.measureText(this.msg.text).width + 24 * u; this.pill(W / 2 - tw / 2, 62 * u, tw, 24 * u, 'rgba(255,211,77,.92)'); g.fillStyle = '#111'; g.textAlign = 'center'; g.fillText(this.msg.text, W / 2, 74 * u); }
    // la bomba plantada: el compte enrere
    if (b.planted && !R.over) { const left = Math.max(0, b.plantT + BOMB_TIME - R.t); g.font = `900 ${13 * u}px ${FONT}`; g.fillStyle = left < 10 ? '#ff5a4e' : '#ffb347'; g.textAlign = 'center'; g.fillText(`BOMB PLANTED AT ${b.site} · ${left.toFixed(1)} s`, W / 2, sy + 58 * u); }
    if (this.banner) this.drawBanner();
  }

  statsP(id) { const s = this.stats[id]; return { handle: s ? s.handle : id }; }

  drawBanner() {
    const g = this.r.ctx, u = this.r.u, W = this.r.W, H = this.r.H, bn = this.banner;
    const a = clamp((performance.now() - bn.at) / 300, 0, 1), won = bn.team === this.team;
    const y = H * 0.24, h = 84 * u;
    g.globalAlpha = a;
    g.fillStyle = won ? 'rgba(20,90,60,.88)' : 'rgba(110,25,25,.88)'; g.fillRect(0, y, W, h);
    g.fillStyle = '#fff'; g.textAlign = 'center'; g.font = `900 ${26 * u}px ${FONT}`;
    g.fillText(`${bn.side === 'T' ? 'TERRORISTS' : 'COUNTER-TERRORISTS'} WIN · ${(this.names[bn.team] || '').toUpperCase()}`, W / 2, y + 28 * u, W - 40 * u);
    g.font = `700 ${13 * u}px ${FONT}`; g.fillStyle = 'rgba(255,255,255,.85)';
    g.fillText(`${REASON[bn.reason] || bn.reason}${bn.score ? ` · ${bn.score[0]} – ${bn.score[1]}` : ''} · they played ${prettyCall(bn.opp)}`, W / 2, y + 58 * u, W - 40 * u);
    g.globalAlpha = 1;
  }

  drawRadar(R, view, x, y, wR) {
    const r = this.r, g = r.ctx, u = r.u, size = this.meta.size || [100, 80], hR = wR * size[1] / size[0], sc = wR / size[0];
    if (!this.radar || this.radar.width !== Math.round(wR)) {
      const c = document.createElement('canvas'); c.width = Math.round(wR); c.height = Math.round(hR);
      const saved = r.fit; r.fit = () => fitMap(size, { x: 0, y: 0, w: c.width, h: c.height });
      const dpr = r.dpr; r.dpr = 0.5; r.renderMap(c.getContext('2d')); r.dpr = dpr; r.fit = saved;
      this.radar = c;
    }
    g.save();
    g.fillStyle = 'rgba(6,9,14,.75)'; g.beginPath(); g.roundRect(x - 3 * u, y - 3 * u, wR + 6 * u, hR + 6 * u, 6 * u); g.fill();
    g.globalAlpha = 0.92; g.drawImage(this.radar, x, y, wR, hR); g.globalAlpha = 1;
    const mx = x + view.x * sc, my = y + view.y * sc;
    const cone = g.createRadialGradient(mx, my, 0, mx, my, 14 * sc);
    cone.addColorStop(0, 'rgba(255,255,255,.4)'); cone.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = cone; g.beginPath(); g.moveTo(mx, my); g.arc(mx, my, 14 * sc, view.ang - 0.6, view.ang + 0.6); g.closePath(); g.fill();
    const seen = R.spotted[this.team];
    for (const p of R.players) {
      const mate = p.team === this.team;
      let px = p.x, py = p.y;
      if (!mate) { const s = seen[p.id]; const meSees = R.human.alive && R.sees(R.human, p); if (!p.alive || !(meSees || (s && R.t - s.t < 1.5))) continue; if (!meSees) { px = s.x; py = s.y; } }
      const X = x + px * sc, Y = y + py * sc;
      if (!p.alive) { g.strokeStyle = r.teamColor(p.team); g.lineWidth = 1.2 * u; g.beginPath(); g.moveTo(X - 2.5 * u, Y - 2.5 * u); g.lineTo(X + 2.5 * u, Y + 2.5 * u); g.moveTo(X + 2.5 * u, Y - 2.5 * u); g.lineTo(X - 2.5 * u, Y + 2.5 * u); g.stroke(); continue; }
      g.fillStyle = mate ? r.teamColor(p.team) : '#ff4b3e'; g.beginPath(); g.arc(X, Y, (p === view ? 4 : 3) * u, 0, Math.PI * 2); g.fill();
      if (p === view) { g.strokeStyle = '#fff'; g.lineWidth = 1.5 * u; g.stroke(); }
      if (R.bomb.carrier === p.id && mate) { g.fillStyle = '#ffb347'; g.fillRect(X + 3 * u, Y - 6 * u, 4 * u, 4 * u); }
    }
    const b = R.bomb;
    if ((b.planted || b.dropped && this.mine === 'T') && !b.exploded) { g.fillStyle = Math.sin(performance.now() / 150) > 0 ? '#ff4b3e' : '#ffb347'; g.beginPath(); g.arc(x + b.x * sc, y + b.y * sc, 3 * u, 0, Math.PI * 2); g.fill(); }
    g.restore();
  }

  // El pla del coach per a aquesta ronda i cada bot (la seva ordre i què fa).
  drawPlanCard(R, x = 12 * this.r.u, y = 200 * this.r.u) {
    const S = this.setup; if (!S || !R) return;
    const g = this.r.ctx, u = this.r.u, me = R.human, side = me.side, call = side === 'T' ? S.t : S.ct;
    const dec = (this.decisions || [])[0];
    const full = R.t < 0 || this.keys.KeyH;
    const w = Math.min(this.r.W * 0.34, (full ? 330 : 250) * u);
    const lines = [];
    const head = side === 'T' ? `${prettyCall(call.strategy)} → ${S.site}${S.fake ? ` (fake ${S.fake})` : ''}` : `${prettyCall(call.setup)}`;
    const sub = side === 'T' ? `${call.buy} buy · utility ${call.utility} · pace ${call.pace}${R.t < S.hitAt && S.hitAt > 2 ? ` · hit at ${fmtClock(ROUND_TIME - S.hitAt)}` : ''}` : `${call.buy} buy · utility ${call.utility}`;
    const mates = R.players.filter(p => p.team === me.team && p !== me);
    const rowH = 30 * u, h = 52 * u + (full && dec && dec.why ? 16 * u : 0) + mates.length * rowH + (full ? 18 * u : 0);
    g.fillStyle = 'rgba(6,9,14,.72)'; g.beginPath(); g.roundRect(x - 3 * u, y, w + 6 * u, h, 8 * u); g.fill();
    g.textAlign = 'left'; g.textBaseline = 'middle';
    g.font = `800 ${9.5 * u}px ${FONT}`; g.fillStyle = '#ffd34d'; g.fillText(`COACH'S CALL · ${side}${dec && dec.fallback ? ' (default: the coach did not answer)' : ''}`, x + 6 * u, y + 11 * u, w - 10 * u);
    g.font = `900 ${13 * u}px ${FONT}`; g.fillStyle = '#fff'; g.fillText(head, x + 6 * u, y + 27 * u, w - 10 * u);
    g.font = `600 ${10.5 * u}px ${FONT}`; g.fillStyle = 'rgba(255,255,255,.7)'; g.fillText(sub, x + 6 * u, y + 42 * u, w - 10 * u);
    let yy = y + 52 * u;
    if (full && dec && dec.why) { g.font = `italic 600 ${10.5 * u}px ${FONT}`; g.fillStyle = 'rgba(200,215,240,.85)'; g.fillText('“' + dec.why + '”', x + 6 * u, yy + 6 * u, w - 10 * u); yy += 16 * u; }
    for (const p of mates) {
      const col = this.r.teamColor(p.team);
      g.fillStyle = p.alive ? col : 'rgba(255,255,255,.25)'; g.fillRect(x + 4 * u, yy + 5 * u, 3 * u, rowH - 10 * u);
      g.font = `800 ${11.5 * u}px ${FONT}`; g.fillStyle = p.alive ? '#fff' : 'rgba(255,255,255,.4)';
      g.fillText(p.handle, x + 12 * u, yy + 10 * u, 90 * u);
      g.font = `800 ${9 * u}px ${FONT}`; g.fillStyle = '#ffd34d'; g.fillText(p.bot.toUpperCase(), x + 100 * u, yy + 10 * u);
      g.font = `600 ${10 * u}px ${FONT}`; g.fillStyle = 'rgba(255,255,255,.7)';
      const act = R.t < 0 ? (p.note ? p.note : `${p.group || ''} · ${(this.metaZone(p.post) || {}).name || ''}`) : R.doing(p) + (p.alive ? ` · ${Math.ceil(p.hp)} HP` : '');
      g.fillText(act, x + 12 * u, yy + 23 * u, w - 16 * u);
      yy += rowH;
    }
    if (full) { g.font = `600 ${9.5 * u}px ${FONT}`; g.fillStyle = 'rgba(255,255,255,.5)'; g.fillText('Your bots follow this call. Hold H to see this card in play.', x + 6 * u, yy + 8 * u, w - 10 * u); }
  }

  metaZone(id) { return (this.meta.zones || []).find(z => z.id === id); }
}

// ---- engegar ----
const game = new LiveGame($('cv'));
window.gregalLive = game;
$('close').onclick = async () => {
  if (game.id && game.state !== 'over' && !confirm('Leave the match? It is saved as stopped.')) return;
  if (game.id && game.state !== 'over') await call({ action: 'stop', id: game.id }).catch(() => {});
  parent.postMessage({ arenaTV: 'close' }, location.origin);
};
addEventListener('keydown', e => { if (e.code === 'Escape' && !$('buy').hidden) game.toggleBuy(false); });
lobby(game);
