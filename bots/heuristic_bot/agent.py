#!/usr/bin/env python3
import sys
import json
import math

def distance(a, b):
    return math.hypot(b[0] - a[0], b[1] - a[1])

def toward(a, b):
    return math.atan2(b[1] - a[1], b[0] - a[0])

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except Exception:
            continue

        phase = msg.get("phase")

        if phase == "INIT":
            # Fase de calentamiento: responder READY inmediatamente
            response = {"status": "READY"}
            sys.stdout.write(json.dumps(response) + "\n")
            sys.stdout.flush()

        elif phase == "TICK":
            you = msg.get("you", {})
            my_pos = you.get("pos", [600.0, 375.0])
            enemies = msg.get("visible_enemies", [])
            mobs = msg.get("visible_mobs", [])
            zone = msg.get("zone", {})
            zone_center = zone.get("center", [600.0, 375.0])
            zone_radius = zone.get("radius", 600.0)

            # Si estamos cerca del borde de la zona, moverse hacia el centro
            dist_to_center = distance(my_pos, zone_center)
            if dist_to_center > zone_radius - 80.0:
                angle = toward(my_pos, zone_center)
                shoot = len(enemies) > 0
            elif enemies:
                # Perseguir al enemigo más cercano
                nearest_enemy = min(enemies, key=lambda e: distance(my_pos, e["pos"]))
                dist = distance(my_pos, nearest_enemy["pos"])
                direct = toward(my_pos, nearest_enemy["pos"])
                # Mantener cierta distancia óptima
                if dist > 120.0:
                    angle = direct
                else:
                    angle = direct + math.pi / 2.0  # Rodear
                shoot = True
            elif mobs:
                # Cazar mob más cercano
                nearest_mob = min(mobs, key=lambda m: distance(my_pos, m["pos"]))
                angle = toward(my_pos, nearest_mob["pos"])
                shoot = True
            else:
                # Explorar hacia el centro
                angle = toward(my_pos, zone_center) + 0.3
                shoot = False

            action = {
                "tick": msg.get("tick"),
                "angle": angle,
                "shoot": shoot
            }
            sys.stdout.write(json.dumps(action) + "\n")
            sys.stdout.flush()

        elif phase == "TERMINATE":
            break

if __name__ == "__main__":
    main()
