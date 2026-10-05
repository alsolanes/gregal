// The Tab shortcut cycles only through the two primary composer choices.
// Advanced modes remain selectable in their disclosure and never enter this cycle.
window.gregalModeCycle = {
  next(current, modes = ['code', 'chat']) {
    if (!modes.length) return current;
    const index = modes.indexOf(current);
    return modes[(index < 0 ? 0 : index + 1) % modes.length];
  },
};
