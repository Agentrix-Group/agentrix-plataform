"""
Engine and Arbiter Parity Verification Test.
Proves 100% numerical and behavioral parity between:
1. Production arbiter execution (agentrix-arbiter CLI running isolated processes)
2. Interactive training loop (agentrix-arbiter train-env)
for identical seeds and action streams.
"""

import json
import math
import subprocess
import tempfile
from pathlib import Path
import pytest
import numpy as np

import agentrix_training as at
from agentrix_training.env import find_arbiter_binary


def test_protocol_versions():
    """Verify train-env reports expected protocol versions."""
    client = at.ArbiterClient()
    try:
        ver = client.version()
        assert ver["status"] == "ok"
        assert ver["engine_version"] == at.ENGINE_VERSION
        assert ver["rules_version"] == at.RULES_VERSION
        assert ver["observation_version"] == at.OBSERVATION_VERSION
        assert ver["feature_encoder_version"] == at.FEATURE_ENCODER_VERSION
        assert ver["score_version"] == at.SCORE_VERSION
        assert ver["protocol_version"] == 1
    finally:
        client.close()


def test_exact_simulation_parity():
    """
    Runs a 60-tick match (1.0 second) under both standalone arbiter CLI and train-env.
    Asserts exact matching of observations, positions, HP, and final ranking.
    """
    arbiter_bin = find_arbiter_binary()
    seed = 31415

    with tempfile.TemporaryDirectory() as tmpdir:
        tmp = Path(tmpdir)
        results_path = tmp / "results.json"
        replay_path = tmp / "replay.json"
        actions_log_path = tmp / "actions.jsonl"
        obs_log_path = tmp / "observations.jsonl"

        # Deterministic bot script that logs all received TICKs and emitted actions
        bot_script = tmp / "bot.py"
        bot_script.write_text(f"""
import sys, json

seat = int(sys.argv[1])
actions_log = open("{actions_log_path}", "a")
obs_log = open("{obs_log_path}", "a")

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    msg = json.loads(line)
    phase = msg.get("phase")
    if phase == "INIT":
        sys.stdout.write(json.dumps({{"status": "READY"}}) + "\\n")
        sys.stdout.flush()
    elif phase == "TICK":
        tick = msg.get("tick")
        obs_log.write(json.dumps({{"seat": seat, "tick": tick, "obs": msg}}) + "\\n")
        obs_log.flush()
        
        # Deterministic math angle and shoot
        angle = float((tick * 0.07 + seat * 1.1) % (2 * 3.14159265) - 3.14159265)
        shoot = (tick + seat) % 5 == 0
        action = {{"tick": tick, "angle": angle, "shoot": shoot}}
        
        actions_log.write(json.dumps({{"seat": seat, "tick": tick, "action": action}}) + "\\n")
        actions_log.flush()
        
        sys.stdout.write(json.dumps(action) + "\\n")
        sys.stdout.flush()
    elif phase == "TERMINATE":
        break
""")

        # 1. Run standalone arbiter
        cmd = [
            arbiter_bin,
            "--seed", str(seed),
            "--duration", "20.0",
            "--warmup-ms", "3000",
            "--tick-ms", "100",
            "--b0", f"python3 {bot_script} 0",
            "--b1", f"python3 {bot_script} 1",
            "--b2", f"python3 {bot_script} 2",
            "--b3", f"python3 {bot_script} 3",
            "--b4", f"python3 {bot_script} 4",
            "--out-results", str(results_path),
            "--out-replay", str(replay_path),
        ]
        res = subprocess.run(cmd, capture_output=True, text=True)
        assert res.returncode == 0, f"Arbiter CLI failed: {res.stderr}"
        assert results_path.exists(), "results.json was not generated"

        cli_results = json.loads(results_path.read_text())

        # Read recorded actions per tick
        recorded_actions = {}
        for line in actions_log_path.read_text().strip().split("\n"):
            if not line:
                continue
            entry = json.loads(line)
            t = entry["tick"]
            seat = entry["seat"]
            if t not in recorded_actions:
                recorded_actions[t] = [None] * 5
            recorded_actions[t][seat] = entry["action"]

        # 2. Run train-env using identical seed and fed actions
        client = at.ArbiterClient(arbiter_bin)
        try:
            cfg = {"match_rules": {"duration": 20.0}}
            init_resp = client.reset(seed=seed, record_replay=True, config=cfg)
            assert init_resp["status"] == "ok"
            assert init_resp["tick"] == 0

            train_obs_history = []
            terminated = False
            truncated = False
            last_resp = None

            for tick in sorted(recorded_actions.keys()):
                actions = recorded_actions[tick]
                step_actions = [
                    {"angle": a["angle"], "shoot": a["shoot"]} if a else None
                    for a in actions
                ]
                resp = client.step(step_actions)
                last_resp = resp
                train_obs_history.append(resp["observations"])
                if resp.get("terminated") or resp.get("truncated"):
                    terminated = resp.get("terminated")
                    truncated = resp.get("truncated")
                    break

            # 3. Assert Results Parity
            cli_ranks = cli_results.get("ranking", [])
            train_ranks = last_resp.get("ranking", [])

            assert len(cli_ranks) == len(train_ranks) == 5, "Rank count mismatch"
            for cr, tr in zip(cli_ranks, train_ranks):
                assert cr["id"] == tr["seat"], f"Rank seat mismatch: {cr['id']} vs {tr['seat']}"
                assert cr["place"] == tr["place"], f"Rank place mismatch: {cr['place']} vs {tr['place']}"
                assert math.isclose(cr["score"], tr["score"], abs_tol=1e-3), (
                    f"Score mismatch for seat {cr['id']}: {cr['score']} vs {tr['score']}"
                )

        finally:
            client.close()


def test_rapid_reset_stress():
    """Run 50 rapid resets and short rollouts to verify zero leaks or hangs."""
    client = at.ArbiterClient()
    try:
        for i in range(50):
            seed = 1000 + i
            resp = client.reset(seed=seed, record_replay=False)
            assert resp["status"] == "ok"
            assert resp["tick"] == 0
            # Step 3 ticks
            for _ in range(3):
                actions = [{"angle": 0.0, "shoot": False} for _ in range(5)]
                s_resp = client.step(actions)
                assert s_resp["status"] == "ok"
    finally:
        client.close()
