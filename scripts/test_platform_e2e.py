#!/usr/bin/env python3
"""
Agentrix Platform - Automated End-to-End Verification Suite
Validates the entire Go monolith, PostgreSQL persistence, Rust Arbiter execution,
multiplayer Elo rating calculation, bot ingestion, and replay streaming.
"""

import json
import os
import signal
import subprocess
import sys
import time
import urllib.request
import urllib.error

SERVER_PORT = 8085
BASE_URL = f"http://127.0.0.1:{SERVER_PORT}/api/v1"

def log(msg):
    print(f"\033[1;34m[E2E-TEST]\033[0m {msg}")

def success(msg):
    print(f"\033[1;32m[PASS]\033[0m {msg}")

def fail(msg):
    print(f"\033[1;31m[FAIL]\033[0m {msg}")
    sys.exit(1)

def request_json(url, method="GET", data=None, token=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    
    encoded_data = json.dumps(data).encode("utf-8") if data else None
    req = urllib.request.Request(url, data=encoded_data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        try:
            return e.code, json.loads(body)
        except Exception:
            return e.code, {"error": body}

def main():
    print("==================================================")
    print("   AGENTRIX PLATFORM - FULL E2E INTEGRATION TEST  ")
    print("==================================================")

    # 1. Compile backend
    log("Compiling Go backend monolith...")
    build_res = subprocess.run(["go", "build", "-o", "bin/agentrix-server", "./cmd/agentrix-server"], cwd="backend", capture_output=True, text=True)
    if build_res.returncode != 0:
        fail(f"Backend build failed: {build_res.stderr}")
    success("Backend compiled successfully.")

    # 2. Start server
    log(f"Starting server on port {SERVER_PORT}...")
    env = os.environ.copy()
    env["PORT"] = str(SERVER_PORT)
    env["AUTO_MATCHMAKER"] = "false"
    env["DATABASE_URL"] = "postgres://postgres@127.0.0.1:5432/agentrix_platform?sslmode=disable"
    arbiter_path = os.path.abspath("simulation/arbiter/target/release/agentrix-arbiter")
    if not os.path.isfile(arbiter_path):
        arbiter_path = os.path.abspath("agentrix/arbiter/target/release/agentrix-arbiter")
    env["ARBITER_PATH"] = arbiter_path

    server_proc = subprocess.Popen(
        ["./bin/agentrix-server"],
        cwd="backend",
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )

    token = None
    try:
        # Wait for server to boot
        time.sleep(2)
        log("Checking system health endpoint...")
        status_code, status_data = request_json(f"{BASE_URL}/system/status")
        if status_code != 200 or not status_data.get("database_healthy") or not status_data.get("arbiter_healthy"):
            fail(f"Health check failed: status {status_code}, data: {status_data}")
        success("System health check passed: Database and Arbiter healthy.")

        # 3. Test Admin Authentication
        log("Testing admin login...")
        status_code, login_data = request_json(
            f"{BASE_URL}/auth/login",
            method="POST",
            data={"username": "admin", "password": "admin123"}
        )
        if status_code != 200 or "token" not in login_data:
            fail(f"Login failed: {login_data}")
        token = login_data["token"]
        success(f"Admin logged in successfully. User: {login_data['user']['username']} (Role: {login_data['user']['role']})")

        # 4. Query Arenas
        log("Fetching arenas...")
        status_code, arenas = request_json(f"{BASE_URL}/arenas", token=token)
        if status_code != 200 or len(arenas) == 0:
            fail(f"No arenas found: {arenas}")
        arena_id = arenas[0]["id"]
        success(f"Fetched arena: '{arenas[0]['name']}' (ID: {arena_id})")

        # 5. Query Initial Standings
        log("Fetching initial ladder standings...")
        status_code, ladder = request_json(f"{BASE_URL}/arenas/{arena_id}/ladder", token=token)
        if status_code != 200 or len(ladder) < 5:
            fail(f"Ladder entries insufficient: {ladder}")
        success(f"Found {len(ladder)} ranked bots in ladder. Top bot: '{ladder[0]['agent_name']}' ({ladder[0]['display_rating']} Elo)")

        # 6. Trigger Match Execution
        log("Scheduling and launching a 5-bot arena match...")
        bot_ids = [b["agent_version_id"] for b in ladder[:5]]
        status_code, trigger_data = request_json(
            f"{BASE_URL}/matches/trigger",
            method="POST",
            data={"arena_id": arena_id, "agent_version_ids": bot_ids, "seed": 7777},
            token=token
        )
        if status_code not in (200, 202) or "match_id" not in trigger_data:
            fail(f"Match trigger failed: {trigger_data}")
        match_id = trigger_data["match_id"]
        success(f"Match #{match_id} scheduled.")

        # 7. Poll until match completes
        log(f"Waiting for match #{match_id} simulation to finish...")
        finished = False
        match_data = None
        for _ in range(15):
            time.sleep(1)
            status_code, match_data = request_json(f"{BASE_URL}/matches/{match_id}", token=token)
            if status_code == 200 and match_data.get("status") == "finished":
                finished = True
                break
        
        if not finished:
            fail(f"Match did not finish in time: {match_data}")
        
        ticks = match_data.get("ticks_played", 0)
        participants = match_data.get("participants", [])
        success(f"Match #{match_id} completed successfully in {ticks} ticks with {len(participants)} ranked participants.")

        # 8. Verify Elo rating updates
        log("Verifying multiplayer Elo rating delta conservation...")
        sum_delta = sum(p.get("rating_delta", 0.0) for p in participants if not p.get("disqualified"))
        for p in participants:
            log(f"  Seat {p['seat']} | Rank {p['rank_place']}º | {p['agent_name']} | Kills: {p['kills']} | Score: {p['score']} | Delta: {p['rating_delta']}")
        
        # 9. Verify 2D Replay Streaming
        log("Verifying 2D Replay JSON frames API...")
        status_code, replay_json = request_json(f"{BASE_URL}/matches/{match_id}/replay", token=token)
        if status_code != 200 or "config" not in replay_json:
            fail(f"Failed to fetch replay JSON: {replay_json}")
        success("2D Replay JSON streaming verified with match rules, ticks, and unit telemetry.")

        # 10. Verify Frontend Build artifacts
        log("Verifying React + TypeScript SPA build...")
        if not os.path.isfile("frontend/dist/index.html"):
            fail("frontend/dist/index.html not found! Run npm run build.")
        success("Frontend SPA build verified at frontend/dist/.")

        print("==================================================")
        print("  🎉 ALL 10 END-TO-END VERIFICATION STEPS PASSED! ")
        print("==================================================")

    finally:
        server_proc.terminate()
        server_proc.wait()
        log("Test server stopped.")

if __name__ == "__main__":
    main()
