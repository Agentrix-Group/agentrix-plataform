# Trusted integration fixture. This file is never accepted as participant code.
import json
import sys

for line in sys.stdin:
    message = json.loads(line)
    if message.get("phase") == "INIT":
        print(json.dumps({"status": "READY"}), flush=True)
    elif message.get("phase") == "TICK":
        print(json.dumps({"tick": message["tick"], "angle": 0.0, "shoot": True}), flush=True)
    elif message.get("phase") == "TERMINATE":
        break
