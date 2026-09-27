use crate::config::{default_models, Config, ModelConfig};
use crate::engine::Engine;
use crate::geometry::{H, W};
use crate::model::Action;
use crate::ranking;
use serde::{Deserialize, Serialize};
use std::io::{self, BufRead, Write};

pub const ENGINE_VERSION: &str = "agentrix-engine-v1";
pub const RULES_VERSION: &str = "agentrix-rules-v1";
pub const PROTOCOL_VERSION: u32 = 1;
pub const OBSERVATION_VERSION: &str = "agentrix-obs-v1";
pub const FEATURE_ENCODER_VERSION: &str = "agentrix-features-v1";
pub const SCORE_VERSION: &str = "agentrix-score-v1";

#[derive(Deserialize, Debug)]
#[serde(deny_unknown_fields)]
pub struct TrainEnvRequest {
    pub command: String,
    pub seed: Option<u32>,
    pub config: Option<Config>,
    pub models: Option<Vec<ModelConfig>>,
    pub record_replay: Option<bool>,
    pub actions: Option<Vec<Option<Action>>>,
}

#[derive(Serialize, Debug)]
pub struct SeatMetrics {
    pub seat: usize,
    pub alive: bool,
    pub hp: f32,
    pub max_hp: f32,
    pub kills: u32,
    pub mob_kills: u32,
    pub level: u32,
    pub xp: f32,
    pub score: f32,
}

pub struct TrainEnvSession {
    pub engine: Engine,
    pub max_duration: f32,
}

impl TrainEnvSession {
    pub fn new(seed: u32, config: Option<Config>, models: Option<Vec<ModelConfig>>, record_replay: bool) -> Result<Self, String> {
        let cfg = config.unwrap_or_default();
        cfg.validate()?;
        let duration = cfg.match_rules.duration;
        let mdl = models.unwrap_or_else(default_models);
        if mdl.len() != 5 {
            return Err("exactly 5 seat models required".into());
        }

        let mut engine = Engine::with_config(seed, cfg, mdl)?;
        engine.record_replay = record_replay;
        Ok(Self {
            engine,
            max_duration: duration,
        })
    }

    pub fn reset(&mut self, seed: u32, record_replay: bool) {
        self.engine.record_replay = record_replay;
        self.engine.reset_with_seed(seed);
    }

    pub fn step(&mut self, actions: &[Option<Action>]) -> Result<serde_json::Value, String> {
        if actions.len() != 5 {
            return Err(format!("expected actions for exactly 5 seats, got {}", actions.len()));
        }

        for (seat, act_opt) in actions.iter().enumerate() {
            if let Some(act) = act_opt {
                if !act.angle.is_finite() {
                    return Err(format!("seat {} provided non-finite angle: {}", seat, act.angle));
                }
            }
        }

        self.engine.step(actions);

        let ended = self.engine.is_ended();
        let truncated = self.engine.time() >= self.max_duration;
        let terminated = ended;

        let ranks = ranking::calculate(
            &self.engine.players,
            self.engine.cfg.match_rules.mobs_as_kills,
            terminated || truncated,
        );

        let mut metrics = Vec::with_capacity(5);
        for i in 0..5 {
            let p = &self.engine.players[i];
            let score = ranks.iter().find(|r| r.id == i).map(|r| r.score).unwrap_or(0.0);
            metrics.push(SeatMetrics {
                seat: i,
                alive: p.alive,
                hp: p.hp,
                max_hp: p.max_hp,
                kills: p.kills,
                mob_kills: p.mob_kills,
                level: p.level,
                xp: p.xp,
                score,
            });
        }

        let mut observations = Vec::with_capacity(5);
        let mut alive = Vec::with_capacity(5);
        for i in 0..5 {
            alive.push(self.engine.players[i].alive);
            let obs_str = self.engine.build_observation(i);
            let obs_json: serde_json::Value = serde_json::from_str(&obs_str)
                .map_err(|e| format!("failed to parse observation for seat {}: {}", i, e))?;
            observations.push(obs_json);
        }

        let mut resp = serde_json::json!({
            "status": "ok",
            "tick": self.engine.tick,
            "time": self.engine.time(),
            "observations": observations,
            "alive": alive,
            "metrics": metrics,
            "terminated": terminated,
            "truncated": truncated,
            "public_info": {
                "zone": {
                    "center": [W / 2.0, H / 2.0],
                    "radius": self.engine.zone_radius,
                },
                "tick": self.engine.tick,
                "time": self.engine.time(),
            }
        });

        if terminated || truncated {
            let final_ranks: Vec<serde_json::Value> = ranks
                .iter()
                .map(|r| {
                    serde_json::json!({
                        "seat": r.id,
                        "place": r.place,
                        "score": r.score,
                        "kills": r.kills,
                        "survival_part": r.survival_part,
                        "kill_part": r.kill_part,
                    })
                })
                .collect();
            resp["ranking"] = serde_json::json!(final_ranks);
        }

        Ok(resp)
    }

    pub fn get_observation_payload(&self) -> Result<serde_json::Value, String> {
        let mut observations = Vec::with_capacity(5);
        let mut alive = Vec::with_capacity(5);
        for i in 0..5 {
            alive.push(self.engine.players[i].alive);
            let obs_str = self.engine.build_observation(i);
            let obs_json: serde_json::Value = serde_json::from_str(&obs_str)
                .map_err(|e| format!("failed to parse observation for seat {}: {}", i, e))?;
            observations.push(obs_json);
        }

        Ok(serde_json::json!({
            "status": "ok",
            "tick": self.engine.tick,
            "time": self.engine.time(),
            "observations": observations,
            "alive": alive,
            "terminated": false,
            "truncated": false,
            "public_info": {
                "zone": {
                    "center": [W / 2.0, H / 2.0],
                    "radius": self.engine.zone_radius,
                },
                "walls": self.engine.walls,
                "tick": self.engine.tick,
                "time": self.engine.time(),
            }
        }))
    }
}

pub fn run_train_env_loop() -> io::Result<()> {
    let stdin = io::stdin();
    let stdout = io::stdout();
    let mut reader = stdin.lock();
    let mut writer = stdout.lock();

    let mut session: Option<TrainEnvSession> = None;
    let mut line_buf = String::new();

    loop {
        line_buf.clear();
        let bytes_read = reader.read_line(&mut line_buf)?;
        if bytes_read == 0 {
            break; // EOF
        }

        let trimmed = line_buf.trim();
        if trimmed.is_empty() {
            continue;
        }

        let req: Result<TrainEnvRequest, _> = serde_json::from_str(trimmed);
        let req = match req {
            Ok(r) => r,
            Err(e) => {
                let err_resp = serde_json::json!({
                    "status": "error",
                    "error": format!("invalid json request: {}", e)
                });
                writeln!(writer, "{}", err_resp)?;
                writer.flush()?;
                continue;
            }
        };

        match req.command.as_str() {
            "version" => {
                let resp = serde_json::json!({
                    "status": "ok",
                    "engine_version": ENGINE_VERSION,
                    "rules_version": RULES_VERSION,
                    "protocol_version": PROTOCOL_VERSION,
                    "observation_version": OBSERVATION_VERSION,
                    "feature_encoder_version": FEATURE_ENCODER_VERSION,
                    "score_version": SCORE_VERSION,
                    "features": ["parallel_5p", "on_demand_replay", "deterministic_reset", "fast_step"]
                });
                writeln!(writer, "{}", resp)?;
                writer.flush()?;
            }
            "reset" => {
                let seed = req.seed.unwrap_or(2026);
                let record_replay = req.record_replay.unwrap_or(false);

                if let Some(ref mut sess) = session {
                    if req.config.is_none() && req.models.is_none() {
                        sess.reset(seed, record_replay);
                        match sess.get_observation_payload() {
                            Ok(payload) => writeln!(writer, "{}", payload)?,
                            Err(e) => writeln!(writer, "{}", serde_json::json!({"status": "error", "error": e}))?,
                        }
                        writer.flush()?;
                        continue;
                    }
                }

                // New session with explicit config / models or initial creation
                match TrainEnvSession::new(seed, req.config, req.models, record_replay) {
                    Ok(sess) => {
                        let payload = sess.get_observation_payload();
                        session = Some(sess);
                        match payload {
                            Ok(p) => writeln!(writer, "{}", p)?,
                            Err(e) => writeln!(writer, "{}", serde_json::json!({"status": "error", "error": e}))?,
                        }
                    }
                    Err(e) => {
                        writeln!(writer, "{}", serde_json::json!({"status": "error", "error": e}))?;
                    }
                }
                writer.flush()?;
            }
            "step" => {
                let sess = match session.as_mut() {
                    Some(s) => s,
                    None => {
                        writeln!(writer, "{}", serde_json::json!({"status": "error", "error": "environment must be reset before step"}))?;
                        writer.flush()?;
                        continue;
                    }
                };

                let actions = match req.actions {
                    Some(a) => a,
                    None => {
                        writeln!(writer, "{}", serde_json::json!({"status": "error", "error": "step requires 'actions' array of 5 seats"}))?;
                        writer.flush()?;
                        continue;
                    }
                };

                match sess.step(&actions) {
                    Ok(resp) => writeln!(writer, "{}", resp)?,
                    Err(e) => writeln!(writer, "{}", serde_json::json!({"status": "error", "error": e}))?,
                }
                writer.flush()?;
            }
            "get_replay" => {
                let sess = match session.as_ref() {
                    Some(s) => s,
                    None => {
                        writeln!(writer, "{}", serde_json::json!({"status": "error", "error": "no active session"}))?;
                        writer.flush()?;
                        continue;
                    }
                };

                if !sess.engine.record_replay {
                    writeln!(writer, "{}", serde_json::json!({"status": "error", "error": "replay recording was not enabled for this episode"}))?;
                } else {
                    let replay_str = sess.engine.replay_json();
                    let replay_val: serde_json::Value = serde_json::from_str(&replay_str)
                        .unwrap_or(serde_json::json!({"raw": replay_str}));
                    writeln!(writer, "{}", serde_json::json!({"status": "ok", "replay": replay_val}))?;
                }
                writer.flush()?;
            }
            "close" => {
                writeln!(writer, "{}", serde_json::json!({"status": "ok", "message": "session closed"}))?;
                writer.flush()?;
                break;
            }
            unknown => {
                writeln!(writer, "{}", serde_json::json!({
                    "status": "error",
                    "error": format!("unknown command '{}'", unknown)
                }))?;
                writer.flush()?;
            }
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_version_constants() {
        assert_eq!(ENGINE_VERSION, "agentrix-engine-v1");
        assert_eq!(RULES_VERSION, "agentrix-rules-v1");
        assert_eq!(PROTOCOL_VERSION, 1);
        assert_eq!(OBSERVATION_VERSION, "agentrix-obs-v1");
        assert_eq!(FEATURE_ENCODER_VERSION, "agentrix-features-v1");
        assert_eq!(SCORE_VERSION, "agentrix-score-v1");
    }

    #[test]
    fn test_session_reset_and_step() {
        let mut sess = TrainEnvSession::new(1234, None, None, false).expect("valid session");
        let initial = sess.get_observation_payload().expect("obs payload");
        assert_eq!(initial["status"], "ok");
        assert_eq!(initial["tick"], 0);
        assert_eq!(initial["observations"].as_array().unwrap().len(), 5);

        let actions = vec![
            Some(Action { angle: 0.0, shoot: false }),
            Some(Action { angle: 1.0, shoot: false }),
            Some(Action { angle: 2.0, shoot: false }),
            Some(Action { angle: 3.0, shoot: false }),
            Some(Action { angle: 4.0, shoot: false }),
        ];

        let step_res = sess.step(&actions).expect("step result");
        assert_eq!(step_res["status"], "ok");
        assert_eq!(step_res["tick"], 1);
        assert_eq!(step_res["metrics"].as_array().unwrap().len(), 5);

        // Deterministic reset with same seed
        sess.reset(1234, false);
        assert_eq!(sess.engine.tick, 0);
        let obs_after_reset = sess.get_observation_payload().expect("reset obs");
        assert_eq!(initial["observations"], obs_after_reset["observations"]);
    }

    #[test]
    fn test_invalid_actions_rejected() {
        let mut sess = TrainEnvSession::new(42, None, None, false).unwrap();
        // Non-finite angle
        let bad_actions = vec![
            Some(Action { angle: f32::NAN, shoot: false }),
            None, None, None, None,
        ];
        let err = sess.step(&bad_actions);
        assert!(err.is_err());
        assert!(err.unwrap_err().contains("non-finite"));

        // Wrong number of seats
        let short_actions = vec![Some(Action { angle: 0.0, shoot: false })];
        let err_short = sess.step(&short_actions);
        assert!(err_short.is_err());
    }

    #[test]
    fn test_many_resets_memory_stability() {
        let mut sess = TrainEnvSession::new(999, None, None, false).unwrap();
        for s in 0..100 {
            sess.reset(s, false);
            assert_eq!(sess.engine.tick, 0);
            assert_eq!(sess.engine.players.len(), 5);
        }
    }
}
