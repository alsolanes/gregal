const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const team = (id, color) => ({
  name: id, abbr: id.slice(0, 3).toUpperCase(), color, color2: '#ffffff',
  players: [['QB', 1], ['RB', 1], ['WR', 3], ['TE', 1], ['OL', 5], ['DL', 4], ['LB', 3], ['CB', 2], ['S', 2], ['K', 1]]
    .flatMap(([pos, n]) => Array.from({ length: n }, (_, i) => ({ id: `${id}-${pos}${i}`, name: `${pos}${i}`, pos, number: 10 + i }))),
});

test('every kind of play builds an animation with sane positions', async () => {
  const { buildPlay, attackDir, fieldX } = await import(pathToFileURL(path.join(__dirname, 'football-view.js')).href);
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  const A = { passer: 'home-QB0', target: 'home-WR1', runner: 'home-RB0', tackler: 'away-LB1', defender: 'away-CB0', kicker: 'home-K0', returner: 'away-WR0' };
  const plays = [
    { kind: 'run', dir: 'left', spot: 30, end: 36, yards: 6 },
    { kind: 'run', dir: 'middle', spot: 95, end: 100, yards: 5, td: true },
    { kind: 'pass', detail: 'short', depth: 6, spot: 40, end: 49, yards: 9, complete: true },
    { kind: 'pass', detail: 'deep', depth: 35, spot: 20, end: 20, yards: 0 },
    { kind: 'pass', detail: 'medium', depth: 14, spot: 50, end: 64, int: true },
    { kind: 'pass', detail: 'screen', depth: -1, spot: 25, end: 31, yards: 6, complete: true },
    { kind: 'sack', spot: 45, end: 38, yards: -7 },
    { kind: 'scramble', spot: 45, end: 52, yards: 7 },
    { kind: 'punt', spot: 30, end: 75, yards: 45 },
    { kind: 'fg', spot: 70, end: 70, good: true },
    { kind: 'fg', spot: 60, end: 60, good: false },
    { kind: 'pat', spot: 98, end: 98, good: true },
    { kind: 'kickoff', spot: 35, end: 30, detail: 'touchback' },
    { kind: 'kickoff', spot: 35, end: 27 },
    { kind: 'kneel', spot: 60, end: 59, yards: -1 },
  ];
  plays.forEach((p, i) => {
    for (const off of [0, 1]) for (const q of [1, 2, 4, 5]) {
      const play = { n: i + 1, q, off, down: 1, dist: 10, actors: A, ...p };
      const anim = buildPlay(play, match);
      assert.ok(anim.dur > 1 && anim.dur < 7, p.kind + ' dur');
      assert.ok(anim.tracks.length >= 11, p.kind + ' players: ' + anim.tracks.length);
      for (const t of [0, .25, .5, .75, 1]) {
        for (const tr of anim.tracks) {
          const pos = tr.path(t);
          assert.ok(Number.isFinite(pos.x) && Number.isFinite(pos.y), `${p.kind} ${tr.p.id} at ${t}`);
          assert.ok(pos.x > -20 && pos.x < 140 && pos.y > -15 && pos.y < 70, `${p.kind} ${tr.p.id} off the field at ${t}: ${pos.x},${pos.y}`);
        }
        const b = anim.ball(t);
        if (b) assert.ok(Number.isFinite(b.x) && Number.isFinite(b.z) && b.z >= 0, `${p.kind} ball at ${t}`);
      }
    }
  });
  // Els equips canvien de camp cada quart; la iarda 0 de l'atac és la seva línia de gol.
  assert.equal(attackDir(0, 1), -attackDir(0, 2));
  assert.equal(attackDir(0, 1), -attackDir(1, 1));
  assert.equal(fieldX(0, 1), 10);
  assert.equal(fieldX(0, -1), 110);
});
