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
