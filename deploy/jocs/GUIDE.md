# Gregal Jocs: deployment and administration guide

This guide covers the whole cycle: deploying, how it works, managing users,
setting API keys, updating, backups and troubleshooting.

## 1. How it works

Gregal Jocs is **a single binary** (`gregal-jocs`): the same server as Gregal
with the games registered (the Arena and the winds league). It serves the web
UI at `/jocs/` and the API on the same port (**8197**).

- **Independent of Gregal.** It has its own profile, config, accounts and keys.
  You do not need Gregal deployed and it shares nothing with it. To reuse a
  Gregal model, the administrator can import it once (a copy, not a link).
- **The server provides the models.** The Arena and the league narration call an
  AI model with the **server's API key**; users do not need one.
- **One container, one volume.** All state (config, keys, accounts, matches) is
  in the Docker volume `jocs-data`, mounted at `/data`. The image can be rebuilt
  without losing anything.
- **Authentication.** API access requires a credential: the **master key**
  (administrator) or a **username and password**. The `/jocs/` page itself is
  served without one, but does nothing until you sign in.

```
internet ──► Caddy (80/443, HTTPS) ──► jocs:8197 ──► volume jocs-data (/data)
              (optional, --domain)
```

## 2. Requirements

- A Linux server with **Docker** and the `docker compose` plugin.
- SSH access (to deploy from your computer) and, for HTTPS, a domain pointing
  to the server with ports **80 and 443** open.
- On the machine you deploy from: `sh`, `tar`, `ssh`.

## 3. Deploying

From the repository root:

```sh
# Local test (http://localhost:8197/jocs/)
./deploy/jocs/deploy.sh --admin aleix

# Remote server, no domain (http://IP:8197/jocs/)
./deploy/jocs/deploy.sh --host user@server --admin aleix

# Remote server with a domain and automatic HTTPS (recommended)
./deploy/jocs/deploy.sh --host user@server --domain jocs.example.com --admin aleix
```

The script uploads the code (`~/gregal-jocs` on the server), creates `.env`,
builds the image and starts the service. When it finishes it shows:

- the URL,
- the **master key** (`GREGAL_API_TOKEN`),
- the **administrator password** (first time only: save it).

Without `--domain` the service listens on `0.0.0.0:8197` over **unencrypted
HTTP**: use it only on a trusted network. With `--domain` it listens only on
`127.0.0.1` and Caddy adds HTTPS (Let's Encrypt, automatic renewal).

**Updating:** run the same command again. It rebuilds the image and keeps the
data, `.env` and accounts.

## 4. User management

There are three kinds of access:

| Access | Who | What it can do |
|---|---|---|
| Master key (`GREGAL_API_TOKEN`) | you, for scripts or emergencies | Full administrator |
| Administrator (`JOCS_ADMIN`) | you, with username and password | Administers the server and accounts |
| Managed accounts | everyone else | Play the games; cannot administer |

### Initial administrator

`--admin NAME` creates the administrator the first time and generates the
password (minimum 8 characters). It is stored in `.env` as `JOCS_ADMIN` and
`JOCS_ADMIN_PASSWORD`. **The password in `.env` wins on every start**: to change
it, edit it and run `./deploy/jocs/deploy.sh --restart`; to stop forcing it,
delete the `JOCS_ADMIN_PASSWORD` line (the current password is kept).

### Inviting someone (recommended)

```sh
./deploy/jocs/deploy.sh --host user@server --invite
```

It gives a **single-use code that expires in 30 minutes**. Send it over a
private channel; the person picks their own name and password (minimum 8
characters) on the web UI or the app. It can also be done from the web UI:
**Settings → Advanced → Server accounts**.

### Listing, creating, changing passwords and deleting

```sh
./deploy/jocs/deploy.sh --host user@server --accounts      # list
```

Creating, changing a password, granting administrator rights and deleting are
done in the web UI as administrator: **Settings → Advanced → Server accounts**.

- Changing a password signs that account out of its earlier sessions.
- Deleting an account signs it out and **sets its data aside**
  (`.deleted-<date>`) rather than destroying it; a new account with the same
  name starts empty.
- Names are case-insensitive (if `Anna` exists, `anna` cannot be created).
- Accounts live in `/data/accounts` (inside the volume).

> Use accounts only with trusted people: this is not strong isolation between
> users who run code on the same server.

## 5. Model API keys

The games need a model. There are **two ways** to provide keys.

### A) In the server's `.env` (reproducible)

Edit `~/gregal-jocs/deploy/jocs/.env` on the server (mode 0600) and fill in the
variable of the provider you use:

| Provider | Variable |
|---|---|
| OpenAI | `OPENAI_API_KEY` |
| Anthropic | `ANTHROPIC_API_KEY` |
| OpenRouter | `OPENROUTER_API_KEY` |
| Groq | `GROQ_API_KEY` |
| DeepSeek | `DEEPSEEK_API_KEY` |
| Mistral | `MISTRAL_API_KEY` |
| Together | `TOGETHER_API_KEY` |
| Cerebras | `CEREBRAS_API_KEY` |
| OpenCode Go (Zen) | `ZEN_API_KEY` |
| Web search (Exa, optional) | `EXA_API_KEY` |

Apply it with `./deploy/jocs/deploy.sh --host user@server --restart`.
(`.env.example` has the full list; `deploy.sh` does not add these lines, you
write them yourself.)

The default config already references these variables (`api_key: ${OPENAI_API_KEY}`),
so no other file needs editing. A provider without a key simply does not work.

### B) From the web UI

As administrator, choose the provider and model under **Settings** and type the
key. The container sets `GREGAL_PERSIST_KEYS=1`, so the key is saved in the
volume (`/data/.config/gregal-jocs/auth.yaml`, mode 0600) and survives restarts
and updates. A key in `.env` takes priority over a saved one.

### Choosing the model

The default role is OpenAI `gpt-4.1-mini`. To use another provider, choose it in
the web UI (Settings) or edit `/data/.config/gregal-jocs/config.yaml`
(`roles.chat.provider` and `roles.chat.model`). The **Arena** plays with the
chat model and the **league** narrates with the short-call model.

### Good practice with keys

- Never put them in the repository or in command lines (`ps` and shell history
  expose them). `.env` is already in `.gitignore`.
- Use keys with a **spending limit** at the provider: the server spends them
  every time someone plays.
- To rotate one: change it in `.env` (or the web UI) and restart.
- If you suspect the **master key** leaked, change `GREGAL_API_TOKEN` in `.env`,
  restart and re-enter the key in the apps.

## 6. Day-to-day operation

```sh
./deploy/jocs/deploy.sh --host user@server --status     # status
./deploy/jocs/deploy.sh --host user@server --logs       # live log
./deploy/jocs/deploy.sh --host user@server --restart    # restart the service
./deploy/jocs/deploy.sh --host user@server --stop       # stop (data is kept)
./deploy/jocs/deploy.sh --host user@server --backup     # backup in deploy/jocs/backups/ on the server
```

If you deployed with `--domain`, add it to these commands too so they also
affect Caddy.

### Backup and restore

`--backup` saves a `.tgz` of the volume on the server with **everything**:
config, API keys, accounts (password hashes) and matches. Treat it as a secret
and download it off the server. To restore, on the server:

```sh
cd ~/gregal-jocs/deploy/jocs
docker compose down
docker run --rm -v gregal-jocs_jocs-data:/d -v "$PWD/backups":/b alpine \
  sh -c 'rm -rf /d/* /d/.[!.]* ; tar xzf /b/jocs-FILE.tgz -C /d'
```

Then run `deploy.sh` again. `.env` (master key, admin, keys set there) is **not**
in the volume: keep a separate copy.

## 7. Troubleshooting

| Symptom | Likely cause |
|---|---|
| `docker: command not found` or cannot reach the daemon | Docker not installed or not running on the server |
| `/jocs/` does not load with a domain | DNS does not point to the server or ports 80/443 are closed (`--logs` shows the Caddy error) |
| Games do not respond / model error | Missing provider key, or the model does not belong to that provider (section 5) |
| 401 from the API | Missing or wrong master key, or the session expired |
| Lost the admin password | Put a new one in `JOCS_ADMIN_PASSWORD` in `.env` and `--restart` |
| `unhealthy` in `--status` | Check `--logs`; the healthcheck requests `/jocs/` internally |

## 8. Without Docker

`./build.sh` produces `dist/gregal-jocs-<os>-<arch>`. It can be run directly:

```sh
GREGAL_API_TOKEN=... GREGAL_PERSIST_KEYS=1 GREGAL_ACCOUNTS_DIR=/var/lib/gregal-jocs/accounts \
  OPENAI_API_KEY=... gregal-jocs --serve --addr 127.0.0.1 --port 8197
```

Put an HTTPS proxy (Caddy, nginx or Cloudflare Tunnel) in front, pointing to
`http://127.0.0.1:8197`. For a systemd service, `deploy/linux/` does the same
for the Gregal product (adapt the name, binary and port).

## 9. Variable reference (`.env`)

| Variable | Purpose |
|---|---|
| `GREGAL_API_TOKEN` | Master key (administrator). Required; generated by `deploy.sh` |
| `GREGAL_PUBLIC_URL` | Public address for links (set by `--domain`) |
| `JOCS_ADMIN`, `JOCS_ADMIN_PASSWORD` | Administrator with username and password |
| `JOCS_BIND`, `JOCS_PORT` | Published interface and port (default `0.0.0.0` and `8197`) |
| `JOCS_DOMAIN` | Caddy domain (set by `--domain`) |
| `*_API_KEY` | Model provider keys (section 5) |
