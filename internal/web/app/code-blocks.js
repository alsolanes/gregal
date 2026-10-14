// Final answers are re-rendered on the server (goldmark + chroma) and the
// sanitizer does not allow buttons, so their code blocks arrived as a bare
// <pre> without the language label and the Copy button the live render
// has. This wraps them in the same .codeblock header; the existing click
// handler on #main does the copying, and data-i18n keeps the label in the
// current language.
(function () {
  const T = (key, fallback) => {
    try { const v = window.gregalT && window.gregalT(key); if (v && v !== key) return v; } catch (e) {}
    return fallback;
  };
  function decorate(pre) {
    if (pre.dataset.decorated || pre.classList.contains('lang-gregal-plot')) return;
    if (pre.parentElement && pre.parentElement.classList.contains('codeblock')) return;
    pre.dataset.decorated = '1';
    const lang = ([...pre.classList].find(c => c.startsWith('lang-')) || '').slice(5);
    const box = document.createElement('div');
    box.className = 'codeblock';
    const head = document.createElement('div');
    head.className = 'codehead';
    const name = document.createElement('span');
    name.textContent = lang || T('md.code', 'code');
    const copy = document.createElement('button');
    copy.type = 'button';
    copy.className = 'copybtn';
    copy.dataset.i18n = 'md.copy';
    copy.textContent = T('md.copy', 'Copy');
    head.append(name, copy);
    pre.replaceWith(box);
    box.append(head, pre);
  }
  function scan(root) {
    (root || document).querySelectorAll('#main .msg .body pre.codeblock').forEach(decorate);
  }
  const main = document.getElementById('main');
  if (!main) return;
  let queued = false;
  new MutationObserver(() => {
    if (queued) return;
    queued = true;
    requestAnimationFrame(() => { queued = false; scan(main); });
  }).observe(main, { childList: true, subtree: true });
  scan(main);
})();
