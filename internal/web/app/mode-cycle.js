// The Tab shortcut cycles through the three modes of the composer picker:
// Ask (chat), Plan and Act. Advanced modes (goal, debug) never enter this
// cycle; from one of them Tab goes back to the first mode.
window.gregalModeCycle = {
  next(current, modes = ['chat', 'plan', 'act']) {
    if (!modes.length) return current;
    const index = modes.indexOf(current);
    return modes[(index < 0 ? 0 : index + 1) % modes.length];
  },
};
