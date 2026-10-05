const RELEASE_PAGE = 'https://github.com/alsolanes/gregal/releases';

function createUpdater({ app, platform = process.platform, env = process.env, resourcesPath = process.resourcesPath, loadAutoUpdater, openExternal, stopBackend = async () => {} }) {
  const currentVersion = app.getVersion();
  let status = { state: 'idle', currentVersion, version: '', progress: null, mode: 'automatic', reason: '' };
  let autoUpdater = null;
  let checkPromise = null;
  let downloadPromise = null;
  let installPromise = null;
  let installationRequested = false;
  let pageOpener = openExternal;
  const listeners = new Set();

  function publish(patch) {
    status = { ...status, ...patch, currentVersion };
    const snapshot = { ...status };
    for (const listener of listeners) {
      try { listener(snapshot); } catch (error) { console.error('[gregal] update listener:', error); }
    }
    return snapshot;
  }

  if (env.GREGAL_NO_UPDATE) {
    status = { ...status, state: 'unsupported', mode: 'manual', reason: 'disabled' };
  } else if (!app.isPackaged) {
    status = { ...status, state: 'unsupported', mode: 'manual', reason: 'development' };
  } else if (env.PORTABLE_EXECUTABLE_FILE) {
    status = { ...status, state: 'unsupported', mode: 'portable', reason: 'portable' };
  } else if (platform !== 'win32') {
    status = { ...status, state: 'unsupported', mode: 'manual', reason: 'unsupported-platform' };
  } else if (!resourcesPath || !require('node:fs').existsSync(require('node:path').join(resourcesPath, 'app-update.yml'))) {
    status = { ...status, state: 'unsupported', mode: 'manual', reason: 'missing-metadata' };
  } else {
    try {
      autoUpdater = (loadAutoUpdater || (() => require('electron-updater').autoUpdater))();
      autoUpdater.autoDownload = false;
      autoUpdater.autoInstallOnAppQuit = false;
      autoUpdater.allowPrerelease = false;
      autoUpdater.allowDowngrade = false;
      autoUpdater.on('checking-for-update', () => publish({ state: 'checking', version: '', progress: null, reason: '' }));
      autoUpdater.on('update-available', info => publish({ state: 'available', version: info?.version || '', progress: 0, reason: '' }));
      autoUpdater.on('update-not-available', () => publish({ state: 'up-to-date', version: '', progress: null, reason: '' }));
      autoUpdater.on('download-progress', progress => publish({ state: 'downloading', version: status.version, progress: Math.max(0, Math.min(100, Number(progress?.percent) || 0)), reason: '' }));
      autoUpdater.on('update-downloaded', info => publish({ state: 'ready', version: info?.version || status.version, progress: 100, reason: '' }));
      autoUpdater.on('error', error => publish({ state: 'error', reason: error?.message || 'Update check failed.' }));
    } catch (error) {
      autoUpdater = null;
      status = { ...status, state: 'unsupported', mode: 'manual', reason: 'updater-unavailable' };
    }
  }

  return {
    getStatus: () => ({ ...status }),
    onStatus(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    async check() {
      if (!autoUpdater) return { ...status };
      if (installPromise || installationRequested) return { ...status };
      if (['available', 'downloading', 'ready', 'installing'].includes(status.state)) return { ...status };
      if (checkPromise) return checkPromise;
      checkPromise = (async () => {
        publish({ state: 'checking', reason: '' });
        try { await autoUpdater.checkForUpdates(); }
        catch (error) { publish({ state: 'error', reason: error?.message || 'Update check failed.' }); }
        return { ...status };
      })().finally(() => { checkPromise = null; });
      return checkPromise;
    },
    async download() {
      if (!autoUpdater || status.state !== 'available' || !status.version) return { ...status };
      if (downloadPromise) return downloadPromise;
      downloadPromise = (async () => {
        publish({ state: 'downloading', progress: 0, reason: '' });
        try { await autoUpdater.downloadUpdate(); }
        catch (error) { publish({ state: 'error', reason: error?.message || 'Update download failed.' }); }
        return { ...status };
      })().finally(() => { downloadPromise = null; });
      return downloadPromise;
    },
    async install() {
      if (!autoUpdater || status.state !== 'ready') return { ...status };
      if (installPromise || installationRequested) return installPromise || { ...status };
      installationRequested = true;
      publish({ state: 'installing', reason: '' });
      installPromise = (async () => {
        try {
          await stopBackend();
          autoUpdater.quitAndInstall(false, true);
        } catch (error) { installationRequested = false; publish({ state: 'error', reason: error?.message || 'Could not install update.' }); }
        return { ...status };
      })().finally(() => { installPromise = null; });
      return installPromise;
    },
    async openPage() { if (pageOpener) await pageOpener(RELEASE_PAGE); return { ...status }; },
  };
}

module.exports = { createUpdater, RELEASE_PAGE };
