// One small outline icon set for the shell; labels and handlers stay intact.
(() => {
  const paths = {
    agent:'M4 5h16v14H4z M8 9l3 3-3 3 M13 15h3',
    objectius:'M12 3a9 9 0 1 0 9 9 M12 7a5 5 0 1 0 5 5 M12 12l8-8 M16 4h4v4',
    activitat:'M4 6h16 M4 12h16 M4 18h10',
    github:'M7 5v10a4 4 0 0 0 4 4h6 M17 5v4a4 4 0 0 1-4 4H7',
    office:'M6 3h8l4 4v14H6z M14 3v5h4 M9 12h6 M9 16h6',
    vents:'M3 8h10a3 3 0 1 0-3-3 M3 12h15a3 3 0 1 1-3 3 M3 16h7',
    programat:'M12 8v4l3 2 M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18',
    automatitzacions:'M12 8v4l3 2 M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18 M19 3l2 2',
    mcp:'M8 3v5 M16 3v5 M6 8h12v3a6 6 0 0 1-12 0z M12 17v4',
    canvis:'M8 4v16 M4 8l4-4 4 4 M16 20V4 M12 16l4 4 4-4',
    fitxers:'M3 7V5h7l2 2h9v13H3z',
    knowledge:'M5 4h10a3 3 0 0 1 3 3v13H8a3 3 0 0 1-3-3z M5 17a3 3 0 0 1 3-3h10 M9 8h5',
    terminal:'M4 5h16v14H4z M7 9l3 3-3 3 M13 15h4',
    grafs:'M12 7v4 M5 15v-4h14v4 M9 3h6v4H9z M2 15h6v6H2z M16 15h6v6h-6z',
    workshop:'M8 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6 M16 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6 M3 20c0-3 2.2-5 5-5s5 2 5 5 M11 20c0-3 2.2-5 5-5s5 2 5 5',
    navegador:'M3 12h18 M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18 M12 3c5 5 5 13 0 18-5-5-5-13 0-18',
    menu:'M4 5h16v14H4z M9 5v14',
    inspToggle:'M4 5h16v14H4z M15 5v14',
    moreBtn:'M5 11a1 1 0 1 0 0 2 1 1 0 0 0 0-2 M12 11a1 1 0 1 0 0 2 1 1 0 0 0 0-2 M19 11a1 1 0 1 0 0 2 1 1 0 0 0 0-2',
    model:'M8 4h8v4h4v8h-4v4H8v-4H4V8h4z M9 9h6v6H9z',
    plus:'M12 5v14 M5 12h14',
    // Capçalera de Codi i barra de pestanyes: abans eren glifs de text (±, ▤, ▣, ⎇＋).
    diff:'M12 4v7 M8.5 7.5h7 M8.5 17h7 M5 3h14v18H5z',
    files:'M3 7V5h7l2 2h9v13H3z',
    preview:'M3 5h18v14H3z M3 9h18 M6 7h.01 M8.5 7h.01',
    branchPlus:'M6 4v12 M6 16a2 2 0 1 0 0 4 2 2 0 0 0 0-4 M6 8a8 8 0 0 0 8 0 M14 8a2 2 0 1 0 0-4 2 2 0 0 0 0 4 M18 14v6 M15 17h6',
    close:'M7 7l10 10 M17 7L7 17',
    search:'M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14 M20 20l-4-4',
    sun:'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v2 M12 20v2 M4.9 4.9l1.4 1.4 M17.7 17.7l1.4 1.4 M2 12h2 M20 12h2 M4.9 19.1l1.4-1.4 M17.7 6.3l1.4-1.4',
    moon:'M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z',
    sliders:'M4 6h9 M17 6h3 M4 12h3 M11 12h9 M4 18h11 M19 18h1 M15 4v4 M9 10v4 M17 16v4',
    // Grups de Preferències: abans eren glifs de text (⚙ ◐ ◇ ✚ 🔊 ✧ ⇄ ⋯),
    // cadascun d'un estil i mida diferents, i l'altaveu era un emoji en color.
    gear:'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6 M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z',
    palette:'M12 3a9 9 0 1 0 0 18c1 0 1.5-.8 1.5-1.6 0-.5-.2-.9-.5-1.2-.3-.4-.5-.7-.5-1.2 0-.9.7-1.6 1.6-1.6H16a5 5 0 0 0 5-5c0-4-4-7.4-9-7.4 M7.5 11.5h.01 M9.5 7.5h.01 M14.5 7.5h.01 M17 11h.01',
    puzzle:'M10 4a2 2 0 1 1 4 0v1h4v4h1a2 2 0 1 1 0 4h-1v5h-5v-1a2 2 0 1 0-4 0v1H4v-5h1a2 2 0 1 0 0-4H4V5h6z',
    speaker:'M11 5L6 9H3v6h3l5 4z M15.5 8.5a5 5 0 0 1 0 7 M18.5 5.5a9 9 0 0 1 0 13',
    sparkles:'M12 3l1.8 4.7L18.5 9.5l-4.7 1.8L12 16l-1.8-4.7L5.5 9.5l4.7-1.8z M19 15l.8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8z',
    plug:'M9 3v5 M15 3v5 M7 8h10v3a5 5 0 0 1-10 0z M12 16v5',
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
  // Botons de la capçalera amb glif de text: el primer node de text es
  // canvia per la icona i es conserven els fills (la xapa de canvis).
  for (const [sel, icon] of [['#top [data-view="canvis"]', 'diff'], ['#top [data-view="fitxers"]', 'files'], ['#previewToggle', 'preview']]) {
    const button = document.querySelector(sel);
    const text = button && [...button.childNodes].find(n => n.nodeType === 3 && n.textContent.trim());
    if (text) { text.remove(); button.insertAdjacentHTML('afterbegin', svg(icon)); }
  }
  // Perquè altres mòduls (pestanyes, paleta) facin servir el mateix joc.
  window.gregalIcon = name => (paths[name] ? svg(name) : '');
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
