// Canvas office scene: the coordinator, the role lanes it hands work to
// (researcher, builder, tester, reviewer) and any real delegated child.
// Team pipeline runs keep the older desk-per-agent layout.
import './workshop-roles.js';
const WIDTH = 960;
const HEIGHT = 400;
const ASSET = '/app/assets/pixel-office/';
const SPRITES = ['coordinator.png', 'researcher.png', 'builder.png', 'reviewer.png'];
const PALETTE = {
  wall: '#e8deca', wallLight: '#f3ead8', plaster: '#d9c9ad', floor: '#bd7658', floorDeep: '#a9614c',
  grout: '#945842', wood: '#936447', woodLight: '#c39162', woodTop: '#d1a878', mint: '#73b69f', mintDark: '#397f72',
  sea: '#558f90', seaLight: '#a7d2ca', ink: '#29463d', paper: '#fff9ec', muted: '#6d7162', glass: '#d8ece3',
  active: '#df9b43', done: '#4b9772', failed: '#b84d3b', canceled: '#847f75',
};
const ACTIVE = new Set(['thinking', 'tools', 'reviewing', 'compacting', 'working']);
const DEFAULT_LABELS = { thinking: 'THINKING', tools: 'USING A TOOL', reviewing: 'REVIEWING', compacting: 'SUMMARIZING', working: 'WORKING', completed: 'DONE', failed: 'FAILED', cancelled: 'CANCELLED', waiting: 'WAITING', idle: 'IDLE' };
const safe = value => typeof value === 'string' ? value : '';
const finite = value => Number.isFinite(value) ? value : null;

function normalize(session, opts = {}) {
  if (!session || typeof session !== 'object') return null;
  const activity = session.activity && typeof session.activity === 'object' ? session.activity : {};
  const agents = Array.isArray(activity.agents) ? activity.agents.filter(a => a && typeof a.id === 'string' && a.id).map(a => ({
    id: a.id, task: safe(a.task), agent: safe(a.agent), status: safe(a.status) || 'idle', model: safe(a.model), tool: safe(a.tool),
    message: safe(a.message), step: finite(a.step), max_steps: finite(a.max_steps),
  })) : [];
  // Waiting (needs a human) wins over the engine state, like the DOM bays:
  // the same session must not read THINKING in paint and "Esperant" in text.
  const waiting = opts.waiting === true;
  return {
    id: safe(session.id), title: safe(session.title), project: safe(session.project),
    state: waiting ? 'waiting' : (safe(activity.state) || (session.busy === true ? 'thinking' : 'idle')), task: safe(activity.task),
    model: safe(activity.model), tool: safe(activity.tool), message: safe(activity.message),
    step: finite(activity.step), limit: finite(activity.limit), agents,
    labels: opts.labels && typeof opts.labels === 'object' ? opts.labels : DEFAULT_LABELS,
    // Role room: everything but team pipelines (their agents are nodes).
    kind: safe(session.kind) || safe(activity.kind),
    roles: rolesApi() ? rolesApi().normalize({ ...activity, state: waiting ? 'waiting' : (safe(activity.state) || (session.busy === true ? 'thinking' : 'idle')) }) : null,
    T: typeof opts.T === 'function' ? opts.T : null,
  };
}

const rolesApi = () => (typeof window !== 'undefined' && window.gregalWorkshopRoles) || (typeof globalThis !== 'undefined' && globalThis.gregalWorkshopRoles) || null;

// Role desks along the front of the room; real delegated children sit in
// a smaller back row. Pure so the node tests can check the layout.
const ROLE_ORDER = ['coordinator', 'researcher', 'builder', 'tester', 'reviewer'];
const ROLE_SPRITE = { coordinator: 0, researcher: 1, builder: 2, tester: 2, reviewer: 3 };
const ROLE_FILTER = { tester: 'hue-rotate(155deg) saturate(1.25)' };
const ROLE_COLOR = { coordinator: '#c98a3c', researcher: '#4f8fa0', builder: '#b5674c', tester: '#5f9a5c', reviewer: '#8a6ab0' };
export function roomLayout(extraCount) {
  const front = 342;
  const xs = [104, 290, 480, 670, 856];
  const roles = ROLE_ORDER.map((role, i) => ({ role, x: xs[i], y: front, scale: role === 'coordinator' ? 1.12 : 1 }));
  const shown = Math.min(Math.max(0, extraCount | 0), 6);
  const extras = [];
  for (let i = 0; i < shown; i++) {
    const span = 560, left = 200 + (shown === 1 ? span / 2 : (span / (shown - 1)) * i);
    extras.push({ x: Math.round(shown === 1 ? 480 : left), y: 236, scale: .72 });
  }
  return { roles, extras, hidden: Math.max(0, (extraCount | 0) - shown) };
}

function rounded(ctx, x, y, w, h, radius, fill, stroke) {
  const r = Math.min(radius, w / 2, h / 2);
  ctx.beginPath(); ctx.moveTo(x + r, y); ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r); ctx.arcTo(x, y + h, x, y, r); ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath(); if (fill) { ctx.fillStyle = fill; ctx.fill(); } if (stroke) { ctx.strokeStyle = stroke; ctx.stroke(); }
}

function fitText(ctx, text, maxWidth) {
  let value = safe(text).replace(/[\r\n\t]+/g, ' ').trim();
  if (!value) return '';
  if (ctx.measureText(value).width <= maxWidth) return value;
  while (value.length > 1 && ctx.measureText(value + '…').width > maxWidth) value = value.slice(0, -1);
  return value + '…';
}

function drawEnvelope(ctx, x, y, scale = 1) {
  ctx.save(); ctx.translate(x, y); ctx.scale(scale, scale);
  ctx.shadowColor = '#f6d178aa'; ctx.shadowBlur = 12;
  rounded(ctx, -9, -6, 18, 13, 2, '#fff2c8', '#8e6848');
  ctx.shadowBlur = 0; ctx.fillStyle = '#dbad64'; ctx.beginPath(); ctx.moveTo(-8, -4); ctx.lineTo(0, 2); ctx.lineTo(8, -4); ctx.strokeStyle = '#ae8050'; ctx.lineWidth = 1; ctx.stroke();
  ctx.restore();
}

export function createWorkshopScene(canvas, onSelectAgent) {
  if (!canvas || typeof canvas.getContext !== 'function') return { update() {}, show() {}, hide() {}, select() {}, highlight() {}, destroy() {} };
  const ctx = canvas.getContext('2d');
  if (!ctx) return { update() {}, show() {}, hide() {}, select() {}, highlight() {}, destroy() {} };

  const images = SPRITES.map(file => {
    const img = new Image(); img.src = ASSET + file; img.onload = () => paint(performance.now()); return img;
  });
  const motion = typeof matchMedia === 'function' ? matchMedia('(prefers-reduced-motion: reduce)') : { matches: false };
  let session = null;
  let agentVisuals = new Map();
  let selectedId = '';
  let shown = false;
  let raf = 0;
  let lastFrame = 0;
  let destroyed = false;
  let resizeObserver = null;

  const now = () => typeof performance !== 'undefined' ? performance.now() : Date.now();
  const stopFrame = () => { if (raf) cancelAnimationFrame(raf); raf = 0; };
  const childStatus = a => ['working', 'completed', 'failed', 'cancelled'].includes(a.status) ? a.status : 'idle';
  const hasMotion = at => {
    if (!shown || document.hidden || motion.matches || !session) return false;
    if (roleMode() && rolesApi()) return roomHasMotion(at);
    if (ACTIVE.has(session.state)) return true;
    for (const [id, v] of agentVisuals) {
      const agent = session.agents.find(a => a.id === id);
      if (!agent) continue;
      const state = childStatus(agent);
      if (state === 'working') return true;
      if (v.walkStart != null && at - v.walkStart < 2200) return true;
      if (v.deliveryStart != null && !v.delivered && at - v.deliveryStart < 1800) return true;
    }
    return false;
  };

  function canvasSize() {
    const box = canvas.getBoundingClientRect();
    if (!box.width) return null;
    const dpr = Math.min(typeof devicePixelRatio === 'number' ? devicePixelRatio : 1, 2);
    const w = Math.max(1, Math.round(box.width * dpr));
    const h = Math.max(1, Math.round(box.height * dpr));
    if (canvas.width !== w) canvas.width = w;
    if (canvas.height !== h) canvas.height = h;
    ctx.setTransform(w / WIDTH, 0, 0, h / HEIGHT, 0, 0);
    ctx.imageSmoothingEnabled = false;
    return box;
  }

  function drawWindow(x, y, w, h) {
    ctx.save();
    rounded(ctx, x, y, w, h, 24, PALETTE.wood, '#704d37');
    ctx.save();
    ctx.beginPath(); ctx.moveTo(x + 5, y + h); ctx.lineTo(x + 5, y + w / 2); ctx.quadraticCurveTo(x + 5, y + 5, x + w / 2, y + 5); ctx.quadraticCurveTo(x + w - 5, y + 5, x + w - 5, y + w / 2); ctx.lineTo(x + w - 5, y + h); ctx.closePath(); ctx.clip();
    const sky = ctx.createLinearGradient(0, y, 0, y + h); sky.addColorStop(0, '#c5e0d8'); sky.addColorStop(.55, '#a9d4cb'); sky.addColorStop(.56, '#70aaa2'); sky.addColorStop(1, '#4f8885');
    ctx.fillStyle = sky; ctx.fillRect(x + 5, y + 5, w - 10, h - 10);
    ctx.fillStyle = '#f2d183'; ctx.beginPath(); ctx.arc(x + w * .73, y + h * .31, 8, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = '#809d7f'; ctx.beginPath(); ctx.moveTo(x + 5, y + h * .72); ctx.quadraticCurveTo(x + w * .25, y + h * .52, x + w * .49, y + h * .7); ctx.quadraticCurveTo(x + w * .77, y + h * .52, x + w - 5, y + h * .68); ctx.lineTo(x + w - 5, y + h); ctx.lineTo(x + 5, y + h); ctx.fill();
    ctx.strokeStyle = '#d9eee2'; ctx.lineWidth = 3; ctx.beginPath(); ctx.moveTo(x + w * .5, y + 7); ctx.lineTo(x + w * .5, y + h); ctx.moveTo(x + 6, y + h * .63); ctx.lineTo(x + w - 6, y + h * .63); ctx.stroke();
    ctx.restore(); ctx.fillStyle = PALETTE.woodLight; ctx.fillRect(x - 4, y + h - 3, w + 8, 7); ctx.restore();
  }

  function drawPlant(x, y, scale = 1) {
    ctx.save(); ctx.translate(x, y); ctx.scale(scale, scale);
    rounded(ctx, -14, -8, 28, 22, 4, '#b96f50', '#84523e'); ctx.fillStyle = '#d99767'; ctx.fillRect(-12, -7, 24, 3);
    ctx.strokeStyle = '#547d5e'; ctx.lineWidth = 4; ctx.lineCap = 'round';
    [[-10,-42,-16,-62],[-5,-42,-3,-70],[0,-42,7,-65],[5,-42,18,-59],[10,-41,20,-49]].forEach(([x1,y1,x2,y2]) => { ctx.beginPath(); ctx.moveTo(x1,y1); ctx.quadraticCurveTo(x1-2,y1-12,x2,y2); ctx.stroke(); });
    ctx.fillStyle = '#72a276'; [[-16,-62],[-3,-70],[7,-65],[18,-59],[20,-49],[-10,-48]].forEach(([lx,ly]) => { ctx.beginPath(); ctx.ellipse(lx,ly,6,11,-.55,0,Math.PI*2); ctx.fill(); });
    ctx.restore();
  }

  function drawClock() {
    ctx.save(); ctx.shadowColor = '#493d2b33'; ctx.shadowBlur = 5;
    ctx.fillStyle = '#f3e7ce'; ctx.strokeStyle = PALETTE.wood; ctx.lineWidth = 4; ctx.beginPath(); ctx.arc(864, 49, 25, 0, Math.PI * 2); ctx.fill(); ctx.stroke(); ctx.shadowBlur = 0;
    ctx.strokeStyle = '#73634c'; ctx.lineWidth = 2; ctx.beginPath(); ctx.moveTo(864, 49); ctx.lineTo(864, 33); ctx.moveTo(864, 49); ctx.lineTo(875, 54); ctx.stroke();
    ctx.fillStyle = PALETTE.wood; ctx.beginPath(); ctx.arc(864, 49, 2, 0, Math.PI * 2); ctx.fill(); ctx.restore();
  }

  function drawBackground() {
    ctx.clearRect(0, 0, WIDTH, HEIGHT);
    ctx.fillStyle = PALETTE.wall; ctx.fillRect(0, 0, WIDTH, 148);
    ctx.fillStyle = PALETTE.wallLight; ctx.fillRect(0, 0, WIDTH, 7);
    ctx.fillStyle = '#fff7e5';
    for (let y = 15; y < 135; y += 29) for (let x = (y % 3) * 13 + 11; x < WIDTH; x += 57) ctx.fillRect(x, y, 2, 2);
    drawWindow(359, 14, 146, 116); drawClock();
    ctx.fillStyle = PALETTE.floorDeep; ctx.fillRect(0, 145, WIDTH, 7);
    ctx.fillStyle = PALETTE.floor; ctx.fillRect(0, 152, WIDTH, HEIGHT - 152);
    ctx.fillStyle = '#d99570'; ctx.fillRect(0, 152, WIDTH, 5);
    ctx.strokeStyle = '#985b48'; ctx.globalAlpha = .34; ctx.lineWidth = 1;
    for (let y = 177; y < HEIGHT; y += 29) { ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(WIDTH, y); ctx.stroke(); }
    for (let x = -350; x < 1300; x += 128) { ctx.beginPath(); ctx.moveTo(480 + (x - 480) * .22, 153); ctx.lineTo(x, HEIGHT); ctx.stroke(); }
    ctx.globalAlpha = 1;
    drawPlant(28, 267, .92); drawPlant(927, 263, .88);
  }

  // A painted walkway links the coordinator to the delegate desks; with no
  // delegates there is nothing to link and it only added noise.
  function drawWalkway() {
    ctx.strokeStyle = '#e2bd91'; ctx.globalAlpha = .43; ctx.lineWidth = 2; ctx.setLineDash([7, 8]);
    ctx.beginPath(); ctx.moveTo(245, 346); ctx.lineTo(910, 346); ctx.stroke(); ctx.setLineDash([]); ctx.globalAlpha = 1;
  }

  // Where the coordinator stands: centred and a little larger when it works
  // alone, at the head of the room when it has delegates.
  function coordinatorSpot() {
    return session && session.agents.length ? { x: 145, y: 262, scale: 1.05 } : { x: 480, y: 258, scale: 1.3 };
  }

  // Name plate on the floor in front of a desk: title on top, a status dot
  // and status text below. Never drawn over the monitor or the person, which
  // is what made the old labels unreadable.
  function drawPlate(x, top, w, title, status, color, compact) {
    const h = compact ? 24 : 42;
    ctx.save();
    ctx.shadowColor = '#4a2e2040'; ctx.shadowBlur = 6; ctx.shadowOffsetY = 2;
    rounded(ctx, x - w / 2, top, w, h, 8, PALETTE.paper, '#d7b992');
    ctx.shadowBlur = 0; ctx.shadowOffsetY = 0;
    ctx.textBaseline = 'alphabetic'; ctx.textAlign = 'left';
    if (!compact) {
      ctx.fillStyle = PALETTE.ink; ctx.font = '600 12px system-ui, sans-serif';
      const t = fitText(ctx, title, w - 18); ctx.fillText(t, x - ctx.measureText(t).width / 2, top + 17);
    }
    ctx.font = (compact ? '600 ' : '500 ') + '11px system-ui, sans-serif';
    const line = fitText(ctx, compact ? title : status, w - 30);
    const lw = ctx.measureText(line).width, sx = x - (lw + 12) / 2, ly = compact ? top + 16 : top + 34;
    ctx.fillStyle = color; ctx.beginPath(); ctx.arc(sx + 4, ly - 4, 4, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = compact ? PALETTE.ink : PALETTE.muted; ctx.fillText(line, sx + 12, ly);
    ctx.restore();
  }

  function stepText(step, limit) {
    if (step == null) return '';
    const word = (session && session.labels && session.labels.step) || 'Step';
    return `${word} ${step}${limit == null ? '' : `/${limit}`}`;
  }

  // At most 12 desks stay legible (3 rows of 4): working agents and the
  // selected one first, stable order otherwise. The rest count as "+N";
  // the DOM bays below list everyone.
  const maxVisibleAgents = 12;
  function visibleAgents() {
    const list = session?.agents || [];
    const rank = a => (a.id === selectedId ? -2 : childStatus(a) === 'working' ? -1 : 0);
    const ordered = [...list].sort((a, b) => rank(a) - rank(b));
    return { shown: ordered.slice(0, maxVisibleAgents), hidden: Math.max(0, ordered.length - maxVisibleAgents) };
  }

  function layoutAgents() {
    const { shown } = visibleAgents();
    const rows = Math.max(1, Math.ceil(shown.length / 4));
    const top = rows === 1 ? 262 : 172;
    const bottom = rows === 1 ? 262 : 302;
    const gap = rows === 1 ? 0 : (bottom - top) / (rows - 1);
    return shown.map((agent, i) => ({ agent, i, col: i % 4, row: Math.floor(i / 4), x: 347 + (i % 4) * 145, y: rows === 1 ? top : top + Math.floor(i / 4) * gap, rows, rowGap: rows === 1 ? 0 : gap }));
  }

  function drawDesk(x, y, width, scale = 1, active = false) {
    ctx.save();
    const monitorW = 38 * scale, monitorH = 28 * scale, mx = x - monitorW / 2, my = y - 67 * scale;
    ctx.fillStyle = '#624b39'; rounded(ctx, mx - 4 * scale, my - 4 * scale, monitorW + 8 * scale, monitorH + 7 * scale, 4 * scale, '#705540');
    ctx.shadowColor = active ? '#69d9b3' : '#93c6b0'; ctx.shadowBlur = active ? 19 : 5;
    const grad = ctx.createLinearGradient(mx, my, mx + monitorW, my + monitorH); grad.addColorStop(0, '#24433e'); grad.addColorStop(1, '#91cbbb');
    ctx.fillStyle = grad; rounded(ctx, mx, my, monitorW, monitorH, 2, grad); ctx.shadowBlur = 0;
    ctx.fillStyle = active ? '#b8f7d7' : '#d8eee2';
    const lineCount = active ? 3 : 2; for (let n = 0; n < lineCount; n++) ctx.fillRect(mx + 5 * scale, my + (7 + n * 6) * scale, (10 + ((n * 7 + x) % 17)) * scale, 2 * scale);
    ctx.fillStyle = '#644b37'; ctx.fillRect(x - 3 * scale, my + monitorH, 6 * scale, 7 * scale); ctx.fillRect(x - 14 * scale, my + monitorH + 6 * scale, 28 * scale, 3 * scale);
    const dx = x - width / 2; const surfaceY = y - 43 * scale;
    ctx.fillStyle = '#754d36'; ctx.fillRect(dx + 5 * scale, surfaceY + 8 * scale, width - 10 * scale, 17 * scale);
    ctx.fillStyle = PALETTE.woodLight; rounded(ctx, dx, surfaceY, width, 10 * scale, 3 * scale, PALETTE.woodTop, '#895d41');
    ctx.fillStyle = '#80563d'; ctx.fillRect(dx + 10 * scale, surfaceY + 22 * scale, 7 * scale, 22 * scale); ctx.fillRect(dx + width - 17 * scale, surfaceY + 22 * scale, 7 * scale, 22 * scale);
    ctx.fillStyle = '#e5c89e'; ctx.fillRect(x - 13 * scale, surfaceY - 3 * scale, 26 * scale, 2 * scale);
    ctx.restore();
  }

  function agentPosition(item, at) {
    const target = { x: item.x, y: item.y };
    const visual = agentVisuals.get(item.agent.id);
    if (!visual || visual.walkStart == null || motion.matches) return target;
    const p = Math.max(0, Math.min(1, (at - visual.walkStart) / 2200));
    if (p >= 1) return target;
    const start = { x: 232, y: 315 };
    const aisle = { x: target.x, y: 315 };
    if (p < .68) { const q = p / .68; return { x: start.x + (aisle.x - start.x) * q, y: start.y + (aisle.y - start.y) * q }; }
    const q = (p - .68) / .32;
    return { x: target.x, y: aisle.y + (target.y - 44 - aisle.y) * q };
  }

  function drawPixelPerson(img, x, footY, scale, at, active, moving) {
    if (!img?.complete || !img.naturalWidth) {
      ctx.fillStyle = '#d7a17f'; ctx.beginPath(); ctx.arc(x, footY - 39 * scale, 8 * scale, 0, Math.PI * 2); ctx.fill();
      ctx.fillStyle = '#5a8c7c'; ctx.fillRect(x - 9 * scale, footY - 31 * scale, 18 * scale, 24 * scale); return;
    }
    const bob = motion.matches ? 0 : moving ? Math.abs(Math.sin(at / 90 + x)) * 3 : active ? Math.sin(at / 150 + x) * 1.5 : 0;
    const w = img.naturalWidth * scale, h = img.naturalHeight * scale;
    ctx.drawImage(img, x - w / 2, footY - h + bob, w, h);
    if (active) {
      ctx.fillStyle = '#e3aa55'; ctx.globalAlpha = .24; ctx.beginPath(); ctx.ellipse(x, footY - 2, 24 * scale, 8 * scale, 0, 0, Math.PI * 2); ctx.fill(); ctx.globalAlpha = 1;
    }
  }

  function stateLabel(session, state) {
    const labels = (session && session.labels) || DEFAULT_LABELS;
    return labels[state] || DEFAULT_LABELS[state] || 'IDLE';
  }



  function drawCoordinator(at) {
    if (!session) return;
    const { x, y, scale } = coordinatorSpot(); const k = scale / 1.05;
    const active = ACTIVE.has(session.state);
    // Coordinator table and chair: a real session anchors the office.
    ctx.fillStyle = '#6f503a'; ctx.fillRect(x - 56 * k, y - 51 * k, 114 * k, 8 * k); ctx.fillStyle = '#d0a373'; ctx.fillRect(x - 61 * k, y - 59 * k, 124 * k, 10 * k);
    ctx.fillStyle = '#80563d'; ctx.fillRect(x - 50 * k, y - 49 * k, 7 * k, 44 * k); ctx.fillRect(x + 45 * k, y - 49 * k, 7 * k, 44 * k);
    drawPixelPerson(images[0], x, y - 46 * k, 2.45 * k, at, active, false);
    drawDesk(x, y, 142 * k, scale, active);
    const color = active ? PALETTE.active : session.state === 'failed' ? PALETTE.failed : session.state === 'waiting' ? '#b97f2e' : session.state === 'completed' ? PALETTE.done : PALETTE.mintDark;
    const delivered = session.agents.filter(a => childStatus(a) === 'completed').length;
    const deliveredText = delivered ? ((session.labels && session.labels.delivered) || '{n} delivered').replace('{n}', delivered) : '';
    const status = [stateLabel(session, session.state), stepText(session.step, session.limit), deliveredText].filter(Boolean).join(' · ');
    drawPlate(x, y + 12 * k, session.agents.length ? 196 : 270, session.title || session.project || 'Coordinator', status, color, false);
  }

  function drawAgent(item, at) {
    const { agent, i, x, y, rows, rowGap } = item;
    const state = childStatus(agent); const active = state === 'working'; const failed = state === 'failed';
    const scale = rows > 2 ? 1 : rows > 1 ? 1.3 : 1.9;
    const width = Math.max(68, Math.min(126, 134 - scale * 3));
    const visual = agentVisuals.get(agent.id); const pos = agentPosition(item, at);
    const walking = visual?.walkStart != null && at - visual.walkStart < 2200 && !motion.matches;
    const color = failed ? PALETTE.failed : state === 'completed' ? PALETTE.done : state === 'cancelled' ? PALETTE.canceled : PALETTE.mintDark;

    // Ground shadow and state outline identify each actual delegate station.
    ctx.fillStyle = '#6e513c22'; ctx.beginPath(); ctx.ellipse(x, y + 1, width * .51, 10 * scale, 0, 0, Math.PI * 2); ctx.fill();
    ctx.strokeStyle = selectedId === agent.id ? '#f2bd61' : (active ? '#8ecbb1' : '#9c7858'); ctx.lineWidth = selectedId === agent.id ? 3 : 1.5;
    rounded(ctx, x - width / 2, y - 46 * scale, width, 53 * scale, 9, selectedId === agent.id ? '#f5dc9b55' : '#fff5e526', ctx.strokeStyle);

    if (state === 'failed') {
      ctx.fillStyle = PALETTE.failed; ctx.beginPath(); ctx.arc(x + width / 2 - 4, y - 42 * scale, 8 * scale, 0, Math.PI * 2); ctx.fill();
      ctx.fillStyle = '#fffaf0'; ctx.font = `700 ${10 * scale}px system-ui`; ctx.textAlign = 'center'; ctx.textBaseline = 'middle'; ctx.fillText('!', x + width / 2 - 4, y - 42 * scale + 1);
    }

    // Seated behind the desk (head above the monitor), like the coordinator;
    // standing at the desk front hid them between the legs of the table.
    const foot = walking ? pos.y : y - 44 * scale;
    drawPixelPerson(images[1 + (i % 3)], pos.x, foot, scale * 1.2, at, active, walking);
    drawDesk(x, y, width * .83, scale, active);
    // One row: full plate (task + status). Two rows: one-line plate. Three
    // rows: only the status dot on the desk; the list below has the rest.
    const plateColor = active ? PALETTE.active : color;
    if (rows === 1) drawPlate(x, y + 10, 136, agent.task || agent.id, [stateLabel(session, state), stepText(agent.step, agent.max_steps)].filter(Boolean).join(' · '), plateColor, false);
    else if (rows === 2) drawPlate(x, y + 8, 130, agent.task || agent.id, '', plateColor, true);
    else { ctx.fillStyle = plateColor; ctx.beginPath(); ctx.arc(x, y - 30 * scale, 5, 0, Math.PI * 2); ctx.fill(); }
    if (active) {
      ctx.fillStyle = '#95f2c744'; ctx.beginPath(); ctx.ellipse(x, y - 69 * scale, 29 * scale, 13 * scale, 0, 0, Math.PI * 2); ctx.fill();
    }

    if (state === 'completed' && visual?.deliveryStart != null && !visual.delivered) {
      const p = Math.max(0, Math.min(1, (at - visual.deliveryStart) / 1800));
      const ease = p * p * (3 - 2 * p);
      const home = coordinatorSpot();
      drawEnvelope(ctx, x + (home.x - x) * ease, y - 67 * scale + (home.y - 59 - (y - 67 * scale)) * ease, scale * .72);
      if (p >= 1) visual.delivered = true;
    }
    if (selectedId === agent.id) {
      ctx.strokeStyle = '#f5bf59'; ctx.lineWidth = 2; ctx.beginPath(); ctx.ellipse(x, y + 4, width * .45, 6 * scale, 0, 0, Math.PI * 2); ctx.stroke();
    }
  }

  // ---- Role room -------------------------------------------------------
  // One character per role. Idle roles stand dimmed beside their desk; the
  // role holding the turn walks to its chair, its monitor glows and a
  // speech bubble says what it is doing. Handoffs fly as an envelope with
  // a label (and file chips) from desk to desk; test results pop as a
  // badge on the tester's monitor.
  const FLIGHT_MS = 1300, WALK_MS = 750, POP_MS = 520;
  let actors = new Map();
  let flights = [];
  let pops = new Map();
  let highlightSeq = 0;
  const roleMode = () => Boolean(session && session.roles && session.kind !== 'team');
  const tr = (key, fallback) => {
    const v = session && session.T ? session.T(key) : '';
    return v && v !== key ? v : fallback;
  };
  const ROLE_NAMES = { coordinator: 'Coordinator', researcher: 'Researcher', builder: 'Builder', tester: 'Tester', reviewer: 'Reviewer' };
  const roleName = role => tr('workshop.role.' + role, ROLE_NAMES[role] || role);

  function roomItems() {
    const R = rolesApi();
    const extra = R ? R.extraAgents(session.roles) : [];
    const layout = roomLayout(extra.length);
    return { layout, extra: extra.slice(0, layout.extras.length) };
  }

  function seatOf(spot) { return { x: spot.x, y: spot.y - 44 * spot.scale }; }
  function standOf(spot) { return { x: spot.x + 60 * spot.scale, y: spot.y + 4 }; }

  // syncActors: walking targets follow the role states; a role that
  // receives a handoff starts walking when the envelope lands.
  function syncActors(at, arrivals) {
    const R = rolesApi(); if (!R || !roleMode()) return;
    const states = R.roleStates(session.roles);
    const { layout } = roomItems();
    for (const spot of layout.roles) {
      const want = states[spot.role] === 'active' ? 'seat' : 'stand';
      let actor = actors.get(spot.role);
      const target = want === 'seat' ? seatOf(spot) : standOf(spot);
      if (!actor) { actors.set(spot.role, { pos: target, from: target, to: target, start: 0, place: want }); continue; }
      if (actor.place === want) { actor.to = target; continue; }
      const delay = want === 'seat' && arrivals.has(spot.role) ? arrivals.get(spot.role) : 0;
      actor.from = actorPos(actor, at); actor.to = target; actor.place = want;
      actor.start = motion.matches ? 0 : at + Math.max(0, delay);
    }
  }

  function actorPhase(actor, at) {
    if (!actor.start) return 1;
    return Math.max(0, Math.min(1, (at - actor.start) / WALK_MS));
  }

  function actorPos(actor, at) {
    if (!actor.start || motion.matches) return actor.to;
    if (at < actor.start) return actor.from;
    const p = actorPhase(actor, at);
    if (p >= 1) { actor.start = 0; return actor.to; }
    const e = p * p * (3 - 2 * p);
    // Walk sideways first, then settle into the chair (or step out of it).
    const mid = { x: actor.to.x, y: actor.from.y };
    if (e < .6) { const q = e / .6; return { x: actor.from.x + (mid.x - actor.from.x) * q, y: actor.from.y }; }
    const q = (e - .6) / .4; return { x: mid.x, y: mid.y + (actor.to.y - mid.y) * q };
  }

  function spotFor(role) {
    return roomItems().layout.roles.find(s => s.role === role);
  }

  // Crisp vector icons: no emoji font dependence.
  function icon(kind, x, y, color) {
    ctx.save(); ctx.translate(x, y); ctx.strokeStyle = color; ctx.fillStyle = color; ctx.lineWidth = 2; ctx.lineCap = 'round'; ctx.lineJoin = 'round';
    ctx.beginPath();
    switch (kind) {
      case 'read': ctx.arc(-1.5, -1.5, 4.6, 0, Math.PI * 2); ctx.moveTo(2, 2); ctx.lineTo(6, 6); ctx.stroke(); break;
      case 'web': ctx.arc(0, 0, 6.2, 0, Math.PI * 2); ctx.moveTo(-6.2, 0); ctx.lineTo(6.2, 0); ctx.stroke(); ctx.beginPath(); ctx.ellipse(0, 0, 2.6, 6.2, 0, 0, Math.PI * 2); ctx.stroke(); break;
      case 'edit': ctx.moveTo(-5.5, 5.5); ctx.lineTo(-4.5, 1.5); ctx.lineTo(3.5, -6.5); ctx.lineTo(6.5, -3.5); ctx.lineTo(-1.5, 4.5); ctx.closePath(); ctx.stroke(); break;
      case 'run': ctx.moveTo(-3.5, -5.5); ctx.lineTo(5.5, 0); ctx.lineTo(-3.5, 5.5); ctx.closePath(); ctx.fill(); break;
      case 'ok': ctx.moveTo(-5.5, 0); ctx.lineTo(-1.8, 4); ctx.lineTo(5.5, -4.5); ctx.lineWidth = 2.6; ctx.stroke(); break;
      case 'fail': ctx.moveTo(-4.5, -4.5); ctx.lineTo(4.5, 4.5); ctx.moveTo(4.5, -4.5); ctx.lineTo(-4.5, 4.5); ctx.lineWidth = 2.6; ctx.stroke(); break;
      case 'review': ctx.ellipse(0, 0, 6.6, 4, 0, 0, Math.PI * 2); ctx.stroke(); ctx.beginPath(); ctx.arc(0, 0, 1.9, 0, Math.PI * 2); ctx.fill(); break;
      case 'wait': ctx.arc(0, 0, 6.2, 0, Math.PI * 2); ctx.moveTo(0, -3.5); ctx.lineTo(0, 0); ctx.lineTo(3, 2); ctx.stroke(); break;
      case 'think': [-4.5, 0, 4.5].forEach(dx => { ctx.moveTo(dx + 1.6, 0); ctx.arc(dx, 0, 1.6, 0, Math.PI * 2); }); ctx.fill(); break;
      default: ctx.rect(-4.5, -5.5, 9, 11); ctx.moveTo(-2.2, -2); ctx.lineTo(2.2, -2); ctx.moveTo(-2.2, 1); ctx.lineTo(2.2, 1); ctx.moveTo(-2.2, 3.6); ctx.lineTo(.8, 3.6); ctx.stroke(); break;
    }
    ctx.restore();
  }

  const TONE = { live: '#c77d22', ok: PALETTE.done, fail: PALETTE.failed, wait: '#b97f2e' };
  function drawBubble(x, headY, b, at, popStart) {
    ctx.save();
    ctx.font = '600 12px system-ui, sans-serif'; ctx.textBaseline = 'middle'; ctx.textAlign = 'left';
    const text = fitText(ctx, b.text, 190);
    const w = Math.round(ctx.measureText(text).width + 42), h = 28;
    let bx = Math.round(x - w / 2); bx = Math.max(6, Math.min(WIDTH - w - 6, bx));
    const by = Math.round(headY - h - 10);
    if (popStart != null && !motion.matches) {
      const p = Math.min(1, (at - popStart) / POP_MS);
      const s = p < 1 ? .55 + .45 * (1 - Math.pow(1 - p, 3)) + Math.sin(p * Math.PI) * .1 : 1;
      ctx.translate(x, by + h); ctx.scale(s, s); ctx.translate(-x, -(by + h));
    }
    const tone = TONE[b.tone] || TONE.live;
    ctx.shadowColor = '#3a281c40'; ctx.shadowBlur = 8; ctx.shadowOffsetY = 2;
    rounded(ctx, bx, by, w, h, 10, PALETTE.paper, null); ctx.shadowBlur = 0; ctx.shadowOffsetY = 0;
    const tipX = Math.max(bx + 14, Math.min(bx + w - 14, x));
    ctx.fillStyle = PALETTE.paper; ctx.beginPath(); ctx.moveTo(tipX - 6, by + h - 1); ctx.lineTo(tipX, by + h + 7); ctx.lineTo(tipX + 6, by + h - 1); ctx.closePath(); ctx.fill();
    ctx.strokeStyle = tone; ctx.lineWidth = 1.5;
    ctx.beginPath(); ctx.moveTo(bx + 10, by); ctx.arcTo(bx + w, by, bx + w, by + h, 10); ctx.arcTo(bx + w, by + h, bx, by + h, 10);
    ctx.lineTo(tipX + 6, by + h); ctx.lineTo(tipX, by + h + 7); ctx.lineTo(tipX - 6, by + h);
    ctx.arcTo(bx, by + h, bx, by, 10); ctx.arcTo(bx, by, bx + w, by, 10); ctx.closePath(); ctx.stroke();
    ctx.fillStyle = tone; ctx.globalAlpha = .15; ctx.beginPath(); ctx.arc(bx + 16, by + h / 2, 10, 0, Math.PI * 2); ctx.fill(); ctx.globalAlpha = 1;
    icon(b.icon, bx + 16, by + h / 2, tone);
    ctx.fillStyle = PALETTE.ink; ctx.fillText(text, bx + 32, by + h / 2 + .5);
    ctx.restore();
  }

  function drawBadge(x, y, ok, at, popStart) {
    ctx.save();
    let s = 1;
    if (popStart != null && !motion.matches) { const p = Math.min(1, (at - popStart) / POP_MS); s = p < 1 ? .3 + .7 * p + Math.sin(p * Math.PI) * .3 : 1; }
    ctx.translate(x, y); ctx.scale(s, s);
    ctx.fillStyle = ok ? PALETTE.done : PALETTE.failed; ctx.strokeStyle = PALETTE.paper; ctx.lineWidth = 2;
    ctx.beginPath(); ctx.arc(0, 0, 10, 0, Math.PI * 2); ctx.fill(); ctx.stroke();
    icon(ok ? 'ok' : 'fail', 0, 0, PALETTE.paper);
    ctx.restore();
  }

  function drawRolePlate(spot, state) {
    const labels = session.labels || DEFAULT_LABELS;
    let status = state === 'active' ? stateLabel(session, session.state === 'waiting' ? 'waiting' : 'working') : state === 'failed' ? (spot.role === 'reviewer' ? tr('workshop.bubble.reviewFail', 'Changes requested') : (labels.failed || 'FAILED')) : state === 'done' ? tr('workshop.roleDone', 'Took part') : tr('workshop.roleIdle', 'Free');
    if (spot.role === 'coordinator') status = [stateLabel(session, session.state), stepText(session.step, session.limit)].filter(Boolean).join(' · ');
    const color = state === 'active' ? PALETTE.active : state === 'failed' ? PALETTE.failed : state === 'done' ? PALETTE.done : '#a59a86';
    drawPlate(spot.x, spot.y + 12, 164, roleName(spot.role), status, color, false);
    ctx.fillStyle = ROLE_COLOR[spot.role]; ctx.fillRect(spot.x - 82, spot.y + 20, 4, 26);
  }

  function drawRole(spot, states, at) {
    const R = rolesApi();
    const state = states[spot.role];
    const active = state === 'active';
    const actor = actors.get(spot.role) || { to: standOf(spot), from: standOf(spot), start: 0, place: 'stand' };
    const pos = actorPos(actor, at);
    const phase = actorPhase(actor, at);
    const walking = Boolean(actor.start) && at >= actor.start && !motion.matches;
    const seated = actor.place === 'seat' && !walking && !(actor.start && at < actor.start);
    const selected = selectedId === 'role:' + spot.role;
    const lane = highlightSeq ? session.roles.lanes.find(l => l.seq === highlightSeq) : null;
    const highlighted = Boolean(lane && lane.role === spot.role);
    ctx.fillStyle = '#6e513c22'; ctx.beginPath(); ctx.ellipse(spot.x + 12, spot.y + 4, 88 * spot.scale, 12, 0, 0, Math.PI * 2); ctx.fill();
    if (selected || highlighted) {
      ctx.strokeStyle = highlighted ? '#f2bd61' : ROLE_COLOR[spot.role]; ctx.lineWidth = 3;
      ctx.beginPath(); ctx.ellipse(spot.x + 12, spot.y + 4, 90 * spot.scale, 13, 0, 0, Math.PI * 2); ctx.stroke();
    }
    const img = images[ROLE_SPRITE[spot.role]];
    const personScale = 2.6 * spot.scale;
    const draw = () => {
      ctx.save();
      if (!active) ctx.globalAlpha = state === 'idle' ? .55 : .85;
      const filters = [];
      if (ROLE_FILTER[spot.role]) filters.push(ROLE_FILTER[spot.role]);
      if (state === 'idle') filters.push('grayscale(.6)');
      if (filters.length && 'filter' in ctx) ctx.filter = filters.join(' ');
      if (walking && actor.to.x < actor.from.x) { ctx.translate(pos.x * 2, 0); ctx.scale(-1, 1); }
      drawPixelPerson(img, pos.x, pos.y, personScale, at, active && seated, walking);
      ctx.restore();
    };
    // Seated (or walking in behind the desk): drawn first so the desk hides
    // the legs. Standing beside the desk: drawn after it, in front.
    const behind = seated || (walking && actor.place === 'seat' && phase > .6) || (walking && actor.place === 'stand' && phase < .4);
    if (behind) draw();
    ctx.save(); if (state === 'idle') ctx.globalAlpha = .8;
    drawDesk(spot.x, spot.y, 118 * spot.scale, spot.scale, active && seated);
    ctx.restore();
    if (!behind) draw();
    drawRolePlate(spot, state);
    const testerLanes = session.roles.lanes.filter(l => l.role === 'tester' && l.done && l.result);
    const lastTest = testerLanes[testerLanes.length - 1];
    if (spot.role === 'tester' && lastTest) drawBadge(spot.x - 20 * spot.scale, spot.y - 70 * spot.scale, lastTest.result === 'ok', at, pops.get('tester:' + lastTest.seq));
    let b = R ? R.bubble(session.roles, spot.role, session.T) : null;
    if (highlighted) b = { icon: R.laneIcon(lane), tone: lane.result === 'fail' ? 'fail' : (lane.role === 'tester' && lane.result === 'ok') ? 'ok' : 'live', text: R.laneText(lane, session.T) };
    if (b) {
      const headY = pos.y - 30 * personScale;
      const pop = spot.role === 'tester' && lastTest && (b.tone === 'ok' || b.tone === 'fail') ? pops.get('tester:' + lastTest.seq) : null;
      drawBubble(pos.x, headY, b, at, pop);
    }
  }

  function drawExtra(agent, spot, i, at) {
    const state = childStatus(agent); const active = state === 'working';
    const visual = agentVisuals.get(agent.id) || {};
    const walking = visual.walkStart != null && at - visual.walkStart < 1800 && !motion.matches;
    let x = spot.x, foot = spot.y - 44 * spot.scale;
    if (walking) { const p = (at - visual.walkStart) / 1800; const e = p * p * (3 - 2 * p); x = 940 + (spot.x - 940) * e; }
    const selected = selectedId === agent.id;
    if (selected) { ctx.strokeStyle = '#f2bd61'; ctx.lineWidth = 2.5; ctx.beginPath(); ctx.ellipse(spot.x, spot.y + 2, 58, 9, 0, 0, Math.PI * 2); ctx.stroke(); }
    ctx.save(); if (!active) ctx.globalAlpha = .75;
    drawPixelPerson(images[1 + (i % 3)], x, foot, 1.7, at, active, walking);
    ctx.restore();
    drawDesk(spot.x, spot.y, 90, spot.scale, active);
    const color = state === 'failed' ? PALETTE.failed : state === 'completed' ? PALETTE.done : active ? PALETTE.active : PALETTE.canceled;
    drawPlate(spot.x, spot.y + 6, 128, agent.agent || agent.task || agent.id, '', color, true);
    if (active && agent.tool) {
      const kind = /web|browser/.test(agent.tool) ? 'web' : /edit|write|patch/.test(agent.tool) ? 'edit' : /bash/.test(agent.tool) ? 'run' : 'read';
      drawBubble(spot.x, foot - 52, { icon: kind, tone: 'live', text: agent.tool }, at, null);
    }
    if (state === 'completed' && visual.deliveryStart != null && !visual.delivered) {
      const p = Math.max(0, Math.min(1, (at - visual.deliveryStart) / 1800)); const e = p * p * (3 - 2 * p);
      const home = spotFor('coordinator');
      drawEnvelope(ctx, spot.x + (home.x - spot.x) * e, spot.y - 60 + (home.y - 120 - (spot.y - 60)) * e - Math.sin(e * Math.PI) * 40, 1);
      if (p >= 1) visual.delivered = true;
    }
  }

  function drawFlights(at) {
    flights = flights.filter(f => at - f.start < FLIGHT_MS + 150);
    for (const f of flights) {
      if (at < f.start) continue;
      const p = Math.max(0, Math.min(1, (at - f.start) / FLIGHT_MS));
      // A low arc that never leaves the floor: long hops stay readable.
      const arc = q => Math.sin(q * Math.PI) * Math.min(70, 34 + Math.abs(f.to.x - f.from.x) * .05);
      const ease = q => (q < .5 ? 2 * q * q : 1 - Math.pow(-2 * q + 2, 2) / 2);
      const e = ease(p);
      const x = f.from.x + (f.to.x - f.from.x) * e, y = Math.max(196, f.from.y + (f.to.y - f.from.y) * e - arc(e));
      f.chips.forEach((name, k) => {
        const q = ease(Math.max(0, p - (k + 1) * .08));
        const cx = f.from.x + (f.to.x - f.from.x) * q, cy = Math.max(196, f.from.y + (f.to.y - f.from.y) * q - arc(q)) + 18 + k * 4;
        ctx.save(); ctx.font = '600 11px ui-monospace, Consolas, monospace'; ctx.textBaseline = 'middle';
        const label = fitText(ctx, name, 130); const w = ctx.measureText(label).width + 22;
        ctx.shadowColor = '#3a281c33'; ctx.shadowBlur = 4;
        rounded(ctx, cx - w / 2, cy - 9, w, 18, 5, '#fbf6ea', '#9b7a55'); ctx.shadowBlur = 0;
        ctx.fillStyle = ROLE_COLOR[f.toRole] || '#c9a36b'; ctx.fillRect(cx - w / 2 + 5, cy - 5, 7, 10);
        ctx.fillStyle = PALETTE.ink; ctx.fillText(label, cx - w / 2 + 16, cy + .5); ctx.restore();
      });
      drawEnvelope(ctx, x, y, 1.4);
      if (f.label) {
        ctx.save(); ctx.font = '700 12px system-ui, sans-serif'; ctx.textBaseline = 'middle';
        const label = fitText(ctx, f.label, 180); const w = ctx.measureText(label).width + 18;
        ctx.shadowColor = '#3a281c40'; ctx.shadowBlur = 6;
        rounded(ctx, x - w / 2, y - 36, w, 22, 11, ROLE_COLOR[f.toRole] || PALETTE.ink, null); ctx.shadowBlur = 0;
        ctx.fillStyle = '#fffaf0'; ctx.fillText(label, x - w / 2 + 9, y - 24.5); ctx.restore();
      }
    }
  }

  // Whiteboard on the wall: the task and the turn's numbers, so the empty
  // wall says what the room is working on.
  function drawBoard() {
    const x = 34, y = 26, w = 290, h = 104;
    ctx.save();
    ctx.shadowColor = '#4a2e2033'; ctx.shadowBlur = 6; ctx.shadowOffsetY = 2;
    rounded(ctx, x - 6, y - 6, w + 12, h + 12, 8, PALETTE.wood, null); ctx.shadowBlur = 0; ctx.shadowOffsetY = 0;
    rounded(ctx, x, y, w, h, 4, '#fbf8f0', null);
    ctx.fillStyle = '#d9cdb5'; ctx.fillRect(x + 10, y + h + 2, 34, 4); ctx.fillStyle = '#c9584a'; ctx.fillRect(x + w - 40, y + h + 1, 14, 5); ctx.fillStyle = '#4f8fa0'; ctx.fillRect(x + w - 22, y + h + 1, 12, 5);
    ctx.textAlign = 'left'; ctx.textBaseline = 'alphabetic';
    ctx.fillStyle = PALETTE.muted; ctx.font = '700 11px system-ui, sans-serif';
    ctx.fillText(tr('workshop.boardTask', 'TASK').toUpperCase(), x + 12, y + 19);
    ctx.fillStyle = PALETTE.ink; ctx.font = '600 13px system-ui, sans-serif';
    // Two lines of the task, word-wrapped.
    const words = safe(session.task || session.title).replace(/\s+/g, ' ').trim().split(' ');
    const lines = []; let line = '';
    for (const word of words) {
      const next = line ? line + ' ' + word : word;
      if (ctx.measureText(next).width > w - 24 && line) { lines.push(line); line = word; if (lines.length === 2) break; } else line = next;
    }
    if (lines.length < 2 && line) lines.push(line);
    lines.slice(0, 2).forEach((l, i) => {
      const last = i === 1 && words.join(' ').length > lines.join(' ').length;
      ctx.fillText(last ? fitText(ctx, l + '…', w - 24) : fitText(ctx, l, w - 24), x + 12, y + 39 + i * 17);
    });
    const r = session.roles;
    const files = new Set(); r.lanes.forEach(l => l.files.forEach(f => files.add(f)));
    const stats = [
      [tr('workshop.boardSteps', 'steps'), String(r.lanes.length)],
      [tr('workshop.boardHandoffs', 'handoffs'), String(r.handoffs.length)],
      [tr('workshop.boardFiles', 'files'), String(files.size)],
    ];
    let sx = x + 12;
    ctx.font = '700 15px system-ui, sans-serif';
    for (const [label, value] of stats) {
      ctx.fillStyle = PALETTE.ink; ctx.fillText(value, sx, y + h - 13);
      const vw = ctx.measureText(value).width;
      ctx.font = '500 11px system-ui, sans-serif'; ctx.fillStyle = PALETTE.muted; ctx.fillText(label, sx + vw + 5, y + h - 13);
      sx += vw + ctx.measureText(label).width + 22; ctx.font = '700 15px system-ui, sans-serif';
    }
    ctx.restore();
  }

  function paintRoom(at) {
    const R = rolesApi();
    const states = R.roleStates(session.roles);
    const { layout, extra } = roomItems();
    drawBoard();
    // Back row first (further away), then the role desks in front.
    if (extra.length) {
      ctx.save(); ctx.font = '700 11px system-ui, sans-serif'; ctx.fillStyle = '#8b6b4f'; ctx.textAlign = 'left'; ctx.textBaseline = 'alphabetic';
      ctx.fillText(tr('workshop.childAgents', 'Child agents').toUpperCase(), 20, 172); ctx.restore();
      extra.forEach((agent, i) => drawExtra(agent, layout.extras[i], i, at));
      if (layout.hidden > 0) { rounded(ctx, 880, 196, 56, 22, 7, '#fff9ec', '#c9aa7d'); ctx.fillStyle = PALETTE.ink; ctx.font = '700 11px system-ui'; ctx.textAlign = 'center'; ctx.textBaseline = 'middle'; ctx.fillText(`+${layout.hidden}`, 908, 207); }
    }
    for (const spot of layout.roles) drawRole(spot, states, at);
    drawFlights(at);
  }

  function roomHasMotion(at) {
    if (flights.length) return true;
    for (const actor of actors.values()) if (actor.start) return true;
    for (const start of pops.values()) if (at - start < POP_MS) return true;
    if (ACTIVE.has(session.state)) return true;
    return session.agents.some(a => a.status === 'working');
  }

  // Called from update(): start animations for what is new since the
  // previous snapshot (only the newest two handoffs fly).
  function animateRoom(prev, at) {
    const R = rolesApi(); if (!R || !roleMode()) return;
    const arrivals = new Map();
    if (prev && prev.roles && prev.kind !== 'team' && prev.id === session.id) {
      const d = R.diff(prev.roles, session.roles);
      d.handoffs.slice(-2).forEach((h, k) => {
        const from = spotFor(h.from), to = spotFor(h.to);
        if (!from || !to || motion.matches) return;
        const start = at + k * 500;
        const fromActor = actors.get(h.from);
        const fromPos = fromActor ? actorPos(fromActor, at) : seatOf(from);
        flights.push({ start, from: { x: fromPos.x, y: fromPos.y - 62 }, to: { x: to.x, y: to.y - 86 * to.scale }, label: R.handoffLabel(h, session.T), toRole: h.to,
          chips: (h.to === 'builder' || h.to === 'tester') ? h.files.slice(0, 3).map(f => R.shortPath(f, 22)) : [] });
        arrivals.set(h.to, start - at + FLIGHT_MS - 200);
      });
      for (const l of d.results) pops.set('tester:' + l.seq, at);
    }
    syncActors(at, arrivals);
  }

  function hitRoom(x, y) {
    const { layout, extra } = roomItems();
    let best = null, bestDist = Infinity;
    layout.roles.forEach(spot => {
      const dx = x - spot.x - 12, dy = y - (spot.y - 50);
      if (Math.abs(dx) > 94 || dy < -125 || dy > 108) return;
      const dist = dx * dx + dy * dy * .6; if (dist < bestDist) { best = 'role:' + spot.role; bestDist = dist; }
    });
    extra.forEach((agent, i) => {
      const spot = layout.extras[i]; const dx = x - spot.x, dy = y - (spot.y - 40);
      if (Math.abs(dx) > 64 || Math.abs(dy) > 62) return;
      const dist = dx * dx + dy * dy; if (dist < bestDist) { best = agent.id; bestDist = dist; }
    });
    return best;
  }

  function paint(at = now()) {
    if (destroyed || !shown || document.hidden) return;
    const box = canvasSize(); if (!box) return;
    drawBackground();
    if (session && roleMode() && rolesApi()) {
      paintRoom(at);
    } else if (session) {
      if (session.agents.length) drawWalkway();
      drawCoordinator(at);
      const items = layoutAgents(); items.forEach(item => drawAgent(item, at));
      const { hidden } = visibleAgents();
      if (hidden > 0) {
        rounded(ctx, 872, 330, 76, 23, 7, '#fff9ec', '#c9aa7d');
        ctx.fillStyle = PALETTE.ink; ctx.font = '700 11px system-ui'; ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
        ctx.fillText(`+${hidden}`, 910, 342);
      }
    }
    if (hasMotion(at)) schedule(); else stopFrame();
  }

  function frame(at) {
    raf = 0;
    if (!shown || document.hidden || destroyed) return;
    if (at - lastFrame < 33) { raf = requestAnimationFrame(frame); return; }
    lastFrame = at; paint(at);
  }
  function schedule() { if (!raf && !motion.matches && shown && !document.hidden) raf = requestAnimationFrame(frame); }

  function onClick(event) {
    if (!session || typeof onSelectAgent !== 'function') return;
    const box = canvas.getBoundingClientRect(); if (!box.width || !box.height) return;
    const x = (event.clientX - box.left) * WIDTH / box.width;
    const y = (event.clientY - box.top) * HEIGHT / box.height;
    if (roleMode() && rolesApi()) {
      const hit = hitRoom(x, y);
      if (hit) { selectedId = hit; highlightSeq = 0; paint(now()); onSelectAgent(hit); }
      return;
    }
    // Nearest desk wins: rows can sit close, and first-match order
    // would open the wrong agent.
    let best = null, bestDist = Infinity;
    for (const item of layoutAgents()) {
      const dx = x - item.x, dy = y - (item.y - 45);
      if (Math.abs(dx) > 70 || dy < -70 || dy > 67) continue;
      const dist = dx * dx + dy * dy;
      if (dist < bestDist) { best = item; bestDist = dist; }
    }
    if (best) { selectedId = best.agent.id; paint(now()); onSelectAgent(best.agent.id); }
  }
  function onVisibility() { if (document.hidden) stopFrame(); else if (shown) paint(now()); }
  function onMotionChange() { if (motion.matches) { stopFrame(); paint(now()); } else if (shown) paint(now()); }
  function onResize() { if (shown) paint(now()); }
  function listenMedia(eventName, handler, add) {
    if (motion.addEventListener) motion[add ? 'addEventListener' : 'removeEventListener'](eventName, handler);
    else if (motion.addListener) motion[add ? 'addListener' : 'removeListener'](handler);
  }

  canvas.addEventListener('click', onClick);
  document.addEventListener('visibilitychange', onVisibility);
  listenMedia('change', onMotionChange, true);
  if (typeof ResizeObserver === 'function') { resizeObserver = new ResizeObserver(onResize); resizeObserver.observe(canvas); }

  return {
    update(rawSession, opts) {
      const next = normalize(rawSession, opts); const at = now();
      const previous = new Map((session?.agents || []).map(a => [a.id, a]));
      const nextVisuals = new Map();
      for (const agent of next?.agents || []) {
        const prior = previous.get(agent.id); const visual = agentVisuals.get(agent.id) || {};
        if (!prior && agent.status === 'working' && !motion.matches) visual.walkStart = at;
        if (!prior && agent.status !== 'working') visual.walkStart = null;
        if (prior && prior.status !== 'completed' && agent.status === 'completed') { visual.deliveryStart = at; visual.delivered = false; }
        nextVisuals.set(agent.id, visual);
      }
      const prev = session;
      agentVisuals = nextVisuals; session = next;
      if (!session || prev?.id !== session.id) { actors = new Map(); flights = []; pops = new Map(); highlightSeq = 0; }
      if (session) animateRoom(prev, at);
      if (selectedId && !selectedId.startsWith('role:') && !session?.agents.some(a => a.id === selectedId)) selectedId = '';
      if (highlightSeq && !session?.roles?.lanes.some(l => l.seq === highlightSeq)) highlightSeq = 0;
      if (shown) paint(at);
    },
    select(agentId) { selectedId = typeof agentId === 'string' && (agentId.startsWith('role:') || session?.agents.some(a => a.id === agentId)) ? agentId : ''; if (shown) paint(now()); },
    // highlight: a step picked in the timeline strip (0 clears it).
    highlight(seq) { highlightSeq = Number.isFinite(seq) ? seq : 0; if (shown) paint(now()); },
    show() { if (destroyed) return; shown = true; paint(now()); },
    hide() { shown = false; stopFrame(); },
    destroy() {
      destroyed = true; shown = false; stopFrame(); canvas.removeEventListener('click', onClick);
      document.removeEventListener('visibilitychange', onVisibility); listenMedia('change', onMotionChange, false);
      resizeObserver?.disconnect(); resizeObserver = null;
    },
  };
}
