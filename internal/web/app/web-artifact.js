export function htmlArtifact(text) {
  const source = String(text || '');
  const fenced = [...source.matchAll(/```html\s*\n([\s\S]*?)```/gi)];
  const html = fenced.length ? fenced[fenced.length - 1][1].trim() : source.trim();
  if (html.length > 200000 || !/^<!doctype html\b|^<html\b/i.test(html) || !/<\/html\s*>\s*$/i.test(html)) return null;
  return html;
}

export function isolatedHTML(html) {
  return '<!doctype html><meta http-equiv="Content-Security-Policy" content="default-src \'none\'; script-src \'unsafe-inline\'; style-src \'unsafe-inline\'; img-src data:; font-src data:; connect-src \'none\'; form-action \'none\'; base-uri \'none\'; frame-src \'none\'">' + html;
}

// A saved website commonly has sibling CSS and JavaScript files. Inline those
// through the authorized file API before opening the existing isolated preview.
// Remote URLs, parent traversal and network access remain disabled.
export async function bundleLocalHTML(html, path, read) {
  const doc = new DOMParser().parseFromString(html, 'text/html');
  const base = String(path).replace(/\\/g,'/').split('/').slice(0,-1);
  let bytes = html.length;
  async function asset(src) {
    if (!src || /^(?:[a-z][\w+.-]*:|\/)/i.test(src)) throw new Error('Preview needs local CSS and JavaScript files.');
    const parts = decodeURIComponent(src.split(/[?#]/)[0]).split('/');
    if(parts.some(p=>p==='..'||p.includes('\\'))) throw new Error('Preview asset must be inside the website folder.');
    const file = [...base,...parts.filter(p=>p&&p!=='.')].join('/');
    const text = await read(file); bytes += text.length;
    if(bytes>195000) throw new Error('This website is too large for the inline preview. Open its local development server.');
    return text;
  }
  for(const link of doc.querySelectorAll('link[rel="stylesheet"][href]')) {
    const style=doc.createElement('style');style.textContent=(await asset(link.getAttribute('href'))).replace(/<\/style/gi,'<\\/style');link.replaceWith(style);
  }
  for(const script of doc.querySelectorAll('script[src]')) {
    script.textContent=(await asset(script.getAttribute('src'))).replace(/<\/script/gi,'<\\/script');script.removeAttribute('src');
  }
  return '<!doctype html>\n'+doc.documentElement.outerHTML;
}
