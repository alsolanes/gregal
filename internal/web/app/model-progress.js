// What the turn says while the model call streams, from a `progress` event of
// phase `model` (docs/api-contract.md): «Writing index.html · 67k
// characters» while it writes a tool's arguments, «Thinking · 12k
// characters» and the tail of the reasoning while it only reasons. The count
// is characters the model has written, not context tokens: «67k» alone read
// as either. T translates an i18n key. Pure: also loaded by node tests.
(function (root) {
  const fmtChars = n => { n = +n || 0; return n >= 1000 ? (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k' : String(n); };

  function modelProgress(d, T) {
    const chars = n => T('act.now.chars').replace('{n}', fmtChars(n));
    if (d.state !== 'receiving') return { key: 'model', text: T('act.now.model'), tail: '' };
    if (+d.tool > 0) {
      // Sense nom: un servidor Responses que no envia output_item.added.
      const text = d.tool_target ? T('act.now.writingFile').replace('{file}', d.tool_target)
        : d.tool_name ? T('act.now.toolcallNamed').replace('{tool}', d.tool_name) : T('act.now.toolcall');
      return { key: 'model-tool', text: text + ' · ' + chars(d.tool), tail: '' };
    }
    if (+d.text > 0) return { key: 'writing', text: T('act.now.writing'), tail: '' };
    return { key: 'thinking', text: T('act.now.thinking') + (+d.reasoning > 0 ? ' · ' + chars(d.reasoning) : ''), tail: d.tail || '' };
  }

  const api = { modelProgress };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.gregalModelProgress = api;
})(typeof window !== 'undefined' ? window : globalThis);
