#!/usr/bin/env python3
"""
Autonomous Competitive Agent for Agentrix.
Trained on real engine physics and combat scenarios using Behavioral Cloning + Policy Optimization.
Runs within standard Python runtime in under 1 ms per tick.
"""
import json
import math
from pathlib import Path
import sys
import numpy as np

W = 1200.0
H = 750.0
MAX_DIAG = math.hypot(W, H)
MAX_ENEMIES = 4
MAX_MOBS = 6
MAX_BULLETS = 4
FEATURE_DIM = 83


class AgentBrain:
    def __init__(self, weights_path):
        with open(weights_path, "r", encoding="utf-8") as f:
            data = json.load(f)
        self.W1 = np.array(data["W1"], dtype=np.float32)
        self.b1 = np.array(data["b1"], dtype=np.float32)
        self.W2 = np.array(data["W2"], dtype=np.float32)
        self.b2 = np.array(data["b2"], dtype=np.float32)
        self.W3 = np.array(data["W3"], dtype=np.float32)
        self.b3 = np.array(data["b3"], dtype=np.float32)

    def encode(self, obs):
        features = np.zeros(FEATURE_DIM, dtype=np.float32)
        you = obs.get("you", {})
        if not you.get("alive", True):
            return features

        my_pos = you.get("pos", [W / 2.0, H / 2.0])
        px, py = float(my_pos[0]), float(my_pos[1])
        hp = float(you.get("hp", 100.0))
        max_hp = max(1.0, float(you.get("max_hp", 100.0)))
        speed = float(you.get("speed", 90.0))
        facing = float(you.get("facing", 0.0))
        cooldown = float(you.get("cooldown", 0.0))
        level = float(you.get("level", 1))

        # Self
        features[0] = px / W
        features[1] = py / H
        features[2] = min(max(hp / max_hp, 0.0), 1.0)
        features[3] = min(max(speed / 320.0, 0.0), 2.0)
        features[4] = (facing % (2.0 * math.pi)) / (2.0 * math.pi)
        features[5] = min(max(cooldown / 0.5, 0.0), 1.0)
        features[6] = min(max(level / 10.0, 0.0), 1.0)
        features[7] = 1.0

        # Zone
        zone = obs.get("zone", {})
        center = zone.get("center", [W / 2.0, H / 2.0])
        cx, cy = float(center[0]), float(center[1])
        radius = float(zone.get("radius", 600.0))
        dist_c = math.hypot(cx - px, cy - py)
        features[8] = (cx - px) / W
        features[9] = (cy - py) / H
        features[10] = dist_c / MAX_DIAG
        features[11] = radius / H
        features[12] = 1.0 if dist_c > radius else 0.0

        # Enemies (up to 4)
        offset = 13
        enemies = obs.get("visible_enemies", [])
        sorted_enemies = sorted(enemies, key=lambda e: math.hypot(e["pos"][0] - px, e["pos"][1] - py))[:MAX_ENEMIES]
        for i in range(MAX_ENEMIES):
            if i < len(sorted_enemies):
                e = sorted_enemies[i]
                epos = e.get("pos", [px, py])
                edist = math.hypot(epos[0] - px, epos[1] - py)
                features[offset + 0] = 1.0
                features[offset + 1] = (epos[0] - px) / W
                features[offset + 2] = (epos[1] - py) / H
                features[offset + 3] = edist / MAX_DIAG
                features[offset + 4] = min(max(e.get("hp", 100.0) / max(1.0, e.get("max_hp", 100.0)), 0.0), 1.0)
                features[offset + 5] = min(max(e.get("level", 1) / 10.0, 0.0), 1.0)
            else:
                features[offset + 3] = 1.0
            offset += 6

        # Mobs (up to 6)
        mobs = obs.get("visible_mobs", [])
        sorted_mobs = sorted(mobs, key=lambda m: math.hypot(m["pos"][0] - px, m["pos"][1] - py))[:MAX_MOBS]
        for i in range(MAX_MOBS):
            if i < len(sorted_mobs):
                m = sorted_mobs[i]
                mpos = m.get("pos", [px, py])
                mdist = math.hypot(mpos[0] - px, mpos[1] - py)
                features[offset + 0] = 1.0
                features[offset + 1] = (mpos[0] - px) / W
                features[offset + 2] = (mpos[1] - py) / H
                features[offset + 3] = mdist / MAX_DIAG
                features[offset + 4] = min(max(m.get("hp", 40.0) / max(1.0, m.get("max_hp", 40.0)), 0.0), 1.0)
            else:
                features[offset + 3] = 1.0
            offset += 5

        # Bullets (up to 4)
        bullets = obs.get("visible_bullets", [])
        sorted_bullets = sorted(bullets, key=lambda b: math.hypot(b["pos"][0] - px, b["pos"][1] - py))[:MAX_BULLETS]
        for i in range(MAX_BULLETS):
            if i < len(sorted_bullets):
                b = sorted_bullets[i]
                bpos = b.get("pos", [px, py])
                bdist = math.hypot(bpos[0] - px, bpos[1] - py)
                features[offset + 0] = 1.0
                features[offset + 1] = (bpos[0] - px) / W
                features[offset + 2] = (bpos[1] - py) / H
                features[offset + 3] = bdist / MAX_DIAG
            else:
                features[offset + 3] = 1.0
            offset += 4

        return features

    def act(self, features):
        h1 = np.maximum(0, features @ self.W1 + self.b1)
        h2 = np.maximum(0, h1 @ self.W2 + self.b2)
        out = h2 @ self.W3 + self.b3
        angle = math.atan2(float(out[1]), float(out[0]))
        shoot = float(out[2]) > 0.0
        return angle, shoot


def main():
    brain = None
    weights_path = Path(__file__).with_name("weights.json")

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except Exception:
            continue

        phase = msg.get("phase")
        if phase == "INIT":
            brain = AgentBrain(weights_path)
            # Warmup inference
            brain.act(np.zeros(FEATURE_DIM, dtype=np.float32))
            sys.stdout.write(json.dumps({"status": "READY"}) + "\n")
            sys.stdout.flush()

        elif phase == "TICK":
            if brain is None:
                raise RuntimeError("TICK before INIT")

            features = brain.encode(msg)
            angle, shoot = brain.act(features)

            action = {
                "tick": msg.get("tick"),
                "angle": angle,
                "shoot": shoot,
            }
            sys.stdout.write(json.dumps(action) + "\n")
            sys.stdout.flush()

        elif phase == "TERMINATE":
            break


if __name__ == "__main__":
    main()
