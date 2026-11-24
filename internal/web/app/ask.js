// gregalAsk(label, value): demana una línia de text en un diàleg de la pàgina.
// prompt() no existeix a Electron (l'app d'escriptori i el panell del
// navegador de l'app): hi llança un error i el botó no fa res.
// Torna el text, o null si la persona cancel·la (Cancel·la o Escape).
(function () {
  const T = (key, en) => {
    const v = typeof globalThis.gregalT === 'function' ? globalThis.gregalT(key) : '';
    return v && v !== key ? v : en;
  };
  function ask(label, value) {
    const el = tag => document.createElement(tag);
    const d = el('dialog'), form = el('form'), text = el('label'), input = el('input'), row = el('div'), ok = el('button'), no = el('button');
    d.className = 'ask-dialog';
    form.className = 'ask-form';
    row.className = 'ask-actions';
    text.textContent = label;
    input.value = value || '';
    input.autocomplete = 'off';
    no.type = 'button';
    no.className = 'ghost';
    no.textContent = T('ask.cancel', 'Cancel');
    ok.type = 'submit';
    ok.className = 'ghost';
    ok.textContent = T('ask.ok', 'OK');
    text.append(input);
    row.append(no, ok);
    form.append(text, row);
    d.append(form);
    document.body.append(d);
    d.showModal();
    input.focus();
    input.select();
    return new Promise(done => {
      const end = v => { d.close(); d.remove(); done(v); };
      form.onsubmit = e => { e.preventDefault(); end(input.value); };
      no.onclick = () => end(null);
      d.oncancel = e => { e.preventDefault(); end(null); };
      // L'Escape és només del diàleg: la pàgina en té d'altres (la finestra del Kanban es tancaria).
      d.onkeydown = e => { if (e.key === 'Escape') e.stopPropagation(); };
    });
  }
  if (typeof module === 'object') module.exports = { ask };
  else window.gregalAsk = ask;
})();
