#!/usr/bin/env bash
# Test-only root for isolation probes. Not the supported ML production runtime.
set -euo pipefail
probe_root=$(mktemp -d /tmp/agentrix-probe-runtime.XXXXXX)
mkdir -p "$probe_root/usr/bin" "$probe_root/usr/lib" "$probe_root/usr/lib64" "$probe_root/tmp" "$probe_root/proc" "$probe_root/dev" "$probe_root/bot"
probe_python=$(readlink -f "$(command -v python3)")
probe_stdlib=$(python3 -I -c 'import sysconfig; print(sysconfig.get_path("stdlib"))')
cp "$probe_python" "$probe_root/usr/bin/python3"
# No host site-packages, credentials or home directories enter this fixture.
tar -C "$probe_stdlib" --exclude=site-packages --exclude=__pycache__ -cf - . | tar -C "$probe_root/usr/lib" -xf - --one-top-level="$(basename "$probe_stdlib")"
if [ "$#" -gt 1 ]; then
 echo 'usage: build-probe-runtime.sh [trusted-pip-target-directory]' >&2
 exit 2
fi
if [ "$#" -eq 1 ]; then
 test -d "$1"
 mkdir -p "$probe_root/usr/lib/$(basename "$probe_stdlib")/site-packages"
 cp -a "$1/." "$probe_root/usr/lib/$(basename "$probe_stdlib")/site-packages/"
fi
while read -r probe_library; do
 # Host library search paths may include ~/.local. Never reproduce host home
 # paths in the root: only trusted resolved library bytes enter /usr/lib.
 cp -L "$probe_library" "$probe_root/usr/lib/$(basename "$probe_library")"
done < <(
 {
  ldd "$probe_python"
  while read -r probe_extension; do ldd "$probe_extension" 2>/dev/null || true; done < <(
   rg --files "$probe_stdlib/lib-dynload" "$probe_root/usr/lib/$(basename "$probe_stdlib")" | rg '\.so($|\.)'
  )
 } | awk '/=> \// {print $3}' | sort -u
)
# ldd's interpreter line is not portable across awk whitespace extensions.
probe_loader=$(ldd "$probe_python" | awk '/ld-linux/ {for(i=1;i<=NF;i++) if ($i ~ /^\//) {print $i; break}}')
mkdir -p "$probe_root$(dirname "$probe_loader")"
cp -L "$probe_loader" "$probe_root$probe_loader"
printf '%s\n' "$probe_root"
