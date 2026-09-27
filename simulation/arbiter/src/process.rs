use serde::Deserialize;
use std::io::{BufRead, BufReader, Read, Write};
use std::os::unix::process::CommandExt;
use std::process::{Child, Command, Stdio};
use std::sync::mpsc::{sync_channel, Receiver, SyncSender, TryRecvError};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread;
use std::time::{Duration, Instant};

const MAX_BOT_LINE_BYTES: usize = 64 * 1024;
// Includes newlines and discarded stale/invalid responses. At 54,000 ticks this
// permits an average of >600 bytes/action, well above the protocol's tiny JSON.
const MAX_BOT_OUTPUT_BYTES: usize = 32 * 1024 * 1024;
const OUTPUT_LIMIT_MARKER: &str = "__AGENTRIX_OUTPUT_LIMIT__";

pub use crate::model::Action;

#[derive(Deserialize)]
struct ActionResponse {
    pub tick: u32,
    pub angle: Option<f32>,
    pub shoot: Option<bool>,
}

#[derive(Deserialize)]
struct ReadyResponse {
    pub status: Option<String>,
}

pub struct BotHandle {
    pub id: usize,
    child: Child,
    input: SyncSender<String>,
    rx: Receiver<String>,
    pub alive: bool,
    pub consecutive_timeouts: u32,
    pub disqualification_reason: Option<String>,
    terminated: bool,
    output_exceeded: Arc<AtomicBool>,
}

impl BotHandle {
    pub fn spawn(id: usize, cmd_str: &str) -> std::io::Result<Self> {
        Self::spawn_with_output_limit(id, cmd_str, MAX_BOT_OUTPUT_BYTES)
    }

    fn spawn_with_output_limit(
        id: usize,
        cmd_str: &str,
        output_limit: usize,
    ) -> std::io::Result<Self> {
        let mut child = Command::new("sh")
            .arg("-c")
            // The runner constructs and shell-quotes the sandbox command. `exec`
            // makes Bubblewrap the direct child so killing this handle also tears
            // down its PID namespace via --die-with-parent.
            .arg(format!("exec {cmd_str}"))
            .env("PYTHONUNBUFFERED", "1")
            .env_clear()
            .env("PATH", "/usr/local/bin:/usr/bin:/bin")
            // systemd-run requires its user bus location, never DB credentials.
            .env(
                "XDG_RUNTIME_DIR",
                std::env::var("XDG_RUNTIME_DIR").unwrap_or_default(),
            )
            .process_group(0)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()?;

        let mut stdin = child.stdin.take().expect("Failed to open child stdin");
        let stdout = child.stdout.take().expect("Failed to open child stdout");

        // Pipe writes can block forever when a bot stops reading. Keep them off
        // the simulation thread and bound queued observations to one tick.
        let (input, input_rx) = sync_channel::<String>(1);
        thread::Builder::new()
            .name(format!("bot-writer-{id}"))
            .spawn(move || {
                while let Ok(payload) = input_rx.recv() {
                    if writeln!(stdin, "{payload}").is_err() || stdin.flush().is_err() {
                        break;
                    }
                }
            })?;

        // A bounded channel prevents a bot from queueing unbounded future actions.
        let (tx, rx) = sync_channel::<String>(8);
        let output_exceeded = Arc::new(AtomicBool::new(false));
        let reader_exceeded = Arc::clone(&output_exceeded);

        thread::Builder::new()
            .name(format!("bot-reader-{}", id))
            .spawn(move || {
                let mut reader = BufReader::new(stdout);
                let mut remaining = output_limit;
                loop {
                    let mut bytes = Vec::new();
                    let read_limit = (MAX_BOT_LINE_BYTES + 1).min(remaining.saturating_add(1));
                    let mut limited = reader.by_ref().take(read_limit as u64);
                    let n = match limited.read_until(b'\n', &mut bytes) {
                        Ok(n) => n,
                        Err(_) => break,
                    };
                    if n == 0 {
                        break;
                    }
                    if bytes.len() > MAX_BOT_LINE_BYTES || bytes.len() > remaining {
                        reader_exceeded.store(true, Ordering::Release);
                        let _ = tx.send(OUTPUT_LIMIT_MARKER.to_owned());
                        break;
                    }
                    remaining -= bytes.len();
                    let line = match String::from_utf8(bytes) {
                        Ok(line) => line,
                        Err(_) => {
                            reader_exceeded.store(true, Ordering::Release);
                            let _ = tx.send(OUTPUT_LIMIT_MARKER.to_owned());
                            break;
                        }
                    };
                    if tx.send(line).is_err() {
                        break;
                    }
                }
            })?;

        Ok(Self {
            id,
            child,
            input,
            rx,
            alive: true,
            consecutive_timeouts: 0,
            disqualification_reason: None,
            terminated: false,
            output_exceeded,
        })
    }

    pub fn send(&mut self, payload: &str) -> bool {
        if self.output_exceeded.load(Ordering::Acquire) {
            self.disqualify("OUTPUT_LIMIT_EXCEEDED");
        }
        if !self.alive {
            return false;
        }
        if payload.len() > 1024 * 1024 || self.input.try_send(payload.to_owned()).is_err() {
            self.disqualify("BROKEN_PIPE");
            return false;
        }
        true
    }

    pub fn kill(&mut self) {
        if self.terminated {
            return;
        }
        self.terminated = true;
        self.alive = false;
        // Also clean up local shell descendants in protocol tests and SDK runs.
        let _ = Command::new("/bin/kill")
            .args(["-KILL", "--", &format!("-{}", self.child.id())])
            .stderr(Stdio::null())
            .status();
        let _ = self.child.kill();
        let _ = self.child.wait();
    }

    pub fn disqualify(&mut self, reason: &str) {
        if self.disqualification_reason.is_none() {
            self.disqualification_reason = Some(reason.to_owned());
        }
        self.kill();
    }
}

impl Drop for BotHandle {
    fn drop(&mut self) {
        self.kill();
    }
}

pub struct BotManager {
    pub bots: Vec<BotHandle>,
}

impl BotManager {
    pub fn new(commands: &[String]) -> std::io::Result<Self> {
        let mut bots = Vec::new();
        for (i, cmd) in commands.iter().enumerate() {
            match BotHandle::spawn(i, cmd) {
                Ok(bot) => bots.push(bot),
                Err(err) => return Err(err),
            }
        }
        Ok(Self { bots })
    }

    pub fn warmup(&mut self, timeout: Duration) -> Vec<bool> {
        let mut results = vec![false; self.bots.len()];
        let mut pending: Vec<bool> = self.bots.iter().map(|bot| bot.alive).collect();
        let deadline = Instant::now() + timeout;

        // 1. Enviar mensaje INIT a todos
        for bot in &mut self.bots {
            let models = crate::config::default_models();
            let msg = serde_json::json!({
                "phase": "INIT",
                "protocol_version": 1,
                "name": models[bot.id].name,
                "color": models[bot.id].color,
                "seat": bot.id,
                "timeout_warmup_ms": timeout.as_millis(),
                "timeout_tick_ms": 50
            });
            let _ = bot.send(&msg.to_string());
        }

        // 2. Poll all seats in round-robin order. A noisy or silent low seat
        // must not consume the common deadline before later seats are observed.
        while pending.iter().any(|value| *value) && Instant::now() < deadline {
            let mut progress = false;
            for (i, bot) in self.bots.iter_mut().enumerate() {
                if !pending[i] {
                    continue;
                }
                if bot.output_exceeded.load(Ordering::Acquire) {
                    bot.disqualify("OUTPUT_LIMIT_EXCEEDED");
                    pending[i] = false;
                    continue;
                }
                match bot.rx.try_recv() {
                    Ok(line) => {
                        progress = true;
                        if line == OUTPUT_LIMIT_MARKER {
                            bot.disqualify("OUTPUT_LIMIT_EXCEEDED");
                            pending[i] = false;
                            continue;
                        }
                        if let Ok(resp) = serde_json::from_str::<ReadyResponse>(line.trim()) {
                            if resp.status.as_deref() == Some("READY") {
                                results[i] = true;
                                pending[i] = false;
                            }
                        }
                    }
                    Err(TryRecvError::Disconnected) => {
                        pending[i] = false;
                    }
                    Err(TryRecvError::Empty) => {}
                }
            }
            if !progress {
                thread::sleep(Duration::from_millis(1));
            }
        }

        for (i, bot) in self.bots.iter_mut().enumerate() {
            if !results[i] {
                eprintln!("Bot {} falló warmup (timeout o respuesta inválida)", i);
                bot.disqualify("WARMUP_FAILED");
            }
        }

        results
    }

    pub fn step(
        &mut self,
        observations: &[String],
        timeout: Duration,
        expected_tick: u32,
    ) -> Vec<Option<Action>> {
        let n = self.bots.len();
        let mut actions = vec![None; n];
        let mut pending = vec![false; n];

        // 1. Despacho simultáneo a todos los bots vivos
        for (i, bot) in self.bots.iter_mut().enumerate() {
            if bot.alive && i < observations.len() && !observations[i].is_empty() {
                if !bot.send(&observations[i]) {
                    actions[i] = None;
                } else {
                    pending[i] = true;
                }
            }
        }

        let deadline = Instant::now() + timeout;

        // 2. Collect one response per seat fairly until the shared deadline.
        while pending.iter().any(|value| *value) && Instant::now() < deadline {
            let mut progress = false;
            for (i, bot) in self.bots.iter_mut().enumerate() {
                if !pending[i] {
                    continue;
                }
                if bot.output_exceeded.load(Ordering::Acquire) {
                    bot.disqualify("OUTPUT_LIMIT_EXCEEDED");
                    pending[i] = false;
                    continue;
                }
                match bot.rx.try_recv() {
                    Ok(line) => {
                        progress = true;
                        if line == OUTPUT_LIMIT_MARKER {
                            pending[i] = false;
                            bot.disqualify("OUTPUT_LIMIT_EXCEEDED");
                            continue;
                        }
                        if let Ok(act) = serde_json::from_str::<ActionResponse>(line.trim()) {
                            if act.tick < expected_tick {
                                continue;
                            }
                            pending[i] = false;
                            let angle = act.angle.unwrap_or(0.0);
                            if act.tick != expected_tick
                                || !angle.is_finite()
                                || act.angle.is_none()
                                || act.shoot.is_none()
                            {
                                bot.consecutive_timeouts += 1;
                            } else {
                                actions[i] = Some(Action {
                                    angle,
                                    shoot: act.shoot.unwrap_or(false),
                                });
                                bot.consecutive_timeouts = 0;
                            }
                        } else {
                            pending[i] = false;
                            // Respuesta no parseable
                            bot.consecutive_timeouts += 1;
                        }
                    }
                    Err(TryRecvError::Disconnected) => {
                        progress = true;
                        pending[i] = false;
                        bot.consecutive_timeouts += 1;
                    }
                    Err(TryRecvError::Empty) => {}
                }
            }
            if !progress {
                thread::sleep(Duration::from_millis(1));
            }
        }

        for (i, bot) in self.bots.iter_mut().enumerate() {
            if pending[i] {
                bot.consecutive_timeouts += 1;
            }
            if bot.consecutive_timeouts >= 10 {
                eprintln!("Bot {} acumuló 10 timeouts y fue eliminado.", i);
                bot.disqualify("TOO_MANY_INVALID_ACTIONS");
            }
        }

        actions
    }

    pub fn finish(&mut self, ranking: &[crate::model::Rank]) {
        if self.bots.iter().all(|bot| bot.terminated) {
            return;
        }
        for bot in &mut self.bots {
            let result = ranking.iter().find(|rank| rank.id == bot.id);
            let msg = serde_json::json!({
                "phase": "TERMINATE",
                "protocol_version": 1,
                "reason": "MATCH_ENDED",
                "rank": result.map(|rank| rank.place),
                "score": result.map(|rank| rank.score)
            });
            let _ = bot.send(&msg.to_string());
        }
        thread::sleep(Duration::from_millis(200));
        self.terminate();
    }

    pub fn terminate(&mut self) {
        for bot in &mut self.bots {
            bot.kill();
        }
    }

    pub fn disqualification_reason(&self, seat: usize) -> Option<&str> {
        self.bots
            .get(seat)
            .and_then(|bot| bot.disqualification_reason.as_deref())
    }
}

impl Drop for BotManager {
    fn drop(&mut self) {
        self.terminate();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cumulative_output_counts_stale_lines_and_cannot_hide_behind_valid_actions() {
        let script = "import sys,json\nsys.stdin.readline()\nprint('{\"status\":\"READY\"}',flush=True)\nsys.stdin.readline()\nwhile True: print(json.dumps({'tick':0,'angle':0,'shoot':False,'padding':'x'*1024}),flush=True)";
        let command = format!("python3 -u -c '{}'", script.replace('\'', "'\"'\"'"));
        let mut manager = BotManager {
            bots: vec![BotHandle::spawn_with_output_limit(0, &command, 4096).unwrap()],
        };
        assert_eq!(manager.warmup(Duration::from_secs(1)), vec![true]);
        let start = Instant::now();
        // Every line is individually legal JSON and smaller than the line cap,
        // but stale responses must still consume the cumulative byte budget.
        assert!(manager.step(&["{}".to_owned()], Duration::from_secs(1), 1)[0].is_none());
        assert_eq!(
            manager.disqualification_reason(0),
            Some("OUTPUT_LIMIT_EXCEEDED")
        );
        assert!(!manager.bots[0].alive);
        assert!(start.elapsed() < Duration::from_secs(1));
    }

    #[test]
    fn output_budget_accepts_exact_boundary_and_rejects_one_extra_byte() {
        for (extra, exceeded) in [("", false), ("x", true)] {
            let command = format!("printf 'abc\\nxyz\\n{extra}'");
            let mut bot = BotHandle::spawn_with_output_limit(0, &command, 8).unwrap();
            assert_eq!(
                bot.rx.recv_timeout(Duration::from_secs(1)).unwrap(),
                "abc\n"
            );
            assert_eq!(
                bot.rx.recv_timeout(Duration::from_secs(1)).unwrap(),
                "xyz\n"
            );
            let last = bot.rx.recv_timeout(Duration::from_secs(1));
            if exceeded {
                assert_eq!(last.unwrap(), OUTPUT_LIMIT_MARKER);
                assert!(!bot.send("{}"));
                assert_eq!(
                    bot.disqualification_reason.as_deref(),
                    Some("OUTPUT_LIMIT_EXCEEDED")
                );
            } else {
                assert!(last.is_err());
                assert!(!bot.output_exceeded.load(Ordering::Acquire));
            }
        }
    }

    #[test]
    fn production_output_budget_stops_a_sustained_legal_line_flood() {
        let script = "import sys,json\nsys.stdin.readline()\nprint('{\"status\":\"READY\"}',flush=True)\nsys.stdin.readline()\nline=json.dumps({'tick':0,'angle':0,'shoot':False,'padding':'x'*60000})\nwhile True: print(line,flush=True)";
        let command = format!("python3 -u -c '{}'", script.replace('\'', "'\"'\"'"));
        let mut manager = BotManager::new(&[command]).unwrap();
        assert_eq!(manager.warmup(Duration::from_secs(2)), vec![true]);
        assert!(manager.step(&["{}".to_owned()], Duration::from_secs(5), 1)[0].is_none());
        assert_eq!(
            manager.disqualification_reason(0),
            Some("OUTPUT_LIMIT_EXCEEDED")
        );
        assert!(!manager.bots[0].alive);
        // Idempotent cleanup cannot replace the recorded output sanction.
        manager.terminate();
        manager.terminate();
        assert_eq!(
            manager.disqualification_reason(0),
            Some("OUTPUT_LIMIT_EXCEEDED")
        );
    }

    #[test]
    fn eliminated_seat_without_observation_is_not_disqualified() {
        let mut manager = BotManager::new(&["sleep 10".to_owned()]).unwrap();
        for tick in 0..20 {
            assert!(manager.step(&[String::new()], Duration::from_millis(1), tick)[0].is_none());
        }
        assert!(manager.disqualification_reason(0).is_none());
        assert_eq!(manager.bots[0].consecutive_timeouts, 0);
    }

    #[test]
    fn init_metadata_and_final_result_are_delivered_once() {
        let script = "import sys,json\nm=json.loads(sys.stdin.readline())\nassert m['protocol_version']==1 and m['name']=='Atlas' and m['color']=='#5ec8e5'\nprint(json.dumps({'status':'READY'}),flush=True)\nm=json.loads(sys.stdin.readline())\nassert m['phase']=='TERMINATE' and m['protocol_version']==1\nprint(json.dumps({'rank':m['rank'],'score':m['score']}),flush=True)";
        let command = format!("python3 -u -c '{}'", script.replace('\'', "'\"'\"'"));
        let mut manager = BotManager::new(&[command]).unwrap();
        assert_eq!(manager.warmup(Duration::from_millis(1000)), vec![true]);
        let engine = crate::engine::Engine::new(42, 20.0, crate::config::default_models());
        let ranking = crate::ranking::calculate(&engine.players, false, true);
        let expected = ranking.iter().find(|rank| rank.id == 0).unwrap();
        manager.finish(&ranking);
        let response: serde_json::Value =
            serde_json::from_str(&manager.bots[0].rx.try_recv().unwrap()).unwrap();
        assert_eq!(response["rank"], expected.place);
        assert_eq!(response["score"], serde_json::json!(expected.score));
        let start = Instant::now();
        manager.finish(&ranking);
        assert!(start.elapsed() < Duration::from_millis(50));
    }

    #[test]
    fn noisy_warmup_seat_cannot_starve_a_later_ready_seat() {
        let commands = vec![
            "sh -c 'read init; while :; do printf \"invalid\\n\"; done'".to_owned(),
            "sh -c 'read init; printf \"{\\\"status\\\":\\\"READY\\\"}\\n\"; while read line; do :; done'".to_owned(),
        ];
        let mut manager = BotManager::new(&commands).unwrap();

        let ready = manager.warmup(Duration::from_millis(100));

        assert!(!ready[0]);
        assert!(ready[1]);
    }

    #[test]
    fn silent_tick_seat_cannot_starve_a_later_action() {
        let commands = vec![
            "sh -c 'read init; printf \"{\\\"status\\\":\\\"READY\\\"}\\n\"; read tick; sleep 1'".to_owned(),
            "sh -c 'read init; printf \"{\\\"status\\\":\\\"READY\\\"}\\n\"; read tick; printf \"{\\\"tick\\\":0,\\\"angle\\\":1.0,\\\"shoot\\\":false}\\n\"; while read line; do :; done'".to_owned(),
        ];
        let mut manager = BotManager::new(&commands).unwrap();
        let ready = manager.warmup(Duration::from_millis(100));
        assert_eq!(vec![true, true], ready);

        let actions = manager.step(
            &[
                "{\"phase\":\"TICK\"}".to_owned(),
                "{\"phase\":\"TICK\"}".to_owned(),
            ],
            Duration::from_millis(30),
            0,
        );

        assert!(actions[0].is_none());
        assert!(actions[1].is_some());
    }

    #[test]
    fn stale_action_is_discarded_before_current_action() {
        let commands = vec!["sh -c 'read init; printf \"{\\\"status\\\":\\\"READY\\\"}\\n\"; read tick; printf \"{\\\"tick\\\":0,\\\"angle\\\":1,\\\"shoot\\\":false}\\n{\\\"tick\\\":1,\\\"angle\\\":2,\\\"shoot\\\":true}\\n\"; while read line; do :; done'".to_owned()];
        let mut manager = BotManager::new(&commands).unwrap();
        assert_eq!(manager.warmup(Duration::from_millis(200)), vec![true]);
        let actions = manager.step(&["{}".to_owned()], Duration::from_millis(100), 1);
        assert_eq!(actions[0].unwrap().angle, 2.0);
    }

    #[test]
    fn blocked_input_does_not_block_the_simulation() {
        let commands = vec!["sleep 10".to_owned()];
        let mut manager = BotManager::new(&commands).unwrap();
        let start = Instant::now();
        let _ = manager.step(&["x".repeat(128 * 1024)], Duration::from_millis(20), 0);
        assert!(start.elapsed() < Duration::from_millis(200));
    }
}
