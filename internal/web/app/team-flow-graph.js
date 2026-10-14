// Shared pipeline graph: who passes the task to whom, with live states.
// Used by the Team page and by Workshop team bays, so one room replaces
// two 2D views. Nodes: [{id}]. Edges: [{from,to,kind}] where kind is
// "data" or "revision". All text comes through callbacks (each page has
// its own language and naming); all state through the states map.
import { layoutFlow } from './team-status.js';

const svgNS = 'http://www.w3.org/2000/svg';
let graphSeq = 0;
// Node boxes are 156x34 centred on their position (see buildFlowGraph).
const HALF_W = 78, HALF_H = 17;

// A revision goes back up the pipeline: leave the source by the side that
// faces the target and enter the target from below, beside the data edge
// that leaves it, so the dashed line never crosses a box or the margin.
// Vertically aligned nodes loop out on the right instead.
export function revisionPath(x1, y1, x2, y2) {
  if (Math.abs(x2 - x1) < HALF_W) {
    const out = Math.max(x1, x2) + HALF_W + 46;
    return `M${x1 + HALF_W},${y1} C${out},${y1} ${out},${y2} ${x2 + HALF_W},${y2}`;
  }
  const dir = x2 > x1 ? 1 : -1;
  const sx = x1 + dir * HALF_W, ex = x2 + dir * 40;
  const ey = y2 < y1 ? y2 + HALF_H + 2 : y2 - HALF_H - 2;
  return `M${sx},${y1} C${ex},${y1} ${ex},${y1} ${ex},${ey}`;
}

export function buildFlowGraph(nodes, edges, opts = {}) {
  const { pos, height } = layoutFlow(nodes.map(n => ({ id: n.id })), edges.filter(e => e.kind === 'data'));
  const uid = 'g' + (++graphSeq);
  const svg = document.createElementNS(svgNS, 'svg');
  svg.setAttribute('viewBox', `0 0 1024 ${height}`);
  svg.setAttribute('role', 'group');
  svg.classList.add('flow-graph');
  const defs = document.createElementNS(svgNS, 'defs');
  // The fill is only a fallback: the classes take the theme colours (CSS).
  const marker = (id, color, cls) => {
    const m = document.createElementNS(svgNS, 'marker');
    m.id = id; m.setAttribute('viewBox', '0 0 10 10'); m.setAttribute('refX', '8'); m.setAttribute('refY', '5');
    m.setAttribute('markerWidth', '7'); m.setAttribute('markerHeight', '7'); m.setAttribute('orient', 'auto-start-reverse');
    const p = document.createElementNS(svgNS, 'path'); p.setAttribute('d', 'M0,0L10,5L0,10z'); p.setAttribute('fill', color);
    p.setAttribute('class', cls);
    m.append(p); return m;
  };
  defs.append(marker(`flowArrow-${uid}`, '#8890A2', 'flow-arrow'), marker(`flowArrowLive-${uid}`, '#f3a64a', 'flow-arrow-live'));
  svg.append(defs);
  svg._flowArrow = `flowArrow-${uid}`;
  svg._flowArrowLive = `flowArrowLive-${uid}`;
  for (const e of edges) {
    const a = pos[e.from], b = pos[e.to];
    if (!a || !b) continue;
    const [x1, y1] = a, [x2, y2] = b;
    const path = document.createElementNS(svgNS, 'path');
    const mx = (x1 + x2) / 2;
    path.setAttribute('d', e.kind === 'revision'
      ? revisionPath(x1, y1, x2, y2)
      : `M${x1},${y1 + 16} C${mx},${y1 + 34} ${mx},${y2 - 34} ${x2},${y2 - 16}`);
    path.dataset.flowEdge = `${e.from}>${e.to}`;
    if (e.kind === 'revision') path.classList.add('is-revision');
    svg.append(path);
  }
  const onSelect = typeof opts.onSelect === 'function' ? opts.onSelect : null;
  for (const n of nodes) {
    const p = pos[n.id];
    if (!p) continue;
    const [x, y] = p;
    const g = document.createElementNS(svgNS, 'g');
    g.dataset.flowNode = n.id; g.setAttribute('tabindex', '0'); g.setAttribute('role', 'button');
    const tip = document.createElementNS(svgNS, 'title'); g.append(tip);
    const rect = document.createElementNS(svgNS, 'rect');
    rect.setAttribute('x', x - 78); rect.setAttribute('y', y - 17); rect.setAttribute('width', 156); rect.setAttribute('height', 34); rect.setAttribute('rx', 9);
    const dot = document.createElementNS(svgNS, 'circle');
    dot.setAttribute('cx', x - 60); dot.setAttribute('cy', y); dot.setAttribute('r', 6); dot.setAttribute('class', 'flow-dot');
    const label = document.createElementNS(svgNS, 'text');
    label.setAttribute('x', x + 2); label.setAttribute('y', y + 5); label.setAttribute('class', 'flow-label');
    g.append(rect, dot, label);
    if (onSelect) {
      g.addEventListener('click', () => onSelect(n.id));
      g.addEventListener('keydown', ev => { if (ev.key === 'Enter' || ev.key === ' ') { ev.preventDefault(); onSelect(n.id); } });
    } else {
      g.setAttribute('aria-hidden', 'true');
      g.removeAttribute('tabindex');
      g.removeAttribute('role');
    }
    svg.append(g);
  }
  return svg;
}

export function updateFlowGraph(svg, nodes, view = {}) {
  const states = view.states || {};
  const selected = view.selected != null ? view.selected : null;
  const liveEdge = view.liveEdge || null;
  const shortName = typeof view.shortName === 'function' ? view.shortName : id => id;
  const fullName = typeof view.fullName === 'function' ? view.fullName : id => id;
  const stateName = typeof view.stateName === 'function' ? view.stateName : st => st;
  if (view.groupName != null) svg.setAttribute('aria-label', view.groupName);
  const arrow = svg._flowArrow || 'flowArrow';
  const arrowLive = svg._flowArrowLive || 'flowArrowLive';
  svg.querySelectorAll('path[data-flow-edge]').forEach(p => {
    const live = p.dataset.flowEdge === liveEdge;
    p.classList.toggle('flow-live', live);
    p.setAttribute('marker-end', `url(#${live ? arrowLive : arrow})`);
  });
  const nodeGroups = new Map();
  svg.querySelectorAll('[data-flow-node]').forEach(g => nodeGroups.set(g.dataset.flowNode, g));
  for (const n of nodes) {
    const g = nodeGroups.get(n.id);
    if (!g) continue;
    const st = states[n.id] || 'idle';
    g.setAttribute('class', 'team-flow-node' + (n.id === selected ? ' is-selected' : '') + (st === 'working' ? ' is-working' : st === 'completed' ? ' is-done' : st === 'failed' || st === 'cancelled' ? ' is-bad' : ''));
    g.setAttribute('aria-label', `${fullName(n.id)} · ${stateName(st)}`);
    const title = g.querySelector('title');
    if (title) title.textContent = fullName(n.id);
    g.querySelector('.flow-label').textContent = shortName(n.id);
    g.querySelector('.flow-dot').setAttribute('class', 'flow-dot dot-' + st);
  }
}
