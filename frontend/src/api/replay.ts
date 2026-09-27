import type { ReplayData, ReplayTickFrame } from '../types/index.ts';

interface EngineFrame {
  tick: number;
  zone_radius: number;
  players: { id: number; pos: { x: number; y: number }; facing: number; vision?: number; hp: number; max_hp: number; alive: boolean; kills: number }[];
  mobs: { id: number; pos: { x: number; y: number }; hp: number }[];
  bullets: { pos: { x: number; y: number } }[];
  events: { text: string }[];
}

type RawFrame = Record<string, any>;
type PlayerState = { id: number; x: number; y: number; facing: number; hp: number; max_hp: number; vision: number; alive: boolean; kills: number };
type MobState = { id: number; x: number; y: number; hp: number };
type BulletState = { id: number; x: number; y: number };

function requireEntity<T>(map: Map<number, T>, id: number, label: string): T {
  const value = map.get(id);
  if (!value) throw new Error(`Invalid replay ${label} delta for entity ${id}`);
  return value;
}

function expandEntityFrames(rawFrames: RawFrame[]): EngineFrame[] {
  const players = new Map<number, PlayerState>();
  const mobs = new Map<number, MobState>();
  const bullets = new Map<number, BulletState>();
  const frames: EngineFrame[] = [];
  for (const frame of rawFrames) {
    if (frame.keyframe) {
      players.clear(); mobs.clear(); bullets.clear();
      for (const row of frame.keyframe.players ?? []) {
        const [id, x, y, facing, hp, maxHp, vision, alive, kills] = row;
        players.set(id, { id, x: x / 16, y: y / 16, facing: facing / 4096,
          hp: hp / 16, max_hp: maxHp / 16, vision: vision / 16, alive, kills });
      }
      for (const row of frame.keyframe.mobs ?? []) {
        const [id, x, y, hp] = row;
        mobs.set(id, { id, x: x / 16, y: y / 16, hp: hp / 16 });
      }
      for (const row of frame.keyframe.bullets ?? []) {
        const [id, x, y] = row;
        bullets.set(id, { id, x: x / 16, y: y / 16 });
      }
    } else if (frame.delta) {
      for (const [id, dx, dy, da] of frame.delta.players ?? []) {
        const player = requireEntity(players, id, 'player');
        player.x += dx / 16; player.y += dy / 16; player.facing += da / 4096;
      }
      for (const patch of frame.delta.player_updates ?? []) {
        const player = requireEntity(players, patch.id, 'player');
        if (patch.hp !== undefined) player.hp = patch.hp / 16;
        if (patch.max_hp !== undefined) player.max_hp = patch.max_hp / 16;
        if (patch.vision !== undefined) player.vision = patch.vision / 16;
        if (patch.alive !== undefined) player.alive = patch.alive;
        if (patch.kills !== undefined) player.kills = patch.kills;
      }
      for (const id of frame.delta.mobs_removed ?? []) mobs.delete(id);
      for (const [id, dx, dy] of frame.delta.mobs ?? []) {
        const mob = requireEntity(mobs, id, 'mob');
        mob.x += dx / 16; mob.y += dy / 16;
      }
      for (const [id, hp] of frame.delta.mob_updates ?? []) requireEntity(mobs, id, 'mob').hp = hp / 16;
      for (const [id, x, y, hp] of frame.delta.mobs_added ?? []) mobs.set(id, { id, x: x / 16, y: y / 16, hp: hp / 16 });
      for (const id of frame.delta.bullets_removed ?? []) bullets.delete(id);
      for (const [id, dx, dy] of frame.delta.bullets ?? []) {
        const bullet = requireEntity(bullets, id, 'bullet');
        bullet.x += dx / 16; bullet.y += dy / 16;
      }
      for (const [id, x, y] of frame.delta.bullets_added ?? []) bullets.set(id, { id, x: x / 16, y: y / 16 });
    } else {
      throw new Error('Replay entity stream must start with a keyframe');
    }
    frames.push({ tick: frame.tick, zone_radius: frame.zone_radius,
      players: [...players.values()].sort((a, b) => a.id - b.id).map(p => ({ id: p.id, pos: { x: p.x, y: p.y }, facing: p.facing,
        hp: p.hp, max_hp: p.max_hp, vision: p.vision, alive: p.alive, kills: p.kills })),
      mobs: [...mobs.values()].sort((a, b) => a.id - b.id).map(m => ({ id: m.id, pos: { x: m.x, y: m.y }, hp: m.hp })),
      bullets: [...bullets.values()].sort((a, b) => a.id - b.id).map(b => ({ pos: { x: b.x, y: b.y } })),
      events: frame.events ?? [],
    });
  }
  return frames;
}

// Rust records compact entity keyframes/deltas; this adapter reconstructs the
// same canonical render frames used by legacy replays before canvas playback.
export function normalizeReplay(data: ReplayData | (Omit<ReplayData, 'frames'> & { frames: RawFrame[] })): ReplayData {
  if (data.event_format !== undefined && data.event_format !== 'delta-v1') {
    throw new Error('Unsupported replay event format');
  }
  if (data.entity_format !== undefined && data.entity_format !== 'keyframe-delta-v1') {
    throw new Error('Unsupported replay entity format');
  }
  const sourceFrames: RawFrame[] = (data.frames as unknown as RawFrame[] | undefined)
    ?? (Array.isArray(data.ticks) ? data.ticks as unknown as RawFrame[] : []);
  const rawFrames = data.entity_format === 'keyframe-delta-v1' ? expandEntityFrames(sourceFrames) : sourceFrames;
  const engineFormat = data.entity_format === 'keyframe-delta-v1' || rawFrames.some(frame => 'players' in frame);
  let feed: string[] = [];
  const frames: ReplayTickFrame[] = rawFrames.map(frame => {
    if (!('players' in frame)) {
      feed = [...feed, ...(frame.events ?? [])].slice(-6);
      return { ...frame, kill_feed: feed } as ReplayTickFrame;
    }
    const events = frame.events.map((event: { text: string } | string) => typeof event === 'string' ? event : event.text);
    feed = data.event_format === 'delta-v1' ? [...feed, ...events].slice(-6) : events.slice(-6);
    return {
      tick: frame.tick,
      zone_radius: frame.zone_radius,
      units: frame.players.map((player: EngineFrame['players'][number]) => ({ id: player.id, seat: player.id,
        ...player.pos, angle: player.facing, hp: player.hp, max_hp: player.max_hp,
        alive: player.alive, kills: player.kills, vision: player.vision })),
      mobs: frame.mobs.map((mob: EngineFrame['mobs'][number]) => ({ id: mob.id, ...mob.pos, hp: mob.hp, alive: mob.hp > 0 })),
      projectiles: frame.bullets.map((bullet: EngineFrame['bullets'][number]) => ({ ...bullet.pos })),
      events,
      kill_feed: feed,
    };
  });
  return { ...data, frames, arena: data.arena ?? { width: engineFormat ? 1200 : 1000, height: engineFormat ? 750 : 1000, tick_hz: 60 } };
}
