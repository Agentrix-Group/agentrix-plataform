#!/usr/bin/env python3
import sys
import json
import math
import time

# Intenta usar numpy / onnx si están instalados, o fallback a álgebra matricial pura
try:
    import numpy as np
    HAVE_NUMPY = True
except ImportError:
    HAVE_NUMPY = False

class NeuralPolicy:
    """Red neuronal MLP ligera: 12 entradas -> 16 ocultas -> 2 salidas (ángulo, disparo)"""
    def __init__(self):
        # Pesos simulados inicializados con semilla fija
        if HAVE_NUMPY:
            rng = np.random.default_rng(2026)
            self.w1 = rng.standard_normal((12, 16)).astype(np.float32)
            self.b1 = np.zeros(16, dtype=np.float32)
            self.w2 = rng.standard_normal((16, 2)).astype(np.float32)
            self.b2 = np.zeros(2, dtype=np.float32)
        else:
            self.w1 = [[0.1 * ((i + j) % 5 - 2) for j in range(16)] for i in range(12)]
            self.w2 = [[0.1 * ((i + j) % 3 - 1) for j in range(2)] for i in range(16)]

    def predict(self, features):
        if HAVE_NUMPY:
            x = np.array(features, dtype=np.float32)
            h = np.maximum(0, np.dot(x, self.w1) + self.b1)  # ReLU
            out = np.dot(h, self.w2) + self.b2
            angle = float(out[0]) % (2.0 * math.pi)
            shoot = bool(out[1] > 0.0)
            return angle, shoot
        else:
            # Fallback matemático sin numpy
            h = [0.0] * 16
            for j in range(16):
                val = sum(features[i] * self.w1[i][j] for i in range(12))
                h[j] = max(0.0, val)
            out = [sum(h[j] * self.w2[j][k] for j in range(16)) for k in range(2)]
            angle = out[0] % (2.0 * math.pi)
            shoot = out[1] > 0.0
            return angle, shoot

def main():
    model = None

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
            # Simular carga de modelo y compilación JIT (100 ms)
            time.sleep(0.05)
            model = NeuralPolicy()
            response = {"status": "READY"}
            sys.stdout.write(json.dumps(response) + "\n")
            sys.stdout.flush()

        elif phase == "TICK":
            if model is None:
                model = NeuralPolicy()

            you = msg.get("you", {})
            my_pos = you.get("pos", [600.0, 375.0])
            enemies = msg.get("visible_enemies", [])
            mobs = msg.get("visible_mobs", [])
            zone = msg.get("zone", {})
            center = zone.get("center", [600.0, 375.0])

            # Normalizar vector de 12 características para la red
            nearest_enemy = enemies[0]["pos"] if enemies else [center[0], center[1]]
            nearest_mob = mobs[0]["pos"] if mobs else [center[0], center[1]]

            features = [
                my_pos[0] / 1200.0,
                my_pos[1] / 750.0,
                you.get("hp", 100.0) / you.get("max_hp", 100.0),
                you.get("speed", 90.0) / 320.0,
                you.get("facing", 0.0) / (2.0 * math.pi),
                (center[0] - my_pos[0]) / 1200.0,
                (center[1] - my_pos[1]) / 750.0,
                (nearest_enemy[0] - my_pos[0]) / 1200.0,
                (nearest_enemy[1] - my_pos[1]) / 750.0,
                (nearest_mob[0] - my_pos[0]) / 1200.0,
                (nearest_mob[1] - my_pos[1]) / 750.0,
                zone.get("radius", 600.0) / 750.0
            ]

            angle, shoot = model.predict(features)

            action = {
                "tick": msg.get("tick"),
                "angle": angle,
                "shoot": shoot
            }
            sys.stdout.write(json.dumps(action) + "\n")
            sys.stdout.flush()

        elif phase == "TERMINATE":
            break

if __name__ == "__main__":
    main()
