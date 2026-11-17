# Deploying Gregal Jocs

Only **Docker** (with `docker compose`) is needed on the server.

```sh
# On this machine (http://localhost:8197/jocs/)
./deploy/jocs/deploy.sh --admin aleix

# On a remote server over SSH
./deploy/jocs/deploy.sh --host user@server --admin aleix

# With a domain and automatic HTTPS (the domain must point to the server; ports 80/443 open)
./deploy/jocs/deploy.sh --host user@server --domain jocs.example.com --admin aleix
```

When it finishes it prints the URL, the **master key** and, the first time, the
administrator password. They are kept in `deploy/jocs/.env` on the server.
Models are added from the web UI (Settings) or with `*_API_KEY` variables in `.env`.

- **Update:** run the same command again. It rebuilds the image and keeps the
  data (Docker volume `jocs-data`).
- **Operate:** `deploy.sh --logs`, `--status`, `--restart`, `--stop`,
  `--accounts`, `--invite`, `--backup` (add `--host`/`--domain` as when deploying).
- **Independent of Gregal:** its own server with its own profile; you do not
  need Gregal deployed.
- **Without Docker:** see section 8 of the guide.

Full guide (users, API keys, backups, troubleshooting): [GUIDE.md](GUIDE.md).
