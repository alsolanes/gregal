#!/usr/bin/env python3
"""Verifica el catàleg de T7 estàticament. Imprimeix TOT BÉ o FALLA: motius."""
import re

html = open("index.html", encoding="utf-8").read()
css = open("styles.css", encoding="utf-8").read()
js = open("app.js", encoding="utf-8").read()
errs = []

if 'id="filtre"' not in html:
    errs.append("falta #filtre a index.html")
if 'id="categoria"' not in html:
    errs.append("falta #categoria a index.html")
if 'id="cataleg"' not in html:
    errs.append("falta #cataleg a index.html")
if "targeta" not in html and "targeta" not in js:
    errs.append("cap targeta amb classe targeta")

noms = re.findall(r'nom\s*:\s*["\']([^"\']+)["\']', js)
if len(noms) < 4:
    errs.append(f"l'array té {len(noms)} noms, calen 4")
cats = set(re.findall(r'categoria\s*:\s*["\']([^"\']+)["\']', js))
if len(cats) < 2:
    errs.append("calen dues categories diferents")
for ev in ["input", "change"]:
    if ev not in js:
        errs.append(f"falta listener {ev} a app.js")
if "cataleg" not in js:
    errs.append("app.js no pinta a #cataleg")
if ".targeta" not in css:
    errs.append("falta .targeta a styles.css")
sec = css[css.find(".targeta"):] if ".targeta" in css else ""
if "var(--" not in sec:
    errs.append(".targeta no usa variables CSS")
if "@media" in css:
    if ".targeta" not in css[css.find("@media"):]:
        errs.append("la @media no reordena .targeta")
else:
    errs.append("falta @media que reordeni .targeta")

print("TOT BÉ" if not errs else "FALLA: " + "; ".join(errs))
