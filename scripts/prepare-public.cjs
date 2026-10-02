// Export tracked source without Git metadata or local runtime data.
const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const root = path.resolve(__dirname, '..');
const destination = path.join(root, 'public-release');
if (fs.existsSync(destination)) throw new Error('public-release already exists; review or move it before exporting again');
const tracked = execFileSync('git', ['ls-files', '-z'], { cwd: root }).toString().split('\0').filter(Boolean);
// Fitxers executables segons git (100755): a Windows el mode del sistema
// no porta el bit +x i l'export el perdria; a Linux la comprovació
// fork/exec (p. ex. examples/mcp-eco.py) fallaria amb permission denied.
const executable = new Set();
try {
  const index = execFileSync('git', ['ls-files', '-s', '-z'], { cwd: root }).toString().split('\0').filter(Boolean);
  for (const line of index) {
    const m = /^(100755) [0-9a-f]+ \d+\t(.*)$/.exec(line);
    if (m) executable.add(m[2]);
  }
} catch { /* sense índex git: cap +x a preservar */ }
const additions = ['README.ca.md', 'CONTRIBUTING.md', 'SECURITY.md', 'docs/open-source-audit.md', 'evals/example.yaml', 'scripts/prepare-public.cjs', 'scripts/audit-public.cjs', 'internal/config/language_test.go', 'internal/web/listen_auth_test.go', '.github/workflows/quality.yml'];
additions.push('harness/harness.go', 'harness/harness_test.go', 'client/client.go',
  'internal/client/interactions.go', 'internal/client/harness_test.go', 'docs/enterprise-harness.md', 'docs/enterprise-harness.ca.md',
  'examples/company-chat/main.go', 'examples/company-chat/config.example.yaml',
  'python/pyproject.toml', 'python/gregal_client/__init__.py', 'python/gregal_client/client.py',
  'python/tests/test_client.py', 'python/tests/test_live_contract.py');
additions.push('scripts/audit-public.test.cjs', 'internal/web/lifecycle_test.go');
additions.push('docs/github-release.md');
additions.push('.github/ISSUE_TEMPLATE/bug-report.yml', 'docs/getting-started.md',
  'docs/getting-started.ca.md', 'internal/web/app/vendor/THIRD_PARTY_NOTICES.md',
  'scripts/verify-vendor.cjs');
additions.push('vscode-gregal/package.nls.json', 'vscode-gregal/package.nls.ca.json',
  'vscode-gregal/l10n/bundle.l10n.json', 'vscode-gregal/l10n/bundle.l10n.ca.json',
  'vscode-gregal/test/l10n.test.ts');
additions.push('vscode-gregal/LICENSE');
additions.push('.gitattributes');
additions.push('cli_i18n.go', 'cli_i18n_test.go', 'internal/web/app/office.i18n.test.cjs');
additions.push('desktop/navigation.js', 'desktop/navigation.test.js');
additions.push('desktop/ipc.test.js');
additions.push('internal/web/auth.test.js');
additions.push('internal/web/secondary_i18n.test.js');
additions.push('android/gradlew.bat');
additions.push('docs/stable-baseline.md');
additions.push('docs/manual.ca.md');
const excluded = /^(?:\.(?:playwright-cli|hermes|claude)\/|docs\/plans\/|output\/|public-release\/)/;
for (const file of [...new Set([...tracked, ...additions])]) {
  if (excluded.test(file)) continue;
  if (/(?:^|\/)(?:__pycache__|\.venv|\.gregal|\.hermes|\.claude|\.playwright-cli)(?:\/|$)|\.(?:pyc|pyo)$/.test(file)) continue;
  const source = path.join(root, file);
  if (!fs.existsSync(source)) throw new Error(`Missing release file: ${file}`);
  if (fs.lstatSync(source).isSymbolicLink()) throw new Error(`Review symlink before publishing: ${file}`);
  // Export reviewed source byte-for-byte; fail the audit rather than rewrite leaks.
  const content = fs.readFileSync(source);
  const target = path.join(destination, file);
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, content, { mode: fs.statSync(source).mode });
  if (executable.has(file)) {
    try { fs.chmodSync(target, 0o755); } catch { /* Windows: ho fixa update-index en publicar */ }
  }
}
execFileSync(process.execPath, [path.join(root, 'scripts/audit-public.cjs'), destination], { stdio: 'inherit' });
console.log('Public candidate exported to public-release. Review docs/open-source-audit.md before publishing.');
