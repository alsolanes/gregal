# Gregal on mobile

Gregal on Android works in two ways:

- **This device** (the default): the app runs the same Go backend as the
  desktop app, inside the phone. You need no computer, and your
  conversations and keys stay on the device.
- **Remote server**: the app connects to `gregal --serve` on another
  computer, with a token or a username and password.

You switch between them from the menu (**Switch server**). Each server keeps
its own conversations, and nothing from one shows up in the other.

This document covers the audit of the app before this work, the design, and
what is left.

## 1. Where we started (Android 0.9.11)

The app was a native client (Kotlin + Compose, ~10k lines) that could only
talk to a Gregal server on another machine:

- **Connection:** host, port and token, or username and password
  (`POST /api/login`), kept in `SharedPreferences`. Every request sends
  `Authorization: Bearer` and `X-Gregal-Session: mobil`, so the phone has
  its own live session on the server and does not take over the desktop's
  main tab.
- **Turns:** the v2 contract (`POST /api/v2/runs` plus polling
  `/api/v2/events` with a cursor), with idempotency keys, re-attach to a
  running turn (`/api/active`), and the legacy SSE path for old servers.
- **Screens:** chat with a collapsible work card, conversations (list,
  resume, delete), #retro, goals, activity (jobs and runs), Office
  documents, GitHub (read-only), undo (checkpoints), graphs, preferences,
  and in-app updates.
- **Background:** a `dataSync` foreground service while a turn runs, and a
  notification when the answer arrives.

Gaps compared with the desktop app:

| Area | Desktop (Electron) | Android 0.9.11 |
| --- | --- | --- |
| Backend | Bundled `gregal --serve`, started and supervised by `desktop/server-manager.js` | None: it needed a computer running Gregal |
| Works offline from a PC | Yes | No |
| Backend crash | Restart, log kept | n/a |
| Turn interrupted by a server restart | Web reconnects; the turn stayed "running" in the log | Waited for the end forever |
| Approval or question while in the background | Desktop notification | No notification: the turn waited until the question expired |
| Provider keys | Kept in the key store | Typed keys were kept in server memory only |

Robustness bugs found on the shared backend while doing this (they affect
every surface, and are fixed here):

- A run that a dying process left half done never got a final event, so a
  client that followed it waited forever.
- `service.lock` counted a recycled PID as a live server. On Android PIDs
  are reused quickly, so the local server would refuse to start
  ("service already running") until the app's data was cleared.
- The event log reader used a 64 KiB line limit, so a single large event
  broke every later read.
- `/api/sessions` marked the default session's conversation as `current`,
  not the conversation of the session that asked.

## 2. Design: the same backend on the device

### Options

| Option | Shared code | Notes |
| --- | --- | --- |
| **Go server as a child process** (chosen) | All of the backend: sessions, run queue, durable events, retries, partial answers, tools, MCP, knowledge, Office | The same model as desktop: `gregal --serve` on loopback with a token. A backend crash cannot take the UI down, and the backend can be restarted. |
| `gomobile bind` / `c-shared` library | The same Go code | The Go runtime lives in the app process: a panic kills the app, the server cannot be restarted cleanly, and it needs a second build mode and a JNI layer to maintain. |
| WebView over the embedded web UI | Backend and web UI | Would replace the native app, which already has phone-specific work (composer, voice, notifications, Office picker, drawer). It could be added later as an extra screen. |

We keep the native Kotlin screens. They already speak the v2 contract, so
on the device they talk to `http://127.0.0.1:<port>` exactly as they talk
to a remote server. The app has one code path for both modes; only how the
server is found changes.

### Packaging

- `android/app/build.gradle.kts` has a `buildGregalBackend` task. It
  cross-compiles the repository's Go module with
  `GOOS=android GOARCH=arm64|amd64` into
  `build/gregal-backend/jniLibs/<abi>/libgregal.so` before the JNI libs are
  merged. Android only allows executing files from the app's native library
  directory, so the binary ships as a "library" and `useLegacyPackaging`
  extracts it to disk.
- With an Android NDK in the SDK, the backend is built with cgo and uses
  the system resolver (Private DNS, VPN). Without one it builds with
  `CGO_ENABLED=0`, and `dns_android.go` sends DNS to the servers in
  `GREGAL_DNS`, or 1.1.1.1 and 8.8.8.8.
- ABIs: `arm64-v8a` (phones) and `x86_64` (emulators). AGP ABI splits
  build one APK per ABI, so a phone downloads only its own backend:
  `app-arm64-v8a-debug.apk` is about 27 MB and `app-x86_64-debug.apk`
  about 28 MB. `app-universal-debug.apk` carries both (about 42 MB). All
  share the `versionCode` and the signing key, so they install over each
  other.
- `-Pgregal.skipBackend` builds a remote-only APK. The app then hides
  "This device".

### Process and storage (`LocalServer.kt`)

The app starts the backend like `server-manager.js` does:

```
libgregal.so --serve --addr 127.0.0.1 --port <p> --token <t>
             --dir <files>/workspace --log-file <files>/logs/backend.log --lifeline
```

| Variable | Value | Why |
| --- | --- | --- |
| `HOME` | `<files>/home` | Config at `~/.config/gregal`, data at `~/.local/share/gregal`. Without it, Go on Android uses `/sdcard`. |
| `GREGAL_DATA_DIR` | `<files>/home/.local/share/gregal` | Sessions, events, jobs, knowledge |
| `GREGAL_AUTH` | `<noBackupFiles>/auth.yaml` | Provider keys, mode 0600, excluded from backups |
| `GREGAL_PERSIST_KEYS` | `1` | A key typed in the app is saved to `auth.yaml` and not only kept in memory (see below) |
| `GREGAL_PLATFORM` | `android` | Mobile defaults for the config |
| `TMPDIR` | `<cache>/tmp` | Go's default `/data/local/tmp` is not writable by apps |
| `TZ`, `LANG` | from the device | |

- **Port and token** are saved in preferences and reused. The port falls
  back to a free one if it is taken. The token is random (192 bits) and
  never leaves the device, because the server only listens on loopback.
- **Lifeline:** the app keeps the backend's stdin open. If the app process
  dies, the pipe closes and `--lifeline` shuts the server down cleanly:
  it cancels runs, releases the lock and removes the descriptor. No orphan
  is left running with the port and the battery. `SIGTERM` does the same.
- **Health:** `ensure()` returns a server that answers `/api/health`, or
  starts one and waits up to 30 s. If the backend process dies first, the
  error includes the tail of its log.
- **Crashes:** the backend is restarted with backoff (1, 2, 4 … 30 s).
  After 5 crashes in 2 minutes the app stops retrying and shows the log on
  the connection screen with a retry button.
- **Adoption:** if the app restarts and the previous backend still answers
  on the saved port with the saved token, the app uses it.

### Mobile config defaults

On Android (`runtime.GOOS == "android"`, or `GREGAL_PLATFORM=android`), the
first `config.yaml` is the normal template with these tools denied:
`bash`, `bash_background`, `browser`, `git_commit` and `gh_pr_create`. A
phone has no development shell or controllable browser, and the workspace
is a private app folder. Chat, documents (Office), web search and fetch,
MCP (remote HTTP servers), knowledge, delegation and plans all work. Anyone
who wants a denied tool back can allow it in `permissions.tools`, as on any
platform.

### Keys

The device is a single-owner server, so `GREGAL_PERSIST_KEYS=1` makes the
`key` action of `/api/providers` behave like the TUI: the secret goes to
`auth.yaml` (0600, in app-private no-backup storage, encrypted at rest by
Android's file-based encryption) and never into `config.yaml`. Shared
servers keep the old behaviour (memory only) unless they set the variable.
Wrapping `auth.yaml` with an Android Keystore key is a possible follow-up;
it would need the backend to read the key from the app at start-up.

### Lifecycle, battery and network

- While a turn runs, the existing `dataSync` foreground service keeps the
  app process, and therefore the backend, at foreground priority. With the
  screen off a long turn keeps going, and the notification links back to
  the app.
- Between turns the app is an ordinary background app. If Android reclaims
  it, the lifeline stops the backend. When you open the app again, the
  backend starts in about a second and the app **resumes the conversation
  it was in**: the conversation name is remembered per device, and
  `/api/sessions` now marks the caller's own conversation as `current`.
- A turn cut off by a process restart is closed by the backend at start-up
  (`CloseOrphanRuns` writes `run_failed`), so the app shows "the server
  restarted, send it again" instead of waiting. Against an older remote
  server that doesn't do this, the app gives up after `GET /runs/{id}`
  returns 404 twice.
- When the agent needs an approval or an answer and the app is not on
  screen, a notification ("Gregal is waiting for you") appears.
- The model's own retries (empty replies, 429/5xx) and partial answers come
  from the shared backend, so they behave exactly as on desktop.
- Network: in local mode the app↔backend link is loopback and never drops.
  The backend retries LLM calls itself. In remote mode the event polling
  retries with backoff for about 2.5 minutes before the turn is shown as
  failed.

### Remote mode

Unchanged in protocol. The connection screen leads with "This device" and
keeps the remote form (host, port, username and password, or a token)
behind "Or connect to a server on another computer". It is open
automatically for people upgrading from a version that had a remote server
saved, so their setup keeps working on first launch.

Switching server clears everything on screen (messages, event cursor,
sessions, pending approvals) so nothing from one server leaks into the
other. Choosing a remote server stops the local backend. Conversations live
on each server, so they are listed and resumed per server.

### Updates

The backend is inside the APK, so the in-app updater (manifest, download,
SHA-256 check, installer) updates the app and the backend together, and the
two never drift. `node scripts/release.cjs apk` builds with
`assembleDebug`, which now also needs Go and preferably an NDK on the
release machine, and attaches the per-ABI APKs, the universal one and the
manifest `gregal-android-version.json`. The manifest's `apks` object has
one entry per ABI (`downloadUrl`, `sha256`, `sizeBytes`), and the app takes
the first of `Build.SUPPORTED_ABIS` listed there. If none matches, or the
manifest has no `apks`, it uses the top-level fields, which point to the
universal APK. CI installs Go in the Android job.

## 3. Verified

- `go test ./...`, the web JS tests, and `gradlew testDebugUnitTest`.
- On an Android 16 emulator (x86_64):
  - an upgrade over 0.9.10 keeps the saved remote server;
  - "Use Gregal on this device" starts the bundled backend;
  - a key set through the app persists across restarts;
  - an agent turn writes a file in the app workspace, with `bash`
    correctly denied;
  - killing the app process also stops the backend, and reopening the app
    restarts it and resumes the same conversation;
  - switching to a remote `gregal --serve` on the host (`10.0.2.2`) works
    with a token, its conversations are separate, and switching back
    brings the phone's conversation back;
  - a backend killed with `kill -9` mid-turn comes back: the stale lock
    is recovered and the interrupted turn is marked as failed.

## 4. Follow-ups

1. Hide tools whose policy is `deny` from the model's tool list. Today the
   model sometimes tries `bash` once and adapts after the denial.
2. Run scheduled jobs when the app is closed (WorkManager waking the
   backend).
4. Keystore-wrapped `auth.yaml`.
5. Remote workspaces from the phone: delegate a turn's workspace to a
   remote server while the conversation stays on the device.
6. Several saved remote servers (profiles) instead of one.
