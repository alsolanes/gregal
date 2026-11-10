// Vents: same authenticated API/session as the host app. All remote text is
// rendered with textContent; SVG shapes and attributes are authored here only.
(function (root) {
  'use strict';
  const doc = root.document;
  const state = { winds: [], conversations: [], active: null, loading: false };
  const i18n = () => root.gregalWindI18n;
  const tr = (key, fallback, vars) => i18n()?.text(key, fallback, vars) ?? interpolate(fallback, vars);
  function interpolate(value, vars) { return String(value).replace(/\{([\w]+)\}/g, (_, key) => vars?.[key] ?? `{${key}}`); }
  function paint(element, value) { if (i18n()) i18n().paint(element, value); else element.textContent = String(value ?? ''); }
  function attribute(element, name, value) { if (i18n()) i18n().attribute(element, name, value); else element.setAttribute(name, String(value ?? '')); }
  const $ = id => doc.getElementById(id);
  function node(tag, cls, text) {
    const n = doc.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) paint(n, text);
    return n;
  }
  function button(text, action, cls) {
    const n = node('button', cls || 'ghost', text);
    n.setAttribute('type', 'button'); n.onclick = action;
    return n;
  }
  async function call(url, method = 'GET', body) {
    const opts = { method };
    if (body !== undefined) { opts.body = JSON.stringify(body); opts.headers = { 'Content-Type': 'application/json' }; }
    const response = await root.api(url, opts);
    if (!response.ok) throw new Error('request-failed');
    return response.json();
  }
  function color(value) { return /^#[\da-f]{6}$/i.test(value || '') ? value : '#F5B45D'; }
  function style(value) {
    const s = String(value || 'pso');
    return ('pbem'.includes(s[0]) ? s[0] : 'p') + ('sont'.includes(s[1]) ? s[1] : 's') + ('ons'.includes(s[2]) ? s[2] : 'o');
  }
  // Squad specialists are small mascots with the tool of their trade; winds
  // without a specialty keep the plain face. Shapes are [tag, attrs] in the
  // 80×80 viewBox. `head` is drawn with the face (face coordinates, scaled with
  // it); `prop` is held in front. Android's WindFace.kt mirrors these shapes.
  const SVG = 'http://www.w3.org/2000/svg', INK = '#262040';
  const ln = (width = 2.2) => ({ stroke:INK, 'stroke-width':width, 'stroke-linejoin':'round', 'stroke-linecap':'round' });
  const kits = {
    shopping: { hands:[[53,59]], prop:[
      ['path', { d:'M57 54v-3a6 6 0 0 1 12 0v3', fill:'none', ...ln(2.4) }],
      ['path', { d:'M52 54H74L72 75H54Z', fill:'#E97A86', ...ln() }],
      ['path', { d:'M53.5 60H72.5', stroke:'#FFF1EC', 'stroke-width':2.4, opacity:'.85' }]] },
    books: { hands:[[17,69.5],[63,69.5]], prop:[
      ['path', { d:'M17 63.5Q28 58.5 40 63.5Q52 58.5 63 63.5V77.5Q52 73.5 40 78.5Q28 73.5 17 77.5Z', fill:'#5E86D6', ...ln() }],
      ['path', { d:'M20 62.5Q30 58.5 40 62.5V75.5Q30 71.5 20 74.5Z', fill:'#FFF8EE', ...ln(1.6) }],
      ['path', { d:'M60 62.5Q50 58.5 40 62.5V75.5Q50 71.5 60 74.5Z', fill:'#FFF8EE', ...ln(1.6) }],
      ['path', { d:'M24 65.5q6-2 12 0M24 69.5q6-2 12 0M44 65.5q6-2 12 0M44 69.5q6-2 12 0', fill:'none', stroke:'#A79FBC', 'stroke-width':1.3, 'stroke-linecap':'round' }]] },
    sports: { hands:[[53,64]], head:[
      ['path', { d:'M11.5 24Q40 11 68.5 24L70 31Q40 19 10 31Z', fill:'#E5484D', ...ln(2) }]], prop:[
      ['circle', { cx:63, cy:62, r:11, fill:'#FFFFFF', ...ln() }],
      ['path', { d:'M63 57.5l4.3 3.1-1.6 5h-5.4l-1.6-5Z', fill:INK }],
      ['path', { d:'M63 57.5V51.5M67.3 60.6l5.4-1.8M65.7 65.6l3.3 4.6M60.3 65.6l-3.3 4.6M58.7 60.6l-5.4-1.8', fill:'none', ...ln(1.5) }]] },
    music: { head:[
      ['path', { d:'M8.5 42A31.5 31.5 0 0 1 71.5 42', fill:'none', stroke:'#585D7E', 'stroke-width':5, 'stroke-linecap':'round' }],
      ['rect', { x:1, y:33, width:11, height:21, rx:5, fill:'#F28C6B', ...ln() }],
      ['rect', { x:68, y:33, width:11, height:21, rx:5, fill:'#F28C6B', ...ln() }]] },
    organisation: { hands:[[50,65]], prop:[
      ['rect', { x:50, y:49, width:24, height:24, rx:4, fill:'#FFFFFF', ...ln() }],
      ['path', { d:'M50 57V53a4 4 0 0 1 4-4h16a4 4 0 0 1 4 4v4Z', fill:'#E5484D', ...ln() }],
      ['path', { d:'M57 46.5v5M67 46.5v5', fill:'none', ...ln(2.4) }],
      ['path', { d:'M56 65l3.5 3.5 7-7', fill:'none', stroke:'#2F9E6A', 'stroke-width':2.8, 'stroke-linecap':'round', 'stroke-linejoin':'round' }]] },
    quality: { prop:[
      ['path', { d:'M66 66L74 74', stroke:'#7A5236', 'stroke-width':6, 'stroke-linecap':'round' }],
      ['circle', { cx:59, cy:59, r:9.5, fill:'#D7EEFA', ...ln(3) }],
      ['path', { d:'M53.5 57a6 6 0 0 1 5-5', fill:'none', stroke:'#FFFFFF', 'stroke-width':2, 'stroke-linecap':'round' }]] },
    regression: { hands:[[48,63]], prop:[
      ['rect', { x:47, y:47, width:28, height:26, rx:3.5, fill:'#FFFFFF', ...ln() }],
      ['path', { d:'M52 52V68H71', fill:'none', ...ln(1.6) }],
      ['path', { d:'M54 66L70 53', stroke:'#E5484D', 'stroke-width':2.2, 'stroke-linecap':'round' }],
      ['path', { d:'M57 63h.01M60.5 64.5h.01M63 59.5h.01M66.5 58h.01M68 54.5h.01', stroke:'#4C7BD9', 'stroke-width':3.6, 'stroke-linecap':'round' }]] },
    ml: { hands:[[50,63]], head:[
      ['path', { d:'M41 8q0-4 5-6', fill:'none', stroke:'#7A7F9C', 'stroke-width':2.6, 'stroke-linecap':'round' }],
      ['circle', { cx:48, cy:2.5, r:3.6, fill:'#8EE3B0', ...ln(2) }]], prop:[
      ['path', { d:'M55 49v-3M61 49v-3M67 49v-3M55 74v-3M61 74v-3M67 74v-3M72 54h3M72 60h3M72 66h3', fill:'none', ...ln(2) }],
      ['rect', { x:50, y:49, width:22, height:22, rx:4, fill:'#5B6BB5', ...ln() }],
      ['rect', { x:56, y:55, width:10, height:10, rx:2, fill:'#A9F0C5' }]] },
    timeseries: { hands:[[52,64]], prop:[
      ['circle', { cx:63, cy:61, r:11.5, fill:'#FFFFFF', ...ln() }],
      ['path', { d:'M55 64.5l3-3.5 3 2.5 4-6 3 3.5 2.5-2', fill:'none', stroke:'#3FA39A', 'stroke-width':2, 'stroke-linecap':'round', 'stroke-linejoin':'round' }],
      ['path', { d:'M63 61V53.5M63 61l4.5 3', fill:'none', ...ln(2) }]] },
    research: { hands:[[52,64]], prop:[
      ['path', { d:'M50 46h7v9l5 13a3 3 0 0 1-2.8 4H47.8A3 3 0 0 1 45 68l5-13Z', fill:'#D7EEFA', ...ln() } ],
      ['path', { d:'M48 61h14l1.5 7a1.5 1.5 0 0 1-1.4 2h-14a1.5 1.5 0 0 1-1.4-2Z', fill:'#5E86D6' } ],
      ['path', { d:'M51 64h3M57 64h3', stroke:'#FFF8EE', 'stroke-width':1.6, 'stroke-linecap':'round' } ],
      ['ellipse', { cx:53.5, cy:69, rx:1.6, ry:2.2, fill:'#FFF8EE' } ],
      ['path', { d:'M52 46v-2.5a2 2 0 0 1 3 0V46', fill:'none', stroke:'#A79FBC', 'stroke-width':1.6, 'stroke-linecap':'round' } ] ] },
    tech: { hands:[[52,63]], prop:[
      ['rect', { x:50, y:50, width:23, height:23, rx:4, fill:'#3A3F5C', ...ln() } ],
      ['path', { d:'M56 56h11M56 60h11M56 64h6', stroke:'#8EE3B0', 'stroke-width':2, 'stroke-linecap':'round' } ],
      ['path', { d:'M61.5 73v4M56 73v4M67 73v4', stroke:'#A79FBC', 'stroke-width':1.8, 'stroke-linecap':'round' } ] ] },
    review: { hands:[[52,63]], head:[
      ['rect', { x:22.6, y:27.8, width:15.6, height:21.2, rx:7.4, fill:'#FFFFFF', 'fill-opacity':'.25', ...ln(2.2) }],
      ['rect', { x:41.8, y:27.8, width:15.6, height:21.2, rx:7.4, fill:'#FFFFFF', 'fill-opacity':'.25', ...ln(2.2) }],
      ['path', { d:'M38.2 36.6q1.8-1.8 3.6 0M22.6 35.6l-7.6-2.6M57.4 35.6l7.6-2.6', fill:'none', ...ln(2) }]], prop:[
      ['circle', { cx:63, cy:61, r:11, fill:'#F2C14E', ...ln() }],
      ['path', { d:'M63 61V50A11 11 0 0 1 73.5 64.5Z', fill:'#5E86D6', ...ln(1.6) }],
      ['path', { d:'M63 61L73.5 64.5A11 11 0 0 1 59 71.3Z', fill:'#E97A86', ...ln(1.6) }]] },
    ux: { hands:[[52,65]], head:[
      ['path', { d:'M40 3.5l1.5-4', fill:'none', stroke:'#6B4FB8', 'stroke-width':2.8, 'stroke-linecap':'round' }],
      ['path', { d:'M7 24C8 9 29 1 50 5C63 8 68 15 63 18C47 13 25 15 9 28Z', fill:'#8E6CD9', ...ln() }]], prop:[
      ['path', { d:'M51 62c0-9 9-14 17-12 7 2 9 8 6 12-2 3-6 1-7 4-1 4 3 6 0 8-4 3-16 0-16-12Z', fill:'#F6E3C5', ...ln() }],
      ['circle', { cx:57.5, cy:58, r:2.4, fill:'#E5484D' }], ['circle', { cx:63.5, cy:55, r:2.4, fill:'#5E86D6' }],
      ['circle', { cx:69, cy:58.5, r:2.2, fill:'#F2C14E' }], ['circle', { cx:57.5, cy:65, r:2.2, fill:'#2F9E6A' }]] },
    frontend: { prop:[
      ['rect', { x:21, y:57, width:38, height:16.5, rx:3, fill:'#3A3F5C', ...ln() }],
      ['path', { d:'M33 61l-4 4 4 4M47 61l4 4-4 4M42 60.2l-4 9.6', fill:'none', stroke:'#8EE3B0', 'stroke-width':2.2, 'stroke-linecap':'round', 'stroke-linejoin':'round' }],
      ['path', { d:'M14 73.5H66L63 78H17Z', fill:'#C3C9D9', ...ln() }]] },
    backend: { hands:[[51,63]], prop:[
      ['path', { d:'M51 52V70A11 4 0 0 0 73 70V52', fill:'#7C8CE0', ...ln() }],
      ['path', { d:'M51 58.5A11 4 0 0 0 73 58.5M51 64.5A11 4 0 0 0 73 64.5', fill:'none', ...ln(1.6) }],
      ['ellipse', { cx:62, cy:52, rx:11, ry:4, fill:'#B4BDF4', ...ln() }]] },
    testing: { head:[
      ['path', { d:'M35 6l1-4M39 8.5l4-2', fill:'none', ...ln(1.6) }],
      ['circle', { cx:37, cy:9.5, r:3.8, fill:INK }],
      ['circle', { cx:30, cy:15, r:8, fill:'#E5484D', ...ln(2) }],
      ['path', { d:'M36 10L24 20', fill:'none', ...ln(1.5) }],
      ['path', { d:'M26.5 13h.01M33 18h.01M27.5 19.5h.01M31 11.5h.01', stroke:INK, 'stroke-width':3.2, 'stroke-linecap':'round' }]] },
    security: { hands:[[50,62]], prop:[
      ['path', { d:'M62 47L74 51.5V60C74 67.5 69 72.5 62 75C55 72.5 50 67.5 50 60V51.5Z', fill:'#4F8FD9', ...ln() }],
      ['circle', { cx:62, cy:58.5, r:3.2, fill:'#FFFFFF' }],
      ['path', { d:'M60.6 60.5h2.8l1 6.5h-4.8Z', fill:'#FFFFFF' }]] },
    delivery: { head:[
      ['path', { d:'M11 25C11 9 24 2 40 2S69 9 69 25Z', fill:'#F5C042', ...ln() }],
      ['path', { d:'M40 3V24', stroke:'#FFE29A', 'stroke-width':4, 'stroke-linecap':'round' }],
      ['rect', { x:4, y:23, width:72, height:6, rx:3, fill:'#F5C042', ...ln() }]], prop:[
      ['path', { d:'M55 73L66 62', stroke:INK, 'stroke-width':8, 'stroke-linecap':'round' }],
      ['path', { d:'M55 73L66 62', stroke:'#A7AFC2', 'stroke-width':4.4, 'stroke-linecap':'round' }],
      ['path', { d:'M71 50.5a7.5 7.5 0 1 0 5 7.5l-4.5 1.2-2.6-2.6 1-4.6Z', fill:'#A7AFC2', ...ln() }]] },
  };
  // Moods mirror observed work only: the chat's pending phase or a live step's
  // state and activity. Anything unknown stays idle.
  const moods = ['idle', 'thinking', 'searching', 'reading', 'writing', 'working', 'discussing', 'done', 'failed', 'news'];
  const activityMoods = {
    read:'reading', ls:'reading', list:'reading', web_fetch:'reading', office_read:'reading', image:'reading', review:'reading', reviewing:'reading',
    glob:'searching', grep:'searching', find:'searching', search:'searching', web_search:'searching', web:'searching', browser:'searching',
    write:'writing', edit:'writing', patch:'writing', office_edit:'writing', office_create:'writing', draft:'writing', revision:'writing', coding:'writing',
    plan:'thinking', analyze:'thinking', question:'thinking', discussion:'discussing',
  };
  // `voices` counts the winds heard in one rotllana turn: the second and later
  // voices answer the earlier ones, so they are discussing, not just writing.
  function moodOf({ state: step, activity, phase, text, voices } = {}) {
    if (voices > 1) return 'discussing';
    if (text) return 'writing';
    if (phase) return { searching:'searching', scheduling:'working', working:'working' }[phase] || 'thinking';
    if (step === 'done' || step === 'completed') return 'done';
    if (step === 'failed') return 'failed';
    if (step === 'working' || step === 'started') return activityMoods[activity] || 'working';
    return 'idle';
  }
  // Tones of a wind's own colour, the same mixes as Android's WindVolume.kt:
  // light and deep model the volume, and the ink of its tools is a dark tone of
  // its own hue instead of one navy for everybody.
  function mix(hex, other, amount) {
    const a = parseInt(hex.slice(1), 16), b = parseInt(other.slice(1), 16);
    return '#' + [16, 8, 0].map(shift => Math.round((a >> shift & 255) * (1 - amount) + (b >> shift & 255) * amount).toString(16).padStart(2, '0')).join('');
  }
  function svgNode(tag, attrs) {
    const el = doc.createElementNS(SVG, tag);
    for (const [key, value] of Object.entries(attrs)) el.setAttribute(key, value);
    return el;
  }
  // INK in the authored tables stands for the ink of the wind that wears them.
  function shapes(parent, list, ink) {
    for (const [tag, attrs] of list) {
      const el = svgNode(tag, attrs);
      if (ink) for (const name of ['fill', 'stroke']) if (attrs[name] === INK) el.setAttribute(name, ink);
      parent.append(el);
    }
    return parent;
  }
  function group(parent, cls, attrs = {}) { const g = svgNode('g', cls ? { class:cls, ...attrs } : attrs); parent.append(g); return g; }
  // Small state marks in the top-right corner; each is drawn only for its mood.
  const badges = {
    thinking: [['circle', { cx:55, cy:22, r:1.8, fill:'#FFFFFF', ...ln(1.4) }], ['circle', { cx:67, cy:12, r:10, fill:'#FFFFFF', ...ln(2) }],
      ['circle', { class:'wf-dot', cx:62, cy:12, r:1.9, fill:INK }], ['circle', { class:'wf-dot', cx:67, cy:12, r:1.9, fill:INK }], ['circle', { class:'wf-dot', cx:72, cy:12, r:1.9, fill:INK }]],
    searching: [['circle', { cx:67, cy:12, r:10, fill:'#FFFFFF', ...ln(2) }], ['g', { class:'wf-tool' }]],
    reading: [['circle', { cx:67, cy:12, r:10, fill:'#FFFFFF', ...ln(2) }],
      ['path', { d:'M60 9q3.5-2 7 0v8q-3.5-2-7 0Z', fill:'#FFF8EE', ...ln(1.5) }], ['path', { d:'M67 9q3.5-2 7 0v8q-3.5-2-7 0Z', fill:'#FFF8EE', ...ln(1.5) }],
      ['path', { class:'wf-line', d:'M67 9q3.5-2 7 0v8q-3.5-2-7 0Z', fill:'#FFF8EE', ...ln(1.5) }]],
    writing: [['circle', { cx:67, cy:12, r:10, fill:'#FFFFFF', ...ln(2) }], ['path', { d:'M61 17.5h7', fill:'none', stroke:'#A79FBC', 'stroke-width':1.5, 'stroke-linecap':'round' }], ['g', { class:'wf-tool' }]],
    working: [['circle', { cx:67, cy:12, r:10, fill:'#FFFFFF', ...ln(2) }], ['g', { class:'wf-tool' }]],
    done: [['g', { class:'wf-pop' }]],
    failed: [['circle', { cx:67, cy:12, r:9, fill:'#F5B45D', ...ln(2) }], ['path', { d:'M67 7.5v5.5M67 16.5h.01', fill:'none', ...ln(2.6) }]],
    discussing: [
      ['path', { class:'wf-say wf-say-b', d:'M66 1.5h9a3 3 0 0 1 3 3v6a3 3 0 0 1-3 3h-1v3.5l-3.5-3.5H66a3 3 0 0 1-3-3v-6a3 3 0 0 1 3-3Z', fill:'#D9CCF5', ...ln(1.8) }],
      ['path', { class:'wf-say wf-say-a', d:'M56 8h11a3 3 0 0 1 3 3v6a3 3 0 0 1-3 3h-6.5L57 23.5V20h-1a3 3 0 0 1-3-3v-6a3 3 0 0 1 3-3Z', fill:'#FFFFFF', ...ln(1.8) }],
      ['path', { d:'M58 14h.01M61.5 14h.01M65 14h.01', fill:'none', ...ln(2.2) }]],
    news: [['path', { d:'M58 3h17a3.5 3.5 0 0 1 3.5 3.5v9a3.5 3.5 0 0 1-3.5 3.5h-10l-4 4v-4h-3a3.5 3.5 0 0 1-3.5-3.5v-9A3.5 3.5 0 0 1 58 3Z', fill:'#FFFFFF', ...ln(2) }], ['g', { class:'wf-tool' }]],
  };
  const tools = {
    news: [['path', { d:'M66.5 4.5l1.7 4 4 1.7-4 1.7-1.7 4-1.7-4-4-1.7 4-1.7Z', fill:'#F2B53A', ...ln(1.3) }]],
    searching: [['circle', { cx:65.5, cy:10.5, r:4, fill:'#D7EEFA', ...ln(1.8) }], ['path', { d:'M68.5 13.5l3.5 3.5', fill:'none', ...ln(2.2) }]],
    writing: [['path', { d:'M62.5 15.5l1-3.5 7.5-7.5 2.5 2.5-7.5 7.5Z', fill:'#F2C14E', ...ln(1.5) }], ['path', { d:'M62.5 15.5l1-3.5 2.5 2.5Z', fill:INK }]],
    working: [['path', { d:'M65.5 4.5h3l.5 2.6 2.2 1.3 2.5-1 1.5 2.6-2 1.7v2.6l2 1.7-1.5 2.6-2.5-1-2.2 1.3-.5 2.6h-3l-.5-2.6-2.2-1.3-2.5 1-1.5-2.6 2-1.7v-2.6l-2-1.7 1.5-2.6 2.5 1 2.2-1.3Z', fill:'#A7AFC2', ...ln(1.4) }],
      ['circle', { cx:67, cy:12, r:2.4, fill:'#FFFFFF', ...ln(1.4) }]],
    done: [['circle', { cx:67, cy:12, r:9.5, fill:'#3FAE73', ...ln(2) }], ['path', { d:'M62.5 12.2l3 3 6-6', fill:'none', stroke:'#FFFFFF', 'stroke-width':2.6, 'stroke-linecap':'round', 'stroke-linejoin':'round' }]],
  };
  // One phase clock for every face: the workspace redraws rows on every event,
  // and a negative delay resumes each loop where it was instead of restarting.
  // A one-shot (the «done» hop) counts from when this wind first reached it.
  // Proactive voices (/api/winds/novetats: watch topics, reminders) the user has
  // not opened yet. Such a wind looks up with a speech bubble until its chat is
  // opened; nothing else changes and nothing is invented while it waits.
  const news = new Map(), newsKey = 'gregal.winds.newsSeen';
  let newsTimer = 0;
  function newsSeen() { try { return JSON.parse(root.localStorage?.getItem(newsKey) || '{}'); } catch (_) { return {}; } }
  function saveSeen(seen) { try { root.localStorage?.setItem(newsKey, JSON.stringify(seen)); } catch (_) {} }
  function refreshNews() {
    for (const svg of doc.querySelectorAll?.('svg.winds-face[data-wind]') || []) {
      const id = svg.getAttribute('data-wind'), mood = svg.getAttribute('data-mood');
      if (news.has(id) && mood === 'idle') setMood(svg, 'news');
      else if (!news.has(id) && mood === 'news') setMood(svg, 'idle');
    }
  }
  async function pollNews() {
    if (doc.hidden || typeof root.api !== 'function') return;
    const seen = newsSeen();
    // First run: only the last day counts, so an old backlog does not wake everyone.
    if (!seen.since) { seen.since = Math.floor(Date.now() / 1000) - 86400; saveSeen(seen); }
    try {
      const response = await root.api('/api/winds/novetats?des_de=' + seen.since);
      if (!response.ok) return;
      for (const n of (await response.json()).novetats || []) {
        if (n.wind && n.ts > (seen[n.wind] || seen.since)) news.set(n.wind, Math.max(n.ts, news.get(n.wind) || 0));
      }
      refreshNews();
    } catch (_) {}
  }
  function watchNews() {
    if (newsTimer) return;
    newsTimer = root.setInterval?.(pollNews, 60000) || 1;
    pollNews();
  }
  function seenNews(id) {
    if (!id) return;
    const seen = newsSeen();
    seen[id] = Math.max(news.get(id) || 0, Math.floor(Date.now() / 1000), seen[id] || 0);
    news.delete(id); saveSeen(seen); refreshNews();
  }
  const moodSince = new Map(), faceKeys = new WeakMap(), faceTones = new WeakMap();
  const clock = () => (root.performance?.now?.() ?? Date.now()) / 1000;
  function setMood(svg, mood, key = faceKeys.get(svg)) {
    mood = moods.includes(mood) ? mood : 'idle';
    const now = clock(), last = moodSince.get(key);
    if (mood !== 'idle' && last?.mood !== mood) moodSince.set(key, { mood, at:now });
    svg.setAttribute('data-mood', mood);
    // News has no text next to it in every row, so it carries its own label.
    svg.querySelector('title')?.remove();
    if (mood === 'news') {
      const label = String(tr('winds.hasNews', 'Té una novetat per explicar-te'));
      svg.removeAttribute('aria-hidden'); svg.setAttribute('role', 'img'); svg.setAttribute('aria-label', label);
      const title = doc.createElementNS(SVG, 'title'); title.textContent = label; svg.append(title);
    } else if (svg.getAttribute('role')) {
      svg.setAttribute('aria-hidden', 'true'); svg.removeAttribute('role'); svg.removeAttribute('aria-label');
    }
    svg.style.setProperty('--wf-since', mood === 'idle' ? '0s' : (moodSince.get(key).at - now).toFixed(2) + 's');
    svg.querySelector('.wf-badge')?.remove();
    if (badges[mood]) {
      const tone = faceTones.get(svg), badge = shapes(group(svg, 'wf-badge'), badges[mood], tone), tool = badge.querySelector('.wf-tool, .wf-pop');
      if (tool) shapes(tool, tools[mood], tone);
    }
    return svg;
  }
  let faceSerial = 0;
  function luminance(hex) {
    const linear = i => {
      const value = parseInt(hex.slice(i, i + 2), 16) / 255;
      return value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4;
    };
    return .2126 * linear(1) + .7152 * linear(3) + .0722 * linear(5);
  }
  // Silhouettes in face coordinates (the ball is r 33 around 40,40; the cloud is
  // Android's, r 30). Its parts share one user-space gradient, so they read as
  // a single shape.
  const bodies = {
    o: [['circle', { cx:40, cy:40, r:33 }]],
    n: [['rect', { x:4.3, y:31, width:71.4, height:38.1, rx:17.4 }], ['circle', { cx:21.7, cy:38.5, r:20.1 }], ['circle', { cx:42.4, cy:34.3, r:25.8 }], ['circle', { cx:59.8, cy:41.2, r:18 }]],
  };
  // `large` picks the level of detail: false is a clean face for lists and chat
  // rows; true is the whole figure on its own ground, with the tool of its
  // trade; 'figure' is that figure without ground, for scenes that draw theirs.
  function face(w, large, mood) {
    w = w || {};
    const s = style(w.style), kitKey = w.kit || w.specialty, kit = kits[kitKey], fill = color(w.color);
    const full = !!large, worn = full ? kit : null, cloud = s[2] === 'n', curl = s[2] === 's', R = cloud ? 30 : 33;
    const darkInk = '#202127';
    // Pick the same higher-contrast ink as Android's WindFace.
    const background = luminance(fill) + .05;
    const ink = background / (luminance(darkInk) + .05) >= 1.05 / background ? darkInk : '#FFFFFF';
    const light = mix(fill, '#FFFFFF', .3), deep = mix(fill, '#1B1530', .3), tone = mix(fill, '#1B1530', .8);
    const svg = doc.createElementNS(SVG, 'svg'), key = w.id || w.name || 'crew';
    svg.setAttribute('viewBox', '0 0 80 80'); svg.setAttribute('class', 'winds-face' + (large === true ? ' winds-face-large' : ''));
    svg.setAttribute('aria-hidden', 'true'); svg.setAttribute('focusable', 'false');
    if (kit) svg.setAttribute('data-kit', kitKey);
    if (w.id) svg.setAttribute('data-wind', w.id);
    if ((!mood || mood === 'idle') && news.has(w.id)) mood = 'news';
    let seed = 0; for (const c of String(key)) seed = (seed * 31 + c.charCodeAt(0)) % 997;
    svg.style.setProperty('--wf-phase', (-((clock() + seed / 83) % 12)).toFixed(2) + 's');
    faceKeys.set(svg, key); faceTones.set(svg, tone);
    // Volume: soft light from the top left, the wind's own deep tone below, a rim
    // of bounced light at the far edge and a satin sheen. No outline, no glare.
    const id = 'wf' + (++faceSerial), defs = svgNode('defs', {}), num = value => +value.toFixed(2);
    const paint = (suffix, attrs, stops) => {
      const gradient = svgNode('radialGradient', { id:id + suffix, ...attrs });
      shapes(gradient, stops.map(([offset, tint, opacity]) => ['stop', { offset, 'stop-color':tint, 'stop-opacity':opacity ?? 1 }]));
      defs.append(gradient);
      return 'url(#' + id + suffix + ')';
    };
    const space = { gradientUnits:'userSpaceOnUse' }, lx = num(40 - .38 * R), ly = num(40 - .5 * R);
    const volume = paint('v', { ...space, cx:lx, cy:ly, r:num(1.9 * R) }, [[0, light], [.45, fill], [1, deep]]);
    const glow = mix(fill, '#FFFFFF', .65);
    const rim = paint('r', { ...space, cx:num(40 - .22 * R), cy:num(40 - .26 * R), r:num((cloud ? 1.62 : 1.32) * R) }, [[.8, glow, 0], [1, glow, .36]]);
    const sheen = paint('s', { ...space, cx:lx, cy:ly, r:num(.5 * R) }, [[0, '#FFFFFF', .28], [1, '#FFFFFF', 0]]);
    const blush = paint('b', {}, [[0, '#E4577A', .5], [1, '#E4577A', 0]]);
    defs.append(shapes(svgNode('clipPath', { id:id + 'c' }), bodies[cloud ? 'n' : 'o']));
    svg.append(defs);
    if (large === true) {
      const ground = mix(fill, '#0B0C14', .72);
      shapes(svg, [['ellipse', { class:'wf-ground', cx:40, cy:72.4, rx:23, ry:4.4, fill:paint('g', {}, [[0, ground, .5], [1, ground, 0]]) }]]);
    }
    const figure = group(svg, 'wf-figure');
    // The whole figure is drawn a little smaller: room for hats, tools and marks.
    // A small face only makes room for its own curl.
    const head = group(figure, '', full ? { transform:'translate(4 3) scale(.9)' } : curl ? { transform:'translate(2 3.6) scale(.95)' } : {});
    const footY = cloud ? 69.8 : 69.4;
    shapes(head, [['ellipse', { class:'wf-foot wf-foot-l', cx:25.3, cy:footY, rx:7.1, ry:3.7, fill:deep }], ['ellipse', { class:'wf-foot wf-foot-r', cx:54.7, cy:footY, rx:7.1, ry:3.7, fill:deep }]]);
    // The curl of a gust on top: it changes the outline, never the face.
    if (curl) shapes(head, [['path', { class:'wf-curl', d:'M26.2 11.5C32.5 9 36.9 5.2 39.4 1.5C41.2-1.6 45.6-3.1 48.8-.9C52 1.6 51 6.1 47 6.4C45 6.5 44 4.9 45 3.4C42.8 4.6 43.2 8.4 46.9 9.9L53.8 11.6Z', fill:volume }]]);
    shapes(group(head, '', { fill:volume }), bodies[cloud ? 'n' : 'o']);
    shapes(group(head, '', { 'clip-path':'url(#' + id + 'c)' }), [
      ['rect', { x:-4, y:-4, width:88, height:88, fill:rim }],
      ['circle', { cx:lx, cy:ly, r:num(.5 * R), fill:sheen }]]);
    shapes(head, [
      ['ellipse', { cx:20.6, cy:48.8, rx:7, ry:4.6, fill:blush }],
      ['ellipse', { cx:59.4, cy:48.8, rx:7, ry:4.6, fill:blush }]]);
    // Two bold marks for eyes, as on Android; the figure also gets a catchlight.
    const eyes = group(group(head, 'wf-gaze'), 'wf-eyes'), ey = 38.4;
    for (const x of [30.4, 49.6]) {
      if (s[0] === 'm') shapes(eyes, [['path', { d:`M${x - 5} ${ey - 1}q5 5.6 10 0`, fill:'none', stroke:ink, 'stroke-width':3.2, 'stroke-linecap':'round' }]]);
      else if (s[0] === 'e') shapes(eyes, [['path', { d:`M${x} ${num(ey - 7.4)}l2.2 5.2 5.2 2.2-5.2 2.2-2.2 5.2-2.2-5.2-5.2-2.2 5.2-2.2Z`, fill:ink, stroke:ink, 'stroke-width':1.2, 'stroke-linejoin':'round' }]]);
      else {
        const half = s[0] === 'b' ? 5.4 : 4.2;
        shapes(eyes, [['rect', { x:num(x - half), y:num(ey - 7.8), width:half * 2, height:15.6, rx:half, fill:ink }]]);
        if (full) shapes(eyes, [['circle', { cx:num(x + half * .3), cy:num(ey - 4.1), r:num(half * .36), fill:ink === darkInk ? '#FFFFFF' : fill, opacity:'.5' }]]);
      }
    }
    // Content, closed eyes replace the chosen ones only while a step is done.
    shapes(group(head, 'wf-joy', { display:'none' }), [['path', { d:'M25.4 40.2q5-7 10 0M44.6 40.2q5-7 10 0', fill:'none', stroke:ink, 'stroke-width':3.2, 'stroke-linecap':'round' }]]);
    // Thin features vanish in a 30 px avatar: the small face draws them bolder.
    const mouth = group(head, 'wf-mouth'), line = { fill:'none', stroke:ink, 'stroke-width':full ? 2.5 : 3.1, 'stroke-linecap':'round' };
    if (s[1] === 'o') shapes(mouth, [['ellipse', { cx:40, cy:54.6, rx:4.8, ry:5.2, fill:ink }], ['ellipse', { cx:40, cy:57.4, rx:3.1, ry:2.2, fill:'#E8667E' }]]);
    else {
      if (s[1] === 't') shapes(mouth, [['path', { d:'M38.6 56.8q-.4 6.6 3.4 6.6t3.6-7.4Z', fill:'#E8667E' }]]);
      shapes(mouth, [['path', { d:s[1] === 'n' ? 'M34.6 54h10.8' : 'M31.8 52.8q8.2 10 16.4 0', ...line }]]);
    }
    shapes(group(head, 'wf-worry', { display:'none' }), [['path', { d:'M32.5 55.4q3.75-3.4 7.5 0t7.5 0', ...line }]]);
    if (worn?.head) shapes(head, worn.head, tone);
    if (worn?.prop) {
      const prop = shapes(group(figure, 'wf-prop'), worn.prop, tone);
      for (const [cx, cy] of worn.hands || []) shapes(prop, [['ellipse', { cx, cy, rx:4.6, ry:4, fill, stroke:mix(fill, '#1B1530', .45), 'stroke-width':1.4 }]]);
    }
    return setMood(svg, mood, key);
  }
  function status(id, text, error) {
    const n = $(id); paint(n, text || ''); n.classList.toggle('winds-error', !!error);
    n.setAttribute('role', error ? 'alert' : 'status');
  }
  function roster(ids) { return ids.map(id => state.winds.find(w => w.id === id)).filter(Boolean); }
  function row(title, subtitle, members, action, selected) {
    const b = button('', action, 'winds-row'); b.setAttribute('aria-pressed', String(!!selected));
    const faces = node('span', 'winds-face-stack'); members.slice(0, 3).forEach(w => faces.append(face(w)));
    const copy = node('span', 'winds-row-copy'); copy.append(node('strong', '', title), node('span', 'winds-muted', subtitle));
    b.append(faces, copy); return b;
  }
  function draw() {
    root.gregalWindDesk?.render(state.winds, face);
    const list = $('windsList'); list.replaceChildren();
    for (const w of state.winds) list.append(row(w.name, w.role || tr('winds.chatWithName', 'Conversa amb {name}', {name:w.name}), [w], () => openChat({ kind:'wind', id:w.id, name:w.name, members:[w.id] }), state.active?.kind === 'wind' && state.active.id === w.id));
    if (!state.winds.length) list.append(node('p', 'winds-empty', tr("winds.youDoNotHaveAnyWindsYetCreate", "La colla es prepara amb el teu primer encàrrec.")));
    const chats = $('windsConversations'); chats.replaceChildren();
    for (const c of state.conversations) chats.append(row(c.name, c.members.length > 1 ? tr('winds.groupCount', 'En grup · {count} vents', {count:c.members.length}) : tr('winds.namedConversation', 'Conversa · {name}', {name:roster(c.members)[0]?.name || tr("winds.oneWind", "Un vent")}), roster(c.members), () => openChat({ ...c, kind:'rotllana' }), state.active?.kind === 'rotllana' && state.active.id === c.id));
    if (!state.conversations.length) chats.append(node('p', 'winds-empty', tr("winds.newConversationsWillAppearHere", "Les noves converses apareixeran aquí.")));
    if (state.active) drawChatHeading();
  }
  function drawChatHeading() {
    const active = state.active, members = roster(active.members);
    if (active.kind === 'wind' && members[0]) active.name = members[0].name;
    paint($('windChatTitle'), active.name);
    paint($('windChatSubtitle'), active.kind === 'wind' ? (members[0]?.role || tr("winds.oneToOneConversation", "Conversa individual")) : members.map(w => w.name).join(' · '));
    $('windChatFaces').replaceChildren(...members.slice(0, 3).map(w => face(w)));
    $('windsChatMembers').replaceChildren(...members.map(w => button(tr('winds.profileOf', 'Perfil de {name}', {name:w.name}), e => openProfile(w.id, e.target), 'ghost winds-profile-button')));
    let edit = $('windsEditConversation');
    if (!edit) {
      edit = button(tr("winds.editTheGroupAndItsMembers", "Edita el grup i els membres"), () => conversationForm(state.active, edit), 'ghost winds-edit-conversation');
      edit.setAttribute('id', 'windsEditConversation'); edit.setAttribute('aria-haspopup', 'dialog'); edit.setAttribute('aria-controls', 'windsDialog');
      $('windChatCard').querySelector('header').append(edit);
    }
    edit.hidden = active.kind !== 'rotllana';
  }
  function load() {
    if (!$('windsList')) return Promise.resolve(false);
    if (state.loading) return state.loading;
    status('windsStatus', tr("winds.loadingYourWinds", "Carregant els teus vents…"));
    state.loading = (async () => {
      try {
        const [winds, chats] = await Promise.all([call('/api/winds'), call('/api/winds/rotllanes')]);
        state.winds = winds.winds || []; state.conversations = chats.rotllanes || [];
        draw(); status('windsStatus', ''); return true;
      } catch (_) { status('windsStatus', tr("winds.weCouldNotLoadYourWindsTryRefresh", "No hem pogut carregar els vents. Torna-ho a provar amb «Actualitza la llista»."), true); return false; }
      finally { state.loading = false; }
    })();
    return state.loading;
  }
  async function refreshAfterWrite() {
    if (state.loading) await state.loading;
    if (!await load()) throw new Error('catalogue-not-verified');
  }
  async function verifyWritten(action) {
    try { return await action(); }
    catch (error) { error.alreadySent = true; throw error; }
  }
  const chatKey = c => c.kind + ':' + c.id;
  const drafts = new Map(), pending = new Set();
  let chatVersion = 0;
  function historyURL(c) { return c.kind === 'wind' ? '/api/winds/chat?wind=' + encodeURIComponent(c.id) : '/api/winds/rotllana?rotlla=' + encodeURIComponent(c.id); }
  function busy() {
    const waiting = !!state.active && pending.has(chatKey(state.active));
    $('windChatSend').disabled = waiting;
    $('windChatInput').disabled = waiting;
    $('windsChatForm').setAttribute('aria-busy', String(waiting));
  }
  function drawHistory(history, active) {
    const hist = $('windChatHistory'); hist.replaceChildren();
    if (!history.length) hist.append(node('p', 'winds-empty', tr("winds.yourConversationStartsHereWhatWouldYouLike", "Aquí comença la conversa. Què us voleu explicar?")));
    for (const m of history) {
      const user = m.from === 'user';
      const w = state.winds.find(w => w.id === m.speaker_id) || (active.members.length === 1 ? roster(active.members)[0] : null);
      const name = user ? tr("winds.you", "Tu") : (m.speaker_name || w?.name || (m.from !== 'assistant' ? m.from : tr("winds.wind", "Vent")));
      // El text durador porta el prefix "Nom: " per compatibilitat; en pintar
      // es mostra el nom a la capçalera i el text sense el prefix.
      let text = m.text || '';
      if (!user && w && text.startsWith(w.name + ':')) text = text.slice(w.name.length + 1).trimStart();
      const msg = node('article', 'winds-message' + (user ? ' winds-message-user' : ''));
      const avatar = user ? node('span', 'winds-user-avatar', tr("winds.you", "Tu")) : face(w || { color:'#BCC4D4', style:'pno' });
      avatar.setAttribute('aria-hidden', 'true');
      const content = node('div', 'winds-message-content');
      content.append(node('strong', 'winds-speaker', name), node('p', 'winds-message-text', text));
      msg.append(avatar, content); hist.append(msg);
    }
    hist.scrollTop = hist.scrollHeight;
  }
  async function fetchHistory(active, version) {
    const d = await call(historyURL(active));
    if (chatVersion === version && state.active && chatKey(state.active) === chatKey(active)) drawHistory(d.history || [], active);
  }
  async function openChat(active) {
    if (state.active) drafts.set(chatKey(state.active), $('windChatInput').value);
    state.active = active; const version = ++chatVersion;
    if (active.kind === 'wind') seenNews(active.id);
    $('windsWelcome').hidden = true; $('windChatCard').hidden = false; $('windsProfile').hidden = true;
    ++profileVersion;
    $('windChatInput').value = drafts.get(chatKey(active)) || '';
    $('windChatHistory').replaceChildren();
    draw(); busy(); status('windsChatStatus', pending.has(chatKey(active)) ? tr("winds.waitingForAReply", "Esperant la resposta…") : tr("winds.loadingTheConversation", "Carregant la conversa…"));
    try { await fetchHistory(active, version); if (version === chatVersion && !pending.has(chatKey(active))) status('windsChatStatus', ''); }
    catch (_) { if (version === chatVersion) status('windsChatStatus', tr("winds.weCouldNotReadTheConversationOpenIt", "No hem pogut llegir la conversa. Torna-la a obrir per reintentar-ho."), true); }
    finally {
      if (version === chatVersion) {
        const panel = $('ventsPage'), heading = panel?.querySelector('.winds-conversation-heading');
        // Scroll only this panel, after history has established its final height.
        // scrollIntoView also scrolls the document and lifts the entire app shell.
        if (heading) panel.scrollTop += heading.getBoundingClientRect().top - panel.getBoundingClientRect().top;
      }
    }
  }
  async function send(event) {
    if (event) event.preventDefault();
    const active = state.active, input = $('windChatInput');
    if (!active || !input.value.trim() || pending.has(chatKey(active))) return;
    const key = chatKey(active), text = input.value.trim(), version = chatVersion;
    pending.add(key); drafts.set(key, input.value); busy(); status('windsChatStatus', tr("winds.waitingForAReply", "Esperant la resposta…"));
    const current = () => state.active && chatKey(state.active) === key;
    let posted = false;
    try {
      await call(active.kind === 'wind' ? '/api/winds/chat' : '/api/winds/rotllana', 'POST', active.kind === 'wind' ? {wind:active.id, message:text} : {rotllana:active.id, message:text});
      posted = true; drafts.delete(key);
      if (current()) input.value = '';
      const refreshVersion = current() ? ++chatVersion : version;
      await fetchHistory(active, refreshVersion);
      if (current() && refreshVersion === chatVersion) status('windsChatStatus', '');
    } catch (_) {
      if (current()) status('windsChatStatus', posted ? tr("winds.yourMessageWasSentButWeCouldNot", "El missatge s’ha enviat, però no hem pogut actualitzar la conversa. Torna-la a obrir.") : tr("winds.weCouldNotSendYourMessageYourDraft", "No hem pogut enviar el missatge. El text continua aquí; torna-ho a provar."), true);
    } finally { pending.delete(key); busy(); if (current()) input.focus(); }
  }
  let profileVersion = 0, profileOrigin = null;
  function closeProfile() { ++profileVersion; $('windsProfile').hidden = true; if (profileOrigin) profileOrigin.focus(); }
  async function openProfile(id, origin) {
    profileOrigin = origin || profileOrigin;
    const version = ++profileVersion, panel = $('windsProfile'); panel.hidden = false;
    panel.replaceChildren(node('p', 'winds-muted', tr("winds.loadingTheProfile", "Carregant el perfil…")));
    panel.setAttribute('aria-busy', 'true');
    try {
      const data = await call('/api/winds/profile?wind=' + encodeURIComponent(id));
      if (!data.wind || data.wind.id !== id) throw new Error('missing-profile');
      if (version === profileVersion) { renderProfile(data); panel.scrollIntoView?.({block:'start'}); $('windsProfileTitle').focus(); }
    } catch (_) {
      if (version === profileVersion) {
        const error = node('p', 'winds-status winds-error', tr("winds.weCouldNotReadTheProfileNoMemories", "No hem pogut llegir el perfil. No s’ha canviat cap record.")); error.setAttribute('role', 'alert');
        panel.replaceChildren(error, button(tr("winds.tryAgain", "Torna-ho a provar"), () => openProfile(id, origin)), button(tr("winds.closeProfile", "Tanca el perfil"), closeProfile));
      }
    } finally { if (version === profileVersion) panel.setAttribute('aria-busy', 'false'); }
  }
  function renderProfile(data) {
    const w = data.wind, panel = $('windsProfile'); panel.replaceChildren();
    const header = node('header', 'winds-profile-heading'); const title = node('h2', '', w.name);
    title.setAttribute('id', 'windsProfileTitle'); title.setAttribute('tabindex', '-1');
    const heading = node('div'); heading.append(title, node('p', 'winds-muted', w.role || tr("winds.aGregalWind", "Vent de Gregal")));
    header.append(face(w, true), heading, button(tr("winds.closeProfile", "Tanca el perfil"), closeProfile)); panel.append(header);
    const character = node('section', 'winds-profile-section');
    const edit = button(tr("winds.editCharacter", "Edita el caràcter"), () => windForm(w, edit)); edit.setAttribute('id', 'windsEditCharacter');
    character.append(node('h3', '', tr("winds.character", "Caràcter")), node('p', 'winds-memory-text', w.persona || tr("winds.youHaveNotDefinedACharacterYet", "Encara no has definit cap caràcter propi.")), edit);
    panel.append(character);
    const form = node('form', 'winds-profile-section'); form.setAttribute('id', 'windsMemoryForm');
    form.append(node('h3', '', tr("winds.whatWouldYouLikeThemToKnow", "Què vols que sàpiga?")), node('p', 'winds-muted', tr("winds.thisWindSSavedKnowledgeSavingReplacesThe", "Coneixement desat d’aquest vent. Desar substitueix tot aquest text; deixar-lo buit l’esborra.")));
    const knowledge = field(form, 'windKnowledge', tr("winds.windKnowledge", "Coneixement del vent"), 'textarea', data.knowledge || ''); knowledge.rows = 6;
    attribute(knowledge, 'placeholder', tr("winds.youHaveNotSavedAnyKnowledgeYet", "Encara no hi has desat cap coneixement."));
    const save = node('button', 'ghost', tr("winds.saveKnowledge", "Desa el coneixement")); save.setAttribute('type', 'submit');
    const feedback = node('p', 'winds-status'); feedback.setAttribute('id', 'windsMemoryStatus'); feedback.setAttribute('role', 'status'); feedback.setAttribute('aria-live', 'polite');
    form.append(save, feedback); panel.append(form);
    const memory = node('section', 'winds-profile-section');
    memory.append(node('h3', '', tr("winds.personalMemories", "Records personals")), node('p', 'winds-muted', tr("winds.personalMemorySharedWithThisWindExactlyAs", "Memòria personal compartida amb el vent, tal com està desada. Només de lectura.")), node('p', 'winds-memory-text', data.personal_memory || tr("winds.thereAreNoSavedPersonalMemoriesYet", "Encara no hi ha records personals desats.")));
    panel.append(memory);
    let saving = false;
    form.onsubmit = async e => {
      e.preventDefault(); if (saving) return;
      saving = true; save.disabled = true; knowledge.disabled = true; form.setAttribute('aria-busy', 'true');
      const version = profileVersion; status('windsMemoryStatus', tr("winds.savingKnowledge", "Desant el coneixement…"));
      let posted = false;
      try {
        await call('/api/winds/memory', 'POST', {wind:w.id, text:knowledge.value}); posted = true;
        const verified = await call('/api/winds/profile?wind=' + encodeURIComponent(w.id));
        if (version === profileVersion && !panel.hidden) { renderProfile(verified); status('windsMemoryStatus', tr("winds.knowledgeSaved", "Coneixement desat.")); }
      } catch (_) {
        if (version === profileVersion && !panel.hidden) status('windsMemoryStatus', posted ? tr("winds.theTextWasSavedButWeCouldNot", "S’ha desat el text, però no hem pogut tornar a llegir el perfil. Reobre’l per comprovar-lo.") : tr("winds.weCouldNotSaveTheKnowledgeYourText", "No hem pogut desar el coneixement. Conservem el text perquè ho puguis tornar a provar."), true);
      } finally { saving = false; save.disabled = false; knowledge.disabled = false; form.setAttribute('aria-busy', 'false'); }
    };
  }
  let dialogSaving = false, returnFocus = null;
  function menu(open) {
    $('windsNewMenu').hidden = !open; $('windNewBtn').setAttribute('aria-expanded', String(open));
    if (open) $('windsCreateWind').focus();
  }
  function field(parent, id, title, tag, value, type) {
    const wrap = node('label', 'winds-field'); wrap.setAttribute('for', id);
    wrap.append(node('span', '', title));
    const input = node(tag || 'input'); input.setAttribute('id', id); input.setAttribute('name', id);
    if (type) input.setAttribute('type', type);
    input.value = value || ''; wrap.append(input); parent.append(wrap); return input;
  }
  function select(parent, id, title, values, value) {
    const input = field(parent, id, title, 'select');
    for (const [key, label] of values) { const option = node('option', '', label); option.setAttribute('value', key); input.append(option); }
    input.value = value; return input;
  }
  // Keep these characters in step with Android's WindSuggestions. User-created
  // roles and personas are deliberately left untouched.
  const presetDefaults = [
    {name:'Marinada', color:'#EFC69A', style:'bsn', role:'cerques i continguts', persona:'Ets observadora i imaginativa. Investiga amb fonts, compara opcions i prepara continguts clars. Distingeix els fets de les hipòtesis.'},
    {name:'Xaloc', color:'#EFA68F', style:'eto', role:'música i descobertes', persona:'Ets curiós i juganer. Comparteixes música i descobertes amb entusiasme i cites les fonts quan investigues.'},
    {name:'Tramuntana', color:'#A8CDE2', style:'pso', role:'webs i codi', persona:'Ets precís, curiós i enginyós. Construeix solucions senzilles, cuida els detalls i comprova el que has fet amb les eines disponibles.'},
    {name:'Mestral', color:'#BFCFA4', style:'esn', role:'revisió i qualitat', persona:'Ets reflexiu, franc i pacient. Revisa el resultat amb criteris concrets, detecta errors i explica com corregir-los amb respecte.'},
    {name:'Llebeig', color:'#E8AEC2', style:'pto', role:'històries i cultura', persona:'T’agraden les històries, l’art i la cultura. Ets càlid, imaginatiu i distingeixes els fets de la ficció.'},
    {name:'Gregal', color:'#C1B4DE', style:'mso', role:'organització del dia a dia', persona:'Ets proper, atent i serè. Ajuda a concretar encàrrecs, organitzar la feina i seguir-ne el fil. Explica què està resolt i què necessita una decisió.'},
    {name:'Ponent', color:'#EACF87', style:'msn', role:'llibres i tardes', persona:'Ets reflexiu i tranquil. T’agrada conversar sobre llibres i idees amb preguntes obertes.'},
    {name:'Migjorn', color:'#9ED3C5', style:'bss', role:'natura i escapades', persona:'Ets alegre i observador. T’agrada la natura i proposes descobertes adaptades al que l’usuari explica.'}
  ];
  const presetTranslations = [
    ['winds.researchAndContent','winds.youAreObservantAndImaginativeResearchWithSources'],
    ['winds.musicAndDiscoveries','winds.youAreCuriousAndPlayfulShareMusicAnd'],
    ['winds.websitesAndCode','winds.youArePreciseCuriousAndResourcefulBuildSimple'],
    ['winds.reviewAndQuality','winds.youAreThoughtfulCandidAndPatientReviewResults'],
    ['winds.storiesAndCulture','winds.youEnjoyStoriesArtAndCultureYouAre'],
    ['winds.everydayOrganisation','winds.youAreWarmAttentiveAndCalmHelpDefine'],
    ['winds.booksAndAfternoons','winds.youAreThoughtfulAndCalmYouEnjoyDiscussing'],
    ['winds.natureAndGetaways','winds.youAreCheerfulAndObservantYouEnjoyNature']
  ];
  function presets() { return presetDefaults.map((p, i) => ({...p,
    role:tr(presetTranslations[i][0], p.role), persona:tr(presetTranslations[i][1], p.persona)
  })); }
  function windGallery() {
    menu(false); returnFocus = $('windNewBtn');
    const body = $('windsDialogBody'), title = node('h2', '', tr('winds.whichWindWillJoinYou', 'Quin vent et farà companyia?'));
    title.setAttribute('id', 'windsDialogTitle');
    const gallery = node('div', 'winds-preset-gallery'); gallery.setAttribute('id', 'windsPresetGallery');
    gallery.setAttribute('role', 'group'); attribute(gallery, 'aria-label', tr('winds.chooseACharacterToGetStarted', 'Tria un personatge per començar'));
    // Els noms vénen del servidor, al gust de com es diuen els ajudants
    // (fullets, minairons…); el rol i la persona són els de la plantilla.
    const list = presets(), labels = [];
    list.forEach((preset, i) => {
      const choice = button('', () => windForm(null, $('windNewBtn'), list[i]), 'winds-preset');
      const label = node('strong', '', preset.name); labels.push(label);
      choice.append(face(preset, true), label, node('span', 'winds-muted', preset.role));
      gallery.append(choice);
    });
    window.gregalCollective?.names(list.length).then(names => {
      names.slice(0, list.length).forEach((n, i) => { list[i] = { ...list[i], name: n }; labels[i].textContent = n; });
    });
    const actions = node('div', 'winds-dialog-actions');
    actions.append(button(tr('winds.cancel', 'Cancel·la'), () => $('windsDialog').close()), button(tr('winds.createACustomWind', 'Crea un vent personalitzat'), () => windForm(), 'ghost winds-primary'));
    body.replaceChildren(title, node('p', 'winds-muted winds-gallery-intro', tr('winds.chooseAFriendlyFaceAndMakeItYours', 'Tria una cara amiga i fes-la teva: podràs canviar-ne el nom, el caràcter i tots els detalls.')), gallery, actions);
    if (!$('windsDialog').open) $('windsDialog').showModal();
    gallery.querySelector('button').focus();
  }
  function dialogForm(title, id, submitText, save, focusBack) {
    menu(false); returnFocus = focusBack || $('windNewBtn');
    const body = $('windsDialogBody'); body.replaceChildren(node('h2', '', title)); body.children[0].setAttribute('id', 'windsDialogTitle');
    const form = node('form', 'winds-form'); form.setAttribute('id', id);
    const fields = node('div', 'winds-form-fields');
    const feedback = node('p', 'winds-status'); feedback.setAttribute('role', 'status'); feedback.setAttribute('aria-live', 'polite');
    const actions = node('div', 'winds-dialog-actions');
    const cancel = button('Cancel·la', () => $('windsDialog').close());
    const submit = node('button', 'ghost winds-primary', submitText); submit.setAttribute('type', 'submit');
    actions.append(cancel, submit); form.append(fields, feedback, actions); body.append(form);
    form.onsubmit = async e => {
      e.preventDefault(); if (dialogSaving) return;
      dialogSaving = true; submit.disabled = true; cancel.disabled = true; form.setAttribute('aria-busy', 'true');
      paint(feedback, tr('winds.saving', 'Desant…')); feedback.classList.remove('winds-error'); feedback.setAttribute('role', 'status');
      try { await save(feedback); }
      catch (error) { paint(feedback, error.alreadySent ? tr('winds.yourChangesWereSentButWeCouldNot', 'Els canvis s’han enviat, però no hem pogut comprovar-los. Actualitza la llista abans de tornar-ho a provar.') : tr('winds.weCouldNotSaveYourChangesCheckYour', 'No hem pogut desar els canvis. Revisa la connexió i torna-ho a provar.')); feedback.classList.add('winds-error'); feedback.setAttribute('role', 'alert'); }
      finally { dialogSaving = false; submit.disabled = false; cancel.disabled = false; form.setAttribute('aria-busy', 'false'); }
    };
    return { fields, feedback, show() { if (!$('windsDialog').open) $('windsDialog').showModal(); const first = fields.querySelector('input,textarea,select'); if (first) first.focus(); } };
  }
  function windForm(w, focusBack, preset) {
    const editing = !!w; w = w || preset || {name:'', role:'', persona:'', color:'#F5B45D', style:'pso'};
    let name, role, persona, tint, eyes, mouth, shape;
    const form = dialogForm(editing ? tr('winds.characterOf', 'El caràcter de {name}', {name:w.name}) : tr('winds.aNewWind', 'Un vent nou'), 'windsWindForm', editing ? tr('winds.saveChanges', 'Desa els canvis') : tr('winds.createWind', 'Crea el vent'), async feedback => {
      if (!name.value.trim()) { paint(feedback, tr('winds.giveYourWindAName', 'Posa un nom al vent.')); name.focus(); return; }
      const payload = {name:name.value.trim(), role:role.value.trim(), color:color(tint.value), persona:persona.value.trim(), style:eyes.value + mouth.value + shape.value};
      if (editing) payload.id = w.id;
      const saved = await call('/api/winds', 'POST', payload);
      await verifyWritten(async () => {
        if (!saved.wind?.id) throw new Error('missing-id');
        const verified = await call('/api/winds/profile?wind=' + encodeURIComponent(saved.wind.id));
        if (!verified.wind || verified.wind.id !== saved.wind.id) throw new Error('missing-profile');
        await refreshAfterWrite();
        if (editing && !$('windsProfile').hidden) { renderProfile(verified); returnFocus = $('windsEditCharacter'); }
      });
      $('windsDialog').close();
      status('windsStatus', editing ? tr('winds.characterUpdated', 'Caràcter actualitzat.') : tr('winds.newWindIsReady', 'El nou vent ja és aquí. Tria’l per començar a parlar.'));
    }, focusBack);
    const preview = node('div', 'winds-face-preview'); preview.setAttribute('id', 'windFacePreview'); form.fields.append(preview);
    name = field(form.fields, 'windName', tr('winds.name', 'Nom'), 'input', w.name, 'text'); name.required = true; name.maxLength = 100;
    role = field(form.fields, 'windRole', tr('winds.howCanTheyHelpYou', 'En què et pot ajudar?'), 'input', w.role, 'text'); attribute(role, 'placeholder', tr('winds.researchWebsitesCodeReviews', 'Cerques, webs, codi, revisió…'));
    persona = field(form.fields, 'windPersona', tr('winds.characterAndVoice', 'Caràcter i manera de parlar'), 'textarea', w.persona); persona.rows = 4;
    attribute(persona, 'placeholder', tr('winds.whatShouldTheyBeLikeWhatMattersTo', 'Com vols que sigui? Què li importa?'));
    tint = field(form.fields, 'windColor', tr('winds.colour', 'Color'), 'input', color(w.color), 'color');
    const choices = node('div', 'winds-face-choices'); form.fields.append(choices); const s = style(w.style);
    eyes = select(choices, 'windEyes', tr('winds.eyes', 'Ulls'), [['p',tr('winds.dots','Punts')],['b',tr('winds.wideOpen','Ben oberts')],['e',tr('winds.starry','Estrellats')],['m',tr('winds.halfClosed','Mig tancats')]], s[0]);
    mouth = select(choices, 'windMouth', tr('winds.mouth', 'Boca'), [['s',tr('winds.smile','Somriure')],['o',tr('winds.open','Oberta')],['n',tr('winds.relaxed','Tranquil·la')],['t',tr('winds.tongue','Llengua')]], s[1]);
    shape = select(choices, 'windShape', tr('winds.shape', 'Forma'), [['o',tr('winds.ball','Bola')],['n',tr('winds.cloud','Núvol')],['s',tr('winds.spiral','Espiral')]], s[2]);
    const update = () => preview.replaceChildren(face({color:tint.value, style:eyes.value + mouth.value + shape.value}, true));
    [tint, eyes, mouth, shape].forEach(n => { n.addEventListener('change', update); n.addEventListener('input', update); }); update(); form.show();
  }
  function conversationForm(conversation, focusBack) {
    const editing = !!conversation, minimum = conversation?.members.length > 1 ? 2 : 1;
    let name; const checks = [];
    const form = dialogForm(editing ? tr('winds.editTheGroupAndItsMembers','Edita el grup i els membres') : tr('winds.aNewConversation','Una conversa nova'), 'windsChatFormDialog', editing ? tr('winds.saveChanges','Desa els canvis') : tr('winds.createConversation','Crea la conversa'), async feedback => {
      const members = checks.filter(c => c.checked).map(c => c.value);
      if (!name.value.trim() || members.length < minimum) { paint(feedback, minimum === 2 ? tr('winds.chooseANameAndAtLeastTwoWinds','Tria un nom i almenys dos vents per a la conversa en grup.') : tr('winds.chooseANameAndAtLeastOneWind','Tria un nom i almenys un vent per conversar.')); return; }
      const payload = {name:name.value.trim(), members};
      if (editing) payload.id = conversation.id;
      const saved = await call('/api/winds/rotllanes', 'POST', payload);
      const target = await verifyWritten(async () => {
        if (!saved.rotllana?.id || (editing && saved.rotllana.id !== conversation.id)) throw new Error('missing-id');
        // Read catalogue and history back; never represent a local-only chat as saved.
        await refreshAfterWrite();
        const persisted = state.conversations.find(c => c.id === saved.rotllana.id);
        if (!persisted || persisted.name !== payload.name || persisted.members.length !== members.length || !members.every(id => persisted.members.includes(id))) throw new Error('not-persisted');
        return persisted;
      });
      // A slow save must not take the user back from another conversation.
      if (!editing || (state.active?.kind === 'rotllana' && state.active.id === conversation.id)) {
        await openChat({...target, kind:'rotllana'});
        if (editing) returnFocus = $('windsEditConversation');
      } else returnFocus = $('windNewBtn');
      $('windsDialog').close();
    }, focusBack);
    name = field(form.fields, 'windsConversationName', tr('winds.conversationName','Nom de la conversa'), 'input', conversation?.name || '', 'text'); name.required = true;
    form.fields.append(node('p', 'winds-muted', minimum === 2 ? tr('winds.aGroupMustKeepAtLeastTwoWinds','Una conversa en grup ha de conservar almenys dos vents.') : tr('winds.chooseOneWindForAPrivateChatOr','Un sol vent per parlar a soles; més d’un per parlar-hi en grup.')));
    const group = node('fieldset', 'winds-picker'); group.append(node('legend', '', tr('winds.whoWillJoin','Qui hi serà?')));
    for (const w of state.winds) {
      const label = node('label', 'winds-member-choice'); const check = node('input'); check.setAttribute('type', 'checkbox'); check.setAttribute('name', 'members'); check.value = w.id;
      check.checked = !!conversation?.members.includes(w.id);
      label.append(check, face(w), node('span', '', w.name)); checks.push(check); group.append(label);
    }
    if (!state.winds.length) group.append(node('p', 'winds-empty', tr('winds.youDoNotHaveAnyWindsYetCancel','Encara no tens cap vent. Cancel·la i crea’n un des de «Nou».')));
    form.fields.append(group); form.show();
  }
  let bound = false;
  function bind() {
    if (bound) return; bound = true;
    $('windsRefresh').onclick = load;
    $('windNewBtn').onclick = () => menu($('windsNewMenu').hidden);
    $('windsCreateWind').onclick = windGallery;
    $('windsCreateChat').onclick = () => conversationForm();
    $('windsDialog').addEventListener('cancel', e => { if (dialogSaving) e.preventDefault(); });
    $('windsDialog').addEventListener('close', () => { if (returnFocus) returnFocus.focus(); });
    doc.addEventListener('click', e => { if (!$('windsNewMenu').hidden && !$('windsNewMenu').contains(e.target) && !$('windNewBtn').contains(e.target)) menu(false); });
    doc.addEventListener('keydown', e => { if (e.key === 'Escape' && !$('windsNewMenu').hidden) { menu(false); $('windNewBtn').focus(); } });
    $('windsChatForm').onsubmit = send;
    $('windChatInput').addEventListener('keydown', e => { if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); send(); } });
  }
  let startingCrew;
  root.gregalWindSetup={ensureReady(){
    if(startingCrew) return startingCrew;
    startingCrew=(async()=>{
      if(!await load()) throw new Error('catalogue-not-verified');
      if(state.winds.length) return;
      const defaults=presets();
      for(const index of [2,0,3]) await call('/api/winds','POST',defaults[index]);
      await refreshAfterWrite();
    })().finally(()=>{startingCrew=null;});
    return startingCrew;
  }};
  root.gregalWinds={face, create:windGallery, mood:setMood, moodOf, watchNews, seen:seenNews};
  root.loadWindsPage = function () { bind(); return load(); };
})(window);
