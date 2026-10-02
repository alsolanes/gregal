const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.resolve(__dirname, '..');
const vendor = path.join(root, 'internal', 'web', 'app', 'vendor');
const artifacts = [
  ['chart.umd.min.js', 'F52399835588EADA7BC2A9F720298D5F12FEA63C01D6A59C34BEEC89CCAA670F'],
  ['docx-preview.min.js', '52AB66FA16C5D02EE11CAF495287BD8E684DE1CE318101ECFC627220B637EC5B'],
  ['jszip.min.js', '0D8C303D7AC0C36B9903431481AA85AD0D1234A81182F6E96B7B687F385DAE1F'],
  ['PptxViewJS.min.js', 'F44EA1CACCB5FE82DD1531B52A110F85E05C2622CD1F896C70E156C73D38FC36'],
  ['xlsx.mini.min.js', '0CB353F830D7288385492C83D277B058DDEAC664CA51CF1393AA1FD3E2B70939'],
];

for (const [name, expected] of artifacts) {
  const file = path.join(vendor, name);
  const source = fs.readFileSync(file);
  const actual = crypto.createHash('sha256').update(source).digest('hex').toUpperCase();
  assert.equal(actual, expected, `${name} differs from the reviewed upstream artifact`);
}

const xlsxPath = path.join(vendor, 'xlsx.mini.min.js');
const context = {
  module: { exports: {} },
  require,
  Buffer,
  process,
  setTimeout,
  clearTimeout,
  console,
};
context.exports = context.module.exports;
vm.runInNewContext(fs.readFileSync(xlsxPath, 'utf8'), context, { filename: xlsxPath });

const XLSX = context.module.exports;
assert.equal(XLSX.version, '0.20.3');
assert.equal(typeof XLSX.utils.sheet_to_json, 'function');
const workbook = XLSX.utils.book_new();
const worksheet = XLSX.utils.aoa_to_sheet([['name', 'score'], ['Ada', 42]]);
XLSX.utils.book_append_sheet(workbook, worksheet, 'Scores');
const bytes = XLSX.write(workbook, { bookType: 'xlsx', type: 'buffer' });
const arrayBuffer = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
const parsed = XLSX.read(arrayBuffer, { type: 'array' });
const rows = XLSX.utils.sheet_to_json(parsed.Sheets.Scores, {
  header: 1,
  raw: true,
  defval: '',
});
assert.deepEqual(JSON.parse(JSON.stringify(rows)), [['name', 'score'], ['Ada', 42]]);

console.log('Vendor artifact pins and XLSX round-trip passed.');
