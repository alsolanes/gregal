#!/usr/bin/env python3
"""Independent structural checks for office eval tasks (stdlib only).

usage: office_check.py FILE [--headings N] [--table] [--list] [--slides N]
                            [--chart] [--formulas N] [--tracked TEXT]
                            [--deleted TEXT] [--contains TEXT]

Exit 0 when the file is a well-formed OOXML package (every XML part parses,
every internal relationship target exists) and meets every requirement.
"""
import argparse
import posixpath
import re
import sys
import zipfile
import xml.etree.ElementTree as ET

W = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
A = "{http://schemas.openxmlformats.org/drawingml/2006/main}"


def fail(msg):
    print("FAIL:", msg)
    sys.exit(1)


def parts(z):
    out = {}
    for n in z.namelist():
        if n.endswith(".xml") or n.endswith(".rels"):
            try:
                out[n] = ET.fromstring(z.read(n))
            except ET.ParseError as e:
                fail(f"{n} is not well-formed XML: {e}")
    return out


def check_rels(z, xml):
    names = set(z.namelist())
    for n, root in xml.items():
        if not n.endswith(".rels"):
            continue
        d, f = posixpath.split(n)
        src_dir = posixpath.dirname(d) if d.endswith("_rels") else d
        for r in root:
            if r.get("TargetMode") == "External":
                continue
            t = r.get("Target", "")
            target = t.lstrip("/") if t.startswith("/") else posixpath.normpath(posixpath.join(src_dir, t))
            if target not in names:
                fail(f"{n}: relationship {r.get('Id')} points to missing {target}")


def text_of(root, tag):
    return "".join(e.text or "" for e in root.iter(tag))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("file")
    ap.add_argument("--headings", type=int, default=0)
    ap.add_argument("--table", action="store_true")
    ap.add_argument("--list", action="store_true")
    ap.add_argument("--slides", type=int, default=0)
    ap.add_argument("--chart", action="store_true")
    ap.add_argument("--formulas", type=int, default=0)
    ap.add_argument("--tracked", default="")
    ap.add_argument("--deleted", default="")
    ap.add_argument("--contains", default="")
    a = ap.parse_args()
    try:
        z = zipfile.ZipFile(a.file)
    except Exception as e:
        fail(f"not a zip package: {e}")
    xml = parts(z)
    if "[Content_Types].xml" not in xml:
        fail("missing [Content_Types].xml")
    check_rels(z, xml)
    alltext = ""
    if "word/document.xml" in xml:
        doc = xml["word/document.xml"]
        alltext = text_of(doc, W + "t")
        styles = [p.get(W + "val", "") for p in doc.iter(W + "pStyle")]
        heads = [s for s in styles if re.match(r"(?i)heading|ttol|titol|t.tulo", s)]
        if len(heads) < a.headings:
            fail(f"{len(heads)} heading paragraphs, expected >= {a.headings} (styles: {sorted(set(styles))})")
        if a.table and not list(doc.iter(W + "tbl")):
            fail("no real Word table (w:tbl)")
        if a.list and not list(doc.iter(W + "numPr")):
            fail("no real list (w:numPr)")
        if a.tracked:
            ins = "".join(text_of(e, W + "t") for e in doc.iter(W + "ins"))
            if a.tracked not in ins:
                fail(f"tracked insertion with {a.tracked!r} not found (inserted: {ins!r})")
        if a.deleted:
            dl = "".join(e.text or "" for e in doc.iter(W + "delText"))
            if a.deleted not in dl:
                fail(f"tracked deletion with {a.deleted!r} not found (deleted: {dl!r})")
    slides = [n for n in xml if re.match(r"ppt/slides/slide\d+\.xml$", n)]
    if a.slides and len(slides) < a.slides:
        fail(f"{len(slides)} slides, expected >= {a.slides}")
    for n in slides:
        alltext += text_of(xml[n], A + "t")
    if a.chart and not any(n.startswith("ppt/charts/") or n.startswith("xl/charts/") for n in z.namelist()):
        fail("no native chart part")
    if a.formulas:
        n = 0
        for name, root in xml.items():
            if name.startswith("xl/worksheets/"):
                n += sum(1 for f in root.iter() if f.tag.endswith("}f") and (f.text or "").strip())
        if n < a.formulas:
            fail(f"{n} formulas, expected >= {a.formulas} (values must be formulas, not typed results)")
        for name, root in xml.items():
            if name.startswith("xl/worksheets/"):
                for v in root.iter():
                    if v.tag.endswith("}v") and (v.text or "").startswith("#"):
                        fail(f"formula error value {v.text} in {name}")
    if a.contains and a.contains not in alltext:
        fail(f"text {a.contains!r} not found")
    print("OK")


if __name__ == "__main__":
    main()
