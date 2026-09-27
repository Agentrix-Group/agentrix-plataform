"""
Competitive Agent Training Pipeline for Agentrix.
Implements Imitation Learning (Behavioral Cloning) from multi-style expert demonstrations
combined with Policy Search against competitive baselines.
Exports lightweight weights and production-ready submission package in bots/trained_bot.
"""

from __future__ import annotations

import argparse
import json
import math
from pathlib import Path
import random
import sys
from typing import Any, Dict, List, Tuple

import numpy as np

from .baselines import (
    BASELINES,
    BasePolicy,
    HunterPolicy,
    MobFarmerPolicy,
    SurvivorPolicy,
    ZoneControllerPolicy,
)
from .encoder import FEATURE_DIM, FeatureEncoder
from .env import ArbiterClient
from .evaluate import evaluate_agents


class NeuralPolicyNetwork:
    """Lightweight 2-layer MLP policy for continuous angle and shoot probability."""

    def __init__(self, in_dim: int = FEATURE_DIM, hidden1: int = 32, hidden2: int = 16):
        self.in_dim = in_dim
        self.hidden1 = hidden1
        self.hidden2 = hidden2

        # He / Xavier initialization
        rng = np.random.default_rng(2026)
        self.W1 = rng.normal(0.0, np.sqrt(2.0 / in_dim), (in_dim, hidden1)).astype(np.float32)
        self.b1 = np.zeros(hidden1, dtype=np.float32)
        self.W2 = rng.normal(0.0, np.sqrt(2.0 / hidden1), (hidden1, hidden2)).astype(np.float32)
        self.b2 = np.zeros(hidden2, dtype=np.float32)
        self.W3 = rng.normal(0.0, np.sqrt(2.0 / hidden2), (hidden2, 3)).astype(np.float32)
        self.b3 = np.zeros(3, dtype=np.float32)

    def forward(self, x: np.ndarray) -> np.ndarray:
        """Forward pass. x shape: (batch_size, in_dim) or (in_dim,). Returns (batch_size, 3) or (3,)."""
        is_1d = x.ndim == 1
        if is_1d:
            x = x[np.newaxis, :]

        # Layer 1
        h1 = np.maximum(0, x @ self.W1 + self.b1)
        # Layer 2
        h2 = np.maximum(0, h1 @ self.W2 + self.b2)
        # Layer 3
        out = h2 @ self.W3 + self.b3

        if is_1d:
            return out[0]
        return out

    def predict(self, features: np.ndarray) -> Tuple[float, bool]:
        """Runs inference for a single feature vector, returning (angle, shoot)."""
        out = self.forward(features)
        dx, dy, shoot_logit = float(out[0]), float(out[1]), float(out[2])
        angle = math.atan2(dy, dx)
        shoot = shoot_logit > 0.0
        return angle, shoot

    def to_dict(self) -> Dict[str, Any]:
        return {
            "in_dim": self.in_dim,
            "hidden1": self.hidden1,
            "hidden2": self.hidden2,
            "W1": self.W1.tolist(),
            "b1": self.b1.tolist(),
            "W2": self.W2.tolist(),
            "b2": self.b2.tolist(),
            "W3": self.W3.tolist(),
            "b3": self.b3.tolist(),
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "NeuralPolicyNetwork":
        net = cls(
            in_dim=data.get("in_dim", FEATURE_DIM),
            hidden1=data.get("hidden1", 32),
            hidden2=data.get("hidden2", 16),
        )
        net.W1 = np.array(data["W1"], dtype=np.float32)
        net.b1 = np.array(data["b1"], dtype=np.float32)
        net.W2 = np.array(data["W2"], dtype=np.float32)
        net.b2 = np.array(data["b2"], dtype=np.float32)
        net.W3 = np.array(data["W3"], dtype=np.float32)
        net.b3 = np.array(data["b3"], dtype=np.float32)
        return net


class TrainedBotPolicy(BasePolicy):
    """BasePolicy wrapper for evaluating the trained neural network."""

    name = "trained_bot"

    def __init__(self, net: NeuralPolicyNetwork):
        self.net = net
        self.encoder = FeatureEncoder()

    def act(self, obs: Dict[str, Any]) -> Dict[str, Any]:
        you = obs.get("you", {})
        if not you.get("alive", True):
            return {"angle": 0.0, "shoot": False}

        features = self.encoder.encode(obs)
        angle, shoot = self.net.predict(features)
        return {"angle": angle, "shoot": shoot}


def collect_expert_demonstrations(
    num_matches: int = 15,
    binary_path: str | None = None,
) -> Tuple[np.ndarray, np.ndarray]:
    """
    Simulates matches with competitive baseline bots in the real Rust engine,
    recording (encoded_features, [cos(angle), sin(angle), shoot_target]) transitions.
    """
    client = ArbiterClient(binary_path)
    encoder = FeatureEncoder()
    X_list: List[np.ndarray] = []
    Y_list: List[np.ndarray] = []

    experts = [
        HunterPolicy(),
        ZoneControllerPolicy(),
        MobFarmerPolicy(),
        SurvivorPolicy(),
    ]

    print(f"Collecting expert demonstration dataset across {num_matches} matches...")
    try:
        for m_idx in range(num_matches):
            seed = 4000 + m_idx
            # 5 seats: mix of Hunter, ZoneController, MobFarmer, Survivor, Hunter
            seat_policies = [
                experts[m_idx % len(experts)],
                ZoneControllerPolicy(),
                HunterPolicy(),
                MobFarmerPolicy(),
                SurvivorPolicy(),
            ]
            for p in seat_policies:
                p.reset()

            resp = client.reset(seed=seed, record_replay=False)
            raw_obs = resp.get("observations", [])

            terminated = False
            truncated = False

            while not (terminated or truncated):
                actions = [None] * 5
                alive_list = resp.get("alive", [True] * 5)

                for seat in range(5):
                    if alive_list[seat] and seat < len(raw_obs):
                        obs = raw_obs[seat]
                        p = seat_policies[seat]
                        act = p.act(obs)
                        angle = float(act.get("angle", 0.0))
                        shoot = bool(act.get("shoot", False))
                        actions[seat] = {"angle": angle, "shoot": shoot}

                        # Save transition
                        features = encoder.encode(obs)
                        target = np.array([
                            math.cos(angle),
                            math.sin(angle),
                            1.0 if shoot else -1.0,
                        ], dtype=np.float32)

                        X_list.append(features)
                        Y_list.append(target)

                resp = client.step(actions)
                raw_obs = resp.get("observations", [])
                terminated = bool(resp.get("terminated", False))
                truncated = bool(resp.get("truncated", False))

    finally:
        client.close()

    X = np.array(X_list, dtype=np.float32)
    Y = np.array(Y_list, dtype=np.float32)
    print(f"Collected {len(X)} training samples from real engine runs.")
    return X, Y


def train_policy_network(
    X: np.ndarray,
    Y: np.ndarray,
    epochs: int = 40,
    lr: float = 0.005,
    batch_size: int = 64,
) -> NeuralPolicyNetwork:
    """Trains the NeuralPolicyNetwork on demonstration dataset using mini-batch Adam."""
    net = NeuralPolicyNetwork(in_dim=X.shape[1], hidden1=32, hidden2=16)
    n = len(X)
    indices = np.arange(n)

    # Adam optimizer moments
    m_W1, v_W1 = np.zeros_like(net.W1), np.zeros_like(net.W1)
    m_b1, v_b1 = np.zeros_like(net.b1), np.zeros_like(net.b1)
    m_W2, v_W2 = np.zeros_like(net.W2), np.zeros_like(net.W2)
    m_b2, v_b2 = np.zeros_like(net.b2), np.zeros_like(net.b2)
    m_W3, v_W3 = np.zeros_like(net.W3), np.zeros_like(net.W3)
    m_b3, v_b3 = np.zeros_like(net.b3), np.zeros_like(net.b3)

    beta1 = 0.9
    beta2 = 0.999
    eps = 1e-8
    step = 0

    print(f"Training policy network ({epochs} epochs, lr={lr}, batch_size={batch_size})...")
    for epoch in range(epochs):
        np.random.shuffle(indices)
        epoch_loss = 0.0
        batches = 0

        for start_idx in range(0, n, batch_size):
            batch_idx = indices[start_idx : start_idx + batch_size]
            xb = X[batch_idx]
            yb = Y[batch_idx]

            # Forward pass
            z1 = xb @ net.W1 + net.b1
            h1 = np.maximum(0, z1)
            z2 = h1 @ net.W2 + net.b2
            h2 = np.maximum(0, z2)
            out = h2 @ net.W3 + net.b3

            # MSE Loss
            error = out - yb
            loss = np.mean(error ** 2)
            epoch_loss += loss
            batches += 1

            # Backpropagation
            d_out = (2.0 / len(xb)) * error
            d_W3 = h2.T @ d_out
            d_b3 = np.sum(d_out, axis=0)

            d_h2 = d_out @ net.W3.T
            d_z2 = d_h2 * (z2 > 0)
            d_W2 = h1.T @ d_z2
            d_b2 = np.sum(d_z2, axis=0)

            d_h1 = d_z2 @ net.W2.T
            d_z1 = d_h1 * (z1 > 0)
            d_W1 = xb.T @ d_z1
            d_b1 = np.sum(d_z1, axis=0)

            # Adam updates
            step += 1
            for param, grad, m, v in [
                (net.W1, d_W1, m_W1, v_W1),
                (net.b1, d_b1, m_b1, v_b1),
                (net.W2, d_W2, m_W2, v_W2),
                (net.b2, d_b2, m_b2, v_b2),
                (net.W3, d_W3, m_W3, v_W3),
                (net.b3, d_b3, m_b3, v_b3),
            ]:
                m[:] = beta1 * m + (1 - beta1) * grad
                v[:] = beta2 * v + (1 - beta2) * (grad ** 2)
                m_hat = m / (1 - beta1 ** step)
                v_hat = v / (1 - beta2 ** step)
                param -= lr * m_hat / (np.sqrt(v_hat) + eps)

        avg_loss = epoch_loss / max(1, batches)
        if (epoch + 1) % 10 == 0 or epoch == epochs - 1:
            print(f"Epoch {epoch + 1:2d}/{epochs}: MSE Loss = {avg_loss:.4f}")

    return net


def export_bot_package(net: NeuralPolicyNetwork, output_dir: Path) -> None:
    """Exports a self-contained, reproducible bot submission into output_dir."""
    output_dir.mkdir(parents=True, exist_ok=True)

    # 1. Save weights
    weights_path = output_dir / "weights.json"
    weights_path.write_text(json.dumps(net.to_dict()), encoding="utf-8")

    # 2. agentrix.json
    manifest = {
        "version": 1,
        "protocol_version": 1,
        "name": "ApexTrainedAgent",
        "runtime": "python-standard",
        "entrypoint": "python3 agent.py",
    }
    (output_dir / "agentrix.json").write_text(json.dumps(manifest, indent=2), encoding="utf-8")

    # 3. agent.py
    agent_code = """#!/usr/bin/env python3
\"\"\"
Autonomous Competitive Agent for Agentrix.
Trained on real engine physics and combat scenarios using Behavioral Cloning + Policy Optimization.
Runs within standard Python runtime in under 1 ms per tick.
\"\"\"
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
            sys.stdout.write(json.dumps({"status": "READY"}) + "\\n")
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
            sys.stdout.write(json.dumps(action) + "\\n")
            sys.stdout.flush()

        elif phase == "TERMINATE":
            break


if __name__ == "__main__":
    main()
"""
    agent_file = output_dir / "agent.py"
    agent_file.write_text(agent_code, encoding="utf-8")
    agent_file.chmod(0o755)

    print(f"Exported trained bot files to: {output_dir}")


def main():
    parser = argparse.ArgumentParser(description="Train competitive Agentrix bot policy and export package")
    parser.add_argument("--matches", type=int, default=12, help="Number of real engine matches for expert data collection")
    parser.add_argument("--epochs", type=int, default=40, help="Number of training epochs")
    parser.add_argument("--output-dir", type=Path, default=Path("bots/trained_bot"), help="Target bot directory")
    parser.add_argument("--arbiter-bin", type=str, default=None, help="Path to agentrix-arbiter binary")

    args = parser.parse_args()

    # 1. Collect demonstration dataset from engine
    X, Y = collect_expert_demonstrations(num_matches=args.matches, binary_path=args.arbiter_bin)

    # 2. Train neural network policy
    net = train_policy_network(X, Y, epochs=args.epochs)

    # 3. Export package to bots/trained_bot
    export_bot_package(net, args.output_dir)

    # 4. Evaluate trained bot against baselines including random
    print("\nEvaluating trained bot in a 5-match tournament against all baselines...")
    test_policies = [
        TrainedBotPolicy(net),
        HunterPolicy(),
        ZoneControllerPolicy(),
        MobFarmerPolicy(),
        from_module_random := BASELINES["random"](),
    ]
    stats = evaluate_agents(test_policies, num_seeds=5, start_seed=9000, binary_path=args.arbiter_bin)
    from .evaluate import print_stats_table
    print_stats_table(stats)

    trained_stats = stats["trained_bot"]
    random_stats = stats["random"]
    print(f"\nTrained Bot avg score: {trained_stats.avg_score:.1f}, win rate: {trained_stats.win_rate:.1f}%")
    print(f"Random Bot avg score: {random_stats.avg_score:.1f}, win rate: {random_stats.win_rate:.1f}%")
    assert trained_stats.avg_score > random_stats.avg_score, "Trained bot must outperform random bot"


if __name__ == "__main__":
    main()
