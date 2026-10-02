const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(process.argv[2] || path.join(__dirname, '..'));
const findings = [];
const rules = [
  ['private key', /-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----/],
  ['provider credential', /\b(?:sk-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9]{20,})\b/],
  ['Telegram credential', /\b\d{8,}:[A-Za-z0-9_-]{30,}\b/],
  ['personal email', /[A-Za-z0-9._%+-]+@(?:gmail\.com|outlook\.com|hotmail\.com|yahoo\.com)/i],
];
const markers = JSON.parse(process.env.GREGAL_PRIVATE_MARKERS || '[]');
if (!Array.isArray(markers) || markers.some(value => typeof value !== 'string' || !value.trim())) {
  throw new Error('GREGAL_PRIVATE_MARKERS must be a JSON array of nonempty strings');
}
for (const marker of markers) {
  rules.push(['configured private marker', new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i')]);
}
function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    const relative = path.relative(root, full).replaceAll('\\', '/');
    if (entry.isSymbolicLink()) {
      findings.push(`${relative}: symlink requires review`);
      continue;
    }
    if (entry.isDirectory()) {
      if (['.gregal', '.hermes', '.claude', '.playwright-cli', '.venv', '__pycache__'].includes(entry.name)) {
        findings.push(`${relative}: local runtime directory`);
        continue;
      }
      if (['.git', 'node_modules', 'output', 'public-release', 'dist', 'build', '.gradle'].includes(entry.name)) continue;
      walk(full);
    } else {
      if (/^(?:\.env(?:\..*)?|tokens\.json|passwords\.json|memory\.md|local\.properties)$/.test(entry.name) && entry.name !== '.env.example') findings.push(`${relative}: local runtime data`);
      const bytes = fs.readFileSync(full);
      if (bytes.includes(0)) continue;
      const text = bytes.toString('utf8');
      for (const [name, pattern] of rules) {
        if (['LICENSE', 'vscode-gregal/LICENSE', 'internal/web/app/vendor/THIRD_PARTY_NOTICES.md'].includes(relative)
          && name.startsWith('personal ')) continue;
        if (pattern.test(text)) findings.push(`${relative}: ${name}`);
      }
    }
  }
}
walk(root);
if (findings.length) {
  console.error(findings.join('\n'));
  process.exitCode = 1;
} else console.log('Public-source privacy checks passed (heuristic scan; binary assets require visual review).');
