// Vents: same authenticated API/session as the host app. All remote text is
// rendered with textContent; SVG shapes and attributes are authored here only.
(function (root) {
  'use strict';
  const doc = root.document;
  const state = { winds: [], conversations: [], active: null, loading: false };
  const $ = id => doc.getElementById(id);
  function node(tag, cls, text) {
    const n = doc.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) n.textContent = text;
    return n;
  }
  function button(text, action, cls) {
    const n = node('button', cls || 'ghost', text);
    n.setAttribute('type', 'button'); n.onclick = action;
    return n;
  }
  async function call(url, method = 'GET', body) {
    const opts = { method };
    if (body !== undefined) { opts.body = JSON.stringify(body); opts.headers = { 'Content-Type': 'application/json' }; }
    const response = await root.api(url, opts);
    if (!response.ok) throw new Error('request-failed');
    return response.json();
  }
  function color(value) { return /^#[\da-f]{6}$/i.test(value || '') ? value : '#F5B45D'; }
  function style(value) {
    const s = String(value || 'pso');
    return ('pbem'.includes(s[0]) ? s[0] : 'p') + ('sont'.includes(s[1]) ? s[1] : 's') + ('ons'.includes(s[2]) ? s[2] : 'o');
  }
  function face(w, large) {
    const s = style(w.style), ink = '#262040';
    const svg = doc.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 80 80'); svg.setAttribute('class', 'winds-face' + (large ? ' winds-face-large' : ''));
    svg.setAttribute('aria-hidden', 'true'); svg.setAttribute('focusable', 'false');
    function shape(tag, attrs) {
      const el = doc.createElementNS('http://www.w3.org/2000/svg', tag);
      for (const [key, value] of Object.entries(attrs)) el.setAttribute(key, value);
      svg.append(el);
    }
    const fill = color(w.color);
    if (s[2] === 'n') shape('path', { d:'M18 62C1 62 1 37 17 34C14 13 43 8 50 24C69 16 83 39 68 49C76 66 49 74 40 63Z', fill });
    else shape('circle', { cx:40, cy:40, r:33, fill });
    if (s[2] === 's') shape('path', { d:'M41 14C67 14 72 56 50 64C28 74 12 51 19 33C25 19 47 19 53 31', fill:'none', stroke:'#FFFFFF', 'stroke-opacity':'.35', 'stroke-width':3 });
    shape('ellipse', { cx:28, cy:21, rx:9, ry:5, fill:'#FFFFFF', opacity:'.3' });
    for (const x of [29, 51]) {
      if (s[0] === 'm') shape('path', { d:`M${x-4} 36q4 5 8 0`, fill:'none', stroke:ink, 'stroke-width':3, 'stroke-linecap':'round' });
      else if (s[0] === 'e') shape('path', { d:`M${x} 29l2 5 5 2-5 2-2 5-2-5-5-2 5-2Z`, fill:ink });
      else shape('ellipse', { cx:x, cy:36, rx:s[0] === 'b' ? 4 : 2.5, ry:s[0] === 'b' ? 6 : 3.5, fill:ink });
    }
    if (s[1] === 'o') shape('ellipse', { cx:40, cy:51, rx:5, ry:6, fill:ink });
    else shape('path', { d:s[1] === 'n' ? 'M35 50h10' : 'M32 47q8 12 16 0', fill:'none', stroke:ink, 'stroke-width':3, 'stroke-linecap':'round' });
    if (s[1] === 't') shape('path', { d:'M39 52q0 10 6 6l1-7', fill:'#DC6784' });
    return svg;
  }
  function status(id, text, error) {
    const n = $(id); n.textContent = text || ''; n.classList.toggle('winds-error', !!error);
    n.setAttribute('role', error ? 'alert' : 'status');
  }
  function roster(ids) { return ids.map(id => state.winds.find(w => w.id === id)).filter(Boolean); }
  function row(title, subtitle, members, action, selected) {
    const b = button('', action, 'winds-row'); b.setAttribute('aria-pressed', String(!!selected));
    const faces = node('span', 'winds-face-stack'); members.slice(0, 3).forEach(w => faces.append(face(w)));
    const copy = node('span', 'winds-row-copy'); copy.append(node('strong', '', title), node('span', 'winds-muted', subtitle));
    b.append(faces, copy); return b;
  }
  function draw() {
    const list = $('windsList'); list.replaceChildren();
    for (const w of state.winds) list.append(row(w.name, w.role || 'Conversa amb ' + w.name, [w], () => openChat({ kind:'wind', id:w.id, name:w.name, members:[w.id] }), state.active?.kind === 'wind' && state.active.id === w.id));
    if (!state.winds.length) list.append(node('p', 'winds-empty', 'Encara no tens cap vent. Crea el primer des de «Nou».'));
    const chats = $('windsConversations'); chats.replaceChildren();
    for (const c of state.conversations) chats.append(row(c.name, c.members.length > 1 ? 'Rotllana · ' + c.members.length + ' vents' : 'Conversa · ' + (roster(c.members)[0]?.name || 'Un vent'), roster(c.members), () => openChat({ ...c, kind:'rotllana' }), state.active?.kind === 'rotllana' && state.active.id === c.id));
    if (!state.conversations.length) chats.append(node('p', 'winds-empty', 'Les noves converses apareixeran aquí.'));
    if (state.active) drawChatHeading();
  }
  function drawChatHeading() {
    const active = state.active, members = roster(active.members);
    if (active.kind === 'wind' && members[0]) active.name = members[0].name;
    $('windChatTitle').textContent = active.name;
    $('windChatSubtitle').textContent = active.kind === 'wind' ? (members[0]?.role || 'Conversa individual') : members.map(w => w.name).join(' · ');
    $('windChatFaces').replaceChildren(...members.slice(0, 3).map(w => face(w)));
    $('windsChatMembers').replaceChildren(...members.map(w => button('Perfil de ' + w.name, e => openProfile(w.id, e.target), 'ghost winds-profile-button')));
    let edit = $('windsEditConversation');
    if (!edit) {
      edit = button('Edita el grup i els membres', () => conversationForm(state.active, edit), 'ghost winds-edit-conversation');
      edit.setAttribute('id', 'windsEditConversation'); edit.setAttribute('aria-haspopup', 'dialog'); edit.setAttribute('aria-controls', 'windsDialog');
      $('windChatCard').querySelector('header').append(edit);
    }
    edit.hidden = active.kind !== 'rotllana';
  }
  function load() {
    if (!$('windsList')) return Promise.resolve(false);
    if (state.loading) return state.loading;
    status('windsStatus', 'Carregant els teus vents…');
    state.loading = (async () => {
      try {
        const [winds, chats] = await Promise.all([call('/api/winds'), call('/api/winds/rotllanes')]);
        state.winds = winds.winds || []; state.conversations = chats.rotllanes || [];
        draw(); status('windsStatus', ''); return true;
      } catch (_) { status('windsStatus', 'No hem pogut carregar els vents. Torna-ho a provar amb «Actualitza la llista».', true); return false; }
      finally { state.loading = false; }
    })();
    return state.loading;
  }
  async function refreshAfterWrite() {
    if (state.loading) await state.loading;
    if (!await load()) throw new Error('catalogue-not-verified');
  }
  async function verifyWritten(action) {
    try { return await action(); }
    catch (error) { error.alreadySent = true; throw error; }
  }
  const chatKey = c => c.kind + ':' + c.id;
  const drafts = new Map(), pending = new Set();
  let chatVersion = 0;
  function historyURL(c) { return c.kind === 'wind' ? '/api/winds/chat?wind=' + encodeURIComponent(c.id) : '/api/winds/rotllana?rotlla=' + encodeURIComponent(c.id); }
  function busy() {
    const waiting = !!state.active && pending.has(chatKey(state.active));
    $('windChatSend').disabled = waiting;
    $('windChatInput').disabled = waiting;
    $('windsChatForm').setAttribute('aria-busy', String(waiting));
  }
  function drawHistory(history, active) {
    const hist = $('windChatHistory'); hist.replaceChildren();
    if (!history.length) hist.append(node('p', 'winds-empty', 'Aquí comença la conversa. Què us voleu explicar?'));
    for (const m of history) {
      const user = m.from === 'user';
      const w = state.winds.find(w => w.id === m.speaker_id) || (active.members.length === 1 ? roster(active.members)[0] : null);
      const name = user ? 'Tu' : (m.speaker_name || w?.name || (m.from !== 'assistant' ? m.from : 'Vent'));
      const msg = node('article', 'winds-message' + (user ? ' winds-message-user' : ''));
      const avatar = user ? node('span', 'winds-user-avatar', 'Tu') : face(w || { color:'#BCC4D4', style:'pno' });
      avatar.setAttribute('aria-hidden', 'true');
      const content = node('div', 'winds-message-content');
      content.append(node('strong', 'winds-speaker', name), node('p', 'winds-message-text', m.text || ''));
      msg.append(avatar, content); hist.append(msg);
    }
    hist.scrollTop = hist.scrollHeight;
  }
  async function fetchHistory(active, version) {
    const d = await call(historyURL(active));
    if (chatVersion === version && state.active && chatKey(state.active) === chatKey(active)) drawHistory(d.history || [], active);
  }
  async function openChat(active) {
    if (state.active) drafts.set(chatKey(state.active), $('windChatInput').value);
    state.active = active; const version = ++chatVersion;
    $('windsWelcome').hidden = true; $('windChatCard').hidden = false; $('windsProfile').hidden = true;
    ++profileVersion;
    $('windChatInput').value = drafts.get(chatKey(active)) || '';
    $('windChatHistory').replaceChildren();
    draw(); busy(); status('windsChatStatus', pending.has(chatKey(active)) ? 'Esperant la resposta…' : 'Carregant la conversa…');
    try { await fetchHistory(active, version); if (version === chatVersion && !pending.has(chatKey(active))) status('windsChatStatus', ''); }
    catch (_) { if (version === chatVersion) status('windsChatStatus', 'No hem pogut llegir la conversa. Torna-la a obrir per reintentar-ho.', true); }
  }
  async function send(event) {
    if (event) event.preventDefault();
    const active = state.active, input = $('windChatInput');
    if (!active || !input.value.trim() || pending.has(chatKey(active))) return;
    const key = chatKey(active), text = input.value.trim(), version = chatVersion;
    pending.add(key); drafts.set(key, input.value); busy(); status('windsChatStatus', 'Esperant la resposta…');
    const current = () => state.active && chatKey(state.active) === key;
    let posted = false;
    try {
      await call(active.kind === 'wind' ? '/api/winds/chat' : '/api/winds/rotllana', 'POST', active.kind === 'wind' ? {wind:active.id, message:text} : {rotllana:active.id, message:text});
      posted = true; drafts.delete(key);
      if (current()) input.value = '';
      const refreshVersion = current() ? ++chatVersion : version;
      await fetchHistory(active, refreshVersion);
      if (current() && refreshVersion === chatVersion) status('windsChatStatus', '');
    } catch (_) {
      if (current()) status('windsChatStatus', posted ? 'El missatge s’ha enviat, però no hem pogut actualitzar la conversa. Torna-la a obrir.' : 'No hem pogut enviar el missatge. El text continua aquí; torna-ho a provar.', true);
    } finally { pending.delete(key); busy(); if (current()) input.focus(); }
  }
  let profileVersion = 0, profileOrigin = null;
  function closeProfile() { ++profileVersion; $('windsProfile').hidden = true; if (profileOrigin) profileOrigin.focus(); }
  async function openProfile(id, origin) {
    profileOrigin = origin || profileOrigin;
    const version = ++profileVersion, panel = $('windsProfile'); panel.hidden = false;
    panel.replaceChildren(node('p', 'winds-muted', 'Carregant el perfil…'));
    panel.setAttribute('aria-busy', 'true');
    try {
      const data = await call('/api/winds/profile?wind=' + encodeURIComponent(id));
      if (!data.wind || data.wind.id !== id) throw new Error('missing-profile');
      if (version === profileVersion) { renderProfile(data); panel.scrollIntoView?.({block:'start'}); $('windsProfileTitle').focus(); }
    } catch (_) {
      if (version === profileVersion) {
        const error = node('p', 'winds-status winds-error', 'No hem pogut llegir el perfil. No s’ha canviat cap record.'); error.setAttribute('role', 'alert');
        panel.replaceChildren(error, button('Torna-ho a provar', () => openProfile(id, origin)), button('Tanca el perfil', closeProfile));
      }
    } finally { if (version === profileVersion) panel.setAttribute('aria-busy', 'false'); }
  }
  function renderProfile(data) {
    const w = data.wind, panel = $('windsProfile'); panel.replaceChildren();
    const header = node('header', 'winds-profile-heading'); const title = node('h2', '', w.name);
    title.setAttribute('id', 'windsProfileTitle'); title.setAttribute('tabindex', '-1');
    const heading = node('div'); heading.append(title, node('p', 'winds-muted', w.role || 'Vent de Gregal'));
    header.append(face(w, true), heading, button('Tanca el perfil', closeProfile)); panel.append(header);
    const character = node('section', 'winds-profile-section');
    const edit = button('Edita el caràcter', () => windForm(w, edit)); edit.setAttribute('id', 'windsEditCharacter');
    character.append(node('h3', '', 'Caràcter'), node('p', 'winds-memory-text', w.persona || 'Encara no has definit cap caràcter propi.'), edit);
    panel.append(character);
    const form = node('form', 'winds-profile-section'); form.setAttribute('id', 'windsMemoryForm');
    form.append(node('h3', '', 'Què vols que sàpiga?'), node('p', 'winds-muted', 'Coneixement desat d’aquest vent. Desar substitueix tot aquest text; deixar-lo buit l’esborra.'));
    const knowledge = field(form, 'windKnowledge', 'Coneixement del vent', 'textarea', data.knowledge || ''); knowledge.rows = 6;
    knowledge.setAttribute('placeholder', 'Encara no hi has desat cap coneixement.');
    const save = node('button', 'ghost', 'Desa el coneixement'); save.setAttribute('type', 'submit');
    const feedback = node('p', 'winds-status'); feedback.setAttribute('id', 'windsMemoryStatus'); feedback.setAttribute('role', 'status'); feedback.setAttribute('aria-live', 'polite');
    form.append(save, feedback); panel.append(form);
    const memory = node('section', 'winds-profile-section');
    memory.append(node('h3', '', 'Records personals'), node('p', 'winds-muted', 'Memòria personal compartida amb el vent, tal com està desada. Només de lectura.'), node('p', 'winds-memory-text', data.personal_memory || 'Encara no hi ha records personals desats.'));
    panel.append(memory);
    let saving = false;
    form.onsubmit = async e => {
      e.preventDefault(); if (saving) return;
      saving = true; save.disabled = true; knowledge.disabled = true; form.setAttribute('aria-busy', 'true');
      const version = profileVersion; status('windsMemoryStatus', 'Desant el coneixement…');
      let posted = false;
      try {
        await call('/api/winds/memory', 'POST', {wind:w.id, text:knowledge.value}); posted = true;
        const verified = await call('/api/winds/profile?wind=' + encodeURIComponent(w.id));
        if (version === profileVersion && !panel.hidden) { renderProfile(verified); status('windsMemoryStatus', 'Coneixement desat.'); }
      } catch (_) {
        if (version === profileVersion && !panel.hidden) status('windsMemoryStatus', posted ? 'S’ha desat el text, però no hem pogut tornar a llegir el perfil. Reobre’l per comprovar-lo.' : 'No hem pogut desar el coneixement. Conservem el text perquè ho puguis tornar a provar.', true);
      } finally { saving = false; save.disabled = false; knowledge.disabled = false; form.setAttribute('aria-busy', 'false'); }
    };
  }
  let dialogSaving = false, returnFocus = null;
  function menu(open) {
    $('windsNewMenu').hidden = !open; $('windNewBtn').setAttribute('aria-expanded', String(open));
    if (open) $('windsCreateWind').focus();
  }
  function field(parent, id, title, tag, value, type) {
    const wrap = node('label', 'winds-field'); wrap.setAttribute('for', id);
    wrap.append(node('span', '', title));
    const input = node(tag || 'input'); input.setAttribute('id', id); input.setAttribute('name', id);
    if (type) input.setAttribute('type', type);
    input.value = value || ''; wrap.append(input); parent.append(wrap); return input;
  }
  function select(parent, id, title, values, value) {
    const input = field(parent, id, title, 'select');
    for (const [key, label] of values) { const option = node('option', '', label); option.setAttribute('value', key); input.append(option); }
    input.value = value; return input;
  }
  // Keep these characters in step with Android's WindSuggestions.
  const presets = [
    {name:'Marinada', color:'#EFC69A', style:'bsn', role:'calma i benestar', persona:'Parles amb calma, escoltes amb cura i proposes petits passos sense jutjar.'},
    {name:'Xaloc', color:'#EFA68F', style:'eto', role:'música i descobertes', persona:'Ets curiós i juganer. Comparteixes música i descobertes amb entusiasme i cites les fonts quan investigues.'},
    {name:'Tramuntana', color:'#A8CDE2', style:'pso', role:'idees i curiositat', persona:'Fas preguntes clares, connectes idees i expliques amb senzillesa el que descobreixes.'},
    {name:'Mestral', color:'#BFCFA4', style:'esn', role:'aprendre plegats', persona:'Ets pacient i enginyós. Ajuda a aprendre amb exemples entenedors i reconeix quan no saps una cosa.'},
    {name:'Llebeig', color:'#E8AEC2', style:'pto', role:'històries i cultura', persona:'T’agraden les històries, l’art i la cultura. Ets càlid, imaginatiu i distingeixes els fets de la ficció.'},
    {name:'Gregal', color:'#C1B4DE', style:'mso', role:'el dia a dia', persona:'Ets proper i atent. Fas companyia en el dia a dia, amb humor suau i sense donar res per sabut.'},
    {name:'Ponent', color:'#EACF87', style:'msn', role:'llibres i tardes', persona:'Ets reflexiu i tranquil. T’agrada conversar sobre llibres i idees amb preguntes obertes.'},
    {name:'Migjorn', color:'#9ED3C5', style:'bss', role:'natura i escapades', persona:'Ets alegre i observador. T’agrada la natura i proposes descobertes adaptades al que l’usuari explica.'}
  ];
  function windGallery() {
    menu(false); returnFocus = $('windNewBtn');
    const body = $('windsDialogBody'), title = node('h2', '', 'Quin vent et farà companyia?');
    title.setAttribute('id', 'windsDialogTitle');
    const gallery = node('div', 'winds-preset-gallery'); gallery.setAttribute('id', 'windsPresetGallery');
    gallery.setAttribute('role', 'group'); gallery.setAttribute('aria-label', 'Tria un personatge per començar');
    for (const preset of presets) {
      const choice = button('', () => windForm(null, $('windNewBtn'), preset), 'winds-preset');
      choice.append(face(preset, true), node('strong', '', preset.name), node('span', 'winds-muted', preset.role));
      gallery.append(choice);
    }
    const actions = node('div', 'winds-dialog-actions');
    actions.append(button('Cancel·la', () => $('windsDialog').close()), button('Crea un vent personalitzat', () => windForm(), 'ghost winds-primary'));
    body.replaceChildren(title, node('p', 'winds-muted winds-gallery-intro', 'Tria una cara amiga i fes-la teva: podràs canviar-ne el nom, el caràcter i tots els detalls.'), gallery, actions);
    if (!$('windsDialog').open) $('windsDialog').showModal();
    gallery.querySelector('button').focus();
  }
  function dialogForm(title, id, submitText, save, focusBack) {
    menu(false); returnFocus = focusBack || $('windNewBtn');
    const body = $('windsDialogBody'); body.replaceChildren(node('h2', '', title)); body.children[0].setAttribute('id', 'windsDialogTitle');
    const form = node('form', 'winds-form'); form.setAttribute('id', id);
    const fields = node('div', 'winds-form-fields');
    const feedback = node('p', 'winds-status'); feedback.setAttribute('role', 'status'); feedback.setAttribute('aria-live', 'polite');
    const actions = node('div', 'winds-dialog-actions');
    const cancel = button('Cancel·la', () => $('windsDialog').close());
    const submit = node('button', 'ghost winds-primary', submitText); submit.setAttribute('type', 'submit');
    actions.append(cancel, submit); form.append(fields, feedback, actions); body.append(form);
    form.onsubmit = async e => {
      e.preventDefault(); if (dialogSaving) return;
      dialogSaving = true; submit.disabled = true; cancel.disabled = true; form.setAttribute('aria-busy', 'true');
      feedback.textContent = 'Desant…'; feedback.classList.remove('winds-error'); feedback.setAttribute('role', 'status');
      try { await save(feedback); }
      catch (error) { feedback.textContent = error.alreadySent ? 'Els canvis s’han enviat, però no hem pogut comprovar-los. Actualitza la llista abans de tornar-ho a provar.' : 'No hem pogut desar els canvis. Revisa la connexió i torna-ho a provar.'; feedback.classList.add('winds-error'); feedback.setAttribute('role', 'alert'); }
      finally { dialogSaving = false; submit.disabled = false; cancel.disabled = false; form.setAttribute('aria-busy', 'false'); }
    };
    return { fields, feedback, show() { if (!$('windsDialog').open) $('windsDialog').showModal(); const first = fields.querySelector('input,textarea,select'); if (first) first.focus(); } };
  }
  function windForm(w, focusBack, preset) {
    const editing = !!w; w = w || preset || {name:'', role:'', persona:'', color:'#F5B45D', style:'pso'};
    let name, role, persona, tint, eyes, mouth, shape;
    const form = dialogForm(editing ? 'El caràcter de ' + w.name : 'Un vent nou', 'windsWindForm', editing ? 'Desa els canvis' : 'Crea el vent', async feedback => {
      if (!name.value.trim()) { feedback.textContent = 'Posa un nom al vent.'; name.focus(); return; }
      const payload = {name:name.value.trim(), role:role.value.trim(), color:color(tint.value), persona:persona.value.trim(), style:eyes.value + mouth.value + shape.value};
      if (editing) payload.id = w.id;
      const saved = await call('/api/winds', 'POST', payload);
      await verifyWritten(async () => {
        if (!saved.wind?.id) throw new Error('missing-id');
        const verified = await call('/api/winds/profile?wind=' + encodeURIComponent(saved.wind.id));
        if (!verified.wind || verified.wind.id !== saved.wind.id) throw new Error('missing-profile');
        await refreshAfterWrite();
        if (editing && !$('windsProfile').hidden) { renderProfile(verified); returnFocus = $('windsEditCharacter'); }
      });
      $('windsDialog').close();
      status('windsStatus', editing ? 'Caràcter actualitzat.' : 'El nou vent ja és aquí. Tria’l per començar a parlar.');
    }, focusBack);
    const preview = node('div', 'winds-face-preview'); preview.setAttribute('id', 'windFacePreview'); form.fields.append(preview);
    name = field(form.fields, 'windName', 'Nom', 'input', w.name, 'text'); name.required = true; name.maxLength = 100;
    role = field(form.fields, 'windRole', 'Què li interessa?', 'input', w.role, 'text'); role.setAttribute('placeholder', 'Música, llibres, idees…');
    persona = field(form.fields, 'windPersona', 'Caràcter i manera de parlar', 'textarea', w.persona); persona.rows = 4;
    persona.setAttribute('placeholder', 'Com vols que sigui? Què li importa?');
    tint = field(form.fields, 'windColor', 'Color', 'input', color(w.color), 'color');
    const choices = node('div', 'winds-face-choices'); form.fields.append(choices); const s = style(w.style);
    eyes = select(choices, 'windEyes', 'Ulls', [['p','Punts'],['b','Ben oberts'],['e','Estrellats'],['m','Mig tancats']], s[0]);
    mouth = select(choices, 'windMouth', 'Boca', [['s','Somriure'],['o','Oberta'],['n','Tranquil·la'],['t','Llengua']], s[1]);
    shape = select(choices, 'windShape', 'Forma', [['o','Bola'],['n','Núvol'],['s','Espiral']], s[2]);
    const update = () => preview.replaceChildren(face({color:tint.value, style:eyes.value + mouth.value + shape.value}, true));
    [tint, eyes, mouth, shape].forEach(n => { n.addEventListener('change', update); n.addEventListener('input', update); }); update(); form.show();
  }
  function conversationForm(conversation, focusBack) {
    const editing = !!conversation, minimum = conversation?.members.length > 1 ? 2 : 1;
    let name; const checks = [];
    const form = dialogForm(editing ? 'Edita el grup i els membres' : 'Una conversa nova', 'windsChatFormDialog', editing ? 'Desa els canvis' : 'Crea la conversa', async feedback => {
      const members = checks.filter(c => c.checked).map(c => c.value);
      if (!name.value.trim() || members.length < minimum) { feedback.textContent = minimum === 2 ? 'Tria un nom i almenys dos vents per a la rotllana.' : 'Tria un nom i almenys un vent per conversar.'; return; }
      const payload = {name:name.value.trim(), members};
      if (editing) payload.id = conversation.id;
      const saved = await call('/api/winds/rotllanes', 'POST', payload);
      const target = await verifyWritten(async () => {
        if (!saved.rotllana?.id || (editing && saved.rotllana.id !== conversation.id)) throw new Error('missing-id');
        // Read catalogue and history back; never represent a local-only chat as saved.
        await refreshAfterWrite();
        const persisted = state.conversations.find(c => c.id === saved.rotllana.id);
        if (!persisted || persisted.name !== payload.name || persisted.members.length !== members.length || !members.every(id => persisted.members.includes(id))) throw new Error('not-persisted');
        return persisted;
      });
      // A slow save must not take the user back from another conversation.
      if (!editing || (state.active?.kind === 'rotllana' && state.active.id === conversation.id)) {
        await openChat({...target, kind:'rotllana'});
        if (editing) returnFocus = $('windsEditConversation');
      } else returnFocus = $('windNewBtn');
      $('windsDialog').close();
    }, focusBack);
    name = field(form.fields, 'windsConversationName', 'Nom de la conversa', 'input', conversation?.name || '', 'text'); name.required = true;
    form.fields.append(node('p', 'winds-muted', minimum === 2 ? 'Una rotllana ha de conservar almenys dos vents.' : 'Un sol vent per parlar a soles; més d’un per fer una rotllana.'));
    const group = node('fieldset', 'winds-picker'); group.append(node('legend', '', 'Qui hi serà?'));
    for (const w of state.winds) {
      const label = node('label', 'winds-member-choice'); const check = node('input'); check.setAttribute('type', 'checkbox'); check.setAttribute('name', 'members'); check.value = w.id;
      check.checked = !!conversation?.members.includes(w.id);
      label.append(check, face(w), node('span', '', w.name)); checks.push(check); group.append(label);
    }
    if (!state.winds.length) group.append(node('p', 'winds-empty', 'Encara no tens cap vent. Cancel·la i crea’n un des de «Nou».'));
    form.fields.append(group); form.show();
  }
  let bound = false;
  function bind() {
    if (bound) return; bound = true;
    $('windsRefresh').onclick = load;
    $('windNewBtn').onclick = () => menu($('windsNewMenu').hidden);
    $('windsCreateWind').onclick = windGallery;
    $('windsCreateChat').onclick = () => conversationForm();
    $('windsDialog').addEventListener('cancel', e => { if (dialogSaving) e.preventDefault(); });
    $('windsDialog').addEventListener('close', () => { if (returnFocus) returnFocus.focus(); });
    doc.addEventListener('click', e => { if (!$('windsNewMenu').hidden && !$('windsNewMenu').contains(e.target) && !$('windNewBtn').contains(e.target)) menu(false); });
    doc.addEventListener('keydown', e => { if (e.key === 'Escape' && !$('windsNewMenu').hidden) { menu(false); $('windNewBtn').focus(); } });
    $('windsChatForm').onsubmit = send;
    $('windChatInput').addEventListener('keydown', e => { if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); send(); } });
  }
  root.loadWindsPage = function () { bind(); return load(); };
})(window);
