// One small outline icon set for the shell; labels and handlers stay intact.
(() => {
  const paths = {
    agent:'M4 5h16v14H4z M8 9l3 3-3 3 M13 15h3',
    objectius:'M12 3a9 9 0 1 0 9 9 M12 7a5 5 0 1 0 5 5 M12 12l8-8 M16 4h4v4',
    activitat:'M4 6h16 M4 12h16 M4 18h10',
    github:'M7 5v10a4 4 0 0 0 4 4h6 M17 5v4a4 4 0 0 1-4 4H7',
    office:'M6 3h8l4 4v14H6z M14 3v5h4 M9 12h6 M9 16h6',
    programat:'M12 8v4l3 2 M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18',
    mcp:'M8 3v5 M16 3v5 M6 8h12v3a6 6 0 0 1-12 0z M12 17v4',
    canvis:'M8 4v16 M4 8l4-4 4 4 M16 20V4 M12 16l4 4 4-4',
    fitxers:'M3 7V5h7l2 2h9v13H3z',
    terminal:'M4 5h16v14H4z M7 9l3 3-3 3 M13 15h4',
    grafs:'M12 7v4 M5 15v-4h14v4 M9 3h6v4H9z M2 15h6v6H2z M16 15h6v6h-6z',
    navegador:'M3 12h18 M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18 M12 3c5 5 5 13 0 18-5-5-5-13 0-18',
    menu:'M4 5h16v14H4z M9 5v14',
    inspToggle:'M4 5h16v14H4z M15 5v14',
    moreBtn:'M5 11a1 1 0 1 0 0 2 1 1 0 0 0 0-2 M12 11a1 1 0 1 0 0 2 1 1 0 0 0 0-2 M19 11a1 1 0 1 0 0 2 1 1 0 0 0 0-2',
    model:'M8 4h8v4h4v8h-4v4H8v-4H4V8h4z M9 9h6v6H9z',
    plus:'M12 5v14 M5 12h14',
  };
  const svg = name => '<svg class="ui-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="' + paths[name] + '"/></svg>';
  document.querySelectorAll('.side-view[data-view]').forEach(button => {
    const slot = button.querySelector('.nav-icon');
    if (slot && paths[button.dataset.view]) slot.innerHTML = svg(button.dataset.view);
  });
  for (const id of ['menu', 'inspToggle', 'moreBtn']) {
    const button = document.getElementById(id);
    if (button) button.innerHTML = svg(id);
  }
  for (const [id, icon] of [['newchat','plus'], ['newproject','fitxers']]) {
    const slot = document.querySelector('#' + id + ' > span:first-child');
    if (slot) slot.innerHTML = svg(icon);
  }
  const pill = document.getElementById('modelPill');
  if (pill) {
    pill.firstChild.textContent = '';
    pill.insertAdjacentHTML('afterbegin', svg('model'));
  }
})();
