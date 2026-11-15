#!/usr/bin/env bash
set -euo pipefail
umask 077
mkdir -p /run/gregal-ordinador /data/perfil

Xvnc :1 \
  -rfbport -1 \
  -rfbunixpath /run/gregal-ordinador/vnc.sock \
  -rfbunixmode 0600 \
  -SecurityTypes None \
  -geometry 1280x900 \
  -depth 24 &
XVNC_PID=$!

cleanup() {
  kill "${CHROME_PID:-}" "${WEBSOCKIFY_PID:-}" "${RELAY_PID:-}" "${OPENBOX_PID:-}" "${XVNC_PID:-}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

for _ in $(seq 1 60); do
  if xdpyinfo -display :1 >/dev/null 2>&1; then break; fi
  sleep 1
done
xdpyinfo -display :1 >/dev/null 2>&1 || { echo 'Xvnc did not become ready' >&2; exit 1; }

openbox-session >/dev/null 2>&1 &
OPENBOX_PID=$!
/usr/bin/websockify --web /usr/share/novnc 0.0.0.0:6080 \
  --unix-target=/run/gregal-ordinador/vnc.sock &
WEBSOCKIFY_PID=$!

# Chrome binds DevTools to its loopback. Forward only over this container's
# private Docker interface; the host publishes that port on its own loopback.
DOCKER_IP=$(ip -o -4 addr show dev eth0 | awk '{split($4,a,"/"); print a[1]}')
test -n "$DOCKER_IP"
socat "TCP-LISTEN:9223,bind=$DOCKER_IP,reuseaddr,fork" TCP:127.0.0.1:9222 &
RELAY_PID=$!

# Chrome's namespace sandbox is blocked by the Docker default seccomp profile.
# Run it as UID 1000 without capabilities inside the restricted container;
# the remote-debugging and viewer ports are only reachable through the host's
# loopback-published ports.
dbus-run-session -- google-chrome-stable \
  --user-data-dir=/data/perfil \
  --remote-debugging-address=0.0.0.0 \
  --remote-debugging-port=9222 \
  --no-first-run \
  --no-default-browser-check \
  --disable-dev-shm-usage \
  --password-store=basic \
  --no-sandbox \
  about:blank &
CHROME_PID=$!

wait -n "$XVNC_PID" "$WEBSOCKIFY_PID" "$RELAY_PID" "$OPENBOX_PID" "$CHROME_PID"
exit 1
