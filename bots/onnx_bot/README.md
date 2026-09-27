# Real ONNX example

`agent.py` loads `model.onnx` using ONNX Runtime CPU and executes inference
before READY and on every tick. Missing files, imports or incompatible tensors
fail admission; there is no random policy or pure-Python fallback.

The packaged model is a tiny linear policy trained on 2,048 synthetic examples
with seed 2026. It demonstrates trained-weight export and inference, not
competitive AI quality. Inputs are the twelve normalized observation features
in `agent.py`; output is direction X/Y plus a shooting score.

Regenerate with `python3 scripts/export-example-policy.py` from the repository
root using `sdk/requirements-py314.lock` with Python 3.14. ONNX opset 17 / IR 9 is explicit.
The model must be inside the submitted ZIP alongside `agent.py` and
`agentrix.json`. The distributed ZIP passes real sandbox admission and a mixed
five-bot match. Runtime/SDK release image parity remains pending.
