// Ajudes pures del canal #retro (sense DOM): mencions, agrupació de
// missatges i aplicació d'events en directe. Es carreguen al navegador
// (window.gregalRetroCore) i als tests de node (require).
(function (root) {
  'use strict';
  const MENTION = /(?:^|[^\w@])@([A-Za-z][\w-]*)/g;
  const slug = name => String(name || '').trim().toLowerCase().replace(/ /g, '-');

  // Mateixa regla que parseRetroMentions (retro_pack.go): per id o pel nom.
  function parseMentions(text, agents) {
    const out = [];
    const list = Array.isArray(agents) ? agents : [];
    let m;
    MENTION.lastIndex = 0;
    while ((m = MENTION.exec(String(text || '')))) {
      const tok = m[1].toLowerCase().replace(/[.,;:!?-]+$/, '');
      const hit = list.find(a => !out.includes(a.id) && (tok === a.id || tok === slug(a.name) || ((tok === 'main' || tok === 'coord') && a.id === 'coordinator')));
      if (hit) out.push(hit.id);
    }
    return out;
  }

  // Què s'està mencionant al cursor («@bu|» → {start, query:'bu'}).
  function mentionQuery(text, caret) {
    const before = String(text || '').slice(0, caret == null ? String(text || '').length : caret);
    const m = /(^|[^\w@])@([\w-]*)$/.exec(before);
    if (!m) return null;
    return { start: before.length - m[2].length - 1, query: m[2].toLowerCase() };
  }

  function mentionMatches(query, agents) {
    const q = String(query || '').toLowerCase();
    return (Array.isArray(agents) ? agents : []).filter(a => !q || a.id.startsWith(q) || slug(a.name).startsWith(q)).slice(0, 8);
  }

  // Substitueix la menció a mig escriure per @id i torna el text i el cursor.
  function insertMention(text, caret, id) {
    const q = mentionQuery(text, caret);
    if (!q) return { text, caret };
    const head = text.slice(0, q.start) + '@' + id + ' ';
    return { text: head + text.slice(caret), caret: head.length };
  }

  // Agrupa els missatges del canal principal per autor consecutiu (com Slack):
  // un grup nou quan canvia l'autor o passen més de gapMs.
  function groupMessages(messages, gapMs) {
    const gap = gapMs == null ? 5 * 60 * 1000 : gapMs;
    const groups = [];
    for (const m of Array.isArray(messages) ? messages : []) {
      if (m.thread_id) continue;
      const at = Date.parse(m.at) || 0;
      const last = groups[groups.length - 1];
      if (last && last.author === m.author && at - last.lastAt <= gap && m.author !== 'system') {
        last.items.push(m); last.lastAt = at;
      } else {
        groups.push({ author: m.author, at, lastAt: at, items: [m] });
      }
    }
    return groups;
  }

  function replyCounts(messages) {
    const out = {};
    for (const m of Array.isArray(messages) ? messages : []) if (m.thread_id) out[m.thread_id] = (out[m.thread_id] || 0) + 1;
    return out;
  }

  function threadMessages(messages, rootId) {
    return (Array.isArray(messages) ? messages : []).filter(m => m.id === rootId || m.thread_id === rootId);
  }

  // Alterna una reacció localment (pintat optimista abans de la resposta).
  function toggleReaction(reactions, emoji, by) {
    const list = (Array.isArray(reactions) ? reactions : []).map(r => ({ emoji: r.emoji, by: (r.by || []).slice() }));
    const r = list.find(x => x.emoji === emoji);
    if (r && r.by.includes(by)) r.by = r.by.filter(b => b !== by);
    else if (r) r.by.push(by);
    else list.push({ emoji, by: [by] });
    return list.filter(x => x.by.length);
  }

  // Aplica un event durable al canal. Torna true si n'ha canviat res.
  function applyEvent(channel, kind, payload) {
    if (!channel || !payload || payload.key !== channel.key) return false;
    if (!Array.isArray(channel.messages)) channel.messages = [];
    if (kind === 'retro_message' && payload.message && payload.message.id) {
      const i = channel.messages.findIndex(m => m.id === payload.message.id);
      if (i >= 0) channel.messages[i] = Object.assign({}, channel.messages[i], payload.message);
      else channel.messages.push(payload.message);
      return true;
    }
    if (kind === 'retro_reaction' && payload.message_id) {
      const m = channel.messages.find(x => x.id === payload.message_id);
      if (!m) return false;
      m.reactions = Array.isArray(payload.reactions) ? payload.reactions : [];
      return true;
    }
    return false;
  }

  // Text de «X està escrivint…» a partir de la llista de noms.
  function typingNames(names, and) {
    const n = (names || []).filter(Boolean);
    if (n.length <= 1) return n.join('');
    return n.slice(0, -1).join(', ') + (and || ' and ') + n[n.length - 1];
  }

  // Troballes de la revisió de l'equip per gravetat (només el canal principal).
  const SEVERITIES = ['high', 'medium', 'low'];
  function findingCounts(messages) {
    const out = { high: 0, medium: 0, low: 0, total: 0 };
    for (const m of Array.isArray(messages) ? messages : []) {
      if (m.kind !== 'finding' || m.thread_id) continue;
      const sev = SEVERITIES.includes(m.severity) ? m.severity : 'low';
      out[sev]++; out.total++;
    }
    return out;
  }

  // Missatges a mostrar amb el filtre de la barra de revisió: sense filtre,
  // tots; amb «all», només les troballes; si no, les d'aquella gravetat.
  function filterFindings(messages, filter) {
    const list = Array.isArray(messages) ? messages : [];
    if (!filter) return list;
    return list.filter(m => m.kind === 'finding' && !m.thread_id && (filter === 'all' || (SEVERITIES.includes(m.severity) ? m.severity : 'low') === filter));
  }

  const api = { parseMentions, mentionQuery, mentionMatches, insertMention, groupMessages, replyCounts, threadMessages, toggleReaction, applyEvent, typingNames, findingCounts, filterFindings, SEVERITIES };
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) root.gregalRetroCore = api;
})(typeof window !== 'undefined' ? window : null);
