const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const R = require('./workshop-roles.js');
const core = require('./replay-core.js');

const lane = (seq, role, kind, extra = {}) => ({ seq, step: seq, role, kind, target: '', n: 0, files: [], done: true, result: '', duration_ms: 1000, ...extra });

const activity = {
  state: 'tools', role: 'tester',
  agents: [{ id: 'scout-1', agent: 'scout', task: 'Investigate…', status: 'completed', output: 'stats.py has mean()' }, { id: 'd2', agent: '', task: 'side task', status: 'working' }],
  lanes: [
    lane(1, 'researcher', 'scout', { agent: 'scout-1', result: 'ok' }),
    lane(3, 'builder', 'edit', { target: 'src/stats.py', files: ['src/stats.py', 'tests/test_stats.py'], n: 2, result: 'ok' }),
    lane(5, 'tester', 'test', { command: 'python -m pytest -q', target: 'python -m pytest -q', done: false }),
  ],
  handoffs: [
    { seq: 2, from: 'researcher', to: 'coordinator', kind: 'brief' },
    { seq: 4, from: 'coordinator', to: 'builder', kind: 'edit', n: 2, files: ['src/stats.py', 'tests/test_stats.py'] },
    { seq: 6, from: 'builder', to: 'tester', kind: 'test' },
  ],
  role_usage: { researcher: { up_tokens: 1200, down_tokens: 300, calls: 2 } },
};

test('role states: the turn holder is active, past roles took part, others idle', () => {
  const a = R.normalize(activity);
  assert.equal(R.currentRole(a), 'tester');
  assert.deepEqual(R.roleStates(a), { coordinator: 'done', researcher: 'done', builder: 'done', tester: 'active', reviewer: 'idle' });
  // A working scout makes the researcher the active role.
  const scouting = R.normalize({ state: 'tools', role: 'researcher', agents: [{ id: 's', agent: 'scout', status: 'working', tool: 'read' }], lanes: [lane(1, 'researcher', 'scout', { done: false })] });
  assert.equal(R.roleStates(scouting).researcher, 'active');
  assert.equal(R.bubble(scouting, 'researcher').text, 'Reading the code');
  assert.equal(R.bubble(scouting, 'researcher').icon, 'read');
});

test('bubbles say what each role is doing, with crisp icon names', () => {
  const a = R.normalize(activity);
  assert.deepEqual(R.bubble(a, 'tester'), { icon: 'run', tone: 'live', text: 'Running tests' });
  assert.equal(R.bubble(a, 'builder'), null, 'only the active role talks');
  const done = R.normalize({ ...activity, state: 'thinking', role: 'tester', lanes: [...activity.lanes.slice(0, 2), { ...activity.lanes[2], done: true, result: 'fail' }] });
  assert.deepEqual(R.bubble(done, 'tester'), { icon: 'fail', tone: 'fail', text: 'Tests failed' });
  const T = key => ({ 'workshop.bubble.editN': 'Editant {n} fitxers', 'workshop.bubble.read': 'Llegint {x}' })[key] || key;
  assert.equal(R.laneText(activity.lanes[1], T), 'Editant 2 fitxers');
  assert.equal(R.laneText(lane(9, 'researcher', 'read', { target: 'internal/web/app/very/long/path/workshop-scene.js' }), T), 'Llegint …/path/workshop-scene.js');
  assert.equal(R.laneText(lane(9, 'researcher', 'web', { target: 'wikipedia.org' })), 'Fetching wikipedia.org');
  const waiting = R.normalize({ ...activity, state: 'waiting' });
  assert.equal(R.bubble(waiting, 'tester').icon, 'wait');
  const fresh = R.normalize({ state: 'thinking', role: 'coordinator', lanes: [] });
  assert.equal(R.bubble(fresh, 'coordinator').text, 'Planning');
});

test('handoff labels and short paths', () => {
  assert.equal(R.handoffLabel({ kind: 'edit', n: 3 }), '3 files to edit');
  assert.equal(R.handoffLabel({ kind: 'edit', n: 1 }), 'A file to edit');
  assert.equal(R.handoffLabel({ kind: 'test' }), 'Tests to run');
  assert.equal(R.handoffLabel({ kind: 'brief' }), 'Research brief');
  assert.equal(R.handoffLabel({ kind: 'other' }), 'Results');
  assert.equal(R.shortPath('src/stats.py'), 'src/stats.py');
  assert.equal(R.shortPath('a/b/c/d/e/internal/stats.py', 20), '…/internal/stats.py');
  assert.equal(R.shortPath('C:\\x\\y\\zzzzzzzzzzzzzzzzzzzzzz\\file.go', 16), 'file.go');
});

test('role summary feeds the detail panel; timeline has one segment per step', () => {
  const sum = R.roleSummary(activity, 'builder');
  assert.deepEqual(sum.files.map(f => f.path), ['src/stats.py', 'tests/test_stats.py']);
  assert.equal(sum.durationMs, 1000);
  assert.equal(sum.handoffsIn, 1);
  const tester = R.roleSummary(activity, 'tester');
  assert.deepEqual(tester.commands.map(c => c.command), ['python -m pytest -q']);
  const researcher = R.roleSummary(activity, 'researcher');
  assert.equal(researcher.children.length, 1, 'the scout is the researcher’s real child');
  assert.deepEqual(researcher.usage, { up: 1200, down: 300, calls: 2 });
  assert.deepEqual(R.timeline(activity).map(s => s.role), ['researcher', 'builder', 'tester']);
  assert.deepEqual(R.extraAgents(R.normalize(activity)).map(a => a.id), ['d2']);
});

test('diff only animates what is new', () => {
  const before = { ...activity, handoffs: activity.handoffs.slice(0, 2), role: 'builder', lanes: activity.lanes.slice(0, 2) };
  const d = R.diff(before, activity);
  assert.deepEqual(d.handoffs.map(h => h.seq), [6]);
  assert.deepEqual(d.activated, ['tester']);
  assert.deepEqual(d.released, ['builder']);
  const finished = { ...activity, lanes: [...activity.lanes.slice(0, 2), { ...activity.lanes[2], done: true, result: 'ok' }] };
  assert.deepEqual(R.diff(activity, finished).results.map(l => l.seq), [5]);
  assert.deepEqual(R.diff(finished, finished).handoffs, []);
});

test('malformed payloads never throw', () => {
  const a = R.normalize({ lanes: [null, { seq: 'x' }, { seq: 1, role: 'hacker', kind: 1 }], handoffs: [{ seq: 1, from: 'a', to: 'b' }], agents: 'no' });
  assert.equal(a.lanes.length, 1);
  assert.equal(a.lanes[0].role, 'coordinator');
  assert.equal(a.handoffs.length, 0);
  assert.equal(R.fmtDuration(0), '');
  assert.equal(R.fmtDuration(65000), '1m 05s');
});

test('replay rebuilds lanes and handoffs from durable events', () => {
  const at = i => new Date(Date.UTC(2026, 9, 13, 10, 0, i)).toISOString();
  const events = [
    { id: 1, kind: 'run_started', at: at(0) },
    { id: 2, kind: 'delegate', at: at(1), payload: { id: 'scout-1', agent: 'scout', status: 'working', task: 'Investigate' } },
    { id: 3, kind: 'lane', at: at(1), payload: { seq: 1, step: 0, role: 'researcher', kind: 'scout', done: false } },
    { id: 3.5, kind: 'delegate', at: at(5), payload: { id: 'scout-1', agent: 'scout', status: 'completed', output: 'brief' } },
    { id: 4, kind: 'lane', at: at(5), payload: { seq: 1, step: 0, role: 'researcher', kind: 'scout', done: true, result: 'ok', duration_ms: 4000 } },
    { id: 5, kind: 'handoff', at: at(5), payload: { seq: 2, from: 'researcher', to: 'coordinator', kind: 'brief' } },
    { id: 6, kind: 'handoff', at: at(7), payload: { seq: 3, from: 'coordinator', to: 'builder', kind: 'edit', n: 1, files: ['src/stats.py'] } },
    { id: 7, kind: 'lane', at: at(7), payload: JSON.stringify({ seq: 4, step: 2, role: 'builder', kind: 'edit', target: 'src/stats.py', files: ['src/stats.py'], done: false }) },
    { id: 8, kind: 'run_completed', at: at(9) },
  ];
  const tl = core.buildTimeline(events);
  const last = tl.frames[tl.frames.length - 1];
  assert.equal(last.handoffs.length, 2);
  assert.deepEqual(last.lanes.map(l => [l.seq, l.role, l.done]), [[1, 'researcher', true], [4, 'builder', true]]);
  const session = core.toSession(last, { id: 'replay-1' });
  assert.equal(session.activity.role, 'builder');
  assert.equal(session.activity.agents[0].agent, 'scout');
  // A mid-run frame: the builder holds the turn while editing.
  const mid = tl.frames.find(f => f.lanes.some(l => l.seq === 4) && f.state !== 'completed');
  assert.equal(R.roleStates(R.normalize(core.toSession(mid).activity)).builder, 'active');
});

test('room layout keeps five role desks in front and children in a bounded back row', () => {
  let source = fs.readFileSync(path.join(__dirname, 'workshop-scene.js'), 'utf8');
  source = source.replace(/^import .*;\s*$/gm, '').replace(/^export /gm, '');
  source += '\n;globalThis.__layout = roomLayout;';
  const sandbox = {};
  vm.runInNewContext(source, sandbox, { filename: 'workshop-scene.js' });
  const layout = sandbox.__layout(0);
  assert.equal(JSON.stringify(layout.roles.map(r => r.role)), JSON.stringify(['coordinator', 'researcher', 'builder', 'tester', 'reviewer']));
  assert.ok(layout.roles.every((r, i, all) => i === 0 || r.x - all[i - 1].x >= 150), 'desks never overlap');
  assert.equal(layout.extras.length, 0);
  const many = sandbox.__layout(9);
  assert.equal(many.extras.length, 6);
  assert.equal(many.hidden, 3);
  assert.ok(many.extras.every(e => e.y < layout.roles[0].y - 60), 'children sit behind the role row');
  assert.ok(many.extras.every(e => e.x >= 150 && e.x <= 810));
});

test('parallel stats count children that worked at the same time', () => {
  const t = s => new Date(Date.UTC(2026, 9, 16, 10, 0, s)).toISOString();
  const now = Date.parse(t(60));
  // Three children overlapping between 0:10 and 0:20, one alone later.
  const agents = [
    { id: 'a', status: 'completed', started_at: t(0), updated_at: t(20) },
    { id: 'b', status: 'completed', started_at: t(5), updated_at: t(25) },
    { id: 'c', status: 'working', started_at: t(10), updated_at: t(30) },
    { id: 'd', status: 'queued', started_at: t(12), updated_at: t(12) },
  ];
  assert.deepEqual(R.parallelStats(agents, now), { working: 1, queued: 1, peak: 3 });
  // Back to back is not parallel: one ends exactly when the next starts.
  const relay = [
    { id: 'a', status: 'completed', started_at: t(0), updated_at: t(10) },
    { id: 'b', status: 'completed', started_at: t(10), updated_at: t(20) },
  ];
  assert.equal(R.parallelStats(relay, now).peak, 1);
  assert.deepEqual(R.parallelStats(null, now), { working: 0, queued: 0, peak: 0 });
  // Spans put each child on the turn's time axis; overlapping bars overlap.
  const s = R.spans(agents.slice(0, 2), t(0), now);
  assert.deepEqual(s.a, { left: 0, width: 80 });
  assert.deepEqual(s.b, { left: 20, width: 80 });
});

test('several researchers sit apart so parallel scouts are all visible', () => {
  const one = R.normalize({ state: 'tools', agents: [{ id: 's1', agent: 'scout', status: 'working' }, { id: 'd', agent: '', status: 'working' }] });
  assert.deepEqual(R.extraAgents(one).map(a => a.id), ['d']);
  const many = R.normalize({ state: 'tools', role: 'researcher', agents: [
    { id: 's1', agent: 'scout', task: 'internal/team', status: 'working', tool: 'read' },
    { id: 's2', agent: 'scout', task: 'internal/web', status: 'working', tool: 'grep' },
    { id: 's3', agent: 'scout', task: 'internal/agent', status: 'completed' },
  ] });
  assert.deepEqual(R.extraAgents(many).map(a => a.id), ['s1', 's2', 's3']);
  assert.equal(R.roleStates(many).researcher, 'active');
  assert.equal(R.bubble(many, 'researcher').text, '2 researching in parallel');
});

test('tool lists collapse repeats', () => {
  assert.equal(R.toolList('read, read, read, grep'), 'read ×3 · grep');
  assert.equal(R.toolList('scout'), 'scout');
  assert.equal(R.toolList(''), '');
});
