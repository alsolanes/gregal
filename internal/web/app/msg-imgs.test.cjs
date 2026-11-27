// L'adjunt va a sobre del text: només imatge, imatge amb comentari i
// diverses imatges. El text no pot quedar mai per sobre de la imatge.
const test = require('node:test');
const assert = require('node:assert');
const imgs = require('./msg-imgs.js');

const A = 'data:image/png;base64,AA';
const B = 'data:image/png;base64,BB';

test('sense imatges no hi ha bloc', () => {
  assert.equal(imgs.html(), '');
  assert.equal(imgs.html([]), '');
  assert.equal(imgs.html(null), '');
});

test('una imatge va abans del text', () => {
  const body = imgs.html([A]) + '<p>descriu-la</p>';
  assert.ok(body.startsWith('<div class="msg-imgs"><img src="' + A + '" alt=""></div>'), body);
  assert.ok(body.indexOf('msg-imgs') < body.indexOf('<p>'), 'the image must come first');
});

test('diverses imatges, totes a sobre del text', () => {
  const body = imgs.html([A, B]) + '<p>compara-les</p>';
  assert.equal((body.match(/<img /g) || []).length, 2);
  assert.ok(body.lastIndexOf('<img ') < body.indexOf('<p>'), 'every image must come before the text');
});

test('una adreça amb cometes no trenca l\'atribut', () => {
  assert.ok(imgs.html(['a"b']).includes('src="a&quot;b"'));
});
