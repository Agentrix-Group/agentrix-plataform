mod config;
mod engine;
mod geometry;
mod model;
mod process;
mod ranking;

use config::default_models;
use engine::Engine;
use process::BotManager;
use std::env;
use std::fs;
use std::time::Duration;

fn print_usage() {
    eprintln!(
        "Uso: agentrix-arbiter [OPCIONES]\n\n\
         Opciones de Bots:\n\
           --b0 <comando>       Comando para lanzar el Bot 0\n\
           --b1 <comando>       Comando para lanzar el Bot 1\n\
           --b2 <comando>       Comando para lanzar el Bot 2\n\
           --b3 <comando>       Comando para lanzar el Bot 3\n\
           --b4 <comando>       Comando para lanzar el Bot 4\n\n\
         Parámetros de Partida:\n\
           --seed <u32>         Semilla aleatoria (defecto: 2026)\n\
           --duration <f32>     Duración máxima en segundos (defecto: 180.0)\n\
           --warmup-ms <u64>    Tiempo de inicialización en ms (defecto: 10000)\n\
           --tick-ms <u64>      Tiempo máximo por tick en ms (defecto: 50)\n\n\
         Archivos de Salida:\n\
           --out-results <ruta> Ruta donde guardar results.json\n\
           --out-replay <ruta>  Ruta donde guardar replay.json\n"
    );
}

fn main() {
    let args: Vec<String> = env::args().collect();
    if args.len() < 2 || args.contains(&"--help".to_string()) || args.contains(&"-h".to_string()) {
        print_usage();
        return;
    }

    let mut bot_cmds: Vec<String> = vec![
        "true".into(),
        "true".into(),
        "true".into(),
        "true".into(),
        "true".into(),
    ];
    let mut seed: u32 = 2026;
    let mut duration: f32 = 180.0;
    let mut warmup_ms: u64 = 10000;
    let mut tick_ms: u64 = 50;
    let mut out_results: Option<String> = None;
    let mut out_replay: Option<String> = None;

    let mut i = 1;
    while i < args.len() {
        match args[i].as_str() {
            "--b0" if i + 1 < args.len() => {
                bot_cmds[0] = args[i + 1].clone();
                i += 2;
            }
            "--b1" if i + 1 < args.len() => {
                bot_cmds[1] = args[i + 1].clone();
                i += 2;
            }
            "--b2" if i + 1 < args.len() => {
                bot_cmds[2] = args[i + 1].clone();
                i += 2;
            }
            "--b3" if i + 1 < args.len() => {
                bot_cmds[3] = args[i + 1].clone();
                i += 2;
            }
            "--b4" if i + 1 < args.len() => {
                bot_cmds[4] = args[i + 1].clone();
                i += 2;
            }
            "--seed" if i + 1 < args.len() => {
                seed = args[i + 1].parse().unwrap_or(2026);
                i += 2;
            }
            "--duration" if i + 1 < args.len() => {
                duration = args[i + 1].parse().unwrap_or(180.0);
                i += 2;
            }
            "--warmup-ms" if i + 1 < args.len() => {
                warmup_ms = args[i + 1].parse().unwrap_or(10000);
                i += 2;
            }
            "--tick-ms" if i + 1 < args.len() => {
                tick_ms = args[i + 1].parse().unwrap_or(50);
                i += 2;
            }
            "--out-results" if i + 1 < args.len() => {
                out_results = Some(args[i + 1].clone());
                i += 2;
            }
            "--out-replay" if i + 1 < args.len() => {
                out_replay = Some(args[i + 1].clone());
                i += 2;
            }
            other => {
                eprintln!("Argumento desconocido: {}", other);
                print_usage();
                std::process::exit(1);
            }
        }
    }

    println!(
        "[Árbitro] Inicializando partida con semilla {} (duración: {} s)",
        seed, duration
    );
    let mut engine = Engine::new(seed, duration, default_models());

    println!("[Árbitro] Lanzando 5 procesos de bots...");
    let mut bot_manager = BotManager::new(&bot_cmds);

    println!("[Árbitro] Fase de Warm-up (hasta {} ms)...", warmup_ms);
    let ready_flags = bot_manager.warmup(Duration::from_millis(warmup_ms));

    for (seat, &ready) in ready_flags.iter().enumerate() {
        if ready {
            println!("  ✔ Asiento {}: READY", seat);
            engine.players[seat].ready = true;
        } else {
            eprintln!("  ✖ Asiento {}: NO RESPONDIÓ (Descalificado)", seat);
            let reason = bot_manager
                .disqualification_reason(seat)
                .unwrap_or("WARMUP_FAILED");
            engine.disqualify(seat, reason);
        }
    }

    let tick_duration = Duration::from_millis(tick_ms);
    println!(
        "[Árbitro] Comenzando simulación por ticks (límite: {} ms/tick)...",
        tick_ms
    );

    let start_time = std::time::Instant::now();
    let mut observations = vec![String::new(); 5];

    while !engine.is_ended() {
        for s in 0..5 {
            observations[s] = engine.build_observation(s);
        }

        let actions = bot_manager.step(&observations, tick_duration, engine.tick);
        for seat in 0..engine.players.len() {
            if let Some(reason) = bot_manager.disqualification_reason(seat) {
                engine.disqualify(seat, reason);
            }
        }
        engine.step(&actions);
    }

    let elapsed = start_time.elapsed();
    println!(
        "[Árbitro] Partida terminada en {} ticks ({:.2} s simulados) en {:.2} s de tiempo real ({:.1}× velocidad real)",
        engine.tick,
        engine.time(),
        elapsed.as_secs_f32(),
        engine.time() / elapsed.as_secs_f32()
    );

    bot_manager.terminate();

    let results_json = engine.results_json();
    if let Some(path) = out_results {
        if let Err(e) = fs::write(&path, &results_json) {
            eprintln!("Error escribiendo results en {}: {}", path, e);
        } else {
            println!("  ✔ Resultados guardados en: {}", path);
        }
    }

    if let Some(path) = out_replay {
        let replay_json = engine.replay_json();
        if let Err(e) = fs::write(&path, &replay_json) {
            eprintln!("Error escribiendo replay en {}: {}", path, e);
        } else {
            println!("  ✔ Replay guardado en: {}", path);
        }
    }

    // Imprimir clasificación final en consola
    println!("\n=== CLASIFICACIÓN FINAL DE LA PARTIDA ===");
    let rank = ranking::calculate(&engine.players, engine.cfg.match_rules.mobs_as_kills, true);
    for (i, r) in rank.iter().enumerate() {
        let p = &engine.players[r.id];
        println!(
            "{}. {} (Asiento {}) - Puntuación: {:.1} | Kills: {} | Puesto: {}º | Estado: {}",
            i + 1,
            p.name,
            r.id,
            r.score,
            r.kills,
            r.place,
            if p.disqualified {
                "Descalificado"
            } else if p.alive {
                "Vivo"
            } else {
                "Eliminado"
            }
        );
    }
}
