"""
Agentrix Training SDK
Official Python environment, encoders, baselines, and evaluation kit for Agentrix.
"""

__version__ = "0.2.0"

ENGINE_VERSION = "agentrix-engine-v1"
RULES_VERSION = "agentrix-rules-v1"
OBSERVATION_VERSION = "agentrix-obs-v1"
FEATURE_ENCODER_VERSION = "agentrix-features-v1"
SCORE_VERSION = "agentrix-score-v1"

from .encoder import FeatureEncoder, ObservationFeatures
from .reward import RewardCalculator, RewardConfig
from .baselines import (
    RandomPolicy,
    HunterPolicy,
    MobFarmerPolicy,
    SurvivorPolicy,
    ZoneControllerPolicy,
    BASELINES,
    get_baseline,
)
from .env import ArbiterClient, AgentrixParallelEnv, AgentrixEnv
from .evaluate import evaluate_agents

__all__ = [
    "ENGINE_VERSION",
    "RULES_VERSION",
    "OBSERVATION_VERSION",
    "FEATURE_ENCODER_VERSION",
    "SCORE_VERSION",
    "FeatureEncoder",
    "ObservationFeatures",
    "RewardCalculator",
    "RewardConfig",
    "RandomPolicy",
    "HunterPolicy",
    "MobFarmerPolicy",
    "SurvivorPolicy",
    "ZoneControllerPolicy",
    "BASELINES",
    "get_baseline",
    "ArbiterClient",
    "AgentrixParallelEnv",
    "AgentrixEnv",
    "evaluate_agents",
]
