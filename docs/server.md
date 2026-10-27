# Your own server on the internet

Gregal can run on an always-on Linux machine (a VPS or a mini-PC at home) and
open on your domain over HTTPS. Winds can then do long work (websites,
analyses, whole crews) that keeps going after you close the app, and you
follow it from the phone or any browser.

## Requirements

- A Linux machine with systemd (Debian or Ubuntu for the automatic Caddy
  install).
- A domain (or subdomain) pointing at that machine: an A record (and AAAA if it
  has IPv6).
- Ports 80 and 443 open to the internet (80 is only used by Let's Encrypt to
  issue the certificate).

## Install

1. Build the Linux binary from the repository (or download it from the
   release):

   ```bash
   ./build.sh
   ```

   It lands in `dist/gregal-linux-amd64` (or `-arm64` for a Raspberry Pi).

2. Copy the binary and the installer to the machine:

   ```bash
   scp dist/gregal-linux-amd64 deploy/linux/install.sh user@server:
   ```

3. On the machine, run the installer with your domain:

   ```bash
   sudo sh install.sh --domain winds.example.com --binary ./gregal-linux-amd64 --user alex --email you@example.com
   ```

   At the end it prints the address, the user and a generated password. Save
   it: it is not shown again.

4. Open `https://winds.example.com` in a browser, sign in and, in Settings, add
   your model provider key and the Exa key (web search). They are stored on the
   server.

5. On the phone: "Switch server" → "Or connect to a server…", type
   `https://winds.example.com` (no port needed), the user and the password.

## What the installer does

- Creates the `gregal` system user (home `/var/lib/gregal`): Gregal never runs
  as root and, with systemd sandboxing, cannot write outside its home.
- Installs the `gregal` service, listening only on `127.0.0.1:8097` and
  restarting by itself if it crashes or the machine reboots.
- Adds an admin account with a password (PBKDF2-hashed): on the internet
  everyone signs in with their own name; there is no shared token.
- Puts Caddy in front: automatic HTTPS certificate, HSTS, and the winds' live
  streams without buffering.

It can be run again to update the binary: it does not touch the config, the
accounts or the data.

## Security

- Failed sign-ins are limited per address (8 per 5 minutes) and overall (60
  per 5 minutes). The address comes from the connection; `X-Forwarded-For` is
  only trusted from the proxy on the same machine.
- `X-Content-Type-Options`, `X-Frame-Options` and `Referrer-Policy` on every
  response; HSTS from Caddy.
- Time limits for request headers and idle connections.
- To change the password:

  ```bash
  sudo runuser -u gregal -- env HOME=/var/lib/gregal gregal --config /var/lib/gregal/.config/gregal/config.yaml --passwd alex
  ```

- To add a person, add them to the `users:` section of
  `/var/lib/gregal/.config/gregal/config.yaml` and set a password with the same
  command.

## Long work

Crew work runs on the server: closing the app or the browser does not stop it.
Its current step is saved to disk; if the server restarts mid-task, the app
says so ("stopped: the server restarted") instead of waiting forever.

## Maintenance

- Log: `journalctl -u gregal -f`
- Restart: `sudo systemctl restart gregal`
- Update: copy the new binary and run `install.sh` again with the same options.
- Data: everything is in `/var/lib/gregal` (back it up).
