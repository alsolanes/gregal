const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const appDir = __dirname;
const previewSource = fs.readFileSync(path.join(appDir, 'preview.js'), 'utf8');
const artifactSource = fs.readFileSync(path.join(appDir, 'web-artifact.js'), 'utf8');
const markdownSource = fs.readFileSync(path.join(appDir, 'md.js'), 'utf8');
const indexSource = fs.readFileSync(path.join(appDir, '../index.html'), 'utf8');

function previewFixture() {
  const elements = new Map();
  const timers = new Map();
  let nextTimer = 0;
  let pane;

  class Element {
    constructor(id = '') {
      this._id = id;
      this.children = [];
      this.hidden = false;
      this.attributes = {};
      this.style = { setProperty() {} };
      this.classList = { toggle() {} };
      this.innerHTMLWrites = 0;
    }
    set id(value) { if (elements.get(this._id) === this) elements.delete(this._id); this._id=value; if(value)elements.set(value,this); }
    get id() {return this._id;}
    set innerHTML(value) {
      this._innerHTML = value;
      this.innerHTMLWrites++;
      for (const [, id] of value.matchAll(/id="([^"]+)"/g)) {
        const child = new Element(id);
        elements.set(id, child);
      }
      if (this.id === 'livePreview') {
        this.heading = new Element('previewHeading');
        elements.set('previewHeading', this.heading);
      }
    }
    get innerHTML() { return this._innerHTML || ''; }
    setAttribute(name, value) { this.attributes[name] = String(value); }
    removeAttribute(name) { delete this[name]; }
    append(...children) {this.children.push(...children);}
    replaceChildren(...children) {this.children=children;}
    remove() {if(elements.get(this.id)===this)elements.delete(this.id);}
    focus() {}
    querySelector(selector) { return selector === 'header strong' ? this.heading : null; }
  }

  const workbench = { append(child) { this.child = child; } };
  const head = { append() {} };
  elements.set('workbench', workbench);
  elements.set('previewToggle', new Element('previewToggle'));
  const document = {
    documentElement: { lang: 'en' },
    head,
    createElement: tag => {
      const element = new Element();
      if (tag === 'aside') pane = element;
      return element;
    },
    getElementById: id => elements.get(id) || null,
    addEventListener() {},
  };
  const window = { gregal: {} };
  const storage=new Map();
  const context = vm.createContext({
    sessionStorage: {getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,value)},
    document, window, URL, Date, location: { origin: 'http://127.0.0.1:8111' }, innerWidth: 1280,
    setTimeout(fn) { const id = ++nextTimer; timers.set(id, fn); return id; },
    clearTimeout(id) { timers.delete(id); },
  });

  const isolatedStart = artifactSource.indexOf('export function isolatedHTML(');
  vm.runInContext(artifactSource.slice(isolatedStart).replace(/^export /, ''), context);
  vm.runInContext(markdownSource, context);
  window.gregal.md = context.gregalMD.render;
  vm.runInContext(previewSource.replace("import { isolatedHTML } from './web-artifact.js';", '')
    .replace('export function previewURL', 'function previewURL'), context);

  return {
    window, elements, pane, timers,
    flushTimers() {
      const pending = [...timers.values()];
      timers.clear();
      pending.forEach(fn => fn());
    },
  };
}

const html = title => `<!doctype html><html><body><h1>${title}</h1></body></html>`;

test('automatic previews respect close until a new turn or a manual preview request', () => {
  const { window, pane, elements, flushTimers } = previewFixture();
  const request = { kind: 'html', content: html('Draft') };

  window.gregalPreview.begin();
  assert.equal(window.gregalPreview.request(request, true), true);
  flushTimers();
  elements.get('previewClose').onclick();
  assert.equal(pane.hidden, true);
  assert.equal(window.gregalPreview.request(request, true), false);
  assert.equal(pane.hidden, true);

  assert.equal(window.gregalPreview.request(request), true);
  flushTimers();
  assert.equal(pane.hidden, false);
  elements.get('previewToggle').onclick();
  assert.equal(pane.hidden, true);
  assert.equal(window.gregalPreview.request(request, true), false);
  elements.get('previewToggle').onclick();
  assert.equal(pane.hidden, false);
  assert.equal(window.gregalPreview.request(request, true), true);
  elements.get('previewClose').onclick();
  window.gregalPreview.begin();
  assert.equal(window.gregalPreview.request(request, true), true);
});

test('HTML, Markdown, and URL opens reset the title and URL controls', () => {
  const { window, elements, flushTimers } = previewFixture();
  const heading = elements.get('previewHeading');
  const form = elements.get('previewForm');
  const title = '<img src=x onerror=alert(1)>';

  assert.equal(window.gregalPreview.request({ kind: 'markdown', content: '# Notes', title }), true);
  assert.equal(heading.textContent, title);
  assert.equal(heading.innerHTMLWrites, 0);
  assert.equal(form.hidden, true);
  flushTimers();

  window.gregalPreview.openHTML(html('Direct artifact'));
  assert.equal(heading.textContent, 'Preview');
  assert.equal(form.hidden, true);
  flushTimers();

  assert.equal(window.gregalPreview.open('https://example.test/page'), undefined);
  assert.equal(heading.textContent, 'Preview');
  assert.equal(form.hidden, false);
  assert.equal(elements.get('previewContent').src, 'https://example.test/page');
});

test('synchronous artifact updates render only the newest isolated document', () => {
  const { window, elements, flushTimers, timers } = previewFixture();
  const frame = elements.get('previewContent');

  window.gregalPreview.begin();
  assert.equal(window.gregalPreview.request({ kind: 'markdown', content: '# Plan', title: 'Plan' }, true), true);
  assert.equal(window.gregalPreview.request({ kind: 'html', content: html('Reviewed website'), title: 'Reviewer' }, true), true);
  assert.equal(timers.size, 1);
  flushTimers();
  assert.match(frame.srcdoc, /Reviewed website/);
  assert.doesNotMatch(frame.srcdoc, /<h1>Plan<\/h1>/);
  assert.match(frame.srcdoc, /Content-Security-Policy/);
  assert.equal(elements.get('previewHeading').textContent, 'Reviewer');

  window.gregalPreview.request({ kind: 'markdown', content: '# Safe\n\n<script>alert(1)</script>' });
  flushTimers();
  assert.match(frame.srcdoc, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.doesNotMatch(frame.srcdoc, /<script>alert\(1\)/);
  assert.equal(elements.get('previewHeading').innerHTMLWrites, 0);
});

test('URL navigation and reload cancel a queued artifact write', () => {
  const { window, elements, flushTimers, timers } = previewFixture();

  window.gregalPreview.openHTML(html('Pending artifact'));
  assert.equal(timers.size, 1);
  elements.get('previewReload').onclick();
  assert.equal(timers.size, 0);
  assert.match(elements.get('previewContent').srcdoc, /Pending artifact/);

  window.gregalPreview.openHTML(html('Stale artifact'));
  assert.equal(timers.size, 1);
  window.gregalPreview.open('https://example.test/next');
  assert.equal(timers.size, 0);
  flushTimers();
  assert.equal(elements.get('previewContent').src, 'https://example.test/next');
  assert.equal(elements.get('previewContent').srcdoc, undefined);
});

test('invalid preview requests leave the current preview alone', () => {
  const { window, pane, elements, flushTimers } = previewFixture();
  assert.equal(window.gregalPreview.request({ kind: 'html', content: html('Current') }), true);
  flushTimers();
  const frame = elements.get('previewContent');
  const currentDocument = frame.srcdoc;

  for (const request of [
    null,
    { kind: 'html', content: '' },
    { kind: 'markdown', content: ' \n ' },
    { kind: 'url', content: 'https://remote.example/' },
    { kind: 'script', content: 'x' },
    { kind: 'html', content: 'x'.repeat(200001) },
  ]) assert.equal(window.gregalPreview.request(request), false);

  assert.equal(pane.hidden, false);
  assert.equal(frame.srcdoc, currentDocument);
});

test('agent tool preview is automatic and each ordinary agent turn clears prior dismissal', () => {
  const doAgent = indexSource.indexOf('async function doAgent(task, resumed)');
  assert.ok(doAgent >= 0);
  const start = indexSource.slice(doAgent, doAgent + 700);
  assert.match(start, /if \(!resumed\) window\.gregalPreview\?\.begin\(runSession\)/);
  assert.match(start, /if \(!text\) return;[\s\S]*window\.gregalPreview\?\.begin\(runSession\)/);
  assert.match(indexSource, /window\.gregalPreview\.request\(JSON\.parse\(d\.output\), true, runSession\)/);
  assert.match(indexSource, /gregalPreview\?\.offer\(previewMatch\[0\], runSession\)/);
});


test('each session keeps its own tabs, frames and dismissed preview state', () => {
  const {window,elements,pane,flushTimers}=previewFixture();
  const preview=window.gregalPreview;
  preview.switchSession('alpha');
  preview.request({kind:'url',content:'http://localhost:3000',title:'Alpha'},true,'alpha');
  const alphaFrame=elements.get('previewContent');
  alphaFrame.formValue='unsaved page input';
  preview.request({kind:'url',content:'http://localhost:4000',title:'Alpha docs'},true,'alpha');
  assert.equal(preview.getState('alpha').tabs.length,2);
  const alphaDocs=elements.get('previewContent');
  preview.request({kind:'html',content:html('Beta'),title:'Beta'},true,'beta');
  flushTimers();
  assert.equal(elements.get('previewContent'),alphaDocs,'background project cannot replace the selected frame');
  assert.equal(elements.get('previewHeading').textContent,'Alpha docs');
  preview.switchSession('beta');
  assert.equal(preview.getState().tabs.length,1);
  assert.equal(elements.get('previewHeading').textContent,'Beta');
  assert.equal(alphaFrame.hidden,true);
  preview.switchSession('alpha');
  preview.selectTab(preview.getState().tabs[0].id);
  assert.equal(elements.get('previewContent'),alphaFrame);
  assert.equal(alphaFrame.formValue,'unsaved page input');
  elements.get('previewClose').onclick();
  assert.equal(pane.hidden,true);
  preview.begin('beta');
  assert.equal(preview.request({kind:'html',content:html('Do not reopen')},true,'alpha'),false);
  preview.switchSession('beta');
  assert.equal(pane.hidden,false);
  preview.forgetSession('alpha');
  assert.equal(preview.getState('alpha').tabs.length,0);
  assert.equal(preview.getState('beta').tabs.length,1);
});

test('repeated automatic navigation updates one site tab and manual tabs close independently', () => {
  const {window,elements}=previewFixture();
  const preview=window.gregalPreview;
  preview.request({kind:'url',content:'http://localhost:3000/a'},true);
  preview.request({kind:'url',content:'http://localhost:3000/b'},true);
  assert.equal(preview.getState().tabs.length,1);
  assert.equal(elements.get('previewContent').src,'http://localhost:3000/b');
  preview.newTab();
  preview.open('http://localhost:4000');
  const state=preview.getState();
  assert.equal(state.tabs.length,2);
  preview.closeTab(state.active);
  assert.equal(preview.getState().tabs.length,1);
  assert.equal(elements.get('previewContent').src,'http://localhost:3000/b');
});
