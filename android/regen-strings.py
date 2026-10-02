"""Regenera Strings.kt des d'internal/web/app/i18n.js (paritat ca/en amb la web)."""
import re
src = open('internal/web/app/i18n.js').read()
def parse(name):
    m = re.search(r'const ' + name + r' = \{(.*?)\n\};', src, re.S)
    return {k: v.replace("\\'", "'") for k, v in re.findall(r"'([^']+)'\s*:\s*'((?:[^'\\]|\\.)*)'", m.group(1))}
ca, en = parse('CA'), parse('EN')
assert set(ca) == set(en), f"diferents: {set(ca) ^ set(en)}"
def kstr(d):
    return '\n'.join(f'    "{k}" to "{v}",' for k, v in sorted(d.items()))
out = '''// Generat des d'internal/web/app/i18n.js — NO EDITAR A MÀ.
// Paritat ca/en amb la web. Regenera amb: python3 android/regen-strings.py
package xyz.solanes.gregal

val STR_CA: Map<String, String> = mapOf(
%s
)

val STR_EN: Map<String, String> = mapOf(
%s
)
''' % (kstr(ca), kstr(en))
open('android/app/src/main/java/xyz/solanes/gregal/Strings.kt', 'w').write(out)
print(f'Strings.kt: {len(ca)} claus')
