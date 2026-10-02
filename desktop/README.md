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

Keep configuration files and credentials private. See the
[Getting Started guide](../docs/getting-started.md) for provider setup and data
handling.

## Build Packages

Build the Windows portable executable from Windows:

```sh
npm ci
npm run dist-win
```

Build the Linux AppImage:

```sh
npm ci
npm run dist-linux
```

The Linux package and runtime should be tested on a Linux host before
distribution. Desktop packaging does not by itself validate signing, updates,
or release readiness.
