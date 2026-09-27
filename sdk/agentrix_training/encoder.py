"""
Feature encoder for Agentrix observations.
Converts JSON observations into deterministic fixed-size NumPy float32 arrays with entity masks.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Any, Dict, List, Optional
import numpy as np

FEATURE_ENCODER_VERSION = "agentrix-features-v1"
W = 1200.0
H = 750.0
MAX_DIAG = math.hypot(W, H)
MAX_ENEMIES = 4
MAX_MOBS = 6
MAX_BULLETS = 4

# Dimensions:
# Self: 8
# Zone: 5
# Enemies: 4 * 6 = 24
# Mobs: 6 * 5 = 30
# Bullets: 4 * 4 = 16
# Total: 83
FEATURE_DIM = 8 + 5 + (MAX_ENEMIES * 6) + (MAX_MOBS * 5) + (MAX_BULLETS * 4)


@dataclass
class ObservationFeatures:
    raw: np.ndarray
    self_pos: tuple[float, float]
    self_alive: bool
    nearest_enemy_dist: Optional[float]
    nearest_mob_dist: Optional[float]
    nearest_bullet_dist: Optional[float]
    zone_dist: float
    outside_zone: bool


class FeatureEncoder:
    """Deterministic feature encoder for Agentrix game observations."""

    version = FEATURE_ENCODER_VERSION
    dim = FEATURE_DIM

    def __init__(self):
        pass

    @property
    def feature_dim(self) -> int:
        return self.dim

    def encode(self, obs: Dict[str, Any]) -> np.ndarray:
        """Encodes an engine observation JSON into a 1D float32 numpy array of length FEATURE_DIM."""
        features = np.zeros(self.dim, dtype=np.float32)
        offset = 0

        you = obs.get("you", {})
        if not you.get("alive", True):
            # Dead player observation: all zeros except alive=0
            return features

        my_pos = you.get("pos", [W / 2.0, H / 2.0])
        px, py = float(my_pos[0]), float(my_pos[1])
        hp = float(you.get("hp", 100.0))
        max_hp = max(1.0, float(you.get("max_hp", 100.0)))
        speed = float(you.get("speed", 90.0))
        facing = float(you.get("facing", 0.0))
        cooldown = float(you.get("cooldown", 0.0))
        level = float(you.get("level", 1))

        # 1. Self features (8)
        features[offset + 0] = px / W
        features[offset + 1] = py / H
        features[offset + 2] = np.clip(hp / max_hp, 0.0, 1.0)
        features[offset + 3] = np.clip(speed / 320.0, 0.0, 2.0)
        features[offset + 4] = (facing % (2.0 * math.pi)) / (2.0 * math.pi)
        features[offset + 5] = np.clip(cooldown / 0.5, 0.0, 1.0)
        features[offset + 6] = np.clip(level / 10.0, 0.0, 1.0)
        features[offset + 7] = 1.0  # alive
        offset += 8

        # 2. Zone features (5)
        zone = obs.get("zone", {})
        center = zone.get("center", [W / 2.0, H / 2.0])
        cx, cy = float(center[0]), float(center[1])
        radius = float(zone.get("radius", 600.0))

        dist_to_center = math.hypot(cx - px, cy - py)
        features[offset + 0] = (cx - px) / W
        features[offset + 1] = (cy - py) / H
        features[offset + 2] = dist_to_center / MAX_DIAG
        features[offset + 3] = radius / H
        features[offset + 4] = 1.0 if dist_to_center > radius else 0.0
        offset += 5

        # 3. Visible Enemies (4 * 6 = 24)
        enemies = obs.get("visible_enemies", [])
        # Sort by distance
        sorted_enemies = sorted(
            enemies,
            key=lambda e: math.hypot(float(e["pos"][0]) - px, float(e["pos"][1]) - py)
        )[:MAX_ENEMIES]

        for i in range(MAX_ENEMIES):
            if i < len(sorted_enemies):
                e = sorted_enemies[i]
                epos = e.get("pos", [px, py])
                ex, ey = float(epos[0]), float(epos[1])
                edist = math.hypot(ex - px, ey - py)
                ehp = float(e.get("hp", 100.0))
                emax_hp = max(1.0, float(e.get("max_hp", 100.0)))
                elevel = float(e.get("level", 1))

                features[offset + 0] = 1.0  # exists mask
                features[offset + 1] = (ex - px) / W
                features[offset + 2] = (ey - py) / H
                features[offset + 3] = edist / MAX_DIAG
                features[offset + 4] = np.clip(ehp / emax_hp, 0.0, 1.0)
                features[offset + 5] = np.clip(elevel / 10.0, 0.0, 1.0)
            else:
                features[offset + 0] = 0.0
                features[offset + 1] = 0.0
                features[offset + 2] = 0.0
                features[offset + 3] = 1.0
                features[offset + 4] = 0.0
                features[offset + 5] = 0.0
            offset += 6

        # 4. Visible Mobs (6 * 5 = 30)
        mobs = obs.get("visible_mobs", [])
        sorted_mobs = sorted(
            mobs,
            key=lambda m: math.hypot(float(m["pos"][0]) - px, float(m["pos"][1]) - py)
        )[:MAX_MOBS]

        for i in range(MAX_MOBS):
            if i < len(sorted_mobs):
                m = sorted_mobs[i]
                mpos = m.get("pos", [px, py])
                mx, my = float(mpos[0]), float(mpos[1])
                mdist = math.hypot(mx - px, my - py)
                mhp = float(m.get("hp", 40.0))
                mmax_hp = max(1.0, float(m.get("max_hp", 40.0)))

                features[offset + 0] = 1.0  # exists mask
                features[offset + 1] = (mx - px) / W
                features[offset + 2] = (my - py) / H
                features[offset + 3] = mdist / MAX_DIAG
                features[offset + 4] = np.clip(mhp / mmax_hp, 0.0, 1.0)
            else:
                features[offset + 0] = 0.0
                features[offset + 1] = 0.0
                features[offset + 2] = 0.0
                features[offset + 3] = 1.0
                features[offset + 4] = 0.0
            offset += 5

        # 5. Visible Bullets (4 * 4 = 16)
        bullets = obs.get("visible_bullets", [])
        sorted_bullets = sorted(
            bullets,
            key=lambda b: math.hypot(float(b["pos"][0]) - px, float(b["pos"][1]) - py)
        )[:MAX_BULLETS]

        for i in range(MAX_BULLETS):
            if i < len(sorted_bullets):
                b = sorted_bullets[i]
                bpos = b.get("pos", [px, py])
                bx, by = float(bpos[0]), float(bpos[1])
                bdist = math.hypot(bx - px, by - py)

                features[offset + 0] = 1.0  # exists mask
                features[offset + 1] = (bx - px) / W
                features[offset + 2] = (by - py) / H
                features[offset + 3] = bdist / MAX_DIAG
            else:
                features[offset + 0] = 0.0
                features[offset + 1] = 0.0
                features[offset + 2] = 0.0
                features[offset + 3] = 1.0
            offset += 4

        return features

    def parse_features(self, obs: Dict[str, Any]) -> ObservationFeatures:
        """Parses observation into structured ObservationFeatures dataclass."""
        vec = self.encode(obs)
        you = obs.get("you", {})
        alive = you.get("alive", False)
        px, py = you.get("pos", [W / 2.0, H / 2.0]) if alive else (W / 2.0, H / 2.0)

        enemies = obs.get("visible_enemies", [])
        nearest_enemy_dist = min([math.hypot(e["pos"][0] - px, e["pos"][1] - py) for e in enemies], default=None)

        mobs = obs.get("visible_mobs", [])
        nearest_mob_dist = min([math.hypot(m["pos"][0] - px, m["pos"][1] - py) for m in mobs], default=None)

        bullets = obs.get("visible_bullets", [])
        nearest_bullet_dist = min([math.hypot(b["pos"][0] - px, b["pos"][1] - py) for b in bullets], default=None)

        zone = obs.get("zone", {})
        cx, cy = zone.get("center", [W / 2.0, H / 2.0])
        radius = zone.get("radius", 600.0)
        zone_dist = math.hypot(cx - px, cy - py)
        outside_zone = zone_dist > radius

        return ObservationFeatures(
            raw=vec,
            self_pos=(px, py),
            self_alive=alive,
            nearest_enemy_dist=nearest_enemy_dist,
            nearest_mob_dist=nearest_mob_dist,
            nearest_bullet_dist=nearest_bullet_dist,
            zone_dist=zone_dist,
            outside_zone=outside_zone,
        )
