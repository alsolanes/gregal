// El pati dels vents: la colla com a canalla a l'hora del pati. Cada vent
// porta el seu racó segons el tema (winds-pati-zones.js): un vent de bàsquet
// porta la pista de bàsquet, un d'informàtica l'aula d'ordinadors… Els racons
// s'endrecen en dues fileres a banda i banda de la plaça (cercle de guix,
// taller, gronxadors i tobogan) i, si n'hi ha molts, el pati es desplaça de
// costat: arrossegant el terra, amb la roda o amb la barra de racons.
//
// Tot el que es veu de feina és real (tasques del servidor); el joc del pati
// és només joc: frases curtes locals, sense cap crida al model.
(function (root) {
  'use strict';
  const ca = () => document.documentElement.lang === 'ca';
  const t = (a, b) => (ca() ? a : b);
  const el = (tag, cls, text) => { const n = document.createElement(tag); if (cls) n.className = cls; if (text !== undefined) n.textContent = text; return n; };
  const reduced = () => root.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
  const rnd = (a, b) => a + Math.random() * (b - a);
  const pick = list => list[Math.floor(Math.random() * list.length)];
  const lower = s => s.charAt(0).toLowerCase() + s.slice(1);
  const clip = (s, n) => (s.length > n ? s.slice(0, n - 1) + '…' : s);
  const hour = () => { const d = new Date(); return d.getHours() + d.getMinutes() / 60; };
  const night = () => { const h = hour(); return h < 7 || h >= 21; };

  // Mides del món (unitats del dibuix).
  const SKY = 70, TW = 360, TH = 270, PW = 470, H = SKY + TH * 2;
  const DEFAULTS = ['football', 'library', 'garden', 'stage'];

  const ACTS = {
    swing: { mood: 'idle', ca: 'es gronxa', en: 'is on the swings', say: [['Més amunt!', 'Higher!'], ['Uiii!', 'Wheee!']] },
    slide: { mood: 'idle', ca: 'baixa pel tobogan', en: 'goes down the slide', say: [['Allà vaaa!', 'Here I gooo!'], ['Un altre cop!', 'Again!']] },
    chat: { mood: 'discussing', ca: 'xerra', en: 'chats' },
    stroll: { mood: 'idle', ca: 'passeja', en: 'strolls around' },
    work: { mood: 'working', fx: '🔨', ca: 'treballa al taller', en: 'works in the workshop' },
    circle: { mood: 'thinking', ca: 'espera a la rotllana', en: 'waits in the circle' },
    line: { mood: 'idle', ca: 'fa fila', en: 'lines up' },
    sleep: { mood: 'idle', fx: '💤', ca: 'fa una becaina', en: 'takes a nap' },
  };
  const CHAT = [['Hola!', 'Hi!'], ['Què fas?', 'What are you up to?'], ['Jo també!', 'Me too!'], ['Ha, ha!', 'Ha, ha!'], ['Saps què?', 'Guess what?'], ['Vine a veure el meu racó!', 'Come see my corner!']];

  /**
   * Munta el pati dins de `host`. ctx: { winds, tasks, talkTo(w), openChat(rot),
   * openThread(id), call(url, method, body), notify(text), refreshTasks() → tasks }.
   */
  function mount(host, ctx) {
    const Z = root.gregalPatiZones;
    // --- Racons: un per tema (i un de propi per a qui no encaixa) ---
    const byKind = new Map();
    const homeOf = new Map();
    for (const w of ctx.winds) {
      const kind = Z.kindOf(w);
      const key = kind === 'corner' ? 'corner:' + w.id : kind;
      if (!byKind.has(key)) byKind.set(key, { key, kind, def: Z.KINDS[kind], winds: [] });
      byKind.get(key).winds.push(w);
      homeOf.set(w.id, key);
    }
    for (const kind of DEFAULTS) {
      if (byKind.size >= 4) break;
      if (!byKind.has(kind)) byKind.set(kind, { key: kind, kind, def: Z.KINDS[kind], winds: [] });
    }
    const themed = [...byKind.values()].sort((a, b) => b.winds.length - a.winds.length);
    // Dues fileres; la plaça al mig perquè quedi a prop de tot.
    const cols = Math.ceil(themed.length / 2), leftCols = Math.ceil(cols / 2);
    const zones = new Map();
    let x = 0;
    const placeCol = (col) => {
      for (let row = 0; row < 2; row++) {
        const z = themed[col * 2 + row];
        if (!z) continue;
        z.x = x; z.y = SKY + row * TH; z.w = TW; z.h = TH;
        z.label = z.kind === 'corner' ? t('Racó de ', 'Corner of ') + z.winds[0].name : t(z.def.ca, z.def.en);
        z.icon = z.def.icon;
        zones.set(z.key, z);
      }
      x += TW;
    };
    for (let c = 0; c < leftCols; c++) placeCol(c);
    const plaza = { key: 'plaza', kind: 'plaza', x, y: SKY, w: PW, h: TH * 2, icon: '🎪', label: t('La plaça', 'The square'), winds: [] };
    zones.set('plaza', plaza);
    x += PW;
    for (let c = leftCols; c < cols; c++) placeCol(c);
    const W = x;
    // Llocs de la plaça.
    const P = {
      circle: { x: plaza.x + PW / 2, y: SKY + TH * 0.95, rx: 115, ry: 72 },
      workshop: { x: plaza.x + PW - 95, y: SKY + TH * 2 - 52 },
      swings: { x: plaza.x + 95, y: SKY + 118 },
      slide: { x: plaza.x + 92, y: SKY + TH * 2 - 50 },
    };

    // --- Escena ---
    const wrap = el('div', 'pati');
    const tools = el('div', 'pati-tools');
    const bell = el('button', 'pati-tool', t('🔔 Toca la campana', '🔔 Ring the bell'));
    bell.type = 'button';
    const pause = el('button', 'pati-tool', t('⏸ Atura el temps', '⏸ Freeze time'));
    pause.type = 'button';
    const zoomOut = el('button', 'pati-tool', '−');
    zoomOut.type = 'button';
    zoomOut.setAttribute('aria-label', t('Allunya', 'Zoom out'));
    const zoomIn = el('button', 'pati-tool', '+');
    zoomIn.type = 'button';
    zoomIn.setAttribute('aria-label', t('Apropa', 'Zoom in'));
    tools.append(bell, pause, zoomOut, zoomIn);
    const bar = el('nav', 'pati-bar');
    bar.setAttribute('aria-label', t('Racons del pati', 'Playground corners'));
    const scene = el('div', 'pati-scene' + (night() ? ' is-night' : ''));
    scene.setAttribute('role', 'application');
    scene.setAttribute('aria-label', t('El pati dels vents', 'The winds’ playground'));
    const scroller = el('div', 'pati-scroller');
    const sizer = el('div', 'pati-sizer');
    const world = el('div', 'pati-world');
    world.style.width = W + 'px';
    world.style.height = H + 'px';
    world.append(drawWorld(W, zones, plaza, P, Z));
    const layer = el('div', 'pati-layer');
    const castles = el('div', 'pati-castles');
    const sign = el('button', 'pati-sign');
    sign.type = 'button';
    sign.hidden = true;
    sign.style.left = (P.workshop.x - 150) + 'px';
    sign.style.top = (P.workshop.y - 190) + 'px';
    world.append(castles, layer, sign);
    sizer.append(world);
    scroller.append(sizer);
    const sky = el('div', 'pati-sky');
    const sun = el('div', 'pati-sun');
    sky.append(sun, bird(0), bird(1));
    scene.append(scroller, sky);
    const panel = el('div', 'pati-circle-panel');
    panel.hidden = true;
    const card = el('div', 'pati-card');
    card.hidden = true;
    card.setAttribute('role', 'dialog');
    scene.append(panel, card);
    wrap.append(tools, bar, scene);
    host.append(wrap);

    // Zoom: 1 = el pati omple l'alçada; menys, en veus més racons alhora.
    let scale = 1, zoom = 1;
    function fit() {
      // L'alçada de l'escena segueix la finestra; amb zoom enfora,
      // l'escena s'encongeix (no queden franges buides).
      const base = Math.min(720, Math.max(420, root.innerHeight * 0.64));
      scale = Math.max(0.3, (base / H) * zoom);
      scene.style.height = Math.round(Math.min(base, H * scale)) + 'px';
      world.style.transform = `scale(${scale})`;
      sizer.style.width = W * scale + 'px';
      sizer.style.height = H * scale + 'px';
      sky.style.height = SKY * scale + 'px';
      sky.style.top = Math.max(0, (scene.clientHeight - H * scale) / 2) + 'px';
      scene.style.setProperty('--pati-scale', scale);
    }
    fit();
    const ro = new ResizeObserver(() => fit());
    ro.observe(scene);
    // Comença mirant la plaça.
    const centerOn = (z, smooth = true) => scroller.scrollTo({ left: (z.x + z.w / 2) * scale - scroller.clientWidth / 2, behavior: smooth && !reduced() ? 'smooth' : 'auto' });
    requestAnimationFrame(() => centerOn(plaza, false));
    function setZoom(z) {
      const mid = (scroller.scrollLeft + scroller.clientWidth / 2) / scale;
      zoom = Math.min(1.5, Math.max(0.45, z));
      fit();
      scroller.scrollLeft = mid * scale - scroller.clientWidth / 2;
      paintBar();
    }
    zoomOut.onclick = () => setZoom(zoom - 0.2);
    zoomIn.onclick = () => setZoom(zoom + 0.2);

    // --- Barra de racons ---
    const chips = new Map();
    const orderedZones = [plaza, ...[...zones.values()].filter(z => z !== plaza).sort((a, b) => a.x - b.x || a.y - b.y)];
    for (const z of orderedZones) {
      const c = el('button', 'pati-chip');
      c.type = 'button';
      c.append(el('span', 'pati-chip-icon', z.icon), el('span', '', z.label));
      const n = el('span', 'pati-chip-count');
      c.append(n);
      c.onclick = () => centerOn(z);
      bar.append(c);
      chips.set(z.key, { c, n });
    }
    function paintBar() {
      const counts = {};
      for (const a of actors.values()) { const k = zoneAt(a.x, a.y)?.key; if (k) counts[k] = (counts[k] || 0) + 1; }
      const mid = (scroller.scrollLeft + scroller.clientWidth / 2) / scale;
      for (const [k, { c, n }] of chips) {
        n.textContent = counts[k] ? String(counts[k]) : '';
        const z = zones.get(k);
        c.setAttribute('aria-current', String(mid >= z.x && mid < z.x + z.w));
      }
    }
    scroller.addEventListener('scroll', () => paintBar(), { passive: true });
    // La roda mou el pati de costat (si hi ha on anar).
    scroller.addEventListener('wheel', e => {
      if (scroller.scrollWidth <= scroller.clientWidth + 2) return;
      if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) { scroller.scrollLeft += e.deltaY; e.preventDefault(); }
    }, { passive: false });
    // Arrossegar el terra mou el pati.
    let pan = null;
    world.addEventListener('pointerdown', e => {
      if (e.target.closest('.pati-wind, .pati-sign')) return;
      pan = { x: e.clientX, left: scroller.scrollLeft };
      world.setPointerCapture(e.pointerId);
      world.classList.add('is-panning');
    });
    world.addEventListener('pointermove', e => { if (pan) scroller.scrollLeft = pan.left - (e.clientX - pan.x); });
    const endPan = () => { pan = null; world.classList.remove('is-panning'); };
    world.addEventListener('pointerup', endPan);
    world.addEventListener('pointercancel', endPan);

    function zoneAt(x, y) {
      for (const z of zones.values()) if (x >= z.x && x < z.x + z.w && y >= z.y - 20 && y < z.y + z.h + 20) return z;
      return null;
    }

    // --- Vents ---
    const actors = new Map();
    let paused = false, alive = true;
    const realWork = new Map();
    let tasks = ctx.tasks || [];
    const runningTasks = () => tasks.filter(task => task.status === 'running' || task.status === 'started');
    const workOf = w => runningTasks().find(task => Object.values(task.agents || {}).includes(w.name)) || null;

    for (const w of ctx.winds) {
      const home = zones.get(homeOf.get(w.id));
      const def = home.def;
      const node = el('button', 'pati-wind');
      node.type = 'button';
      const shadow = el('span', 'pati-shadow');
      const svg = root.gregalWinds.face(w, false);
      const fx = el('span', 'pati-fx');
      const bubble = el('span', 'pati-bubble');
      bubble.hidden = true;
      const name = el('span', 'pati-name', w.name);
      const doing = el('span', 'pati-doing');
      node.append(bubble, fx, shadow, svg, name, doing);
      layer.append(node);
      const energy = def.energy ?? 0.45;
      const a = { w, node, svg, fx, bubble, doing, home, def, energy, x: 0, y: 0, tx: 0, ty: 0, act: 'stroll', where: home, until: 0, inCircle: false, sayUntil: 0 };
      actors.set(w.id, a);
      const s = spotIn(home, a);
      place(a, s.x, s.y, 0);
      setAct(a, 'home', home);
      a.until = performance.now() + rnd(500, 4000);
      wire(a);
    }

    // Un lloc dins del racó lluny dels altres (no s'amunteguen).
    function spotIn(z, self) {
      const [x0, y0, x1, y1] = z === plaza ? [40, 300, PW - 40, TH * 2 - 30] : (z.def.area || [50, 150, 310, 225]);
      let best = null, bestGap = -1;
      for (let i = 0; i < 10; i++) {
        let p = { x: z.x + rnd(x0, x1), y: z.y + rnd(y0, y1) };
        if (z === plaza) p = outOfCircle(p);
        let gap = Infinity;
        for (const o of actors.values()) if (o !== self) gap = Math.min(gap, Math.hypot(p.x - o.tx, (p.y - o.ty) * 1.6));
        if (gap > bestGap) { best = p; bestGap = gap; }
        if (gap > 70) break;
      }
      return best;
    }
    const inCircle = (x, y, grow = 1) => ((x - P.circle.x) / (P.circle.rx * grow)) ** 2 + ((y - P.circle.y) / (P.circle.ry * grow)) ** 2 <= 1;
    function outOfCircle(p) {
      if (!inCircle(p.x, p.y, 1.15)) return p;
      const ang = Math.atan2((p.y - P.circle.y) / P.circle.ry, (p.x - P.circle.x) / P.circle.rx);
      return { x: P.circle.x + Math.cos(ang) * P.circle.rx * 1.25, y: P.circle.y + Math.sin(ang) * P.circle.ry * 1.25 };
    }

    function place(a, x, y, secs) {
      a.x = x; a.y = y; a.tx = x; a.ty = y;
      a.node.style.transition = secs > 0 ? `left ${secs}s linear, top ${secs}s linear` : 'none';
      a.node.style.left = x + 'px';
      a.node.style.top = y + 'px';
      a.node.style.zIndex = String(Math.round(y));
      a.node.classList.toggle('is-moving', secs > 0);
      a.node.classList.toggle('is-running', secs > 0 && a.energy > 0.7);
      clearTimeout(a.stopTimer);
      if (secs > 0) a.stopTimer = setTimeout(() => a.node.classList.remove('is-moving', 'is-running'), secs * 1000);
    }
    function walkTo(a, x, y) {
      if (!a.inCircle) ({ x, y } = outOfCircle({ x, y }));
      const dx = x - a.x, dist = Math.hypot(dx, (y - a.y) * 0.8);
      const speed = (70 + a.energy * 160) * (night() ? 0.5 : 1);
      a.node.classList.toggle('face-left', dx < -4);
      const secs = Math.max(0.4, dist / speed);
      place(a, x, y, secs);
      return secs;
    }
    // «home» és l'activitat del seu racó (la que defineix el catàleg).
    function setAct(a, act, where) {
      a.act = act; a.where = where || a.where;
      const def = act === 'home' ? a.home.def.act : ACTS[act] || ACTS.stroll;
      root.gregalWinds.mood(a.svg, def.mood);
      a.fx.textContent = def.fx || '';
      a.node.dataset.act = act === 'home' ? 'home-' + (a.home.def.act.move || 'still') : act;
      const placeName = a.where && !['circle', 'line', 'chat', 'work', 'home'].includes(act) ? ' · ' + lower(a.where.label) : '';
      const doing = t(def.ca, def.en) + placeName;
      a.doing.textContent = doing;
      a.node.setAttribute('aria-label', `${a.w.name} — ${doing}`);
    }
    function say(a, text, ms = 2800) {
      a.bubble.textContent = text;
      a.bubble.hidden = false;
      a.sayUntil = performance.now() + ms;
    }
    const homeSay = a => t(...pick(a.home.def.say || CHAT));

    // Què fa cada vent quan acaba el que feia.
    function decide(a, now) {
      if (a.inCircle || a.dragging) return;
      if (realWork.has(a.w.id)) {
        if (a.act !== 'work') {
          const secs = walkTo(a, P.workshop.x + rnd(-70, 40), P.workshop.y + rnd(-14, 12));
          setAct(a, 'work', plaza);
          a.until = now + secs * 1000 + 6000;
          setTimeout(() => alive && a.act === 'work' && say(a, t('Som-hi!', 'On it!')), secs * 1000);
        } else a.until = now + 6000;
        return;
      }
      if (night() && Math.random() < 0.5) { setAct(a, 'sleep', a.where); a.until = now + rnd(9000, 16000); return; }
      const e = a.energy, roll = Math.random();
      if (roll < 0.14) {
        // Va a xerrar amb algú lliure (també d'un altre racó: es visiten).
        const others = [...actors.values()].filter(o => o !== a && !o.inCircle && !realWork.has(o.w.id) && o.act !== 'chat' && !o.dragging);
        if (others.length) {
          const o = Math.random() < 0.6 ? pick(others.filter(x => x.home === a.home).concat(others)) : pick(others);
          const side = a.x < o.x ? -48 : 48;
          const secs = walkTo(a, o.x + side, o.y + rnd(-6, 6));
          setAct(a, 'stroll', zoneAt(o.x, o.y));
          setTimeout(() => {
            if (!alive || a.inCircle || a.dragging) return;
            setAct(a, 'chat', o.where);
            a.node.classList.toggle('face-left', a.x > o.x);
            say(a, t(...pick(CHAT)));
            if (!o.inCircle && !realWork.has(o.w.id) && !o.dragging) {
              setAct(o, 'chat', o.where);
              o.node.classList.toggle('face-left', o.x > a.x);
              setTimeout(() => alive && say(o, homeSay(o)), 1400);
              o.until = performance.now() + 5600;
            }
          }, secs * 1000);
          a.until = now + secs * 1000 + 5600;
          return;
        }
      }
      if (roll < 0.14 + e * 0.12) {
        // A la plaça: gronxadors o tobogan.
        const act = Math.random() < 0.5 ? 'swing' : 'slide', p = P[act === 'swing' ? 'swings' : 'slide'];
        const secs = walkTo(a, p.x + rnd(-24, 24), p.y + rnd(-6, 6));
        setAct(a, 'stroll', plaza);
        setTimeout(() => {
          if (!alive || a.inCircle || a.dragging || realWork.has(a.w.id)) return;
          setAct(a, act, plaza);
          say(a, t(...pick(ACTS[act].say)), 2000);
        }, secs * 1000);
        a.until = now + secs * 1000 + rnd(4000, 7000);
        return;
      }
      if (roll < 0.14 + e * 0.12 + 0.12) {
        // Visita un altre racó (curiositat).
        const z = pick([...zones.values()].filter(z => z !== a.home));
        if (z) {
          const s = spotIn(z, a);
          const secs = walkTo(a, s.x, s.y);
          setAct(a, 'stroll', z);
          a.until = now + secs * 1000 + rnd(3000, 6000);
          return;
        }
      }
      // Torna o es queda al seu racó, fent el seu.
      const back = a.where !== a.home || Math.random() < 0.55;
      const secs = back ? walkTo(a, ...Object.values(spotIn(a.home, a))) : 0;
      setTimeout(() => {
        if (!alive || a.inCircle || a.dragging || realWork.has(a.w.id)) return;
        setAct(a, 'home', a.home);
        if (a.home.kind === 'computers' || a.home.kind === 'corner' || Math.random() < 0.35) { if (Math.random() < 0.45) say(a, homeSay(a)); }
        if (a.home.def.act.move === 'still' && a.home.kind === 'garden') addSprout(a);
      }, secs * 1000);
      a.until = now + secs * 1000 + rnd(5000, 12000) * (1.4 - e);
    }

    // Pilotes: als racons amb pilota, si hi juguen dos o més, se la passen.
    const balls = new Map();
    function tickBalls(now) {
      for (const z of zones.values()) {
        if (!z.def?.ball) continue;
        const players = [...actors.values()].filter(a => a.home === z && a.act === 'home' && !a.node.classList.contains('is-moving'));
        let b = balls.get(z.key);
        if (players.length < 2 || reduced()) { if (b) b.el.hidden = true; continue; }
        if (!b) { b = { el: el('div', 'pati-ball', z.def.ball), owner: null, next: 0 }; world.append(b.el); balls.set(z.key, b); }
        b.el.hidden = false;
        if (now < b.next) continue;
        const from = b.owner && players.includes(b.owner) ? b.owner : players[0];
        const to = pick(players.filter(p => p !== from));
        b.el.animate([
          { left: from.x + 'px', top: from.y - 10 + 'px' },
          { left: (from.x + to.x) / 2 + 'px', top: Math.min(from.y, to.y) - 70 + 'px', offset: 0.5 },
          { left: to.x + 'px', top: to.y - 10 + 'px' },
        ], { duration: 900, easing: 'ease-in-out', fill: 'forwards' });
        if (Math.random() < 0.3) say(from, homeSay(from), 1400);
        b.owner = to;
        b.next = now + rnd(1500, 2600);
      }
    }
    function addSprout(a) {
      if (castles.childElementCount > 10 || Math.random() < 0.5) return;
      const c = el('span', 'pati-castle', pick(['🌱', '🌷', '🌼']));
      c.style.left = a.x + rnd(-30, 30) + 'px';
      c.style.top = a.y + rnd(-8, 8) + 'px';
      castles.append(c);
      setTimeout(() => c.classList.add('fade'), 60000);
      setTimeout(() => c.remove(), 62000);
    }

    // Sol i lluna (travessen el cel segons l'hora) i ocells.
    function paintSky() {
      const h = hour(), isNight = night();
      scene.classList.toggle('is-night', isNight);
      const p = isNight ? ((h + 24 - 21) % 24) / 10 : (h - 7) / 14;
      sun.classList.toggle('moon', isNight);
      sun.style.left = 6 + Math.min(1, Math.max(0, p)) * 86 + '%';
    }
    function bird(i) {
      const b = el('span', 'pati-bird');
      b.style.animationDelay = -i * 9 + 's';
      b.style.top = 14 + i * 22 + '%';
      b.innerHTML = '<svg viewBox="0 0 20 8" aria-hidden="true"><path d="M1 6q4-6 9 0q5-6 9 0" fill="none" stroke="#3B3024" stroke-width="1.6" stroke-linecap="round"/></svg>';
      return b;
    }

    function tick() {
      if (!alive || !scene.isConnected) { stop(); return; }
      if (paused) return;
      const now = performance.now();
      for (const a of actors.values()) {
        if (a.sayUntil && now > a.sayUntil) { a.bubble.hidden = true; a.sayUntil = 0; }
        if (now >= a.until) decide(a, now);
      }
      tickBalls(now);
    }
    const timer = setInterval(tick, 400);
    const barTimer = setInterval(() => alive && paintBar(), 2000);
    paintSky();
    const skyTimer = setInterval(paintSky, 60000);

    // Feina real: qui hi treballa va al taller; el cartell diu quina feina és.
    let prevRunning = new Set();
    async function refreshWork() {
      try { tasks = (await ctx.refreshTasks()) || tasks; } catch (_) { /* el pati continua */ }
      realWork.clear();
      for (const a of actors.values()) {
        const job = workOf(a.w);
        if (job) realWork.set(a.w.id, job);
        else if (a.act === 'work') { root.gregalWinds.mood(a.svg, 'done'); say(a, t('Fet!', 'Done!')); a.until = performance.now() + 2500; a.act = 'stroll'; }
      }
      const running = runningTasks();
      const ids = new Set(running.map(x => x.id));
      for (const id of prevRunning) if (!ids.has(id)) celebrate();
      prevRunning = ids;
      paintSign(running);
    }
    function paintSign(running) {
      sign.hidden = !running.length;
      if (!running.length) return;
      const job = running[0];
      sign.replaceChildren(
        el('strong', '', '🔨 ' + clip(job.task, 46)),
        el('span', '', [Object.values(job.agents || {}).join(', '), job.files?.length ? job.files.length + t(' fitxers', ' files') : '', running.length > 1 ? '+' + (running.length - 1) : ''].filter(Boolean).join(' · ')),
      );
      sign.title = t('Obre el fil de la feina', 'Open the task thread');
      sign.onclick = () => job.thread && ctx.openThread?.(job.thread);
    }
    function celebrate() {
      if (reduced()) return;
      for (let i = 0; i < 7; i++) {
        const b = el('span', 'pati-balloon', pick(['🎈', '🎈', '🎉', '✨']));
        b.style.left = P.workshop.x + rnd(-90, 40) + 'px';
        b.style.top = P.workshop.y - 90 + 'px';
        b.style.animationDelay = i * 0.18 + 's';
        world.append(b);
        setTimeout(() => b.remove(), 4200);
      }
    }
    refreshWork();
    const workTimer = setInterval(() => alive && scene.isConnected && !paused && refreshWork(), 12000);

    function stop() {
      alive = false;
      clearInterval(timer); clearInterval(workTimer); clearInterval(skyTimer); clearInterval(barTimer);
      ro.disconnect();
    }

    // --- Interacció amb un vent ---

    function toWorld(e) {
      const r = world.getBoundingClientRect();
      return { x: (e.clientX - r.left) / scale, y: (e.clientY - r.top) / scale };
    }
    function wire(a) {
      let down = null;
      a.node.addEventListener('pointerdown', e => {
        e.stopPropagation();
        down = { x: e.clientX, y: e.clientY, moved: false };
        a.node.setPointerCapture(e.pointerId);
      });
      a.node.addEventListener('pointermove', e => {
        if (!down) return;
        if (!down.moved && Math.hypot(e.clientX - down.x, e.clientY - down.y) < 6) return;
        down.moved = true;
        a.dragging = true;
        a.node.classList.add('is-dragging');
        // A prop de la vora, el pati es desplaça sol mentre arrossegues.
        const r = scene.getBoundingClientRect();
        if (e.clientX < r.left + 50) scroller.scrollLeft -= 14;
        else if (e.clientX > r.right - 50) scroller.scrollLeft += 14;
        const p = toWorld(e);
        place(a, Math.min(W - 20, Math.max(20, p.x)), Math.min(H - 10, Math.max(SKY + 40, p.y + 30)), 0);
        scene.classList.toggle('drop-hot', inCircle(a.x, a.y));
      });
      const end = () => {
        if (!down) return;
        const moved = down.moved;
        down = null;
        a.node.classList.remove('is-dragging');
        scene.classList.remove('drop-hot');
        if (!moved) { openCard(a); return; }
        a.dragging = false;
        if (inCircle(a.x, a.y)) joinCircle(a);
        else {
          if (a.inCircle) leaveCircle(a, false);
          setAct(a, 'stroll', zoneAt(a.x, a.y));
          say(a, pick([t('On sóc?', 'Where am I?'), t('Uau!', 'Whoa!'), '😮']), 1600);
          a.until = performance.now() + 2500;
        }
      };
      a.node.addEventListener('pointerup', end);
      a.node.addEventListener('pointercancel', end);
      a.node.addEventListener('keydown', e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openCard(a); } });
    }

    // La rotllana del cercle de guix.
    const circle = [];
    function seatCircle() {
      const n = circle.length;
      circle.forEach((a, i) => {
        const ang = -Math.PI / 2 + (i / Math.max(n, 1)) * Math.PI * 2;
        walkTo(a, P.circle.x + Math.cos(ang) * P.circle.rx * 0.6, P.circle.y + Math.sin(ang) * P.circle.ry * 0.6);
        a.node.classList.toggle('face-left', Math.cos(ang) > 0.1);
      });
      paintPanel();
    }
    function joinCircle(a) {
      if (!a.inCircle) { a.inCircle = true; circle.push(a); }
      setAct(a, 'circle', plaza);
      say(a, t('Aquí sóc!', 'Here I am!'), 1500);
      seatCircle();
    }
    function leaveCircle(a, walk = true) {
      a.inCircle = false;
      const i = circle.indexOf(a);
      if (i >= 0) circle.splice(i, 1);
      a.until = 0;
      if (walk) { const s = spotIn(a.home, a); walkTo(a, s.x, s.y); setAct(a, 'stroll', a.home); }
      seatCircle();
    }
    function paintPanel() {
      panel.replaceChildren();
      panel.hidden = circle.length === 0;
      if (!circle.length) return;
      panel.append(el('strong', '', t('A la rotllana: ', 'In the circle: ') + circle.map(a => a.w.name).join(', ')));
      const row = el('div', 'pati-circle-actions');
      const talk = el('button', 'pati-btn primary', circle.length > 1 ? t('Obre una conversa amb ells', 'Start a chat with them') : t('Parla-hi', 'Talk to them'));
      talk.type = 'button';
      talk.onclick = async () => {
        if (circle.length === 1) { ctx.talkTo(circle[0].w); return; }
        const names = circle.map(a => a.w.name).join(', ');
        const data = await ctx.call('/api/winds/rotllanes', 'POST', { name: t('Rotllana del pati: ', 'Playground circle: ') + names, members: circle.map(a => a.w.id), workspace: '' });
        if (data?.rotllana) ctx.openChat(data.rotllana);
      };
      const job = el('button', 'pati-btn', t('Encarrega’ls una feina', 'Give them a task'));
      job.type = 'button';
      job.onclick = () => askTask();
      const clear = el('button', 'pati-btn', t('Torneu a jugar', 'Back to play'));
      clear.type = 'button';
      clear.onclick = () => { for (const a of [...circle]) leaveCircle(a); };
      row.append(talk, job, clear);
      panel.append(row);
    }
    function askTask() {
      if (panel.querySelector('.pati-task')) return;
      const form = el('form', 'pati-task');
      const field = el('textarea');
      field.rows = 2;
      field.placeholder = t('Què han de fer? (p. ex. una web per a la fleca del barri)', 'What should they do? (e.g. a website for the local bakery)');
      const go = el('button', 'pati-btn primary', t('Al taller!', 'To the workshop!'));
      go.type = 'submit';
      form.append(field, go);
      panel.append(form);
      field.focus();
      form.onsubmit = async e => {
        e.preventDefault();
        const task = field.value.trim();
        if (!task) return;
        go.disabled = true;
        try {
          await ctx.call('/api/winds/crew/start', 'POST', { task, lang: ca() ? 'ca' : 'en', reuse: circle.map(a => a.w.id), create: [] });
          ctx.notify(t('La colla se’n va al taller. La seguiràs al cartell, a Feines i al fil.', 'The crew heads to the workshop. Follow it on the sign, in Work and in the thread.'));
          const crew = [...circle];
          for (const a of crew) { realWork.set(a.w.id, { task }); leaveCircle(a, false); a.until = 0; }
          paintSign([{ task, agents: Object.fromEntries(crew.map((a, i) => [i, a.w.name])), files: [] }]);
          setTimeout(() => refreshWork(), 4000);
        } catch (err) {
          ctx.notify(t('No s’ha pogut encarregar: ', 'Could not start it: ') + err.message);
          go.disabled = false;
        }
      };
    }

    // La fitxa d'un vent: què fa, què pensa i què hi pots fer.
    async function openCard(a) {
      card.replaceChildren();
      card.hidden = false;
      const def = a.act === 'home' ? a.home.def.act : ACTS[a.act] || ACTS.stroll;
      const z = zoneAt(a.x, a.y);
      const head = el('div', 'pati-card-head');
      head.append(root.gregalWinds.face(a.w, true, def.mood));
      const who = el('div');
      who.append(el('strong', '', a.w.name), el('span', '', [a.w.role, a.home.icon + ' ' + a.home.label].filter(Boolean).join(' · ')));
      head.append(who);
      const close = el('button', 'pati-close', '✕');
      close.type = 'button';
      close.setAttribute('aria-label', t('Tanca', 'Close'));
      close.onclick = () => { card.hidden = true; };
      head.append(close);
      card.append(head);
      const job = realWork.get(a.w.id);
      card.append(el('p', 'pati-now', job ? t('Ara treballa de debò: ', 'Now really working on: ') + clip(job.task || '', 120)
        : t('Ara ', 'Now ') + t(def.ca, def.en) + (z && !['circle', 'line', 'home'].includes(a.act) ? ' · ' + lower(z.label) : '')));
      const th = a.home.def.think;
      card.append(el('p', 'pati-think', t('Pensa: ', 'Thinking: ') + (job ? t('en acabar-ho bé', 'about getting it done well') : th ? lower(t(th[0], th[1])) : '…')));
      const last = el('p', 'pati-last');
      card.append(last);
      const row = el('div', 'pati-card-actions');
      const talk = el('button', 'pati-btn primary', t('Parla-hi', 'Talk'));
      talk.type = 'button';
      talk.onclick = () => ctx.talkTo(a.w);
      const toCircle = el('button', 'pati-btn', a.inCircle ? t('Treu-lo de la rotllana', 'Out of the circle') : t('Porta’l a la rotllana', 'Bring to the circle'));
      toCircle.type = 'button';
      toCircle.onclick = () => { card.hidden = true; if (a.inCircle) leaveCircle(a); else { joinCircle(a); centerOn(plaza); } };
      row.append(talk, toCircle);
      if (job?.thread) {
        const open = el('button', 'pati-btn', t('Obre la feina', 'Open the task'));
        open.type = 'button';
        open.onclick = () => ctx.openThread?.(job.thread);
        row.append(open);
      }
      card.append(row);
      try {
        const data = await ctx.call('/api/winds/chat?wind=' + encodeURIComponent(a.w.id));
        const lastMsg = (data.history || []).filter(m => m.from !== 'user' && m.role !== 'user').pop();
        const text = (lastMsg?.text || lastMsg?.content || '').replace(/[#*_`>]/g, '').replace(/\s+/g, ' ').trim();
        if (text) last.textContent = t('L’últim que et va dir: «', 'Last thing it told you: “') + clip(text, 160) + t('»', '”');
      } catch (_) { /* sense historial */ }
    }

    bell.onclick = () => {
      // Campana: tothom corre a fer fila davant de la rotllana.
      centerOn(plaza);
      const list = [...actors.values()].filter(a => !a.inCircle && !realWork.has(a.w.id));
      const perRow = Math.min(9, Math.ceil(list.length / Math.ceil(list.length / 9)));
      list.forEach((a, i) => {
        const row = Math.floor(i / perRow), col = i % perRow;
        const x = plaza.x + 60 + ((PW - 120) * (col + 0.5)) / perRow;
        walkTo(a, x, SKY + TH * 1.45 + row * 46);
        setAct(a, 'line', plaza);
        a.until = performance.now() + 10000;
        setTimeout(() => alive && say(a, pick([t('Sóc aquí!', 'Here!'), '👋', t('Ja vinc!', 'Coming!')]), 1800), 600 + i * 140);
      });
      scene.classList.add('bell');
      setTimeout(() => scene.classList.remove('bell'), 1200);
    };
    pause.onclick = () => {
      paused = !paused;
      if (paused) {
        for (const a of actors.values()) {
          const r = a.node.getBoundingClientRect(), wr = world.getBoundingClientRect();
          place(a, (r.left + r.width / 2 - wr.left) / scale, (r.top + r.height * 0.92 - wr.top) / scale, 0);
        }
      }
      scene.classList.toggle('is-paused', paused);
      pause.textContent = paused ? t('▶ Torna el temps', '▶ Resume time') : t('⏸ Atura el temps', '⏸ Freeze time');
      if (!paused) for (const a of actors.values()) a.until = performance.now() + rnd(0, 1500);
    };
    if (reduced()) {
      for (const a of actors.values()) { const s = spotIn(a.home, a); place(a, s.x, s.y, 0); setAct(a, 'home', a.home); }
      paused = true;
      pause.hidden = true;
    }
    paintBar();
    return { stop };
  }

  // El món dibuixat: cel, terra, camins, la plaça i cada racó.
  function drawWorld(W, zones, plaza, P, Z) {
    const NS = 'http://www.w3.org/2000/svg';
    const svg = document.createElementNS(NS, 'svg');
    svg.setAttribute('class', 'pati-ground');
    svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
    svg.setAttribute('width', W);
    svg.setAttribute('height', H);
    svg.setAttribute('aria-hidden', 'true');
    const midY = SKY + TH;
    let out = `
      <defs>
        <linearGradient id="patiSky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="var(--pati-sky-a)"/><stop offset="1" stop-color="var(--pati-sky-b)"/></linearGradient>
        <radialGradient id="patiSand" cx=".5" cy=".55" r=".9"><stop offset="0" stop-color="var(--pati-sand-a)"/><stop offset="1" stop-color="var(--pati-sand-b)"/></radialGradient>
      </defs>
      <rect width="${W}" height="${H}" fill="url(#patiSand)"/>
      <rect width="${W}" height="${SKY}" fill="url(#patiSky)"/>
      <path d="M0 ${SKY}h${W}" stroke="var(--pati-wall)" stroke-width="14"/>
      <g class="pati-clouds">${Array.from({ length: Math.ceil(W / 420) }, (_, i) => `<ellipse cx="${160 + i * 420}" cy="${30 + (i % 2) * 10}" rx="${46 + (i % 3) * 10}" ry="12" fill="#fff" opacity=".55"/>`).join('')}</g>
      <g class="pati-stars">${Array.from({ length: Math.ceil(W / 120) }, (_, i) => `<circle cx="${60 + i * 120 + (i % 3) * 17}" cy="${18 + (i * 13) % 40}" r="${1.4 + (i % 2) * 0.6}" fill="#fff"/>`).join('')}</g>
      <path d="M10 ${midY} H${W - 10}" stroke="var(--pati-path)" stroke-width="26" stroke-linecap="round" opacity=".6"/>`;
    for (const z of zones.values()) {
      if (z === plaza) continue;
      out += `<g transform="translate(${z.x} ${z.y})"><rect x="8" y="10" width="${TW - 16}" height="${TH - 20}" rx="20" fill="var(--pati-plot)" opacity=".35"/>${z.def.draw()}<text x="${TW / 2}" y="${TH - 14}" text-anchor="middle" class="pati-label-svg">${z.icon} ${escapeXML(z.label)}</text></g>`;
    }
    const px = plaza.x;
    out += `<g transform="translate(${px} ${SKY})">
      <rect x="10" y="10" width="${PW - 20}" height="${TH * 2 - 20}" rx="26" fill="var(--pati-plaza)" opacity=".55"/>
      <path d="M${PW / 2} 20 V${TH * 2 - 20}" stroke="var(--pati-path)" stroke-width="22" opacity=".6"/>
      <!-- gronxadors -->
      <path d="M45 150l18-82h64l18 82M63 68h64" fill="none" stroke="#7C8FB0" stroke-width="6" stroke-linejoin="round"/>
      <g class="pati-swing"><path d="M82 68v54M98 68v54" stroke="#5B4A35" stroke-width="2"/><rect x="76" y="120" width="28" height="6" rx="2" fill="#D9694F"/></g>
      <g class="pati-swing" style="animation-delay:-1.3s"><path d="M110 68v54M124 68v54" stroke="#5B4A35" stroke-width="2"/><rect x="104" y="120" width="26" height="6" rx="2" fill="#7BBF6A"/></g>
      <text x="95" y="172" text-anchor="middle" class="pati-label-svg">${t('els gronxadors', 'the swings')}</text>
      <!-- font -->
      <circle cx="${PW - 90}" cy="110" r="34" fill="#BFD5E0" stroke="#8EA9B6" stroke-width="5"/><circle cx="${PW - 90}" cy="110" r="10" fill="#E8F4FA"/><path d="M${PW - 90} 100q-10-26 0-40q10 14 0 40" fill="#9FD3EE" opacity=".8" class="pati-fountain"/>
      <!-- cercle de guix de la rotllana -->
      <ellipse class="pati-chalk" cx="${PW / 2}" cy="${P.circle.y - SKY}" rx="${P.circle.rx}" ry="${P.circle.ry}" fill="none" stroke="#FFFFFF" stroke-width="5" stroke-dasharray="14 10" opacity=".9"/>
      <text x="${PW / 2}" y="${P.circle.y - SKY + P.circle.ry + 26}" text-anchor="middle" class="pati-label-svg">${t('la rotllana (arrossega-hi vents)', 'the circle (drag winds here)')}</text>
      <!-- tobogan -->
      <path d="M70 ${TH * 2 - 60}l-10-70h14l10 70M60 ${TH * 2 - 130}h18" fill="none" stroke="#7C8FB0" stroke-width="5"/><path d="M76 ${TH * 2 - 129}q30 6 46 66l-10 4q-16-54-40-58Z" fill="#F2B53A" stroke="#C98E2A" stroke-width="2"/>
      <text x="95" y="${TH * 2 - 22}" text-anchor="middle" class="pati-label-svg">${t('el tobogan', 'the slide')}</text>
      <!-- taller -->
      <rect x="${PW - 190}" y="${TH * 2 - 150}" width="170" height="100" rx="6" fill="#C9B08A" stroke="#8A6A45" stroke-width="4"/><path d="M${PW - 200} ${TH * 2 - 150}l95-44 95 44Z" fill="#7C8FB0"/>
      <rect x="${PW - 172}" y="${TH * 2 - 104}" width="134" height="12" rx="3" fill="#8A6A45"/><path d="M${PW - 160} ${TH * 2 - 108}l14-14 6 6-14 14Z M${PW - 70} ${TH * 2 - 116}h14v12h-14Z" fill="#6F7E99"/>
      <text x="${PW - 105}" y="${TH * 2 - 22}" text-anchor="middle" class="pati-label-svg">🔨 ${t('el taller', 'the workshop')}</text>
    </g>`;
    svg.innerHTML = out;
    return svg;
  }
  const escapeXML = s => String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

  root.gregalPati = { mount };
})(window);
