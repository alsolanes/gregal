// Punt d'entrada dels extres: repetició de torns a la sala, Gregal
// Wrapped, la mascota i el composer amb veu i imatges enganxades. Cada
// peça arrenca sola i, si falla, no arrossega les altres.
import { boot as replay } from './replay.js';
import { boot as wrapped } from './wrapped.js';
import { boot as mascot } from './mascot.js';
import { boot as voice } from './voice.js';

function start() {
  for (const [name, fn] of [['replay', replay], ['wrapped', wrapped], ['mascot', mascot], ['voice', voice]]) {
    try { fn(); } catch (e) { console.error('[gregal] ' + name + ': ' + (e && e.message)); }
  }
}

if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
else start();
