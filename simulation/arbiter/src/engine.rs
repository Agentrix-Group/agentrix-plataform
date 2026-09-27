use crate::config::{Config, ModelConfig};
use crate::geometry::{
    blocked, circle_rect, line_of_sight, segment_circle, segment_wall, steer, Vec2, Wall, DT, H,
    MOB_R, PLAYER_R, W,
};
use crate::model::{Bullet, Event, Mob, Player};
use crate::process::Action;
use crate::ranking;
use serde::Serialize;
use std::io::{self, Write};

const MAX_REPLAY_JSON_BYTES: usize = 128 * 1024 * 1024;
// Keep room for the replay envelope, configuration, walls, models and ranking.
const REPLAY_ENVELOPE_RESERVE: usize = 1024 * 1024;

struct BoundedCounter {
    bytes: usize,
    limit: usize,
}

impl Write for BoundedCounter {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        let Some(total) = self.bytes.checked_add(buf.len()) else {
            return Err(io::Error::new(
                io::ErrorKind::Other,
                "replay byte count overflow",
            ));
        };
        if total > self.limit {
            return Err(io::Error::new(
                io::ErrorKind::Other,
                "replay byte quota exceeded",
            ));
        }
        self.bytes = total;
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

struct Random(u32);
impl Random {
    fn new(seed: u32) -> Self {
        Self(if seed == 0 { 1 } else { seed })
    }
    fn unit(&mut self) -> f32 {
        self.0 ^= self.0 << 13;
        self.0 ^= self.0 >> 17;
        self.0 ^= self.0 << 5;
        self.0 as f32 / u32::MAX as f32
    }
    fn between(&mut self, a: f32, b: f32) -> f32 {
        a + (b - a) * self.unit()
    }
    fn sign(&mut self) -> f32 {
        if self.unit() < 0.5 {
            -1.0
        } else {
            1.0
        }
    }
}

#[derive(Serialize)]
pub struct ReplayFrame {
    pub tick: u32,
    pub time: f32,
    pub zone_radius: f32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub keyframe: Option<ReplayKeyframe>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub delta: Option<ReplayDelta>,
    pub events: Vec<Event>,
}

#[derive(Serialize)]
pub struct ReplayKeyframe {
    pub players: Vec<PlayerKeyframe>,
    pub mobs: Vec<MobKeyframe>,
    pub bullets: Vec<BulletKeyframe>,
}

#[derive(Serialize)]
pub struct ReplayDelta {
    pub players: Vec<PlayerPositionDelta>,
    pub player_updates: Vec<PlayerUpdate>,
    pub mobs: Vec<MobPositionDelta>,
    pub mob_updates: Vec<MobUpdate>,
    pub mobs_added: Vec<MobKeyframe>,
    pub mobs_removed: Vec<u64>,
    pub bullets: Vec<BulletPositionDelta>,
    pub bullets_added: Vec<BulletKeyframe>,
    pub bullets_removed: Vec<u64>,
}

// Tuple structs serialize as compact JSON arrays. Coordinates/attributes use
// fixed-point units (1/16 px or HP, 1/4096 rad) to keep deltas small and stable.
#[derive(Serialize)]
pub struct PlayerKeyframe(
    pub usize,
    pub i32,
    pub i32,
    pub i32,
    pub i32,
    pub i32,
    pub i32,
    pub bool,
    pub u32,
);
#[derive(Serialize)]
pub struct MobKeyframe(pub u64, pub i32, pub i32, pub i32);
#[derive(Serialize)]
pub struct BulletKeyframe(pub u64, pub i32, pub i32);
#[derive(Serialize)]
pub struct PlayerPositionDelta(pub usize, pub i32, pub i32, pub i32);
#[derive(Serialize)]
pub struct MobPositionDelta(pub u64, pub i32, pub i32);
#[derive(Serialize)]
pub struct MobUpdate(pub u64, pub i32);
#[derive(Serialize)]
pub struct BulletPositionDelta(pub u64, pub i32, pub i32);

#[derive(Serialize)]
pub struct PlayerUpdate {
    pub id: usize,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub hp: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_hp: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub vision: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub alive: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kills: Option<u32>,
}

#[derive(Clone, Copy)]
struct PlayerReplayState {
    id: usize,
    x: i32,
    y: i32,
    facing: i32,
    hp: i32,
    max_hp: i32,
    vision: i32,
    alive: bool,
    kills: u32,
}

#[derive(Clone, Copy)]
struct MobReplayState {
    id: u64,
    x: i32,
    y: i32,
    hp: i32,
}
#[derive(Clone, Copy)]
struct BulletReplayState {
    id: u64,
    x: i32,
    y: i32,
}

#[derive(Clone)]
struct ReplayStateSnapshot {
    players: Vec<PlayerReplayState>,
    mobs: Vec<MobReplayState>,
    bullets: Vec<BulletReplayState>,
}

fn fixed(value: f32, scale: f32) -> i32 {
    (value * scale).round() as i32
}

impl ReplayStateSnapshot {
    fn keyframe(&self) -> ReplayKeyframe {
        ReplayKeyframe {
            players: self
                .players
                .iter()
                .map(|p| {
                    PlayerKeyframe(
                        p.id, p.x, p.y, p.facing, p.hp, p.max_hp, p.vision, p.alive, p.kills,
                    )
                })
                .collect(),
            mobs: self
                .mobs
                .iter()
                .map(|m| MobKeyframe(m.id, m.x, m.y, m.hp))
                .collect(),
            bullets: self
                .bullets
                .iter()
                .map(|b| BulletKeyframe(b.id, b.x, b.y))
                .collect(),
        }
    }

    fn delta(&self, previous: &Self) -> ReplayDelta {
        let old_players: std::collections::HashMap<_, _> =
            previous.players.iter().map(|p| (p.id, p)).collect();
        let players = self
            .players
            .iter()
            .filter_map(|p| {
                old_players.get(&p.id).map(|old| {
                    PlayerPositionDelta(p.id, p.x - old.x, p.y - old.y, p.facing - old.facing)
                })
            })
            .collect();
        let player_updates = self
            .players
            .iter()
            .filter_map(|p| {
                let old = old_players.get(&p.id)?;
                let update = PlayerUpdate {
                    id: p.id,
                    hp: (p.hp != old.hp).then_some(p.hp),
                    max_hp: (p.max_hp != old.max_hp).then_some(p.max_hp),
                    vision: (p.vision != old.vision).then_some(p.vision),
                    alive: (p.alive != old.alive).then_some(p.alive),
                    kills: (p.kills != old.kills).then_some(p.kills),
                };
                (update.hp.is_some()
                    || update.max_hp.is_some()
                    || update.vision.is_some()
                    || update.alive.is_some()
                    || update.kills.is_some())
                .then_some(update)
            })
            .collect();

        let old_mobs: std::collections::HashMap<_, _> =
            previous.mobs.iter().map(|m| (m.id, m)).collect();
        let new_mobs: std::collections::HashMap<_, _> =
            self.mobs.iter().map(|m| (m.id, m)).collect();
        let mobs = self
            .mobs
            .iter()
            .filter_map(|m| {
                old_mobs
                    .get(&m.id)
                    .map(|old| MobPositionDelta(m.id, m.x - old.x, m.y - old.y))
            })
            .collect();
        let mob_updates = self
            .mobs
            .iter()
            .filter_map(|m| {
                old_mobs
                    .get(&m.id)
                    .filter(|old| m.hp != old.hp)
                    .map(|_| MobUpdate(m.id, m.hp))
            })
            .collect();
        let mobs_added = self
            .mobs
            .iter()
            .filter(|m| !old_mobs.contains_key(&m.id))
            .map(|m| MobKeyframe(m.id, m.x, m.y, m.hp))
            .collect();
        let mobs_removed = previous
            .mobs
            .iter()
            .filter(|m| !new_mobs.contains_key(&m.id))
            .map(|m| m.id)
            .collect();

        let old_bullets: std::collections::HashMap<_, _> =
            previous.bullets.iter().map(|b| (b.id, b)).collect();
        let new_bullets: std::collections::HashMap<_, _> =
            self.bullets.iter().map(|b| (b.id, b)).collect();
        let bullets = self
            .bullets
            .iter()
            .filter_map(|b| {
                old_bullets
                    .get(&b.id)
                    .map(|old| BulletPositionDelta(b.id, b.x - old.x, b.y - old.y))
            })
            .collect();
        let bullets_added = self
            .bullets
            .iter()
            .filter(|b| !old_bullets.contains_key(&b.id))
            .map(|b| BulletKeyframe(b.id, b.x, b.y))
            .collect();
        let bullets_removed = previous
            .bullets
            .iter()
            .filter(|b| !new_bullets.contains_key(&b.id))
            .map(|b| b.id)
            .collect();

        ReplayDelta {
            players,
            player_updates,
            mobs,
            mob_updates,
            mobs_added,
            mobs_removed,
            bullets,
            bullets_added,
            bullets_removed,
        }
    }
}

pub struct Engine {
    pub cfg: Config,
    pub models: Vec<ModelConfig>,
    pub seed: u32,
    pub tick: u32,
    pub state: String,
    pub zone_radius: f32,
    pub walls: Vec<Wall>,
    pub players: Vec<Player>,
    pub mobs: Vec<Mob>,
    pub bullets: Vec<Bullet>,
    pub events: Vec<Event>,
    pub mob_timer: f32,
    rng: Random,
    next_mob_id: u64,
    next_bullet_id: u64,
    pub replay_frames: Vec<ReplayFrame>,
    pub replay_overflow: bool,
    replay_bytes: usize,
    pending_replay_events: Vec<Event>,
    previous_replay_state: Option<ReplayStateSnapshot>,
}

impl Engine {
    pub fn new(seed: u32, duration: f32, models: Vec<ModelConfig>) -> Self {
        let mut cfg = Config::default();
        cfg.match_rules.duration = duration;
        cfg.sanitize();

        Self::with_config(seed, cfg, models).expect("sanitized internal config")
    }

    pub fn with_config(seed: u32, cfg: Config, models: Vec<ModelConfig>) -> Result<Self, String> {
        cfg.validate()?;

        let mut engine = Self {
            cfg,
            models,
            seed,
            tick: 0,
            state: "ready".into(),
            zone_radius: initial_radius(),
            walls: vec![],
            players: vec![],
            mobs: vec![],
            bullets: vec![],
            events: vec![],
            mob_timer: 0.0,
            rng: Random::new(seed),
            next_mob_id: 0,
            next_bullet_id: 0,
            replay_frames: vec![],
            replay_overflow: false,
            replay_bytes: 0,
            pending_replay_events: vec![],
            previous_replay_state: None,
        };

        engine.reset();
        Ok(engine)
    }

    pub fn reset(&mut self) {
        self.rng = Random::new(self.seed);
        self.tick = 0;
        self.state = "running".into();
        self.zone_radius = initial_radius();
        self.mob_timer = 0.0;
        self.walls.clear();
        self.players.clear();
        self.mobs.clear();
        self.bullets.clear();
        self.events.clear();
        self.replay_frames.clear();
        self.replay_overflow = false;
        self.replay_bytes = 0;
        self.pending_replay_events.clear();
        self.previous_replay_state = None;
        self.next_mob_id = 0;
        self.next_bullet_id = 0;

        let spawns: Vec<Vec2> = (0..5)
            .map(|i| {
                let angle = -std::f32::consts::FRAC_PI_2 + i as f32 * std::f32::consts::TAU / 5.0;
                Vec2::new(
                    W / 2.0 + angle.cos() * W * 0.36,
                    H / 2.0 + angle.sin() * H * 0.36,
                )
            })
            .collect();

        // Generar muros
        for _ in 0..2000 {
            if self.walls.len() >= self.cfg.match_rules.walls {
                break;
            }
            let horizontal = self.rng.unit() < 0.5;
            let length = self.rng.between(80.0, 230.0);
            let thickness = self.rng.between(20.0, 30.0);
            let (w, h) = if horizontal {
                (length, thickness)
            } else {
                (thickness, length)
            };
            let wall = Wall {
                x: self.rng.between(40.0, W - 40.0 - w),
                y: self.rng.between(40.0, H - 40.0 - h),
                w,
                h,
            };
            let overlaps = self.walls.iter().any(|other| {
                !(other.x > wall.x + wall.w + 46.0
                    || other.x + other.w < wall.x - 46.0
                    || other.y > wall.y + wall.h + 46.0
                    || other.y + other.h < wall.y - 46.0)
            });
            if !overlaps
                && spawns.iter().all(|p| !circle_rect(*p, 70.0, &wall))
                && !circle_rect(Vec2::new(W / 2.0, H / 2.0), 60.0, &wall)
            {
                self.walls.push(wall);
            }
        }

        // Posiciones de inicio aleatorias
        let mut slots = [0, 1, 2, 3, 4];
        for i in (1..5).rev() {
            let j = (self.rng.unit() * (i + 1) as f32).floor().min(i as f32) as usize;
            slots.swap(i, j);
        }

        for (i, model) in self.models.iter().take(5).enumerate() {
            let bias = self.rng.sign();
            let cooldown = self.rng.between(0.0, 0.4);
            self.players.push(Player::create(
                i,
                model,
                spawns[slots[i]],
                self.cfg.xp.base,
                bias,
                cooldown,
            ));
        }

        for _ in 0..self.cfg.mobs.count {
            self.spawn_mob();
        }

        let message = format!(
            "Partida iniciada · semilla {} · {} muros · {} mobs",
            self.seed,
            self.walls.len(),
            self.mobs.len()
        );
        self.log(message);
    }

    pub fn time(&self) -> f32 {
        self.tick as f32 * DT
    }

    pub fn disqualify(&mut self, id: usize, reason: &str) {
        if let Some(player) = self.players.get_mut(id) {
            if player.disqualified {
                return;
            }
            player.alive = false;
            player.death_tick = Some(self.tick);
            player.disqualified = true;
            player.disqualification_reason = Some(reason.to_owned());
            let name = player.name.clone();
            self.log(format!("{} fue descalificado: {}.", name, reason));
        }
    }

    pub fn is_ended(&self) -> bool {
        self.state == "ended" || self.time() >= self.cfg.match_rules.duration
    }

    fn log(&mut self, text: String) {
        let event = Event {
            tick: self.tick,
            text,
        };
        self.pending_replay_events.push(event.clone());
        self.events.push(event);
        if self.events.len() > 100 {
            self.events.remove(0);
        }
    }

    fn free_point(&mut self, r: f32, avoid: &[Vec2], gap: f32, zone_margin: f32) -> Vec2 {
        for _ in 0..400 {
            let p = Vec2::new(
                self.rng.between(r + 10.0, W - r - 10.0),
                self.rng.between(r + 10.0, H - r - 10.0),
            );
            if !blocked(p, r + 4.0, &self.walls)
                && (!self.cfg.match_rules.zone
                    || p.distance(Vec2::new(W / 2.0, H / 2.0))
                        <= (self.zone_radius - zone_margin).max(10.0))
                && avoid.iter().all(|q| p.distance(*q) >= gap)
            {
                return p;
            }
        }
        Vec2::new(W / 2.0, H / 2.0)
    }

    fn spawn_mob(&mut self) {
        let avoid: Vec<Vec2> = self
            .players
            .iter()
            .filter(|p| p.alive)
            .map(|p| p.pos)
            .collect();
        let pos = self.free_point(MOB_R, &avoid, 120.0, 40.0);
        let mob = Mob {
            id: self.next_mob_id,
            pos,
            hp: self.cfg.mobs.hp,
            max_hp: self.cfg.mobs.hp,
            facing: self.rng.between(0.0, std::f32::consts::TAU),
            cooldown: 0.0,
            wander_time: 0.0,
            turn_bias: self.rng.sign(),
            target: None,
        };
        self.next_mob_id += 1;
        self.mobs.push(mob);
    }

    pub fn build_observation(&self, i: usize) -> String {
        let p = &self.players[i];
        if !p.alive {
            return serde_json::json!({
                "phase": "TICK",
                "tick": self.tick,
                "time": self.time(),
                "you": { "alive": false }
            })
            .to_string();
        }

        let visible_enemies: Vec<_> = self
            .players
            .iter()
            .enumerate()
            .filter(|(j, q)| {
                *j != i
                    && q.alive
                    && p.pos.distance(q.pos) <= p.vision
                    && line_of_sight(p.pos, q.pos, &self.walls)
            })
            .map(|(j, q)| {
                serde_json::json!({
                    "id": j,
                    "pos": [q.pos.x, q.pos.y],
                    "hp": q.hp,
                    "max_hp": q.max_hp,
                    "level": q.level
                })
            })
            .collect();

        let visible_mobs: Vec<_> = self
            .mobs
            .iter()
            .filter(|m| {
                p.pos.distance(m.pos) <= p.vision && line_of_sight(p.pos, m.pos, &self.walls)
            })
            .map(|m| {
                serde_json::json!({
                    "id": m.id,
                    "pos": [m.pos.x, m.pos.y],
                    "hp": m.hp,
                    "max_hp": m.max_hp
                })
            })
            .collect();

        let visible_bullets: Vec<_> = self
            .bullets
            .iter()
            .filter(|b| p.pos.distance(b.pos) <= p.vision)
            .map(|b| {
                serde_json::json!({
                    "pos": [b.pos.x, b.pos.y],
                    "color": b.color
                })
            })
            .collect();

        serde_json::json!({
            "phase": "TICK",
            "tick": self.tick,
            "time": self.time(),
            "you": {
                "pos": [p.pos.x, p.pos.y],
                "facing": p.facing,
                "hp": p.hp,
                "max_hp": p.max_hp,
                "speed": p.speed,
                "vision": p.vision,
                "damage": p.damage,
                "level": p.level,
                "xp": p.xp,
                "xp_next": p.xp_next,
                "cooldown": p.cooldown
            },
            "visible_enemies": visible_enemies,
            "visible_mobs": visible_mobs,
            "visible_bullets": visible_bullets,
            "zone": {
                "radius": self.zone_radius,
                "center": [W / 2.0, H / 2.0]
            },
            "walls": self.walls
        })
        .to_string()
    }

    pub fn step(&mut self, actions: &[Option<Action>]) {
        if self.replay_overflow {
            return;
        }
        self.tick += 1;
        self.update_zone();

        // 1. Procesar jugadores vivos
        for i in 0..self.players.len() {
            if !self.players[i].alive {
                continue;
            }
            self.players[i].cooldown -= DT;

            {
                // A missing/late action preserves movement along current facing,
                // but must never repeat a previous shot.
                let act = actions.get(i).copied().flatten().unwrap_or(Action {
                    angle: self.players[i].facing,
                    shoot: false,
                });
                let previous = self.players[i].pos;
                let (speed, bias) = (self.players[i].speed, self.players[i].turn_bias);
                let p = &mut self.players[i];
                steer(
                    &mut p.pos,
                    &mut p.facing,
                    act.angle,
                    speed * DT,
                    bias,
                    PLAYER_R,
                    &self.walls,
                );
                p.velocity = Vec2::new((p.pos.x - previous.x) / DT, (p.pos.y - previous.y) / DT);

                if act.shoot && p.cooldown <= 0.0 {
                    let angle = p.facing;
                    let bullet = Bullet {
                        id: self.next_bullet_id,
                        pos: p.pos.moved(angle, PLAYER_R + 3.0),
                        owner: i,
                        damage: p.damage,
                        life: p.vision / 430.0 + 0.3,
                        color: p.color.clone(),
                        vx: angle.cos() * 430.0,
                        vy: angle.sin() * 430.0,
                    };
                    self.next_bullet_id = self.next_bullet_id.wrapping_add(1);
                    self.bullets.push(bullet);
                    self.players[i].cooldown = 0.8;
                }
            }
        }

        // 2. Mobs, separación y proyectiles
        self.move_mobs();
        self.separate();
        self.move_bullets();

        // 3. Daño de zona a jugadores y mobs
        if self.cfg.match_rules.zone {
            for i in 0..self.players.len() {
                if self.players[i].alive
                    && self.players[i].pos.distance(Vec2::new(W / 2.0, H / 2.0)) > self.zone_radius
                {
                    self.hurt_player(i, 8.0 * DT, None, "la zona");
                }
            }
            // Daño de zona a mobs fuera del área segura
            for m in &mut self.mobs {
                if m.pos.distance(Vec2::new(W / 2.0, H / 2.0)) > self.zone_radius {
                    m.hp -= 8.0 * DT;
                }
            }
            self.mobs.retain(|m| m.hp > 0.0);
        }

        // 4. Respawn de mobs
        self.mob_timer += DT;
        if self.mob_timer >= self.cfg.mobs.respawn {
            self.mob_timer = 0.0;
            if self.mobs.len() < self.cfg.mobs.count {
                self.spawn_mob();
            }
        }

        // 6. Verificar condición de victoria o límite de tiempo
        let left = self.players.iter().filter(|p| p.alive).count();
        if left <= 1 || self.time() >= self.cfg.match_rules.duration {
            self.state = "ended".into();
            let rank = ranking::calculate(&self.players, self.cfg.match_rules.mobs_as_kills, true);
            let winner = rank
                .iter()
                .find(|r| !r.disqualified)
                .map(|r| self.players[r.id].name.clone())
                .unwrap_or_else(|| "ninguno".to_owned());
            self.log(format!(
                "Fin por {}. Gana {}.",
                if left <= 1 { "eliminación" } else { "tiempo" },
                winner
            ));
        }
        // Record only events not included in a previous frame, including the
        // terminal event generated above. Results retain their bounded tail.
        let snapshot = ReplayStateSnapshot {
            players: self
                .players
                .iter()
                .map(|p| PlayerReplayState {
                    id: p.id,
                    x: fixed(p.pos.x, 16.0),
                    y: fixed(p.pos.y, 16.0),
                    facing: fixed(p.facing, 4096.0),
                    hp: fixed(p.hp, 16.0),
                    max_hp: fixed(p.max_hp, 16.0),
                    vision: fixed(p.vision, 16.0),
                    alive: p.alive,
                    kills: p.kills,
                })
                .collect(),
            mobs: self
                .mobs
                .iter()
                .map(|m| MobReplayState {
                    id: m.id,
                    x: fixed(m.pos.x, 16.0),
                    y: fixed(m.pos.y, 16.0),
                    hp: fixed(m.hp, 16.0),
                })
                .collect(),
            bullets: self
                .bullets
                .iter()
                .map(|b| BulletReplayState {
                    id: b.id,
                    x: fixed(b.pos.x, 16.0),
                    y: fixed(b.pos.y, 16.0),
                })
                .collect(),
        };
        let is_keyframe = self.previous_replay_state.is_none() || (self.tick - 1) % 60 == 0;
        let (keyframe, delta) = if is_keyframe {
            (Some(snapshot.keyframe()), None)
        } else {
            (
                None,
                Some(
                    snapshot.delta(
                        self.previous_replay_state
                            .as_ref()
                            .expect("prior replay state"),
                    ),
                ),
            )
        };
        let frame = ReplayFrame {
            tick: self.tick,
            time: self.time(),
            zone_radius: self.zone_radius,
            keyframe,
            delta,
            events: std::mem::take(&mut self.pending_replay_events),
        };
        let separator_bytes = usize::from(!self.replay_frames.is_empty());
        let counter = BoundedCounter {
            bytes: 0,
            limit: MAX_REPLAY_JSON_BYTES
                - REPLAY_ENVELOPE_RESERVE
                - self.replay_bytes
                - separator_bytes,
        };
        let mut counter = counter;
        if serde_json::to_writer(&mut counter, &frame).is_err() {
            self.replay_overflow = true;
            self.state = "ended".into();
            return;
        }
        let Some(total) = self
            .replay_bytes
            .checked_add(counter.bytes)
            .and_then(|bytes| bytes.checked_add(separator_bytes))
        else {
            self.replay_overflow = true;
            self.state = "ended".into();
            return;
        };
        self.replay_bytes = total;
        self.replay_frames.push(frame);
        self.previous_replay_state = Some(snapshot);
    }

    fn update_zone(&mut self) {
        if !self.cfg.match_rules.zone {
            return;
        }
        let duration = self.cfg.match_rules.duration;
        let start = duration * 0.3;
        let end = duration * 0.85;
        let k = ((self.time() - start) / (end - start)).clamp(0.0, 1.0);
        self.zone_radius = initial_radius() + (110.0 - initial_radius()) * k;
    }

    fn move_mobs(&mut self) {
        for i in 0..self.mobs.len() {
            let own = self.mobs[i].pos;
            let mut best = None;
            let mut best_distance = self.cfg.mobs.aggro;
            for (j, p) in self.players.iter().enumerate() {
                let d = own.distance(p.pos);
                if p.alive && d < best_distance && line_of_sight(own, p.pos, &self.walls) {
                    best = Some(j);
                    best_distance = d;
                }
            }
            self.mobs[i].target = best;
            self.mobs[i].cooldown -= DT;
            if let Some(j) = best {
                if best_distance > MOB_R + PLAYER_R + 1.0 {
                    let angle = own.toward(self.players[j].pos);
                    let m = &mut self.mobs[i];
                    let bias = m.turn_bias;
                    steer(
                        &mut m.pos,
                        &mut m.facing,
                        angle,
                        self.cfg.mobs.speed * DT,
                        bias,
                        MOB_R,
                        &self.walls,
                    );
                }
                if best_distance <= MOB_R + PLAYER_R + 4.0 && self.mobs[i].cooldown <= 0.0 {
                    self.hurt_player(j, self.cfg.mobs.dmg, None, "un mob");
                    self.mobs[i].cooldown = 0.8;
                }
            } else {
                let m = &mut self.mobs[i];
                m.wander_time -= DT;
                if m.wander_time <= 0.0 {
                    m.facing += self.rng.between(-1.2, 1.2);
                    m.wander_time = self.rng.between(0.8, 2.2);
                }
                let angle = m.facing;
                let bias = m.turn_bias;
                if !steer(
                    &mut m.pos,
                    &mut m.facing,
                    angle,
                    self.cfg.mobs.speed * DT * 0.5,
                    bias,
                    MOB_R,
                    &self.walls,
                ) {
                    m.facing += std::f32::consts::FRAC_PI_2;
                }
            }
        }
    }

    fn separate(&mut self) {
        let count = self.players.len() + self.mobs.len();
        // 2 pasadas de relajación para evitar atascos en esquinas
        for _ in 0..2 {
            for i in 0..count {
                for j in i + 1..count {
                    if (i < self.players.len() && !self.players[i].alive)
                        || (j < self.players.len() && !self.players[j].alive)
                    {
                        continue;
                    }
                    let a = self.entity_pos(i);
                    let b = self.entity_pos(j);
                    let ra = if i < self.players.len() {
                        PLAYER_R
                    } else {
                        MOB_R
                    };
                    let rb = if j < self.players.len() {
                        PLAYER_R
                    } else {
                        MOB_R
                    };
                    let distance = a.distance(b);
                    if distance > 1e-6 && distance < ra + rb {
                        let push = (ra + rb - distance) / 2.0;
                        let nx = (b.x - a.x) / distance;
                        let ny = (b.y - a.y) / distance;
                        let na = Vec2::new(a.x - nx * push, a.y - ny * push);
                        let nb = Vec2::new(b.x + nx * push, b.y + ny * push);
                        if !blocked(na, ra, &self.walls) {
                            self.set_entity_pos(i, na);
                        }
                        if !blocked(nb, rb, &self.walls) {
                            self.set_entity_pos(j, nb);
                        }
                    }
                }
            }
        }
    }

    fn entity_pos(&self, i: usize) -> Vec2 {
        if i < self.players.len() {
            self.players[i].pos
        } else {
            self.mobs[i - self.players.len()].pos
        }
    }

    fn set_entity_pos(&mut self, i: usize, p: Vec2) {
        if i < self.players.len() {
            self.players[i].pos = p;
        } else {
            let c = self.players.len();
            self.mobs[i - c].pos = p;
        }
    }

    fn move_bullets(&mut self) {
        let mut remaining = Vec::new();
        for mut bullet in std::mem::take(&mut self.bullets) {
            let start = bullet.pos;
            let end = Vec2::new(start.x + bullet.vx * DT, start.y + bullet.vy * DT);
            bullet.life -= DT;
            let mut best = (1.1f32, Hit::None);

            for wall in &self.walls {
                if let Some(t) = segment_wall(start, end, wall, 2.0) {
                    if t < best.0 {
                        best = (t, Hit::Wall);
                    }
                }
            }
            for (j, p) in self.players.iter().enumerate() {
                if j == bullet.owner || !p.alive {
                    continue;
                }
                if let Some(t) = segment_circle(start, end, p.pos, PLAYER_R + 2.0) {
                    if t < best.0 {
                        best = (t, Hit::Player(j));
                    }
                }
            }
            for m in &self.mobs {
                if let Some(t) = segment_circle(start, end, m.pos, MOB_R + 2.0) {
                    if t < best.0 {
                        best = (t, Hit::Mob(m.id));
                    }
                }
            }

            match best.1 {
                Hit::Player(j) => {
                    self.hurt_player(j, bullet.damage, Some(bullet.owner), "un disparo")
                }
                Hit::Mob(id) => self.hurt_mob(id, bullet.damage, bullet.owner),
                Hit::Wall => {}
                Hit::None => {
                    bullet.pos = end;
                    if bullet.life > 0.0 && end.x >= 0.0 && end.x <= W && end.y >= 0.0 && end.y <= H
                    {
                        remaining.push(bullet);
                    }
                }
            }
        }
        self.bullets = remaining;
    }

    fn hurt_player(&mut self, id: usize, amount: f32, attacker: Option<usize>, cause: &str) {
        if !self.players[id].alive {
            return;
        }
        {
            let p = &mut self.players[id];
            p.hp -= amount;
            if p.hp > 0.0 {
                return;
            }
            p.hp = 0.0;
            p.alive = false;
            p.death_tick = Some(self.tick);
            p.killed_by = attacker;
        }
        let name = self.players[id].name.clone();
        if let Some(owner) = attacker {
            self.players[owner].kills += 1;
            self.players[owner].kill_times.push(self.tick);
            let killer = self.players[owner].name.clone();
            self.log(format!("{} elimina a {}.", killer, name));
            self.gain_xp(owner, self.cfg.xp.per_player_kill);
        } else {
            self.log(format!("{} cae por {}.", name, cause));
        }
    }

    fn hurt_mob(&mut self, id: u64, amount: f32, owner: usize) {
        let Some(index) = self.mobs.iter().position(|m| m.id == id) else {
            return;
        };
        self.mobs[index].hp -= amount;
        if self.mobs[index].hp <= 0.0 {
            self.mobs.remove(index);
            self.players[owner].mob_kills += 1;
            self.players[owner].mob_kill_times.push(self.tick);
            self.gain_xp(owner, self.cfg.mobs.xp);
        }
    }

    fn gain_xp(&mut self, id: usize, amount: f32) {
        if amount <= 0.0 || !self.players[id].alive {
            return;
        }
        self.players[id].xp += amount;
        while self.players[id].xp >= self.players[id].xp_next && self.players[id].level < 100 {
            let required = self.players[id].xp_next;
            self.players[id].xp -= required;
            self.players[id].level += 1;
            self.players[id].xp_next = (self.cfg.xp.base
                * self.cfg.xp.growth.powi(self.players[id].level as i32 - 1))
            .round()
            .clamp(1.0, 1_000_000_000.0);
            let stat = match self.players[id].strategy.as_str() {
                "agresiva" => "dano",
                "tanque" => "vida",
                "exploradora" => "velocidad",
                _ => "vida",
            };
            self.apply_upgrade(id, stat);
        }
    }

    fn apply_upgrade(&mut self, id: usize, stat: &str) {
        let p = &mut self.players[id];
        match stat {
            "vida" => {
                p.max_hp = (p.max_hp + self.cfg.up.vida).min(2000.0);
                p.hp = (p.hp + self.cfg.up.vida).min(p.max_hp);
            }
            "velocidad" => p.speed = (p.speed + self.cfg.up.velocidad).min(320.0),
            "vision" => p.vision = (p.vision + self.cfg.up.vision).min(800.0),
            "dano" => p.damage = (p.damage + self.cfg.up.dano).min(200.0),
            _ => {}
        }
        p.upgrades += 1;
    }

    pub fn results_json(&self) -> String {
        let ranking = ranking::calculate(&self.players, self.cfg.match_rules.mobs_as_kills, true);
        serde_json::json!({
            "score_version": ranking::SCORE_VERSION,
            "engine_version": env!("CARGO_PKG_VERSION"),
            "rules_version": "agentrix-rules-v1",
            "effective_config": self.cfg,
            "seed": self.seed,
            "ticks": self.tick,
            "duration": self.time(),
            "winner": ranking.iter().find(|r| !r.disqualified).map(|r| self.players[r.id].name.clone()),
            "winner_id": ranking.iter().find(|r| !r.disqualified).map(|r| r.id),
            "ranking": ranking,
            "players": self.players,
            "events": self.events,
        }).to_string()
    }

    pub fn replay_json(&self) -> String {
        serde_json::json!({
            "arena": {"width": W,"height": H,"tick_hz":60},
            "event_format": "delta-v1",
            "entity_format": "keyframe-delta-v1",
            "score_version": ranking::SCORE_VERSION,
            "engine_version": env!("CARGO_PKG_VERSION"),
            "rules_version": "agentrix-rules-v1",
            "effective_config": self.cfg,
            "ticks": self.tick,
            "seed": self.seed,
            "walls": self.walls,
            "models": self.models,
            "config": self.cfg,
            "frames": self.replay_frames,
            "ranking": ranking::calculate(&self.players, self.cfg.match_rules.mobs_as_kills, true)
        })
        .to_string()
    }
}

fn initial_radius() -> f32 {
    W.hypot(H) / 2.0 + 20.0
}
enum Hit {
    None,
    Wall,
    Player(usize),
    Mob(u64),
}

#[cfg(test)]
mod result_tests {
    use super::*;
    use crate::model::ReplayPlayer;

    #[test]
    fn replay_player_size_does_not_grow_with_kill_histories() {
        let mut engine = Engine::new(42, 20.0, crate::config::default_models());
        let baseline = serde_json::to_string(&ReplayPlayer::from(&engine.players[0])).unwrap();
        engine.players[0].kill_times = vec![1; 10000];
        engine.players[0].mob_kill_times = vec![1; 10000];
        let snapshot = serde_json::to_string(&ReplayPlayer::from(&engine.players[0])).unwrap();
        assert_eq!(snapshot, baseline);
        let snapshot: serde_json::Value = serde_json::from_str(&snapshot).unwrap();
        assert!(snapshot.get("kill_times").is_none());
        assert!(snapshot.get("name").is_none());
        assert_eq!(snapshot["vision"], engine.players[0].vision);
        assert_eq!(
            serde_json::to_value(&engine.players[0]).unwrap()["kill_times"]
                .as_array()
                .unwrap()
                .len(),
            10000
        );
    }

    #[test]
    fn maximum_duration_render_replay_stays_within_runner_byte_quota() {
        let mut cfg = Config::default();
        cfg.match_rules.duration = 900.0;
        cfg.match_rules.zone = false;
        cfg.match_rules.walls = 0;
        cfg.mobs.count = 0;
        let mut engine = Engine::with_config(42, cfg, crate::config::default_models()).unwrap();
        while !engine.is_ended() {
            engine.step(&vec![None; 5]);
        }
        assert_eq!(engine.tick, 54000);
        assert_eq!(engine.replay_frames.len(), 54000);
        let first_frame = serde_json::to_value(&engine.replay_frames[0]).unwrap();
        let second_frame = serde_json::to_value(&engine.replay_frames[1]).unwrap();
        let sixty_first_frame = serde_json::to_value(&engine.replay_frames[60]).unwrap();
        assert!(first_frame.get("keyframe").is_some());
        assert!(second_frame.get("delta").is_some());
        assert!(sixty_first_frame.get("keyframe").is_some());
        let replay = engine.replay_json();
        assert!(
            replay.len() <= 128 * 1024 * 1024,
            "replay exceeds runner quota: {}",
            replay.len()
        );
    }

    #[test]
    fn replay_events_are_recorded_once_including_the_terminal_event() {
        let mut cfg = Config::default();
        cfg.match_rules.duration = 20.0;
        cfg.match_rules.zone = false;
        cfg.match_rules.walls = 0;
        cfg.mobs.count = 0;
        let mut engine = Engine::with_config(42, cfg, crate::config::default_models()).unwrap();
        while !engine.is_ended() {
            engine.log(format!("event {}", engine.tick));
            engine.step(&vec![None; 5]);
        }
        let flattened: Vec<&Event> = engine
            .replay_frames
            .iter()
            .flat_map(|frame| &frame.events)
            .collect();
        assert_eq!(flattened.len(), 1202);
        assert_eq!(
            serde_json::to_value(&flattened[flattened.len() - 100..]).unwrap(),
            serde_json::to_value(&engine.events).unwrap()
        );
        assert!(engine
            .replay_frames
            .last()
            .unwrap()
            .events
            .last()
            .unwrap()
            .text
            .starts_with("Fin por"));
        let replay: serde_json::Value = serde_json::from_str(&engine.replay_json()).unwrap();
        assert_eq!(replay["event_format"], "delta-v1");
        assert_eq!(replay["entity_format"], "keyframe-delta-v1");
    }

    #[test]
    fn zero_seed_is_preserved_and_reset_reproduces_the_same_match() {
        let mut engine = Engine::new(0, 20.0, crate::config::default_models());
        while !engine.is_ended() {
            engine.step(&vec![None; 5]);
        }
        let result = engine.results_json();
        let replay = engine.replay_json();
        let parsed: serde_json::Value = serde_json::from_str(&result).unwrap();
        let parsed_replay: serde_json::Value = serde_json::from_str(&replay).unwrap();
        assert_eq!(engine.seed, 0);
        assert_eq!(parsed["seed"], 0);
        assert_eq!(parsed_replay["seed"], 0);
        engine.reset();
        while !engine.is_ended() {
            engine.step(&vec![None; 5]);
        }
        assert_eq!(engine.results_json(), result);
        assert_eq!(engine.replay_json(), replay);
    }

    #[test]
    fn configured_rules_are_applied_and_recorded_in_both_artifacts() {
        let mut cfg = Config::default();
        cfg.match_rules.duration = 20.0;
        cfg.match_rules.walls = 0;
        cfg.match_rules.zone = false;
        cfg.match_rules.mobs_as_kills = true;
        cfg.mobs.count = 0;
        cfg.up.vida = 50.0;
        let mut engine =
            Engine::with_config(42, cfg.clone(), crate::config::default_models()).unwrap();
        assert!(engine.walls.is_empty() && engine.mobs.is_empty());
        while !engine.is_ended() {
            engine.step(&vec![None; 5]);
        }
        assert_eq!(engine.tick, 1200);
        assert!(engine.players.iter().all(|player| player.alive));
        let result: serde_json::Value = serde_json::from_str(&engine.results_json()).unwrap();
        let replay: serde_json::Value = serde_json::from_str(&engine.replay_json()).unwrap();
        assert_eq!(
            result["effective_config"],
            serde_json::to_value(cfg).unwrap()
        );
        assert_eq!(result["effective_config"], replay["effective_config"]);
        assert_eq!(result["rules_version"], "agentrix-rules-v1");
    }

    #[test]
    fn missing_action_keeps_moving_without_firing() {
        let mut engine = Engine::new(42, 20.0, crate::config::default_models());
        engine.walls.clear();
        engine.mobs.clear();
        engine.players[0].pos = Vec2::new(600.0, 375.0);
        engine.players[0].facing = 0.0;
        engine.players[0].cooldown = 0.0;
        let before = engine.players[0].pos;
        engine.step(&vec![None; 5]);
        assert!(engine.players[0].pos.x > before.x);
        assert_eq!(engine.players[0].facing, 0.0);
        assert!(engine.bullets.is_empty());
    }

    #[test]
    fn result_replay_and_winner_share_the_canonical_ranking() {
        let mut engine = Engine::new(42, 20.0, crate::config::default_models());
        engine.tick = 60;
        for i in 1..5 {
            engine.players[i].alive = false;
            engine.players[i].death_tick = Some((50 - i) as u32);
        }
        engine.players[1].kills = 4;
        engine.players[1].kill_times = vec![1, 2, 3, 4];
        let result: serde_json::Value = serde_json::from_str(&engine.results_json()).unwrap();
        let replay: serde_json::Value = serde_json::from_str(&engine.replay_json()).unwrap();
        assert_eq!(result["ranking"], replay["ranking"]);
        assert_eq!(result["score_version"], replay["score_version"]);
        assert_eq!(result["engine_version"], replay["engine_version"]);
        assert_eq!(result["winner_id"], 1);
        assert_eq!(result["ranking"][0]["place"], 1);
        assert_eq!(result["ranking"][0]["survival_place"], 2);
        assert_eq!(result["winner"], engine.players[1].name);
    }
}
