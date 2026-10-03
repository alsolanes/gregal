const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname,'../internal/web/app/preview.js'),'utf8');
const fn = source.slice(source.indexOf('export function previewURL'),source.indexOf('function show(')).replace('export ','');
const ctx=vm.createContext({URL,location:{origin:'http://127.0.0.1:8111'}});
vm.runInContext(fn,ctx);
test('preview only accepts credential-free external HTTP addresses',()=>{
  assert.equal(ctx.previewURL('http://localhost:3000'),'http://localhost:3000/');
  assert.equal(ctx.previewURL('https://example.com'),'https://example.com/');
  for(const url of ['javascript:alert(1)','file:///C:/secret','data:text/html,test','http://user:secret@example.com','http://127.0.0.1:8111/api/state','not a URL'])assert.equal(ctx.previewURL(url),null,url);
});
test('preview has no same-origin sandbox privilege or referrer leakage',()=>{
  assert.ok(source.includes('sandbox="allow-scripts allow-forms"'));
  assert.ok(source.includes('referrerpolicy="no-referrer"'));
});
