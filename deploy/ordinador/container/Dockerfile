FROM ubuntu:24.04@sha256:2198e4c65076e9cb0b3d695219df667c0e3b0072da64ca3c2648aca17dcecaf5
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl gnupg dbus-x11 tigervnc-standalone-server \
      openbox novnc python3-websockify fonts-noto fonts-noto-color-emoji \
      x11-utils xauth iproute2 socat && \
    install -d -m 0755 /etc/apt/keyrings && \
    curl -fsSL https://dl.google.com/linux/linux_signing_key.pub | gpg --dearmor --yes -o /etc/apt/keyrings/google-chrome.gpg && \
    chmod 0644 /etc/apt/keyrings/google-chrome.gpg && \
    printf '%s\n' 'deb [arch=amd64 signed-by=/etc/apt/keyrings/google-chrome.gpg] https://dl.google.com/linux/chrome/deb/ stable main' > /etc/apt/sources.list.d/google-chrome.list && \
    apt-get update && apt-get install -y --no-install-recommends google-chrome-stable && \
    install -d -m 1777 /tmp/.X11-unix && \
    install -d -o 1000 -g 1000 -m 0700 /run/gregal-ordinador /data/perfil && \
    rm -rf /var/lib/apt/lists/*
COPY entrypoint.sh /usr/local/bin/gregal-browser-entrypoint
RUN chmod 0755 /usr/local/bin/gregal-browser-entrypoint
USER 1000:1000
ENV HOME=/home/ubuntu DISPLAY=:1
EXPOSE 6080 9223
HEALTHCHECK --interval=20s --timeout=5s --start-period=30s --retries=3 \
  CMD curl -fsS http://127.0.0.1:9222/json/version >/dev/null && curl -fsS http://127.0.0.1:6080/vnc.html >/dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/gregal-browser-entrypoint"]
