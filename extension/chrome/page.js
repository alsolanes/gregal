// Funcions que s'executen dins la pestanya (chrome.scripting.executeScript).
// Han de ser autònomes: no poden fer servir res de fora d'elles mateixes.

/** Què és la pestanya i què se'n pot llegir sense descarregar res. */
export function pageInfo() {
  const u = new URL(location.href);
  const info = { url: location.href, title: document.title, kind: 'web', id: '', gid: '', range: '', selection: '' };
  let m;
  if (u.host === 'docs.google.com' && (m = u.pathname.match(/^\/document\/d\/([^/]+)/))) {
    info.kind = 'docs'; info.id = m[1];
  } else if (u.host === 'docs.google.com' && (m = u.pathname.match(/^\/spreadsheets\/d\/([^/]+)/))) {
    info.kind = 'sheets'; info.id = m[1];
    info.gid = (u.hash.match(/gid=(\d+)/) || u.search.match(/gid=(\d+)/) || [])[1] || '0';
    const box = document.querySelector('#t-name-box, input.waffle-name-box');
    info.range = box ? box.value : '';
  } else if (u.host === 'script.google.com') {
    info.kind = 'script';
  }
  info.title = info.title.replace(/ - Google (Docs|Sheets|Fulls de càlcul|Documents)$/, '').replace(/ - Projecte d'Apps Script$| - Apps Script$/, '');
  const sel = String(window.getSelection ? window.getSelection() : '').trim();
  if (sel) info.selection = sel.slice(0, 20000);
  if (info.kind === 'web') {
    const main = document.querySelector('main, article, [role=main]') || document.body;
    info.text = (main.innerText || '').replace(/\n{3,}/g, '\n\n').slice(0, 20000);
  }
  return info;
}

/** Codi de l'editor d'Apps Script (Monaco): tots els fitxers oberts, o les línies visibles. */
export function scriptFiles() {
  const out = [];
  const ed = window.monaco && window.monaco.editor;
  if (ed && ed.getModels) {
    for (const m of ed.getModels()) {
      const name = decodeURIComponent(String(m.uri.path || m.uri).split('/').pop() || 'Codi.gs');
      out.push({ name, code: m.getValue() });
    }
  }
  if (!out.length) {
    const lines = [...document.querySelectorAll('.view-lines .view-line')]
      .sort((a, b) => parseFloat(a.style.top) - parseFloat(b.style.top))
      .map((l) => l.innerText.replace(/ /g, ' '));
    const tab = document.querySelector('[aria-selected="true"] [data-file-name], [aria-selected="true"]');
    if (lines.length) out.push({ name: (tab && tab.innerText.trim()) || 'Codi.gs', code: lines.join('\n'), partial: true });
  }
  return out;
}

/**
 * Posa el text a la pàgina. Docs i Sheets no tenen el text al DOM: se'ls
 * dona com un «enganxa» (és el que entenen). A Apps Script, l'editor Monaco
 * (al cursor o substituint el fitxer). En una web, al camp que tingui el focus.
 * Torna true si la pàgina ho ha acceptat; si no, l'app ho deixa al porta-retalls.
 */
export function insertText(text, kind, replaceAll) {
  const paste = (el) => {
    if (!el) return false;
    const dt = new DataTransfer();
    dt.setData('text/plain', text);
    const ev = new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true });
    return !el.dispatchEvent(ev); // cancel·lat = la pàgina l'ha gestionat
  };
  if (kind === 'docs') {
    const frame = document.querySelector('iframe.docs-texteventtarget-iframe');
    const doc = frame && frame.contentDocument;
    const el = doc && (doc.querySelector('[contenteditable=true]') || doc.body);
    return paste(el);
  }
  if (kind === 'sheets') {
    return paste(document.activeElement && document.activeElement !== document.body ? document.activeElement
      : document.querySelector('.cell-input, #waffle-rich-text-editor'));
  }
  if (kind === 'script') {
    const ed = window.monaco && window.monaco.editor;
    const editors = ed && ed.getEditors ? ed.getEditors() : [];
    const editor = editors.find((e) => e.hasTextFocus && e.hasTextFocus()) || editors[0];
    if (!editor) return false;
    const model = editor.getModel();
    if (replaceAll) {
      editor.executeEdits('gregal', [{ range: model.getFullModelRange(), text }]);
    } else {
      editor.executeEdits('gregal', [{ range: editor.getSelection(), text, forceMoveMarkers: true }]);
    }
    editor.focus();
    return true;
  }
  const el = document.activeElement;
  if (el && (el.isContentEditable || /^(TEXTAREA|INPUT)$/.test(el.tagName))) {
    el.focus();
    return document.execCommand('insertText', false, text);
  }
  return false;
}
