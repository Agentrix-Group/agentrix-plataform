#!/usr/bin/env python3
"""Train a tiny synthetic demonstration policy and export actual ONNX weights.

This demonstrates packaging/inference, not a competitive or production model.
Run with the pinned SDK export dependencies. The resulting model is a build asset.
"""
import argparse
from pathlib import Path
import numpy as np
import onnx
from onnx import TensorProto, helper, numpy_helper

def export(destination):
    rng = np.random.default_rng(2026)
    features = rng.uniform(-1, 1, (2048, 12)).astype(np.float32)
    targets = np.column_stack((features[:, 5], features[:, 6],
                               (np.hypot(features[:, 7], features[:, 8]) < .3))).astype(np.float32)
    # Supervised linear regression on synthetic movement/shoot labels.
    design = np.column_stack((features, np.ones(len(features), dtype=np.float32)))
    fitted = np.linalg.lstsq(design, targets, rcond=None)[0].astype(np.float32)
    weights, bias = fitted[:-1], fitted[-1]
    graph = helper.make_graph(
        [helper.make_node("Gemm", ["features", "weights", "bias"], ["policy"])],
        "agentrix-synthetic-demo-policy",
        [helper.make_tensor_value_info("features", TensorProto.FLOAT, [1, 12])],
        [helper.make_tensor_value_info("policy", TensorProto.FLOAT, [1, 3])],
        [numpy_helper.from_array(weights, "weights"), numpy_helper.from_array(bias, "bias")])
    model = helper.make_model(graph, producer_name="agentrix-example-sdk",
                              opset_imports=[helper.make_opsetid("", 17)])
    model.ir_version = 9
    model.doc_string = "Synthetic supervised demo, seed 2026, 2048 examples; not a competitive policy."
    onnx.checker.check_model(model)
    onnx.save(model, str(destination))

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("bots/onnx_bot/model.onnx"))
    export(parser.parse_args().output)
