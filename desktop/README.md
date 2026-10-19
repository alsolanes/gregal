# Gregal Desktop

The desktop client uses Electron and can start a local Gregal backend. Run it
from this directory with Node.js 24, npm and Go 1.27.1 or newer:

```sh
npm ci
npm start
```

`npm start` builds the backend for the current platform before opening the
desktop window. The package lockfile pins the JavaScript dependencies.

## Configuration

By default, the client uses Gregal's normal configuration. Set `GREGAL_CONFIG`
to use a specific existing configuration file. `GREGAL_DIR` selects the
initial project directory. Set `GREGAL_URL` to connect to an existing Gregal
server instead of starting a managed local backend.

For example, in PowerShell:

```powershell
$env:GREGAL_CONFIG = "$HOME\gregal\config.yaml"
npm start
```

With a managed backend, the agent's `browser` tool drives an "Agent browser"
pane inside the app instead of opening an external Chrome window
(`browser-bridge.js`: loopback-only HTTP, random port and token passed to the
backend as `GREGAL_APP_BROWSER`). With `GREGAL_URL` there is no bridge and the
backend uses headless Chrome. See "Agent browser" in the
[manual](../docs/manual.md).

Keep configuration files and credentials private. See the
[Getting Started guide](../docs/getting-started.md) for provider setup and data
handling.

## Build Packages

Build the Windows installer and portable executable from Windows:

```sh
npm ci
npm run dist-win
npm run verify-release
```

Build the Linux AppImage:

```sh
npm ci
npm run dist-linux
```

The Linux package and runtime should be tested on a Linux host before
distribution. Desktop packaging does not by itself validate signing, updates,
or release readiness.

## Updates

Install the Windows app with `Gregal-Setup-<version>-x64.exe` for updates
inside the app. Open **Preferences → App updates** to check, download a new
version, and restart to install it. Checking also runs at launch. Downloads
and installation require an explicit action; closing the app does not
silently install an update. Save your work before restarting.

The portable app offers **Open downloads** instead. Close it and replace the
portable executable with the new version from the official release page.
Its updater never installs an NSIS package over the portable app.

Update discovery requires a published GitHub release in `alsolanes/gregal`.
Keep the installer, its `.blockmap`, and `latest.yml` from the same build.
`npm run verify-release` checks the version, installer filename, download size,
and SHA512 checksum before distribution. A draft release is not visible to
installed apps. The bundled backend updates with the desktop application;
an external server is managed independently.

Backend log: the app appends the managed backend's output to
`gregal-backend.log` in its data folder (`%APPDATA%/Gregal` on Windows,
rotated to `.old` past 2 MB). When a run stops unexpectedly, that file —
not the UI — holds the real provider error (credentials redacted).


## Web views and sessions

Preview keeps page tabs separately for each conversation. Switching conversations
restores the selected page without reloading its frame, so page input and running
scripts remain intact. New local website addresses can open in additional tabs;
use **+** to open another manually. Tab addresses and generated documents survive
a page reload in the current app window; page form input is preserved while
switching sessions, rather than across reloads.

The agent browser also keeps an independent page and control connection for each
live session. A background agent can continue using its own browser without
bringing another project's page into the selected conversation. Closing or
releasing one session's browser does not release the others.
