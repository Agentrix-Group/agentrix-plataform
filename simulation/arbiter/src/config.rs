use serde::{Deserialize, Serialize};

#[derive(Clone, Serialize, Deserialize, Debug)]
#[serde(default)]
pub struct MatchConfig {
    pub duration: f32,
    pub walls: usize,
    pub zone: bool,
    pub show_vision: bool,
    pub mobs_as_kills: bool,
}

impl Default for MatchConfig {
    fn default() -> Self {
        Self {
            duration: 180.0,
            walls: 9,
            zone: true,
            show_vision: true,
            mobs_as_kills: false,
        }
    }
}

#[derive(Clone, Serialize, Deserialize, Debug)]
#[serde(default)]
pub struct MobConfig {
    pub count: usize,
    pub hp: f32,
    pub dmg: f32,
    pub speed: f32,
    pub aggro: f32,
    pub xp: f32,
    pub respawn: f32,
}

impl Default for MobConfig {
    fn default() -> Self {
        Self {
            count: 18,
            hp: 24.0,
            dmg: 6.0,
            speed: 55.0,
            aggro: 140.0,
            xp: 25.0,
            respawn: 2.0,
        }
    }
}

#[derive(Clone, Serialize, Deserialize, Debug)]
#[serde(default)]
pub struct XpConfig {
    pub base: f32,
    pub growth: f32,
    pub per_player_kill: f32,
    pub heal_pct: f32,
}

impl Default for XpConfig {
    fn default() -> Self {
        Self {
            base: 40.0,
            growth: 1.3,
            per_player_kill: 60.0,
            heal_pct: 20.0,
        }
    }
}

#[derive(Clone, Serialize, Deserialize, Debug)]
#[serde(default)]
pub struct UpgradeConfig {
    pub vida: f32,
    pub velocidad: f32,
    pub vision: f32,
    pub dano: f32,
}

impl Default for UpgradeConfig {
    fn default() -> Self {
        Self {
            vida: 25.0,
            velocidad: 10.0,
            vision: 30.0,
            dano: 2.0,
        }
    }
}

#[derive(Clone, Default, Serialize, Deserialize, Debug)]
#[serde(default)]
pub struct Config {
    pub match_rules: MatchConfig,
    pub mobs: MobConfig,
    pub xp: XpConfig,
    pub up: UpgradeConfig,
}

impl Config {
    pub fn sanitize(&mut self) {
        self.match_rules.duration = bound(self.match_rules.duration, 20.0, 900.0, 180.0);
        self.match_rules.walls = self.match_rules.walls.min(30);
        self.mobs.count = self.mobs.count.min(60);
        self.mobs.hp = bound(self.mobs.hp, 1.0, 1000.0, 24.0);
        self.mobs.dmg = bound(self.mobs.dmg, 0.0, 200.0, 6.0);
        self.mobs.speed = bound(self.mobs.speed, 0.0, 300.0, 55.0);
        self.mobs.aggro = bound(self.mobs.aggro, 0.0, 600.0, 140.0);
        self.mobs.xp = bound(self.mobs.xp, 0.0, 1000.0, 25.0);
        self.mobs.respawn = bound(self.mobs.respawn, 0.5, 60.0, 2.0);
        self.xp.base = bound(self.xp.base, 5.0, 2000.0, 40.0);
        self.xp.growth = bound(self.xp.growth, 1.0, 3.0, 1.3);
        self.xp.per_player_kill = bound(self.xp.per_player_kill, 0.0, 2000.0, 60.0);
        self.xp.heal_pct = bound(self.xp.heal_pct, 0.0, 100.0, 20.0);
        self.up.vida = bound(self.up.vida, 0.0, 500.0, 25.0);
        self.up.velocidad = bound(self.up.velocidad, 0.0, 200.0, 10.0);
        self.up.vision = bound(self.up.vision, 0.0, 400.0, 30.0);
        self.up.dano = bound(self.up.dano, 0.0, 100.0, 2.0);
    }
}

pub fn bound(value: f32, min: f32, max: f32, fallback: f32) -> f32 {
    if value.is_finite() {
        value.clamp(min, max)
    } else {
        fallback
    }
}

#[derive(Clone, Serialize, Deserialize, Debug)]
#[serde(default)]
pub struct ModelConfig {
    pub name: String,
    pub color: String,
    pub vida: f32,
    pub velocidad: f32,
    pub vision: f32,
    pub dano: f32,
    pub strategy: String,
}

impl Default for ModelConfig {
    fn default() -> Self {
        Self {
            name: "Modelo".into(),
            color: "#5ec8e5".into(),
            vida: 140.0,
            velocidad: 90.0,
            vision: 190.0,
            dano: 8.0,
            strategy: "equilibrada".into(),
        }
    }
}

pub fn default_models() -> Vec<ModelConfig> {
    let data = [
        ("Atlas", "#5ec8e5", 140., 90., 190., 8., "equilibrada"),
        ("Brasa", "#f07167", 120., 95., 180., 11., "agresiva"),
        ("Coloso", "#9bd66b", 200., 70., 170., 8., "tanque"),
        ("Dron", "#c49bff", 115., 115., 250., 7., "exploradora"),
        ("Eco", "#ff9f5a", 140., 90., 190., 8., "dinamica"),
    ];
    data.into_iter()
        .map(
            |(name, color, vida, velocidad, vision, dano, strategy)| ModelConfig {
                name: name.into(),
                color: color.into(),
                vida,
                velocidad,
                vision,
                dano,
                strategy: strategy.into(),
            },
        )
        .collect()
}
