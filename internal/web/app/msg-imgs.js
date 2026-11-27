// L'adjunt d'un missatge: les imatges van a sobre del text, mai a sota.
// El text és el comentari del que has adjuntat, no una capçalera. Pur:
// també el carreguen els tests de node.
(function (root) {
  const esc = s => String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  const html = list => (Array.isArray(list) && list.length
    ? '<div class="msg-imgs">' + list.map(u => '<img src="' + esc(u) + '" alt="">').join('') + '</div>'
    : '');
  const api = { html };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.gregalMsgImgs = api;
})(typeof window !== 'undefined' ? window : globalThis);
