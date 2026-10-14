// Hydrate before any UI code reads localStorage. Desktop preferences are
// independent of the ephemeral port chosen by the managed backend.
(() => {
  const bridge = window.gregalDesktop;
  if (!bridge?.preferences) return;
  const saved = bridge.preferences;
  const keys = new Set(bridge.preferenceKeys);
  const originalSet = Storage.prototype.setItem;
  const originalRemove = Storage.prototype.removeItem;
  const originalClear = Storage.prototype.clear;
  try {
    for (const [key, value] of Object.entries(saved)) originalSet.call(localStorage, key, value);
    // Migrate existing preferences from this origin on the first launch.
    for (const key of keys) {
      const value = localStorage.getItem(key);
      if (value !== null && !(key in saved)) bridge.savePreference(key, value);
    }
    Storage.prototype.setItem = function (key, value) {
      originalSet.call(this, key, value);
      if (this === localStorage && keys.has(String(key))) bridge.savePreference(String(key), String(value));
    };
    Storage.prototype.removeItem = function (key) {
      originalRemove.call(this, key);
      if (this === localStorage && keys.has(String(key))) bridge.savePreference(String(key), null);
    };
    Storage.prototype.clear = function () {
      originalClear.call(this);
      if (this === localStorage) for (const key of keys) bridge.savePreference(key, null);
    };
    // The main process changed a preference on its own (e.g. the floating
    // weathervane was hidden from its window): mirror it without echoing.
    if (typeof bridge.onPreferenceChanged === 'function') bridge.onPreferenceChanged((key, value) => {
      if (!keys.has(key)) return;
      if (value === null) originalRemove.call(localStorage, key);
      else originalSet.call(localStorage, key, value);
      document.dispatchEvent(new CustomEvent('gregal:fun-prefs'));
    });
  } catch (_) { /* Browser storage unavailable: retain the normal UI defaults. */ }
})();
