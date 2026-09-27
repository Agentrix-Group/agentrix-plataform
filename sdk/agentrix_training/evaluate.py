"""
Agentrix Match Evaluation CLI and Library.
Evaluates 5 bot policies with seat rotation across multiple seeds.
Computes win rates, average scores, placement distributions, and can export exhibition replays.
"""

from __future__ import annotations

import argparse
import json
import math
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Dict, List, Optional, Sequence, Union

from .baselines import BASELINES, BasePolicy, get_baseline
from .env import ArbiterClient


@dataclass
class PolicyStats:
    name: str
    matches: int = 0
    wins: int = 0
    placements: Dict[int, int] = field(default_factory=lambda: {1: 0, 2: 0, 3: 0, 4: 0, 5: 0})
    total_score: float = 0.0
    total_survival_ticks: int = 0
    total_kills: int = 0
    total_mob_kills: int = 0

    @property
    def win_rate(self) -> float:
        return (self.wins / self.matches * 100.0) if self.matches > 0 else 0.0

    @property
    def avg_score(self) -> float:
        return (self.total_score / self.matches) if self.matches > 0 else 0.0

    @property
    def avg_survival(self) -> float:
        return (self.total_survival_ticks / self.matches) if self.matches > 0 else 0.0

    @property
    def avg_kills(self) -> float:
        return (self.total_kills / self.matches) if self.matches > 0 else 0.0

    @property
    def avg_mob_kills(self) -> float:
        return (self.total_mob_kills / self.matches) if self.matches > 0 else 0.0


def evaluate_agents(
    policies: Sequence[BasePolicy],
    num_seeds: int = 10,
    start_seed: int = 2026,
    rotate_seats: bool = True,
    binary_path: Optional[str] = None,
    save_best_replay: Optional[str] = None,
) -> Dict[str, PolicyStats]:
    """
    Evaluates 5 policies across seeds with seat rotation.

    Args:
        policies: Exactly 5 policy instances to compete.
        num_seeds: Number of matches (seeds) to run.
        start_seed: Starting random seed.
        rotate_seats: If True, cycles seat order every match to eliminate spawn bias.
        binary_path: Path to agentrix-arbiter binary.
        save_best_replay: Optional path to save the match replay with highest score.

    Returns:
        Dictionary mapping policy name to PolicyStats.
    """
    if len(policies) != 5:
        raise ValueError(f"Evaluation requires exactly 5 policies, got {len(policies)}")

    client = ArbiterClient(binary_path)
    stats: Dict[str, PolicyStats] = {p.name: PolicyStats(name=p.name) for p in policies}

    best_score = -1.0
    best_replay_data: Optional[Dict[str, Any]] = None

    try:
        for match_idx in range(num_seeds):
            seed = start_seed + match_idx

            # Seat rotation: cyclic permutation
            offset = (match_idx % 5) if rotate_seats else 0
            match_policies = [policies[(i + offset) % 5] for i in range(5)]

            for p in match_policies:
                p.reset()

            record_this_replay = save_best_replay is not None
            resp = client.reset(seed=seed, record_replay=record_this_replay)
            raw_obs = resp.get("observations", [])

            terminated = False
            truncated = False
            final_resp: Dict[str, Any] = {}

            while not (terminated or truncated):
                # Gather actions from live seats
                actions: List[Optional[Dict[str, Any]]] = [None] * 5
                alive_list = resp.get("alive", [True] * 5)

                for seat in range(5):
                    if alive_list[seat] and seat < len(raw_obs):
                        obs = raw_obs[seat]
                        act = match_policies[seat].act(obs)
                        actions[seat] = {
                            "angle": float(act.get("angle", 0.0)),
                            "shoot": bool(act.get("shoot", False)),
                        }

                resp = client.step(actions)
                raw_obs = resp.get("observations", [])
                terminated = bool(resp.get("terminated", False))
                truncated = bool(resp.get("truncated", False))
                final_resp = resp

            # Match ended: parse ranking
            rankings = final_resp.get("ranking", [])
            metrics = final_resp.get("metrics", [])
            match_max_score = 0.0

            for rank_item in rankings:
                seat = rank_item.get("seat", 0)
                place = rank_item.get("place", 5)
                score = float(rank_item.get("score", 0.0))
                kills = int(rank_item.get("kills", 0))

                policy = match_policies[seat]
                pol_stat = stats[policy.name]
                pol_stat.matches += 1
                pol_stat.placements[place] = pol_stat.placements.get(place, 0) + 1
                if place == 1:
                    pol_stat.wins += 1
                pol_stat.total_score += score
                pol_stat.total_kills += kills

                if score > match_max_score:
                    match_max_score = score

                if seat < len(metrics):
                    m = metrics[seat]
                    pol_stat.total_mob_kills += int(m.get("mob_kills", 0))

            # Record survival ticks from engine tick
            match_ticks = final_resp.get("tick", 0)
            for seat in range(5):
                policy = match_policies[seat]
                # For survival ticks, if seat died earlier, we approximate or take from engine
                stats[policy.name].total_survival_ticks += match_ticks

            if save_best_replay and match_max_score > best_score:
                best_score = match_max_score
                best_replay_data = client.get_replay()

        if save_best_replay and best_replay_data:
            out_path = Path(save_best_replay)
            out_path.parent.mkdir(parents=True, exist_ok=True)
            with open(out_path, "w") as f:
                json.dump(best_replay_data, f)

    finally:
        client.close()

    return stats


def print_stats_table(stats: Dict[str, PolicyStats]):
    """Prints a clean ASCII evaluation summary table."""
    headers = ["Policy", "Matches", "Wins", "Win Rate", "Avg Score", "Avg Kills", "Avg Mobs", "1st/2nd/3rd/4th/5th"]
    rows = []

    for name, s in sorted(stats.items(), key=lambda item: item[1].avg_score, reverse=True):
        dist = f"{s.placements[1]}/{s.placements[2]}/{s.placements[3]}/{s.placements[4]}/{s.placements[5]}"
        rows.append([
            name,
            str(s.matches),
            str(s.wins),
            f"{s.win_rate:.1f}%",
            f"{s.avg_score:.1f}",
            f"{s.avg_kills:.2f}",
            f"{s.avg_mob_kills:.2f}",
            dist,
        ])

    col_widths = [max(len(row[i]) for row in [headers] + rows) for i in range(len(headers))]
    separator = "+-" + "-+-".join("-" * w for w in col_widths) + "-+"

    print(separator)
    header_line = "| " + " | ".join(h.ljust(w) for h, w in zip(headers, col_widths)) + " |"
    print(header_line)
    print(separator)
    for row in rows:
        row_line = "| " + " | ".join(c.ljust(w) for c, w in zip(row, col_widths)) + " |"
        print(row_line)
    print(separator)


def main():
    parser = argparse.ArgumentParser(description="Evaluate 5 bot policies in Agentrix")
    parser.add_argument(
        "--policies",
        default="hunter,mob_farmer,survivor,zone_controller,random",
        help="Comma-separated list of 5 baseline policy names: hunter, mob_farmer, survivor, zone_controller, random",
    )
    parser.add_argument("--seeds", type=int, default=10, help="Number of seeds/matches to run (default: 10)")
    parser.add_argument("--start-seed", type=int, default=2026, help="Initial random seed (default: 2026)")
    parser.add_argument("--save-best-replay", type=str, default=None, help="File path to save replay of best match")
    parser.add_argument("--no-seat-rotation", action="store_true", help="Disable cyclic seat rotation")

    args = parser.parse_args()

    names = [n.strip() for n in args.policies.split(",") if n.strip()]
    if len(names) != 5:
        parser.error(f"Must specify exactly 5 policies, got {len(names)}: {names}")

    policies = [get_baseline(n) for n in names]

    print(f"Running evaluation: {len(names)} policies across {args.seeds} seeds (starting at {args.start_seed})...")
    stats = evaluate_agents(
        policies=policies,
        num_seeds=args.seeds,
        start_seed=args.start_seed,
        rotate_seats=not args.no_seat_rotation,
        save_best_replay=args.save_best_replay,
    )

    print("\n=== Agentrix Tournament Evaluation Results ===")
    print_stats_table(stats)
    if args.save_best_replay:
        print(f"\nBest match replay saved to: {args.save_best_replay}")


if __name__ == "__main__":
    main()
