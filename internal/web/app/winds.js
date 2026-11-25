// Vents: same authenticated API/session as the host app. All remote text is
// rendered with textContent; SVG shapes and attributes are authored here only.
(function (root) {
  'use strict';
  const doc = root.document;
  const i18n = () => root.gregalWindI18n;
  const tr = (key, fallback, vars) => i18n()?.text(key, fallback, vars) ?? interpolate(fallback, vars);
  function interpolate(value, vars) { return String(value).replace(/\{([\w]+)\}/g, (_, key) => vars?.[key] ?? `{${key}}`); }
  function paint(element, value) { if (i18n()) i18n().paint(element, value); else element.textContent = String(value ?? ''); }
  function attribute(element, name, value) { if (i18n()) i18n().attribute(element, name, value); else element.setAttribute(name, String(value ?? '')); }
  function node(tag, cls, text) {
    const n = doc.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) paint(n, text);
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
    cooking: { hands:[[52,67]], head:[
      ['path', { d:'M18 21C10 16 13 6 22 7C24 1 34-1 40 3C46-1 56 1 58 7C67 6 70 16 62 21Q40 12 18 21Z', fill:'#FFFFFF', ...ln() }],
      ['path', { d:'M16 20Q40 9 64 20V27Q40 16 16 27Z', fill:'#FFFFFF', ...ln() }],
      ['path', { d:'M28 15.5v6M40 13v6M52 15.5v6', fill:'none', stroke:'#C3C9D9', 'stroke-width':1.6, 'stroke-linecap':'round' }]], prop:[
      ['path', { d:'M59 61.5L51 67.5', stroke:'#8A5A3B', 'stroke-width':4.4, 'stroke-linecap':'round' }],
      ['circle', { cx:66, cy:56, r:9.5, fill:'#3A3F5C', ...ln() }],
      ['path', { d:'M60.5 55.5c0-4 4-6 7-4.5 3-.5 5.5 2.5 4 5.5 1 3-2.5 5-5 4-3 1.5-6.5-1-6-5Z', fill:'#FFFFFF' }],
      ['circle', { cx:66.5, cy:56, r:2.7, fill:'#F5B731' }]] },
    weather: { hands:[[65,60]], prop:[
      ['path', { d:'M65 45V66a3 3 0 0 1-6 0', fill:'none', ...ln(2.6) }],
      ['path', { d:'M65 33v-2.5', fill:'none', ...ln(2.4) }],
      ['path', { d:'M52.5 45A12.5 12 0 0 1 77.5 45Q74.4 42.5 71.25 45Q68.1 42.5 65 45Q61.9 42.5 58.75 45Q55.6 42.5 52.5 45Z', fill:'#5E86D6', ...ln() }],
      ['path', { d:'M65 33Q59.5 37 58.75 45M65 33Q70.5 37 71.25 45', fill:'none', ...ln(1.6) }]] },
    travel: { hands:[[51,62]], prop:[
      ['path', { d:'M57.5 52v-3a3 3 0 0 1 3-3h3a3 3 0 0 1 3 3v3', fill:'none', ...ln(2.4) }],
      ['rect', { x:50, y:52, width:24, height:20, rx:3.5, fill:'#F0A04B', ...ln() }],
      ['path', { d:'M56 53v18M68 53v18', stroke:'#A8612E', 'stroke-width':2.4 }],
      ['circle', { cx:62, cy:62, r:3, fill:'#FFF8EE' }]] },
    fitness: { hands:[[63,64]], head:[
      ['path', { d:'M11 24Q40 11 69 24L70 30.5Q40 18 10 30.5Z', fill:'#2FB58A', ...ln(2) }],
      ['path', { d:'M10.6 27.3Q40 14.6 69.4 27.3', fill:'none', stroke:'#FFFFFF', 'stroke-width':1.8, 'stroke-linecap':'round' }],
      ['path', { d:'M11 26.5L3.5 24L4.5 30.5ZM11 28.5L5.5 34L9.5 35.5Z', fill:'#2FB58A', ...ln(2) }]], prop:[
      ['path', { d:'M53 64H73', fill:'none', ...ln(3) }],
      ['rect', { x:49, y:56, width:5, height:16, rx:2, fill:'#E5484D', ...ln() }],
      ['rect', { x:72, y:56, width:5, height:16, rx:2, fill:'#E5484D', ...ln() }],
      ['rect', { x:54, y:58.5, width:3.5, height:11, rx:1.5, fill:'#F28C6B', ...ln(1.6) }],
      ['rect', { x:68.5, y:58.5, width:3.5, height:11, rx:1.5, fill:'#F28C6B', ...ln(1.6) }]] },
    finance: { hands:[[51,67]], prop:[
      ['rect', { x:54, y:67, width:4.5, height:7, rx:1.5, fill:'#F4A3B5', ...ln(2) }],
      ['rect', { x:64.5, y:67, width:4.5, height:7, rx:1.5, fill:'#F4A3B5', ...ln(2) }],
      ['path', { d:'M63.5 56L66 49.5L71 56Z', fill:'#F4A3B5', ...ln(2) }],
      ['ellipse', { cx:61, cy:62.5, rx:11.5, ry:9, fill:'#F4A3B5', ...ln() }],
      ['ellipse', { cx:72.5, cy:63, rx:3, ry:3.6, fill:'#F7C3CF', ...ln(2) }],
      ['path', { d:'M71.6 62h.01M73.4 64h.01M67 59.5h.01', stroke:INK, 'stroke-width':2.2, 'stroke-linecap':'round' }],
      ['path', { d:'M56.5 54.5h6', fill:'none', ...ln(2) }],
      ['circle', { cx:59.5, cy:48, r:4, fill:'#F5C042', ...ln(2) }]] },
    movies: { hands:[[51,66]], head:[
      ['rect', { x:21, y:29, width:18, height:16, rx:3.5, fill:'#E5484D', 'fill-opacity':'.45' }],
      ['rect', { x:41, y:29, width:18, height:16, rx:3.5, fill:'#3FB6D9', 'fill-opacity':'.45' }],
      ['path', { d:'M24.5 29h11a3.5 3.5 0 0 1 3.5 3.5v9a3.5 3.5 0 0 1-3.5 3.5h-11a3.5 3.5 0 0 1-3.5-3.5v-9a3.5 3.5 0 0 1 3.5-3.5ZM44.5 29h11a3.5 3.5 0 0 1 3.5 3.5v9a3.5 3.5 0 0 1-3.5 3.5h-11a3.5 3.5 0 0 1-3.5-3.5v-9a3.5 3.5 0 0 1 3.5-3.5ZM39 35h2M21 34l-7-2.5M59 34l7-2.5', fill:'none', ...ln(5.2) }],
      ['path', { d:'M24.5 29h11a3.5 3.5 0 0 1 3.5 3.5v9a3.5 3.5 0 0 1-3.5 3.5h-11a3.5 3.5 0 0 1-3.5-3.5v-9a3.5 3.5 0 0 1 3.5-3.5ZM44.5 29h11a3.5 3.5 0 0 1 3.5 3.5v9a3.5 3.5 0 0 1-3.5 3.5h-11a3.5 3.5 0 0 1-3.5-3.5v-9a3.5 3.5 0 0 1 3.5-3.5ZM39 35h2M21 34l-7-2.5M59 34l7-2.5', fill:'none', stroke:'#FFFFFF', 'stroke-width':2.4, 'stroke-linecap':'round', 'stroke-linejoin':'round' }]], prop:[
      ['rect', { x:50, y:56, width:24, height:16, rx:2.5, fill:'#3A3F5C' }],
      ['path', { d:'M50 56h24v5H50Z', fill:'#FFFFFF' }],
      ['path', { d:'M54 56h4l-3 5h-4ZM62 56h4l-3 5h-4ZM70 56h4l-3 5h-4Z', fill:INK }],
      ['path', { d:'M54 65.5h8M54 68.5h12', fill:'none', stroke:'#A79FBC', 'stroke-width':1.6, 'stroke-linecap':'round' }],
      ['rect', { x:50, y:56, width:24, height:16, rx:2.5, fill:'none', ...ln() }],
      ['path', { d:'M50 55L73.2 48.8L71.9 44L48.7 50.2Z', fill:'#FFFFFF' }],
      ['path', { d:'M54.8 53.7L57.7 52.9L59.3 47.4L56.4 48.1ZM60.6 52.2L63.5 51.4L65.1 45.8L62.2 46.6ZM66.4 50.6L69.3 49.8L70.9 44.3L68 45Z', fill:INK }],
      ['path', { d:'M50 55L73.2 48.8L71.9 44L48.7 50.2Z', fill:'none', ...ln() }]] },
  };
  // Moods mirror observed work only: the chat's pending phase or a live step's
  // state and activity. Anything unknown stays idle. «waiting» is the chat's
  // own mark (a question of the wind's that is not answered yet).
  const moods = ['idle', 'thinking', 'searching', 'reading', 'writing', 'working', 'discussing', 'waiting', 'done', 'failed', 'news'];
  // The pose each mood springs to (winds.css .wf-pose, .wf-aim, .wf-lid and
  // .wf-smile): body offset, tilt and squash, where the eyes look, how far the
  // upper lids close (0..1 of the eye) and their slant (+ determined, − sad),
  // and how far the lower lids rise into happy arcs. The loops play on top.
  const poses = {
    idle:       { x:0, y:0, tilt:0, sx:1, sy:1, gx:0, gy:0, lid:.07, brow:0, smile:0 },
    thinking:   { x:0, y:-.5, tilt:-5, sx:1, sy:1, gx:2.4, gy:-3.2, lid:0, brow:0, smile:0 },
    searching:  { x:1.5, y:0, tilt:4, sx:1, sy:1, gx:0, gy:-.6, lid:.22, brow:8, smile:0 },
    reading:    { x:.5, y:1, tilt:3, sx:1, sy:1, gx:0, gy:2.2, lid:.32, brow:0, smile:0 },
    writing:    { x:.5, y:.5, tilt:2, sx:1, sy:1, gx:-.6, gy:2.6, lid:.26, brow:5, smile:.1 },
    working:    { x:0, y:.8, tilt:0, sx:1.04, sy:.96, gx:0, gy:.6, lid:.38, brow:14, smile:0 },
    discussing: { x:0, y:0, tilt:-3, sx:1, sy:1, gx:0, gy:0, lid:.08, brow:0, smile:.15 },
    waiting:    { x:-1.5, y:0, tilt:-7, sx:1, sy:1, gx:-1.8, gy:1.6, lid:0, brow:0, smile:.12 },
    done:       { x:0, y:0, tilt:0, sx:1, sy:1, gx:0, gy:-1, lid:0, brow:0, smile:.5 },
    failed:     { x:0, y:2.4, tilt:5, sx:1.05, sy:.92, gx:-.6, gy:2.6, lid:.42, brow:-16, smile:0 },
    news:       { x:.5, y:-.5, tilt:-4, sx:1, sy:1, gx:2, gy:-1.6, lid:0, brow:0, smile:.22 },
  };
  function pose(svg, mood) {
    const p = poses[mood], unit = { x:'px', y:'px', gx:'px', gy:'px', tilt:'deg', brow:'deg' };
    for (const [name, value] of Object.entries(p)) svg.style.setProperty('--wf-' + name, value + (unit[name] || ''));
  }
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
  // `heard` holds what the first poll found (it only shows on the faces) and what
  // was announced since; `onNews` is the workspace's toast or notification.
  const heard = new Set();
  let newsTimer = 0, onNews = null, newsPolled = false;
  function newsSeen() { try { return JSON.parse(root.localStorage?.getItem(newsKey) || '{}'); } catch (_) { return {}; } }
  function saveSeen(seen) { try { root.localStorage?.setItem(newsKey, JSON.stringify(seen)); } catch (_) {} }
  function refreshNews() {
    // A clay face (characters.js) changes through its player.
    for (const svg of doc.querySelectorAll?.('svg.winds-face[data-wind], canvas.winds-face[data-wind]') || []) {
      const id = svg.getAttribute('data-wind'), mood = svg.getAttribute('data-mood'), change = svg.tagName === 'CANVAS' ? root.gregalCharacters.mood : setMood;
      if (news.has(id) && mood === 'idle') change(svg, 'news');
      else if (!news.has(id) && mood === 'news') change(svg, 'idle');
    }
  }
  async function pollNews() {
    // A hidden tab keeps polling: that is when a system notification is worth it.
    if (typeof root.api !== 'function') return;
    const seen = newsSeen();
    // First run: only the last day counts, so an old backlog does not wake everyone.
    if (!seen.since) { seen.since = Math.floor(Date.now() / 1000) - 86400; saveSeen(seen); }
    try {
      const response = await root.api('/api/winds/novetats?des_de=' + seen.since);
      if (!response.ok) return;
      const fresh = [];
      for (const n of (await response.json()).novetats || []) {
        // Read elsewhere (opened on the phone, «Avui»): no bubble here either (B7).
        if (!n.wind || n.read || n.ts <= (seen[n.wind] || seen.since)) continue;
        news.set(n.wind, Math.max(n.ts, news.get(n.wind) || 0));
        const key = n.id || n.wind + ':' + n.ts;
        if (!heard.has(key)) { heard.add(key); fresh.push(n); }
      }
      refreshNews();
      if (newsPolled && fresh.length) onNews?.(fresh);
      newsPolled = true;
    } catch (_) {}
  }
  function watchNews(listener) {
    if (listener) onNews = listener;
    if (newsTimer) return;
    newsTimer = root.setInterval?.(pollNews, 60000) || 1;
    pollNews();
  }
  function seenNews(id) {
    if (!id) return;
    const seen = newsSeen();
    seen[id] = Math.max(news.get(id) || 0, Math.floor(Date.now() / 1000), seen[id] || 0);
    news.delete(id); saveSeen(seen); refreshNews();
    // Same read state on every device: the phone drops its notification too (B7).
    if (typeof root.api === 'function') {
      root.api('/api/winds/novetats/read', { method:'POST', headers:{ 'Content-Type':'application/json' }, body:JSON.stringify({ wind:id }) }).catch(() => {});
    }
  }
  const moodSince = new Map(), faceKeys = new WeakMap(), faceTones = new WeakMap(), faceBreath = new WeakMap();
  const lastPose = new Map(), poseKeys = new WeakMap();
  const clock = () => (root.performance?.now?.() ?? Date.now()) / 1000;
  // Cares que bufen (mockups/vents-avatars.html): while a Cares face works it
  // blows downwind. Lines in face units along +x from the rim, turned to the
  // opposite of the wind's direction; they reach 48 from the centre, so they
  // stay within the face box plus 8 (10 %), less than the gap before a name in
  // any list row. The short, bolder pair is for faces under 28 px (winds.css).
  const breathMoods = ['thinking', 'searching', 'reading', 'writing', 'working'];
  const gusts = [['M35 -7C39 -9 43 -5 47 -7.5', 3.2], ['M36 0C40 -2 44 2 48 0', 3.2], ['M35 7C39 9 43 5 47 7.5', 3.2]];
  const puffs = [['M36 -4C39 -5 42 -3 45 -4', 5], ['M36 4C39 5 42 3 45 4', 5]];
  function blow(svg, { deg, fill }) {
    const id = 'wf' + (++faceSerial), box = svgNode('svg', { class:'wf-breath', x:-8, y:-8, width:96, height:96, viewBox:'-8 -8 96 96' });
    box.append(shapes(svgNode('linearGradient', { id, x1:34, y1:0, x2:48, y2:0, gradientUnits:'userSpaceOnUse' }),
      [['stop', { offset:0, 'stop-color':mix(fill, '#FFFFFF', .15) }], ['stop', { offset:1, 'stop-color':fill, 'stop-opacity':0 }]]));
    const turn = group(box, '', { transform:`translate(40 40) rotate(${deg + 90})`, fill:'none', stroke:'url(#' + id + ')', 'stroke-linecap':'round' });
    for (const [cls, list] of [['wf-gust', gusts], ['wf-puff', puffs]]) shapes(group(turn, cls), list.map(([d, width]) => ['path', { d, 'stroke-width':width }]));
    svg.append(box);
  }
  function setMood(svg, mood, key = faceKeys.get(svg)) {
    mood = moods.includes(mood) ? mood : 'idle';
    const now = clock(), last = moodSince.get(key);
    if (mood !== 'idle' && last?.mood !== mood) moodSince.set(key, { mood, at:now });
    svg.setAttribute('data-mood', mood);
    // A face redrawn in a new mood (the chat corner replaces its face) starts
    // from the pose this wind had at that size and springs to the new one once
    // it is on screen; the same face changing in place transitions in CSS.
    const posed = poseKeys.get(svg), from = posed && lastPose.get(posed);
    if (posed) lastPose.set(posed, mood);
    if (from && from !== mood && !svg.isConnected && !root.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
      pose(svg, from);
      root.requestAnimationFrame(() => root.requestAnimationFrame(() => { if (svg.getAttribute('data-mood') === mood) pose(svg, mood); }));
    } else pose(svg, mood);
    if (maps.has(svg)) mapWake(maps.get(svg), mood);
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
    svg.querySelector('.wf-breath')?.remove();
    const breath = faceBreath.get(svg);
    if (breath && breathMoods.includes(mood)) blow(svg, breath);
    // In Cares the breath replaces the working cog; the other marks stay.
    if (badges[mood] && !(breath && mood === 'working')) {
      const tone = faceTones.get(svg), badge = shapes(group(svg, 'wf-badge'), badges[mood], tone), tool = badge.querySelector('.wf-tool, .wf-pop');
      if (tool) shapes(tool, tools[mood], tone);
    }
    return svg;
  }
  let faceSerial = 0;
  // Pick the same higher-contrast ink as Android's WindFace.
  const darkInk = '#202127';
  function inkOn(fill) {
    const background = luminance(fill) + .05;
    return background / (luminance(darkInk) + .05) >= 1.05 / background ? darkInk : '#FFFFFF';
  }
  function luminance(hex) {
    const linear = i => {
      const value = parseInt(hex.slice(i, i + 2), 16) / 255;
      return value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4;
    };
    return .2126 * linear(1) + .7152 * linear(3) + .0722 * linear(5);
  }
  // Silhouettes in face coordinates: a pebble, domed on top and settled on its
  // base (style 'o', and 's' with its curl), and Android's cloud (r 30). Its
  // parts share one user-space gradient, so they read as a single shape.
  const pebble = 'M40 9.5C65 9.5 74.5 24 74.5 43C74.5 62 62 71.5 40 71.5S5.5 62 5.5 43C5.5 24 15 9.5 40 9.5Z';
  const gust = 'M26.2 11.5C32.5 9 36.9 5.2 39.4 1.5C41.2-1.6 45.6-3.1 48.8-.9C52 1.6 51 6.1 47 6.4C45 6.5 44 4.9 45 3.4C42.8 4.6 43.2 8.4 46.9 9.9L53.8 11.6Z';
  const bodies = {
    o: [['path', { d:pebble }]],
    s: [['path', { d:pebble }], ['path', { d:gust }]],
    n: [['rect', { x:4.3, y:31, width:71.4, height:38.1, rx:17.4 }], ['circle', { cx:21.7, cy:38.5, r:20.1 }], ['circle', { cx:42.4, cy:34.3, r:25.8 }], ['circle', { cx:59.8, cy:41.2, r:18 }]],
  };
  const num = value => +value.toFixed(2);
  // Avatar packs: the account's theme draws every wind with one pack, or 'free'
  // lets each wind use its own `avatar` (empty is Cares). The server keeps both.
  const packs = ['cares', 'esperit', 'cap', 'rosa', 'mapa', 'minairo', 'pelut'];
  let theme = 'cares';
  function setAvatarTheme(value) { theme = value === 'free' || packs.includes(value) ? value : 'cares'; }
  function avatarPack(w) { return theme !== 'free' ? theme : packs.includes(w.avatar) ? w.avatar : 'cares'; }
  // Compass bearing (0 = N) a wind blows from: the eight Catalan winds by name,
  // case and accents aside; any other wind one of the eight from its id.
  // Android's WindFace mirrors both: h = h * 31 + each UTF-16 unit, unsigned 32
  // bit (the bits of Java's String.hashCode), then (h % 8) * 45.
  const rose = { tramuntana:0, gregal:45, llevant:90, xaloc:135, migjorn:180, garbi:225, ponent:270, mestral:315 };
  const plainName = w => String(w?.name || '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').trim().toLowerCase();
  // A wind not saved yet (the editor's preview) uses the id its name will get.
  const windId = w => String(w?.id || plainName(w).replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, ''));
  function idHash(w) {
    const id = windId(w);
    let h = 0;
    for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0;
    return h;
  }
  function direction(w) {
    const name = plainName(w);
    if (Object.hasOwn(rose, name)) return rose[name];
    return idHash(w) % 8 * 45;
  }
  // The volume of Cares for the round heads of the other packs, in layers over
  // the head's own gradient: a core shadow that turns the far side away from the
  // top-left light, reflected light along the bottom-right edge, a broad sheen
  // and a crisp specular highlight. Under 28 px the sheen goes and the highlight
  // grows a little, so the head stays one clean shape. The figure stands on a
  // soft shadow over a bounce of its own colour, which still reads on a dark
  // page. The gradients take the face's own id: faces on one page never share.
  const stops = list => list.map(([offset, tint, opacity]) => ['stop', { offset, 'stop-color':tint, 'stop-opacity':opacity }]);
  function relief(defs, id, fill, cx, cy, rx, ry = rx, small = false) {
    const core = mix(fill, '#1B1530', .6), glow = mix(fill, '#FFFFFF', .5), R = Math.max(rx, ry), lx = num(cx - .38 * rx), ly = num(cy - .48 * ry);
    const space = { gradientUnits:'userSpaceOnUse' }, url = suffix => 'url(#' + id + suffix + ')';
    defs.append(
      shapes(svgNode('radialGradient', { id:id + 'k', ...space, cx:lx, cy:ly, r:num(1.7 * R) }), stops([[.4, core, 0], [.78, core, small ? .5 : .62], [.93, core, .45], [1, core, .2]])),
      shapes(svgNode('radialGradient', { id:id + 'r', ...space, cx:num(cx - .22 * rx), cy:num(cy - .26 * ry), r:num(1.3 * R) }), stops([[.86, glow, 0], [1, glow, .55]])),
      shapes(svgNode('radialGradient', { id:id + 'w', ...space, cx:lx, cy:ly, r:num(.65 * R) }), stops([[0, '#FFFFFF', .38], [1, '#FFFFFF', 0]])),
      shapes(svgNode('radialGradient', { id:id + 's' }), stops([[0, '#FFFFFF', .95], [.5, '#FFFFFF', .8], [1, '#FFFFFF', 0]])),
      shapes(svgNode('radialGradient', { id:id + 'o' }), stops([[0, core, .55], [1, core, 0]])));
    const sx = num(cx - .36 * rx), sy = num(cy - .5 * ry), head = { cx, cy, rx, ry };
    return [['ellipse', { class:'wf-shade', ...head, fill:url('k') }], ['ellipse', { class:'wf-rim', ...head, fill:url('r') }],
      ...small ? [] : [['ellipse', { class:'wf-sheen', ...head, fill:url('w') }]],
      ['ellipse', { class:'wf-spec', cx:sx, cy:sy, rx:num((small ? .28 : .2) * rx), ry:num((small ? .18 : .12) * ry), transform:`rotate(-35 ${sx} ${sy})`, fill:url('s') }]];
  }
  function contact(defs, id, fill, cx, cy, rx) {
    const ground = mix(fill, '#0B0C14', .75);
    defs.append(
      shapes(svgNode('radialGradient', { id:id + 'b' }), stops([[0, fill, .4], [1, fill, 0]])),
      shapes(svgNode('radialGradient', { id:id + 'g' }), stops([[0, ground, .85], [.55, ground, .4], [1, ground, 0]])));
    return [['ellipse', { class:'wf-bounce', cx, cy, rx:num(1.3 * rx), ry:num(.32 * rx), fill:'url(#' + id + 'b)' }],
      ['ellipse', { class:'wf-ground', cx, cy, rx, ry:num(.2 * rx), fill:'url(#' + id + 'g)' }]];
  }
  // Esperit de vent (mockups/vents-avatars.html): a head with two tails that
  // point to where the wind comes from, its eyes looking where it goes. Drawn
  // in a 100 box scaled into the 80 one, so mood badges keep their corner. The
  // idle sway is left out when the system asks for less motion; work moves the
  // tails from winds.css either way.
  function esperit(svg, w, fill, ink, full, small) {
    const deg = direction(w), rad = deg * Math.PI / 180, ux = Math.sin(rad), uy = -Math.cos(rad), R = 28;
    const hx = num(50 - 12 * ux), hy = num(50 - 12 * uy), px = -1.6 * ux, py = -1.6 * uy, q = v => num(v * R);
    const id = 'wf' + (++faceSerial), defs = svgNode('defs', {}), tail = 'url(#' + id + 't)';
    defs.append(
      shapes(svgNode('radialGradient', { id:id + 'h', cx:num(hx - 9), cy:num(hy - 11), r:num(1.9 * R), gradientUnits:'userSpaceOnUse' }),
        [[0, mix(fill, '#FFFFFF', .5)], [.5, fill], [1, mix(fill, '#1B1530', .35)]].map(([offset, tint]) => ['stop', { offset, 'stop-color':tint }])),
      shapes(svgNode('linearGradient', { id:id + 't', x1:q(.4), y1:0, x2:q(1.9), y2:0, gradientUnits:'userSpaceOnUse' }),
        [[0, 1], [.55, .55], [1, 0]].map(([offset, opacity]) => ['stop', { offset, 'stop-color':fill, 'stop-opacity':opacity }])));
    svg.append(defs);
    svg.setAttribute('data-avatar', 'esperit');
    if (!root.matchMedia?.('(prefers-reduced-motion: reduce)').matches) svg.setAttribute('class', svg.getAttribute('class') + ' wf-idle');
    const box = group(svg, '', { transform:'scale(.8)' });
    if (full) shapes(box, contact(defs, id, fill, hx, num(hy + R + 5), num(.8 * R)));
    const drift = group(box, 'wf-drift', { style:`--wf-dx:${num(px)}px;--wf-dy:${num(py - 1.4)}px` });
    shapes(group(group(drift, '', { transform:`translate(${hx} ${hy}) rotate(${deg - 90})` }), 'wf-tails'), [
      ['path', { d:`M0 ${q(-1)}C${q(.9)} ${q(-1.06)} ${q(1.45)} ${q(-.72)} ${q(1.9)} ${q(-.58)}C${q(1.3)} ${q(-.42)} ${q(.85)} ${q(-.18)} ${q(.3)} ${q(-.02)}Z`, fill:tail }],
      ['path', { d:`M0 ${q(1)}C${q(.8)} ${q(1.03)} ${q(1.22)} ${q(.78)} ${q(1.58)} ${q(.64)}C${q(1.1)} ${q(.5)} ${q(.7)} ${q(.3)} ${q(.25)} ${q(.12)}Z`, fill:tail }],
      ['path', { d:`M${q(1.02)} ${q(.04)}C${q(1.3)} ${q(-.04)} ${q(1.5)} ${q(.1)} ${q(1.8)} ${q(.02)}`, fill:'none', stroke:tail, 'stroke-width':3.2, 'stroke-linecap':'round' }]]);
    shapes(drift, [
      ['circle', { cx:hx, cy:hy, r:R, fill:'url(#' + id + 'h)' }], ...relief(defs, id, fill, hx, hy, R, R, small),
      ...[-1, 1].flatMap(side => [
        ['ellipse', { cx:num(hx + 14 * side), cy:num(hy + 6), rx:4.6, ry:2.8, fill:'#E4577A', opacity:.32 }],
        // The eyes sit a little into the head: a soft shadow just above them.
        ...full ? [['ellipse', { cx:num(hx + 8 * side + px), cy:num(hy - 3.4 + py), rx:4.4, ry:4.8, fill:'url(#' + id + 'o)' }],
          ['ellipse', { cx:num(hx + 14 * side - 1), cy:num(hy + 5), rx:2.2, ry:1, fill:'#FFFFFF', opacity:.4 }]] : [],
        ['ellipse', { cx:num(hx + 8 * side + px), cy:num(hy - 2 + py), rx:2.9, ry:3.7, fill:ink }],
        ['circle', { cx:num(hx + 8 * side + 1 + px), cy:num(hy - 3.4 + py), r:1, fill:ink === '#FFFFFF' ? fill : '#FFFFFF' }]]),
      ['path', { d:`M${num(hx - 5)} ${num(hy + 6)}Q${hx} ${num(hy + 10.5)} ${num(hx + 5)} ${num(hy + 6)}`, fill:'none', stroke:ink, 'stroke-width':2.2, 'stroke-linecap':'round' }]]);
    return svg;
  }
  // Cap de vent (bufador() in the mockup): a head of the old maps with full
  // cheeks and closed eyes, blowing toward where the wind goes, so its head
  // sits on the side the wind comes from. Small faces keep the mouth and two
  // short thick lines of breath.
  function cap(svg, w, fill, ink, small, full) {
    const deg = direction(w), rad = deg * Math.PI / 180, ux = Math.sin(rad), uy = -Math.cos(rad), R = small ? 28 : 25;
    const hx = num(50 + 11 * ux), hy = num(50 + 11 * uy), mx = num(hx - 9 * ux), my = num(hy + 7 - 3 * uy), breath = mix(fill, '#FFFFFF', .25);
    const id = 'wf' + (++faceSerial), defs = svgNode('defs', {});
    defs.append(
      shapes(svgNode('radialGradient', { id:id + 'h', cx:num(hx - 9), cy:num(hy - 11), r:num(1.9 * R), gradientUnits:'userSpaceOnUse' }),
        [[0, mix(fill, '#FFFFFF', .5)], [.5, fill], [1, mix(fill, '#1B1530', .35)]].map(([offset, tint]) => ['stop', { offset, 'stop-color':tint }])),
      shapes(svgNode('linearGradient', { id:id + 't', x1:0, y1:0, x2:small ? 18 : 36, y2:0, gradientUnits:'userSpaceOnUse' }),
        [[0, 1], [1, 0]].map(([offset, opacity]) => ['stop', { offset, 'stop-color':breath, 'stop-opacity':opacity }])));
    svg.append(defs);
    svg.setAttribute('data-avatar', 'cap');
    if (!root.matchMedia?.('(prefers-reduced-motion: reduce)').matches) svg.setAttribute('class', svg.getAttribute('class') + ' wf-idle');
    const box = group(svg, '', { transform:'scale(.8)' });
    if (full) shapes(box, contact(defs, id, fill, hx, num(hy + R + 4), num(.8 * R)));
    shapes(box, [['ellipse', { cx:hx, cy:hy, rx:R + 1.5, ry:R, fill:'url(#' + id + 'h)' }], ...relief(defs, id, fill, hx, hy, R + 1.5, R, small), ...small ? [['circle', { cx:mx, cy:my, r:4, fill:ink }]] : [
      // Full cheeks: lit on top, deeper below.
      ...[-13, 13].flatMap(dx => [['circle', { cx:num(hx + dx), cy:num(hy + 5), r:6.5, fill:'#E4577A', opacity:.38 }],
        ['ellipse', { cx:num(hx + dx - 1.5), cy:num(hy + 2.6), rx:3, ry:1.6, fill:'#FFFFFF', opacity:.4 }]]),
      ['path', { d:`M${num(hx - 12)} ${num(hy - 3)}q4-4.5 8 0M${num(hx + 4)} ${num(hy - 3)}q4-4.5 8 0`, fill:'none', stroke:ink, 'stroke-width':2.3, 'stroke-linecap':'round' }],
      ['ellipse', { cx:mx, cy:my, rx:3.4, ry:3.9, fill:ink }]]]);
    const lines = small ? [['M0-5C6-7 12-4 18-6', 7], ['M0 6C6 8 12 5 18 7', 7]] : [['M0-7C10-11 18-5 30-9', 3.6], ['M0 0C12-3 22 3 36 0', 3.6], ['M0 7C10 11 18 5 30 9', 3.6]];
    shapes(group(box, 'wf-cap-breath', { transform:`translate(${num(hx - ux * (R + 3))} ${num(hy - uy * (R + 3))}) rotate(${(deg + 90) % 360})`, fill:'none', stroke:'url(#' + id + 't)', 'stroke-linecap':'round' }),
      lines.map(([d, width]) => ['path', { d, 'stroke-width':width, 'stroke-dasharray':small ? 'none' : '12 6' }]));
    return svg;
  }
  // Rosa de rellotge (rosa() in the mockup): the wind rose as a dial on a dark
  // tone of the wind's colour, its long point in the colour of the wind and
  // turned to where it comes from. Small faces draw only that point, wider; the
  // whole figure adds the minute ticks.
  const cream = '#F3EAD8';
  function rosa(svg, w, fill, small, full) {
    const id = 'wf' + (++faceSerial), defs = svgNode('defs', {}), dark = mix(fill, '#0F0C18', .72), faint = mix(cream, dark, .45);
    const point = (len, wd, left, right) => [['path', { d:`M0 0L${-wd} ${-wd}L0 ${-len}Z`, fill:left }], ['path', { d:`M0 0L${wd} ${-wd}L0 ${-len}Z`, fill:right }]];
    defs.append(shapes(svgNode('radialGradient', { id:id + 'd', cx:-10, cy:-16, r:70, gradientUnits:'userSpaceOnUse' }),
      [[0, mix(fill, '#1B1530', .3)], [1, dark]].map(([offset, tint]) => ['stop', { offset, 'stop-color':tint }])));
    svg.append(defs);
    svg.setAttribute('data-avatar', 'rosa');
    if (!root.matchMedia?.('(prefers-reduced-motion: reduce)').matches) svg.setAttribute('class', svg.getAttribute('class') + ' wf-idle');
    const dial = group(svg, '', { transform:'scale(.8) translate(50 50)' });
    if (full) shapes(dial, contact(defs, id, fill, 0, 52, 36));
    shapes(dial, [['circle', { r:50, fill:'url(#' + id + 'd)' }]]);
    if (!small) {
      shapes(dial, [['circle', { r:44, fill:'none', stroke:cream, 'stroke-opacity':.3, 'stroke-width':1 }]]);
      if (full) shapes(group(dial, '', { stroke:cream, 'stroke-opacity':.4, 'stroke-width':1 }), Array.from({ length:32 }, (_, i) => ['line', { x1:0, y1:-43.5, x2:0, y2:i % 4 ? -41.5 : -40, transform:`rotate(${i * 11.25})` }]));
      for (const a of [0, 90, 180, 270]) shapes(group(dial, '', { transform:`rotate(${a})`, opacity:.55 }), point(29, 5.5, cream, faint));
      for (const a of [45, 135, 225, 315]) shapes(group(dial, '', { transform:`rotate(${a})`, opacity:.32 }), point(20, 4.5, cream, faint));
    }
    shapes(group(group(dial, '', { transform:`rotate(${direction(w)})` }), 'wf-point'), point(small ? 40 : 43, small ? 12 : 8, mix(fill, '#FFFFFF', .55), fill));
    // The light sits on the glass over the dial, so it goes on top of the points;
    // a rim in the dial's own dark tone keeps the ticks readable at the edge, and
    // the whole figure adds a curved reflection across the glass.
    shapes(dial, [['circle', { r:small ? 7 : 4.5, fill:cream }], ['circle', { r:small ? 3 : 1.8, fill:dark }], ...relief(defs, id, dark, 0, 0, 50, 50, small)]);
    if (full) {
      defs.append(shapes(svgNode('linearGradient', { id:id + 'q', x1:-40, y1:-40, x2:5, y2:5, gradientUnits:'userSpaceOnUse' }), stops([[0, '#FFFFFF', .55], [1, '#FFFFFF', .04]])));
      shapes(dial, [['path', { class:'wf-glass', d:'M-44.5-10.3A45.7 45.7 0 0 1 10.3-44.5A90 90 0 0 0-44.5-10.3Z', fill:'url(#' + id + 'q)' }]]);
    }
    return svg;
  }
  // Mapa de vent (mockups/vents-avatars.html): a live weather map in a circle,
  // streaks blowing away from where the wind comes from. mapStep is pure, so
  // the same wind always gets the same picture; the canvas only paints its
  // lines. Positions are 0..1 of the canvas, angles compass radians.
  const workMoods = ['thinking', 'searching', 'reading', 'writing', 'working', 'discussing'];
  function mapRandom(r) {
    r = (r + 0x6D2B79F5) >>> 0;
    let z = Math.imul(r ^ r >>> 15, r | 1);
    z ^= z + Math.imul(z ^ z >>> 7, z | 61);
    return [((z ^ z >>> 14) >>> 0) / 4294967296, r];
  }
  function mapSpawn(r) {
    let x, y, life;
    [x, r] = mapRandom(r); [y, r] = mapRandom(r); [life, r] = mapRandom(r);
    return [{ x, y, age:0, life:40 + life * 60 }, r];
  }
  function mapStart(w) {
    let r = 0, p;
    for (const c of windId(w)) r = (r * 31 + c.charCodeAt(0)) >>> 0;
    const ps = [], seed = r % 1000 / 100;
    for (let i = 0; i < 90; i++) { [p, r] = mapSpawn(r); ps.push(p); }
    return { phi:(direction(w) + 180) * Math.PI / 180, seed, t:0, r, ps, lines:[] };
  }
  function mapStep(st, busy) {
    const sp = busy ? .013 : .0055, t = st.t + .016, lines = [];
    let r = st.r;
    const ps = st.ps.map(p => {
      const ang = st.phi + .6 * Math.sin(p.x * 5.5 + st.seed + t * .4) * Math.cos(p.y * 4.5 + t * .7);
      const x = p.x + Math.sin(ang) * sp, y = p.y - Math.cos(ang) * sp, age = p.age + 1;
      lines.push([p.x, p.y, x, y, Math.sin(Math.PI * p.age / p.life) * .5]);
      if (age <= p.life && x > -.05 && x < 1.05 && y > -.05 && y < 1.05) return { x, y, age, life:p.life };
      let next; [next, r] = mapSpawn(r); return next;
    });
    return { ...st, t, r, ps, lines };
  }
  function mapPaint(m, busy) {
    const ctx = m.ctx, S = m.canvas.width;
    m.st = mapStep(m.st, busy);
    ctx.globalCompositeOperation = 'source-over'; ctx.globalAlpha = busy ? .16 : .1; ctx.fillStyle = m.base; ctx.fillRect(0, 0, S, S);
    ctx.globalCompositeOperation = 'lighter'; ctx.lineCap = 'round'; ctx.strokeStyle = m.ink; ctx.lineWidth = Math.max(1.2, S / (S < 60 ? 28 : 70));
    for (const [x0, y0, x1, y1, alpha] of m.st.lines) { ctx.globalAlpha = alpha; ctx.beginPath(); ctx.moveTo(x0 * S, y0 * S); ctx.lineTo(x1 * S, y1 * S); ctx.stroke(); }
    ctx.globalAlpha = 1;
  }
  // One requestAnimationFrame for every map on screen; none while the tab is
  // hidden. Under reduced motion no map moves, not even while its wind works:
  // CSS cannot stop a canvas, so each keeps the still frame it was painted with.
  const maps = new WeakMap(), mapShown = new Set();
  let mapFrame = 0, mapSeen = null, mapReduce = null;
  function mapRun() {
    if (!mapFrame && !doc.hidden && !mapReduce?.matches && mapShown.size) mapFrame = root.requestAnimationFrame(mapTick);
  }
  function mapTick() {
    mapFrame = 0;
    for (const m of mapShown) mapPaint(m, m.busy);
    mapRun();
  }
  function mapWake(m, mood) {
    m.busy = workMoods.includes(mood);
    mapRun();
  }
  function mapWatch(svg) {
    if (!mapSeen) {
      doc.addEventListener('visibilitychange', mapRun);
      mapReduce = root.matchMedia?.('(prefers-reduced-motion: reduce)');
      mapReduce?.addEventListener('change', mapRun);
      mapSeen = new root.IntersectionObserver(entries => {
        for (const entry of entries) {
          const m = maps.get(entry.target);
          if (!entry.isIntersecting) {
            mapShown.delete(m);
            if (!entry.target.isConnected) mapSeen.unobserve(entry.target);
            continue;
          }
          mapShown.add(m);
          // ×2 for sharpness; a new size starts the same picture over.
          const S = Math.max(48, Math.round(entry.boundingClientRect.width * 2));
          if (m.size !== S) {
            m.size = m.canvas.width = m.canvas.height = S; m.st = m.start;
            m.ctx.fillStyle = m.base; m.ctx.fillRect(0, 0, S, S);
            for (let i = 0; i < 50; i++) m.st = mapStep(m.st, false);
            for (let i = 0; i < 20; i++) mapPaint(m, false);
          }
        }
        mapRun();
      });
    }
    mapSeen.observe(svg);
  }
  function mapa(svg, w, fill) {
    const canvas = doc.createElement('canvas'), holder = svgNode('foreignObject', { x:0, y:0, width:80, height:80 }), id = 'wf' + (++faceSerial);
    canvas.setAttribute('class', 'wf-map');
    holder.append(canvas);
    const defs = svgNode('defs', {});
    defs.append(shapes(svgNode('radialGradient', { id, cx:.32, cy:.26, r:.46 }), [['stop', { offset:0, 'stop-color':'#FFFFFF', 'stop-opacity':.22 }], ['stop', { offset:1, 'stop-color':'#FFFFFF', 'stop-opacity':0 }]]));
    svg.append(defs, holder);
    svg.setAttribute('data-avatar', 'mapa');
    shapes(svg, [['circle', { cx:40, cy:40, r:40, fill:'url(#' + id + ')' }], ['circle', { cx:40, cy:40, r:39.5, fill:'none', stroke:'#FFFFFF', 'stroke-opacity':.08 }]]);
    const start = mapStart(w);
    maps.set(svg, { canvas, ctx:canvas.getContext('2d'), start, st:start, base:mix(fill, '#0C0A13', .4), ink:mix(fill, '#FFFFFF', .55), size:0, busy:false });
    mapWatch(svg);
    return svg;
  }
  // Minairó (117): the tiny tireless helper of Pyrenean folklore, drawn our own
  // way. A round body in the wind's colour, modelled like Cares, under a long
  // soft white beard and a red barretina whose tube folds over to one side; the
  // 110 eyes peek out between the two, and tiny feet stick out below. Each wind
  // its own: the mouth style sets the beard length, the shape a round, plump or
  // slim body, the direction the side the tip falls to (downwind; north and
  // south by the seed), and the seed one thing from the legend: the boxwood
  // needle case they live in, the St John's wort they are born from, or a
  // pebble. Under 28 px only the silhouette is left: barretina, beard, eyes.
  // 120: the seed also sets white brows (straight, arched, bushy), the
  // moustache (lobes, walrus, handlebar) and, rarely, round glasses; small
  // faces keep the plain lobes.
  const red = '#C9342C', beardTop = '#FFFDF7', beardBottom = '#E2DACD', boxwood = '#D9B26A', wort = '#F4C431', leaf = '#6C9A3E', pebbleGrey = '#A8A39B';
  // Folk barretina colours, each under the black band; Android's WindFace
  // keeps the same list, letters and pick.
  const barretines = [['v', red], ['m', '#6E2F6B'], ['f', '#2E6B3F'], ['o', '#C8962E'], ['b', '#26406E'], ['g', '#7D1F33']];
  function hue(hex) {
    const [r, g, b] = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255), max = Math.max(r, g, b), d = max - Math.min(r, g, b);
    if (!d) return 0;
    const h = max === r ? (g - b) / d % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
    return (h * 60 + 360) % 360;
  }
  // The wind's own pick (the fourth letter of its style), or one from its id,
  // moving on to the next colour while the hue is within 30° of the body's.
  function barretinaOf(w, fill) {
    const own = barretines.find(([letter]) => letter === String(w.style || '')[3]);
    if (own) return own[1];
    const start = idHash(w) % barretines.length, gap = c => { const d = Math.abs(hue(c) - hue(fill)); return Math.min(d, 360 - d); };
    return barretines.map((_, i) => barretines[(start + i) % barretines.length][1]).find(c => gap(c) >= 30);
  }
  function minairo(svg, w, s, fill, ink, large, small, seed) {
    const full = !!large, id = 'wf' + (++faceSerial), defs = svgNode('defs', {}), url = suffix => 'url(#' + id + suffix + ')', space = { gradientUnits:'userSpaceOnUse' };
    const [rx, ry, cy] = { o:[24, 23.5, 49], n:[26.5, 22, 50.5], s:[22, 25, 48] }[s[2]], top = cy - ry, R = Math.max(rx, ry);
    const L = small ? 67 : { n:63.5, s:67.5, o:71, t:74.5 }[s[1]], deg = direction(w), fold = deg % 180 ? (deg < 180 ? -1 : 1) : seed % 2 ? 1 : -1;
    const deep = mix(fill, '#1B1530', .4), core = mix(fill, '#1B1530', .6), band = '#17141A', crease = mix(beardBottom, '#1B1530', .25), barretina = barretinaOf(w, fill);
    const mouthInk = mix(red, '#1B1530', .65), nose = mix(fill, '#E4577A', .5), mirror = { transform:'matrix(-1 0 0 1 80 0)' };
    // Moduli prime to the 2 and 3 above, so ids one letter apart still differ.
    const brows = small ? '' : ['straight', 'arched', 'bushy'][seed % 7 % 3], moustache = small ? 'lobes' : ['lobes', 'walrus', 'handlebar'][seed % 11 % 3];
    defs.append(
      shapes(svgNode('radialGradient', { id:id + 'v', ...space, cx:num(40 - .38 * rx), cy:num(cy - .5 * ry), r:num(1.9 * R) }), stops([[0, mix(fill, '#FFFFFF', .3), 1], [.45, fill, 1], [1, deep, 1]])),
      shapes(svgNode('radialGradient', { id:id + 'h', cx:.3, cy:.2, r:.95 }), stops([[0, mix(barretina, '#FFFFFF', .3), 1], [.5, barretina, 1], [1, mix(barretina, '#1B1530', .35), 1]])),
      shapes(svgNode('linearGradient', { id:id + 'f', x1:0, y1:0, x2:.25, y2:1 }), stops([[0, beardTop, 1], [.55, beardTop, 1], [1, beardBottom, 1]])),
      shapes(svgNode('radialGradient', { id:id + 'q', cx:.35, cy:.3, r:.7 }), stops([[0, mix(nose, '#FFFFFF', .45), 1], [1, nose, 1]])),
      shapes(svgNode('radialGradient', { id:id + 'b' }), stops([[0, '#E4577A', .45], [1, '#E4577A', 0]])),
      shapes(svgNode('radialGradient', { id:id + 'i', cy:.4 }), stops([[0, core, .4], [1, core, 0]])),
      shapes(svgNode('clipPath', { id:id + 'c' }), [['ellipse', { cx:40, cy, rx, ry }]]));
    svg.append(defs);
    svg.setAttribute('data-avatar', 'minairo');
    svg.setAttribute('data-barretina', barretina); // the clay minairó (characters.js) tints its barretina with it
    if (large === true) {
      const ground = mix(fill, '#0B0C14', .72);
      defs.append(shapes(svgNode('radialGradient', { id:id + 'g' }), stops([[0, ground, .5], [1, ground, 0]])));
      shapes(svg, [['ellipse', { class:'wf-ground', cx:40, cy:73, rx:22, ry:4.2, fill:url('g') }]]);
    }
    const head = group(group(group(svg, 'wf-pose'), 'wf-figure'), '', full ? { transform:'translate(4 3) scale(.9)' } : {});
    // Accessories are drawn on the right and mirrored to the side away from the tip.
    const away = fold > 0 ? mirror : {}, thing = small ? -1 : seed % 3;
    if (thing === 2) shapes(group(head, 'wf-pedra', away), [['ellipse', { cx:num(40 + rx + 5), cy:num(cy + ry - 2), rx:5.2, ry:3.8, fill:pebbleGrey }],
      ['ellipse', { cx:num(40 + rx + 3.6), cy:num(cy + ry - 3.6), rx:2.2, ry:1.1, fill:'#FFFFFF', opacity:.45 }]]);
    shapes(head, [-1, 1].map(side => ['ellipse', { class:'wf-foot wf-foot-' + (side < 0 ? 'l' : 'r'), cx:num(40 + side * .42 * rx), cy:num(cy + ry - .5), rx:5.6, ry:3.4, fill:deep }]));
    shapes(head, [['ellipse', { class:'wf-body', cx:40, cy, rx, ry, fill:url('v') }]]);
    shapes(group(head, '', { 'clip-path':url('c') }), [...relief(defs, id, fill, 40, cy, rx, ry, small).filter(([, a]) => a.class === 'wf-shade' || a.class === 'wf-rim'),
      // The barretina sits into the head and shades the brow under it.
      ['ellipse', { cx:40, cy:num(top + 11), rx:num(rx * .9), ry:3.6, fill:core, opacity:.28 }]]);
    shapes(head, [-1, 1].map(side => ['ellipse', { cx:num(40 + side * .66 * rx), cy:num(cy - 3.5), rx:4.6, ry:3.2, fill:url('b') }]));
    eyePair(head, defs, id, { s, fill, ink, volume:url('v'), inset:full && url('i'), small, xs:[num(40 - .34 * rx), num(40 + .34 * rx)], y:cy - 10, k:.84 });
    // The beard's right half as cubic segments from under the nose to the tip,
    // in (share of the body's half width, y); the left half goes back mirrored.
    const beard = group(head, 'wf-beard'), pt = (f, y) => num(40 + f * rx) + ' ' + num(y);
    const half = side => [[[.28, cy - .5], [.58, cy - 1], [.82, cy - 5.5]], [[.94, cy + 1], [.84, L - 12], [.58, L - 5.5]], [[.47, L - 2.5], [.34, L - 4], [.25, L - 2]], [[.16, L], [.07, L], [0, L + .8]]]
      .map(c => c.map(([f, y]) => pt(side * f, y)));
    const right = half(1), left = half(-1);
    shapes(beard, [['path', { d:`M${pt(0, cy - 2.5)}` + right.map(c => 'C' + c.join(' ')).join('') + [3, 2, 1, 0].map(i => `C${left[i][1]} ${left[i][0]} ${i ? left[i - 1][2] : pt(0, cy - 2.5)}`).join('') + 'Z', fill:url('f') }]]);
    if (!small) shapes(beard, [['path', { d:[-.4, 0, .4].map(f => `M${pt(f, cy + 7)}Q${pt(f * 1.15, (cy + L) / 2)} ${pt(f * .55, L - 6)}`).join(''), fill:'none', stroke:crease, 'stroke-width':1, 'stroke-linecap':'round', opacity:.6 }]]);
    const mouth = group(beard, 'wf-mouth');
    if (s[1] === 'o') shapes(mouth, [['ellipse', { cx:40, cy:cy + 5.4, rx:2.6, ry:2.4, fill:mouthInk }]]);
    else shapes(mouth, [['path', { d:s[1] === 'n' ? `M37.6 ${cy + 5.2}h4.8` : `M37.2 ${cy + 4.4}q2.8 3 5.6 0`, fill:'none', stroke:mouthInk, 'stroke-width':1.8, 'stroke-linecap':'round' }]]);
    shapes(group(beard, 'wf-worry', { display:'none' }), [['path', { d:`M36.8 ${cy + 6}q1.6-1.6 3.2 0t3.2 0`, fill:'none', stroke:mouthInk, 'stroke-width':1.8, 'stroke-linecap':'round' }]]);
    // The moustache from under the nose, over the mouth and clear of it: two
    // soft lobes, drooping walrus ends, or handlebar ends curled up.
    const lip = { lobes:[[[.2, -5.4], [.6, -4.6], [.66, .2]], [[.52, 3.2], [.2, 2.8], [0, .6]]],
      walrus:[[[.24, -5.6], [.72, -4.4], [.74, 3.8]], [[.5, 4.2], [.2, 2.4], [0, .6]]],
      handlebar:[[[.2, -5.2], [.55, -4.4], [.74, -1.4]], [[.84, -.2], [1, -1.8], [.94, -4.8]], [[.92, 1.4], [.4, 3.2], [0, .6]]] }[moustache];
    shapes(group(beard, 'wf-moustache wf-moustache-' + moustache), [-1, 1].map(side => ['path', { d:`M${pt(0, cy - 1.8)}` + lip.map(c => 'C' + c.map(([f, dy]) => pt(side * f, cy + dy)).join(' ')).join('') + 'Z', fill:beardTop, stroke:crease, 'stroke-width':small ? 0 : .8 }]));
    shapes(head, [['circle', { cx:40, cy:num(cy - 2.6), r:small ? 4.4 : 3.8, fill:url('q') }]]);
    // The barretina, drawn for the round body and fitted to this one: a low
    // crown, and the tube rising off it, folding over and hanging by the head.
    const cap = group(head, 'wf-barretina', { transform:`translate(40 ${num(top - 25.5)}) scale(${num(rx / 24)} 1) translate(-40 0)` });
    shapes(cap, [['path', { d:'M17 32C17.5 22 27.5 16 40 16S62.5 22 63 32Z', fill:url('h') }]]);
    shapes(group(group(cap, '', fold < 0 ? mirror : {}), 'wf-tip'), [
      ['path', { d:'M30 18C28.5 8.5 33 1.5 42.5 1C52.5 .5 60.5 5 64 12.5C67 18.5 68 27 67 34.5C66.6 37.6 62.8 37.8 62.5 34.5C62.2 29 60.5 23.5 57 20C54.5 17.5 51.5 16.5 49 16.5C43 16 36 16.5 30 18Z', fill:url('h') }],
      // The hanging part turns away from the light and lies on the crown.
      ['path', { d:'M64 12.5C67 18.5 68 27 67 34.5C66.6 37.6 62.8 37.8 62.5 34.5C62.2 29 60.5 23.5 57 20Z', fill:band, opacity:.32 }],
      ...small ? [] : [['path', { d:'M62.5 34.5C62.2 29 60.5 23.5 57 20C54.5 17.5 51.5 16.5 49 16.5', fill:'none', stroke:band, 'stroke-width':1.2, 'stroke-linecap':'round', opacity:.6 }]]]);
    shapes(cap, [['path', { class:'wf-band', d:'M15.5 37.5C19 32 29 30.5 40 30.5S61 32 64.5 37.5L63.2 29C59 24.5 50 22.5 40 22.5S21 24.5 16.8 29Z', fill:band }],
      ...small ? [] : [['path', { d:'M17.5 29.4C22 25.6 31 24 40 24S58 25.6 62.5 29.4', fill:'none', stroke:'#FFFFFF', 'stroke-width':.9, 'stroke-linecap':'round', opacity:.18 }]]]);
    // Glasses and white brows sit over the band's edge, so both read on it and on the body.
    const eyeAt = side => ({ x:40 + side * .34 * rx, y:cy - 10 }), at = (side, dx, dy) => { const e = eyeAt(side); return num(e.x + side * dx) + ' ' + num(e.y + dy); };
    if (!small && seed % 13 === 5) shapes(group(head, 'wf-glasses'), [
      ...[-1, 1].map(side => ['circle', { cx:num(eyeAt(side).x), cy:cy - 10, r:7.4, fill:'#FFFFFF', 'fill-opacity':.14, stroke:'#2B2530', 'stroke-width':1.4 }]),
      ['path', { d:`M${at(-1, -7.4, 0)}Q40 ${cy - 12.5} ${at(1, -7.4, 0)}`, fill:'none', stroke:'#2B2530', 'stroke-width':1.4 }]]);
    if (brows) shapes(group(head, 'wf-brows wf-brows-' + brows), [-1, 1].map(side => ['path', { d:{
      straight:`M${at(side, -5, -8.2)}L${at(side, 5, -7.4)}L${at(side, 5, -5.2)}L${at(side, -5, -6)}Z`,
      arched:`M${at(side, -5.6, -5.6)}Q${at(side, 0, -11.6)} ${at(side, 5.8, -5.8)}Q${at(side, 0, -8.6)} ${at(side, -5.6, -5.6)}Z`,
      bushy:`M${at(side, -6.4, -5)}C${at(side, -5, -10.8)} ${at(side, 4, -11.6)} ${at(side, 7.2, -7.6)}C${at(side, 6, -5.4)} ${at(side, 2, -6.4)} ${at(side, -1, -5.2)}C${at(side, -3, -4.6)} ${at(side, -5, -4.2)} ${at(side, -6.4, -5)}Z`,
    }[brows], fill:beardTop, stroke:crease, 'stroke-width':.8, 'stroke-linejoin':'round' }]));
    if (thing === 1) shapes(group(cap, 'wf-herba', away), [['path', { d:'M58 30C61 24 63 19 67 14.5', fill:'none', stroke:leaf, 'stroke-width':1.4, 'stroke-linecap':'round' }],
      ['ellipse', { cx:62.6, cy:22.5, rx:3, ry:1.3, transform:'rotate(-40 62.6 22.5)', fill:leaf }], ['ellipse', { cx:65.4, cy:20.2, rx:2.6, ry:1.1, transform:'rotate(25 65.4 20.2)', fill:leaf }],
      ...[0, 72, 144, 216, 288].map(a => ['ellipse', { cx:67, cy:11.9, rx:1.5, ry:2.4, transform:`rotate(${a} 67 14.5)`, fill:wort }]), ['circle', { cx:67, cy:14.5, r:1.3, fill:mix(wort, barretina, .4) }]]);
    // The needle case stands by its feet, as tall as half of it.
    if (thing === 0) shapes(group(group(head, 'wf-canut', away), '', { transform:`translate(${num(40 + rx + 4.5)} ${num(cy + ry - 9)}) rotate(8)` }), [
      ['rect', { x:-3.4, y:-9, width:6.8, height:18, rx:3, fill:boxwood }], ['rect', { x:-3.7, y:-10, width:7.4, height:4, rx:1.8, fill:mix(boxwood, '#1B1530', .35) }],
      ['path', { d:'M-1.5-4v10', stroke:'#FFFFFF', 'stroke-width':1.2, 'stroke-linecap':'round', opacity:.45 }], ['path', { d:'M-3.4 3h6.8', stroke:mix(boxwood, '#1B1530', .2), 'stroke-width':.8 }]]);
    return svg;
  }
  // Pelut (155): a round ball of short fur in the wind's colour, a smooth face
  // with big eyes, small feet. It shows while its clay sheets load (characters.js)
  // or if they cannot: swept tufts round the edge give it the fur's soft outline.
  function pelut(svg, s, fill, ink, large, small) {
    const full = !!large, id = 'wf' + (++faceSerial), defs = svgNode('defs', {}), url = suffix => 'url(#' + id + suffix + ')';
    const cx = 40, cy = 45, R = 23, tuft = small ? 2.6 : 3.4, n = small ? 20 : 36, deep = mix(fill, '#1B1530', .4);
    const at = (a, r) => num(cx + r * Math.cos(a)) + ' ' + num(cy + r * Math.sin(a)), step = 2 * Math.PI / n;
    let outline = 'M' + at(0, R);
    for (let i = 0; i < n; i++) {
      const a = i * step, out = R + tuft * (i % 3 === 1 ? 1.3 : 1);
      outline += `Q${at(a + .3 * step, R + .6 * out - .6 * R)} ${at(a + .6 * step, out)}Q${at(a + .85 * step, R + .3 * tuft)} ${at(a + step, R)}`;
    }
    outline += 'Z';
    defs.append(
      shapes(svgNode('radialGradient', { id:id + 'v', gradientUnits:'userSpaceOnUse', cx:num(cx - .38 * R), cy:num(cy - .5 * R), r:num(1.9 * R) }), stops([[0, mix(fill, '#FFFFFF', .3), 1], [.45, fill, 1], [1, deep, 1]])),
      shapes(svgNode('radialGradient', { id:id + 'b' }), stops([[0, '#E4577A', .45], [1, '#E4577A', 0]])),
      shapes(svgNode('clipPath', { id:id + 'c' }), [['path', { d:outline }]]));
    svg.append(defs);
    svg.setAttribute('data-avatar', 'pelut');
    if (large === true) {
      const ground = mix(fill, '#0B0C14', .72);
      defs.append(shapes(svgNode('radialGradient', { id:id + 'g' }), stops([[0, ground, .5], [1, ground, 0]])));
      shapes(svg, [['ellipse', { class:'wf-ground', cx:40, cy:73, rx:22, ry:4.2, fill:url('g') }]]);
    }
    const head = group(group(group(svg, 'wf-pose'), 'wf-figure'), '', full ? { transform:'translate(4 3) scale(.9)' } : {});
    shapes(head, [-1, 1].map(side => ['ellipse', { class:'wf-foot wf-foot-' + (side < 0 ? 'l' : 'r'), cx:cx + side * 10, cy:cy + R + 1, rx:5.4, ry:3.2, fill:deep }]));
    shapes(head, [['path', { class:'wf-body', d:outline, fill:url('v') }]]);
    shapes(group(head, '', { 'clip-path':url('c') }), relief(defs, id, fill, cx, cy, R + tuft, R + tuft, small).filter(([, a]) => a.class === 'wf-shade' || a.class === 'wf-rim'));
    // A few strands catch the light on top; the face is smoother and a little lighter.
    if (!small) shapes(head, [['path', { d:[-2.2, -1.9, -1.6, -1.3, -1].map(a => `M${at(a, R - 6)}L${at(a - .06, R - 1)}`).join(''), fill:'none', stroke:mix(fill, '#FFFFFF', .45), 'stroke-width':1, 'stroke-linecap':'round', opacity:.6 }]]);
    shapes(head, [['ellipse', { cx, cy:cy + 2, rx:15, ry:12.5, fill:mix(fill, '#FFFFFF', .18), opacity:.7 }]]);
    shapes(head, [-1, 1].map(side => ['ellipse', { cx:cx + side * 14, cy:cy + 6, rx:4.6, ry:3.2, fill:url('b') }]));
    eyePair(head, defs, id, { s, fill, ink, volume:url('v'), inset:false, small, xs:[cx - 7.5, cx + 7.5], y:cy - 3, k:.95 });
    const line = { fill:'none', stroke:ink, 'stroke-width':small ? 2.6 : 1.9, 'stroke-linecap':'round' };
    shapes(group(head, 'wf-mouth'), [['path', { d:`M36.4 ${cy + 8.4}q3.6 3.4 7.2 0`, ...line }]]);
    shapes(group(head, 'wf-worry', { display:'none' }), [['path', { d:`M36.4 ${cy + 10}q1.8-1.6 3.6 0t3.6 0`, ...line }]]);
    return svg;
  }
  // Big glossy eyes carry the mood (110). The eye style sets their shape (tall,
  // big, a star catchlight, or calm and low); every eye has an upper lid that
  // closes and slants and a lower lid that rises into a happy arc, both in the
  // body's own paint and cut to the eye, so the eye itself changes shape. A
  // small face keeps one bold catchlight and no glow. `k` scales them for a
  // smaller head; `inset` is the soft socket the whole figure draws.
  function eyePair(head, defs, id, { s, fill, ink, volume, inset, small, xs, y, k = 1 }) {
    const [ex, ey0, edy] = { p:[5.8, 8, 0], b:[7.2, 8.6, 0], e:[6.2, 7.8, 0], m:[6.8, 5.8, 1.6] }[s[0]], erx = num(k * ex * (small ? 1.12 : 1)), ery = num(k * ey0 * (small ? 1.08 : 1));
    const lash = mix(fill, '#1B1530', .45), shine = ink === darkInk ? '#FFFFFF' : fill, iris = 'url(#' + id + 'y)';
    defs.append(shapes(svgNode('radialGradient', { id:id + 'y', cy:.85, r:.6 }), stops([[0, mix(fill, '#FFFFFF', .2), .7], [1, fill, 0]])));
    const eyes = group(group(group(head, 'wf-aim'), 'wf-gaze'), 'wf-eyes'), ey = num(y + k * edy);
    xs.forEach((x, i) => {
      const eye = { cx:x, cy:ey, rx:erx, ry:ery }, h = 2 * ery + 2;
      defs.append(shapes(svgNode('clipPath', { id:id + 'e' + i }), [['ellipse', eye]]));
      if (inset) shapes(eyes, [['ellipse', { cx:x, cy:num(ey - .6), rx:num(erx + 1.2), ry:num(ery + 1.4), fill:inset }]]);
      const cut = group(eyes, '', { 'clip-path':'url(#' + id + 'e' + i + ')' });
      shapes(cut, [['ellipse', { ...eye, fill:ink }], ...small ? [] : [['ellipse', { cx:x, cy:num(ey + .45 * ery), rx:num(.9 * erx), ry:num(.6 * ery), fill:iris }]]]);
      const cx = num(x + .3 * erx), cy = num(ey - .38 * ery), r = num((small ? .42 : .34) * erx);
      shapes(cut, s[0] === 'e' && !small
        ? [['path', { d:`M${cx} ${num(cy - 2 * r)}Q${cx} ${cy} ${num(cx + 2 * r)} ${cy}Q${cx} ${cy} ${cx} ${num(cy + 2 * r)}Q${cx} ${cy} ${num(cx - 2 * r)} ${cy}Q${cx} ${cy} ${cx} ${num(cy - 2 * r)}Z`, fill:shine }]]
        : [['circle', { cx, cy, r, fill:shine }]]);
      if (!small) shapes(cut, [['circle', { cx:num(x - .32 * erx), cy:num(ey + .42 * ery), r:num(.15 * erx), fill:shine, opacity:.7 }]]);
      // The lid's edge is a soft crease in the body's deep tone, so it reads as a lid.
      shapes(group(cut, 'wf-lid wf-lid-' + (i ? 'r' : 'l')), [['rect', { x:num(x - 1.6 * erx), y:num(ey - ery - h), width:num(3.2 * erx), height:h, fill:volume }],
        ['path', { d:`M${num(x - 1.6 * erx)} ${num(ey - ery)}h${num(3.2 * erx)}`, stroke:lash, 'stroke-width':1.2 }]]);
      shapes(cut, [['ellipse', { class:'wf-smile', cx:x, cy:num(ey + 2.15 * ery + .5), rx:num(1.7 * erx), ry:num(1.15 * ery), fill:volume, stroke:lash, 'stroke-width':1.2 }]]);
    });
  }
  // `large` picks the level of detail: false is a clean face for lists and chat
  // rows; true is the whole figure on its own ground, with the tool of its
  // trade; 'figure' is that figure without ground, for scenes that draw theirs.
  // `size` is the pixel size when the caller draws it under 28 px, so the packs
  // that have one draw their simpler small variant.
  function face(w, large, mood, size) {
    w = w || {};
    const s = style(w.style), kitKey = w.kit || w.specialty, kit = kits[kitKey], fill = color(w.color);
    const full = !!large, worn = full ? kit : null, cloud = s[2] === 'n', curl = s[2] === 's', R = cloud ? 30 : 33;
    const ink = inkOn(fill);
    const light = mix(fill, '#FFFFFF', .3), deep = mix(fill, '#1B1530', .3), tone = mix(fill, '#1B1530', .8);
    const svg = doc.createElementNS(SVG, 'svg'), key = w.id || w.name || 'crew';
    svg.setAttribute('viewBox', '0 0 80 80'); svg.setAttribute('class', 'winds-face' + (large === true ? ' winds-face-large' : ''));
    svg.setAttribute('aria-hidden', 'true'); svg.setAttribute('focusable', 'false');
    if (w.id) svg.setAttribute('data-wind', w.id);
    if ((!mood || mood === 'idle') && news.has(w.id)) mood = 'news';
    let seed = 0; for (const c of String(key)) seed = (seed * 31 + c.charCodeAt(0)) % 997;
    svg.style.setProperty('--wf-phase', (-((clock() + seed / 83) % 12)).toFixed(2) + 's');
    // Each wind glances its own way when idle (winds.css wf-look): a side and a reach from its seed.
    svg.style.setProperty('--wf-look', [-3.5, -2.5, 3, 3.5, -3, 2.5][seed % 6] + 'px');
    faceKeys.set(svg, key); faceTones.set(svg, tone);
    const pack = avatarPack(w);
    if (pack === 'esperit') return setMood(esperit(svg, w, fill, ink, large === true, size < 28), mood, key);
    if (pack === 'cap') return setMood(cap(svg, w, fill, ink, size < 28, large === true), mood, key);
    if (pack === 'rosa') return setMood(rosa(svg, w, fill, size < 28, large === true), mood, key);
    if (pack === 'mapa') return setMood(mapa(svg, w, fill), mood, key);
    // A list row and the large figure of one wind each spring from their own last pose.
    poseKeys.set(svg, key + '|' + !!large);
    if (pack === 'minairo') return setMood(minairo(svg, w, s, fill, ink, large, size < 28, seed), mood, key);
    // The pelut is clay at every size: a large face plays its sheet, a smaller one
    // holds its still frame. A scene's figure keeps the drawing, whose mood it changes.
    if (pack === 'pelut') {
      const drawn = setMood(pelut(svg, s, fill, ink, large, size < 28), mood, key);
      return large === 'figure' || !root.gregalCharacters ? drawn : root.gregalCharacters.play(drawn, w, large !== true);
    }
    if (kit) svg.setAttribute('data-kit', kitKey);
    const small = size < 28;
    // Volume: soft light from the top left, the wind's own deep tone below, a rim
    // of bounced light at the far edge and a satin sheen. No outline, no glare.
    const id = 'wf' + (++faceSerial), defs = svgNode('defs', {});
    const paint = (suffix, attrs, stops) => {
      const gradient = svgNode('radialGradient', { id:id + suffix, ...attrs });
      shapes(gradient, stops.map(([offset, tint, opacity]) => ['stop', { offset, 'stop-color':tint, 'stop-opacity':opacity ?? 1 }]));
      defs.append(gradient);
      return 'url(#' + id + suffix + ')';
    };
    const space = { gradientUnits:'userSpaceOnUse' }, lx = num(40 - .38 * R), ly = num(40 - .5 * R);
    const volume = paint('v', { ...space, cx:lx, cy:ly, r:num(1.9 * R) }, [[0, light], [.45, fill], [1, deep]]);
    const glow = mix(fill, '#FFFFFF', .5), core = mix(fill, '#1B1530', .6);
    const rim = paint('r', { ...space, cx:num(40 - .22 * R), cy:num(40 - .26 * R), r:num((cloud ? 1.62 : 1.32) * R) }, [[.8, glow, 0], [1, glow, .55]]);
    const sheen = paint('s', { ...space, cx:lx, cy:ly, r:num(.7 * R) }, [[0, '#FFFFFF', .3], [1, '#FFFFFF', 0]]);
    // Clay, not glass: a soft broad highlight instead of a crisp glint.
    const spec = paint('p', {}, [[0, '#FFFFFF', .7], [.45, '#FFFFFF', .4], [1, '#FFFFFF', 0]]);
    // A second, darker falloff turns the far side away from the light and lets
    // the rim's reflected light show at the very edge; the base darkens where
    // the face meets its feet, and the eyes sit a little into the face.
    const shade = paint('d', { ...space, cx:lx, cy:ly, r:num(1.7 * R) }, [[.4, core, 0], [.78, core, .5], [.93, core, .38], [1, core, .18]]);
    const occlusion = paint('o', {}, [[0, core, .75], [1, deep, 0]]);
    const inset = paint('i', { cy:.4 }, [[0, core, .45], [1, core, 0]]);
    const blush = paint('b', {}, [[0, '#E4577A', .5], [1, '#E4577A', 0]]);
    defs.append(shapes(svgNode('clipPath', { id:id + 'c' }), bodies[s[2]]));
    svg.append(defs);
    if (large === true) {
      const ground = mix(fill, '#0B0C14', .72);
      shapes(svg, [['ellipse', { class:'wf-ground', cx:40, cy:72.4, rx:23, ry:4.4, fill:paint('g', {}, [[0, ground, .5], [1, ground, 0]]) }]]);
    }
    // The mood's pose wraps the figure; its loops play inside it.
    const figure = group(group(svg, 'wf-pose'), 'wf-figure');
    // The whole figure is drawn a little smaller: room for hats, tools and marks.
    // A small face only makes room for its own curl.
    const head = group(figure, '', full ? { transform:'translate(4 3) scale(.9)' } : curl ? { transform:'translate(2 3.6) scale(.95)' } : {});
    const footY = cloud ? 69.8 : 69.4;
    shapes(head, [['ellipse', { class:'wf-foot wf-foot-l', cx:25.3, cy:footY, rx:7.1, ry:3.7, fill:deep }], ['ellipse', { class:'wf-foot wf-foot-r', cx:54.7, cy:footY, rx:7.1, ry:3.7, fill:deep }]]);
    // The curl of a gust on top: it changes the outline, never the face.
    if (curl) shapes(head, [['path', { class:'wf-curl', d:gust, fill:volume }]]);
    shapes(group(head, '', { fill:volume }), bodies[cloud ? 'n' : 'o']);
    shapes(group(head, '', { 'clip-path':'url(#' + id + 'c)' }), [
      ['rect', { class:'wf-shade', x:-4, y:-4, width:88, height:88, fill:shade }],
      ['ellipse', { class:'wf-ao', cx:40, cy:num(footY + 1), rx:num(.7 * R), ry:num(.24 * R), fill:occlusion }],
      ['rect', { x:-4, y:-4, width:88, height:88, fill:rim }],
      ['circle', { cx:lx, cy:ly, r:num(.65 * R), fill:sheen }]]);
    const sx = num(40 - .36 * R), sy = num(40 - .5 * R);
    shapes(head, [['ellipse', { class:'wf-spec', cx:sx, cy:sy, rx:num(.27 * R), ry:num(.16 * R), transform:`rotate(-35 ${sx} ${sy})`, fill:spec }]]);
    shapes(head, [
      ['ellipse', { cx:20.6, cy:48.8, rx:7, ry:4.6, fill:blush }],
      ['ellipse', { cx:59.4, cy:48.8, rx:7, ry:4.6, fill:blush }]]);
    if (full) shapes(head, [19.1, 57.9].map(cx => ['ellipse', { cx, cy:46.8, rx:2.6, ry:1.2, fill:'#FFFFFF', opacity:.35 }]));
    eyePair(head, defs, id, { s, fill, ink, volume, inset:full && inset, small, xs:[30.4, 49.6], y:38.4 });
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
    faceBreath.set(svg, { deg:direction(w), fill });
    return setMood(svg, mood, key);
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
  // Packs by name; the first ones are the packs a wind can pick in Free mode.
  const themeChoices = () => [['cares', tr('winds.avatarCares', 'Cares')], ['esperit', tr('winds.avatarEsperit', 'Esperit de vent')], ['cap', tr('winds.avatarCap', 'Cap de vent')], ['rosa', tr('winds.avatarRosa', 'Rosa de rellotge')], ['mapa', tr('winds.avatarMapa', 'Mapa de vent')], ['minairo', tr('winds.avatarMinairo', 'Minairons')], ['pelut', tr('winds.avatarPelut', 'Pelut')], ['free', tr('winds.avatarFree', 'Lliure')]];
  // The editor offers a pack only in Free mode; with a theme it names the pack
  // that draws every wind, and saving leaves the wind's own choice untouched.
  function avatarField(parent, w) {
    if (theme !== 'free') {
      parent.append(node('p', 'winds-muted', tr('winds.avatarThemeInUse', 'Tots els vents es dibuixen amb «{pack}». Es canvia a Preferències › Avatars.', {pack:themeChoices().find(([value]) => value === theme)[1]})));
      return null;
    }
    // Radios that show each pack drawn as this wind; `value` and `change` work
    // like the <select> they replace, so the editor's preview follows them.
    const wrap = node('div', 'winds-field'), group = node('div', 'winds-packs'), title = node('span', '', tr('winds.avatar', 'Avatar'));
    title.setAttribute('id', 'windAvatarLabel'); group.setAttribute('id', 'windAvatar');
    group.setAttribute('role', 'radiogroup'); group.setAttribute('aria-labelledby', 'windAvatarLabel');
    const radios = themeChoices().filter(([value]) => packs.includes(value)).map(([value, name]) => {
      const radio = node('button', 'winds-pack'); radio.setAttribute('type', 'button'); radio.setAttribute('role', 'radio'); radio.setAttribute('data-v', value);
      radio.append(packFace(value, w), node('span', '', name)); radio.onclick = () => { if (value !== group.value) pick(value); };
      return radio;
    });
    const pick = (value, focus) => {
      group.value = value;
      for (const radio of radios) { const on = radio.getAttribute('data-v') === value; radio.setAttribute('aria-checked', String(on)); radio.setAttribute('tabindex', on ? '0' : '-1'); if (on && focus) radio.focus(); }
      group.dispatchEvent(new root.Event('change'));
    };
    group.addEventListener('keydown', e => {
      const step = {ArrowRight:1, ArrowDown:1, ArrowLeft:-1, ArrowUp:-1}[e.key];
      if (!step) return;
      e.preventDefault();
      pick(packs[(packs.indexOf(group.value) + step + packs.length) % packs.length], true);
    });
    group.append(...radios); wrap.append(title, group); parent.append(wrap);
    pick(packs.includes(w?.avatar) ? w.avatar : 'cares');
    return group;
  }
  // The Vents workspace editors' barretina picker; empty is the automatic colour.
  function select(parent, id, title, values, value) {
    const wrap = node('label', 'winds-field'); wrap.setAttribute('for', id);
    wrap.append(node('span', '', title));
    const input = node('select'); input.setAttribute('id', id); input.setAttribute('name', id);
    for (const [key, label] of values) { const option = node('option', '', label); option.setAttribute('value', key); input.append(option); }
    input.value = value || ''; wrap.append(input); parent.append(wrap); return input;
  }
  function barretinaField(parent, w) {
    return select(parent, 'windBarretina', tr('winds.barretina', 'Barretina'), [['', tr('winds.barretinaAuto', 'Automàtica')], ['v', tr('winds.barretinaV', 'Vermella')], ['m', tr('winds.barretinaM', 'Morada')], ['f', tr('winds.barretinaF', 'Verd fosc')], ['o', tr('winds.barretinaO', 'Mostassa')], ['b', tr('winds.barretinaB', 'Blau marí')], ['g', tr('winds.barretinaG', 'Granat')]], barretines.some(([letter]) => letter === String(w.style || '')[3]) ? w.style[3] : '');
  }
  // A sample of one pack drawn as `w`, whatever pack the account's theme uses.
  // The clay packs show their 3D figure (characters.js), still and without a trade kit.
  function packFace(pack, w) {
    const was = theme; theme = 'free';
    try {
      const sample = {...w, id:undefined, avatar:pack, kit:undefined, specialty:undefined}, drawn = face(sample, false, 'idle', 64);
      return ['cares', 'minairo'].includes(pack) && root.gregalCharacters ? root.gregalCharacters.play(drawn, sample, true) : drawn;
    } finally { theme = was; }
  }
  // «Avatars» in Preferences, saved per account so every device draws the same.
  async function avatarPrefs(box) {
    const label = node('div', 'pf-lab'), choices = node('div', 'pf-seg'), feedback = node('span', 'winds-status');
    label.append(node('b', '', tr('winds.avatars', 'Avatars')), node('span', '', tr('winds.avatarsHelp', 'Un estil per a tots els vents, o Lliure perquè cada vent triï el seu.')), feedback);
    choices.setAttribute('role', 'group'); attribute(choices, 'aria-label', tr('winds.avatars', 'Avatars'));
    feedback.setAttribute('role', 'status');
    box.replaceChildren(label, choices); box.hidden = false;
    try { setAvatarTheme((await call('/api/winds')).avatarTheme); }
    catch (_) { paint(feedback, tr('winds.avatarsNotLoaded', 'No s’han pogut llegir els avatars. Torna a obrir Preferències.')); return; }
    const mark = () => { for (const b of choices.children) { const on = b.getAttribute('data-v') === theme; b.classList.toggle('on', on); b.setAttribute('aria-pressed', String(on)); } };
    for (const [value, name] of themeChoices()) {
      const choice = node('button', 'winds-pack'); choice.setAttribute('type', 'button'); choice.setAttribute('data-v', value);
      // Free mode lets every wind pick its own: two winds with different packs.
      const samples = value === 'free' ? [packFace('esperit', presetDefaults[1]), packFace('rosa', presetDefaults[3])] : [packFace(value, presetDefaults[2])];
      const faces = node('span', 'winds-pack-faces'); faces.append(...samples); choice.append(faces, node('span', '', name));
      choice.onclick = async () => {
        try {
          await call('/api/winds/avatar-theme', 'POST', {theme:value});
          setAvatarTheme(value); mark(); paint(feedback, '');
          if (doc.body?.classList.contains('shell-winds')) root.loadWindsPage?.();
        } catch (_) { paint(feedback, tr('winds.avatarsNotSaved', 'No s’han pogut desar els avatars. Torna-ho a provar.')); }
      };
      choices.append(choice);
    }
    mark();
  }
  // The person's avatar in the sidebar foot (user-avatar.js) wears these too.
  root.gregalWinds={palette:presetDefaults.map(p => ({name:p.name, color:p.color})), inkOn, face, mood:setMood, moodOf, watchNews, hasNews:id => news.has(id), seen:seenNews, direction, mapStart, mapStep, setAvatarTheme, avatarTheme:() => theme, avatarField, avatarPrefs, avatarPack, barretinaField};
})(window);
