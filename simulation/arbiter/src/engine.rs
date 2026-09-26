use crate::config::{Config, ModelConfig};
use crate::geometry::{
    blocked, circle_rect, line_of_sight, segment_circle, segment_wall, steer, Vec2, Wall, DT, H,
    MOB_R, PLAYER_R, W,
};
use crate::model::{Bullet, Event, Mob, Player};
use crate::process::Action;
use crate::ranking;
use serde::Serialize;

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
    pub players: Vec<Player>,
    pub mobs: Vec<Mob>,
    pub bullets: Vec<Bullet>,
    pub events: Vec<Event>,
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
    pub replay_frames: Vec<ReplayFrame>,
}

impl Engine {
    pub fn new(seed: u32, duration: f32, models: Vec<ModelConfig>) -> Self {
        let mut cfg = Config::default();
        cfg.match_rules.duration = duration;
        cfg.sanitize();

        let mut engine = Self {
            cfg,
            models,
            seed: if seed == 0 { 1 } else { seed },
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
            replay_frames: vec![],
        };

        engine.reset();
        engine
    }

    pub fn reset(&mut self) {
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
        self.next_mob_id = 0;

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
        self.events.push(Event {
            tick: self.tick,
            text,
        });
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
        self.tick += 1;
        self.update_zone();

        // 1. Procesar jugadores vivos
        for i in 0..self.players.len() {
            if !self.players[i].alive {
                continue;
            }
            self.players[i].cooldown -= DT;

            if let Some(Some(act)) = actions.get(i) {
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
                        pos: p.pos.moved(angle, PLAYER_R + 3.0),
                        owner: i,
                        damage: p.damage,
                        life: p.vision / 430.0 + 0.3,
                        color: p.color.clone(),
                        vx: angle.cos() * 430.0,
                        vy: angle.sin() * 430.0,
                    };
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

        // 5. Grabar fotograma de replay
        self.replay_frames.push(ReplayFrame {
            tick: self.tick,
            time: self.time(),
            zone_radius: self.zone_radius,
            players: self.players.clone(),
            mobs: self.mobs.clone(),
            bullets: self.bullets.clone(),
            events: self.events.clone(),
        });

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
            "seed": self.seed,
            "ticks": self.tick,
            "duration": self.time(),
            "winner": ranking.iter().find(|r| !r.disqualified).map(|r| self.players[r.id].name.clone()),
            "ranking": ranking,
            "players": self.players,
            "events": self.events,
        }).to_string()
    }

    pub fn replay_json(&self) -> String {
        serde_json::json!({
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
