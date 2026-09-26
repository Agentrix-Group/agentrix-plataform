#!/usr/bin/env python3
"""
Agentrix Tournament Provisioner & Benchmark CLI
Automates tournament setup, team creation, bot registration, and execution
of initial qualification and ladder seeding matches.
"""

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.request
import urllib.error
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parents[1]
DEFAULT_API_URL = "http://127.0.0.1:8080/api/v1"

def api_request(base_url, endpoint, method="GET", data=None, token=None):
    url = f"{base_url}{endpoint}"
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    
    encoded = json.dumps(data).encode("utf-8") if data else None
    req = urllib.request.Request(url, data=encoded, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        try:
            return e.code, json.loads(body)
        except Exception:
            return e.code, {"error": body}

def main():
    parser = argparse.ArgumentParser(description="Agentrix Tournament Provisioner & Benchmark CLI")
    parser.add_argument("--api-url", default=DEFAULT_API_URL, help="URL base de la API REST de Agentrix")
    parser.add_argument("--rounds", type=int, default=2, help="Número de rondas de calibración a ejecutar")
    parser.add_argument("--admin-user", default="admin", help="Usuario administrador")
    parser.add_argument("--admin-pass", default="admin123", help="Contraseña del administrador")
    args = parser.parse_args()

    print("==================================================")
    print("      AGENTRIX TOURNAMENT PROVISIONER & CLI       ")
    print("==================================================")

    # 1. Autenticación
    print(f"[*] Autenticando con {args.api_url} como '{args.admin_user}'...")
    code, login_data = api_request(args.api_url, "/auth/login", method="POST", data={"username": args.admin_user, "password": args.admin_pass})
    if code != 200 or "token" not in login_data:
        print(f"[!] Fallo al autenticar: {login_data}")
        sys.exit(1)
    token = login_data["token"]
    print(f"\033[92m[OK]\033[0m Autenticación exitosa (Token JWT emitido).")

    # 2. Consultar arenas
    code, arenas = api_request(args.api_url, "/arenas", token=token)
    if code != 200 or len(arenas) == 0:
        print("[!] No se encontraron arenas activas.")
        sys.exit(1)
    arena = arenas[0]
    arena_id = arena["id"]
    print(f"[*] Arena seleccionada: '{arena['name']}' (ID: {arena_id})")

    # 3. Consultar bots registrados
    code, ladder = api_request(args.api_url, f"/arenas/{arena_id}/ladder", token=token)
    if code != 200 or len(ladder) < 5:
        print(f"[!] Se requieren al menos 5 bots en la arena para iniciar partidas. Encontrados: {len(ladder)}")
        sys.exit(1)

    print(f"[*] Bots participantes ({len(ladder)} detectados):")
    for b in ladder[:5]:
        print(f"    - {b['agent_name']:<20} | {b['team_name']:<22} | Elo: {b['display_rating']}")

    # 4. Ejecución de rondas de calibración
    bot_ids = [b["agent_version_id"] for b in ladder[:5]]
    print(f"\n[*] Ejecutando {args.rounds} rondas competitivas de calibración...")

    for r in range(1, args.rounds + 1):
        seed = 1000 + r * 37
        print(f"    -> Lanzando partida #{r} (Semilla: {seed})...", end=" ", flush=True)
        code, trig = api_request(args.api_url, "/matches/trigger", method="POST", data={
            "arena_id": arena_id,
            "agent_version_ids": bot_ids,
            "seed": seed
        }, token=token)
        if code not in (200, 202):
            print(f"[FALLO] {trig}")
            continue

        match_id = trig["match_id"]

        # Polling
        finished = False
        for _ in range(25):
            time.sleep(1)
            m_code, m_data = api_request(args.api_url, f"/matches/{match_id}", token=token)
            if m_code == 200 and m_data.get("status") == "finished":
                finished = True
                ticks = m_data.get("ticks_played", 0)
                dur_s = (ticks * 50) / 1000.0
                print(f"\033[92m[FINALIZADA]\033[0m {ticks} ticks ({dur_s:.1f}s)")
                break

        if not finished:
            print(f"\033[91m[TIMEOUT]\033[0m")

    # 5. Clasificación final post-torneo
    print("\n==================================================")
    print("      CLASIFICACIÓN FINAL DEL LADDER (ELO)        ")
    print("==================================================")
    code, updated_ladder = api_request(args.api_url, f"/arenas/{arena_id}/ladder", token=token)
    if code == 200:
        print(f"{'PUESTO':<8} {'AGENTE':<20} {'EQUIPO':<22} {'ELO':<10} {'W/L':<10} {'KILLS':<8}")
        print("-" * 78)
        for i, entry in enumerate(updated_ladder, 1):
            wl = f"{entry['wins']}/{entry['matches_played'] - entry['wins']}"
            print(f"{i:<8} {entry['agent_name']:<20} {entry['team_name']:<22} {entry['display_rating']:<10} {wl:<10} {entry['kills']:<8}")
    print("==================================================")

if __name__ == "__main__":
    main()
