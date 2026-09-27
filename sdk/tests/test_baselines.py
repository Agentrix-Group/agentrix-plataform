"""
Tests for baseline policies: Random, Hunter, MobFarmer, Survivor, ZoneController.
"""

import math
import pytest
from agentrix_training.baselines import (
    BASELINES,
    HunterPolicy,
    MobFarmerPolicy,
    RandomPolicy,
    SurvivorPolicy,
    ZoneControllerPolicy,
    get_baseline,
)


def sample_observation(
    hp=100.0,
    alive=True,
    enemies=None,
    mobs=None,
    bullets=None,
    pos=None,
):
    return {
        "phase": "TICK",
        "tick": 42,
        "time": 0.7,
        "you": {
            "pos": pos or [600.0, 375.0],
            "facing": 0.0,
            "hp": hp,
            "max_hp": 100.0,
            "speed": 90.0,
            "vision": 250.0,
            "damage": 12.0,
            "level": 1,
            "xp": 0.0,
            "xp_next": 40.0,
            "cooldown": 0.0,
            "alive": alive,
        },
        "visible_enemies": enemies or [],
        "visible_mobs": mobs or [],
        "visible_bullets": bullets or [],
        "zone": {
            "radius": 500.0,
            "center": [600.0, 375.0],
        },
        "walls": [],
    }


def test_baselines_registry():
    assert len(BASELINES) == 5
    for name in ["random", "hunter", "mob_farmer", "survivor", "zone_controller"]:
        p = get_baseline(name)
        assert p is not None
        assert p.name == name


def test_baseline_actions_schema():
    obs = sample_observation(
        enemies=[{"id": 1, "pos": [650.0, 375.0], "hp": 50.0, "max_hp": 100.0, "level": 1}],
        mobs=[{"id": 10, "pos": [550.0, 375.0], "hp": 30.0, "max_hp": 30.0}],
    )

    for name in BASELINES.keys():
        policy = get_baseline(name)
        policy.reset()
        action = policy.act(obs)
        assert "angle" in action
        assert "shoot" in action
        assert isinstance(action["angle"], (float, int))
        assert isinstance(action["shoot"], bool)
        assert -2.0 * math.pi <= action["angle"] <= 2.0 * math.pi


def test_hunter_aggression():
    policy = HunterPolicy(engage_distance=150.0)
    policy.strafe_dir = 1.0
    # Far: direct approach (dist = 200 > 150)
    obs_far = sample_observation(
        pos=[600.0, 375.0],
        enemies=[{"id": 1, "pos": [800.0, 375.0], "hp": 100.0, "max_hp": 100.0, "level": 1}],
    )
    act_far = policy.act(obs_far)
    assert act_far["shoot"] is True
    assert math.isclose(act_far["angle"], 0.0, abs_tol=0.1)

    # Close: circle strafe (dist = 100 <= 150)
    obs_close = sample_observation(
        pos=[600.0, 375.0],
        enemies=[{"id": 1, "pos": [700.0, 375.0], "hp": 100.0, "max_hp": 100.0, "level": 1}],
    )
    act_close = policy.act(obs_close)
    assert act_close["shoot"] is True
    assert math.isclose(act_close["angle"], 0.8, abs_tol=0.1)


def test_survivor_dodging():
    policy = SurvivorPolicy(threat_radius=200.0)
    obs = sample_observation(
        pos=[600.0, 375.0],
        bullets=[{"pos": [610.0, 375.0], "color": 1}],
    )
    act = policy.act(obs)
    # Should dodge perpendicularly away from bullet line
    assert abs(act["angle"]) > 0.5
