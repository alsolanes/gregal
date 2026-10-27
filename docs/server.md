# Your own server

The Android app works on its own, inside the phone. If you want the winds to do
long work (websites, analyses, whole crews) that keeps going after you close the
app, run Gregal on a computer or server and connect the phone to it. At the top
of the Winds screen there is a **📱 On the phone · Server** switch: one tap and
you change. Each place keeps its own winds and chats.

Connecting the phone is always the same: **address, port and key**. The server
tells you when it starts.

## At home (same Wi-Fi)

On the computer:

```bash
gregal --serve --addr 0.0.0.0
```

It prints a message like this:

```
Gregal is running.
In the phone app, "Connect to a server", enter:
  Address:   192.168.1.62
  Port:      8097
  Key:       7buvc-9brxu-mujdw-emenx
```

On the phone: tap **Add a server** (on the switch at the top), type the three
things and **Connect**. The key is created the first time and saved: it is the
same on every start.

Away from home, the switch's dot turns red and you stay on the phone.

## On the internet (an always-on machine with your domain)

To reach it from anywhere you need a domain with HTTPS. You need:

- A Linux machine with systemd (Debian or Ubuntu for the automatic Caddy
  install): a VPS or a mini-PC.
- A domain or subdomain pointing at it (A record, and AAAA if it has IPv6).
- Ports 80 and 443 open (80 is only used by Let's Encrypt).

1. Build the Linux binary (or download it from the release):

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
   sudo sh install.sh --domain winds.example.com --binary ./gregal-linux-amd64 --email you@example.com
   ```

   At the end it says what to enter in the app:

   ```
   Gregal ja corre. A l'app del mòbil, «Connecta't a un servidor», escriu:
     Adreça:  https://winds.example.com
     Port:    (no cal)
     Clau:    xxxxx-xxxxx-xxxxx-xxxxx
   ```

4. Open `https://winds.example.com` in a browser, enter the key and, in
   Settings, add your model provider key and the Exa key (web search). They are
   stored on the server.

### What the installer does

- Creates the `gregal` system user (home `/var/lib/gregal`): Gregal never runs
  as root and, with systemd sandboxing, cannot write outside its home.
- Installs the `gregal` service, listening only on `127.0.0.1:8097` and
  restarting by itself if it crashes or the machine reboots.
- Generates the access key and stores it in `/etc/gregal/env` (root only): it
  does not show in the process list or the config. It is kept on reinstall.
- Puts Caddy in front: automatic HTTPS certificate, HSTS, and the winds' live
  replies without buffering.
- Optional: `--user NAME` also adds an account with a password (for servers
  shared by several people).

It can be run again to update the binary: it does not touch the key, the
config, the accounts or the data.

## Security

- The key has 20 random characters (about 99 bits): it cannot be guessed.
- Failed password sign-ins are limited per address (8 per 5 minutes) and
  overall (60 per 5 minutes). The address comes from the connection;
  `X-Forwarded-For` is only trusted from the proxy on the same machine.
- `X-Content-Type-Options`, `X-Frame-Options` and `Referrer-Policy` on every
  response; HSTS from Caddy.
- Time limits for request headers and idle connections.
- To change the key, delete the `GREGAL_API_TOKEN` line from `/etc/gregal/env`,
  run the installer again and reconnect the phone.

## Long work

Crew work runs on the server: closing the app or the browser does not stop it.
Its current step is saved to disk; if the server restarts mid-task, the app
says so ("stopped: the server restarted") instead of waiting forever.

## Maintenance

- Log (includes the address, port and key at start): `journalctl -u gregal -f`
- Restart: `sudo systemctl restart gregal`
- Update: copy the new binary and run `install.sh` again with the same options.
- Data: everything is in `/var/lib/gregal` (back it up).
