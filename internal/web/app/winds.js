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
    books: { hands:[[17,67],[63,67]], prop:[
      ['path', { d:'M17 61Q28 56 40 61Q52 56 63 61V75Q52 71 40 76Q28 71 17 75Z', fill:'#5E86D6', ...ln() }],
      ['path', { d:'M20 60Q30 56 40 60V73Q30 69 20 72Z', fill:'#FFF8EE', ...ln(1.6) }],
      ['path', { d:'M60 60Q50 56 40 60V73Q50 69 60 72Z', fill:'#FFF8EE', ...ln(1.6) }],
      ['path', { d:'M24 63q6-2 12 0M24 67q6-2 12 0M44 63q6-2 12 0M44 67q6-2 12 0', fill:'none', stroke:'#A79FBC', 'stroke-width':1.3, 'stroke-linecap':'round' }]] },
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
    review: { hands:[[52,63]], head:[
      ['circle', { cx:29, cy:36, r:8.5, fill:'#FFFFFF', 'fill-opacity':'.25', ...ln(2.2) }],
      ['circle', { cx:51, cy:36, r:8.5, fill:'#FFFFFF', 'fill-opacity':'.25', ...ln(2.2) }],
      ['path', { d:'M37.5 35q2.5-2 5 0M20.5 34.5l-6-2M59.5 34.5l6-2', fill:'none', ...ln(2) }]], prop:[
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
      ['rect', { x:21, y:53, width:38, height:19, rx:3, fill:'#3A3F5C', ...ln() }],
      ['path', { d:'M33 58.5l-4 4 4 4M47 58.5l4 4-4 4M42 57.5l-4 10', fill:'none', stroke:'#8EE3B0', 'stroke-width':2.2, 'stroke-linecap':'round', 'stroke-linejoin':'round' }],
      ['path', { d:'M14 72H66L63 77H17Z', fill:'#C3C9D9', ...ln() }]] },
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
  // Darker tone of the same colour for paws and feet.
  function shade(hex, amount) {
    const n = parseInt(hex.slice(1), 16);
    return '#' + [n >> 16, n >> 8 & 255, n & 255].map(c => Math.round(c * (1 - amount)).toString(16).padStart(2, '0')).join('');
  }
  function svgNode(tag, attrs) {
    const el = doc.createElementNS(SVG, tag);
    for (const [key, value] of Object.entries(attrs)) el.setAttribute(key, value);
    return el;
  }
  function shapes(parent, list) { for (const [tag, attrs] of list) parent.append(svgNode(tag, attrs)); return parent; }
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
  const moodSince = new Map(), faceKeys = new WeakMap();
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
      const badge = shapes(group(svg, 'wf-badge'), badges[mood]), tool = badge.querySelector('.wf-tool, .wf-pop');
      if (tool) shapes(tool, tools[mood]);
    }
    return svg;
  }
  function face(w, large, mood) {
    w = w || {};
    const s = style(w.style), kit = kits[w.specialty], fill = color(w.color);
    const svg = doc.createElementNS(SVG, 'svg'), key = w.id || w.name || 'crew';
    svg.setAttribute('viewBox', '0 0 80 80'); svg.setAttribute('class', 'winds-face' + (large ? ' winds-face-large' : ''));
    svg.setAttribute('aria-hidden', 'true'); svg.setAttribute('focusable', 'false');
    if (kit) svg.setAttribute('data-kit', w.specialty);
    if (w.id) svg.setAttribute('data-wind', w.id);
    if ((!mood || mood === 'idle') && news.has(w.id)) mood = 'news';
    let seed = 0; for (const c of String(key)) seed = (seed * 31 + c.charCodeAt(0)) % 997;
    svg.style.setProperty('--wf-phase', (-((clock() + seed / 83) % 12)).toFixed(2) + 's');
    faceKeys.set(svg, key);
    if (large) shapes(svg, [['ellipse', { cx:40, cy:kit ? 76 : 74, rx:23, ry:3, fill, opacity:'.15' }]]);
    const figure = group(svg, 'wf-figure');
    if (kit) shapes(figure, [['ellipse', { class:'wf-foot wf-foot-l', cx:30, cy:71, rx:6.5, ry:3.6, fill:shade(fill, .2) }], ['ellipse', { class:'wf-foot wf-foot-r', cx:50, cy:71, rx:6.5, ry:3.6, fill:shade(fill, .2) }]]);
    const head = group(figure, '', kit ? { transform:'translate(4 3) scale(.9)' } : {});
    if (s[2] === 'n') shapes(head, [['path', { d:'M18 62C1 62 1 37 17 34C14 13 43 8 50 24C69 16 83 39 68 49C76 66 49 74 40 63Z', fill }]]);
    else shapes(head, [['circle', { cx:40, cy:40, r:33, fill }]]);
    if (s[2] === 's') shapes(head, [['path', { d:'M41 14C67 14 72 56 50 64C28 74 12 51 19 33C25 19 47 19 53 31', fill:'none', stroke:'#FFFFFF', 'stroke-opacity':'.35', 'stroke-width':3 }]]);
    shapes(head, [
      ['ellipse', { cx:28, cy:21, rx:9, ry:5, fill:'#FFFFFF', opacity:'.3' }],
      ['ellipse', { cx:22, cy:45, rx:5, ry:3, fill:'#DC6784', opacity:'.16' }],
      ['ellipse', { cx:58, cy:45, rx:5, ry:3, fill:'#DC6784', opacity:'.16' }]]);
    const eyes = group(group(head, 'wf-gaze'), 'wf-eyes');
    for (const x of [29, 51]) {
      if (s[0] === 'm') shapes(eyes, [['path', { d:`M${x-4} 36q4 5 8 0`, fill:'none', stroke:INK, 'stroke-width':3, 'stroke-linecap':'round' }]]);
      else if (s[0] === 'e') shapes(eyes, [['path', { d:`M${x} 29l2 5 5 2-5 2-2 5-2-5-5-2 5-2Z`, fill:INK }]]);
      else shapes(eyes, [['ellipse', { cx:x, cy:36, rx:s[0] === 'b' ? 4 : 2.5, ry:s[0] === 'b' ? 6 : 3.5, fill:INK }]]);
    }
    // Content, closed eyes replace the chosen ones only while a step is done.
    shapes(group(head, 'wf-joy', { display:'none' }), [['path', { d:'M25 37q4-6 8 0M47 37q4-6 8 0', fill:'none', stroke:INK, 'stroke-width':3, 'stroke-linecap':'round' }]]);
    const mouth = group(head, 'wf-mouth');
    if (s[1] === 'o') shapes(mouth, [['ellipse', { cx:40, cy:51, rx:5, ry:6, fill:INK }]]);
    else shapes(mouth, [['path', { d:s[1] === 'n' ? 'M35 50h10' : 'M32 47q8 12 16 0', fill:'none', stroke:INK, 'stroke-width':3, 'stroke-linecap':'round' }]]);
    if (s[1] === 't') shapes(mouth, [['path', { d:'M39 52q0 10 6 6l1-7', fill:'#DC6784' }]]);
    shapes(group(head, 'wf-worry', { display:'none' }), [['path', { d:'M33 51q3.5-3 7 0t7 0', fill:'none', stroke:INK, 'stroke-width':3, 'stroke-linecap':'round' }]]);
    if (kit?.head) shapes(head, kit.head);
    if (kit?.prop) {
      const prop = shapes(group(figure, 'wf-prop'), kit.prop);
      for (const [cx, cy] of kit.hands || []) shapes(prop, [['ellipse', { cx, cy, rx:4.6, ry:4, fill, stroke:shade(fill, .35), 'stroke-width':1.4 }]]);
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
    for (const c of state.conversations) chats.append(row(c.name, c.members.length > 1 ? tr('winds.groupCount', 'Rotllana · {count} vents', {count:c.members.length}) : tr('winds.namedConversation', 'Conversa · {name}', {name:roster(c.members)[0]?.name || tr("winds.oneWind", "Un vent")}), roster(c.members), () => openChat({ ...c, kind:'rotllana' }), state.active?.kind === 'rotllana' && state.active.id === c.id));
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
    for (const preset of presets()) {
      const choice = button('', () => windForm(null, $('windNewBtn'), presets().find(p => p.name === preset.name)), 'winds-preset');
      choice.append(face(preset, true), node('strong', '', preset.name), node('span', 'winds-muted', preset.role));
      gallery.append(choice);
    }
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
      if (!name.value.trim() || members.length < minimum) { paint(feedback, minimum === 2 ? tr('winds.chooseANameAndAtLeastTwoWinds','Tria un nom i almenys dos vents per a la rotllana.') : tr('winds.chooseANameAndAtLeastOneWind','Tria un nom i almenys un vent per conversar.')); return; }
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
    form.fields.append(node('p', 'winds-muted', minimum === 2 ? tr('winds.aGroupMustKeepAtLeastTwoWinds','Una rotllana ha de conservar almenys dos vents.') : tr('winds.chooseOneWindForAPrivateChatOr','Un sol vent per parlar a soles; més d’un per fer una rotllana.')));
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
