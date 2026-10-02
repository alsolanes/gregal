#!/usr/bin/env python3
"""Verifica la secció #contacte de T5. Imprimeix TOT BÉ o FALLA: motius."""
import re
import sys

html = open("index.html", encoding="utf-8").read()
css = open("styles.css", encoding="utf-8").read()
errs = []

m = re.search(r'<section[^>]*\bid=["\']contacte["\'][\s\S]*?</section>', html)
if not m:
    errs.append("no hi ha <section id=\"contacte\">")
    sec = ""
else:
    sec = m.group(0)
if '<h2' not in sec or "Contacte" not in sec:
    errs.append("falta <h2>Contacte</h2>")
if "<form" not in sec:
    errs.append("falta <form>")
for camp, etiq in [("nom", r'<input\b[^>]*\bname=["\']nom["\']'),
                   ("email", r'<input\b[^>]*\bname=["\']email["\']'),
                   ("missatge", r'<textarea\b[^>]*\bname=["\']missatge["\']')]:
    mm = re.search(etiq + r'[^>]*>', sec)
    if not mm:
        errs.append(f"falta camp {camp}")
        continue
    tag = mm.group(0)
    if "required" not in tag:
        errs.append(f"camp {camp} sense required")
    idm = re.search(r'\bid=["\']([^"\']+)["\']', tag)
    if not idm or not re.search(r'<label[^>]*\bfor=["\']' + idm.group(1) + r'["\']', sec):
        errs.append(f"camp {camp} sense <label for>")
if not re.search(r'<button[^>]*>Envia</button>', sec):
    errs.append("falta botó Envia")
if not re.search(r'@media', css):
    errs.append("s'ha perdut la @media")
if "#contacte" in css:
    bloc = css[css.index("#contacte"):]
    for prop in re.findall(r'(?:color|background(?:-color)?)\s*:\s*([^;]+);', bloc):
        if "var(--" not in prop:
            errs.append(f"color hardcoded fora de :root: {prop.strip()}")
            break
if 'lang="ca"' not in html:
    errs.append("s'ha perdut lang=\"ca\"")

if errs:
    print("FALLA: " + "; ".join(errs))
    sys.exit(1)
print("TOT BÉ")
