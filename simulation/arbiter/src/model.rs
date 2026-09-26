use crate::config::ModelConfig;
use crate::geometry::Vec2;
use serde::{Deserialize, Serialize};

#[derive(Clone, Serialize, Deserialize, Debug)]
pub struct Player {
    pub id: usize,
    pub name: String,
    pub color: String,
    pub strategy: String,
    pub pos: Vec2,
    pub facing: f32,
    pub hp: f32,
    pub max_hp: f32,
    pub speed: f32,
    pub vision: f32,
    pub damage: f32,
    pub xp: f32,
    pub level: u32,
    pub xp_next: f32,
    pub upgrades: u32,
    pub kills: u32,
    pub mob_kills: u32,
    pub kill_times: Vec<u32>,
    pub mob_kill_times: Vec<u32>,
    pub alive: bool,
    pub death_tick: Option<u32>,
    pub killed_by: Option<usize>,
    pub disqualified: bool,
    pub disqualification_reason: Option<String>,
    pub cooldown: f32,
    #[serde(skip)]
    pub turn_bias: f32,
    #[serde(skip)]
    pub velocity: Vec2,
    #[serde(skip)]
    pub ready: bool,
}

impl Player {
    pub fn create(
        id: usize,
        model: &ModelConfig,
        pos: Vec2,
        xp_base: f32,
        bias: f32,
        cooldown: f32,
    ) -> Self {
        Self {
            id,
            name: model.name.clone(),
            color: model.color.clone(),
            strategy: model.strategy.clone(),
            pos,
            facing: pos.toward(Vec2::new(600.0, 375.0)),
            hp: model.vida,
            max_hp: model.vida,
            speed: model.velocidad,
            vision: model.vision,
            damage: model.dano,
            xp: 0.0,
            level: 1,
            xp_next: xp_base,
            upgrades: 0,
            kills: 0,
            mob_kills: 0,
            kill_times: vec![],
            mob_kill_times: vec![],
            alive: true,
            death_tick: None,
            killed_by: None,
            disqualified: false,
            disqualification_reason: None,
            cooldown,
            turn_bias: bias,
            velocity: Vec2::default(),
            ready: false,
        }
    }
}

#[derive(Clone, Serialize, Deserialize, Debug)]
pub struct Mob {
    pub id: u64,
    pub pos: Vec2,
    pub hp: f32,
    pub max_hp: f32,
    pub facing: f32,
    #[serde(skip)]
    pub cooldown: f32,
    #[serde(skip)]
    pub wander_time: f32,
    #[serde(skip)]
    pub turn_bias: f32,
    #[serde(skip)]
    pub target: Option<usize>,
}

#[derive(Clone, Serialize, Deserialize, Debug)]
pub struct Bullet {
    pub pos: Vec2,
    pub owner: usize,
    pub damage: f32,
    pub life: f32,
    pub color: String,
    #[serde(skip)]
    pub vx: f32,
    #[serde(skip)]
    pub vy: f32,
}

#[derive(Clone, Serialize, Deserialize, Debug)]
pub struct Rank {
    pub id: usize,
    pub kills: u32,
    pub place: usize,
    pub kill_part: f32,
    pub survival_part: f32,
    pub score: f32,
    pub disqualified: bool,
}

#[derive(Clone, Serialize, Deserialize, Debug)]
pub struct Event {
    pub tick: u32,
    pub text: String,
}
