// Catàleg de racons del pati. Cada vent porta el seu racó segons el tema
// (ofici, rol, descripció): un vent de bàsquet porta la pista de bàsquet, un
// d'informàtica l'aula d'ordinadors, un de llibres la biblioteca… El pati
// només dibuixa els racons dels vents que hi ha, i creix quan en crees més.
//
// Cada racó: com es diu, la icona, les paraules que el fan aparèixer, què hi
// fan els vents (verb, efecte, cara i frases), si hi ha pilota, l'energia
// típica i el dibuix (SVG en coordenades del racó, 360×270).
(function (root) {
  'use strict';
  const W = 360, H = 270;
  // Peces de dibuix reutilitzables.
  const tree = (x, y, s = 1) => `<rect x="${x - 6 * s}" y="${y}" width="${12 * s}" height="${34 * s}" fill="#8A6A45"/><circle cx="${x}" cy="${y - 4 * s}" r="${30 * s}" fill="var(--pati-leaves)"/><circle cx="${x - 20 * s}" cy="${y + 6 * s}" r="${18 * s}" fill="var(--pati-leaves)"/><circle cx="${x + 20 * s}" cy="${y + 4 * s}" r="${19 * s}" fill="var(--pati-leaves)"/>`;
  const bench = (x, y) => `<rect x="${x}" y="${y}" width="70" height="8" rx="3" fill="#A07A50"/><path d="M${x + 8} ${y + 8}v12M${x + 62} ${y + 8}v12" stroke="#7A5A38" stroke-width="5"/>`;
  const desk = (x, y) => `<rect x="${x}" y="${y}" width="70" height="10" rx="3" fill="#A88A62"/><path d="M${x + 6} ${y + 10}v22M${x + 64} ${y + 10}v22" stroke="#7A5A38" stroke-width="5"/>`;
  const monitor = (x, y) => `<rect x="${x}" y="${y}" width="34" height="24" rx="3" fill="#2E3A4F" stroke="#5E6B82" stroke-width="3"/><rect x="${x + 4}" y="${y + 4}" width="26" height="15" fill="#7FD1F2" opacity=".85"/><path d="M${x + 17} ${y + 24}v6M${x + 9} ${y + 31}h16" stroke="#5E6B82" stroke-width="3"/>`;
  const laptop = (x, y) => `<path d="M${x} ${y + 18}h34l-4-18h-26Z" fill="#B9C2D3" stroke="#6F7A8F" stroke-width="2"/><rect x="${x + 6}" y="${y + 3}" width="22" height="12" fill="#A6E3C9"/><rect x="${x - 3}" y="${y + 18}" width="40" height="4" rx="2" fill="#8E98AB"/>`;

  const KINDS = {
    plaza: { icon: '🎪', ca: 'La plaça', en: 'The square' },
    football: {
      icon: '⚽', ca: 'Camp de futbol', en: 'Football pitch', ball: '⚽', energy: 0.95,
      words: ['futbol', 'football', 'soccer', 'barca', 'barça', 'lliga', 'gol'], specialties: ['sports'],
      act: { ca: 'juga a futbol', en: 'plays football', fx: '⚽', mood: 'idle', move: 'hop' },
      say: [['Passa-la!', 'Pass it!'], ['Goooool!', 'Goooal!'], ['Un partidet?', 'Quick match?']],
      think: ['En com guanyar el partit de dissabte', 'How to win Saturday’s match'],
      area: [40, 70, 320, 230],
      draw: () => `<rect x="20" y="40" width="320" height="200" rx="10" fill="var(--pati-grass)" stroke="#fff" stroke-width="3"/><path d="M180 40v200" stroke="#fff" stroke-width="3"/><circle cx="180" cy="140" r="30" fill="none" stroke="#fff" stroke-width="3"/><rect x="20" y="105" width="24" height="70" fill="none" stroke="#fff" stroke-width="3"/><rect x="316" y="105" width="24" height="70" fill="none" stroke="#fff" stroke-width="3"/>`,
    },
    amfootball: {
      icon: '🏈', ca: 'Camp de futbol americà', en: 'American football field', ball: '🏈', energy: 0.95,
      words: ['futbol americ', 'american football', 'nfl', 'touchdown', 'rugby'],
      act: { ca: 'fa un touchdown', en: 'scores a touchdown', fx: '🏈', mood: 'idle', move: 'hop' },
      say: [['Touchdown!', 'Touchdown!'], ['Hut, hut!', 'Hut, hut!'], ['Placa’l!', 'Tackle!']],
      think: ['En la jugada perfecta', 'The perfect play'],
      area: [40, 70, 320, 230],
      draw: () => `<rect x="20" y="40" width="320" height="200" rx="8" fill="#6FAE5A" stroke="#fff" stroke-width="3"/>${[60, 100, 140, 180, 220, 260, 300].map(x => `<path d="M${x} 40v200" stroke="#fff" stroke-width="2" opacity=".8"/>`).join('')}<path d="M24 110v60M24 110h-6M24 170h-6M336 110v60M336 110h6M336 170h6" stroke="#F2D45C" stroke-width="5"/><text x="180" y="150" text-anchor="middle" font-size="26" fill="#fff" opacity=".6" font-weight="700">50</text>`,
    },
    basketball: {
      icon: '🏀', ca: 'Pista de bàsquet', en: 'Basketball court', ball: '🏀', energy: 0.9,
      words: ['basquet', 'bàsquet', 'basket', 'nba', 'acb', 'cistella'],
      act: { ca: 'tira a cistella', en: 'shoots hoops', fx: '🏀', mood: 'idle', move: 'hop' },
      say: [['Triple!', 'Three-pointer!'], ['Esmaixada!', 'Slam dunk!'], ['Defensa!', 'Defence!']],
      think: ['En el tir de tres de l’última jugada', 'That last three-pointer'],
      area: [50, 80, 310, 230],
      draw: () => `<rect x="25" y="45" width="310" height="195" rx="8" fill="#D98B4A" stroke="#fff" stroke-width="3"/><path d="M180 45v195" stroke="#fff" stroke-width="3"/><circle cx="180" cy="142" r="26" fill="none" stroke="#fff" stroke-width="3"/><path d="M25 95a60 60 0 0 1 0 95M335 95a60 60 0 0 0 0 95" fill="none" stroke="#fff" stroke-width="3"/><path d="M36 122v40M324 122v40" stroke="#555" stroke-width="5"/><circle cx="48" cy="142" r="9" fill="none" stroke="#E7735C" stroke-width="3"/><circle cx="312" cy="142" r="9" fill="none" stroke="#E7735C" stroke-width="3"/>`,
    },
    tennis: {
      icon: '🎾', ca: 'Pista de tennis', en: 'Tennis court', ball: '🎾', energy: 0.8,
      words: ['tennis', 'padel', 'pàdel', 'raqueta'],
      act: { ca: 'juga a tennis', en: 'plays tennis', fx: '🎾', mood: 'idle', move: 'hop' },
      say: [['Fora!', 'Out!'], ['Bona bola!', 'Nice shot!'], ['Quinze iguals', 'Fifteen all']],
      think: ['En com tornar aquell revés', 'How to return that backhand'],
      area: [50, 70, 310, 230],
      draw: () => `<rect x="25" y="45" width="310" height="195" rx="6" fill="#4C8DC4" stroke="#fff" stroke-width="3"/><rect x="55" y="70" width="250" height="145" fill="none" stroke="#fff" stroke-width="2.5"/><path d="M180 40v205" stroke="#EEE" stroke-width="5" stroke-dasharray="4 3"/><path d="M55 142h250" stroke="#fff" stroke-width="2"/>`,
    },
    pool: {
      icon: '🏊', ca: 'Piscina', en: 'Pool', energy: 0.75,
      words: ['natacio', 'natació', 'swim', 'piscina', 'pool', 'surf', 'platja', 'beach', 'mar'],
      act: { ca: 'neda', en: 'swims', fx: '💦', mood: 'idle', move: 'swim' },
      say: [['L’aigua és bona!', 'The water’s great!'], ['Un llarg més', 'One more lap'], ['Splash!', 'Splash!']],
      think: ['En baixar dos segons del rècord', 'Shaving two seconds off the record'],
      area: [60, 90, 300, 220],
      draw: () => `<rect x="30" y="55" width="300" height="180" rx="22" fill="#E8EEF2" stroke="#B9C7D2" stroke-width="5"/><rect x="45" y="70" width="270" height="150" rx="14" fill="#6EC3E8"/>${[105, 145, 185].map(y => `<path d="M55 ${y}h250" stroke="#fff" stroke-width="3" stroke-dasharray="8 6" opacity=".8"/>`).join('')}<path d="M70 95q20-8 40 0t40 0" fill="none" stroke="#fff" stroke-width="3" opacity=".6"/>`,
    },
    track: {
      icon: '🏃', ca: 'Pista d’atletisme', en: 'Running track', energy: 0.95,
      words: ['correr', 'córrer', 'running', 'atletisme', 'athlet', 'marato', 'marató', 'runner', 'cursa', 'triatl', 'cicl', 'bici'],
      act: { ca: 'corre voltes', en: 'runs laps', fx: '🏁', mood: 'idle', move: 'laps' },
      say: [['Una volta més!', 'One more lap!'], ['Rècord!', 'Personal best!'], ['Respira…', 'Breathe…']],
      think: ['En la propera cursa', 'The next race'],
      area: [70, 85, 290, 205],
      draw: () => `<rect x="25" y="45" width="310" height="195" rx="97" fill="#C9624A"/><rect x="70" y="85" width="220" height="115" rx="57" fill="var(--pati-grass)"/>${[1, 2, 3].map(i => `<rect x="${25 + i * 11}" y="${45 + i * 10}" width="${310 - i * 22}" height="${195 - i * 20}" rx="${97 - i * 10}" fill="none" stroke="#fff" stroke-width="1.5" opacity=".7"/>`).join('')}`,
    },
    gym: {
      icon: '🏋️', ca: 'Gimnàs', en: 'Gym', energy: 0.8,
      words: ['gimnas', 'gimnàs', 'fitness', 'salut', 'health', 'ioga', 'yoga', 'entrena', 'muscul'],
      act: { ca: 'fa exercici', en: 'works out', fx: '💪', mood: 'working', move: 'hop' },
      say: [['Deu més!', 'Ten more!'], ['Estira bé', 'Stretch it out'], ['Uf!', 'Phew!']],
      think: ['En un pla d’entrenament per a tu', 'A training plan for you'],
      area: [60, 120, 300, 225],
      draw: () => `<rect x="30" y="90" width="300" height="150" rx="12" fill="#C7D3DF"/><rect x="50" y="200" width="120" height="16" rx="4" fill="#7C8FB0"/><path d="M220 150h80" stroke="#555" stroke-width="6"/><rect x="212" y="138" width="10" height="26" rx="2" fill="#333"/><rect x="298" y="138" width="10" height="26" rx="2" fill="#333"/><circle cx="90" cy="140" r="16" fill="#E7735C"/>`,
    },
    chess: {
      icon: '♟️', ca: 'Escacs gegants', en: 'Giant chess', energy: 0.2,
      words: ['escacs', 'chess', 'estrateg', 'strateg', 'trencaclosques', 'puzzle', 'joc de taula', 'board game'],
      act: { ca: 'pensa la jugada', en: 'plans a move', fx: '♟️', mood: 'thinking', move: 'still' },
      say: [['Escac!', 'Check!'], ['Hmm…', 'Hmm…'], ['Bona obertura', 'Nice opening']],
      think: ['En tres jugades d’avançada', 'Three moves ahead'],
      area: [60, 70, 300, 225],
      draw: () => `${Array.from({ length: 36 }, (_, i) => { const r = Math.floor(i / 6), c = i % 6; return `<rect x="${60 + c * 40}" y="${40 + r * 32}" width="40" height="32" fill="${(r + c) % 2 ? '#5B4A35' : '#F2E6CF'}"/>`; }).join('')}<text x="120" y="98" font-size="26">♜</text><text x="220" y="196" font-size="26">♞</text>`,
    },
    stage: {
      icon: '🎤', ca: 'L’escenari', en: 'The stage', energy: 0.6,
      words: ['music', 'músic', 'concert', 'canço', 'cançó', 'song', 'banda', 'band', 'cantant', 'singer', 'dj', 'teatre', 'theatre', 'ball', 'dance'], specialties: ['music'],
      act: { ca: 'canta a l’escenari', en: 'sings on stage', fx: '♪', mood: 'discussing', move: 'dance' },
      say: [['♪ la-la-la ♪', '♪ la-la-la ♪'], ['Fem una banda?', 'Shall we start a band?'], ['Aquesta tornada!', 'That chorus!']],
      think: ['En una cançó que et faria somriure', 'A song that would make you smile'],
      area: [70, 150, 290, 200],
      draw: () => `<rect x="40" y="150" width="280" height="80" rx="8" fill="#9C6FB0"/><rect x="40" y="140" width="280" height="14" rx="5" fill="#B88BCB"/><path d="M52 140v-90h256v90" fill="none" stroke="#7A4E8E" stroke-width="7"/><path d="M52 50q64 34 128 0q64 34 128 0" fill="#E7735C" opacity=".88"/><circle cx="80" cy="70" r="7" fill="#F7E07A"/><circle cx="280" cy="70" r="7" fill="#F7E07A"/>`,
    },
    library: {
      icon: '📚', ca: 'La biblioteca', en: 'The library', energy: 0.15,
      words: ['llibre', 'book', 'lectura', 'reading', 'literatur', 'novel', 'poesia', 'poetry', 'escriptor', 'writer', 'biblio', 'cultura', 'culture', 'histori'], specialties: ['books', 'review'],
      act: { ca: 'llegeix a la biblioteca', en: 'reads in the library', fx: '📖', mood: 'reading', move: 'still' },
      say: [['Escolta això…', 'Listen to this…'], ['Quin final!', 'What an ending!'], ['Un capítol més', 'One more chapter']],
      think: ['En quin llibre et recomanaria', 'Which book to recommend you'],
      area: [70, 150, 300, 225],
      draw: () => `${[40, 130, 220].map(x => `<rect x="${x}" y="40" width="80" height="100" rx="4" fill="#8A6A45"/>${[55, 80, 105].map(y => `<path d="M${x + 4} ${y + 16}h72" stroke="#6B5236" stroke-width="3"/>${[0, 1, 2, 3, 4, 5].map(i => `<rect x="${x + 8 + i * 11}" y="${y}" width="8" height="15" fill="${['#E7735C', '#7BBF6A', '#F2B53A', '#7C8FB0', '#B88BCB', '#6EC3E8'][(i + y) % 6]}"/>`).join('')}`).join('')}`).join('')}${bench(150, 200)}<rect x="60" y="180" width="50" height="40" rx="10" fill="#C96F5A"/>`,
    },
    computers: {
      icon: '💻', ca: 'L’aula d’ordinadors', en: 'The computer lab', energy: 0.35,
      words: ['informatic', 'informàtic', 'ordinador', 'computer', ' pc', 'portatil', 'portàtil', 'laptop', 'software', 'codi', 'code', 'program', 'backend', 'frontend', 'desenvolup', 'developer', 'tecnolog', 'tech', 'xarxa', 'network', 'servidor', 'server', 'api', 'web'],
      specialties: ['backend', 'frontend', 'testing'],
      act: { ca: 'programa a l’ordinador', en: 'codes at the computer', fx: '⌨️', mood: 'writing', move: 'still' },
      say: [['Compila!', 'It compiles!'], ['Un bug…', 'A bug…'], ['git push!', 'git push!'], ['Funciona!', 'It works!']],
      think: ['En com fer-ho sòlid i ràpid', 'How to make it solid and fast'],
      area: [50, 150, 310, 225],
      draw: () => `${[40, 140, 240].map(x => desk(x, 120)).join('')}${monitor(58, 92)}${laptop(158, 100)}${monitor(258, 92)}<rect x="40" y="40" width="280" height="40" rx="6" fill="#2F4A3A" stroke="#8A6A45" stroke-width="5"/><text x="180" y="66" text-anchor="middle" font-size="16" fill="#E9F1E6" font-family="monospace">&lt;hola /&gt;</text>`,
    },
    lab: {
      icon: '🧪', ca: 'El laboratori de dades', en: 'The data lab', energy: 0.3,
      words: ['dades', 'data', 'estadist', 'statist', 'machine learning', ' ml', 'model', 'ciencia', 'ciència', 'science', 'quimica', 'química', 'biolog', 'fisica', 'física', 'analis', 'anàlisi', 'regress', 'previsi', 'forecast'],
      specialties: ['quality', 'regression', 'ml', 'timeseries'],
      act: { ca: 'fa gràfics i experiments', en: 'runs experiments and charts', fx: '📈', mood: 'searching', move: 'still' },
      say: [['Quina corba!', 'Nice curve!'], ['Hi ha un patró', 'There’s a pattern'], ['Una prova més', 'One more test']],
      think: ['En què diuen de debò les dades', 'What the data really says'],
      area: [50, 160, 310, 225],
      draw: () => `${desk(40, 140)}${desk(250, 140)}<path d="M70 132l-8 8h20Z M90 120v20M84 120h12" fill="#9CE0C2" stroke="#4F7E6A" stroke-width="2"/><circle cx="270" cy="128" r="10" fill="#B88BCB" opacity=".8"/><rect x="120" y="40" width="120" height="85" rx="6" fill="#FFFFFF" stroke="#8A6A45" stroke-width="5"/><path d="M135 110l20-20 18 12 22-30 22 10" fill="none" stroke="#E7735C" stroke-width="4"/><path d="M135 115h90M135 50v65" stroke="#555" stroke-width="2"/>`,
    },
    kitchen: {
      icon: '🍳', ca: 'La cuina', en: 'The kitchen', energy: 0.5,
      words: ['cuina', 'cook', 'recepta', 'recipe', 'menjar', 'food', 'xef', 'chef', 'pastis', 'pastís', 'baking', 'forn', 'restaurant', 'vi ', 'wine'],
      act: { ca: 'cuina alguna cosa bona', en: 'cooks something tasty', fx: '🍲', mood: 'working', move: 'stir' },
      say: [['Fa olor de bo!', 'Smells great!'], ['Una mica de sal', 'A pinch of salt'], ['Ja és a punt!', 'Almost ready!']],
      think: ['En què et cuinaria per sopar', 'What to cook you for dinner'],
      area: [60, 160, 300, 225],
      draw: () => `<rect x="40" y="90" width="280" height="60" rx="6" fill="#E9D3A8" stroke="#9A7A50" stroke-width="4"/><rect x="60" y="80" width="60" height="12" rx="3" fill="#555"/><circle cx="90" cy="76" r="14" fill="#7C8FB0"/><path d="M90 58q-6-10 0-16M98 58q-6-10 0-16" fill="none" stroke="#bbb" stroke-width="3"/><rect x="200" y="60" width="50" height="30" rx="4" fill="#F2B53A"/><circle cx="280" cy="78" r="9" fill="#E7735C"/>`,
    },
    garden: {
      icon: '🌻', ca: 'L’hort', en: 'The garden', energy: 0.35,
      words: ['jardi', 'jardí', 'hort', 'planta', 'plant', 'natura', 'nature', 'flor', 'flower', 'animal', 'gos', 'dog', 'gat', 'cat', 'mascot', 'pet', 'medi ambient', 'environment', 'clima', 'weather', 'temps'],
      act: { ca: 'rega les plantes', en: 'waters the plants', fx: '🌱', mood: 'idle', move: 'still' },
      say: [['Ha brotat!', 'It sprouted!'], ['Necessita aigua', 'Needs water'], ['Quina flor!', 'What a flower!']],
      think: ['En com fer créixer les coses', 'How to help things grow'],
      area: [50, 150, 310, 230],
      draw: () => `${[50, 140, 230].map(x => `<rect x="${x}" y="70" width="80" height="60" rx="6" fill="#8A6A45"/>${[0, 1, 2].map(i => `<circle cx="${x + 15 + i * 25}" cy="92" r="8" fill="${['#F2B53A', '#E7735C', '#B88BCB'][i]}"/><path d="M${x + 15 + i * 25} 100v18" stroke="#4F7E3A" stroke-width="3"/>`).join('')}`).join('')}${tree(320, 160, 0.7)}`,
    },
    art: {
      icon: '🎨', ca: 'El taller d’art', en: 'The art studio', energy: 0.5,
      words: ['art', 'disseny', 'design', 'dibuix', 'drawing', 'pintur', 'paint', 'ux', 'interfic', 'interface', 'il·lustr', 'illustr', 'moda', 'fashion', 'arquitect'],
      specialties: ['ux'],
      act: { ca: 'pinta un quadre', en: 'paints a picture', fx: '🎨', mood: 'writing', move: 'still' },
      say: [['Més color!', 'More colour!'], ['I si és blau?', 'What if it’s blue?'], ['Ho dibuixo', 'Let me sketch it']],
      think: ['En com fer-ho més clar i bonic', 'How to make it clearer and nicer'],
      area: [50, 160, 310, 225],
      draw: () => `${[70, 170, 270].map((x, i) => `<path d="M${x} 150l20-90 20 90M${x + 20} 60v95" fill="none" stroke="#8A6A45" stroke-width="4"/><rect x="${x + 2}" y="70" width="36" height="46" fill="#FFFDF7" stroke="#9A7A50" stroke-width="2"/><circle cx="${x + 20}" cy="93" r="10" fill="${['#E7735C', '#6EC3E8', '#7BBF6A'][i]}" opacity=".85"/>`).join('')}`,
    },
    market: {
      icon: '🛒', ca: 'El mercat', en: 'The market', energy: 0.55,
      words: ['compr', 'shopping', 'preu', 'price', 'mercat', 'market', 'botig', 'shop', 'oferta', 'deal', 'producte'], specialties: ['shopping'],
      act: { ca: 'compara preus', en: 'compares prices', fx: '🏷️', mood: 'searching', move: 'still' },
      say: [['Això està de rebaixes', 'This is on sale'], ['Massa car!', 'Too pricey!'], ['Ho he comparat tot', 'I compared them all']],
      think: ['En on trobar-ho més bé de preu', 'Where to find it cheaper'],
      area: [50, 160, 310, 225],
      draw: () => `${[50, 200].map(x => `<rect x="${x}" y="80" width="110" height="60" fill="#E9D3A8" stroke="#9A7A50" stroke-width="3"/><path d="M${x - 10} 80h130l-10-28h-110Z" fill="${x === 50 ? '#E7735C' : '#7BBF6A'}"/><circle cx="${x + 25}" cy="118" r="9" fill="#F2B53A"/><circle cx="${x + 55}" cy="118" r="9" fill="#E7735C"/><circle cx="${x + 85}" cy="118" r="9" fill="#7BBF6A"/>`).join('')}`,
    },
    cinema: {
      icon: '🎬', ca: 'El cinema a la fresca', en: 'Open-air cinema', energy: 0.2,
      words: ['cinema', 'pel·licul', 'pelicul', 'movie', 'film', 'netflix', 'tv', 'anime', 'series de tv', 'tv series'],
      act: { ca: 'mira una pel·lícula', en: 'watches a film', fx: '🍿', mood: 'reading', move: 'still' },
      say: [['Quin gir!', 'What a twist!'], ['Crispetes?', 'Popcorn?'], ['Sense spoilers!', 'No spoilers!']],
      think: ['En quina sèrie et faria enganxar', 'Which series you’d binge'],
      area: [60, 165, 300, 225],
      draw: () => `<rect x="60" y="35" width="240" height="105" rx="4" fill="#F7F7F2" stroke="#555" stroke-width="5"/><path d="M80 120l50-50 30 30 30-45 40 65" fill="none" stroke="#9CC4E8" stroke-width="6" opacity=".6"/>${[90, 150, 210].map(x => `<rect x="${x}" y="170" width="44" height="12" rx="4" fill="#C96F5A"/>`).join('')}`,
    },
    travel: {
      icon: '✈️', ca: 'L’estació de viatges', en: 'The travel stop', energy: 0.6,
      words: ['viatg', 'travel', 'vacanc', 'holiday', 'turism', 'tourism', 'vol ', 'flight', 'hotel', 'muntany', 'mountain', 'excursi', 'hiking'],
      act: { ca: 'planeja un viatge', en: 'plans a trip', fx: '🗺️', mood: 'searching', move: 'still' },
      say: [['On anem?', 'Where to?'], ['Bon viatge!', 'Bon voyage!'], ['Mira el mapa', 'Check the map']],
      think: ['En una escapada perfecta per a tu', 'A perfect getaway for you'],
      area: [50, 160, 310, 225],
      draw: () => `<rect x="50" y="45" width="150" height="95" rx="6" fill="#CFE7F2" stroke="#7C8FB0" stroke-width="4"/><path d="M70 100q20-30 40-10t40-20 40 10" fill="none" stroke="#7BBF6A" stroke-width="10" opacity=".7"/><path d="M95 75l3 9 9 3-9 3-3 9-3-9-9-3 9-3Z" fill="#E7735C"/><path d="M260 140v-90" stroke="#7A5A38" stroke-width="5"/><path d="M262 60h46l8 10-8 10h-46Z M258 90h-46l-8 10 8 10h46Z" fill="#F2B53A" stroke="#9A7A50" stroke-width="2"/>`,
    },
    bank: {
      icon: '🏦', ca: 'La guardiola', en: 'The money corner', energy: 0.3,
      words: ['diner', 'money', 'financ', 'banc', 'bank', 'inversi', 'invest', 'econom', 'estalvi', 'saving', 'pressupost', 'budget', 'cripto', 'crypto', 'borsa', 'stock'],
      act: { ca: 'compta les monedes', en: 'counts coins', fx: '🪙', mood: 'searching', move: 'still' },
      say: [['Estalviem!', 'Let’s save!'], ['Quadra!', 'It balances!'], ['Interès compost', 'Compound interest']],
      think: ['En com fer rendir els estalvis', 'How to make savings grow'],
      area: [60, 160, 300, 225],
      draw: () => `<path d="M80 140v-60h200v60" fill="#E9E2D0" stroke="#9A8A6A" stroke-width="4"/><path d="M70 80l110-40 110 40Z" fill="#C9B08A"/>${[100, 140, 180, 220, 260].map(x => `<rect x="${x - 6}" y="88" width="12" height="50" fill="#F2EBDA"/>`).join('')}<circle cx="130" cy="185" r="12" fill="#F2B53A" stroke="#C98E2A" stroke-width="2"/><circle cx="150" cy="190" r="12" fill="#F2B53A" stroke="#C98E2A" stroke-width="2"/>`,
    },
    arcade: {
      icon: '🕹️', ca: 'La sala recreativa', en: 'The arcade', energy: 0.6,
      words: ['videojoc', 'video game', 'gaming', 'gamer', 'consola', 'console', 'nintendo', 'playstation', 'xbox', 'joc', 'game'],
      act: { ca: 'juga a la màquina', en: 'plays arcade games', fx: '👾', mood: 'idle', move: 'hop' },
      say: [['Nivell superat!', 'Level up!'], ['Game over…', 'Game over…'], ['Rècord!', 'High score!']],
      think: ['En la partida de després', 'The next game'],
      area: [50, 165, 310, 225],
      draw: () => `${[60, 150, 240].map((x, i) => `<rect x="${x}" y="50" width="60" height="105" rx="6" fill="${['#7C5BB0', '#E7735C', '#4C8DC4'][i]}"/><rect x="${x + 8}" y="62" width="44" height="34" rx="3" fill="#1D2233"/><circle cx="${x + 18}" cy="115" r="5" fill="#F2B53A"/><circle cx="${x + 40}" cy="115" r="5" fill="#7BBF6A"/>`).join('')}`,
    },
    observatory: {
      icon: '🔭', ca: 'L’observatori', en: 'The observatory', energy: 0.25,
      words: ['astronom', 'espai', 'space', 'estrel', 'star', 'planeta', 'planet', 'cosmos', 'nasa', 'cel '],
      act: { ca: 'mira les estrelles', en: 'stargazes', fx: '✨', mood: 'searching', move: 'still' },
      say: [['Una estrella fugaç!', 'A shooting star!'], ['Saturn!', 'Saturn!'], ['Que gran que és', 'It’s so big']],
      think: ['En què hi ha més enllà', 'What lies beyond'],
      area: [60, 165, 300, 225],
      draw: () => `<rect x="110" y="95" width="140" height="60" fill="#D7DCE6" stroke="#8E98AB" stroke-width="3"/><path d="M110 95a70 60 0 0 1 140 0Z" fill="#B9C2D3" stroke="#8E98AB" stroke-width="3"/><path d="M180 70l50-30" stroke="#555" stroke-width="10" stroke-linecap="round"/>`,
    },
    office: {
      icon: '📋', ca: 'El despatx', en: 'The office', energy: 0.45,
      words: ['organitz', 'organis', 'organiz', 'agenda', 'calendar', 'planific', 'plan', 'productiv', 'gestio', 'gestió', 'manag', 'tasques', 'tasks', 'reunio', 'meeting'],
      specialties: ['organisation'],
      act: { ca: 'apunta coses a la pissarra', en: 'writes on the board', fx: '📝', mood: 'writing', move: 'still' },
      say: [['Ho apunto', 'Noting it down'], ['Primer això', 'This first'], ['A les cinc, reunió', 'Meeting at five']],
      think: ['En com encaixar-ho tot a l’agenda', 'How to fit everything in the schedule'],
      area: [50, 160, 310, 225],
      draw: () => `<rect x="70" y="40" width="220" height="95" rx="6" fill="#2F4A3A" stroke="#8A6A45" stroke-width="7"/><path d="M90 65h60M90 85h110M90 105h40" stroke="#E9F1E6" stroke-width="4" stroke-linecap="round" opacity=".85"/><rect x="215" y="60" width="50" height="40" fill="#F7E07A" transform="rotate(4 240 80)"/>${desk(140, 150)}`,
    },
    lookout: {
      icon: '🛡️', ca: 'La torre de guaita', en: 'The lookout', energy: 0.3,
      words: ['segureta', 'security', 'privacitat', 'privacy', 'ciberseg', 'cyber', 'vigil', 'guard', 'protecci'],
      specialties: ['security'],
      act: { ca: 'vigila des de dalt', en: 'keeps watch', fx: '🔭', mood: 'searching', move: 'still' },
      say: [['Tot en ordre', 'All clear'], ['Qui va?', 'Who goes there?'], ['Ull viu', 'Eyes open']],
      think: ['En si tot està ben tancat', 'Whether everything is locked up'],
      area: [70, 170, 290, 225],
      draw: () => `<path d="M150 40l-18 120M210 40l18 120M140 100h80" stroke="#8A6A45" stroke-width="8"/><rect x="138" y="22" width="84" height="24" rx="3" fill="#B08A5C"/><path d="M130 22l50-24 50 24Z" fill="#D9694F"/>${tree(60, 140, 0.8)}`,
    },
    post: {
      icon: '📮', ca: 'L’oficina de correus', en: 'The post office', energy: 0.85,
      words: ['lliurament', 'delivery', 'enviament', 'shipping', 'logisti', 'infra', 'deploy', 'desplega', 'correu', 'mail', 'paquet', 'parcel'],
      specialties: ['delivery'],
      act: { ca: 'porta paquets amunt i avall', en: 'carries parcels around', fx: '📦', mood: 'working', move: 'carry' },
      say: [['Paquet en camí!', 'Parcel on its way!'], ['On ho deixo?', 'Where do I drop it?']],
      think: ['En com fer arribar les coses abans', 'How to deliver things faster'],
      area: [50, 160, 310, 225],
      draw: () => `<rect x="60" y="60" width="140" height="80" rx="6" fill="#F2D45C" stroke="#9A7A50" stroke-width="4"/><text x="130" y="108" text-anchor="middle" font-size="22">📮</text>${[230, 260, 245].map((x, i) => `<rect x="${x}" y="${120 - i * 22}" width="34" height="22" fill="#C9A86A" stroke="#8A6A45" stroke-width="2"/>`).join('')}`,
    },
    corner: {
      icon: '🧺', ca: 'Racó', en: 'Corner', energy: 0.4,
      act: { ca: 'fa les seves coses', en: 'does its own thing', fx: '✨', mood: 'idle', move: 'still' },
      say: [['Hola!', 'Hi!'], ['Què fas?', 'What are you up to?'], ['M’agrada aquí', 'I like it here']],
      think: ['En el que més li agrada', 'What it loves most'],
      area: [50, 150, 310, 225],
      draw: () => `<rect x="110" y="110" width="140" height="14" rx="4" fill="#A07A50"/><path d="M120 124v26M240 124v26" stroke="#7A5A38" stroke-width="6"/><path d="M90 150h180" stroke="#C9A86A" stroke-width="10" stroke-linecap="round" opacity=".6"/><circle cx="160" cy="104" r="10" fill="#E7735C"/><circle cx="200" cy="104" r="10" fill="#F2B53A"/>${tree(60, 120, 0.7)}${tree(300, 120, 0.7)}`,
    },
  };

  const fold = s => String(s || '').normalize('NFD').replace(/\p{M}+/gu, '').toLowerCase();

  const escape = s => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  // Puntua un text: cada paraula del tema que hi surt a l'inici d'una paraula
  // suma la seva llargada («futbol americà» guanya a «futbol»).
  function scores(text) {
    const out = {};
    for (const [k, def] of Object.entries(KINDS)) {
      for (const word of def.words || []) {
        const w = fold(word).trim();
        const m = w && new RegExp('(^|[^a-z0-9])' + escape(w)).exec(text);
        if (m) out[k] = { len: (out[k]?.len || 0) + w.length, at: Math.min(out[k]?.at ?? Infinity, m.index) };
      }
    }
    return out;
  }
  // Guanya el tema que surt primer (el rol diu el principal al davant); a
  // igualtat, el més llarg («futbol americà» abans que «futbol»).
  const best = sc => Object.entries(sc).sort((a, b) => a[1].at - b[1].at || b[1].len - a[1].len)[0];

  /**
   * El tipus de racó d'un vent: primer el que diu el seu rol o el nom (un
   * vent «de bàsquet» va a la pista de bàsquet), després el seu ofici
   * conegut, després la descripció, i si res no encaixa, un racó propi.
   */
  function kindOf(w) {
    const byRole = best(scores(fold((w.role || '') + ' ' + (w.name || ''))));
    if (byRole) return byRole[0];
    const spec = w.specialty || w.kit || '';
    for (const [k, def] of Object.entries(KINDS)) if (def.specialties?.includes(spec)) return k;
    const byPersona = best(scores(fold(String(w.persona || '').slice(0, 600))));
    if (byPersona && byPersona[1].len >= 8) return byPersona[0];
    return 'corner';
  }

  root.gregalPatiZones = { KINDS, kindOf, TILE: { W, H } };
})(window);
