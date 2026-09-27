import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeReplay } from '../src/api/replay.ts';

test('Rust frames display players, bullets, mobs and preserve official ranking', () => {
  const ranking = [{ id: 2, place: 1, score: 85 }];
  const replay = normalizeReplay({ score_version: 'agentrix-score-v1', ticks: 12, ranking, frames: [{ tick: 12, zone_radius: 500,
    players: [{ id: 2, pos: { x: 1100, y: 700 }, facing: 1, vision: 180, hp: 50, max_hp: 100, alive: true, kills: 3 }],
    mobs: [{ id: 8, pos: { x: 40, y: 60 }, hp: 0 }], bullets: [{ pos: { x: 15, y: 25 } }], events: [{ text: 'Finish' }] }] });
  assert.equal(replay.frames[0].units[0].seat, 2);
  assert.equal(replay.frames[0].units[0].x, 1100);
  assert.equal(replay.frames[0].units[0].angle, 1);
  assert.equal(replay.frames[0].units[0].vision, 180);
  assert.deepEqual(replay.frames[0].projectiles, [{ x: 15, y: 25 }]);
  assert.equal(replay.frames[0].mobs[0].alive, false);
  assert.deepEqual(replay.frames[0].events, ['Finish']);
  assert.equal(replay.arena.width, 1200);
  assert.equal(replay.arena.height, 750);
  assert.deepEqual(replay.ranking, ranking);
});

test('legacy flat frames and explicit arena dimensions remain supported', () => {
  const frame = { tick: 0, units: [] };
  const replay = normalizeReplay({ ticks: [frame], arena: { width: 800, height: 500, tick_hz: 60 } });
  assert.deepEqual(replay.frames, [{...frame,kill_feed:[]}]);
  assert.equal(replay.arena.width, 800);
});

test('kill feed is a frame snapshot so redraw and rewind do not append duplicates', () => {
  const replay = normalizeReplay({ticks:[{tick:0,units:[],events:['first']},
    {tick:1,units:[],events:[]},{tick:2,units:[],events:['second']}]});
  assert.deepEqual(replay.frames.map(frame=>frame.kill_feed),[['first'],['first'],['first','second']]);
  assert.deepEqual(replay.frames[0].kill_feed,['first']);
});

test('delta-v1 engine events accumulate once while legacy cumulative events remain supported', () => {
  const frame = (tick,events)=>({tick,players:[],mobs:[],bullets:[],events:events.map(text=>({text}))});
  const delta = normalizeReplay({event_format:'delta-v1',frames:[frame(1,['start']),frame(2,[]),frame(3,['finish'])]});
  assert.deepEqual(delta.frames.map(f=>f.kill_feed),[['start'],['start'],['start','finish']]);
  const legacy = normalizeReplay({frames:[frame(1,['start']),frame(2,['start','finish'])]});
  assert.deepEqual(legacy.frames[1].kill_feed,['start','finish']);
  assert.throws(()=>normalizeReplay({event_format:'unknown',frames:[]}),/Unsupported/);
});

test('reconstructs entity keyframes/deltas, additions, removals and periodic seeks', () => {
  const normalized = normalizeReplay({
    entity_format: 'keyframe-delta-v1', event_format: 'delta-v1',
    arena: { width: 1200, height: 750, tick_hz: 60 },
    frames: [
      { tick: 1, zone_radius: 500, events: [], keyframe: {
        players: [[0, 160, 320, 0, 1600, 1600, 1280, true, 0]],
        mobs: [[10, 800, 800, 800]], bullets: [[20, 1600, 1600]],
      } },
      { tick: 2, zone_radius: 499, events: [{ text: 'hit' }], delta: {
        players: [[0, 16, -16, 4096]], player_updates: [{ id: 0, hp: 1440, kills: 1 }],
        mobs: [[10, -16, 0]], mob_updates: [[10, 640]], mobs_added: [[11, 320, 480, 400]], mobs_removed: [],
        bullets: [[20, 32, 0]], bullets_added: [[21, 48, 64]], bullets_removed: [],
      } },
      { tick: 3, zone_radius: 498, events: [], keyframe: {
        players: [[0, 192, 304, 4096, 1440, 1600, 1280, true, 1]],
        mobs: [[11, 320, 480, 400]], bullets: [[21, 48, 64]],
      } },
    ],
  });
  assert.equal(normalized.frames[1].units[0].x, 11);
  assert.equal(normalized.frames[1].units[0].y, 19);
  assert.equal(normalized.frames[1].units[0].hp, 90);
  assert.equal(normalized.frames[1].units[0].angle, 1);
  assert.deepEqual(normalized.frames[1].mobs.map(({ id }) => id), [10, 11]);
  assert.equal(normalized.frames[1].mobs[0].hp, 40);
  assert.deepEqual(normalized.frames[1].projectiles, [{ x: 102, y: 100 }, { x: 3, y: 4 }]);
  assert.deepEqual(normalized.frames[2].mobs.map(({ id }) => id), [11]);
  assert.deepEqual(normalized.frames[2].projectiles, [{ x: 3, y: 4 }]);
  assert.throws(() => normalizeReplay({ entity_format: 'unknown', frames: [] }), /Unsupported replay entity/);
});
