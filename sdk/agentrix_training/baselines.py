"""
Baseline Policies for Agentrix Training and Evaluation.
Includes 1 random baseline and 4 distinct, competitive heuristic styles:
- Random: Baseline noise
- Hunter: Aggressive player chaser and duel specialist
- MobFarmer: XP and leveling specialist
- Survivor: Evasive dodge and placement specialist
- ZoneController: Central area controller and opportunist
"""

from __future__ import annotations

import math
import random
from abc import ABC, abstractmethod
from typing import Any, Dict, List, Optional, Tuple

W = 1200.0
H = 750.0


def distance(p1: List[float], p2: List[float]) -> float:
    return math.hypot(p2[0] - p1[0], p2[1] - p1[1])


def angle_to(p1: List[float], p2: List[float]) -> float:
    return math.atan2(p2[1] - p1[1], p2[0] - p1[0])


def normalize_angle(angle: float) -> float:
    while angle > math.pi:
        angle -= 2.0 * math.pi
    while angle < -math.pi:
        angle += 2.0 * math.pi
    return angle


class BasePolicy(ABC):
    """Abstract base class for bot policies."""

    name: str = "base"

    def reset(self):
        """Reset policy internal state between matches."""
        pass

    @abstractmethod
    def act(self, obs: Dict[str, Any]) -> Dict[str, Any]:
        """Compute action given engine observation. Returns {'angle': float, 'shoot': bool}."""
        pass


class RandomPolicy(BasePolicy):
    """Stochastic baseline policy for sanity checks."""

    name = "random"

    def __init__(self, shoot_prob: float = 0.3):
        self.shoot_prob = shoot_prob

    def act(self, obs: Dict[str, Any]) -> Dict[str, Any]:
        return {
            "angle": random.uniform(-math.pi, math.pi),
            "shoot": random.random() < self.shoot_prob,
        }


class HunterPolicy(BasePolicy):
    """
    Aggressive predator policy.
    Pursues enemy bots relentlessly, engages in tactical strafing in close quarters,
    and shoots continuously.
    """

    name = "hunter"

    def __init__(self, engage_distance: float = 140.0, strafe_angle: float = 0.8):
        self.engage_distance = engage_distance
        self.strafe_angle = strafe_angle
        self.strafe_dir = 1.0

    def reset(self):
        self.strafe_dir = 1.0 if random.random() < 0.5 else -1.0

    def act(self, obs: Dict[str, Any]) -> Dict[str, Any]:
        you = obs.get("you", {})
        if not you.get("alive", True):
            return {"angle": 0.0, "shoot": False}

        my_pos = you.get("pos", [W / 2.0, H / 2.0])
        zone = obs.get("zone", {})
        center = zone.get("center", [W / 2.0, H / 2.0])
        radius = float(zone.get("radius", 600.0))
        dist_to_center = distance(my_pos, center)

        # 1. Zone safety check
        if dist_to_center > radius - 70.0:
            return {
                "angle": angle_to(my_pos, center),
                "shoot": len(obs.get("visible_enemies", [])) > 0,
            }

        enemies = obs.get("visible_enemies", [])
        mobs = obs.get("visible_mobs", [])

        # 2. Priority: Enemy bots
        if enemies:
            # Target weakest or closest enemy
            target = min(enemies, key=lambda e: (e.get("hp", 100.0), distance(my_pos, e["pos"])))
            t_pos = target["pos"]
            d = distance(my_pos, t_pos)
            direct_angle = angle_to(my_pos, t_pos)

            if d > self.engage_distance:
                # Approach directly
                move_angle = direct_angle
            else:
                # Circle/strafe around target to dodge their return fire
                move_angle = direct_angle + (self.strafe_dir * self.strafe_angle)

            return {
                "angle": move_angle,
                "shoot": True,
            }

        # 3. Secondary: Neutral mobs for XP
        if mobs:
            target_mob = min(mobs, key=lambda m: distance(my_pos, m["pos"]))
            return {
                "angle": angle_to(my_pos, target_mob["pos"]),
                "shoot": True,
            }

        # 4. Explore towards center
        return {
            "angle": angle_to(my_pos, center) + 0.2,
            "shoot": False,
        }


class MobFarmerPolicy(BasePolicy):
    """
    Economic / farming specialist.
    Focuses on neutral creeps/mobs to power-level and accumulate score safely.
    Evades enemy bots when at a disadvantage.
    """

    name = "mob_farmer"

    def __init__(self, safe_hp_threshold: float = 70.0, standoff_dist: float = 120.0):
        self.safe_hp_threshold = safe_hp_threshold
        self.standoff_dist = standoff_dist

    def act(self, obs: Dict[str, Any]) -> Dict[str, Any]:
        you = obs.get("you", {})
        if not you.get("alive", True):
            return {"angle": 0.0, "shoot": False}

        my_pos = you.get("pos", [W / 2.0, H / 2.0])
        hp = float(you.get("hp", 100.0))
        max_hp = float(you.get("max_hp", 100.0))
        hp_pct = (hp / max_hp) * 100.0 if max_hp > 0 else 100.0

        zone = obs.get("zone", {})
        center = zone.get("center", [W / 2.0, H / 2.0])
        radius = float(zone.get("radius", 600.0))
        dist_to_center = distance(my_pos, center)

        # 1. Stay in safe zone
        if dist_to_center > radius - 80.0:
            return {
                "angle": angle_to(my_pos, center),
                "shoot": len(obs.get("visible_mobs", [])) > 0 or len(obs.get("visible_enemies", [])) > 0,
            }

        enemies = obs.get("visible_enemies", [])
        mobs = obs.get("visible_mobs", [])

        # 2. Check if threatened by enemy bots
        if enemies:
            closest_enemy = min(enemies, key=lambda e: distance(my_pos, e["pos"]))
            enemy_dist = distance(my_pos, closest_enemy["pos"])

            # If enemy is very close or bot is low HP, retreat away from enemy
            if enemy_dist < 180.0 or hp_pct < self.safe_hp_threshold:
                flee_angle = angle_to(closest_enemy["pos"], my_pos)
                # Keep towards center if fleeing would push out of zone
                future_x = my_pos[0] + math.cos(flee_angle) * 50.0
                future_y = my_pos[1] + math.sin(flee_angle) * 50.0
                if distance([future_x, future_y], center) > radius - 50.0:
                    flee_angle = angle_to(my_pos, center)
                return {
                    "angle": flee_angle,
                    "shoot": enemy_dist < 220.0,
                }

        # 3. Farm mobs
        if mobs:
            target_mob = min(mobs, key=lambda m: distance(my_pos, m["pos"]))
            d = distance(my_pos, target_mob["pos"])
            t_angle = angle_to(my_pos, target_mob["pos"])

            if d > self.standoff_dist:
                move_angle = t_angle
            else:
                # Orbit mob while shooting
                move_angle = t_angle + 1.2

            return {
                "angle": move_angle,
                "shoot": True,
            }

        # 4. If enemies present and no mobs, defend
        if enemies:
            closest_enemy = min(enemies, key=lambda e: distance(my_pos, e["pos"]))
            return {
                "angle": angle_to(my_pos, closest_enemy["pos"]),
                "shoot": True,
            }

        # 5. Move towards center to find more mobs
        return {
            "angle": angle_to(my_pos, center),
            "shoot": False,
        }


class SurvivorPolicy(BasePolicy):
    """
    Longevity and evasion specialist (Dodger).
    Avoids all engagements, sidesteps bullets, hugs safe zone perimeter,
    and outlives opponents to secure high placement.
    """

    name = "survivor"

    def __init__(self, threat_radius: float = 260.0):
        self.threat_radius = threat_radius

    def act(self, obs: Dict[str, Any]) -> Dict[str, Any]:
        you = obs.get("you", {})
        if not you.get("alive", True):
            return {"angle": 0.0, "shoot": False}

        my_pos = you.get("pos", [W / 2.0, H / 2.0])
        zone = obs.get("zone", {})
        center = zone.get("center", [W / 2.0, H / 2.0])
        radius = float(zone.get("radius", 600.0))
        dist_to_center = distance(my_pos, center)

        # 1. Stay strictly inside zone
        if dist_to_center > radius - 60.0:
            return {
                "angle": angle_to(my_pos, center),
                "shoot": False,
            }

        bullets = obs.get("visible_bullets", [])
        enemies = obs.get("visible_enemies", [])

        # 2. Dodge incoming bullets
        dangerous_bullets = [b for b in bullets if distance(my_pos, b["pos"]) < 120.0]
        if dangerous_bullets:
            closest_bullet = min(dangerous_bullets, key=lambda b: distance(my_pos, b["pos"]))
            b_angle = angle_to(closest_bullet["pos"], my_pos)
            # Dodge perpendicularly
            dodge_angle = b_angle + math.pi / 2.0
            return {
                "angle": dodge_angle,
                "shoot": False,
            }

        # 3. Evade visible enemies
        threats = [e for e in enemies if distance(my_pos, e["pos"]) < self.threat_radius]
        if threats:
            closest = min(threats, key=lambda e: distance(my_pos, e["pos"]))
            flee_angle = angle_to(closest["pos"], my_pos)
            # Check if flee direction puts us outside zone
            projected = [
                my_pos[0] + math.cos(flee_angle) * 80.0,
                my_pos[1] + math.sin(flee_angle) * 80.0,
            ]
            if distance(projected, center) > radius - 70.0:
                # Lateral evasion along circle perimeter
                flee_angle = angle_to(my_pos, center) + (math.pi / 2.0)

            # Fire defensively only if very close
            shoot = distance(my_pos, closest["pos"]) < 140.0
            return {
                "angle": flee_angle,
                "shoot": shoot,
            }

        # 4. Safe positioning: maintain a sweet spot (around 50% radius) away from center mayhem
        target_dist = radius * 0.55
        if dist_to_center < target_dist - 40.0:
            # Move slightly outward
            angle = angle_to(center, my_pos) + 0.5
        elif dist_to_center > target_dist + 40.0:
            # Move slightly inward
            angle = angle_to(my_pos, center) + 0.5
        else:
            # Patrol the perimeter
            angle = angle_to(my_pos, center) + (math.pi / 2.0)

        return {
            "angle": angle,
            "shoot": False,
        }


class ZoneControllerPolicy(BasePolicy):
    """
    Strategic zone controller and opportunist.
    Holds advantageous positions near the inner circle, snipes weakened bots and mobs,
    and secures decisive kills.
    """

    name = "zone_controller"

    def __init__(self, patrol_radius_ratio: float = 0.25):
        self.patrol_radius_ratio = patrol_radius_ratio
        self.angle_offset = 0.0

    def reset(self):
        self.angle_offset = random.uniform(0.0, 2.0 * math.pi)

    def act(self, obs: Dict[str, Any]) -> Dict[str, Any]:
        you = obs.get("you", {})
        if not you.get("alive", True):
            return {"angle": 0.0, "shoot": False}

        my_pos = you.get("pos", [W / 2.0, H / 2.0])
        hp = float(you.get("hp", 100.0))
        max_hp = float(you.get("max_hp", 100.0))

        zone = obs.get("zone", {})
        center = zone.get("center", [W / 2.0, H / 2.0])
        radius = float(zone.get("radius", 600.0))
        dist_to_center = distance(my_pos, center)

        # 1. Zone border escape
        if dist_to_center > radius - 75.0:
            return {
                "angle": angle_to(my_pos, center),
                "shoot": len(obs.get("visible_enemies", [])) > 0,
            }

        enemies = obs.get("visible_enemies", [])
        mobs = obs.get("visible_mobs", [])

        # 2. Opportunistic kill execution: hunt low HP enemies in range
        weak_enemies = [e for e in enemies if float(e.get("hp", 100.0)) < 35.0]
        if weak_enemies:
            target = min(weak_enemies, key=lambda e: distance(my_pos, e["pos"]))
            return {
                "angle": angle_to(my_pos, target["pos"]),
                "shoot": True,
            }

        # 3. If under fire by close enemy, engage tactically
        if enemies:
            closest_enemy = min(enemies, key=lambda e: distance(my_pos, e["pos"]))
            d = distance(my_pos, closest_enemy["pos"])
            if d < 180.0:
                t_angle = angle_to(my_pos, closest_enemy["pos"])
                if hp < 40.0:
                    # Retreat towards center
                    return {
                        "angle": angle_to(closest_enemy["pos"], my_pos),
                        "shoot": True,
                    }
                else:
                    return {
                        "angle": t_angle + 0.6,
                        "shoot": True,
                    }

        # 4. Mob pickings for XP while holding zone
        if mobs:
            closest_mob = min(mobs, key=lambda m: distance(my_pos, m["pos"]))
            if distance(my_pos, closest_mob["pos"]) < 200.0:
                return {
                    "angle": angle_to(my_pos, closest_mob["pos"]),
                    "shoot": True,
                }

        # 5. Position inside inner control zone
        ideal_radius = radius * self.patrol_radius_ratio
        if dist_to_center > ideal_radius + 50.0:
            angle = angle_to(my_pos, center)
        elif dist_to_center < ideal_radius - 50.0:
            angle = angle_to(center, my_pos)
        else:
            # Circle patrol
            angle = angle_to(my_pos, center) + (math.pi / 2.0)

        # Shoot if any enemy visible
        shoot = len(enemies) > 0

        return {
            "angle": angle,
            "shoot": shoot,
        }


BASELINES = {
    "random": RandomPolicy,
    "hunter": HunterPolicy,
    "mob_farmer": MobFarmerPolicy,
    "survivor": SurvivorPolicy,
    "zone_controller": ZoneControllerPolicy,
}


def get_baseline(name: str, **kwargs) -> BasePolicy:
    """Instantiate a baseline policy by name."""
    cls = BASELINES.get(name.lower())
    if cls is None:
        raise ValueError(f"Unknown baseline: '{name}'. Available: {list(BASELINES.keys())}")
    return cls(**kwargs)
