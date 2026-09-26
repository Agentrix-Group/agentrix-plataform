use serde::Deserialize;
use std::io::{BufRead, BufReader, Read, Write};
use std::process::{Child, ChildStdin, Command, Stdio};
use std::sync::mpsc::{sync_channel, Receiver, TryRecvError};
use std::thread;
use std::time::{Duration, Instant};

const MAX_BOT_LINE_BYTES: usize = 64 * 1024;
const OUTPUT_LIMIT_MARKER: &str = "__AGENTRIX_OUTPUT_LIMIT__";

#[derive(Clone, Copy, Debug, Default)]
pub struct Action {
    pub angle: f32,
    pub shoot: bool,
}

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
    stdin: ChildStdin,
    rx: Receiver<String>,
    pub alive: bool,
    pub consecutive_timeouts: u32,
    pub disqualification_reason: Option<String>,
}

impl BotHandle {
    pub fn spawn(id: usize, cmd_str: &str) -> std::io::Result<Self> {
        let mut child = Command::new("sh")
            .arg("-c")
            // The runner constructs and shell-quotes the sandbox command. `exec`
            // makes Bubblewrap the direct child so killing this handle also tears
            // down its PID namespace via --die-with-parent.
            .arg(format!("exec {cmd_str}"))
            .env("PYTHONUNBUFFERED", "1")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()?;

        let stdin = child.stdin.take().expect("Failed to open child stdin");
        let stdout = child.stdout.take().expect("Failed to open child stdout");

        // A bounded channel prevents a bot from queueing unbounded future actions.
        let (tx, rx) = sync_channel::<String>(8);

        thread::Builder::new()
            .name(format!("bot-reader-{}", id))
            .spawn(move || {
                let mut reader = BufReader::new(stdout);
                loop {
                    let mut bytes = Vec::new();
                    let mut limited = reader.by_ref().take((MAX_BOT_LINE_BYTES + 1) as u64);
                    let n = match limited.read_until(b'\n', &mut bytes) {
                        Ok(n) => n,
                        Err(_) => break,
                    };
                    if n == 0 {
                        break;
                    }
                    if bytes.len() > MAX_BOT_LINE_BYTES {
                        let _ = tx.send(OUTPUT_LIMIT_MARKER.to_owned());
                        break;
                    }
                    let line = match String::from_utf8(bytes) {
                        Ok(line) => line,
                        Err(_) => OUTPUT_LIMIT_MARKER.to_owned(),
                    };
                    if tx.send(line).is_err() {
                        break;
                    }
                }
            })?;

        Ok(Self {
            id,
            child,
            stdin,
            rx,
            alive: true,
            consecutive_timeouts: 0,
            disqualification_reason: None,
        })
    }

    pub fn send(&mut self, payload: &str) -> bool {
        if !self.alive {
            return false;
        }
        if writeln!(self.stdin, "{}", payload).is_err() || self.stdin.flush().is_err() {
            self.disqualify("BROKEN_PIPE");
            return false;
        }
        true
    }

    pub fn kill(&mut self) {
        self.alive = false;
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

pub struct BotManager {
    pub bots: Vec<BotHandle>,
}

impl BotManager {
    pub fn new(commands: &[String]) -> Self {
        let mut bots = Vec::new();
        for (i, cmd) in commands.iter().enumerate() {
            match BotHandle::spawn(i, cmd) {
                Ok(bot) => bots.push(bot),
                Err(err) => {
                    eprintln!("Error spawning bot {}: {}", i, err);
                }
            }
        }
        Self { bots }
    }

    pub fn warmup(&mut self, timeout: Duration) -> Vec<bool> {
        let mut results = vec![false; self.bots.len()];
        let mut pending: Vec<bool> = self.bots.iter().map(|bot| bot.alive).collect();
        let deadline = Instant::now() + timeout;

        // 1. Enviar mensaje INIT a todos
        for bot in &mut self.bots {
            let msg = serde_json::json!({
                "phase": "INIT",
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
            if bot.alive && i < observations.len() {
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
                match bot.rx.try_recv() {
                    Ok(line) => {
                        progress = true;
                        pending[i] = false;
                        if line == OUTPUT_LIMIT_MARKER {
                            bot.disqualify("OUTPUT_LIMIT_EXCEEDED");
                            continue;
                        }
                        if let Ok(act) = serde_json::from_str::<ActionResponse>(line.trim()) {
                            let angle = act.angle.unwrap_or(0.0);
                            if act.tick != expected_tick || !angle.is_finite() {
                                bot.consecutive_timeouts += 1;
                            } else {
                                actions[i] = Some(Action {
                                    angle,
                                    shoot: act.shoot.unwrap_or(false),
                                });
                                bot.consecutive_timeouts = 0;
                            }
                        } else {
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

    pub fn terminate(&mut self) {
        for bot in &mut self.bots {
            let msg = serde_json::json!({
                "phase": "TERMINATE",
                "reason": "MATCH_ENDED"
            });
            let _ = bot.send(&msg.to_string());
        }
        thread::sleep(Duration::from_millis(200));
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
    fn noisy_warmup_seat_cannot_starve_a_later_ready_seat() {
        let commands = vec![
            "sh -c 'read init; while :; do printf \"invalid\\n\"; done'".to_owned(),
            "sh -c 'read init; printf \"{\\\"status\\\":\\\"READY\\\"}\\n\"; while read line; do :; done'".to_owned(),
        ];
        let mut manager = BotManager::new(&commands);

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
        let mut manager = BotManager::new(&commands);
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
}
