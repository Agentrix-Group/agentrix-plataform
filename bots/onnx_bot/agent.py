#!/usr/bin/env python3
import sys
import json
import math

from pathlib import Path
import numpy as np
import onnxruntime as ort

class NeuralPolicy:
    """Load the packaged, exported ONNX policy. No random/fallback policy."""
    def __init__(self):
        options = ort.SessionOptions()
        options.intra_op_num_threads = 1
        options.inter_op_num_threads = 1
        options.execution_mode = ort.ExecutionMode.ORT_SEQUENTIAL
        self.session = ort.InferenceSession(
            str(Path(__file__).with_name("model.onnx")),
            sess_options=options, providers=["CPUExecutionProvider"])
        inputs = self.session.get_inputs()
        if len(inputs) != 1 or inputs[0].name != "features" or inputs[0].shape != [1, 12]:
            raise ValueError("policy must accept float32 features [1,12]")
        # Actual inference during INIT, before READY, catches incompatible models.
        self.predict([0.0] * 12)

    def predict(self, features):
        output = self.session.run(["policy"], {"features": np.asarray([features], dtype=np.float32)})[0]
        if output.shape != (1, 3) or not np.isfinite(output).all():
            raise ValueError("policy must produce finite direction_x/direction_y/shoot [1,3]")
        dx, dy, shoot = map(float, output[0])
        return math.atan2(dy, dx), shoot > 0.5

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
            model = NeuralPolicy()
            response = {"status": "READY"}
            sys.stdout.write(json.dumps(response) + "\n")
            sys.stdout.flush()

        elif phase == "TICK":
            if model is None:
                raise RuntimeError("TICK received before INIT")

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
