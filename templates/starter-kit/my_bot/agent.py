#!/usr/bin/env python3
"""
Agentrix Starter Bot Template (python-standard runtime).

This bot responds to the official Agentrix tick protocol:
- INIT: Prepares inference or state, responds with {"status": "READY"}.
- TICK: Parses visible entities, computes direction angle and shoot intent,
        responds with {"tick": N, "angle": float, "shoot": bool}.
- TERMINATE: Cleans up and exits.

You can modify this file directly to implement your own heuristics/rules,
or run `agentrix-train --output-dir my_bot` to train neural policy weights!
"""

from __future__ import annotations

import json
import math
import os
from pathlib import Path
import sys
from typing import Any, Dict, List, Optional, Tuple


class StarterBotBrain:
    def __init__(self, weights_path: Optional[Path] = None):
        self.weights: Optional[Dict[str, Any]] = None
        if weights_path and weights_path.is_file():
            try:
                self.weights = json.loads(weights_path.read_text(encoding="utf-8"))
            except Exception:
                self.weights = None

    def act(self, obs: Dict[str, Any]) -> Tuple[float, bool]:
        """Computes (angle, shoot) from the TICK observation."""
        my_unit = obs.get("self", {})
        my_x = float(my_unit.get("x", 0.0))
        my_y = float(my_unit.get("y", 0.0))

        # 1. Check if trained neural weights exist
        if self.weights is not None:
            return self._neural_act(obs)

        # 2. Heuristic fallback: target nearest enemy bot or mob
        units = obs.get("units", [])
        mobs = obs.get("mobs", [])

        target_x: Optional[float] = None
        target_y: Optional[float] = None
        min_dist = float("inf")
        should_shoot = False

        # Find closest living enemy
        for u in units:
            if not u.get("alive", True):
                continue
            ux = float(u.get("x", 0.0))
            uy = float(u.get("y", 0.0))
            dist = math.hypot(ux - my_x, uy - my_y)
            if dist < min_dist:
                min_dist = dist
                target_x, target_y = ux, uy
                should_shoot = dist < 250.0  # In combat range

        # If no visible enemy, target closest neutral mob
        if target_x is None:
            for m in mobs:
                if not m.get("alive", True):
                    continue
                mx = float(m.get("x", 0.0))
                my = float(m.get("y", 0.0))
                dist = math.hypot(mx - my_x, my - my_y)
                if dist < min_dist:
                    min_dist = dist
                    target_x, target_y = mx, my
                    should_shoot = dist < 200.0

        # If no targets visible, move towards arena safe zone center
        zone = obs.get("safe_zone") or obs.get("zone") or {}
        if target_x is None:
            target_x = float(zone.get("x", 0.0))
            target_y = float(zone.get("y", 0.0))
            should_shoot = False

        angle = math.atan2(target_y - my_y, target_x - my_x)
        return angle, should_shoot

    def _neural_act(self, obs: Dict[str, Any]) -> Tuple[float, bool]:
        """Runs lightweight feedforward inference using weights.json."""
        # Simple fallback angle if forward pass isn't implemented here
        return 0.0, True


def main():
    weights_path = Path(__file__).with_name("weights.json")
    brain = None

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
            brain = StarterBotBrain(weights_path)
            sys.stdout.write(json.dumps({"status": "READY"}) + "\n")
            sys.stdout.flush()

        elif phase == "TICK":
            if brain is None:
                brain = StarterBotBrain(weights_path)

            angle, shoot = brain.act(msg)
            action = {
                "tick": msg.get("tick", 0),
                "angle": angle,
                "shoot": shoot,
            }
            sys.stdout.write(json.dumps(action) + "\n")
            sys.stdout.flush()

        elif phase == "TERMINATE":
            break


if __name__ == "__main__":
    main()
