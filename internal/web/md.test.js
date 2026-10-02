// node --test internal/web/md.test.js — el renderitzador de markdown de la UI.
const test = require("node:test");
const assert = require("node:assert/strict");
const path = require("node:path");
const { render } = require(path.join(__dirname, "app", "md.js"));

test("títols, llistes i paràgrafs (el que sortia en cru)", () => {
  const h = render("## Què he trobat\n\nHe llistat:\n\n- `README.md` — la porta\n- `CHANGELOG.md` — l'historial\n\n**Conclusió:** bé.");
  assert.match(h, /<h2>Què he trobat<\/h2>/);
  assert.match(h, /<ul>\s*<li><code>README.md<\/code> — la porta<\/li>/);
  assert.match(h, /<p><b>Conclusió:<\/b> bé.<\/p>/);
  assert.doesNotMatch(h, /##|^- /m);
});

test("llistes numerades i nivells per sagnat", () => {
  const h = render("1. un\n2. dos\n   - sub a\n   - sub b\n3. tres");
  assert.match(h, /<ol>\s*<li>un<\/li>\s*<li>dos<\/li>\s*<ul>\s*<li>sub a<\/li>\s*<li>sub b<\/li>\s*<\/ul>\s*<li>tres<\/li>\s*<\/ol>/);
});

test("bloc de codi intacte, amb capçalera i botó de copiar", () => {
  const h = render("abans\n\n```go\nfmt.Println(\"# no és un títol\")\n- ni una llista\n```\n\ndesprés");
  assert.match(h, /<div class="codeblock"><div class="codehead"><span>go<\/span><button class="copybtn">copia<\/button><\/div><pre><code>/);
  // El contingut porta ressaltat de sintaxi, però el text ha de ser
  // exactament el codi: dins del bloc no s'hi busquen títols («#») ni
  // llistes («- »).
  const cos = /<pre><code>([\s\S]*?)<\/code><\/pre>/.exec(h)[1].replace(/<[^>]+>/g, "");
  assert.equal(cos, 'fmt.Println(&quot;# no és un títol&quot;)\n- ni una llista');
  assert.match(h, /<p>abans<\/p>/);
  assert.match(h, /<p>després<\/p>/);
});

test("res d'HTML del model passa: tot escapat", () => {
  const h = render('<img src=x onerror=alert(1)> i <script>alert(2)</script>');
  assert.doesNotMatch(h, /<img|<script/);
  assert.match(h, /&lt;img src=x onerror=alert\(1\)&gt;/);
});

test("enllaços només http(s); javascript: queda com a text", () => {
  assert.match(render("[doc](https://example.com/a?b=1)"), /<a href="https:\/\/example.com\/a\?b=1" target="_blank" rel="noopener noreferrer">doc<\/a>/);
  assert.doesNotMatch(render("[x](javascript:alert(1))"), /<a /);
  assert.match(render("mira https://example.com ara"), /<a href="https:\/\/example.com"/);
});

test("cursiva, ratllat, cita, regla i taula", () => {
  assert.match(render("això *va* i _això_ també, ~~no~~"), /<i>va<\/i>.*<i>això<\/i>.*<s>no<\/s>/);
  assert.match(render("> una cita\n> de dues línies"), /<blockquote><p>una cita<br>de dues línies<\/p><\/blockquote>/);
  assert.match(render("a\n\n---\n\nb"), /<hr>/);
  const t = render("| a | b |\n|---|---|\n| 1 | **2** |");
  assert.match(t, /<table class="md"><thead><tr><th>a<\/th><th>b<\/th><\/tr><\/thead><tbody><tr><td>1<\/td><td><b>2<\/b><\/td><\/tr><\/tbody><\/table>/);
});

test("caselles de tasca", () => {
  const h = render("- [ ] pendent\n- [x] fet");
  assert.match(h, /<li><input type="checkbox" disabled> pendent<\/li>/);
  assert.match(h, /<li><input type="checkbox" disabled checked> fet<\/li>/);
});

test("guions dins d'un paràgraf no són llistes; asteriscs de multiplicació no són cursiva", () => {
  assert.match(render("preu - iva"), /<p>preu - iva<\/p>/);
  assert.match(render("2 * 3 * 4"), /<p>2 \* 3 \* 4<\/p>/);
});

test("ressaltat de sintaxi: paraules clau, cadenes, comentaris i números", () => {
  const h = render("```go\n// compta\nfunc Suma(a, b int) int {\n  return a + 42\n}\n```");
  assert.match(h, /<span class="hl-com">\/\/ compta<\/span>/);
  assert.match(h, /<span class="hl-kw">func<\/span>/);
  assert.match(h, /<span class="hl-kw">return<\/span>/);
  assert.match(h, /<span class="hl-num">42<\/span>/);
});

test("una paraula clau dins d'una cadena no es pinta com a clau", () => {
  const h = render('```js\nconst s = "return this";\n```');
  const dins = /<span class="hl-str">[^<]*return[^<]*<\/span>/.test(h);
  assert.ok(dins, "la cadena sencera ha d'anar en un sol span: " + h);
});

test("el ressaltat no altera el codi ni deixa marcadors", () => {
  const codi = 'func main() { fmt.Println("hola <b>&amp; adéu</b>") }';
  const h = render("```go\n" + codi + "\n```");
  assert.doesNotMatch(h, /<<H/);
  // El text visible del bloc és el codi original, sense res afegit.
  const cos = /<pre><code>([\s\S]*?)<\/code><\/pre>/.exec(h)[1].replace(/<[^>]+>/g, "");
  assert.equal(cos, "func main() { fmt.Println(&quot;hola &lt;b&gt;&amp;amp; adéu&lt;/b&gt;&quot;) }");
});
