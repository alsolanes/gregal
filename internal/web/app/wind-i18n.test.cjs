const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

test('all Winds translation keys have both languages and matching template variables', async () => {
  const { CLAUS } = await import('./i18n.js');
  const files = ['winds.js','wind-desk.js','team.js'];
  const sources = files.map(file=>fs.readFileSync(path.join(__dirname,file),'utf8')).join('\n') + fs.readFileSync(path.join(__dirname,'../index.html'),'utf8');
  const keys = new Set([...sources.matchAll(/["'](winds\.[A-Za-z0-9]+)["']/g)].map(m=>m[1]));
  for (const key of keys) {
    assert.equal(typeof CLAUS.ca[key],'string',`Catalan: ${key}`);
    assert.equal(typeof CLAUS.en[key],'string',`English: ${key}`);
    const vars = value => [...value.matchAll(/\{(\w+)\}/g)].map(m=>m[1]).sort();
    assert.deepEqual(vars(CLAUS.ca[key]),vars(CLAUS.en[key]),key);
  }
});

test('live translation updates UI text and attributes, preserving user content and drafts', async () => {
  const { CLAUS } = await import('./i18n.js');
  let lang = 'en';
  const nodes = [], listeners = {};
  function element() {
    const attrs={};
    const el={textContent:'',value:'',setAttribute(k,v){attrs[k]=String(v);},getAttribute(k){return attrs[k]??null;},removeAttribute(k){delete attrs[k];}};
    nodes.push(el); return el;
  }
  const root={gregalT:key=>CLAUS[lang][key]||key, document:{addEventListener(name,fn){listeners[name]=fn;},querySelectorAll(selector){return nodes.filter(el=>el.getAttribute(selector.slice(1,-1))!==null);}}};
  vm.runInNewContext(fs.readFileSync(path.join(__dirname,'wind-i18n.js'),'utf8'),{window:root});
  const {text,paint,attribute}=root.gregalWindI18n;
  const label=element(), field=element(), userMessage=element();
  paint(label,text('winds.profileOf','Perfil de {name}',{name:'My wind'}));
  field.value='My custom draft';
  attribute(field,'placeholder',text('winds.whatSOnYourMind','Què tens al cap?'));
  paint(userMessage,'My saved message');
  assert.equal(label.textContent,'Profile of My wind');
  assert.equal(field.getAttribute('placeholder'),'What’s on your mind?');
  lang='ca'; listeners['gregal:idioma']();
  assert.equal(label.textContent,'Perfil de My wind');
  assert.equal(field.getAttribute('placeholder'),'Què tens al cap?');
  assert.equal(field.value,'My custom draft');
  assert.equal(userMessage.textContent,'My saved message');
  // Clearing a status removes metadata, so changing language cannot revive it.
  paint(label,''); lang='en'; listeners['gregal:idioma']();
  assert.equal(label.textContent,'');
});
