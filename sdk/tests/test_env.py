"""
Tests for AgentrixParallelEnv (PettingZoo) and AgentrixEnv (Gymnasium).
"""

import math
import numpy as np
import pytest
from agentrix_training.encoder import FEATURE_DIM
from agentrix_training.env import AgentrixEnv, AgentrixParallelEnv


def test_parallel_env_lifecycle():
    env = AgentrixParallelEnv()
    try:
        assert len(env.possible_agents) == 5
        assert env.observation_space("player_0") is not None
        assert env.action_space("player_0") is not None

        obs, infos = env.reset(seed=12345)
        assert len(obs) == 5
        assert len(infos) == 5
        for agent in env.possible_agents:
            assert obs[agent].shape == (FEATURE_DIM,)
            assert obs[agent].dtype == np.float32

        # Step with 5 continuous actions: [angle, shoot]
        actions = {agent: np.array([0.5, 1.0], dtype=np.float32) for agent in env.agents}
        next_obs, rewards, terminations, truncations, next_infos = env.step(actions)

        assert len(next_obs) <= 5
        for agent in env.agents:
            assert isinstance(rewards[agent], (float, int))
            assert isinstance(terminations[agent], bool)
            assert isinstance(truncations[agent], bool)

    finally:
        env.close()


def test_gym_single_agent_env():
    env = AgentrixEnv()
    try:
        obs, info = env.reset(seed=9876)
        assert obs.shape == (FEATURE_DIM,)
        assert "seat" in info

        # Step 5 times
        for _ in range(5):
            action = np.array([0.0, 1.0], dtype=np.float32)
            obs, reward, terminated, truncated, info = env.step(action)
            assert obs.shape == (FEATURE_DIM,)
            assert isinstance(reward, float)
            assert isinstance(terminated, bool)
            assert isinstance(truncated, bool)
            if terminated or truncated:
                break
    finally:
        env.close()
