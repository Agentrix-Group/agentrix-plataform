"""
Agentrix Training Environments:
- ArbiterClient: Low-level IPC client communicating with `agentrix-arbiter train-env`
- AgentrixParallelEnv: PettingZoo multi-agent ParallelEnv (5 players)
- AgentrixEnv: Gymnasium single-agent Env with opponent baselines
"""

from __future__ import annotations

import json
import math
import os
import subprocess
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple, Union

import numpy as np

from .encoder import FEATURE_DIM, FeatureEncoder
from .reward import RewardCalculator, RewardConfig
from .baselines import (
    BasePolicy,
    HunterPolicy,
    MobFarmerPolicy,
    RandomPolicy,
    SurvivorPolicy,
    ZoneControllerPolicy,
)

# Optional Gymnasium / PettingZoo imports with graceful fallbacks
try:
    import gymnasium as gym
    from gymnasium import spaces

    HAS_GYM = True
except ImportError:
    HAS_GYM = False
    gym = object  # type: ignore
    spaces = None  # type: ignore

try:
    import pettingzoo
    from pettingzoo import ParallelEnv

    HAS_PETTINGZOO = True
except ImportError:
    HAS_PETTINGZOO = False
    ParallelEnv = object  # type: ignore


def find_arbiter_binary(custom_path: Optional[str] = None) -> str:
    """Finds the agentrix-arbiter binary path."""
    if custom_path and os.path.isfile(custom_path) and os.access(custom_path, os.X_OK):
        return str(Path(custom_path).resolve())

    env_path = os.environ.get("AGENTRIX_ARBITER_BIN")
    if env_path and os.path.isfile(env_path) and os.access(env_path, os.X_OK):
        return str(Path(env_path).resolve())

    # Candidate locations relative to repo root
    current = Path(__file__).resolve()
    # current is in sdk/agentrix_training/env.py -> repo root is parents[2]
    repo_root = current.parents[2]
    candidates = [
        repo_root / "simulation/arbiter/target/release/agentrix-arbiter",
        repo_root / "simulation/arbiter/target/debug/agentrix-arbiter",
    ]
    for cand in candidates:
        if cand.is_file() and os.access(cand, os.X_OK):
            return str(cand.resolve())

    # Fallback to PATH
    import shutil
    which = shutil.which("agentrix-arbiter")
    if which:
        return which

    raise FileNotFoundError(
        "Could not find agentrix-arbiter binary. Please compile it with 'cargo build --release' "
        "inside simulation/arbiter or set AGENTRIX_ARBITER_BIN."
    )


class ArbiterClient:
    """IPC client for agentrix-arbiter train-env protocol."""

    def __init__(self, binary_path: Optional[str] = None):
        self.binary_path = find_arbiter_binary(binary_path)
        self.process: Optional[subprocess.Popen] = None
        self._start_process()

    def _start_process(self):
        if self.process is not None:
            self.close()
        self.process = subprocess.Popen(
            [self.binary_path, "train-env"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )

    def _send(self, req: Dict[str, Any]) -> Dict[str, Any]:
        if self.process is None or self.process.poll() is not None:
            self._start_process()

        line = json.dumps(req) + "\n"
        try:
            assert self.process.stdin is not None
            self.process.stdin.write(line)
            self.process.stdin.flush()

            assert self.process.stdout is not None
            resp_line = self.process.stdout.readline()
            if not resp_line:
                stderr = self.process.stderr.read() if self.process.stderr else ""
                raise RuntimeError(f"Arbiter process terminated unexpectedly: {stderr}")

            resp = json.loads(resp_line)
            if resp.get("status") == "error":
                raise RuntimeError(f"Arbiter error: {resp.get('error')}")
            return resp
        except Exception as e:
            self.close()
            raise RuntimeError(f"IPC communication failure with arbiter: {e}") from e

    def version(self) -> Dict[str, Any]:
        return self._send({"command": "version"})

    def reset(
        self,
        seed: int = 2026,
        record_replay: bool = False,
        config: Optional[Dict[str, Any]] = None,
        models: Optional[List[Dict[str, Any]]] = None,
    ) -> Dict[str, Any]:
        req: Dict[str, Any] = {
            "command": "reset",
            "seed": seed,
            "record_replay": record_replay,
        }
        if config is not None:
            req["config"] = config
        if models is not None:
            req["models"] = models
        return self._send(req)

    def step(self, actions: List[Optional[Dict[str, Any]]]) -> Dict[str, Any]:
        if len(actions) != 5:
            raise ValueError(f"Step requires exactly 5 action entries, got {len(actions)}")
        return self._send({"command": "step", "actions": actions})

    def get_replay(self) -> Dict[str, Any]:
        return self._send({"command": "get_replay"})

    def close(self):
        if self.process is not None:
            try:
                if self.process.poll() is None and self.process.stdin:
                    self.process.stdin.write(json.dumps({"command": "close"}) + "\n")
                    self.process.stdin.flush()
                self.process.terminate()
                self.process.wait(timeout=1.0)
            except Exception:
                if self.process.poll() is None:
                    self.process.kill()
            finally:
                self.process = None

    def __del__(self):
        self.close()


class AgentrixParallelEnv(ParallelEnv if HAS_PETTINGZOO else object):  # type: ignore
    """
    PettingZoo ParallelEnv implementation for 5-player Agentrix matches.
    """

    metadata = {"render_modes": ["ansi", "replay"], "name": "agentrix_parallel_v1"}

    def __init__(
        self,
        binary_path: Optional[str] = None,
        reward_config: Optional[RewardConfig] = None,
        record_replay: bool = False,
        config: Optional[Dict[str, Any]] = None,
    ):
        self.client = ArbiterClient(binary_path)
        self.encoder = FeatureEncoder()
        self.reward_calculator = RewardCalculator(reward_config)
        self.record_replay = record_replay
        self.config = config

        self.possible_agents = [f"player_{i}" for i in range(5)]
        self.agents = list(self.possible_agents)
        self.agent_to_seat = {f"player_{i}": i for i in range(5)}
        self.seat_to_agent = {i: f"player_{i}" for i in range(5)}

        if HAS_GYM and spaces is not None:
            self.observation_spaces = {
                agent: spaces.Box(
                    low=-2.0,
                    high=2.0,
                    shape=(FEATURE_DIM,),
                    dtype=np.float32,
                )
                for agent in self.possible_agents
            }
            # Action: [angle in radians (-pi..pi), shoot flag (>0.5)]
            self.action_spaces = {
                agent: spaces.Box(
                    low=np.array([-math.pi, 0.0], dtype=np.float32),
                    high=np.array([math.pi, 1.0], dtype=np.float32),
                    dtype=np.float32,
                )
                for agent in self.possible_agents
            }
        else:
            self.observation_spaces = {}
            self.action_spaces = {}

    def observation_space(self, agent: str):
        return self.observation_spaces.get(agent)

    def action_space(self, agent: str):
        return self.action_spaces.get(agent)

    def reset(
        self,
        seed: Optional[int] = None,
        options: Optional[Dict[str, Any]] = None,
    ) -> Tuple[Dict[str, np.ndarray], Dict[str, Any]]:
        self.agents = list(self.possible_agents)
        self.reward_calculator.reset()

        actual_seed = seed if seed is not None else 2026
        record = (options or {}).get("record_replay", self.record_replay)
        cfg = (options or {}).get("config", self.config)

        resp = self.client.reset(seed=actual_seed, record_replay=record, config=cfg)

        raw_observations = resp.get("observations", [])
        observations: Dict[str, np.ndarray] = {}
        infos: Dict[str, Any] = {}

        for i, agent in enumerate(self.agents):
            obs_json = raw_observations[i] if i < len(raw_observations) else {}
            observations[agent] = self.encoder.encode(obs_json)
            infos[agent] = {"raw_obs": obs_json, "seat": i}

        return observations, infos

    def step(
        self,
        actions: Dict[str, Union[np.ndarray, List[float], Dict[str, Any]]],
    ) -> Tuple[
        Dict[str, np.ndarray],
        Dict[str, float],
        Dict[str, bool],
        Dict[str, bool],
        Dict[str, Any],
    ]:
        seat_actions: List[Optional[Dict[str, Any]]] = [None] * 5

        for agent, act in actions.items():
            if agent not in self.agent_to_seat:
                continue
            seat = self.agent_to_seat[agent]
            if isinstance(act, dict):
                seat_actions[seat] = {
                    "angle": float(act.get("angle", 0.0)),
                    "shoot": bool(act.get("shoot", False)),
                }
            elif isinstance(act, (list, tuple, np.ndarray)) and len(act) >= 2:
                seat_actions[seat] = {
                    "angle": float(act[0]),
                    "shoot": bool(act[1] > 0.5),
                }

        resp = self.client.step(seat_actions)

        raw_obs = resp.get("observations", [])
        metrics = resp.get("metrics", [])
        alive = resp.get("alive", [False] * 5)
        terminated_match = bool(resp.get("terminated", False))
        truncated_match = bool(resp.get("truncated", False))
        ranking = resp.get("ranking")

        step_rewards = self.reward_calculator.compute_step_rewards(
            current_metrics=metrics,
            observations=raw_obs,
            terminated=terminated_match,
            truncated=truncated_match,
            ranking=ranking,
        )

        observations: Dict[str, np.ndarray] = {}
        rewards: Dict[str, float] = {}
        terminations: Dict[str, bool] = {}
        truncations: Dict[str, bool] = {}
        infos: Dict[str, Any] = {}

        active_agents = []
        for i, agent in enumerate(self.possible_agents):
            if agent not in self.agents and not (terminated_match or truncated_match):
                continue

            obs_json = raw_obs[i] if i < len(raw_obs) else {}
            observations[agent] = self.encoder.encode(obs_json)
            rewards[agent] = step_rewards[i]

            agent_alive = alive[i] if i < len(alive) else False
            terminations[agent] = terminated_match or (not agent_alive)
            truncations[agent] = truncated_match

            infos[agent] = {
                "seat": i,
                "alive": agent_alive,
                "metrics": metrics[i] if i < len(metrics) else {},
                "raw_obs": obs_json,
            }
            if ranking:
                infos[agent]["ranking"] = ranking

            if agent_alive and not (terminated_match or truncated_match):
                active_agents.append(agent)

        self.agents = active_agents
        return observations, rewards, terminations, truncations, infos

    def get_replay(self) -> Dict[str, Any]:
        return self.client.get_replay()

    def close(self):
        self.client.close()


class AgentrixEnv(gym.Env if HAS_GYM else object):  # type: ignore
    """
    Single-agent Gymnasium environment for training an agent in seat 0
    against 4 baseline bots.
    """

    metadata = {"render_modes": ["ansi", "replay"]}

    def __init__(
        self,
        binary_path: Optional[str] = None,
        opponent_policies: Optional[List[BasePolicy]] = None,
        reward_config: Optional[RewardConfig] = None,
        record_replay: bool = False,
        config: Optional[Dict[str, Any]] = None,
        learning_seat: int = 0,
    ):
        if HAS_GYM and hasattr(super(), "__init__"):
            super().__init__()

        self.parallel_env = AgentrixParallelEnv(
            binary_path=binary_path,
            reward_config=reward_config,
            record_replay=record_replay,
            config=config,
        )
        self.learning_seat = learning_seat
        self.learning_agent = f"player_{learning_seat}"

        # Default 4 opponents: 1 Hunter, 1 MobFarmer, 1 Survivor, 1 ZoneController
        self.opponents: List[BasePolicy] = opponent_policies or [
            HunterPolicy(),
            MobFarmerPolicy(),
            SurvivorPolicy(),
            ZoneControllerPolicy(),
        ]
        if len(self.opponents) != 4:
            raise ValueError(f"AgentrixEnv requires exactly 4 opponent policies, got {len(self.opponents)}")

        if HAS_GYM and spaces is not None:
            self.observation_space = spaces.Box(
                low=-2.0,
                high=2.0,
                shape=(FEATURE_DIM,),
                dtype=np.float32,
            )
            self.action_space = spaces.Box(
                low=np.array([-math.pi, 0.0], dtype=np.float32),
                high=np.array([math.pi, 1.0], dtype=np.float32),
                dtype=np.float32,
            )

        self.latest_raw_obs: List[Dict[str, Any]] = []

    def reset(
        self,
        seed: Optional[int] = None,
        options: Optional[Dict[str, Any]] = None,
    ) -> Tuple[np.ndarray, Dict[str, Any]]:
        if HAS_GYM and hasattr(super(), "reset"):
            super().reset(seed=seed)

        for opp in self.opponents:
            opp.reset()

        obs_dict, info_dict = self.parallel_env.reset(seed=seed, options=options)
        self.latest_raw_obs = [
            info_dict.get(f"player_{i}", {}).get("raw_obs", {}) for i in range(5)
        ]

        my_obs = obs_dict[self.learning_agent]
        my_info = info_dict[self.learning_agent]
        return my_obs, my_info

    def step(
        self,
        action: Union[np.ndarray, List[float], Dict[str, Any]],
    ) -> Tuple[np.ndarray, float, bool, bool, Dict[str, Any]]:
        # Compute opponent actions from latest raw observations
        actions_dict: Dict[str, Any] = {self.learning_agent: action}

        opp_idx = 0
        for seat in range(5):
            if seat == self.learning_seat:
                continue
            opp_policy = self.opponents[opp_idx]
            opp_idx += 1
            opp_agent = f"player_{seat}"
            if opp_agent in self.parallel_env.agents and seat < len(self.latest_raw_obs):
                opp_obs = self.latest_raw_obs[seat]
                actions_dict[opp_agent] = opp_policy.act(opp_obs)

        obs_dict, rew_dict, term_dict, trunc_dict, info_dict = self.parallel_env.step(actions_dict)

        self.latest_raw_obs = [
            info_dict.get(f"player_{i}", {}).get("raw_obs", {}) for i in range(5)
        ]

        my_obs = obs_dict.get(self.learning_agent, np.zeros(FEATURE_DIM, dtype=np.float32))
        my_reward = rew_dict.get(self.learning_agent, 0.0)
        my_term = term_dict.get(self.learning_agent, True)
        my_trunc = trunc_dict.get(self.learning_agent, False)
        my_info = info_dict.get(self.learning_agent, {})

        return my_obs, my_reward, my_term, my_trunc, my_info

    def get_replay(self) -> Dict[str, Any]:
        return self.parallel_env.get_replay()

    def close(self):
        self.parallel_env.close()
