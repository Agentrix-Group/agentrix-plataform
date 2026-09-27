# Example packaging SDK

The host-side builder compiles repository-owned C++ source before packaging.
Never use it to compile untrusted participant submissions on the host.
The server receives a prebuilt `bot_bin` and never invokes a compiler at startup.

Fetch `json.hpp` from the official nlohmann/json v3.12.0 single-header release.
The builder enforces its published SHA-256
`aaf127c04cb31c406e5b04a63f1ae89369fccde6d8fa7cdda1ed4f32dfc5de63`.
The header includes upstream copyright/license notices and is included in the
C++ ZIP. See https://github.com/nlohmann/json/releases/tag/v3.12.0.

```bash
python3 scripts/package-example-bots.py \
  --json-header /absolute/path/to/json.hpp --output /tmp/new-example-packages
```

Output directory must not contain existing ZIPs. Packages have explicit file
allowlists, fixed timestamps and executable permissions; no pycache, legacy
run scripts or compiler outputs accidentally enter them. C++ links statically
to avoid relying on the host's shared-library ABI. Linux x86_64 is the tested
target; a pinned compiler/runtime image is still required for a release SDK.

The ML probe/export environment and Docker runtime use Python 3.14 and
`requirements-py314.lock`, including the actual ONNX Runtime CPU engine. The
lock pins versions but does not include wheel hashes; image digests, runtime
filesystem construction and parity acceptance remain pending. This is not yet
a certified reproducible deployment image.
