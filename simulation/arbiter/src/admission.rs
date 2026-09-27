use crate::{config::default_models, engine::Engine, process::BotManager};
use std::time::Duration;

// Admission uses the actual engine observation builder and process protocol.
// Only the selected candidate is executed; other engine seats are fixture state.
pub fn validate(command: String) -> Result<(), String> {
    let mut engine = Engine::new(2026, 20.0, default_models());
    let mut bots = BotManager::new(&[command]).map_err(|e| e.to_string())?;
    if bots.warmup(Duration::from_secs(10)) != vec![true] {
        return Err("candidate failed READY warmup".into());
    }
    engine.players[0].ready = true;
    for _ in 0..3 {
        let observations = vec![engine.build_observation(0)];
        let actions = bots.step(&observations, Duration::from_millis(50), engine.tick);
        if actions[0].is_none() || bots.disqualification_reason(0).is_some() {
            return Err(format!(
                "candidate failed action deadline or schema at tick {}",
                engine.tick
            ));
        }
        let mut engine_actions = vec![None; 5];
        engine_actions[0] = actions[0].clone();
        engine.step(&engine_actions);
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    fn candidate(body: &str) -> String {
        let script = format!("import sys,json,time\nfor line in sys.stdin:\n m=json.loads(line)\n if m['phase']=='INIT':\n  print(json.dumps({{'status':'READY'}}),flush=True)\n elif m['phase']=='TICK':\n  {}\n", body);
        format!("python3 -u -c '{}'", script.replace('\'', "'\"'\"'"))
    }
    #[test]
    fn actual_observation_and_actions_are_required() {
        assert!(validate(candidate("assert 'you' in m and 'self' not in m; print(json.dumps({'tick':m['tick'],'angle':0,'shoot':False}),flush=True)")).is_ok());
        assert!(validate(candidate(
            "print(json.dumps({'tick':m['tick']}),flush=True)"
        ))
        .is_err());
        assert!(validate(candidate(
            "print(json.dumps({'tick':m['tick']+1,'angle':0,'shoot':False}),flush=True)"
        ))
        .is_err());
    }
    #[test]
    fn slow_action_is_rejected_at_match_deadline() {
        assert!(validate(candidate("time.sleep(.2); print(json.dumps({'tick':m['tick'],'angle':0,'shoot':False}),flush=True)")).is_err());
    }
}
