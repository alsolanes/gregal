// Published content has its own management screen, separate from settings.
import { t as T } from './i18n.js';
import { shares } from './export.js';
import { webs } from './webs.js';

const $ = id => document.getElementById(id);
const G = () => window.gregal;
export const publications = {
  active: 'links',
  back: null,
  async load(previousView) {
    const page = $('publicacionsPage'), body = $('publicationsBody');
    if (!page || !body) return;
    if (previousView && previousView !== 'publicacions') this.back = previousView;
    $('publicationsBack').textContent = T('library.back');
    $('publicationsBack').onclick = () => G().setView(this.back || (window.gregalProduct === 'vents' ? 'vents' : 'agent'));
    const tabs = document.createElement('div');
    tabs.className = 'pf-seg publications-tabs'; tabs.setAttribute('role', 'tablist'); tabs.setAttribute('aria-label', T('library.title'));
    for (const [id, key] of [['links', 'share.section'], ['sites', 'webs.section']]) {
      const button = document.createElement('button'); button.type = 'button'; button.id = 'publicationsTab-' + id;
      button.setAttribute('role', 'tab'); button.setAttribute('aria-controls', 'publicationsPane-' + id);
      button.textContent = T(key); button.onclick = () => this.select(id);
      tabs.append(button);
    }
    body.replaceChildren(tabs);
    for (const [id, html] of [['links', shares.section()], ['sites', webs.section()]]) {
      const pane = document.createElement('section'); pane.id = 'publicationsPane-' + id;
      pane.className = 'publications-pane'; pane.setAttribute('role', 'tabpanel'); pane.setAttribute('aria-labelledby', 'publicationsTab-' + id);
      pane.innerHTML = html; body.append(pane);
    }
    tabs.addEventListener('keydown', event => {
      if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
      event.preventDefault(); this.select(event.key === 'Home' ? 'links' : event.key === 'End' ? 'sites' : this.active === 'links' ? 'sites' : 'links');
      $('publicationsTab-' + this.active).focus();
    });
    this.select(this.active);
    await Promise.allSettled([shares.bind(page), webs.bind(page)]);
  },
  select(id) {
    this.active = id === 'sites' ? 'sites' : 'links';
    for (const key of ['links', 'sites']) {
      const selected = key === this.active, button = $('publicationsTab-' + key), pane = $('publicationsPane-' + key);
      if (button) { button.classList.toggle('on', selected); button.setAttribute('aria-selected', String(selected)); button.tabIndex = selected ? 0 : -1; }
      if (pane) pane.hidden = !selected;
    }
  },
};
window.gregalPublications = publications;
document.addEventListener('gregal:idioma', () => { if (G()?.viewActual?.() === 'publicacions') publications.load(); });
if (G()?.viewActual?.() === 'publicacions') publications.load();
