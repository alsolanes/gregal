// «Què pot fer Gregal» (B14): la guia del que fa l'assistent i com demanar-ho,
// amb exemples que es proven d'un clic (s'envien al fil de Gregal). Mateix
// contingut que Android (GregalGuide.kt). S'obre la primera vegada i amb el
// botó ✦ de la capçalera de Gregal.
(function (root) {
  'use strict';
  const ca = () => document.documentElement.lang === 'ca', t = (a, b) => ca() ? a : b;
  const el = (tag, cls, text) => { const n = document.createElement(tag); if (cls) n.className = cls; if (text !== undefined) n.textContent = text; return n; };
  const SEEN = 'gregal.guide.seen';

  function intro() {
    return [t('Què pot fer Gregal', 'What Gregal can do'),
      t("Parla-li com a un assistent que té el seu propi ordinador. Fa la feina ell mateix, torna aquí amb el resultat i només et pregunta abans d'actuar fora.",
        'Talk to it as you would to an assistant who has its own computer. It does the work itself, comes back here with the result and only asks before acting outside.')];
  }

  function topics() {
    return [
      ['🔎', t('Encarrega-li feina', 'Hand it a task'),
        t('Cerca, compara, resumeix i prepara. No cal dir-li com: ho resol i et porta el resultat al fil.', 'It searches, compares, summarises and prepares. No need to tell it how: it works it out and brings the result to the thread.'),
        [t("Busca'm tres restaurants tranquils per sopar divendres a prop de casa", 'Find me three quiet restaurants for dinner on Friday near home'),
          t('Compara els tres mòbils més ben valorats per menys de 400 €', 'Compare the three best-rated phones under 400')]],
      ['⏰', t("Recorda-te'n per mi", 'Remember it for me'),
        t("Recordatoris d'una vegada o que es repeteixen. Arriben a l'hora encara que no obris l'app, i els pots ajornar o canviar.", "One-off or repeating reminders. They arrive on time even if you don't open the app, and you can snooze or change them."),
        [t("Recorda'm demà a les 9 de trucar al metge", 'Remind me tomorrow at 9 to call the doctor'),
          t("Cada dilluns a les 8, recorda'm de revisar les factures", 'Every Monday at 8, remind me to check the bills')]],
      ['👀', t('Vigila coses per tu', 'Keep an eye on things'),
        t("Una web, un preu, el teu calendari o el correu. Si no canvia res, calla; quan canvia, prepara la feina i t'avisa. Sota cada avís, «Per què?» diu d'on ve.", 'A web page, a price, your calendar or your mail. If nothing changes it stays quiet; when it does, it prepares the work and tells you. Under each note, «Why?» says where it came from.'),
        [t("Vigila el preu del Zen 14 i avisa'm si baixa de 900 €", 'Watch the price of the Zen 14 and tell me if it drops under 900'),
          t('Cada matí mira la meva agenda i digues-me què he de preparar', 'Every morning look at my calendar and tell me what to prepare')]],
      ['🔗', t('Connecta els teus comptes', 'Connect your accounts'),
        t('Correu, calendari i apps des de Preferències → Connexions. Les contrasenyes es guarden xifrades i Gregal no les veu mai.', 'Mail, calendar and apps from Preferences → Connections. Passwords are stored encrypted and Gregal never sees them.'),
        [t('Què tinc avui al calendari?', "What's on my calendar today?"),
          t("Tinc algun correu important d'aquesta setmana sense respondre?", 'Do I have any important unanswered email from this week?')]],
      ['🖥️', t('El seu ordinador', 'Its own computer'),
        t('Navega i fa tràmits en un ordinador propi. Toca 🖥 a dalt per veure on clica i què escriu, i pren el control quan calgui iniciar sessió.', 'It browses and runs errands on a computer of its own. Tap 🖥 at the top to see where it clicks and what it types, and take control when you need to sign in.'),
        [t("Entra a la web de l'ajuntament i mira quins tràmits puc fer en línia", 'Go to the city council website and see which errands I can do online')]],
      ['🧑‍🤝‍🧑', t('Especialistes quan calen', 'Specialists when they help'),
        t("Si un dels teus vents en sap més (compres, dades, codi…), Gregal li ho pregunta en segon pla i et diu qui l'ha ajudat. També pots parlar-hi directament des del menú.", 'If one of your winds knows more (shopping, data, code…), Gregal asks it in the background and tells you who helped. You can also talk to them directly from the menu.'),
        [t('Demana als especialistes quin portàtil em convé per programar', 'Ask the specialists which laptop suits me for coding')]],
      ['🌐', t('Webs que es publiquen', 'Websites that go live'),
        t("Et fa una web o una app petita, te l'ensenya en privat i la publica amb el teu nom, a Internet o només a casa.", 'It builds a website or a small app, shows it to you privately and publishes it under your name, on the internet or only at home.'),
        [t('Fes-me una web senzilla amb les receptes de la família', 'Make me a simple website with our family recipes')]],
      ['✋', t("Tu tens l'última paraula", 'You have the last word'),
        t('Abans d\'enviar, comprar, publicar o esborrar, et pregunta. Pots dir-li «fes-ho així la propera vegada» i ho recordarà.', 'Before sending, buying, publishing or deleting, it asks you. Say «do it like this next time» and it remembers.'),
        []],
    ];
  }

  function open({ accent, onTry } = {}) {
    const d = el('dialog', 'vw-dialog vw-guide');
    if (accent) d.style.setProperty('--g-accent', accent);
    const [title, lead] = intro();
    d.setAttribute('aria-label', title);
    const close = el('button', 'vw-guide-close', '×'); close.type = 'button'; close.setAttribute('aria-label', t('Tanca', 'Close'));
    close.onclick = () => d.close();
    const head = el('header', 'vw-guide-head');
    head.append(el('h2', '', title), el('p', '', lead));
    const grid = el('div', 'vw-guide-grid');
    topics().forEach(([icon, name, how, examples], i) => {
      const card = el('section', 'vw-guide-card'); card.style.setProperty('--i', String(i));
      const top = el('div', 'vw-guide-top'); top.append(el('span', 'vw-guide-icon', icon), el('h3', '', name));
      card.append(top, el('p', 'vw-guide-how', how));
      for (const ex of examples) {
        const b = el('button', 'vw-guide-try', '“' + ex + '”'); b.type = 'button';
        b.onclick = () => { d.close(); onTry?.(ex); };
        card.append(b);
      }
      grid.append(card);
    });
    d.append(close, head, grid);
    d.addEventListener('close', () => { try { localStorage.setItem(SEEN, '1'); } catch (_) {} d.remove(); });
    document.body.append(d);
    d.showModal();
  }

  function seen() { try { return localStorage.getItem(SEEN) === '1'; } catch (_) { return true; } }

  root.gregalGuide = { open, seen, topics, intro };
})(window);
