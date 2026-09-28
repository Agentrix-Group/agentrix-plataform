#!/usr/bin/env python3
import os
import sys
import glob
import re
import shutil
import subprocess

UUID_REGEX = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", re.I)

def main():
    print("==========================================================")
    print("🌐 Configurando Cloudflare Tunnel para cocorium.online")
    print("==========================================================")

    # 1. Determinar el usuario real
    sudo_user = os.environ.get("SUDO_USER") or os.environ.get("USER") or "acm"
    user_home = os.path.expanduser(f"~{sudo_user}")
    user_cf_dir = os.path.join(user_home, ".cloudflared")
    etc_cf_dir = "/etc/cloudflared"

    os.makedirs(etc_cf_dir, exist_ok=True)
    os.makedirs(user_cf_dir, exist_ok=True)

    # 2. Buscar el archivo de credenciales JSON
    search_dirs = [user_cf_dir, etc_cf_dir, os.path.expanduser("~/.cloudflared")]
    json_files = []
    for d in search_dirs:
        if os.path.isdir(d):
            json_files.extend(glob.glob(os.path.join(d, "*.json")))

    tunnel_id = None
    source_json = None
    for jf in json_files:
        base = os.path.basename(jf).replace(".json", "")
        if UUID_REGEX.match(base):
            tunnel_id = base
            source_json = jf
            break

    if not tunnel_id:
        print("❌ Error: No se encontró ningún archivo de credenciales de túnel (*.json) en ~/.cloudflared/")
        sys.exit(1)

    print(f"✅ Túnel ID detectado: {tunnel_id}")
    dest_json = os.path.join(etc_cf_dir, f"{tunnel_id}.json")
    if os.path.abspath(source_json) != os.path.abspath(dest_json):
        shutil.copy2(source_json, dest_json)
        print(f"✅ Credenciales copiadas a {dest_json}")

    # 3. Generar el contenido YAML perfecto
    config_content = f"""tunnel: {tunnel_id}
credentials-file: {dest_json}

ingress:
  - hostname: cocorium.online
    service: http://localhost:3000
  - hostname: www.cocorium.online
    service: http://localhost:3000
  - service: http_status:404
"""

    etc_config = os.path.join(etc_cf_dir, "config.yml")
    user_config = os.path.join(user_cf_dir, "config.yml")

    with open(etc_config, "w") as f:
        f.write(config_content)
    with open(user_config, "w") as f:
        f.write(config_content)

    print(f"✅ Configuración escrita en {etc_config} y {user_config}")

    # 4. Enrutar DNS
    print("📡 Enrutando dominio DNS en Cloudflare...")
    for host in ["cocorium.online", "www.cocorium.online"]:
        cmd = ["cloudflared", "tunnel", "route", "dns", "--overwrite-dns", "agentrix-tunnel", host]
        res = subprocess.run(cmd, capture_output=True, text=True)
        if res.returncode == 0:
            print(f"  ✓ DNS {host} enrutado correctamente.")
        else:
            print(f"  ℹ️ DNS {host}: {res.stderr.strip() or res.stdout.strip()}")

    # 5. Instalar y activar el servicio systemd
    print("⚙️ Instalando y activando servicio permanente de systemd...")
    subprocess.run(["cloudflared", "service", "install"], capture_output=True)
    subprocess.run(["systemctl", "daemon-reload"], check=False)
    subprocess.run(["systemctl", "enable", "--now", "cloudflared"], check=False)

    # 6. Comprobar estado
    status = subprocess.run(["systemctl", "is-active", "cloudflared"], capture_output=True, text=True)
    if "active" in status.stdout:
        print("✅ Servicio cloudflared activo y funcionando.")
    else:
        print(f"ℹ️ Estado de cloudflared: {status.stdout.strip()}")

    print("==========================================================")
    print("🎉 ¡Túnel configurado con éxito!")
    print("Ya puedes entrar a: https://cocorium.online")
    print("==========================================================")

if __name__ == "__main__":
    main()
