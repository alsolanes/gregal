// Tradueix la pàgina abans del primer pintat. L'HTML porta els textos en
// anglès i fins ara els traduïa panels.js, que és gros i arriba tard: en
// passar de Codi a Vents es veia un instant l'anglès i després el català.
// index.html el carrega al <head> amb blocking="render": el navegador no
// pinta res fins que aquest mòdul ha corregut (just després de llegir el
// document). És el mateix mòdul que importa panels.js, no una còpia.
import { aplica, t } from './i18n.js';

window.gregalT = window.gregalT || t;
aplica();
