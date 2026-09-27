"""
Reward wrapper and shaping functions for Agentrix training.
Supports terminal scores, survival incentives, combat feedback, and zone penalties.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

REWARD_VERSION = "agentrix-reward-v1"


@dataclass
class RewardConfig:
    version: str = REWARD_VERSION
    survival_tick_reward: float = 0.02
    bot_kill_reward: float = 10.0
    mob_kill_reward: float = 2.0
    damage_dealt_multiplier: float = 0.05
    damage_taken_multiplier: float = 0.05
    outside_zone_penalty: float = 0.1
    death_penalty: float = 10.0
    terminal_score_scale: float = 0.5
    placement_bonus: Dict[int, float] = field(default_factory=lambda: {
        1: 25.0,
        2: 12.0,
        3: 5.0,
        4: 0.0,
        5: -5.0
    })


class RewardCalculator:
    """Calculates dense shaped rewards and terminal bonuses for all 5 seats."""

    def __init__(self, config: Optional[RewardConfig] = None):
        self.config = config or RewardConfig()
        self.prev_metrics: Dict[int, Dict[str, Any]] = {}

    def reset(self):
        """Resets tracking state between episodes."""
        self.prev_metrics.clear()

    def compute_step_rewards(
        self,
        current_metrics: List[Dict[str, Any]],
        observations: List[Dict[str, Any]],
        terminated: bool,
        truncated: bool,
        ranking: Optional[List[Dict[str, Any]]] = None,
    ) -> List[float]:
        """Calculates step reward for each of the 5 seats."""
        rewards = [0.0] * 5

        for i in range(5):
            curr = current_metrics[i] if i < len(current_metrics) else {}
            obs = observations[i] if i < len(observations) else {}
            prev = self.prev_metrics.get(i)

            alive = curr.get("alive", False)
            hp = float(curr.get("hp", 0.0))
            kills = int(curr.get("kills", 0))
            mob_kills = int(curr.get("mob_kills", 0))

            if prev is not None:
                prev_alive = prev.get("alive", False)
                prev_hp = float(prev.get("hp", 0.0))
                prev_kills = int(prev.get("kills", 0))
                prev_mob_kills = int(prev.get("mob_kills", 0))

                if alive:
                    # Survival reward
                    rewards[i] += self.config.survival_tick_reward

                    # Kills rewards
                    if kills > prev_kills:
                        rewards[i] += (kills - prev_kills) * self.config.bot_kill_reward

                    if mob_kills > prev_mob_kills:
                        rewards[i] += (mob_kills - prev_mob_kills) * self.config.mob_kill_reward

                    # Damage taken penalty
                    if hp < prev_hp:
                        rewards[i] -= (prev_hp - hp) * self.config.damage_taken_multiplier

                    # Outside safe zone penalty
                    zone = obs.get("zone", {})
                    center = zone.get("center", [600.0, 375.0])
                    radius = float(zone.get("radius", 600.0))
                    you = obs.get("you", {})
                    pos = you.get("pos", center)
                    dx = pos[0] - center[0]
                    dy = pos[1] - center[1]
                    dist_to_center = (dx * dx + dy * dy) ** 0.5
                    if dist_to_center > radius:
                        rewards[i] -= self.config.outside_zone_penalty

                elif prev_alive and not alive:
                    # Just died this tick
                    rewards[i] -= self.config.death_penalty

            # Update cache
            self.prev_metrics[i] = {
                "alive": alive,
                "hp": hp,
                "kills": kills,
                "mob_kills": mob_kills,
            }

        # Terminal rewards
        if (terminated or truncated) and ranking is not None:
            for rank_item in ranking:
                seat = rank_item.get("seat", 0)
                place = rank_item.get("place", 5)
                score = float(rank_item.get("score", 0.0))

                if 0 <= seat < 5:
                    bonus = self.config.placement_bonus.get(place, 0.0)
                    rewards[seat] += bonus + (score * self.config.terminal_score_scale)

        return rewards
