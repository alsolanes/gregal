// Routes remain stable when a page is renamed or opened from another screen.
export const PREFS_PAGES = [
  { id: 'general', keywords: 'advanced avançat shortcuts dreceres updates actualitzacions' },
  { id: 'appearance', keywords: 'extras extres appearance aparença personalization personalització' },
  { id: 'models', keywords: 'models i agent providers proveïdors' },
  { id: 'permissions', keywords: 'models i agent permissions permisos approvals aprovacions reviewer revisor' },
  { id: 'skills', keywords: 'models i agent skills habilitats specialists especialistes' },
  { id: 'connections', keywords: 'integrations integracions connections connexions' },
  { id: 'plugins', keywords: 'plugins features funcions advanced avançat experimental kanban' },
  { id: 'server', admin: true, keywords: 'advanced avançat server servidor administration administració' },
  { id: 'connected', parent: 'connections', capability: 'connected' },
  { id: 'calendar', parent: 'connections', capability: 'calendar' },
  { id: 'chrome', parent: 'connections' },
  { id: 'word', parent: 'connections' },
  { id: 'identity', parent: 'server', admin: true, keywords: 'brandings branding brands identitat logo colors' },
  { id: 'accounts', parent: 'server', admin: true, capability: 'accounts' },
  { id: 'powers', parent: 'server', admin: true, capability: 'powers' },
];

const aliases = { agent: 'models', integrations: 'connections', extras: 'appearance', advanced: 'general', voice: 'plugins', brands: 'identity' };
export function resolvePrefsPage(value) {
  const id = aliases[value] || value;
  return PREFS_PAGES.some(page => page.id === id) ? id : 'general';
}
export function prefsNavPage(value) {
  const id = resolvePrefsPage(value);
  return PREFS_PAGES.find(page => page.id === id)?.parent || id;
}
export function prefsPageAvailable(id, capabilities = {}) {
  const page = PREFS_PAGES.find(page => page.id === id);
  return !!page && (!page.admin || !!capabilities.admin) && (!page.capability || !!capabilities[page.capability]);
}
export function normalizePrefsSearch(value) {
  return String(value || '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
}
export function searchPrefsPages(query, pages) {
  const q = normalizePrefsSearch(query).trim();
  if (!q) return [];
  return pages.filter(page => normalizePrefsSearch([page.title, page.description, page.text,
    PREFS_PAGES.find(meta => meta.id === page.id)?.keywords].filter(Boolean).join(' ')).includes(q));
}
