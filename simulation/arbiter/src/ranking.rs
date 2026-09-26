use crate::model::{Player, Rank};

pub fn calculate(players: &[Player], mobs_as_kills: bool, final_result: bool) -> Vec<Rank> {
    let n = players.len();
    if n < 2 {
        return vec![];
    }
    let eligible = players.iter().filter(|p| !p.disqualified).count();
    let alive = players
        .iter()
        .filter(|p| p.alive && !p.disqualified)
        .count();
    let mut alive_ids: Vec<usize> = players
        .iter()
        .enumerate()
        .filter(|(_, p)| p.alive && !p.disqualified)
        .map(|(i, _)| i)
        .collect();

    if final_result {
        alive_ids.sort_by(|a, b| players[*b].hp.total_cmp(&players[*a].hp).then(a.cmp(b)));
    }

    let mut dead_ids: Vec<usize> = players
        .iter()
        .enumerate()
        .filter(|(_, p)| !p.alive && !p.disqualified)
        .map(|(i, _)| i)
        .collect();

    dead_ids.sort_by(|a, b| {
        players[*b]
            .death_tick
            .cmp(&players[*a].death_tick)
            .then(a.cmp(b))
    });

    let mut places = vec![n; n];
    for (i, id) in alive_ids.iter().enumerate() {
        places[*id] = if i > 0 && (players[*id].hp - players[alive_ids[i - 1]].hp).abs() < 1e-6 {
            places[alive_ids[i - 1]]
        } else if final_result {
            i + 1
        } else {
            1
        };
    }

    for (i, id) in dead_ids.iter().enumerate() {
        places[*id] = if i > 0 && players[*id].death_tick == players[dead_ids[i - 1]].death_tick {
            places[dead_ids[i - 1]]
        } else {
            alive + i + 1
        };
    }

    let kills: Vec<u32> = players
        .iter()
        .map(|p| {
            if p.disqualified {
                0
            } else {
                p.kills + if mobs_as_kills { p.mob_kills } else { 0 }
            }
        })
        .collect();
    let max_kills = kills.iter().copied().max().unwrap_or(0);

    let mut result: Vec<Rank> = players
        .iter()
        .enumerate()
        .map(|(i, _)| {
            let kill_part = if max_kills > 0 {
                kills[i] as f32 / max_kills as f32
            } else {
                0.0
            };
            let survival_part = if players[i].disqualified {
                0.0
            } else if eligible > 1 {
                (eligible - places[i]) as f32 / (eligible - 1) as f32
            } else {
                1.0
            };
            Rank {
                id: i,
                kills: kills[i],
                place: places[i],
                kill_part,
                survival_part,
                // This is the single canonical per-match score. Consumers aggregate
                // this persisted value instead of reimplementing the formula.
                score: if players[i].disqualified {
                    0.0
                } else {
                    60.0 * survival_part + 40.0 * kill_part
                },
                disqualified: players[i].disqualified,
            }
        })
        .collect();

    // Precalcular marcas de tiempo de última kill para evitar clones en el comparador
    let reached_times: Vec<u32> = (0..n)
        .map(|id| {
            let p = &players[id];
            let mut times = p.kill_times.clone();
            if mobs_as_kills {
                times.extend(&p.mob_kill_times);
                times.sort_unstable();
            }
            times.last().copied().unwrap_or(u32::MAX)
        })
        .collect();

    result.sort_by(|a, b| {
        a.disqualified.cmp(&b.disqualified).then_with(|| {
            b.score
                .total_cmp(&a.score)
                .then_with(|| reached_times[a.id].cmp(&reached_times[b.id]))
                .then(a.place.cmp(&b.place))
                .then(a.id.cmp(&b.id))
        })
    });

    result
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::default_models;
    use crate::geometry::Vec2;

    fn players() -> Vec<Player> {
        default_models()
            .iter()
            .enumerate()
            .map(|(id, model)| Player::create(id, model, Vec2::default(), 40.0, 1.0, 0.0))
            .collect()
    }

    #[test]
    fn disqualified_players_are_last_and_score_zero() {
        let mut input = players();
        input[0].disqualified = true;
        input[0].alive = false;
        input[0].kills = 99;

        let result = calculate(&input, false, true);

        let disqualified = result.last().unwrap();
        assert_eq!(0, disqualified.id);
        assert_eq!(0.0, disqualified.score);
        assert!(disqualified.disqualified);
    }

    #[test]
    fn canonical_score_weights_survival_before_kills() {
        let mut input = players();
        input[0].kills = 2;
        input[1].alive = false;
        input[1].death_tick = Some(10);
        input[2].alive = false;
        input[2].death_tick = Some(9);
        input[3].alive = false;
        input[3].death_tick = Some(8);
        input[4].alive = false;
        input[4].death_tick = Some(7);

        let result = calculate(&input, false, true);
        let winner = result.iter().find(|rank| rank.id == 0).unwrap();

        assert_eq!(100.0, winner.score);
    }
}
