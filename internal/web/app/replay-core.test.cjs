const { test } = require('node:test');
const assert = require('node:assert/strict');
const core = require('./replay-core.js');
const fun = require('./fun-core.js');

const at = s => new Date(Date.UTC(2026, 9, 13, 10, 0, 0) + s * 1000).toISOString();
let id = 0;
const ev = (s, kind, payload, text) => ({ id: ++id, at: at(s), run_id: 7, kind, payload, text: text || '' });

const trace = () => {
  id = 0;
  return [
    ev(0, 'run_queued'),
    ev(1, 'run_started'),
    ev(2, 'progress', { phase: 'model', state: 'waiting' }),
    ev(3, 'tool_call', { id: 'c1', name: 'read', args: '{"path":"a.go"}' }),
    ev(4, 'tool_result', { id: 'c1', name: 'read', output: 'package a' }),
    ev(5, 'delegate', { id: 'd1', task: 'write the parser', status: 'working', model: 'm' }),
    ev(6, 'delegate', { id: 'd1', status: 'working', tool: 'write' }),
    ev(7, 'delegate', { id: 'd2', task: 'review', status: 'working' }),
    // Una espera humana de 10 minuts: s'escurça a MAX_GAP.
    ev(607, 'approve_request', { name: 'bash' }),
    ev(608, 'delegate', { id: 'd1', status: 'completed', output: 'parser written' }),
    ev(609, 'review', { name: 'edit', decision: 'ok', reason: 'fine' }),
    ev(610, 'token', null, 'ignored'),
    ev(611, 'done', null, 'All good'),
    ev(612, 'run_completed'),
  ];
};

test('timeline reconstructs room states, children and tools from durable events', () => {
  const tl = core.buildTimeline(trace());
  const states = tl.frames.map(f => f.state);
  assert.equal(states[0], 'waiting');
  assert.ok(states.includes('thinking') && states.includes('tools') && states.includes('reviewing'));
  assert.equal(tl.outcome, 'completed');
  assert.equal(tl.tools, 1);
  assert.equal(tl.agents, 2);
  const toolFrame = tl.frames.find(f => f.state === 'tools');
  assert.equal(toolFrame.tool, 'read');
  const waiting = tl.frames.find(f => f.state === 'waiting' && f.tool === 'bash');
  assert.ok(waiting, 'approval wait is visible');
  // Children: d1 enters working with its tool, completes; d2 still working at the end is closed.
  const d1Tool = tl.frames.find(f => f.agents.some(a => a.id === 'd1' && a.tool === 'write'));
  assert.equal(d1Tool.agents.find(a => a.id === 'd1').task, 'write the parser');
  const last = tl.frames[tl.frames.length - 1];
  assert.deepEqual(last.agents.map(a => a.status), ['completed', 'completed']);
  // The 600 s gap is compressed; real duration is kept apart.
  assert.equal(tl.realMs, 612000);
  assert.ok(tl.duration < 40000 && tl.duration >= core.MAX_GAP, 'duration ' + tl.duration);
  // Frames are strictly ordered in replay time.
  for (let i = 1; i < tl.frames.length; i++) assert.ok(tl.frames[i].t > tl.frames[i - 1].t);
});

test('failed and cancelled runs end in that state and close working children', () => {
  id = 0;
  const tl = core.buildTimeline([
    ev(0, 'run_started'),
    ev(1, 'delegate', { id: 'x', task: 't', status: 'working' }),
    ev(2, 'run_failed', null, 'torn fallit: boom'),
  ]);
  assert.equal(tl.outcome, 'failed');
  assert.equal(tl.frames[tl.frames.length - 1].agents[0].status, 'cancelled');
  assert.deepEqual(core.buildTimeline([]).frames, []);
  assert.equal(core.buildTimeline(null).duration, 0);
});

test('scrubbing, speed and the scene session shape', () => {
  const tl = core.buildTimeline(trace());
  assert.equal(core.frameIndexAt(tl, -5), 0);
  assert.equal(core.frameIndexAt(tl, tl.duration + 1), tl.frames.length - 1);
  const mid = tl.frames[3];
  assert.equal(core.frameIndexAt(tl, mid.t), 3);
  assert.equal(core.frameIndexAt(tl, mid.t + 1), 3);
  assert.equal(core.frameIndexAt({ frames: [] }, 3), -1);
  assert.equal(core.advance(0, 1000, 10, 50000), 10000);
  assert.equal(core.advance(0, 1000, 7, 50000), 1000, 'unknown speed falls back to 1x');
  assert.equal(core.advance(49000, 1000, 4, 50000), 50000, 'clamped at the end');
  const s = core.toSession(tl.frames.find(f => f.agents.length === 2), { id: 'r', title: 'Fix parser' });
  assert.equal(s.activity.agents.length, 2);
  assert.equal(s.activity.task, 'Fix parser');
  assert.equal(core.toSession(null), null);
  assert.equal(core.fmtClock(65000), '1:05');
  assert.equal(core.fmtClock(3723000), '1:02:03');
});

test('work groups map to finished runs from the end', () => {
  const runs = [{ run_id: 9, state: 'running' }, { run_id: 8, state: 'completed' }, { run_id: 5, state: 'failed' }];
  assert.equal(core.runForGroup(runs, 0).run_id, 8);
  assert.equal(core.runForGroup(runs, 1).run_id, 5);
  assert.equal(core.runForGroup(runs, 2), null);
});

test('fun helpers: week key, wrapped card, mascot mood, dictation, images', () => {
  assert.equal(fun.weekKey(new Date(2026, 9, 13)), '2026-W42');
  assert.equal(fun.weekKey(new Date(2027, 0, 1)), '2026-W53');
  const now = new Date(2026, 9, 13);
  assert.equal(fun.wrappedDue({ stats: { turns: 3 }, now }), true);
  assert.equal(fun.wrappedDue({ stats: { turns: 3 }, now, dismissed: '2026-W42' }), false);
  assert.equal(fun.wrappedDue({ stats: { turns: 3 }, now, dismissed: '2026-W41' }), true);
  assert.equal(fun.wrappedDue({ stats: { turns: 0 }, now }), false);
  assert.equal(fun.wrappedDue({ stats: { turns: 3 }, now, enabled: false }), false);
  assert.equal(fun.mascotMood({ working: true, waiting: true }), 'waiting');
  assert.equal(fun.mascotMood({ working: true, error: true }), 'working');
  assert.equal(fun.mascotMood({ error: true, done: true }), 'error');
  assert.equal(fun.mascotMood({ done: true }), 'done');
  assert.equal(fun.mascotMood(), 'idle');
  assert.equal(fun.speechLang('ca'), 'ca-ES');
  assert.equal(fun.speechLang('en'), 'en-US');
  assert.equal(fun.dictation('fix', 'the bug', 'in'), 'fix the bug in');
  assert.equal(fun.dictation('fix ', '', 'it'), 'fix it');
  assert.equal(fun.dictation('', ' ', ''), '');
  const files = [{ type: 'image/png', size: 10, name: 'a' }, { type: 'text/plain', size: 1, name: 'b' }, { type: 'image/jpeg', size: 9e6, name: 'big' }, { type: 'image/webp', size: 5, name: 'c' }];
  const imgs = fun.imageFiles(files);
  assert.deepEqual(imgs.map(f => f.name), ['a', 'big', 'c']);
  assert.equal(fun.fmtTokens(1234), '1.2k');
  assert.equal(fun.fmtTokens(2500000), '2.5M');
});

test('wrapped highlights only state known numbers', () => {
  const t = k => ({ 'wrapped.hl.turns': '{n} turns in {s} sessions', 'wrapped.hl.tools': '{n} tools', 'wrapped.hl.files': '{n} files', 'wrapped.hl.green': '{n} green', 'wrapped.hl.stubborn': '{n} fails: {cmd}', 'wrapped.hl.streak': '{n} days', 'wrapped.hl.model': 'fav {m}' })[k] || k;
  assert.deepEqual(fun.wrappedHighlights({ turns: 4, sessions: 2, tool_calls: 0, tests_turned_green: 1, stubborn_bug: { failures: 3, command: 'go test' }, longest_streak_days: 1, favorite_model: '' }, t),
    ['4 turns in 2 sessions', '1 green', '3 fails: go test']);
  assert.deepEqual(fun.wrappedHighlights({}, t), []);
});
