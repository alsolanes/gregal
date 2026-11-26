// Clay winds (tools/characters, task 112) where a wind's face is large: the
// mood's sprite sheet on a canvas, the body tinted with the wind's colour
// through its mask and the minairó's barretina with its own through the cap mask. The code-drawn face from winds.js shows until the sheet is
// ready and stays if it cannot load; it follows every mood, ready to come back.
(function (root) {
  'use strict';
  const doc = root.document, base = '/app/characters/', fade = 180;
  const clock = () => root.performance.now();
  const reduced = () => !!root.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
  let manifest;
  const sheets = new Map(), ready = new Map(), players = new WeakMap(), running = new Set();
  const seen = root.IntersectionObserver && new root.IntersectionObserver(entries => {
    for (const e of entries) { const p = players.get(e.target); if (p) p.visible = e.isIntersecting; }
    schedule();
  });
  doc.addEventListener('visibilitychange', schedule);

  function canvas(width, height) { const c = doc.createElement('canvas'); c.width = width; c.height = height; return c; }
  function image(src) {
    return new Promise((resolve, reject) => { const img = new root.Image(); img.onload = () => resolve(img); img.onerror = reject; img.src = src; });
  }
  // Multiply the clay by the colour as far as the mask covers it: the colour
  // where it is full, the clay as it is where it is empty (eyes, mouth, blush).
  function tint(pixels, mask, rgb) {
    for (let i = 0; i < pixels.length; i += 4) {
      const a = mask[i + 3] / 255;
      for (let c = 0; c < 3; c++) pixels[i + c] = Math.round(pixels[i + c] * (1 - a + a * rgb[c] / 255));
    }
    return pixels;
  }
  // One tinted sheet per silhouette, mood and colours, shared by every face; a
  // `still` one is only the still frame (155: a 256 px sheet is megabytes once
  // tinted). The faces keep theirs: the cache forgets the oldest past `kept`,
  // so dragging the editor's colour picker does not pile up sheets.
  const kept = 24;
  function sheet(letter, mood, color, cap, still = false) {
    const key = letter + '/' + mood + '/' + color + cap + (still ? '/still' : '');
    if (!sheets.has(key)) {
      for (const old of sheets.keys()) { if (sheets.size < kept) break; sheets.delete(old); ready.delete(old); }
      manifest ||= root.fetch(base + 'manifest.json').then(r => { if (!r.ok) throw new Error('characters-manifest'); return r.json(); });
      sheets.set(key, manifest.then(async m => {
        const spec = m.moods[mood], silhouette = Object.keys(m.silhouettes).find(k => m.silhouettes[k] === letter);
        if (!spec || !silhouette) throw new Error('characters-' + key);
        const file = pattern => base + pattern.replace('<silhouette>', silhouette).replace('<mood>', mood);
        const masks = [[m.mask, color], ...(m.caps || []).includes(silhouette) ? [[m.cap, cap]] : []];
        const [body, ...shapes] = await Promise.all([image(file(m.files)), ...masks.map(([pattern]) => image(file(pattern)))]);
        const size = m.size, full = spec.frames * size, width = still ? size : full;
        const from = still ? -(spec.still || 0) * size : 0, out = canvas(width, size), ctx = out.getContext('2d');
        ctx.drawImage(body, from, 0);
        const pixels = ctx.getImageData(0, 0, width, size);
        shapes.forEach((shape, i) => {
          const up = canvas(width, size).getContext('2d'), n = parseInt(masks[i][1].slice(1), 16);
          up.drawImage(shape, from, 0, full, size);
          tint(pixels.data, up.getImageData(0, 0, width, size).data, [n >> 16 & 255, n >> 8 & 255, n & 255]);
        });
        ctx.putImageData(pixels, 0, 0);
        const done = still ? { canvas: out, size, fps: m.fps, frames: 1, loop: false, still: 0 }
          : { canvas: out, size, fps: m.fps, frames: spec.frames, loop: spec.loop, still: spec.still || 0 };
        ready.set(key, done);
        return done;
      }));
    }
    return sheets.get(key);
  }

  // Reduced motion holds one frame: the sheet's still (142: one with the prop in view), or the first.
  function frameOf(s, since, now) {
    if (reduced()) return s.still;
    const f = Math.floor((now - since) * s.fps / 1000);
    return s.loop ? f % s.frames : Math.min(f, s.frames - 1);
  }
  function draw(p, now) {
    const t = p.from && !reduced() ? Math.min(1, (now - p.fadeAt) / fade) : 1;
    if (t >= 1) p.from = null;
    const f = frameOf(p.now, p.at, now), same = p.now.canvas === p.drawn?.canvas && f === p.drawn.f && t === p.drawn.t;
    if (same) return;
    p.drawn = { canvas: p.now.canvas, f, t };
    const ctx = p.el.getContext('2d'), size = p.now.size;
    ctx.clearRect(0, 0, size, size);
    // The two sheets add up, so the figure never thins out half way.
    if (p.from) {
      ctx.globalAlpha = 1 - t;
      ctx.drawImage(p.from.canvas, frameOf(p.from, p.fromAt, now) * size, 0, size, size, 0, 0, size, size);
      ctx.globalCompositeOperation = 'lighter';
    }
    ctx.globalAlpha = t;
    ctx.drawImage(p.now.canvas, f * size, 0, size, size, 0, 0, size, size);
    ctx.globalAlpha = 1; ctx.globalCompositeOperation = 'source-over';
    if (p.badge) badge(ctx, size, p.badge, p.color);
  }
  // 159: a wind's trade on its clay figure, the trade chip's icon on a disc at
  // the bottom-right corner, where the figure leaves room beside its feet.
  function badge(ctx, size, icon, color) {
    const r = size * .15, cx = size * .82, cy = size * .8, unit = r * 1.25 / 16;
    ctx.save();
    ctx.beginPath(); ctx.arc(cx, cy, r, 0, 2 * Math.PI);
    ctx.fillStyle = '#FFFFFF'; ctx.fill();
    ctx.lineWidth = size * .012; ctx.strokeStyle = color; ctx.stroke();
    ctx.translate(cx - 8 * unit, cy - 8 * unit); ctx.scale(unit, unit);
    ctx.lineWidth = 1.6; ctx.lineCap = 'round'; ctx.lineJoin = 'round'; ctx.strokeStyle = '#262040';
    ctx.stroke(new root.Path2D(icon));
    ctx.restore();
  }
  let ticking = false;
  function schedule() {
    if (ticking || doc.hidden || reduced() || ![...running].some(p => p.visible)) return;
    ticking = true;
    root.requestAnimationFrame(tick);
  }
  function tick(now) {
    ticking = false;
    if (doc.hidden) return;
    for (const p of running) {
      // A face handed out but not on the page yet keeps its place; one that left the page is done.
      if (p.el.isConnected) { p.placed = true; if (p.visible) draw(p, now); }
      else if (p.placed) { running.delete(p); seen?.unobserve(p.el); }
    }
    schedule();
  }

  function show(p, s, at = clock(), from = null) {
    const c = p.el = canvas(s.size, s.size);
    for (const name of ['class', 'data-wind', 'data-mood', 'aria-hidden', 'role', 'aria-label']) if (p.svg.getAttribute(name) !== null) c.setAttribute(name, p.svg.getAttribute(name));
    c.setAttribute('class', c.getAttribute('class') + ' winds-clay');
    p.now = s; p.at = at; p.visible = true;
    if (from && from.sheet !== s && !reduced() && !p.still) Object.assign(p, { from: from.sheet, fromAt: from.at, fadeAt: clock() });
    // A still face is drawn once (and again on a mood change): no animation loop for it.
    players.set(c, p);
    if (!p.still) { running.add(p); seen?.observe(c); }
    remember(p);
    draw(p, clock());
    schedule();
    return c;
  }
  // The sheet each wind showed last in each spot (the chat corner, a start
  // face): a face drawn again there goes on from it instead of popping.
  const last = new Map();
  function remember(p) { last.set(p.spot, { look: p.letter + p.color + p.cap + p.still, sheet: p.now, mood: p.mood, at: p.at }); }
  // Takes the face winds.js drew for a large spot and returns what to put on
  // the page: the clay when a sheet for it is already here, or that face,
  // swapped in place for the clay once its sheet loads. A `still` face holds
  // the sheet's still frame.
  const handed = new WeakMap();
  function play(svg, w, still = false) {
    // A face already handed here (winds.js plays the pelut and small Cares itself) or already clay stays as it is,
    // except that a caller playing it moves a face handed out still (167: the start faces call face(w) and play it).
    const known = handed.get(svg) || players.get(svg);
    if (known) { if (!still) animate(known); return svg; }
    // The clay is Cares, with its trade as a badge (159), or the minairó and the pelut in their own sheets; the other packs stay drawn.
    const avatar = svg.getAttribute('data-avatar');
    if (!w || (avatar && avatar !== 'minairo' && avatar !== 'pelut')) return svg;
    const letter = avatar || ('ons'.includes(String(w.style || '')[2]) ? w.style[2] : 'o'), color = /^#[\da-f]{6}$/i.test(w.color || '') ? w.color : '#F5B45D';
    // The minairó's barretina colour, as winds.js picked it for this wind.
    const cap = svg.getAttribute('data-barretina') || '';
    const p = { w, svg, letter, color, cap, still, badge: root.gregalWinds.kitIcon(svg.getAttribute('data-kit')), mood: svg.getAttribute('data-mood') || 'idle', spot: w.id + '|' + svg.getAttribute('class') }, key = letter + '/' + p.mood + '/' + color + cap + (still ? '/still' : '');
    handed.set(svg, p);
    const before = w.id && last.get(p.spot), same = before?.look === letter + color + cap + still ? before : null;
    if (ready.has(key)) return show(p, ready.get(key), clock(), same);
    const mood = p.mood;
    if (same) { p.mood = same.mood; return change(show(p, same.sheet, same.at), mood); }
    sheet(letter, mood, color, cap, still).then(s => {
      if (!svg.isConnected) return;
      const c = show(p, s);
      svg.replaceWith(c);
      if (still && !p.still) moving(p);
      if (svg.getAttribute('data-mood') !== mood) change(c, svg.getAttribute('data-mood'));
    }, error => root.console?.warn('clay wind: the drawn face stays', error));
    return svg;
  }
  // A still face that starts moving: into the loop, on its mood's whole sheet. One not shown yet does it in play().
  function animate(p) {
    if (!p.still) return;
    p.still = false;
    if (!p.el) return;
    running.add(p); seen?.observe(p.el);
    moving(p);
  }
  function moving(p) {
    const mood = p.mood;
    sheet(p.letter, mood, p.color, p.cap).then(s => {
      if (p.mood !== mood) return;
      p.now = s; p.at = clock();
      remember(p);
      draw(p, p.at);
      schedule();
    }, error => root.console?.warn('clay wind: the still frame stays', error));
  }
  function change(el, mood) {
    const p = players.get(el);
    if (!p) return root.gregalWinds.mood(el, mood);
    if (mood === 'idle' && root.gregalWinds.hasNews(p.w.id)) mood = 'news';
    if (p.mood === mood) return el;
    p.mood = mood;
    root.gregalWinds.mood(p.svg, mood);
    for (const name of ['data-mood', 'aria-hidden', 'role', 'aria-label']) {
      if (p.svg.getAttribute(name) === null) el.removeAttribute(name); else el.setAttribute(name, p.svg.getAttribute(name));
    }
    // A still face that a caller set moving meanwhile (animate) has its whole sheet on the way: this one is stale.
    const still = p.still;
    sheet(p.letter, mood, p.color, p.cap, still).then(s => {
      if (p.mood !== mood || p.still !== still) return;
      if (!reduced() && !p.still) { p.from = p.now; p.fromAt = p.at; p.fadeAt = clock(); }
      p.now = s; p.at = clock();
      remember(p);
      draw(p, p.at);
      schedule();
    }, () => {
      if (p.mood !== mood) return;
      running.delete(p); seen?.unobserve(el);
      el.replaceWith(p.svg);
    });
    return el;
  }
  root.gregalCharacters = { play, mood: change, tint };
})(window);
